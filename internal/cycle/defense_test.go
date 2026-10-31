package cycle_test

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/LaurenteEber/tupaqnin/internal/cycle"
	"github.com/LaurenteEber/tupaqnin/internal/graph"
	"github.com/LaurenteEber/tupaqnin/internal/provider"
	"github.com/LaurenteEber/tupaqnin/internal/store"
)

// The section the defense tests write and the step that touches it: it
// uses the decision of yours already used in it, so a conclusion
// section has no «escrito sin decisión» (B-42) and the stop proposes
// signing it (backlog-r2 2.5).
const (
	defenseText = "Comparamos solo estudios con el mismo tamaño de partícula.\n\nAsí la comparación es justa."
	defenseStep = `{"stretch":"build","triage_line":"Construir: cierro la regla.","closing_risk":"none","note":"Cerré la regla.",` +
		`"used_decisions":["dec-1"],` +
		`"new_text":[{"section_id":"sec-1","base_version":1,"anchor":"Así la comparación es justa.","claim_origin":"material",` +
		`"operation":"replace","text":"Así la comparación es justa y repetible."}]}`
)

// seedDefense seeds the project of the defense tests: the request with
// its audience («para quién»), a criterion, sec-1 with the role given
// and dec-1, a decision of yours used in it.
func seedDefense(t *testing.T, db *store.DB, audience, role string) {
	t.Helper()
	human := store.EventInput{Type: store.EventStateChanged, Actor: "human"}
	err := db.Write(context.Background(), "p1", func(tx *store.Tx) error {
		req, err := tx.CreateObject(store.ObjectRequest, "", map[string]any{
			"text": "Escribe la regla de comparación de los estudios.", "audience": audience}, human)
		if err != nil {
			return err
		}
		if _, err := tx.CreateObject(store.ObjectCriterion, "current",
			map[string]any{"text": "Comparar conteos de partículas.", "request_id": req.ID}, human); err != nil {
			return err
		}
		sec, err := tx.CreateObject(store.ObjectSection, "draft", map[string]any{
			"title": "Regla de comparación", "text": defenseText,
			"role": map[string]any{"value": role, "state": "confirmed"}, "open_changes": []any{},
		}, human)
		if err != nil {
			return err
		}
		dec, err := tx.CreateObject(store.ObjectDecision, "human", map[string]any{
			"statement": "Comparar solo estudios con el mismo tamaño de partícula.", "origin": "human",
			"instead_of": "Comparar los conteos tal cual.",
		}, human)
		if err != nil {
			return err
		}
		_, err = tx.AddEdge(store.EdgeUsedIn, dec.ID, sec.ID, "", nil, store.EventInput{Type: store.EventEdge, Actor: "human"})
		return err
	})
	if err != nil {
		t.Fatalf("seed defense: %v", err)
	}
}

func judgeFixture(t *testing.T, name string) provider.Fixture {
	t.Helper()
	fx, err := provider.LoadFixture(filepath.Join("..", "provider", "testdata", "judge", name+".json"))
	if err != nil {
		t.Fatalf("LoadFixture: %v", err)
	}
	return fx
}

// judgeTextFixture is a hand-made judge output.
func judgeTextFixture(t *testing.T, text string) provider.Fixture {
	t.Helper()
	fx := textFixture(t, text)
	fx.TurnKind = provider.TurnJudgeDefense
	return fx
}

// defenseOutput is the judge output a recording carries: the tests read
// their expectations from it, so a real recording keeps them meaningful
// (P-71).
type defenseOutput struct {
	AudienceQuestions []struct {
		Question string `json:"question"`
		Section  string `json:"section"`
	} `json:"audience_questions"`
	NotCovered []struct {
		Statement string `json:"statement"`
		Section   string `json:"section"`
	} `json:"not_covered"`
}

func recordedDefense(t *testing.T, fx provider.Fixture) defenseOutput {
	t.Helper()
	resp, err := provider.NewFake(fx).Respond(context.Background(), provider.Request{TurnKind: provider.TurnJudgeDefense}, nil)
	if err != nil {
		t.Fatalf("fake Respond: %v", err)
	}
	var out defenseOutput
	if err := json.Unmarshal([]byte(resp.Text), &out); err != nil {
		t.Fatalf("decode judge output: %v", err)
	}
	return out
}

