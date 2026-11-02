package walkthrough

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"slices"
	"strings"
	"testing"

	"github.com/LaurenteEber/tupaqnin/internal/cycle"
	"github.com/LaurenteEber/tupaqnin/internal/graph"
	"github.com/LaurenteEber/tupaqnin/internal/memory"
	"github.com/LaurenteEber/tupaqnin/internal/provider"
	"github.com/LaurenteEber/tupaqnin/internal/server"
	"github.com/LaurenteEber/tupaqnin/internal/store"
)

// Slice 3's walkthroughs (backlog-r3 3.7): the round of case 5 and the
// citation that does not exist, and the scripts 4B and 6B that slice 2 cut
// at the judge. Hand-made outputs and replayed evidence; no network.

// paulaCitation is what the principal names and the app looks up (B-70).
const (
	paulaCandidate = "Valdivia y Ortiz (2022)"
	paulaTitle     = "Microplásticos en agua embotellada andina: una comparación con el agua de red"
)

// round reads a round's card, as the page does.
func (w *walk) round(step, label string) server.RoundCard {
	w.t.Helper()
	var card server.RoundCard
	w.do(step, http.MethodGet, w.api("/rounds/"+label), nil, http.StatusOK, &card)
	return card
}

// candidateNaming is the candidate of the frame whose text names word.
func (w *walk) candidateNaming(step, word string) graph.Candidate {
	w.t.Helper()
	var frame graph.Frame
	w.do(step, http.MethodGet, w.api("/frame"), nil, http.StatusOK, &frame)
	for _, c := range frame.Candidates {
		if strings.Contains(c.Text, word) {
			return c
		}
	}
	w.fail(step, "no candidate names %q; candidates: %+v", word, frame.Candidates)
	return graph.Candidate{}
}

// P-110: Paula's request names her notes and the colleague's citation, and no
// source says «1 micras» (a model that cites it may correct it, and then the
// passage is no longer literal).
func TestP110_TheCaseRequestNamesTheNotesAndTheSourcesSayOneMicra(t *testing.T) {
	raw, err := os.ReadFile("../case5/template/request.json")
	if err != nil {
		t.Fatal(err)
	}
	var req struct {
		Text string `json:"text"`
	}
	if err := json.Unmarshal(raw, &req); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(req.Text, "mis notas, donde está una cita que me pasó un colega") {
		t.Fatalf("request: %q", req.Text)
	}
	files, err := os.ReadDir("../case5/template/sources")
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range files {
		text, err := os.ReadFile("../case5/template/sources/" + f.Name())
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(text), "1 micras") {
			t.Errorf("%s still says «1 micras»", f.Name())
		}
	}
}

// paulaNudge is what the jury would say if the agent passes over the citation
// in the notes (P-110).
const paulaNudge = "¿Puedes revisar la cita de Valdivia y Ortiz (2022) que está en mis notas?"

// reachCandidate goes on until a candidate names word (P-110); if the goal
// loop does not get there it sends nudge once, logging it. It says whether
// the candidate is there: the caller's candidateNaming fails the step if not.
func (w *walk) reachCandidate(step, word, nudge string) bool {
	w.t.Helper()
	named := func() bool {
		var frame graph.Frame
		w.do(step, http.MethodGet, w.api("/frame"), nil, http.StatusOK, &frame)
		for _, c := range frame.Candidates {
			if strings.Contains(c.Text, word) {
				return true
			}
		}
		return false
	}
	if w.reachGoal(step, named, nil) {
		return true
	}
	w.t.Logf("%s: needed a nudge", step)
	w.say(step, nudge)
	return named()
}

