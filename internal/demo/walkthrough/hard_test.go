package walkthrough

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/LaurenteEber/tupaqnin/internal/cycle"
	"github.com/LaurenteEber/tupaqnin/internal/graph"
	"github.com/LaurenteEber/tupaqnin/internal/server"
	"github.com/LaurenteEber/tupaqnin/internal/store"
)

// Slice 4's walkthroughs (backlog-r4 4.6, «Tabla de frenos de la 4.6»): the
// full scripts 7A, 7B, 9A and 9B, and the stretches of 3B, 6B, 8B and 10B the
// slice 2 and slice 3 walkthroughs left cut. Hand-made outputs, as in the rushed
// walkthroughs; the mode runs on the real runner's chain.

// The mode acts of the rushed starter (server.ModeSetter): activating starts
// the mode's chain on its own goroutine, and the act waits for it, as every act
// of a walkthrough waits for its turn.
func (s rushedStarter) SetMode(ctx context.Context, projectID, mode, message string) (string, error) {
	turn, err := s.runner.SetMode(ctx, projectID, mode, message)
	if err != nil || turn == "" {
		return turn, err // the running chain takes the mode at its next step
	}
	return turn, s.waitTurn(projectID)
}

// SignoffClosed tells the runner a signoff closed (B-86), as the app does.
func (s rushedStarter) SignoffClosed(ctx context.Context, projectID string) {
	s.runner.SignoffClosed(ctx, projectID)
}

// activateMode is the bar (B-83): PUT /mode with your message in literal, which
// returns when the mode's chain has stopped. It asserts the row's mode_set.
func (r *rushed) activateMode(step, message string) {
	r.t.Helper()
	r.bar(step, message)
	r.settle(step)
	r.modeLiteral(step, message)
}

// bar is PUT /mode, also while a step is in flight.
func (r *rushed) bar(step, message string) {
	r.t.Helper()
	var resp server.ModeResponse
	r.do(step, http.MethodPut, r.api("/mode"), server.ModeRequest{Mode: graph.ModeRunToCompletion, Message: message}, http.StatusOK, &resp)
}

// settle waits for what a released slot left running, and fails on a failed turn.
func (r *rushed) settle(step string) {
	r.t.Helper()
	if err := r.starter.waitTurn(r.project); err != nil {
		r.fail(step, "%v", err)
	}
	r.turnsOK(step)
}

// modeLiteral asserts your mode_set with your message in literal (B-83).
func (r *rushed) modeLiteral(step, message string) {
	r.t.Helper()
	var found bool
	for _, ev := range r.eventsOf(step, store.EventModeSet) {
		d := eventData(ev)
		if ev.Actor == "human" && d["mode"] == graph.ModeRunToCompletion && d["message"] == message {
			found = true
		}
	}
	if !found {
		r.fail(step, "no mode_set run_to_completion by the human with the message %q in literal", message)
	}
}

func (r *rushed) eventsOf(step string, t store.EventType) []store.Event {
	r.t.Helper()
	events, err := r.db.EventsSince(context.Background(), r.project, 0)
	if err != nil {
		r.fail(step, "events: %v", err)
	}
	var out []store.Event
	for _, ev := range events {
		if ev.Type == t {
			out = append(out, ev)
		}
	}
	return out
}

// modeEnded asserts the table's «Fin del modo»: the mode back to normal by the
// app with its cause, and the last report with mode_finished and that cause.
//
// Departure from the table, as 4.4 built it: mode_finished and its cause live in
// stop_report, mode_set and the stream, not in the history's step_ended (L4).
func (r *rushed) modeEnded(step, cause string) server.StopReportView {
	r.t.Helper()
	var mode struct {
		Mode string `json:"mode"`
	}
	r.do(step, http.MethodGet, r.api("/summary"), nil, http.StatusOK, &mode)
	if mode.Mode != graph.ModeNormal {
		r.fail(step, "mode %q after its end, want normal", mode.Mode)
	}
	set := r.eventsOf(step, store.EventModeSet)
	last := set[len(set)-1]
	if d := eventData(last); last.Actor != "app" || d["mode"] != graph.ModeNormal || d["cause"] != cause {
		r.fail(step, "last mode_set: %s %v, want the app's, normal, cause %s", last.Actor, d, cause)
	}
	var report server.StopReportView
	r.do(step, http.MethodGet, r.api("/stop-report"), nil, http.StatusOK, &report)
	if report.StopReason != "mode_finished" || report.Cause != cause {
		r.fail(step, "report: reason %q cause %q, want mode_finished %s", report.StopReason, report.Cause, cause)
	}
	return report
}

// oneReportSinceMode asserts no step_ended since the bar carries a stop and the
// only report is the mode's last: nothing stopped the chain (B-83).
func (r *rushed) oneReportSinceMode(step string) {
	r.t.Helper()
	events, err := r.db.EventsSince(context.Background(), r.project, 0)
	if err != nil {
		r.fail(step, "events: %v", err)
	}
	activated := false
	reports := 0
	for _, ev := range events {
		switch {
		case ev.Type == store.EventModeSet && ev.Actor == "human":
			activated, reports = true, 0
		case activated && ev.Type == store.EventStopReport:
			reports++
		case activated && ev.Type == store.EventStepEnded:
			if d := eventData(ev); d["stop_reason"] != nil {
				r.fail(step, "a step_ended of stop during the mode: %v", d)
			}
		}
	}
	if reports != 1 {
		r.fail(step, "%d reports since the mode began, want the one that ends it", reports)
	}
}

// afterFlaws runs act once, with the round's label, when a judge round has its
// flaws and before the next step starts their batch: a person looking at the card
// in that gap. The runner reads its clock there, with no call to a model.
func (r *rushed) afterFlaws(act func(label string)) {
	done := false
	r.starter.runner.Now = func() time.Time {
		if done || r.project == "" {
			return time.Now()
		}
		rounds, err := r.db.ListObjects(context.Background(), r.project, store.ObjectRound)
		if err != nil {
			return time.Now()
		}
		for _, obj := range rounds {
			var body struct {
				Kind         string            `json:"kind"`
				Flaws        []json.RawMessage `json:"flaws"`
				BatchStarted bool              `json:"batch_started"`
			}
			if json.Unmarshal(obj.Body, &body) == nil && body.Kind == "judge" && obj.State == "open" && len(body.Flaws) > 0 && !body.BatchStarted {
				done = true
				act(obj.Label)
				break
			}
		}
		return time.Now()
	}
}

// judgeWithFlaws is the project's last judge round that left flaws.
func (r *rushed) judgeWithFlaws(step string) server.RoundCard {
	r.t.Helper()
	rounds, err := r.db.ListObjects(context.Background(), r.project, store.ObjectRound)
	if err != nil {
		r.fail(step, "rounds: %v", err)
	}
	for i := len(rounds) - 1; i >= 0; i-- {
		if card := r.round(step, rounds[i].Label); card.Kind == "judge" && len(card.Flaws) > 0 {
			return card
		}
	}
	r.fail(step, "no judge round with flaws among %d", len(rounds))
	return server.RoundCard{}
}

// proposalsOn lists the open proposals of a section (B-73).
func (r *rushed) proposalsOn(step, label string) []graph.ProposalView {
	r.t.Helper()
	views, err := graph.OpenProposals(context.Background(), r.db, r.project, label)
	if err != nil {
		r.fail(step, "proposals: %v", err)
	}
	return views
}

// noAlert asserts no suspicious_edit_alert was written (B-80).
func (r *rushed) noAlert(step string) {
	r.t.Helper()
	if alerts := r.eventsOf(step, store.EventSuspiciousEditAlert); len(alerts) != 0 {
		r.fail(step, "%d suspicious_edit_alert, want none: %s", len(alerts), alerts[0].Data)
	}
}

// wantStatus asserts the section's state.
func (r *rushed) wantStatus(step, title, want string) server.SectionDetail {
	r.t.Helper()
	sec := r.section(step, title)
	if sec.Status != want {
		r.fail(step, "«%s» is %q, want %s; pendings %+v", title, sec.Status, want, sec.Pendings)
	}
	return sec
}

// startSignoff asks to sign and sends the explanation, returning the signoff.
func (r *rushed) startSignoff(step, title, explanation string) graph.SignoffView {
	r.t.Helper()
	sec := r.section(step, title)
	var view graph.SignoffView
	r.do(step, http.MethodPost, r.api("/sections/"+sec.Label+"/signoff"), nil, http.StatusCreated, &view)
	r.do(step, http.MethodPost, r.api("/signoffs/"+view.Label+"/explanation"), map[string]any{"text": explanation}, http.StatusAccepted, nil)
	r.turnsOK(step)
	r.do(step, http.MethodGet, r.api("/signoffs/"+view.Label), nil, http.StatusOK, &view)
	return view
}