// audienceQuestions are the project's audience questions with the
// section their about edge points to.
func audienceQuestions(t *testing.T, db *store.DB) ([]store.Object, map[string]string) {
	t.Helper()
	ctx := context.Background()
	qs, err := db.ListObjects(ctx, "p1", store.ObjectAudienceQuestion)
	if err != nil {
		t.Fatalf("ListObjects: %v", err)
	}
	about := map[string]string{}
	for _, q := range qs {
		edges, err := db.EdgesFrom(ctx, "p1", q.ID)
		if err != nil {
			t.Fatalf("EdgesFrom: %v", err)
		}
		for _, e := range edges {
			if e.Type != store.EdgeAbout {
				continue
			}
			to, err := db.GetObject(ctx, "p1", e.ToID)
			if err != nil {
				t.Fatalf("GetObject: %v", err)
			}
			about[q.Label] = to.Label
		}
	}
	return qs, about
}

func bodyOfObject(t *testing.T, obj store.Object) map[string]any {
	t.Helper()
	var b map[string]any
	if err := json.Unmarshal(obj.Body, &b); err != nil {
		t.Fatalf("body: %v", err)
	}
	return b
}

func userTexts(items []json.RawMessage) string {
	var b strings.Builder
	for _, raw := range items {
		var item struct {
			Role    string `json:"role"`
			Content []struct {
				Text string `json:"text"`
			} `json:"content"`
		}
		if json.Unmarshal(raw, &item) == nil && item.Role == "user" {
			for _, c := range item.Content {
				b.WriteString(c.Text + "\n")
			}
		}
	}
	return b.String()
}

// runDefense seeds the project, runs one turn whose step touches sec-1
// and returns what it left.
func runDefense(t *testing.T, audience, role string, judge ...provider.Fixture) (*store.DB, *cycle.Runner, *captureProvider) {
	t.Helper()
	fixtures := append([]provider.Fixture{textFixture(t, defenseStep)}, judge...)
	db, runner, cap, _ := newCaptureRunner(t, fixtures...)
	seedDefense(t, db, audience, role)
	if err := runner.RunTurn(context.Background(), "p1", "turn-1", "Cierra la regla."); err != nil {
		t.Fatalf("RunTurn: %v", err)
	}
	reports := stopReports(t, db)
	if len(reports) != 1 || reports[0].Section != "sec-1" || !reports[0].ProposeSignoff {
		t.Fatalf("stop reports: %+v, want one proposing to sign sec-1", reports)
	}
	return db, runner, cap
}

// Backlog-r2 2.6 (fichas §2 Parada, §8.2, K-1): at the stop that proposes
// signing a section with role conclusion, the judge makes one call with
// its reduced brief (Spark 1.3 high), and each likely question and each
// «no cubre» becomes an open audience question with its about edge.
func TestK1_ConclusionSectionGetsThreeToFiveAudienceQuestionsWithAbout(t *testing.T) {
	fx := judgeFixture(t, "conclusion-section")
	want := recordedDefense(t, fx)
	db, _, cap := runDefense(t, "", "conclusion", fx)

	if len(cap.judges) != 1 {
		t.Fatalf("judge calls: %d, want 1", len(cap.judges))
	}
	req := cap.judges[0]
	if req.Role != provider.RoleJudge || req.Model != provider.ModelSpark13 || req.Effort != provider.EffortHigh ||
		req.InstanceID != "" {
		t.Fatalf("judge call: role %q model %q effort %q instance %q", req.Role, req.Model, req.Effort, req.InstanceID)
	}
	brief := userTexts(req.Input)
	sec, err := db.GetObject(context.Background(), "p1", "sec-1")
	if err != nil {
		t.Fatal(err)
	}
	for _, part := range []string{"Así la comparación es justa y repetible.", "Escribe la regla de comparación",
		"Comparar conteos de partículas.", "Comparar solo estudios con el mismo tamaño de partícula.", "sec-1"} {
		if !strings.Contains(brief, part) {
			t.Fatalf("judge brief lacks %q:\n%s", part, brief)
		}
	}
	if strings.Contains(brief, sec.ID) {
		t.Fatal("the judge brief carries an internal id (rule 7)")
	}

	if n := len(want.AudienceQuestions); n < 3 || n > 5 {
		t.Fatalf("the recording has %d questions, want 3 to 5", n)
	}
	qs, about := audienceQuestions(t, db)
	if len(qs) != len(want.AudienceQuestions)+len(want.NotCovered) {
		t.Fatalf("audience questions: %d, want %d", len(qs), len(want.AudienceQuestions)+len(want.NotCovered))
	}
	questions, notCovered := 0, 0
	for _, q := range qs {
		b := bodyOfObject(t, q)
		if q.State != "open" || about[q.Label] != "sec-1" || b["section_id"] != sec.ID {
			t.Fatalf("%s: state %q, about %q, body %v", q.Label, q.State, about[q.Label], b)
		}
		if nc, _ := b["not_covered"].(bool); nc {
			notCovered++
			continue
		}
		questions++
	}
	if questions != len(want.AudienceQuestions) || notCovered != len(want.NotCovered) {
		t.Fatalf("questions %d, not covered %d; want %d and %d", questions, notCovered,
			len(want.AudienceQuestions), len(want.NotCovered))
	}
	b := bodyOfObject(t, qs[0])
	if b["question"] != want.AudienceQuestions[0].Question {
		t.Fatalf("aq-1 question %q, want the judge's literal %q", b["question"], want.AudienceQuestions[0].Question)
	}
}

