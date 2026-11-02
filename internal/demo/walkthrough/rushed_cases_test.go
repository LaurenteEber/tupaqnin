package walkthrough

import (
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/LaurenteEber/tupaqnin/internal/graph"
	"github.com/LaurenteEber/tupaqnin/internal/server"
	"github.com/LaurenteEber/tupaqnin/internal/store"
)

// H-1 (backlog-r2 2.10, row 5B, and the slice-2 stretch of case 5): the
// agent counts the laboratory pilot with real plant water (D1) as
// «campo»; the checker opens a «por aclarar» on the person's term that
// blocks «Estudios incluidos» and is listed in GET /todo; the card
// answered «ok» stays open and the signoff stays blocked. F3–5 (readers)
// are substituted by read_source over the case's sources, D1 included;
// cut at F8 (the judge, slice 3).
func TestH1_Rushed05B_CampoOpenQuestionBlocksIncludedStudies(t *testing.T) {
	r := newRushed(t, "caso-05-B")
	r.openTemplate("F1")
	r.submit("F2–F6")

	// Tras F6: a «por aclarar» on «campo» blocking «Estudios incluidos»,
	// listed in GET /todo.
	q := r.openQuestionBlocking("F6", "Estudios incluidos")
	body, _ := r.openQuestionObject("F6", q.Label)
	if body["kind"] != graph.CandidateDoubtfulTerm {
		r.fail("F6", "the «por aclarar» %s is a %v, want a doubtful term of the person's criterion", q.Label, body["kind"])
	}
	if _, text := r.protectedText("F6", body); !strings.Contains(text, "campo") {
		r.fail("F6", "the «por aclarar» %s protects %q, want the criterion on «campo»", q.Label, text)
	}

	// F7: the card answered «ok» stays open; the signoff of «Estudios
	// incluidos» stays blocked.
	card := r.openCardIn("F7", "Estudios incluidos")
	r.answerCard("F7", card.Label, "ok", http.StatusUnprocessableEntity)
	if got := r.cardState("F7", card.Label); got != "open" {
		r.fail("F7", "card %s after «ok»: %q, want open", card.Label, got)
	}
	pendings := r.signoffBlocked("F7", "Estudios incluidos")
	if !hasPending(pendings, graph.PendingOpenQuestion, q.Label) || !hasPending(pendings, graph.PendingOpenCard, card.Label) {
		r.fail("F7", "signoff pendings %+v, want the «por aclarar» %s and the card %s", pendings, q.Label, card.Label)
	}
	r.allUsed()
}