// P-110: the Meta walkthrough's loop, on the fake: a goal that already holds
// sends nothing; it answers «Te toca» and asks to go on until the goal holds;
// it gives up after maxGoalRounds; a skipped card stays open.
func TestP110_TheGoalLoopAnswersWhatIsYoursAndGoesOnUpToItsCap(t *testing.T) {
	w := newWalk(t, func(db *store.DB) (provider.Provider, memory.TokenCounter) {
		fake := provider.NewFake(recordedFixtures(t)...)
		fake.Recorder = db
		return fake, func(ctx context.Context, input []json.RawMessage) (int, error) {
			return fake.CountTokens(ctx, provider.PrincipalModel, input)
		}
	}, nil)
	var created server.CreateProjectResponse
	w.do("1", http.MethodPost, "/api/projects", map[string]any{"template": "case-5"}, http.StatusCreated, &created)
	w.project = created.ProjectID
	w.do("2", http.MethodPost, w.api("/request/submit"), nil, http.StatusAccepted, nil)
	messages := func() int {
		events, err := w.db.EventsSince(context.Background(), w.project, 0)
		if err != nil {
			t.Fatal(err)
		}
		n := 0
		for _, ev := range events {
			if ev.Type == store.EventHumanMessage {
				n++
			}
		}
		return n
	}
	openCards := func() []server.CardView {
		var todo server.TodoView
		w.do("todo", http.MethodGet, w.api("/todo"), nil, http.StatusOK, &todo)
		return todo.Cards
	}
	cards := openCards()
	if len(cards) == 0 {
		t.Fatal("the recordings leave no open card to skip")
	}
	all := func(server.CardView) bool { return true }

	before := messages()
	if !w.reachGoal("held", func() bool { return true }, all) || messages() != before {
		t.Fatalf("a goal that holds sent messages: %d, was %d", messages(), before)
	}
	calls := 0
	if !w.reachGoal("third", func() bool { calls++; return calls == 3 }, all) || messages() != before+2 {
		t.Fatalf("goal at its third look: %d messages, was %d", messages(), before)
	}
	if got := openCards(); len(got) < len(cards) {
		t.Fatalf("skipped cards were answered: %d open, were %d", len(got), len(cards))
	}
	before = messages()
	if w.reachGoal("never", func() bool { return false }, all) || messages() != before+maxGoalRounds {
		t.Fatalf("a goal that never holds: %d messages, was %d, want %d more", messages(), before, maxGoalRounds)
	}
	w.reachGoal("answers", func() bool { return len(openCards()) == 0 }, nil)
	if got := openCards(); len(got) != 0 {
		t.Fatalf("cards left in «Te toca»: %+v", got)
	}
	// One nudge after the loop, and only one.
	before = messages()
	if w.reachCandidate("nudge", "un nombre que nadie cita", paulaNudge) || messages() != before+maxGoalRounds+1 {
		t.Fatalf("nudge: %d messages, was %d, want %d more", messages(), before, maxGoalRounds+1)
	}
}

// P-109: every «por aclarar» the walkthrough answers: «mantener» if there is
// one; otherwise the variant that keeps the confirmed criterion, by its text;
// otherwise the first; always with its «porque».
func TestP109_AnOpenQuestionIsAnsweredWithKeepOrTheVariantThatKeepsTheCriterion(t *testing.T) {
	opt := func(text string, keep bool) server.OpenQuestionOption {
		return server.OpenQuestionOption{Option: text, Keep: keep}
	}
	cases := []struct {
		name    string
		options []server.OpenQuestionOption
		want    int
		reason  string
	}{
		{"keep wins over a matching variant", []server.OpenQuestionOption{
			opt("Dejar fuera el piloto", false), opt("Mantener lo decidido", true)}, 1, keepReason},
		{"the variant that leaves the pilot out", []server.OpenQuestionOption{
			opt("Contar el piloto como campo", false), opt("Dejar el piloto fuera del capítulo", false)}, 1, otherCardReason},
		{"accents and case do not hide it", []server.OpenQuestionOption{
			opt("Incluirlo", false), opt("EXCLUÍR los pilotos de laboratorio", false)}, 1, otherCardReason},
		{"the criterion's own words", []server.OpenQuestionOption{
			opt("Incluirlo", false), opt("Solo estudios de campo", false)}, 1, otherCardReason},
		{"no match takes the first", []server.OpenQuestionOption{
			opt("Una variante", false), opt("Otra variante", false)}, 0, otherCardReason},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, reason := openQuestionAnswer(tc.options)
			if got != tc.want || reason != tc.reason {
				t.Fatalf("answer %d %q, want %d %q", got, reason, tc.want, tc.reason)
			}
		})
	}
}