// B-38: with «para quién» empty and role other, nobody but you reads the
// section and the judge is not called.
func TestB38_EmptyAudienceAndRoleOtherDoNotCallTheJudge(t *testing.T) {
	db, _, cap := runDefense(t, "  ", "other", judgeFixture(t, "conclusion-section"))
	if len(cap.judges) != 0 {
		t.Fatalf("judge calls: %d, want 0", len(cap.judges))
	}
	if qs, _ := audienceQuestions(t, db); len(qs) != 0 {
		t.Fatalf("audience questions: %d, want 0", len(qs))
	}
}

// B-38: «para quién» with text is someone else than you (R6), so a
// section with role other gets its defense too, with the audience in the
// brief.
func TestB38_AudienceWithTextCallsTheJudgeOnASectionWithRoleOther(t *testing.T) {
	_, _, cap := runDefense(t, "El comité de la universidad", "other", judgeFixture(t, "conclusion-section"))
	if len(cap.judges) != 1 {
		t.Fatalf("judge calls: %d, want 1", len(cap.judges))
	}
	if brief := userTexts(cap.judges[0].Input); !strings.Contains(brief, "El comité de la universidad") {
		t.Fatalf("judge brief lacks the audience:\n%s", brief)
	}
}

// Backlog-r2 2.6: a stop that does not propose signing does not call the
// judge, whatever the role.
func TestK1_AStopThatDoesNotProposeSigningDoesNotCallTheJudge(t *testing.T) {
	db, runner, cap, _ := newCaptureRunner(t, textFixture(t, pendingFigureStep), judgeFixture(t, "conclusion-section"))
	seedDefense(t, db, "El comité", "conclusion")
	if err := runner.RunTurn(context.Background(), "p1", "turn-1", "Escribe."); err != nil {
		t.Fatalf("RunTurn: %v", err)
	}
	reports := stopReports(t, db)
	if len(reports) != 1 || reports[0].ProposeSignoff {
		t.Fatalf("stop reports: %+v, want one that does not propose signing", reports)
	}
	if len(cap.judges) != 0 {
		t.Fatalf("judge calls: %d, want 0", len(cap.judges))
	}
}

// Estados §9, K-1: «no» leaves it declined with audience_question_declined
// («no abordado, decidido por ti»); it cannot be answered again.
func TestK1_NoRecordsAudienceQuestionDeclined(t *testing.T) {
	db, _, _ := runDefense(t, "", "conclusion", judgeFixture(t, "conclusion-section"))
	ctx := context.Background()
	gate := cycle.NewGate(db)
	if err := gate.AnswerAudienceQuestion(ctx, "p1", "aq-1", cycle.AnswerNo); err != nil {
		t.Fatalf("AnswerAudienceQuestion: %v", err)
	}
	aq, err := db.GetObject(ctx, "p1", "aq-1")
	if err != nil || aq.State != "declined" {
		t.Fatalf("aq-1: %q %v, want declined", aq.State, err)
	}
	declined := eventsOfType(t, db, store.EventAudienceQuestionDeclined)
	if len(declined) != 1 || declined[0].Actor != "human" || declined[0].ObjectID != aq.ID {
		t.Fatalf("audience_question_declined: %+v", declined)
	}
	if err := gate.AnswerAudienceQuestion(ctx, "p1", "aq-1", cycle.AnswerYes); !errors.Is(err, cycle.ErrAudienceQuestionNotOpen) {
		t.Fatalf("second answer: %v, want ErrAudienceQuestionNotOpen", err)
	}
	if err := gate.AnswerAudienceQuestion(ctx, "p1", "aq-2", "maybe"); !errors.Is(err, cycle.ErrInvalidAudienceAnswer) {
		t.Fatalf("answer «maybe»: %v, want ErrInvalidAudienceAnswer", err)
	}
	if err := gate.AnswerAudienceQuestion(ctx, "p1", "sec-1", cycle.AnswerNo); !errors.Is(err, cycle.ErrAudienceQuestionNotFound) {
		t.Fatalf("answer on a section: %v, want ErrAudienceQuestionNotFound", err)
	}
	if qs, _ := audienceQuestions(t, db); qs[1].State != "open" {
		t.Fatalf("aq-2: %q, want still open", qs[1].State)
	}
}