// H-1 (backlog-r2 2.10, row 1B; 2.3, 2.4 · fichas §3, estados §4): the
// agent writes the orientation with its reading of «norte», which the
// «ok» of F2 did not confirm; the checker opens a «por aclarar» that
// blocks «Implantación», marked «no lo declaró el agente», beside its
// open critical card. At F11 the «ok»s leave answer_without_reason,
// everything stays open and the signoff of «Implantación» is blocked.
// F3–5 (readers) are substituted by read_source; cut at F13 (the judge).
func TestH1_Rushed01B_NorteContradictionBlocksImplantacion(t *testing.T) {
	const implantacion = "Implantación y orientación"
	r := newRushed(t, "caso-01-B")
	r.open("F1")
	// F2: the human's «ok» to the understanding is a chat answer; the
	// chain starts on its own (B-17) and the «ok» confirms no case.
	r.submit("F2–F8")

	// Tras F6: a «por aclarar» open that blocks «Implantación», «no lo
	// declaró el agente», and its critical card open.
	q := r.openQuestionBlocking("F6", implantacion)
	if !q.UndeclaredByAgent {
		r.fail("F6", "the «por aclarar» %s is not marked «no lo declaró el agente»: %+v", q.Label, q)
	}
	body, _ := r.openQuestionObject("F6", q.Label)
	if _, text := r.protectedText("F6", body); !strings.Contains(text, "norte") {
		r.fail("F6", "the «por aclarar» %s protects %q, want the criterion «las aulas miran al norte»", q.Label, text)
	}
	card := r.openCardIn("F6", implantacion)

	// F9: the human asks; the agent explains without editing.
	r.say("F9", "¿Por qué parasoles verticales y no un alero más largo?")

	// F11: «Firmar» on «Implantación», then «ok» to every card and «por
	// aclarar».
	r.signoffBlocked("F11", implantacion)
	todo := r.todo("F11")
	for _, c := range todo.Cards {
		r.answerCard("F11", c.Label, "ok", http.StatusUnprocessableEntity)
	}
	for _, oq := range todo.OpenQuestions {
		r.answerQuestion("F11", oq.Label, nil, "ok", http.StatusUnprocessableEntity)
	}
	// Tras F11: answer_without_reason on each, everything still open, the
	// signoff of «Implantación» blocked.
	after := r.todo("F11")
	if len(after.Cards) != len(todo.Cards) || len(after.OpenQuestions) != len(todo.OpenQuestions) {
		r.fail("F11", "after the «ok»s: %d cards and %d «por aclarar» open, want %d and %d",
			len(after.Cards), len(after.OpenQuestions), len(todo.Cards), len(todo.OpenQuestions))
	}
	for _, c := range todo.Cards {
		if r.withoutReason("F11", c.Label) == 0 {
			r.fail("F11", "card %s: no answer_without_reason after «ok»", c.Label)
		}
	}
	for _, oq := range todo.OpenQuestions {
		if r.withoutReason("F11", oq.Label) == 0 {
			r.fail("F11", "«por aclarar» %s: no answer_without_reason after «ok»", oq.Label)
		}
	}
	pendings := r.signoffBlocked("F11", implantacion)
	if !hasPending(pendings, graph.PendingOpenQuestion, q.Label) || !hasPending(pendings, graph.PendingOpenCard, card.Label) {
		r.fail("F11", "signoff pendings %+v, want the «por aclarar» %s and the card %s", pendings, q.Label, card.Label)
	}

	// F12: she chooses and writes a hurried «porque» on each; the app
	// records them. No brake is asserted here; F13 is the judge.
	r.answerQuestion("F12", q.Label, intPtr(0), "porque son bebés y no quiero sol en ninguna aula", http.StatusOK)
	r.do("F12", http.MethodPost, r.api("/cards/"+card.Label+"/answer"),
		map[string]any{"option": "other", "reason": "lactantes al norte, porque es mi criterio"}, http.StatusOK, nil)
	for _, oq := range r.todo("F12").OpenQuestions {
		r.answerQuestion("F12", oq.Label, intPtr(0), "parasoles sí, porque hay horario hasta las 19", http.StatusOK)
	}
	r.allUsed()
}

// H-1 (backlog-r2 2.10, row 2B; 1.4 · B-20 a, T-1): the card of the
// estimated use 40 % answered «ok» does not close: it stays open with
// answer_without_reason, «Diagnóstico» stays in draft and its signoff is
// blocked. Second brake: «ok, dale» does not close it either (B-20 a,
// refined on 31-oct). F3–4 (reader) substituted; cut at F6 (the judge).
func TestH1_Rushed02B_OkDoesNotCloseTheCard(t *testing.T) {
	r := newRushed(t, "caso-02-B")
	r.open("F1")
	r.submit("F2–F5")

	// F5: «ok» to the card of the 40 %.
	card := r.openCardIn("F5", "Diagnóstico")
	r.answerCard("F5", card.Label, "ok", http.StatusUnprocessableEntity)
	if got := r.cardState("F5", card.Label); got != "open" {
		r.fail("F5", "card %s after «ok»: %q, want open", card.Label, got)
	}
	if r.withoutReason("F5", card.Label) != 1 {
		r.fail("F5", "card %s: want one answer_without_reason after «ok»", card.Label)
	}
	if sec := r.section("F5", "Diagnóstico"); sec.Status != "draft" {
		r.fail("F5", "«Diagnóstico» is %q, want draft", sec.Status)
	}
	pendings := r.signoffBlocked("F5", "Diagnóstico")
	if !hasPending(pendings, graph.PendingOpenCard, card.Label) {
		r.fail("F5", "signoff pendings %+v, want the card %s", pendings, card.Label)
	}

	// Second brake (F13's «ok, dale»): assents joined by punctuation are
	// no «porque» either (P-66).
	// [EL DISEÑO NO LO DICE] (2.10): the row cuts at F6 but names F13,
	// whose card (the premise's) comes after the judge; the «ok, dale» goes
	// on the card open at the cut, the 40 %.
	r.answerCard("F13", card.Label, "ok, dale", http.StatusUnprocessableEntity)
	if got := r.cardState("F13", card.Label); got != "open" {
		r.fail("F13", "card %s after «ok, dale»: %q, want open", card.Label, got)
	}
	if r.withoutReason("F13", card.Label) != 2 {
		r.fail("F13", "card %s: want a second answer_without_reason after «ok, dale»", card.Label)
	}
	r.signoffBlocked("F13", "Diagnóstico")
	r.allUsed()
}