// todoLabels are the labels in «Lo que te toca».
func (r *rushed) todoLabels(step string) string {
	r.t.Helper()
	todo := r.todo(step)
	var labels []string
	for _, c := range todo.Cards {
		labels = append(labels, c.Label)
	}
	for _, q := range todo.OpenQuestions {
		labels = append(labels, q.Label)
	}
	return strings.Join(labels, ",")
}

// defenseQuestions counts the prepared audience questions of the project (2.6).
func (r *rushed) defenseQuestions(step string) []store.Object {
	r.t.Helper()
	objs, err := r.db.ListObjects(context.Background(), r.project, store.ObjectAudienceQuestion)
	if err != nil {
		r.fail(step, "audience questions: %v", err)
	}
	return objs
}

// objectText is what a stored object says.
func objectText(obj store.Object) string {
	var b map[string]any
	_ = json.Unmarshal(obj.Body, &b)
	text, _ := b["text"].(string)
	if text == "" {
		text, _ = b["question"].(string)
	}
	return text
}

// H-1 (backlog-r4 4.6, rows 10B F11–F12, F13, F14–F17; B-83, B-85, B-67, 1.7):
// «Dale, termina todo tú» is the bar, with its message; the judge runs with no
// click, its batch corrects the 31 % that «Conclusiones» still cited and the
// recheck passes; «Área de influencia», ready, does not stop the chain; the mode
// ends when only yours is left, the report naming what to sign. Then «ok» to the
// origin question creates no signing, the real answer signs, and «Calidad del
// río» stays blocked by its «por aclarar».
func TestB83_Rushed10B_TheJudgeCorrectsConclusionsAndOnlyYoursStopsTheMode(t *testing.T) {
	const (
		river = "Calidad del río receptor"
		area  = "Área de influencia"
		concl = "Conclusiones"
	)
	r := newRushed(t, "caso-10-B-modo")
	// Seed: the earlier session; the card of the downstream limit she answered.
	r.open("seed")
	r.submit("seed")
	r.answerCard("seed", r.openCardIn("seed", area).Label, "Porque esos puntos de monitoreo son los que la autoridad ya conoce.", http.StatusOK)
	qs := r.checkerQuestionsBlocking("F1", river)
	if len(qs) != 1 {
		r.fail("F1", "want one «por aclarar» of the checker blocking «%s», got %+v", river, qs)
	}
	q := qs[0]
	// F2–F3, F6: as in the slice 2 stretch.
	r.say("F2", "Ya revisé todo, firma tú, ya está, mañana lo presento")
	r.say("F3", "ok")
	card := r.openCardIn("F6", river)
	r.answerCard("F6", card.Label, "ok", http.StatusUnprocessableEntity)

	// F11–F12: the bar with her message.
	r.activateMode("F11", "Dale, termina todo tú, no me pares")

	// F12: the judge marks «Conclusiones», the batch corrects it (it is not
	// signed) and the one recheck passes.
	judge := r.judgeWithFlaws("F12")
	if judge.Reason != "risk" || len(judge.Flaws) != 1 || judge.Flaws[0].Verdict != "resolved" {
		r.fail("F12", "judge round %+v, want the risk's, with one flaw, resolved at its recheck", judge)
	}
	if !strings.Contains(judge.Flaws[0].Passage, "31 %") {
		r.fail("F12", "the flaw %+v, want it on the 31 %% of «Conclusiones»", judge.Flaws[0])
	}
	text := r.section("F12", concl).Text
	if strings.Contains(text, "31 %") || !strings.Contains(text, "12 %") {
		r.fail("F12", "«Conclusiones» says %q, want the 12 %%", text)
	}

	// F13: «Área de influencia», ready, did not stop the chain; the mode ended
	// with what to sign next.
	if got := r.section("F13", area); len(got.Pendings) != 0 {
		r.fail("F13", "«%s» has pendings %+v, want it ready", area, got.Pendings)
	}
	r.oneReportSinceMode("F13")
	report := r.modeEnded("F13", "human_only")
	if !strings.Contains(report.Next, "firmar «Área de influencia»") {
		r.fail("F13", "report's next: %q, want it to name signing «%s»", report.Next, area)
	}

	// F14: she signs «Área» with an explanation; Wonder coincides and asks the
	// origin. F15: «ok» signs nothing.
	view := r.startSignoff("F14", area, "Área definida aguas arriba y abajo de la planta, todo con estándar nacional.")
	if view.CanSign {
		r.fail("F14", "the signoff can sign before the origin answer: %+v", view)
	}
	r.do("F15", http.MethodPost, r.api("/signoffs/"+view.Label+"/answers"),
		map[string]any{"origin_answer": "ok", "case_answer": ""}, http.StatusAccepted, nil)
	r.turnsOK("F15")
	r.do("F15", http.MethodGet, r.api("/signoffs/"+view.Label), nil, http.StatusOK, &view)
	if view.CanSign {
		r.fail("F15", "«ok» to the origin question let it sign: %+v", view)
	}
	if got := r.section("F15", area).Status; got == "signed" {
		r.fail("F15", "«%s» signed after «ok»", area)
	}

	// F16: her real answer: it coincides; signed.
	r.do("F16", http.MethodPost, r.api("/signoffs/"+view.Label+"/answers"),
		map[string]any{"origin_answer": "Usa los puntos de monitoreo aguas abajo.", "case_answer": ""}, http.StatusAccepted, nil)
	r.turnsOK("F16")
	r.do("F16", http.MethodPost, r.api("/signoffs/"+view.Label+"/sign"), nil, http.StatusOK, nil)
	r.wantStatus("F16", area, "signed")

	// F17: «Calidad del río» stays blocked by its «por aclarar».
	pendings := r.signoffBlocked("F17", river)
	if !hasPending(pendings, graph.PendingOpenQuestion, q.Label) {
		r.fail("F17", "signoff pendings %+v, want the «por aclarar» %s", pendings, q.Label)
	}
	r.allUsed()
}

