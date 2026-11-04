package server_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/LaurenteEber/tupaqnin/internal/graph"
	"github.com/LaurenteEber/tupaqnin/internal/server"
	"github.com/LaurenteEber/tupaqnin/internal/store"
)

// Piece 4.3 (backlog-r4), the routes and what the page reads.

const objectionReason = "Porque el promedio esconde los valores extremos."

// objectionAPI: sec-1 not critical by role, with an agent's decision dec-1 and
// your decision dec-2, both used in it.
func objectionAPI(t *testing.T) (http.Handler, *store.DB, string, string) {
	t.Helper()
	h, db, base, projectID := proposalsAPI(t)
	ctx := context.Background()
	err := db.Write(ctx, projectID, func(tx *store.Tx) error {
		sec, err := tx.GetObject(ctx, projectID, "sec-1")
		if err != nil {
			return err
		}
		var b map[string]any
		_ = json.Unmarshal(sec.Body, &b)
		b["role"] = map[string]any{"value": "other", "state": "confirmed"}
		if _, err := tx.UpdateObject(sec.ID, sec.CurrentVersion, sec.State, b, store.EventInput{Type: store.EventRoleChanged, Actor: "human"}); err != nil {
			return err
		}
		for _, d := range []struct{ state, text string }{{"agent", "Cobrar por asiento."}, {"human", "Cobrar en dólares."}} {
			dec, err := tx.CreateObject(store.ObjectDecision, d.state, map[string]any{"statement": d.text, "instead_of": "Otra vía.",
				"would_choose_if": "Hubiera uso medido.", "origin": d.state}, store.EventInput{Type: store.EventStateChanged, Actor: d.state})
			if err != nil {
				return err
			}
			if _, err := tx.AddEdge(store.EdgeUsedIn, dec.ID, sec.ID, "", graph.UsageOf(dec), store.EventInput{Type: store.EventEdge, Actor: d.state}); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return h, db, base, projectID
}

func objectState(t *testing.T, db *store.DB, projectID, label string) string {
	t.Helper()
	obj, err := db.GetObject(context.Background(), projectID, label)
	if err != nil {
		t.Fatal(err)
	}
	return obj.State
}

// E-1, B-81: the route objects and answers with the question's label; GET /todo
// shows its two options; the answer route closes it with «mantener».
func TestE1_TheObjectRouteOpensTheQuestionTheTodoShows(t *testing.T) {
	h, db, base, projectID := objectionAPI(t)
	rec := do(t, h, http.MethodPost, base+"/decisions/dec-1/object", map[string]any{"reason": objectionReason})
	if rec.Code != http.StatusOK {
		t.Fatalf("object: %d %s", rec.Code, rec.Body)
	}
	if got := decode[server.ObjectDecisionResponse](t, rec); got.OpenQuestion != "oq-1" {
		t.Fatalf("response: %+v", got)
	}
	if st := objectState(t, db, projectID, "dec-1"); st != "in_open_question" {
		t.Fatalf("dec-1: %s", st)
	}
	todo := decode[server.TodoView](t, do(t, h, http.MethodGet, base+"/todo", nil))
	if len(todo.OpenQuestions) != 1 {
		t.Fatalf("todo: %+v", todo)
	}
	q := todo.OpenQuestions[0]
	if q.Question != "Cobrar por asiento." || q.ObjectionReason != objectionReason || len(q.Options) != 2 || !q.Options[0].Keep || q.Options[0].Option != "Cobrar por asiento." ||
		!q.Options[1].Other || q.Options[1].Option != "Otra vía." || q.Options[0].Contrast != "Hubiera uso medido." ||
		len(q.Sections) != 1 || len(q.Decisions) != 1 {
		t.Fatalf("question: %+v", q)
	}
	rec = do(t, h, http.MethodPost, base+"/open-questions/oq-1/answer", map[string]any{"option": 0, "reason": "Porque hay asientos claros."})
	if rec.Code != http.StatusOK {
		t.Fatalf("answer: %d", rec.Code)
	}
	if st := objectState(t, db, projectID, "dec-1"); st != "human" {
		t.Fatalf("dec-1: %s", st)
	}
}

// The routes' codes: what is refused changes nothing.
func TestE1_B82_TheRoutesRefuseWithTheirCodesAndChangeNothing(t *testing.T) {
	h, db, base, projectID := objectionAPI(t)
	before := eventsCount(t, db, projectID)
	for _, tc := range []struct {
		path, reason string
		code         int
		want         string
	}{
		{"/decisions/dec-1/revoke", objectionReason, http.StatusConflict, "decision_not_yours"},
		{"/decisions/dec-2/revoke", "ok", http.StatusUnprocessableEntity, "invalid_reason"},
		{"/decisions/dec-2/revoke", "", http.StatusUnprocessableEntity, "invalid_reason"},
		{"/decisions/dec-1/object", "sí", http.StatusUnprocessableEntity, "invalid_reason"},
		{"/decisions/sec-1/object", objectionReason, http.StatusNotFound, "decision_not_found"},
		{"/decisions/sec-1/revoke", objectionReason, http.StatusNotFound, "decision_not_found"},
	} {
		rec := do(t, h, http.MethodPost, base+tc.path, map[string]any{"reason": tc.reason})
		var body struct {
			Error string `json:"error"`
		}
		_ = json.Unmarshal(rec.Body.Bytes(), &body)
		if rec.Code != tc.code || body.Error != tc.want {
			t.Fatalf("%s %q: %d %s", tc.path, tc.reason, rec.Code, rec.Body)
		}
	}
	if n := eventsCount(t, db, projectID); n != before {
		t.Fatalf("events: %d, want %d", n, before)
	}
	if rec := do(t, h, http.MethodPost, base+"/decisions/dec-2/revoke", map[string]any{"reason": objectionReason}); rec.Code != http.StatusNoContent {
		t.Fatalf("revoke: %d", rec.Code)
	}
	if st := objectState(t, db, projectID, "dec-2"); st != "revoked" {
		t.Fatalf("dec-2: %s", st)
	}
	if rec := do(t, h, http.MethodPost, base+"/decisions/dec-2/object", map[string]any{"reason": objectionReason}); rec.Code != http.StatusConflict {
		t.Fatalf("objecting what is revoked: %d", rec.Code)
	}
}

// G-1, B-78 with a critical decision really signed: an objection answers false
// in EqualsSignature, and neither the open question nor keeping it lets an undo
// bring the section back.
func TestB78_AnObjectionKeepsTheSectionChangedAfterAnUndo(t *testing.T) {
	h, db, projectID := newSignoffAPI(t)
	seedSignSection(t, db, projectID, plainText, true)
	base := "/api/projects/" + projectID
	v := openSignoffRoute(t, h, base)
	if rec := do(t, h, http.MethodPost, base+"/signoffs/"+v.Label+"/explanation", map[string]any{"text": "Solo comparo estudios con el mismo tamaño, porque si no la diferencia sale del filtro."}); rec.Code != http.StatusAccepted {
		t.Fatalf("explanation: %d", rec.Code)
	}
	waitRounds(t, h, base, v.Label, 1)
	if rec := do(t, h, http.MethodPost, base+"/signoffs/"+v.Label+"/answers", map[string]any{"origin_answer": "Del supuesto de que todos cuentan desde el mismo tamaño.", "case_answer": ""}); rec.Code != http.StatusAccepted {
		t.Fatalf("answers: %d", rec.Code)
	}
	waitRounds(t, h, base, v.Label, 2)
	if rec := do(t, h, http.MethodPost, base+"/signoffs/"+v.Label+"/sign", nil); rec.Code != http.StatusOK {
		t.Fatalf("sign: %d %s", rec.Code, rec.Body.String())
	}
	saveOn(t, h, base, "sec-1", plainText+" Un añadido mío.")
	if rec := do(t, h, http.MethodPost, base+"/decisions/dec-1/object", map[string]any{"reason": objectionReason}); rec.Code != http.StatusOK {
		t.Fatalf("object: %d %s", rec.Code, rec.Body)
	}
	if got := undoRoute(t, h, base, "sec-1"); got["status"] != "changed_since_signoff" {
		t.Fatalf("undo with the question open: %v", got)
	}
	if st := objectState(t, db, projectID, "sec-1"); st != "changed_since_signoff" {
		t.Fatalf("sec-1: %s", st)
	}
	if rec := do(t, h, http.MethodPost, base+"/open-questions/oq-1/answer", map[string]any{"option": 0, "reason": "Porque sigue siendo la mejor regla."}); rec.Code != http.StatusOK {
		t.Fatalf("answer: %d", rec.Code)
	}
	ctx := context.Background()
	sec, err := db.GetObject(ctx, projectID, "sec-1")
	if err != nil {
		t.Fatal(err)
	}
	signed, ok, err := graph.CurrentSignoff(ctx, db, projectID, sec.ID)
	if err != nil || !ok {
		t.Fatalf("signature: %v %v", ok, err)
	}
	if same, err := graph.EqualsSignature(ctx, db, projectID, sec, signed); err != nil || same {
		t.Fatalf("EqualsSignature = %v, %v: the critical decision changed version", same, err)
	}
	saveOn(t, h, base, "sec-1", plainText+" Otro añadido mío.")
	if got := undoRoute(t, h, base, "sec-1"); got["status"] != "changed_since_signoff" {
		t.Fatalf("undo after keeping: %v", got)
	}
}

// The frame and the card say what the page may offer (Go computes, React
// presents), and the frame shows the «porque» you kept it with.
func TestE1_TheFrameSaysWhichActsEachDecisionHas(t *testing.T) {
	h, _, base, projectID := objectionAPI(t)
	for _, label := range []string{"dec-1", "dec-2"} {
		if rec := do(t, h, http.MethodPut, base+"/decisions/"+label+"/critical", map[string]any{"marked": true}); rec.Code != http.StatusNoContent {
			t.Fatalf("mark %s: %d", label, rec.Code)
		}
	}
	decisions := func() map[string]graph.FrameDecision {
		out := map[string]graph.FrameDecision{}
		for _, sec := range getFrame(t, h, projectID).Sections {
			for _, d := range sec.Decisions {
				out[d.Label] = d
			}
		}
		return out
	}
	got := decisions()
	if !got["dec-1"].CanObject || got["dec-1"].CanRevoke || !got["dec-2"].CanObject || !got["dec-2"].CanRevoke {
		t.Fatalf("acts: %+v", got)
	}
	todo := decode[server.TodoView](t, do(t, h, http.MethodGet, base+"/todo", nil))
	if len(todo.Cards) != 1 || todo.Cards[0].Decision != "dec-1" || !todo.Cards[0].CanObject || todo.Cards[0].CanRevoke {
		t.Fatalf("card: %+v", todo.Cards)
	}
	if rec := do(t, h, http.MethodPost, base+"/decisions/dec-1/object", map[string]any{"reason": objectionReason}); rec.Code != http.StatusOK {
		t.Fatalf("object: %d", rec.Code)
	}
	got = decisions()
	if got["dec-1"].CanObject || got["dec-1"].CanRevoke {
		t.Fatalf("acts while objected: %+v", got["dec-1"])
	}
	todo = decode[server.TodoView](t, do(t, h, http.MethodGet, base+"/todo", nil))
	if len(todo.Cards) != 0 || len(todo.OpenQuestions) != 1 || len(todo.OpenQuestions[0].Options[0].RiskReasons) == 0 {
		t.Fatalf("todo: %+v", todo)
	}
	reason := "Porque hay asientos claros."
	if rec := do(t, h, http.MethodPost, base+"/open-questions/oq-1/answer", map[string]any{"option": 0, "reason": reason}); rec.Code != http.StatusOK {
		t.Fatalf("answer: %d", rec.Code)
	}
	if d := decisions()["dec-1"]; d.Reason != reason || !d.CanRevoke {
		t.Fatalf("after keeping: %+v", d)
	}
}

// B-82, estados §1: in the section's view a figure of a revoked decision is not
// linked, and asking to sign lists it instead of a brief signoff.
func TestB82_TheSectionViewAndTheSignoffSeeTheFigureOfARevokedDecisionUnlinked(t *testing.T) {
	h, db, base, projectID := objectionAPI(t)
	sec, err := db.GetObject(context.Background(), projectID, "sec-1")
	if err != nil {
		t.Fatal(err)
	}
	step := fmt.Sprintf(`{"stretch":"build","triage_line":"Construir.","closing_risk":"none","note":"Paso.",`+
		`"new_text":[{"section_id":"sec-1","base_version":%d,"anchor":%q,"operation":"replace","claim_origin":"material","text":"Cierre con 7 casos."}],`+
		`"figure_links":[{"figure_anchor":"7 casos","decision_id":"dec-2"}]}`, sec.CurrentVersion, strings.Fields(proposalText)[0])
	if applied := stepOn(t, db, projectID, step); !applied.Applied {
		t.Fatalf("step: %v", applied.Errors)
	}
	if rec := do(t, h, http.MethodPost, base+"/decisions/dec-2/revoke", map[string]any{"reason": objectionReason}); rec.Code != http.StatusNoContent {
		t.Fatalf("revoke: %d %s", rec.Code, rec.Body)
	}
	if rec := do(t, h, http.MethodPost, base+"/changes/chg-1/close", map[string]any{"result": "reviewed_unchanged", "reason": objectionReason}); rec.Code >= 300 {
		t.Fatalf("close: %d %s", rec.Code, rec.Body)
	}
	detail := decode[server.SectionDetail](t, do(t, h, http.MethodGet, base+"/sections/sec-1", nil))
	if len(detail.Figures) != 1 || detail.Figures[0].Linked || detail.Figures[0].Target != "dec-2" {
		t.Fatalf("figures: %+v", detail.Figures)
	}
	rec := do(t, h, http.MethodPost, base+"/sections/sec-1/signoff", nil)
	if rec.Code == http.StatusCreated || !strings.Contains(rec.Body.String(), graph.PendingFigureWithoutDecision) {
		t.Fatalf("signoff: %d %s", rec.Code, rec.Body)
	}
}

// E-1, B-14: a real signoff in progress refuses signing while the objection is
// open; keeping signs, and «la otra» is refused by its notice.
func TestB14_ARealSignoffInProgressWaitsForTheObjection(t *testing.T) {
	for _, tc := range []struct {
		option   int
		signCode int
		state    string
	}{{0, http.StatusOK, "signed"}, {1, http.StatusConflict, "stale"}} {
		h, db, projectID := newSignoffAPI(t)
		seedSignSection(t, db, projectID, plainText, true)
		base := "/api/projects/" + projectID
		v := openSignoffRoute(t, h, base)
		if rec := do(t, h, http.MethodPost, base+"/signoffs/"+v.Label+"/explanation", map[string]any{"text": "Solo comparo estudios con el mismo tamaño, porque si no la diferencia sale del filtro."}); rec.Code != http.StatusAccepted {
			t.Fatalf("explanation: %d", rec.Code)
		}
		waitRounds(t, h, base, v.Label, 1)
		if rec := do(t, h, http.MethodPost, base+"/signoffs/"+v.Label+"/answers", map[string]any{"origin_answer": "Del supuesto de que todos cuentan desde el mismo tamaño.", "case_answer": ""}); rec.Code != http.StatusAccepted {
			t.Fatalf("answers: %d", rec.Code)
		}
		waitRounds(t, h, base, v.Label, 2)
		if rec := do(t, h, http.MethodPost, base+"/decisions/dec-1/object", map[string]any{"reason": objectionReason}); rec.Code != http.StatusOK {
			t.Fatalf("object: %d", rec.Code)
		}
		if s := getSignoff(t, h, base, v.Label); s.CanSign || s.Phase != graph.SignoffInProgress || len(s.Pendings) == 0 {
			t.Fatalf("with the objection: %+v", s)
		}
		if rec := do(t, h, http.MethodPost, base+"/signoffs/"+v.Label+"/sign", nil); rec.Code == http.StatusOK {
			t.Fatalf("signed with the objection open: %s", rec.Body)
		}
		if rec := do(t, h, http.MethodPost, base+"/open-questions/oq-1/answer", map[string]any{"option": tc.option, "reason": objectionReason}); rec.Code != http.StatusOK {
			t.Fatalf("answer: %d", rec.Code)
		}
		rec := do(t, h, http.MethodPost, base+"/signoffs/"+v.Label+"/sign", nil)
		if rec.Code != tc.signCode || objectState(t, db, projectID, "sec-1") != tc.state {
			t.Fatalf("option %d: sign %d %s, sec-1 %s", tc.option, rec.Code, rec.Body, objectState(t, db, projectID, "sec-1"))
		}
	}
}

// B-107 through the route: only an objection's question offers your own option, and
// each refusal has its code.
func TestB107_TheRouteAnswersYourOption(t *testing.T) {
	h, _, base, _ := objectionAPI(t)
	rec := do(t, h, http.MethodPost, base+"/decisions/dec-1/object", map[string]string{"reason": objectionReason})
	if rec.Code >= 300 {
		t.Fatalf("object: %d %s", rec.Code, rec.Body.String())
	}
	label := decode[server.ObjectDecisionResponse](t, rec).OpenQuestion
	if todo := decode[server.TodoView](t, do(t, h, http.MethodGet, base+"/todo", nil)); len(todo.OpenQuestions) != 1 || !todo.OpenQuestions[0].OffersOwn {
		t.Fatalf("the view: %+v", todo.OpenQuestions)
	}
	url := base + "/open-questions/" + label + "/answer"
	const statement = "Cobrar por uso medido."
	objected := " " + strings.ToUpper(objectedStatementOf(t, h, base, label)) + " "
	own := func(option *int, statement, reason string) (int, string) {
		rec := do(t, h, http.MethodPost, url, server.AnswerOpenQuestionRequest{Option: option, OwnStatement: &statement, Reason: reason})
		return rec.Code, rec.Body.String()
	}
	zero := 0
	for _, tc := range []struct {
		option            *int
		statement, reason string
		status            int
		code              string
	}{
		{nil, statement, "ok", http.StatusUnprocessableEntity, "invalid_reason"},
		{nil, " ", objectionReason, http.StatusUnprocessableEntity, "empty_statement"},
		{nil, "...", objectionReason, http.StatusUnprocessableEntity, "empty_statement"},
		{nil, "ok", objectionReason, http.StatusUnprocessableEntity, "empty_statement"},
		{nil, objected, objectionReason, http.StatusUnprocessableEntity, "same_statement"},
		{&zero, statement, objectionReason, http.StatusBadRequest, "invalid_option"},
	} {
		if code, body := own(tc.option, tc.statement, tc.reason); code != tc.status || !strings.Contains(body, tc.code) {
			t.Fatalf("%+v: %d %s", tc, code, body)
		}
	}
	code, body := own(nil, statement, objectionReason)
	if code != http.StatusOK || !strings.Contains(body, "new_decision") {
		t.Fatalf("answer: %d %s", code, body)
	}
}

// objectedStatementOf is the decision a «por aclarar» born from an objection names:
// its question is that decision (P-115).
func objectedStatementOf(t *testing.T, h http.Handler, base, label string) string {
	t.Helper()
	todo := decode[server.TodoView](t, do(t, h, http.MethodGet, base+"/todo", nil))
	for _, q := range todo.OpenQuestions {
		if q.Label == label {
			return q.Question
		}
	}
	t.Fatalf("no question %s in the todo", label)
	return ""
}

// An ordinary «por aclarar» does not offer it, and the route refuses it.
func TestB107_AnOrdinaryQuestionDoesNotOfferIt(t *testing.T) {
	h, db, base, projectID := objectionAPI(t)
	var label string
	if err := db.Write(context.Background(), projectID, func(tx *store.Tx) error {
		q, err := tx.CreateObject(store.ObjectOpenQuestion, "open", map[string]any{"question": "¿Cuál?",
			"options": []any{map[string]any{"option": "Una"}, map[string]any{"option": "Otra"}}},
			store.EventInput{Type: store.EventStateChanged, Actor: "agent"})
		if err != nil {
			return err
		}
		label = q.Label
		sec, err := tx.GetObject(context.Background(), projectID, "sec-1")
		if err != nil {
			return err
		}
		_, err = tx.AddEdge(store.EdgeBlocks, q.ID, sec.ID, "", nil, store.EventInput{Type: store.EventEdge, Actor: "agent"})
		return err
	}); err != nil {
		t.Fatal(err)
	}
	if todo := decode[server.TodoView](t, do(t, h, http.MethodGet, base+"/todo", nil)); len(todo.OpenQuestions) != 1 || todo.OpenQuestions[0].OffersOwn {
		t.Fatalf("the view: %+v", todo.OpenQuestions)
	}
	statement := "Otra cosa."
	rec := do(t, h, http.MethodPost, base+"/open-questions/"+label+"/answer", server.AnswerOpenQuestionRequest{OwnStatement: &statement, Reason: objectionReason})
	if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), "own_option_not_offered") {
		t.Fatalf("%d %s", rec.Code, rec.Body.String())
	}
}