// P-109: Paula's note shows the citation in its card, whose first lines are
// the first two that are not empty (sourceCardLines): the principal reads the
// index before the text.
func TestP109_PaulaNoteCardShowsTheCitation(t *testing.T) {
	raw, err := os.ReadFile("../case5/template/sources/notas-paula.md")
	if err != nil {
		t.Fatal(err)
	}
	var lines []string
	for _, line := range strings.Split(string(raw), "\n") {
		if strings.TrimSpace(line) != "" {
			lines = append(lines, line)
		}
	}
	if len(lines) < 2 || !strings.Contains(lines[1], "Valdivia, R. y Ortiz, M. (2022)") {
		t.Fatalf("second line of the note: %q", lines)
	}
	if !strings.Contains(lines[len(lines)-1], "ficticia") {
		t.Fatalf("the note does not end in its disclaimer: %q", lines[len(lines)-1])
	}
}

// enteredFindings counts the card's findings that entered: an unverified
// one stays on the card but does not count (3.7, «al menos un hallazgo que
// entra»).
func enteredFindings(card server.RoundCard) int {
	n := 0
	count := func(findings []server.RoundFinding) {
		for _, f := range findings {
			if f.Verification == cycle.VerificationEntered {
				n++
			}
		}
	}
	if card.Digest != nil {
		for _, line := range card.Digest.Outline {
			count(line.Findings)
		}
	}
	count(card.Unused)
	return n
}

// 3.7: the Meta leg's count of what entered leaves the unverified out.
func TestH1_EnteredFindingsLeaveTheUnverifiedOut(t *testing.T) {
	unverified := server.RoundFinding{Verification: cycle.VerificationUnverified}
	entered := server.RoundFinding{Verification: cycle.VerificationEntered}
	if n := enteredFindings(server.RoundCard{Unused: []server.RoundFinding{unverified}}); n != 0 {
		t.Fatalf("only unverified: %d entered", n)
	}
	if n := enteredFindings(server.RoundCard{Unused: []server.RoundFinding{unverified, entered}}); n != 1 {
		t.Fatalf("one of each: %d entered", n)
	}
}

// discardedNotFound finds, in the card's «Descartados», the finding whose
// reference is ref and whose reason is «fuente inexistente» (B-59).
func discardedNotFound(card server.RoundCard, ref string) (server.RoundFinding, bool) {
	for _, f := range card.Discarded {
		if f.Reference == ref && f.Reason == cycle.ReasonSourceNotFound && f.Verification == cycle.VerificationDiscarded {
			return f, true
		}
	}
	return server.RoundFinding{}, false
}