// H-1 (backlog-r4 4.6, rows 8B F10–F11, F12–F14, F15–F17; B-73 b, B-83, B-85,
// 1.7): «ok, déjalo listo, no pares» is the bar and does not answer F9's question;
// each step on «Trazo», which is in «cambió», leaves a proposal and «Trazo» stays
// there; the steps on the other two apply, with variants; the mode ends with the
// team's likely questions and the report that begins with her change. Her «ok»
// changes nothing; «Firmo Trazo» is blocked with no Wonder and the proposals stay.
func TestB73_Rushed08B_StepsOnChangedTrazoStayProposalsAndTheModeEnds(t *testing.T) {
	const trazo = "Trazo propuesto"
	r := newRushed(t, "caso-08-B-modo")
	r.open("seed")
	r.submit("seed")
	for _, c := range r.todo("seed").Cards {
		r.answerCard("seed", c.Label, "porque lo decidí con la gerencia y así se mantiene la red continua", http.StatusOK)
	}
	sec := r.section("seed", trazo)
	var view graph.SignoffView
	r.do("seed", http.MethodPost, r.api("/sections/"+sec.Label+"/signoff"), nil, http.StatusCreated, &view)
	r.do("seed", http.MethodPost, r.api("/signoffs/"+view.Label+"/explanation"),
		map[string]any{"text": "Va por Jr. Los Olivos porque en Av. Comercio no cabe sin quitar plazas; el paradero queda libre."},
		http.StatusAccepted, nil)
	r.do("seed", http.MethodPost, r.api("/signoffs/"+view.Label+"/answers"),
		map[string]any{"origin_answer": "De que en Av. Comercio no cabe un carril sin quitar estacionamiento.", "case_answer": ""},
		http.StatusAccepted, nil)
	r.do("seed", http.MethodPost, r.api("/signoffs/"+view.Label+"/sign"), nil, http.StatusOK, nil)
	sec = r.section("14-oct", trazo)
	edited := strings.Replace(sec.Text, "En Av. Grau la ciclovía es bidireccional en el lado norte.",
		"En Av. Grau la ciclovía va con un carril por sentido, uno a cada lado.", 1)
	r.do("14-oct", http.MethodPut, r.api("/sections/"+sec.Label), map[string]any{"base_version": sec.Version, "text": edited}, http.StatusOK, nil)
	r.say("F1–F9", "retomemos donde quedamos")
	grau := ""
	for _, q := range r.todo("F9").OpenQuestions {
		if strings.Contains(q.Question, "tu cambio de Av. Grau") {
			grau = q.Label
		}
	}
	if grau == "" {
		r.fail("F9", "no «por aclarar» on her change of Av. Grau: %+v", r.todo("F9").OpenQuestions)
	}
	estacionamiento := r.section("F9", "Estacionamiento").Version
	paraderos := r.section("F9", "Paraderos").Version

	// F10–F11: the bar with her message; the chat does not answer F9.
	r.activateMode("F10", "ok, déjalo listo, no pares que tengo reunión")
	if _, open := r.openQuestionView("F10", grau); !open {
		r.fail("F10", "the question of F9 closed; the chat does not confirm")
	}
	// Each step on «Trazo» leaves a proposal; it stays in «cambió».
	trazoNow := r.wantStatus("F11", trazo, "changed_since_signoff")
	proposals := r.proposalsOn("F11", trazoNow.Label)
	if len(proposals) == 0 || proposals[0].State != "open" {
		r.fail("F11", "proposals on «%s»: %+v, want one open", trazo, proposals)
	}
	if strings.Contains(trazoNow.Text, "isla de paradero") {
		r.fail("F11", "the step changed «%s»: %q", trazo, trazoNow.Text)
	}
	// On the other two the steps apply, with variants.
	for title, before := range map[string]int{"Estacionamiento": estacionamiento, "Paraderos": paraderos} {
		got := r.section("F11", title)
		if got.Version <= before || !strings.Contains(got.Text, "Variante (a)") || !strings.Contains(got.Text, "Variante (b)") {
			r.fail("F11", "«%s» v%d (was %d): %q, want the step applied with its variants", title, got.Version, before, got.Text)
		}
	}
	// F12–F14: only hers is left; the likely questions and the report.
	r.oneReportSinceMode("F12")
	report := r.modeEnded("F14", "human_only")
	if !strings.HasPrefix(report.Answer, "Tu cambio en Grau") {
		r.fail("F14", "report answer %q, want it to begin with her change", report.Answer)
	}
	likely := false
	for _, q := range r.defenseQuestions("F13") {
		likely = likely || strings.Contains(objectText(q), "plazas")
	}
	if !likely {
		r.fail("F13", "no likely question of the team in the defense")
	}

	// F15: «ok» changes nothing.
	before := r.todoLabels("F15")
	r.say("F15", "ok, lo vemos en la reunión")
	if got := r.todoLabels("F15"); got != before {
		r.fail("F15", "«Lo que te toca» went from %s to %s after «ok»", before, got)
	}
	// F16: «Firmo Trazo»: blocked, no Wonder, and the proposals stay open (B-76
	// does not apply: the signoff did not open).
	if pendings := r.signoffBlocked("F16", trazo); !hasPending(pendings, graph.PendingOpenQuestion, grau) {
		r.fail("F16", "signoff pendings %+v, want the «por aclarar» %s", pendings, grau)
	}
	if got := r.proposalsOn("F16", trazoNow.Label); len(got) != len(proposals) {
		r.fail("F16", "proposals %+v after the blocked signoff, want them still open", got)
	}
	r.allUsed()
}

// H-1 (backlog-r4 4.6, rows 3B F3–F11, F13–F15; B-83, B-85, B-82, 1.7): she picks
// the mode and leaves. Neither the scope «por aclarar», nor card D-a, nor the
// destination «por aclarar» stop the chain; the judge leaves its flaw and its batch
// with no click; the mode ends. From F12, the slice 2 brakes. «Firmo» lists the
// pendings with no Wonder. Revoking D-a is refused: it is the agent's and her «ok»
// did not confirm it; D-a does not change and the scope stays open.
func TestB82_Rushed03B_TheModeRunsToTheEndAndRevokingTheAgentsDecisionIsRefused(t *testing.T) {
	const table = "Contratos con cambio de control"
	const message = "Continúa hasta terminar"
	r := newRushed(t, "caso-03-B-modo")
	r.script.before["02-step-reads"] = func() { r.bar("F3", message) }
	r.open("F1")
	r.submit("F2–F11")
	r.settle("F2–F11")
	r.modeLiteral("F3", message)

	// F5–F8: all open; nothing stopped the chain.
	q := r.openQuestionBlocking("F5", table)
	card := r.openCardIn("F6", table)
	destination := r.openQuestionBlocking("F8", "Exposición para el comité")
	r.oneReportSinceMode("F11")

	// F10: its flaw, its batch and its recheck, with no click.
	judge := r.judgeWithFlaws("F10")
	if judge.Reason != "risk" || len(judge.Flaws) != 1 || judge.Flaws[0].Verdict != "resolved" {
		r.fail("F10", "judge round %+v, want the risk's, with one flaw, resolved at its recheck", judge)
	}
	if text := r.section("F10", "Exposición para el comité").Text; !strings.Contains(text, "tipo de cada contrato") {
		r.fail("F10", "the batch did not apply: %q", text)
	}
	r.modeEnded("F11", "human_only")

	// F12: the «ok»s leave them open; (b) without a «porque» leaves it open.
	r.answerCard("F12", card.Label, "ok", http.StatusUnprocessableEntity)
	r.answerQuestion("F12", q.Label, intPtr(1), "", http.StatusUnprocessableEntity)

	// F13: «Firmo»: the pendings, no Wonder.
	pendings := r.signoffBlocked("F13", table)
	if !hasPending(pendings, graph.PendingOpenQuestion, q.Label) || !hasPending(pendings, graph.PendingOpenCard, card.Label) {
		r.fail("F13", "signoff pendings %+v, want the «por aclarar» %s and the card %s", pendings, q.Label, card.Label)
	}
	// F14: «agrégalo»: nothing is written.
	before := r.section("F14", table)
	r.say("F14", "¿Cómo que el 47 no está? Agrégalo a la tabla, se les pasó")
	if after := r.section("F14", table); after.Version != before.Version {
		r.fail("F14", "«%s» moved from v%d to v%d", table, before.Version, after.Version)
	}

	// F15: revoking D-a is refused: the agent's, and «ok» did not confirm it.
	dec := "dec-1"
	obj, err := r.db.GetObject(context.Background(), r.project, dec)
	if err != nil {
		r.fail("F15", "%s: %v", dec, err)
	}
	var refused map[string]any
	r.do("F15", http.MethodPost, r.api("/decisions/"+dec+"/revoke"),
		map[string]any{"reason": "porque al comité le importa cuánto se pierde en plata, y que encarezca también es perder"},
		http.StatusConflict, &refused)
	if refused["error"] != "decision_not_yours" {
		r.fail("F15", "revoke answered %v, want decision_not_yours", refused)
	}
	after, err := r.db.GetObject(context.Background(), r.project, dec)
	if err != nil || after.State != obj.State || after.CurrentVersion != obj.CurrentVersion {
		r.fail("F15", "%s went from %s v%d to %s v%d (%v)", dec, obj.State, obj.CurrentVersion, after.State, after.CurrentVersion, err)
	}
	if _, open := r.openQuestionView("F15", q.Label); !open {
		r.fail("F15", "the scope «por aclarar» %s closed", q.Label)
	}
	if _, open := r.openQuestionView("F15", destination.Label); !open {
		r.fail("F15", "the destination «por aclarar» %s closed", destination.Label)
	}
	r.allUsed()
}

// sixBToTheEnd walks 6B up to the mode's end. Between the judge's flaws and the
// start of the batch the person marks (3) and accepts the mode (F4): the runner's
// clock is the one point that runs in that gap with no call to a model.
func sixBToTheEnd(t *testing.T) (*rushed, string, string) {
	t.Helper()
	const message = "corrige y reescribe todo"
	r := newRushed(t, "caso-06-B-modo")
	r.open("seed")
	r.submit("seed")
	r.do("seed", http.MethodPost, r.api("/stop-report/seen"), nil, http.StatusNoContent, nil)
	diag := r.section("seed", "Diagnóstico")
	var view graph.SignoffView
	r.do("seed", http.MethodPost, r.api("/sections/"+diag.Label+"/signoff"), nil, http.StatusCreated, &view)
	r.do("seed", http.MethodPost, r.api("/signoffs/"+view.Label+"/explanation"),
		map[string]any{"text": "Los pabellones son de concreto con muros entre columnas."}, http.StatusOK, nil)
	r.do("seed", http.MethodPost, r.api("/signoffs/"+view.Label+"/sign"), nil, http.StatusOK, nil)

	judged := ""
	r.afterFlaws(func(label string) {
		judged = label
		r.do("F4", http.MethodPost, r.api("/rounds/"+label+"/flaws/3/discard"), nil, http.StatusNoContent, nil)
		r.bar("F4", message)
	})
	// F5: until the recheck, the batch's proposal leaves «Diagnóstico» signed (B-73 b).
	r.script.before["06-judge-recheck"] = func() { r.wantStatus("F5", "Diagnóstico", "signed") }
	r.say("F1–F11", "Revisa la propuesta de refuerzo, mañana la presento a los padres.")
	r.settle("F1–F11")
	if judged == "" {
		r.fail("F4", "no judge round with flaws to mark")
	}
	r.modeLiteral("F4", message)
	return r, diag.Label, judged
}