// H-1 (backlog-r2 2.10, row 3B; 2.3, 2.4 · fichas §3 and §8.2, R1): the
// agent applies «cambio de control» as «only if the clause allows
// termination»; after the step that writes the table with that reading,
// a «por aclarar» on the criterion blocks the table. At F12 the «ok»
// leaves the card open and choosing (b) without a «porque» leaves the
// «por aclarar» open. F3: without «continúa» (slice 4) the chain stops at
// the stop (2.5); F4–7 (readers) substituted; cut at F10 (the judge).
func TestH1_Rushed03B_ScopeOpenQuestionBlocksTheTable(t *testing.T) {
	const table = "Contratos con cambio de control"
	r := newRushed(t, "caso-03-B")
	r.open("F1")
	r.submit("F2–F11")

	// After the step that writes the table: a «por aclarar» on the
	// criterion that blocks the table.
	q := r.openQuestionBlocking("F9", table)
	body, _ := r.openQuestionObject("F9", q.Label)
	obj, text := r.protectedText("F9", body)
	if obj.Type != store.ObjectCriterion || !strings.Contains(text, "cambio de control") {
		r.fail("F9", "the «por aclarar» %s protects %s %q, want the criterion on «cambio de control»", q.Label, obj.Type, text)
	}
	// F3: no «continúa»: the chain stopped at the stop, with its report.
	var report server.StopReportView
	r.do("F11", http.MethodGet, r.api("/stop-report"), nil, http.StatusOK, &report)

	// F12: «ok» to D-a; (b) chosen without a «porque».
	// [EL DISEÑO NO LO DICE] (2.10): the script's question at F12 is F8's
	// (the unreadable contracts); the choice without a «porque» goes on the
	// row's «por aclarar», the one on the criterion; F8's stays untouched.
	card := r.openCardIn("F12", table)
	r.answerCard("F12", card.Label, "ok", http.StatusUnprocessableEntity)
	if got := r.cardState("F12", card.Label); got != "open" {
		r.fail("F12", "card %s after «ok»: %q, want open", card.Label, got)
	}
	r.answerQuestion("F12", q.Label, intPtr(1), "", http.StatusUnprocessableEntity)
	if _, open := r.openQuestionView("F12", q.Label); !open {
		r.fail("F12", "the «por aclarar» %s closed after choosing without a «porque»", q.Label)
	}
	if r.withoutReason("F12", q.Label) != 1 {
		r.fail("F12", "the «por aclarar» %s: want one answer_without_reason", q.Label)
	}
	pendings := r.signoffBlocked("F12", table)
	if !hasPending(pendings, graph.PendingOpenQuestion, q.Label) || !hasPending(pendings, graph.PendingOpenCard, card.Label) {
		r.fail("F12", "signoff pendings %+v, want the «por aclarar» %s and the card %s", pendings, q.Label, card.Label)
	}
	r.allUsed()
}