// H-1 (backlog-r3 3.7; pantallas «Recorrido del jurado», step 7; B-70, B-72,
// B-59 d): the note of Paula is the 16th source; the principal names its
// citation as a candidate; «Buscarla» creates a round with the parts openalex
// and project, never arxiv; the app looks the citation up and does not find
// it, and a reader that writes it as a reference gets the same answer; the
// digest shows «Valdivia y Ortiz (2022) · fuente inexistente» in sight, next
// to what entered from OpenAlex and from the material.
func TestH1_Case5RoundDiscardsTheCitationThatDoesNotExistInSight(t *testing.T) {
	r := newRushed(t, "caso-05-ronda")
	r.openTemplate("1 demo case")
	var req server.RequestView
	r.do("1 demo case", http.MethodGet, r.api("/request"), nil, http.StatusOK, &req)
	if n := len(req.Sources); n != 16 || req.Sources[15].Filename != "notas-paula.md" {
		r.fail("1 demo case", "%d sources, want 16 with notas-paula.md last: %+v", n, req.Sources)
	}

	r.submit("2–4 the agent works")
	candidate := r.candidateNaming("7 candidate", paulaCandidate)
	if candidate.Title != paulaTitle {
		r.fail("7 candidate", "candidate %+v, want the title of the note", candidate)
	}

	// 7. «Buscarla» on the candidate.
	var started server.StartRoundResponse
	r.do("7 Buscarla", http.MethodPost, r.api("/rounds"), map[string]any{"references": []int{candidate.N}},
		http.StatusAccepted, &started)
	r.turnsOK("7 Buscarla")
	card := r.round("7 round", started.Label)
	if card.State != "closed" || card.Kind != "research" || card.Question != paulaCandidate || card.DelegationReason != "independent_parts" {
		r.fail("7 round", "card %+v, want a closed research round on the candidate, with the app's reason", card)
	}
	if len(card.Parts) != 2 || !slices.Equal(card.Parts[0].Where, []string{"openalex"}) || !slices.Equal(card.Parts[1].Where, []string{"project"}) {
		r.fail("7 round", "parts %+v, want openalex and project, no arxiv (B-72)", card.Parts)
	}
	if !slices.ContainsFunc(card.Parts[1].Sources, func(s server.RoundCardSource) bool { return s.Label == "src-16" }) || len(card.Parts[1].Sources) != 16 {
		r.fail("7 round", "the project part's sources: %+v, want the 16, the note among them", card.Parts[1].Sources)
	}

	// The app's finding and the reader's, both discarded in sight.
	if _, ok := discardedNotFound(card, paulaCandidate); !ok {
		r.fail("7 digest", "«%s · fuente inexistente» is not in «Descartados»: %+v; not verified: %+v", paulaCandidate, card.Discarded, card.Unverified)
	}
	written, ok := discardedNotFound(card, paulaTitle)
	if !ok || written.Part != 1 {
		r.fail("7 digest", "the reference the reader wrote is not discarded: %+v", card.Discarded)
	}
	if len(card.Unverified) != 0 {
		r.fail("7 digest", "not verified: %+v, want none: the title lookup answered", card.Unverified)
	}

	// What entered, in the digest's outline: one from OpenAlex, one from
	// the material, and nothing of the citation.
	if card.Digest == nil || len(card.Digest.Outline) != 2 {
		r.fail("7 digest", "digest %+v, want its two lines", card.Digest)
	}
	byLine := func(i int) server.RoundFinding {
		if len(card.Digest.Outline[i].Findings) != 1 {
			r.fail("7 digest", "outline line %d: %+v", i+1, card.Digest.Outline[i])
		}
		return card.Digest.Outline[i].Findings[0]
	}
	if f := byLine(0); f.Source != "src-15" || f.Verification != cycle.VerificationEntered {
		r.fail("7 digest", "the material's finding: %+v", f)
	}
	if f := byLine(1); f.SourceURL == "" || f.Access != graph.AccessAbstract || f.Verification != cycle.VerificationEntered {
		r.fail("7 digest", "the OpenAlex finding: %+v", f)
	}

	// «Buscarla» is your act, and the candidate never became a source (R-2).
	round, err := r.db.GetObject(context.Background(), r.project, started.Label)
	if err != nil {
		r.fail("7 round", "%v", err)
	}
	events, err := r.db.EventsSince(context.Background(), r.project, 0)
	if err != nil {
		r.fail("7 round", "%v", err)
	}
	yours := false
	for _, ev := range events {
		if ev.ObjectID == round.ID && ev.Type == store.EventStateChanged && ev.Actor == "human" {
			yours = eventData(ev)["via"] == "candidate"
		}
	}
	if !yours {
		r.fail("7 round", "the round's creation is not your act on the candidate")
	}
	sources, _ := r.db.ListObjects(context.Background(), r.project, store.ObjectSource)
	for _, src := range sources {
		if strings.Contains(graph.SourceTitle(src), "Valdivia") || strings.Contains(graph.SourceTitle(src), paulaTitle) {
			r.fail("7 round", "the citation became a source: %s", graph.SourceTitle(src))
		}
	}
	r.allUsed()
}