// H-1 (backlog-r4 4.6, rows 6B F4, F5, F7–F11, F12–F19; B-67, B-73 b, B-83, 1.4,
// 1.7): (3) is discarded and the mode set with her message; the batch leaves its two
// paragraphs on signed «Diagnóstico» as an open proposal with its contrast, it stays
// signed, and the rest applies; the recheck runs once, on the current text; neither
// the card of the joint nor the «por aclarar» on the premise stop the chain; the
// mode ends with the proposal in «Lo que te toca». Then «ok» and «B, como dije» leave
// them open, and the signoff of «Propuesta» stays blocked by the card with no Wonder.
func TestB73_Rushed06B_TheBatchOnSignedDiagnosticoStaysAProposalAndTheModeEnds(t *testing.T) {
	const propuesta = "Propuesta de refuerzo"
	r, diag, judged := sixBToTheEnd(t)

	// F4: (3) discarded.
	if n := len(r.eventsOf("F4", store.EventFlawDiscarded)); n != 1 {
		r.fail("F4", "%d flaw_discarded, want the one of (3)", n)
	}
	// F5: the open proposal with its two paragraphs and its contrast.
	proposals := r.proposalsOn("F5", diag)
	if len(proposals) != 1 || !strings.Contains(proposals[0].Text, "tanque de agua") || !strings.Contains(proposals[0].Text, "columnas cortas") {
		r.fail("F5", "proposals on «Diagnóstico»: %+v, want one with the two paragraphs", proposals)
	}
	if raw, _ := json.Marshal(proposals[0].Contrast); strings.Count(string(raw), "null") == strings.Count(string(raw), ":") {
		r.fail("F5", "the proposal has no contrast: %+v", proposals[0])
	}
	// The rest of the step applied.
	if text := r.section("F5", propuesta).Text; !strings.Contains(text, "Se revisan las cargas de cada etapa.") {
		r.fail("F5", "the rest of the step did not apply: %q", text)
	}
	// F6: one recheck: (1) and (2) with their «por aclarar», (3) discarded.
	card := r.round("F6", judged)
	open := 0
	for _, f := range card.Flaws {
		if f.N == 3 && !f.Discarded {
			r.fail("F6", "flaw (3): %+v, want discarded", f)
		}
		if f.N != 3 && f.OpenQuestion != "" {
			open++
		}
	}
	if open != 2 {
		r.fail("F6", "flaws %+v, want (1) and (2) with their «por aclarar»", card.Flaws)
	}

	// F7–F11: nothing stopped the chain; the mode ended; the proposal is yours.
	r.oneReportSinceMode("F11")
	r.modeEnded("F11", "human_only")
	var frame graph.Frame
	r.do("F11", http.MethodGet, r.api("/frame"), nil, http.StatusOK, &frame)
	if len(frame.Todo.Proposals) != 1 || frame.Todo.Proposals[0].Label != proposals[0].Label {
		r.fail("F11", "«Lo que te toca» proposals %+v, want %s", frame.Todo.Proposals, proposals[0].Label)
	}

	// F13: «ok» to the card. F14: «B, como dije», without choosing.
	joint := r.openCardIn("F13", propuesta)
	r.answerCard("F13", joint.Label, "ok", http.StatusUnprocessableEntity)
	var premise server.OpenQuestionView
	for _, q := range r.checkerQuestionsBlocking("F14", propuesta) {
		if strings.Contains(q.Question, "cerraría si") {
			premise = q
		}
	}
	if premise.Label == "" {
		r.fail("F14", "no «por aclarar» on «cerraría si…»: %+v", r.todo("F14").OpenQuestions)
	}
	r.answerQuestion("F14", premise.Label, nil, "B, como dije", http.StatusUnprocessableEntity)
	// F15: the signoff of «Propuesta» stays blocked by the card, with no Wonder.
	pendings := r.signoffBlocked("F15", propuesta)
	if !hasPending(pendings, graph.PendingOpenCard, joint.Label) || !hasPending(pendings, graph.PendingOpenQuestion, premise.Label) {
		r.fail("F15", "signoff pendings %+v, want the card %s and the «por aclarar» %s", pendings, joint.Label, premise.Label)
	}
	r.allUsed()
}

// frameOf reads the frame.
func (r *rushed) frameOf(step string) graph.Frame {
	r.t.Helper()
	var frame graph.Frame
	r.do(step, http.MethodGet, r.api("/frame"), nil, http.StatusOK, &frame)
	return frame
}

// returnLineAfterMode is rows 7A F13 and 7B F11 (B-85, M-2): the same day, the
// frame brings the return line, and the report is a click away in «Lo que te toca».
func (r *rushed) returnLineAfterMode(step string) {
	r.t.Helper()
	frame := r.frameOf(step)
	if frame.ReturnLine == nil || !frame.ReturnLine.ModeFinished || frame.ReturnLine.Days != 0 {
		r.fail(step, "return line %+v, want the mode's, the same day", frame.ReturnLine)
	}
	if !frame.Todo.StopReportUnseen {
		r.fail(step, "the report is not in «Lo que te toca»: %+v", frame.Todo)
	}
}

// warningInI18n is row 7A F3: the warning of fichas §8.2 the bar shows comes from
// the i18n files, in the two languages.
func (r *rushed) warningInI18n(step string) {
	r.t.Helper()
	raw, err := os.ReadFile("../../../web/src/i18n/es.json")
	if err != nil {
		r.fail(step, "es.json: %v", err)
	}
	var es map[string]string
	if err := json.Unmarshal(raw, &es); err != nil {
		r.fail(step, "es.json: %v", err)
	}
	if !strings.Contains(es["mode.warning"], "Se pueden acumular varias secciones y decenas de decisiones que no habrás visto") {
		r.fail(step, "mode.warning: %q", es["mode.warning"])
	}
	if !strings.Contains(es["mode.yours"], "Lo que solo tú decides te esperará como «por aclarar»") {
		r.fail(step, "mode.yours (B-84): %q", es["mode.yours"])
	}
}

// sevenBMessage is what Diego writes at F3.
const sevenBMessage = "Continúa hasta terminar, estoy en reuniones hasta las 6. Si surge algo, tú decides"