// Estados §9, K-1: «sí» leaves it addressed; it opens no turn, and it is
// an instruction of yours in what the principal receives in its next
// turn. One you did not answer is not.
func TestK1_YesBecomesAnInstructionInThePrincipalsNextTurnWithoutOpeningOne(t *testing.T) {
	fx := judgeFixture(t, "conclusion-section")
	want := recordedDefense(t, fx)
	db, runner, cap := runDefense(t, "", "conclusion", fx)
	ctx := context.Background()
	callsBefore := modelCallCount(t, db)
	eventsBefore, err := db.EventsSince(ctx, "p1", 0)
	if err != nil {
		t.Fatal(err)
	}
	if err := cycle.NewGate(db).AnswerAudienceQuestion(ctx, "p1", "aq-1", cycle.AnswerYes); err != nil {
		t.Fatalf("AnswerAudienceQuestion: %v", err)
	}
	aq, err := db.GetObject(ctx, "p1", "aq-1")
	if err != nil || aq.State != "addressed" {
		t.Fatalf("aq-1: %q %v, want addressed", aq.State, err)
	}
	events, err := db.EventsSince(ctx, "p1", eventsBefore[len(eventsBefore)-1].Seq)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 || events[0].Type != store.EventStateChanged || events[0].Actor != "human" {
		t.Fatalf("events of «sí»: %+v, want one state_changed of yours", events)
	}
	if callsAfter := modelCallCount(t, db); callsAfter != callsBefore || len(cap.sent) != 1 {
		t.Fatalf("«sí» called a model: calls %d → %d, steps %d", callsBefore, callsAfter, len(cap.sent))
	}

	if err := runner.RunTurn(ctx, "p1", "turn-2", "Sigue."); err != nil {
		t.Fatalf("RunTurn: %v", err)
	}
	if len(cap.sent) < 2 {
		t.Fatalf("step calls: %d, want the next turn's", len(cap.sent))
	}
	next := userTexts(cap.sent[1])
	if !strings.Contains(next, want.AudienceQuestions[0].Question) {
		t.Fatalf("the next turn lacks the addressed question:\n%s", next)
	}
	if strings.Contains(next, want.AudienceQuestions[1].Question) {
		t.Fatalf("the next turn carries a question you did not answer:\n%s", next)
	}
	sec, _ := db.GetObject(ctx, "p1", "sec-1")
	if strings.Contains(next, sec.ID) || strings.Contains(next, aq.ID) {
		t.Fatal("the instruction carries an internal id (rule 7)")
	}
}

// Estados §9: the defense does not block; with open audience questions,
// asking to sign opens the signoff, and the questions stay open.
func TestK1_WithOpenAudienceQuestionsTheSignoffOpensAnyway(t *testing.T) {
	db, runner, _ := runDefense(t, "", "conclusion", judgeFixture(t, "conclusion-section"))
	ctx := context.Background()
	open, err := runner.RequestSignoff(ctx, "p1", "sec-1")
	if err != nil {
		t.Fatalf("RequestSignoff: %v", err)
	}
	if !open.Opened || len(open.Pendings) != 0 || len(open.OpenChanges) != 0 {
		t.Fatalf("signoff: %+v, want opened with nothing blocking", open)
	}
	qs, _ := audienceQuestions(t, db)
	for _, q := range qs {
		if q.State != "open" {
			t.Fatalf("%s: %q, want still open", q.Label, q.State)
		}
	}
}