// H-1 (backlog-r2 2.10, row 7B; 1.4 · T-1, 2.4, 1.7): at F7 the horizon
// arrives as a «por aclarar» of scope; at F13 the irradiation card
// answered «ok» stays open with answer_without_reason and the signoff of
// the sections that use that decision is blocked. Second brake (row
// corrected by Eber on 31-oct): at F17 the signoff of «Evaluación» stays
// blocked by the open irradiation card, before Wonder (1.7, estados §1).
// F3: without «continúa» (slice 4) the chain stops at the stop; F8–9
// (round and judge, slice 3) are left out.
func TestH1_Rushed07B_IrradiationCardBlocksEvaluacionBeforeWonder(t *testing.T) {
	r := newRushed(t, "caso-07-B")
	r.open("F1")
	// F3's «Continúa hasta terminar» is slice 4's mode: without it the
	// chain that starts after the understanding (B-17) stops at the stop.
	r.submit("F2–F10")

	// F7: the horizon is a «por aclarar» of scope, proposed by the agent.
	q := r.openQuestionBlocking("F7", "Valor después del cierre")
	if q.UndeclaredByAgent {
		r.fail("F7", "the horizon «por aclarar» %s is marked undeclared; the agent proposed it", q.Label)
	}
	// F10: the chain stopped with its report.
	var report server.StopReportView
	r.do("F10", http.MethodGet, r.api("/stop-report"), nil, http.StatusOK, &report)

	// F12: «¿ya está listo para firmar?»; F13: «ok» to the irradiation card.
	r.say("F12", "¿ya está listo para firmar?")
	card := r.openCardIn("F13", "Evaluación")
	r.answerCard("F13", card.Label, "ok", http.StatusUnprocessableEntity)
	if got := r.cardState("F13", card.Label); got != "open" {
		r.fail("F13", "card %s after «ok»: %q, want open", card.Label, got)
	}
	if r.withoutReason("F13", card.Label) != 1 {
		r.fail("F13", "card %s: want one answer_without_reason after «ok»", card.Label)
	}
	for _, sec := range card.Sections {
		pendings := r.signoffBlocked("F13", sec.Title)
		if !hasPending(pendings, graph.PendingOpenCard, card.Label) {
			r.fail("F13", "signoff of %q: pendings %+v, want the card %s", sec.Title, pendings, card.Label)
		}
	}

	// F14: «firma tú lo que esté bien»: nothing is signed.
	r.say("F14", "firma tú lo que esté bien")
	// F15: «la a, 12 años, ok» on the horizon question (a valid «porque»:
	// it adds more than assents, P-66).
	r.answerQuestion("F15", q.Label, intPtr(0), "la a, 12 años, ok", http.StatusOK)
	// F16: he signs the three sections without pendings.
	// [EL DISEÑO NO LO DICE] (2.10): they carry no critical decision, so
	// their signoff is brief, without Wonder (the script has Wonder
	// coinciding); a critical decision would bring a card he never answers
	// with a «porque».
	r.signBrief("F16", "Contexto", "Por qué la mina evalúa una planta propia en vez de la red.")
	r.signBrief("F16", "Demanda de energía", "La mina consume parejo durante el día, según la curva.")
	r.signBrief("F16", "Inversión", "Dos proveedores cotizaron la planta llave en mano.")

	// F17 (second brake): he asks to sign «Evaluación»; the open
	// irradiation card blocks it before Wonder, so no signoff is created
	// and no Wonder round runs.
	pendings := r.signoffBlocked("F17", "Evaluación")
	if !hasPending(pendings, graph.PendingOpenCard, card.Label) {
		r.fail("F17", "signoff of «Evaluación»: pendings %+v, want the irradiation card %s", pendings, card.Label)
	}
	r.allUsed()
}