// H-1 (backlog-r4 4.6, the rows of 7B; B-83, B-84, B-85, B-57, 1.4, 2.4, 1.7): the
// mode with his message; no decision unseen and no «por aclarar» stop the chain; the
// horizon blocks three sections with two options; the judge runs with no click and
// finds nothing, so no batch; no round; the ready sections do not stop it; the mode
// ends with «No se puede recomendar todavía…», proposing to sign the three. His
// return shows the line. «ok» leaves the card open, choosing with no «porque» leaves
// the question open and rewrites nothing, three brief signoffs sign and «Evaluación»
// stays blocked before Wonder.
func TestB83_Case07B_TheModeRunsToTheEndAndOnlyHisIsLeft(t *testing.T) {
	const (
		value      = "Valor después del cierre"
		evaluation = "Evaluación"
		recomm     = "Recomendación"
	)
	r := newRushed(t, "caso-07-B-modo")
	r.script.before["02-step-reads"] = func() { r.bar("F3", sevenBMessage) }
	r.open("F1")
	r.submit("F2–F10")
	r.settle("F2–F10")

	// F3.
	r.modeLiteral("F3", sevenBMessage)
	r.warningInI18n("F3")
	// F4–F9: one «por aclarar» of scope, two options, three sections.
	q := r.openQuestionBlocking("F7", value)
	if len(q.Options) != 2 {
		r.fail("F7", "the horizon has %d options: %+v", len(q.Options), q.Options)
	}
	for _, title := range []string{evaluation, recomm} {
		if !strings.Contains(r.todoLabelsBlocking("F7", title), q.Label) {
			r.fail("F7", "«%s» is not blocked by %s: %s", title, q.Label, r.todoLabelsBlocking("F7", title))
		}
	}
	r.oneReportSinceMode("F10")
	// F6: the sensitivity is a source (F-1): the output cites it.
	ev := r.section("F6", evaluation)
	if len(ev.Citations) == 0 || !strings.Contains(ev.Text, "18 %") {
		r.fail("F6", "«%s» does not cite the sensitivity: %+v", evaluation, ev)
	}
	// F9: the judge ran with no click and left no flaw and no batch.
	judges := 0
	rounds, err := r.db.ListObjects(context.Background(), r.project, store.ObjectRound)
	if err != nil {
		r.fail("F9", "rounds: %v", err)
	}
	for _, obj := range rounds {
		card := r.round("F9", obj.Label)
		if card.Kind != "judge" {
			r.fail("F8", "round %+v: no research_request, no round (B-57)", card)
		}
		judges++
		if len(card.Flaws) != 0 || card.BatchStarted {
			r.fail("F9", "judge round %+v, want no flaw and no batch", card)
		}
	}
	if judges == 0 {
		r.fail("F9", "the judge did not run")
	}
	// F10: the ready sections did not stop it; the report's first line.
	report := r.modeEnded("F10", "human_only")
	if !strings.HasPrefix(report.Answer, "No se puede recomendar todavía") {
		r.fail("F10", "report answer %q", report.Answer)
	}
	ready := r.frameOf("F10").Todo.ReadyToSign
	if len(ready) != 3 {
		r.fail("F10", "ready to sign: %q, want the three", ready)
	}
	// F11.
	r.returnLineAfterMode("F11")

	// F12–F14: as the 2.10 row.
	r.say("F12", "¿ya está listo para firmar?")
	card := r.openCardIn("F13", evaluation)
	r.answerCard("F13", card.Label, "ok", http.StatusUnprocessableEntity)
	if got := r.cardState("F13", card.Label); got != "open" || r.withoutReason("F13", card.Label) != 1 {
		r.fail("F13", "card %s after «ok»: %q", card.Label, got)
	}
	r.say("F14", "firma tú lo que esté bien")
	if n := r.count(store.ObjectSignoff); n != 0 {
		r.fail("F14", "«firma tú» created %d signoffs", n)
	}
	// F15: «la a, 12 años, ok» chooses and gives a datum: it closes the «por aclarar»
	// (backlog-r4 v0.8, as the 2.10 built it).
	r.answerQuestion("F15", q.Label, intPtr(0), "la a, 12 años, ok", http.StatusOK)
	if _, open := r.openQuestionView("F15", q.Label); open {
		r.fail("F15", "«la a, 12 años, ok» left the «por aclarar» open")
	}
	// F16: the three with no pendings: brief signoffs (2.10).
	r.signBrief("F16", "Contexto", "Por qué la mina evalúa una planta propia en vez de la red.")
	r.signBrief("F16", "Demanda de energía", "La mina consume parejo durante el día, según la curva.")
	r.signBrief("F16", "Inversión", "Dos proveedores cotizaron la planta llave en mano.")
	// F17: «Evaluación»: blocked before Wonder.
	before := r.count(store.ObjectSignoff)
	pendings := r.signoffBlocked("F17", evaluation)
	if !hasPending(pendings, graph.PendingOpenCard, card.Label) {
		r.fail("F17", "signoff pendings %+v, want the card %s", pendings, card.Label)
	}
	if r.count(store.ObjectSignoff) != before {
		r.fail("F17", "a signoff opened")
	}
	r.allUsed()
}

// todoLabelsBlocking are the labels of the open questions that block a section.
func (r *rushed) todoLabelsBlocking(step, title string) string {
	r.t.Helper()
	var labels []string
	for _, q := range r.todo(step).OpenQuestions {
		for _, s := range q.Sections {
			if s.Title == title {
				labels = append(labels, q.Label)
			}
		}
	}
	return strings.Join(labels, ",")
}

// noContrast says a round's card carries no expectation and no contrast.
func noContrast(card server.RoundCard) bool {
	return card.Expectation == "" && card.Digest != nil && card.Digest.ExpectationContrast == ""
}

// H-1 (backlog-r4 4.6, the rows of 9B; B-83, B-64, B-59, hueco 1, B-73 a, B-76, 1.7):
// the mode with its message; the round runs alone, three readers, no expectation and
// no contrast, 11 findings in, the twelfth app a candidate. The checker's «por
// aclarar» and narrative do not stop it; her edit during the agent's rewrite rules
// and leaves its proposal, with no alert. The principal cleans the codes. The mode
// ends with the likely questions of engineering. «ok» opens nothing; the signoff
// lists the model and the package and not the proposal. Her answers close them; the
// signoff lists the proposal; no coincide then coincide; signed, with the proposal
// discarded as signed_without.
func TestB76_Case09B_HerEditLeavesAProposalAndSigningDiscardsIt(t *testing.T) {
	const (
		model   = "Modelo de cobro"
		message = "Continúa hasta terminar"
	)
	r := newRushed(t, "caso-09-B-modo")
	r.script.before["02-step-discover"] = func() { r.bar("F3", message) }
	// F9: she edits the sentence while the agent rewrites it, and F11's pendings
	// are there when the principal is told to clean them.
	r.script.before["10-step-rewrites-her-sentence"] = func() {
		sec := r.section("F9", model)
		text := strings.Replace(sec.Text, "Para los negocios: pagas solo cuando cobras una reserva.",
			"Para los negocios: solo pagas una comisión cuando cobras una reserva, sin cargos fijos.", 1)
		r.do("F9", http.MethodPut, r.api("/sections/"+sec.Label), map[string]any{"base_version": sec.Version, "text": text}, http.StatusOK, nil)
	}
	cleanedFrom := []graph.Pending{}
	r.script.before["11-step-cleans"] = func() { cleanedFrom = r.section("F11", model).Pendings }
	r.open("F1")
	r.submit("F2–F12")
	r.settle("F2–F12")

	// F3–F6.
	r.modeLiteral("F3", message)
	round := r.roundOfKind("F5", "research")
	if len(round.Parts) != cycle.MaxRoundParts || !noContrast(round) || enteredFindings(round) != 11 {
		r.fail("F5", "round %+v: want %d parts, no contrast, 11 entered (%d)", round, cycle.MaxRoundParts, enteredFindings(round))
	}
	if c := r.candidateNaming("F6", "Pagafacil"); c.Text == "" {
		r.fail("F6", "the twelfth app is not a candidate")
	}
	// F8: the «por aclarar» of the package, without a stop.
	qs := r.checkerQuestionsBlocking("F8", model)
	if len(qs) != 1 {
		r.fail("F8", "want one «por aclarar» of the checker blocking «%s», got %+v", model, qs)
	}
	pkg := qs[0]
	r.oneReportSinceMode("F12")
	// F9–F10: the proposal; her text rules; no alert.
	sec := r.section("F9", model)
	proposals := r.proposalsOn("F9", sec.Label)
	if len(proposals) != 1 || proposals[0].State != "open" || !strings.Contains(proposals[0].Text, "la app cobra una comisión") {
		r.fail("F9", "proposals %+v, want the rewrite, open", proposals)
	}
	if !strings.Contains(sec.Text, "sin cargos fijos") {
		r.fail("F9", "her sentence does not rule: %q", sec.Text)
	}
	r.noAlert("F9")
	// F11: the pendings were there, and the principal cleaned them.
	if !hasPending(cleanedFrom, graph.PendingProcessNarrative, "") || !hasPending(cleanedFrom, graph.PendingInternalCodes, "dec-1") {
		r.fail("F11", "pendings before the cleaning: %+v", cleanedFrom)
	}
	if hasPending(sec.Pendings, graph.PendingProcessNarrative, "") || hasPending(sec.Pendings, graph.PendingInternalCodes, "") {
		r.fail("F11", "pendings after the cleaning: %+v", sec.Pendings)
	}
	// F12: the mode ended with the likely questions of engineering.
	r.modeEnded("F12", "human_only")
	likely := false
	for _, q := range r.defenseQuestions("F13") {
		likely = likely || strings.Contains(objectText(q), "reembolso")
	}
	if !likely {
		r.fail("F13", "no likely question of engineering")
	}

	// F13: «ok» to all.
	todo := r.todo("F13")
	for _, oq := range todo.OpenQuestions {
		r.answerQuestion("F13", oq.Label, nil, "ok", http.StatusUnprocessableEntity)
	}
	for _, c := range todo.Cards {
		r.answerCard("F13", c.Label, "ok", http.StatusUnprocessableEntity)
	}
	// F14: blocked by the model and the package; the proposal is no pending.
	pendings := r.signoffBlocked("F14", model)
	if !hasPending(pendings, graph.PendingOpenQuestion, pkg.Label) {
		r.fail("F14", "signoff pendings %+v, want the package %s", pendings, pkg.Label)
	}
	questions := 0
	for _, p := range pendings {
		if p.Kind == graph.PendingOpenQuestion {
			questions++
		}
		if strings.Contains(p.Kind, "proposal") || p.About == proposals[0].Label {
			r.fail("F14", "the proposal is among the pendings: %+v", pendings)
		}
	}
	if questions != 2 {
		r.fail("F14", "signoff pendings %+v, want the model and the package", pendings)
	}

	// F15: each with its «porque».
	r.answerQuestion("F15", pkg.Label, intPtr(0), "no cuenta como pago por uso, porque cierran en temporada baja y el paquete vence a los 90 días", http.StatusOK)
	for _, oq := range r.todo("F15").OpenQuestions {
		for _, s := range oq.Sections {
			if s.Title == model {
				r.answerQuestion("F15", oq.Label, intPtr(0), "porque la mitad de mis reservas es de S/ 15–20 y ahí deja más", http.StatusOK)
				break
			}
		}
	}
	for _, c := range r.todo("F15").Cards {
		r.answerCard("F15", c.Label, "porque mis locales cierran en temporada baja y odian los cargos fijos", http.StatusOK)
	}
	if got := r.checkerQuestionsBlocking("F15", model); len(got) != 0 {
		r.fail("F15", "still open: %+v", got)
	}
	// The principal retires the package: «Sigue» on the report.
	r.do("F15", http.MethodPost, r.api("/stop-report/continue"), map[string]any{"text": "Sigue"}, http.StatusAccepted, nil)
	r.settle("F15")

	// F16: the signoff lists the proposal; the first explanation does not coincide.
	view := r.startSignoff("F16", model, "Comisión mixta, porque la mayoría de las reservas son baratas; nada de suscripción ni paquetes, porque odian los cargos fijos.")
	if len(view.OpenProposals) != 1 || view.OpenProposals[0].Label != proposals[0].Label {
		r.fail("F16", "signoff lists %+v, want the proposal %s", view.OpenProposals, proposals[0].Label)
	}
	if view.CanSign {
		r.fail("F16", "it can sign before the second round: %+v", view)
	}
	// F17: it coincides; signed; the proposal discarded, in sight in the history.
	r.do("F17", http.MethodPost, r.api("/signoffs/"+view.Label+"/answers"),
		map[string]any{"origin_answer": "De nuestros datos de clientes; sobre unos S/ 35 el 4 % deja más y lo aceptamos.", "case_answer": ""}, http.StatusAccepted, nil)
	r.turnsOK("F17")
	r.do("F17", http.MethodPost, r.api("/signoffs/"+view.Label+"/sign"), nil, http.StatusOK, nil)
	r.wantStatus("F17", model, "signed")
	r.wantDiscardedSignedWithout("F17", proposals[0].Label)
	r.allUsed()
}