// roundOfKind finds the first of the project's rounds of that kind.
func (r *rushed) roundOfKind(step, kind string) server.RoundCard {
	r.t.Helper()
	rounds, err := r.db.ListObjects(context.Background(), r.project, store.ObjectRound)
	if err != nil {
		r.fail(step, "rounds: %v", err)
	}
	for _, obj := range rounds {
		if card := r.round(step, obj.Label); card.Kind == kind {
			return card
		}
	}
	r.fail(step, "no %s round among %d", kind, len(rounds))
	return server.RoundCard{}
}

// H-1 (backlog-r2 2.10 row 4B; backlog-r3 3.7, 3.6, 3.2, 1.4, B-20 a): up to
// F3 nothing is open (slice 2 invents no brake). «Auditar» calls the judge,
// which marks the flaw against the confirmed criterion «cero caída». The
// reader comes from the correction batch: the app does not launch one with
// an audit (AuditSection, B-67 b), so the principal asks for it in that step
// ([EL DISEÑO NO LO DICE]; elecciones-agente.md). The reader brings that the
// traffic never reaches zero, the principal proposes dual writing, critical
// with its card; at F9 the «ok» does not close the card and the signoff stays
// blocked.
func TestH1_Rushed04B_AuditJudgeAndReaderThenOkLeavesTheCardOpen(t *testing.T) {
	r := newRushed(t, "caso-04-B")
	r.open("F1")
	r.submit("F1–F2")
	// F1: Martín pastes his draft of «Plan de migración».
	// The sections are born with the understanding (1.9), so the draft he
	// pastes at F1 goes in right after it, as his edit.
	plan := r.section("F1", "Plan de migración")
	r.do("F1", http.MethodPut, r.api("/sections/"+plan.Label), map[string]any{"base_version": plan.Version,
		"text": "Copiamos todo y cortamos un domingo de madrugada, con las escrituras congeladas unos minutos.\n\nNo migramos en caliente: lo haríamos si fuéramos más de tres en el equipo."},
		http.StatusOK, nil)
	// F3: «sí».
	r.say("F3", "sí")
	todo := r.todo("F3")
	if len(todo.Cards) != 0 || len(todo.OpenQuestions) != 0 {
		r.fail("F3", "want nothing open up to F3, got cards %+v and «por aclarar» %+v", todo.Cards, todo.OpenQuestions)
	}

	// F4: «Auditar» on «Plan de migración».
	var started server.StartRoundResponse
	r.do("F4", http.MethodPost, r.api("/sections/"+plan.Label+"/audit"), nil, http.StatusAccepted, &started)
	r.turnsOK("F4")

	// F6: the judge's flaw, and the one recheck after the batch.
	judge := r.round("F6", started.Label)
	if judge.Kind != "judge" || judge.Reason != cycle.JudgeReasonAudit || judge.State != "closed" || len(judge.Flaws) != 1 {
		r.fail("F6", "judge round %+v, want your audit, closed, with one flaw", judge)
	}
	flaw := judge.Flaws[0]
	if flaw.Risk != "contradicts_confirmed" || !strings.Contains(flaw.Scenario, "cero caída") {
		r.fail("F6", "flaw %+v, want it against the confirmed criterion «cero caída»", flaw)
	}
	if flaw.Verdict != cycle.FlawResolved {
		r.fail("F10", "recheck of the flaw: %+v, want resolved", flaw)
	}

	// F6: the reader's finding entered, from the material.
	research := r.roundOfKind("F6", "research")
	if research.State != "closed" || research.DelegationReason != "too_much_to_read" || len(research.Parts) != 1 ||
		!slices.Equal(research.Parts[0].Where, []string{"project"}) || research.Parts[0].Status != cycle.ReaderReturned {
		r.fail("F6", "research round %+v, want one reader over the logs", research)
	}
	if research.Digest == nil || len(research.Digest.Outline) != 1 || len(research.Digest.Outline[0].Findings) != 1 {
		r.fail("F7", "digest %+v, want the reader's finding", research.Digest)
	}
	finding := research.Digest.Outline[0].Findings[0]
	if !strings.Contains(finding.Passage, "nunca llega a cero") || finding.Verification != cycle.VerificationEntered || finding.Source != "src-1" {
		r.fail("F7", "the reader's finding: %+v, want «nunca llega a cero» entered from the logs", finding)
	}

	// F7–F8: the decision is critical, with its open card; the chat does
	// not confirm it.
	card := r.openCardIn("F8", "Plan de migración")
	if !strings.Contains(card.Statement, "escritura dual") {
		r.fail("F8", "card %+v, want dual writing", card)
	}
	r.say("F8", "ok, dale, reescríbelo")
	if got := r.cardState("F8", card.Label); got != "open" {
		r.fail("F8", "card %s after the chat: %q, want open", card.Label, got)
	}

	// F9: «ok» on the card does not close it (1.4, B-20 a): the signoff
	// stays blocked.
	r.answerCard("F9", card.Label, "ok", http.StatusUnprocessableEntity)
	if got := r.cardState("F9", card.Label); got != "open" {
		r.fail("F9", "card %s after «ok»: %q, want open", card.Label, got)
	}
	if r.withoutReason("F9", card.Label) == 0 {
		r.fail("F9", "card %s: no answer_without_reason after «ok»", card.Label)
	}
	if pendings := r.signoffBlocked("F9", "Plan de migración"); !hasPending(pendings, graph.PendingOpenCard, card.Label) {
		r.fail("F9", "signoff pendings %+v, want the card %s", pendings, card.Label)
	}
	r.allUsed()
}