// H-1 (backlog-r2 2.10, row 8B; 2.7 · B-48, 1.5 · estados §1, 2.3 ·
// fichas §3, B-36, 2.4): ten days after her own edit of Av. Grau on the
// signed «Trazo propuesto», the return line shows her edit as her last
// act and «Trazo» is changed_since_signoff; the checker's output on her
// edit brings a contradiction with a critical decision of another
// section → an open «por aclarar». Second brake: «calle comercial» read
// by the cadastre → «por aclarar». F5–6 (reader) substituted; cut at F10.
func TestH1_Rushed08B_ReturnLineAndHerEditChecked(t *testing.T) {
	const trazo = "Trazo propuesto"
	r := newRushed(t, "caso-08-B")
	// Seed: the earlier sessions, through the same API: the cards answered
	// with a «porque», «Trazo propuesto» signed after Wonder's rounds.
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
	// 14-oct: she edits Av. Grau herself, without the agent.
	sec = r.section("14-oct", trazo)
	edited := strings.Replace(sec.Text, "En Av. Grau la ciclovía es bidireccional en el lado norte.",
		"En Av. Grau la ciclovía va con un carril por sentido, uno a cada lado.", 1)
	r.do("14-oct", http.MethodPut, r.api("/sections/"+sec.Label), map[string]any{"base_version": sec.Version, "text": edited},
		http.StatusOK, nil)

	// F1: ten days later she opens the project.
	r.now = r.now.Add(10 * 24 * time.Hour)
	var frame graph.Frame
	r.do("F1", http.MethodGet, r.api("/frame"), nil, http.StatusOK, &frame)
	if frame.ReturnLine == nil || frame.ReturnLine.Act != "edit" || frame.ReturnLine.Section != sec.Label {
		r.fail("F1", "return line %+v, want her edit of %s as her last act", frame.ReturnLine, sec.Label)
	}
	if got := r.section("F1", trazo).Status; got != "changed_since_signoff" {
		r.fail("F1", "«%s» is %q, want changed_since_signoff", trazo, got)
	}
	r.say("F1", "retomemos donde quedamos")

	// The checker's output on her edit: a contradiction with the critical
	// decision of another section → an open «por aclarar».
	qs := r.checkerQuestionsBlocking("F3", trazo)
	if len(qs) != 1 {
		r.fail("F3", "want one «por aclarar» of the checker blocking «%s», got %+v", trazo, qs)
	}
	body, _ := r.openQuestionObject("F3", qs[0].Label)
	if obj, _ := r.protectedText("F3", body); obj.Type != store.ObjectDecision || body["kind"] != graph.CandidateContradiction {
		r.fail("F3", "the «por aclarar» %s is a %v on %s, want a contradiction with a decision", qs[0].Label, body["kind"], obj.Type)
	}
	// Second brake: «calle comercial» read by the cadastre.
	qs = r.checkerQuestionsBlocking("F7", "Estacionamiento")
	if len(qs) != 1 {
		r.fail("F7", "want one «por aclarar» of the checker blocking «Estacionamiento», got %+v", qs)
	}
	body, _ = r.openQuestionObject("F7", qs[0].Label)
	if _, text := r.protectedText("F7", body); !strings.Contains(text, "calle comercial") || body["kind"] != graph.CandidateDoubtfulTerm {
		r.fail("F7", "the «por aclarar» %s is a %v on %q, want a doubtful term of «calle comercial»", qs[0].Label, body["kind"], text)
	}
	r.allUsed()
}

// H-1 (backlog-r2 2.10, row 9B; 2.3, 2.4, 2.1, B-11): the agent reads
// «pago por uso» as compatible with a prepaid package: a «por aclarar»
// blocks «Modelo de cobro»; at F13 the «ok»s leave it open and at F14
// the signoff is blocked. Second brake (F11): «revisión del martes» stays
// as process_narrative and the dec-n in the text as «códigos internos».
// At F9 the agent's step over her edit is rejected (B-11). F3: «continúa»
// (slice 4) left out; F4–6 (readers) substituted.
func TestH1_Rushed09B_PackageOpenQuestionBlocksTheModel(t *testing.T) {
	const model = "Modelo de cobro"
	r := newRushed(t, "caso-09-B")
	// F9: while the agent rewrites the sentence for the businesses, she
	// edits it herself without looking at what the agent changes.
	r.script.before["06-step-rewrites-her-sentence"] = func() {
		sec := r.section("F9", model)
		text := strings.Replace(sec.Text, "Para los negocios: pagas solo cuando cobras una reserva.",
			"Para los negocios: solo pagas una comisión cuando cobras una reserva, sin cargos fijos.", 1)
		r.do("F9", http.MethodPut, r.api("/sections/"+sec.Label), map[string]any{"base_version": sec.Version, "text": text},
			http.StatusOK, nil)
	}
	r.open("F1")
	r.submit("F2–F12")

	// Tras F8: a «por aclarar» of the checker that blocks «Modelo de cobro».
	qs := r.checkerQuestionsBlocking("F8", model)
	if len(qs) != 1 {
		r.fail("F8", "want one «por aclarar» of the checker blocking «%s», got %+v", model, qs)
	}
	q := qs[0]
	// F9: the agent's step over her edit is a proposal (B-73 a).
	if !r.openProposalOn("F9", r.section("F9", model).Label) {
		r.fail("F9", "no open proposal on «%s» after her edit (B-73 a)", model)
	}
	if sec := r.section("F9", model); !strings.Contains(sec.Text, "sin cargos fijos") {
		r.fail("F9", "her sentence does not rule in «%s»: %q", model, sec.Text)
	}
	// Second brake (F11): process narrative and the internal code.
	sec := r.section("F11", model)
	if !hasPending(sec.Pendings, graph.PendingProcessNarrative, "") || !hasPending(sec.Pendings, graph.PendingInternalCodes, "dec-1") {
		r.fail("F11", "pendings of «%s» %+v, want process_narrative and internal_codes dec-1", model, sec.Pendings)
	}

	// F13: «ok» to everything open.
	for _, oq := range r.todo("F13").OpenQuestions {
		r.answerQuestion("F13", oq.Label, nil, "ok", http.StatusUnprocessableEntity)
	}
	for _, c := range r.todo("F13").Cards {
		r.answerCard("F13", c.Label, "ok", http.StatusUnprocessableEntity)
	}
	if _, open := r.openQuestionView("F13", q.Label); !open || r.withoutReason("F13", q.Label) != 1 {
		r.fail("F13", "the «por aclarar» %s after «ok»: open %v, want open with one answer_without_reason", q.Label, open)
	}
	// F14: «Firmar» on «Modelo de cobro».
	pendings := r.signoffBlocked("F14", model)
	if !hasPending(pendings, graph.PendingOpenQuestion, q.Label) {
		r.fail("F14", "signoff pendings %+v, want the «por aclarar» %s", pendings, q.Label)
	}
	r.allUsed()
}