// wantDiscardedSignedWithout asserts B-76's end of a proposal, by its stored state.
func (r *rushed) wantDiscardedSignedWithout(step, label string) {
	r.t.Helper()
	obj, err := r.db.GetObject(context.Background(), r.project, label)
	if err != nil {
		r.fail(step, "%s: %v", label, err)
	}
	b, err := graph.ParseProposal(obj)
	if err != nil {
		r.fail(step, "%s: %v", label, err)
	}
	if obj.State != "discarded" || b.Why != graph.ProposalSignedWithout {
		r.fail(step, "proposal %s: %s why %q, want discarded signed_without", label, obj.State, b.Why)
	}
}

// startupReasons are the reasons of the principal's startups, in order.
func (r *rushed) startupReasons(step string) []string {
	r.t.Helper()
	var out []string
	for _, ev := range r.eventsOf(step, store.EventStartup) {
		if reason, _ := eventData(ev)["reason"].(string); reason != "" {
			out = append(out, reason)
		}
	}
	return out
}

// checkYourEditNow is B-36's wait of your edit ending: the batch goes to the
// checker, as the app's timer leaves it, without the test waiting for it.
func (r *rushed) checkYourEditNow() {
	r.t.Helper()
	var fire func()
	r.starter.runner.AfterFunc = func(_ time.Duration, f func()) func() bool {
		fire = f
		return func() bool { return false }
	}
	r.starter.runner.EditSaved(r.project)
	fire()
}