// Fichas §7: a judge output that names a section other than the one in
// question is retried once; the valid retry creates the questions.
func TestK1_AJudgeOutputOnAnotherSectionIsRetried(t *testing.T) {
	bad := `{"audience_questions":[{"question":"¿Uno?","section":"sec-9"},{"question":"¿Dos?","section":"sec-1"},` +
		`{"question":"¿Tres?","section":"sec-1"}],"not_covered":[]}`
	good := judgeFixture(t, "conclusion-section")
	db, _, cap := runDefense(t, "", "conclusion", judgeTextFixture(t, bad), good)
	if len(cap.judges) != 2 {
		t.Fatalf("judge calls: %d, want 2", len(cap.judges))
	}
	retries := judgeRetries(t, db)
	if len(retries) != 1 || retries[0]["result"] != "retry" || !strings.Contains(strings.Join(anyStrings(retries[0]["errors"]), " "), "sec-9") {
		t.Fatalf("judge output_retry: %v", retries)
	}
	if qs, _ := audienceQuestions(t, db); len(qs) != len(recordedDefense(t, good).AudienceQuestions)+len(recordedDefense(t, good).NotCovered) {
		t.Fatalf("audience questions: %d", len(qs))
	}
}

// Fichas §7: a judge that fails its shape twice stays visible and stops
// nothing: no audience question, no pending of the agent, and the
// section can still be signed.
func TestK1_AJudgeFailingTwiceStaysVisibleAndBlocksNothing(t *testing.T) {
	malformed := judgeTextFixture(t, `{"audience_questions":[]}`)
	db, runner, cap := runDefense(t, "", "conclusion", malformed)
	if len(cap.judges) != 2 {
		t.Fatalf("judge calls: %d, want 2", len(cap.judges))
	}
	retries := judgeRetries(t, db)
	if len(retries) != 2 || retries[1]["result"] != cycle.DefenseFailed {
		t.Fatalf("judge output_retry: %v", retries)
	}
	if qs, _ := audienceQuestions(t, db); len(qs) != 0 {
		t.Fatalf("audience questions: %d, want 0", len(qs))
	}
	if _, pendings := status(t, db, "sec-1"); len(pendings) != 0 {
		t.Fatalf("sec-1 pendings: %v, want none", pendings)
	}
	if clear, err := graph.SectionClear(context.Background(), db, "p1", "sec-1"); err != nil || !clear {
		t.Fatalf("SectionClear: %v %v", clear, err)
	}
	if open, err := runner.RequestSignoff(context.Background(), "p1", "sec-1"); err != nil || !open.Opened {
		t.Fatalf("RequestSignoff: %+v %v", open, err)
	}
}

func modelCallCount(t *testing.T, db *store.DB) int {
	t.Helper()
	var n int
	if err := db.Reader.QueryRowContext(context.Background(),
		`SELECT COUNT(*) FROM model_call WHERE project_id = 'p1'`).Scan(&n); err != nil {
		t.Fatalf("count model calls: %v", err)
	}
	return n
}

func judgeRetries(t *testing.T, db *store.DB) []map[string]any {
	t.Helper()
	var out []map[string]any
	for _, ev := range eventsOfType(t, db, store.EventOutputRetry) {
		data := eventData(t, ev)
		if data["turn_kind"] == "judge_defense" {
			if ev.AgentRole != "judge" {
				t.Fatalf("judge output_retry agent_role %q", ev.AgentRole)
			}
			out = append(out, data)
		}
	}
	return out
}

// Backlog-r2 2.6: the embedded judge schema is the design's, byte for
// byte.
func TestK1_EmbeddedJudgeDefenseSchemaIsTheDesignSchema(t *testing.T) {
	embedded, err := os.ReadFile(filepath.Join("schema", "judge_defense_output.schema.json"))
	if err != nil {
		t.Fatalf("read embedded schema: %v", err)
	}
	design, err := os.ReadFile(filepath.Join("..", "..", "docs", "diseno", "bloque-3", "salida-juez-defensa.schema.json"))
	if err != nil {
		t.Fatalf("read design schema: %v", err)
	}
	if string(embedded) != string(design) {
		t.Fatal("embedded schema differs from docs/diseno/bloque-3/salida-juez-defensa.schema.json")
	}
}