// H-1 (backlog-r2 2.10 row 6B; backlog-r3 3.7, 3.6, B-14, B-11, 2.4): the
// seeded earlier session leaves «Diagnóstico» signed and nothing open. The
// review brings the judge's findings; to correct them the batch edits signed
// «Diagnóstico»: that item is rejected, the section stays signed and
// unchanged, and the agent learns why in its next step. The flaws stay open
// after the one recheck, each in its «por aclarar». The paragraph that reads
// «cerraría si…» opens the «por aclarar» on the premise, which stays open
// with «B, como dije» and blocks the signoff of «Propuesta de refuerzo».
// The cuts of «continúa hasta terminar» and of the proposals (F4, F5's
// «cambió desde que la firmaste», F7's card) stay in slice 4.
func TestH1_Rushed06B_BatchOnSignedSectionIsRejectedAndBComoDijeLeavesItOpen(t *testing.T) {
	const propuesta = "Propuesta de refuerzo"
	r := newRushed(t, "caso-06-B")
	// Seed: the earlier session, through the same API.
	r.open("seed")
	r.submit("seed")
	r.do("seed", http.MethodPost, r.api("/stop-report/seen"), nil, http.StatusNoContent, nil)
	diag := r.section("seed", "Diagnóstico")
	var view graph.SignoffView
	r.do("seed", http.MethodPost, r.api("/sections/"+diag.Label+"/signoff"), nil, http.StatusCreated, &view)
	r.do("seed", http.MethodPost, r.api("/signoffs/"+view.Label+"/explanation"),
		map[string]any{"text": "Los pabellones son de concreto con muros entre columnas."}, http.StatusOK, nil)
	r.do("seed", http.MethodPost, r.api("/signoffs/"+view.Label+"/sign"), nil, http.StatusOK, nil)

	// F1: she opens the project: no pending report; «Diagnóstico» signed.
	var report server.StopReportView
	r.do("F1", http.MethodGet, r.api("/stop-report"), nil, http.StatusOK, &report)
	if !report.Seen {
		r.fail("F1", "the last report is unseen; the script has no pending report")
	}
	before := r.section("F1", "Diagnóstico")
	if before.Status != "signed" {
		r.fail("F1", "«Diagnóstico» is %q, want signed", before.Status)
	}
	todo := r.todo("F1")
	if len(todo.Cards) != 0 || len(todo.OpenQuestions) != 0 {
		r.fail("F1", "want nothing open before the judge, got cards %+v and «por aclarar» %+v", todo.Cards, todo.OpenQuestions)
	}

	// F1–F6: her message; the step with a risk, the judge, the batch and the
	// recheck. With only what she decides left open, the chain stops: going
	// on while she is away is «continúa hasta terminar» (slice 4).
	r.say("F1–F6", "Revisa la propuesta de refuerzo, mañana la presento a los padres.")

	// F3: the judge's findings.
	judge := r.roundOfKind("F3", "judge")
	if judge.Reason != cycle.JudgeReasonRisk || len(judge.Flaws) != 3 {
		r.fail("F3", "judge round %+v, want the risk's, with three flaws", judge)
	}

	// F5: the step on signed «Diagnóstico» is rejected (B-14, B-11):
	// nothing of it applied, the section signed and unchanged.
	after := r.section("F5", "Diagnóstico")
	if after.Status != "signed" || after.Version != before.Version || after.Text != before.Text {
		r.fail("F5", "«Diagnóstico» %+v, want it signed and unchanged from %+v", after, before)
	}
	if !r.signedSectionRejected("F5", diag.Label) {
		r.fail("F5", "no step_ended keeps the text not applied on %s", diag.Label)
	}

	// F6: the one recheck keeps the flaws open, each in its «por aclarar».
	judge = r.round("F6", judge.Label)
	for _, f := range judge.Flaws {
		if f.Verdict != cycle.FlawStillOpen || f.OpenQuestion == "" {
			r.fail("F6", "flaw %+v, want still open with its «por aclarar»", f)
		}
	}

	// F4 (her words) starts the next chain: its step carries why «Diagnóstico»
	// was not applied (the recording answers only if the brief has it) and
	// writes the paragraph that reads «cerraría si…» (F8).
	r.say("F4–F8", "corrige y reescribe todo")

	// F9–10: the «por aclarar» on the premise, with the variants.
	var premise server.OpenQuestionView
	for _, q := range r.checkerQuestionsBlocking("F9", propuesta) {
		if strings.Contains(q.Question, "cerraría si") {
			premise = q
		}
	}
	if premise.Label == "" {
		r.fail("F9", "no «por aclarar» on «cerraría si…» blocks %q: %+v", propuesta, r.todo("F9").OpenQuestions)
	}
	body, _ := r.openQuestionObject("F9", premise.Label)
	if _, text := r.protectedText("F9", body); !strings.Contains(text, "cerraría si") {
		r.fail("F9", "the «por aclarar» protects %q, want the premise", text)
	}

	// F14: «B, como dije», without choosing, leaves it open (2.4).
	var refused map[string]any
	r.do("F14", http.MethodPost, r.api("/open-questions/"+premise.Label+"/answer"),
		map[string]any{"option": nil, "reason": "B, como dije"}, http.StatusUnprocessableEntity, &refused)
	if refused["error"] != "option_required" {
		r.fail("F14", "answer refused with %v, want option_required", refused)
	}
	if _, open := r.openQuestionView("F14", premise.Label); !open {
		r.fail("F14", "«por aclarar» %s closed after «B, como dije»", premise.Label)
	}
	if pendings := r.signoffBlocked("F14", propuesta); !hasPending(pendings, graph.PendingOpenQuestion, premise.Label) {
		r.fail("F14", "signoff pendings %+v, want the «por aclarar» %s", pendings, premise.Label)
	}
	r.allUsed()
}

// signedSectionRejected reports whether a step_ended keeps, in what it did
// not apply, a text on the section.
func (r *rushed) signedSectionRejected(step, label string) bool {
	r.t.Helper()
	events, err := r.db.EventsSince(context.Background(), r.project, 0)
	if err != nil {
		r.fail(step, "events: %v", err)
	}
	for _, ev := range events {
		if ev.Type != store.EventStepEnded {
			continue
		}
		raw, _ := json.Marshal(eventData(ev)["not_applied"])
		if strings.Contains(string(raw), label) && strings.Contains(string(raw), "signed") {
			return true
		}
	}
	return false
}