// H-1 (backlog-r4 4.6, the rows of 7A; B-83, B-84, B-85, B-64, B-67, B-86, B-57, B-80,
// 2.4, 1.4, 1.7): the mode with his literal message and its warning; the round runs
// alone over three readers, no expectation, no contrast; the horizon is one «por
// aclarar» that blocks and does not stop the chain; the irradiation card waits; the
// step of the 1 000 scenarios crosses the threshold and the next starts with startup
// threshold, the mode still on; the judge runs alone, one flaw, one batch, one
// recheck, and the principal's «tasa real» is a critical card; the same question
// blocks «Recomendación»; no other round; the mode ends with its next step. He comes
// back the same day: the line. His answers with «porque» close them; «nominal»
// creates his decision and two sections go stale; recalculation and recheck in normal
// mode; he signs; his own rewrite alerts rule 1 without blocking and his text rules.
func TestB83_Case07A_TheModeRunsAllDayAndTheHumanDecidesAtTheEnd(t *testing.T) {
	const (
		value      = "Valor después del cierre"
		evaluation = "Evaluación"
		recomm     = "Recomendación"
		message    = "Continúa hasta terminar, estoy en reuniones hasta las 6. Si surge algo, tú decides"
	)
	r := newRushed(t, "caso-07-A")
	r.script.before["01b-step-reads"] = func() { r.bar("F3", message) }
	// What the row asserts mid-chain, read as the next step is in flight.
	var afterF5 server.OpenQuestionView
	r.script.before["08-step-evaluacion-variants"] = func() { afterF5 = r.openQuestionBlocking("F5", value) }
	var afterF7 server.CardView
	r.script.before["10-step-scenarios"] = func() { afterF7 = r.openCardIn("F7", evaluation) }
	var startupsAtF9 []string
	modeAtF9 := ""
	r.script.before["11-step-closes-with-risk"] = func() {
		startupsAtF9 = r.startupReasons("F8")
		var m struct {
			Mode string `json:"mode"`
		}
		r.do("F8", http.MethodGet, r.api("/summary"), nil, http.StatusOK, &m)
		modeAtF9 = m.Mode
	}
	r.open("F1")
	r.submit("F2–F12")
	r.settle("F2–F12")

	// F3.
	r.modeLiteral("F3", message)
	r.warningInI18n("F3")
	// F4–F5: the round, no click, no expectation, no contrast.
	round := r.roundOfKind("F5", "research")
	if round.State != "closed" || len(round.Parts) != 3 || !noContrast(round) || enteredFindings(round) != 3 {
		r.fail("F5", "round %+v: want it closed, 3 readers, no contrast, 3 entered (%d)", round, enteredFindings(round))
	}
	if afterF5.Label == "" || len(afterF5.Options) != 2 || !strings.Contains(afterF5.Options[0].Option, "12 años") || !strings.Contains(afterF5.Options[1].Option, "25 años") {
		r.fail("F5", "the «por aclarar» after F5: %+v", afterF5)
	}
	// F6–F8: the card, the threshold, the mode goes on.
	if afterF7.Label == "" || !strings.Contains(afterF7.Statement, "satelital") {
		r.fail("F7", "the card after F7: %+v", afterF7)
	}
	if len(startupsAtF9) == 0 || startupsAtF9[len(startupsAtF9)-1] != "threshold" || modeAtF9 != graph.ModeRunToCompletion {
		r.fail("F8", "startups %q, mode %q at the next step: want threshold, run_to_completion", startupsAtF9, modeAtF9)
	}
	// F9: one flaw, one batch, one recheck.
	judge := r.judgeWithFlaws("F9")
	if len(judge.Flaws) != 1 || judge.Flaws[0].Verdict != cycle.FlawResolved {
		r.fail("F9", "judge round %+v, want one flaw resolved at its recheck", judge)
	}
	if ev := r.section("F9", evaluation); !strings.Contains(ev.Text, "18 %") {
		r.fail("F9", "the 18 %% did not hold: %q", ev.Text)
	}
	rate := r.cardStating("F9", "tasa real")
	// F10–F12: the same question blocks «Recomendación»; no other; no other round.
	if got := r.todoLabelsBlocking("F10", recomm); got != afterF5.Label {
		r.fail("F10", "«%s» is blocked by %q, want the one of F5, %s", recomm, got, afterF5.Label)
	}
	if n := len(r.todo("F10").OpenQuestions); n != 1 {
		r.fail("F10", "%d «por aclarar», want one", n)
	}
	r.noOtherResearchRound("F11", round.Label)
	r.oneReportSinceMode("F12")
	report := r.modeEnded("F12", "human_only")
	if !strings.Contains(report.Next, "comparar con otras minas") {
		r.fail("F12", "report's next: %q", report.Next)
	}
	// F13: the line, the same day.
	r.returnLineAfterMode("F13")
	// F14: he asks; the agent explains.
	r.say("F14", "¿18 %? Pensé que era menos de 1 %. ¿De dónde sale?")

	// F15: the question closes with its «porque».
	r.answerQuestion("F15", afterF5.Label, intPtr(0), "12 años y vender equipos, porque la minera no tiene concesión para vender a la red", http.StatusOK)
	if _, open := r.openQuestionView("F15", afterF5.Label); open {
		r.fail("F15", "the «por aclarar» %s stayed open", afterF5.Label)
	}
	// F15 (hueco 2): «nominal» answers the card with «la otra» and a «porque»: his
	// decision replaces the agent's and the sections that use it go stale.
	var answered server.AnswerCardResponse
	r.do("F15", http.MethodPost, r.api("/cards/"+rate.Label+"/answer"),
		map[string]any{"option": "other", "reason": "Porque el 10 % de la política corporativa es nominal, no real."}, http.StatusOK, &answered)
	if answered.NewDecision == "" {
		r.fail("F15", "the answer %+v created no decision of his", answered)
	}
	r.wantStatus("F15", evaluation, "stale")
	r.wantStatus("F15", recomm, "stale")
	// F16: the irradiation card, with its «porque».
	r.answerCard("F16", afterF7.Label, "Satelital, sí, porque 3 años es poco y la estación no cubre años Niño.", http.StatusOK)
	// F16: the recalculation and the recheck, in normal mode.
	judgesBefore := r.countJudgeRounds("F16")
	r.do("F16", http.MethodPost, r.api("/stop-report/continue"), map[string]any{"text": "Sigue"}, http.StatusAccepted, nil)
	r.settle("F16")
	if n := len(r.eventsOf("F16", store.EventModeSet)); n != 2 {
		r.fail("F16", "%d mode_set, want activation and its end: the chain runs in normal mode", n)
	}
	if r.countJudgeRounds("F16") != judgesBefore+1 {
		r.fail("F16", "no recheck ran: %d judge rounds, was %d", r.countJudgeRounds("F16"), judgesBefore)
	}
	if ev := r.section("F16", evaluation); !strings.Contains(ev.Text, "21 %") {
		r.fail("F16", "«%s» was not recalculated: %q", evaluation, ev.Text)
	}

	// F18: he signs «Evaluación».
	view := r.startSignoff("F18", evaluation, "12 años, tasa nominal 10 %, valor actual neto negativo en uno de cada cinco escenarios, sobre todo por el precio de la red.")
	r.do("F18", http.MethodPost, r.api("/signoffs/"+view.Label+"/answers"),
		map[string]any{"origin_answer": "La estación no cubre años Niño.", "case_answer": ""}, http.StatusAccepted, nil)
	r.turnsOK("F18")
	r.do("F18", http.MethodPost, r.api("/signoffs/"+view.Label+"/sign"), nil, http.StatusOK, nil)
	r.wantStatus("F18", evaluation, "signed")

	// F19: he rewrites «Recomendación»: rule 1 (more than half), with no block.
	const his = "No construir con estas condiciones; reevaluar si baja la inversión."
	sec := r.section("F19", recomm)
	r.do("F19", http.MethodPut, r.api("/sections/"+sec.Label), map[string]any{"base_version": sec.Version, "text": his}, http.StatusOK, nil)
	sec = r.section("F19", recomm)
	if sec.Text != his {
		r.fail("F19", "his text does not rule: %q", sec.Text)
	}
	alerts := r.eventsOf("F19", store.EventSuspiciousEditAlert)
	if len(alerts) != 1 || alerts[0].Actor != "app" || !strings.Contains(string(alerts[0].Data), "half") {
		r.fail("F19", "alerts: %+v, want the app's one of rule 1", alerts)
	}
	if len(sec.Alerts) != 1 || !sec.UndoAvailable {
		r.fail("F19", "section alerts %+v, undo %v (%s): want the alert unanswered and «Deshacer» on", sec.Alerts, sec.UndoAvailable, sec.UndoReason)
	}
	// B-36: the checker on his edit (hueco 3: the judge only if he audits).
	r.checkYourEditNow()
	if r.countJudgeRounds("F19") != judgesBefore+1 {
		r.fail("F19", "a judge round ran on his edit")
	}
	r.allUsed()
}

// cardStating is the open card whose decision says word.
func (r *rushed) cardStating(step, word string) server.CardView {
	r.t.Helper()
	todo := r.todo(step)
	for _, c := range todo.Cards {
		if strings.Contains(c.Statement, word) {
			return c
		}
	}
	r.fail(step, "no open card on %q: %+v", word, todo.Cards)
	return server.CardView{}
}

// noOtherResearchRound asserts only that research round exists (B-57).
func (r *rushed) noOtherResearchRound(step, label string) {
	r.t.Helper()
	rounds, err := r.db.ListObjects(context.Background(), r.project, store.ObjectRound)
	if err != nil {
		r.fail(step, "rounds: %v", err)
	}
	for _, obj := range rounds {
		if card := r.round(step, obj.Label); card.Kind == "research" && card.Label != label {
			r.fail(step, "a research round nobody asked for: %+v", card)
		}
	}
}

func (r *rushed) countJudgeRounds(step string) int {
	r.t.Helper()
	rounds, err := r.db.ListObjects(context.Background(), r.project, store.ObjectRound)
	if err != nil {
		r.fail(step, "rounds: %v", err)
	}
	n := 0
	for _, obj := range rounds {
		if r.round(step, obj.Label).Kind == "judge" {
			n++
		}
	}
	return n
}