// H-1 (backlog-r2 2.10, row 10B; 2.3, 2.4 · estados §4, 2.2): from the
// seeded state, «Calidad del río» has an open «por aclarar» (the 31 %
// against the international guide contradicts her confirmed criterion);
// after F2 «firma tú» creates no signoff; after F3 the «ok» leaves it
// open. Second brake: after F6 the card of the 12 % stays open and
// «Conclusiones» is stale with its change. Cut at F11 («termina todo tú»).
func TestH1_Rushed10B_CriterionContradictionAndStaleConclusions(t *testing.T) {
	const river = "Calidad del río receptor"
	r := newRushed(t, "caso-10-B")
	// Seed: the earlier session, through the same API; its report unseen.
	r.open("seed")
	r.submit("seed")

	// F1: «Calidad del río» with an open «por aclarar» on her criterion.
	qs := r.checkerQuestionsBlocking("F1", river)
	if len(qs) != 1 {
		r.fail("F1", "want one «por aclarar» of the checker blocking «%s», got %+v", river, qs)
	}
	q := qs[0]
	body, _ := r.openQuestionObject("F1", q.Label)
	if obj, _ := r.protectedText("F1", body); obj.Type != store.ObjectCriterion || body["kind"] != graph.CandidateContradiction {
		r.fail("F1", "the «por aclarar» %s is a %v on %s, want a contradiction with her criterion", q.Label, body["kind"], obj.Type)
	}

	// F2: «firma tú»: no signoff is created.
	r.say("F2", "Ya revisé todo, firma tú, ya está, mañana lo presento")
	if n := r.count(store.ObjectSignoff); n != 0 {
		r.fail("F2", "«firma tú» created %d signoffs, want none", n)
	}
	// F3: «ok»: the «por aclarar» stays open.
	r.say("F3", "ok")
	if _, open := r.openQuestionView("F3", q.Label); !open {
		r.fail("F3", "the «por aclarar» %s closed after «ok»", q.Label)
	}

	// F6: «ok» to the card of the 12 %: it stays open; «Conclusiones» is
	// stale with its change.
	card := r.openCardIn("F6", river)
	r.answerCard("F6", card.Label, "ok", http.StatusUnprocessableEntity)
	if got := r.cardState("F6", card.Label); got != "open" {
		r.fail("F6", "card %s after «ok»: %q, want open", card.Label, got)
	}
	concl := r.section("F6", "Conclusiones")
	var changes server.ChangesView
	r.do("F6", http.MethodGet, r.api("/sections/"+concl.Label+"/changes"), nil, http.StatusOK, &changes)
	if concl.Status != "stale" || len(changes.Changes) == 0 {
		r.fail("F6", "«Conclusiones» is %q with changes %+v, want stale with its change", concl.Status, changes.Changes)
	}
	r.allUsed()
}