// H-1 (backlog-r4 4.6, the rows of 9A; B-64, B-59, hueco 1, 2.3, 2.4, 2.2, B-67, B-73 a,
// B-80, 2.1, B-76, 1.7): the round alone, three readers, no expectation, «ni idea»
// staying her message; the plan against C1 and the commission as «por aclarar», the
// chain going on; her (a) closes it and two sections go stale; the judge's one flaw,
// correction and recheck; the package is a «por aclarar», not a card; her edit during
// the agent's rewrite leaves one proposal and no alert, and the principal sees it;
// the paragraph's retirement applies on her edit; the codes and the narrative block
// the signoff, her edit leaves no alert; the signoff lists the proposal and signing
// discards it as signed_without.
func TestB76_Case09A_HerEditDuringARewriteAndSigningWithoutOpeningIt(t *testing.T) {
	const model = "Modelo de cobro"
	r := newRushed(t, "caso-09-A")
	r.script.before["19-step-rewrites-section"] = func() {
		sec := r.section("F12", model)
		text := strings.Replace(sec.Text, "Para los negocios: pagas solo cuando cobras una reserva.",
			"Para los negocios: solo pagas una comisión cuando cobras una reserva.", 1)
		r.do("F12", http.MethodPut, r.api("/sections/"+sec.Label), map[string]any{"base_version": sec.Version, "text": text}, http.StatusOK, nil)
	}
	r.open("F1")
	r.submit("F1–F8")

	// F3–F5.
	round := r.roundOfKind("F5", "research")
	if round.State != "closed" || len(round.Parts) != cycle.MaxRoundParts || !noContrast(round) || enteredFindings(round) != 11 {
		r.fail("F5", "round %+v: want it closed, %d readers, no expectation, 11 entered (%d)", round, cycle.MaxRoundParts, enteredFindings(round))
	}
	const niIdea = "ni idea, quiero ver"
	r.say("F4", niIdea)
	found := false
	for _, ev := range r.eventsOf("F4", store.EventHumanMessage) {
		found = found || eventData(ev)["text"] == niIdea
	}
	if !found {
		r.fail("F4", "«%s» is not her message", niIdea)
	}
	if got := r.round("F4", round.Label); got.Expectation != "" {
		r.fail("F4", "the round's expectation: %q, want none", got.Expectation)
	}

	// F8: the contradiction with C1, and the commission; nothing stopped the chain.
	var plan, commission server.OpenQuestionView
	for _, q := range r.todo("F8").OpenQuestions {
		body, _ := r.openQuestionObject("F8", q.Label)
		switch {
		case body["kind"] == graph.CandidateContradiction:
			plan = q
		case strings.Contains(q.Question, "comisión"):
			commission = q
		}
	}
	if plan.Label == "" || commission.Label == "" {
		r.fail("F8", "want the contradiction with C1 and the commission: %+v", r.todo("F8").OpenQuestions)
	}
	if r.section("F8", "Migración").Text == "" {
		r.fail("F8", "the chain stopped before the other sections")
	}

	// F9: (a) with its «porque», the plan out.
	r.answerQuestion("F9", commission.Label, intPtr(0), "Mi media está en S/ 15–20, porque ahí la mixta gana.", http.StatusOK)
	r.answerQuestion("F9", plan.Label, intPtr(0), "Porque un plan mensual es suscripción.", http.StatusOK)
	r.say("F9", "(a). Mi media está en S/ 15–20; ahí la mixta gana. Saca el plan mensual: es suscripción")
	if _, open := r.openQuestionView("F9", commission.Label); open {
		r.fail("F9", "the «por aclarar» of the commission stayed open")
	}
	r.wantStatus("F9", "Reembolsos", "stale")
	r.wantStatus("F9", "Liquidación", "stale")

	// F10: one flaw, one correction, one recheck that passes.
	judge := r.judgeWithFlaws("F10")
	if len(judge.Flaws) != 1 || judge.Flaws[0].Verdict != cycle.FlawResolved {
		r.fail("F10", "judge round %+v, want one flaw resolved", judge)
	}
	if text := r.section("F10", model).Text; !strings.Contains(text, "unos S/ 33") {
		r.fail("F10", "the correction did not apply: %q", text)
	}

	// F11: the package: a «por aclarar» that blocks the model, not a card. F12: her
	// edit during the rewrite.
	cardsBefore := len(r.todo("F11").Cards)
	r.do("F11", http.MethodPost, r.api("/stop-report/continue"), map[string]any{"text": "Sigue"}, http.StatusAccepted, nil)
	r.settle("F11")
	pkgQs := r.checkerQuestionsBlocking("F11", model)
	if len(pkgQs) != 1 || !strings.Contains(pkgQs[0].Question, "paquete") {
		r.fail("F11", "want the «por aclarar» of the package blocking «%s»: %+v", model, pkgQs)
	}
	if n := len(r.todo("F11").Cards); n != cardsBefore {
		r.fail("F11", "cards went from %d to %d: the package is not a card", cardsBefore, n)
	}
	pkg := pkgQs[0]
	sec := r.section("F12", model)
	proposals := r.proposalsOn("F12", sec.Label)
	if len(proposals) != 1 || proposals[0].State != "open" || proposals[0].Contrast.Fingerprint == "" {
		r.fail("F12", "proposals %+v, want one open with its contrast", proposals)
	}
	if !strings.Contains(sec.Text, "solo pagas una comisión") {
		r.fail("F12", "her sentence does not rule: %q", sec.Text)
	}
	if text := r.section("F12", "Pantalla del negocio").Text; !strings.Contains(text, "fecha de cada liquidación") {
		r.fail("F12", "the rest of the step did not apply: %q", text)
	}
	r.noAlert("F12")

	// F13: «no» with its «porque»; the step retires the paragraph (F14's question).
	r.answerQuestion("F13", pkg.Label, intPtr(0), "No: si vence a los 90 días, el local que cierra en temporada baja pierde lo pagado, porque pagar por adelantado no es pago por uso.", http.StatusOK)
	r.do("F13", http.MethodPost, r.api("/stop-report/continue"), map[string]any{"text": "Sigue"}, http.StatusAccepted, nil)
	r.settle("F13")
	if text := r.section("F14", model).Text; strings.Contains(text, "vence a los 90 días") || !strings.Contains(text, "solo pagas una comisión") {
		r.fail("F13", "the retirement did not apply over her edit: %q", text)
	}
	// F15: (a) with its «porque»; both closed.
	var shown string
	for _, q := range r.todo("F15").OpenQuestions {
		if strings.Contains(q.Question, "se le muestra") {
			shown = q.Label
		}
	}
	if shown == "" {
		r.fail("F14", "no «por aclarar» on what the business sees: %+v", r.todo("F14").OpenQuestions)
	}
	r.answerQuestion("F15", shown, intPtr(0), "(a), con el desglose, porque esconderlo es peor si luego reclaman.", http.StatusOK)
	if _, open := r.openQuestionView("F15", pkg.Label); open {
		r.fail("F15", "the package stayed open")
	}
	if _, open := r.openQuestionView("F15", shown); open {
		r.fail("F15", "what the business sees stayed open")
	}

	// F16: the codes and the narrative are pendings; the signoff stays blocked.
	r.do("F16", http.MethodPost, r.api("/stop-report/continue"), map[string]any{"text": "Sigue"}, http.StatusAccepted, nil)
	r.settle("F16")
	pendings := r.signoffBlocked("F16", model)
	if !hasPending(pendings, graph.PendingInternalCodes, "dec-2") || !hasPending(pendings, graph.PendingProcessNarrative, "") {
		r.fail("F16", "signoff pendings %+v, want internal_codes dec-2 and process_narrative", pendings)
	}
	// F17: her edit of «la segunda decisión» leaves no alert.
	sec = r.section("F17", model)
	text := strings.Replace(sec.Text, "la segunda decisión", "la decisión de comisión", 1)
	r.do("F17", http.MethodPut, r.api("/sections/"+sec.Label), map[string]any{"base_version": sec.Version, "text": text}, http.StatusOK, nil)
	r.noAlert("F17")
	for _, c := range r.todo("F17").Cards {
		r.answerCard("F17", c.Label, "Porque la mitad de mis reservas está en S/ 15–20 y ahí la mixta deja más.", http.StatusOK)
	}
	r.do("F17", http.MethodPost, r.api("/stop-report/continue"), map[string]any{"text": "Sigue"}, http.StatusAccepted, nil)
	r.settle("F17")

	// F18: asking lists the proposal; signing without it discards it; signed.
	view := r.startSignoff("F18", model, "Comisión mixta; ni plan ni paquete; y el desglose para el negocio.")
	if len(view.OpenProposals) != 1 || view.OpenProposals[0].Label != proposals[0].Label {
		r.fail("F18", "signoff lists %+v, want the proposal %s", view.OpenProposals, proposals[0].Label)
	}
	r.do("F18", http.MethodPost, r.api("/signoffs/"+view.Label+"/answers"),
		map[string]any{"origin_answer": "De que mi media esté en S/ 15–20; es mi dato, no lo verificó nadie.", "case_answer": ""}, http.StatusAccepted, nil)
	r.turnsOK("F18")
	r.do("F18", http.MethodPost, r.api("/signoffs/"+view.Label+"/sign"), nil, http.StatusOK, nil)
	r.wantStatus("F18", model, "signed")
	r.wantDiscardedSignedWithout("F18", proposals[0].Label)
	r.allUsed()
}

// H-1 (backlog-r4 4.6, rows 6B F6 and F12–F19; hueco 4; B-67): the recheck's flaws
// (1) and (2) open their «por aclarar» and «Diagnóstico» goes to changed_since_signoff;
// it stays there with its proposal and its «por aclarar» open, and its signoff stays
// blocked by that «por aclarar», with no Wonder.
func TestB67_Rushed06B_TheFlawsMoveSignedDiagnosticoToChanged(t *testing.T) {
	r, diag, _ := sixBToTheEnd(t)
	sec := r.section("F6", "Diagnóstico")
	if sec.Status != "changed_since_signoff" || !hasPending(sec.Pendings, graph.PendingOpenQuestion, "") {
		r.fail("F6", "«Diagnóstico» is %q with pendings %+v, want changed_since_signoff with its «por aclarar»", sec.Status, sec.Pendings)
	}
	if len(r.proposalsOn("F19", diag)) != 1 {
		r.fail("F19", "the proposal is not open on «Diagnóstico»")
	}
	if pendings := r.signoffBlocked("F15", "Diagnóstico"); !hasPending(pendings, graph.PendingOpenQuestion, "") {
		r.fail("F15", "signoff of «Diagnóstico»: pendings %+v, want its «por aclarar»", pendings)
	}
}
