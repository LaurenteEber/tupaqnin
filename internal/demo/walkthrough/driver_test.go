package walkthrough

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
	"unicode"

	"golang.org/x/text/unicode/norm"

	"github.com/LaurenteEber/tupaqnin/internal/cycle"
	"github.com/LaurenteEber/tupaqnin/internal/graph"
	"github.com/LaurenteEber/tupaqnin/internal/memory"
	"github.com/LaurenteEber/tupaqnin/internal/provider"
	"github.com/LaurenteEber/tupaqnin/internal/server"
	"github.com/LaurenteEber/tupaqnin/internal/store"
)

// What the jury writes: the card's «porque», an explanation without the
// why of the rule (pantallas.md step 6) and the answers to Wonder.
const (
	cardReason = "Porque los estudios con filtro más fino cuentan partículas que los otros no ven."
	// otherCardReason answers any other card that blocks the signoff
	// (P-56).
	otherCardReason = "Porque mi criterio pide solo estudios de campo y así lo revisará el comité."
	explanation     = "La embotellada trae 1,5 veces más partículas que la de grifo, comparando los estudios elegidos."
	originAnswer    = "Del supuesto de que el conteo depende del tamaño mínimo que cada estudio puede ver: con filtros distintos no se cuenta lo mismo."
	caseAnswer      = "Solo los comparo desde el tamaño mínimo que ambos ven; si no, la diferencia sale del filtro y no del agua."
	maxRounds       = 4
	// caseFigure is the datum of case 5: 1,5 times, not 60.
	caseFigure = "1,5"
	// ruleWord marks the comparison rule (P-55), compared without case
	// or accents.
	ruleWord = "micra"
)

// hasRuleWord reports whether text names ruleWord, ignoring case and
// accents.
func hasRuleWord(text string) bool {
	var b strings.Builder
	for _, r := range norm.NFD.String(strings.ToLower(text)) {
		if !unicode.Is(unicode.Mn, r) {
			b.WriteRune(r)
		}
	}
	return strings.Contains(b.String(), ruleWord)
}

// syncStarter runs each turn before the request returns, so every act
// of the walkthrough sees the state its turn left. It records the
// turns' errors for the report.
type syncStarter struct {
	runner *cycle.Runner
	gate   *cycle.Gate
	db     *store.DB
	mu     sync.Mutex
	errs   []string
}

func turnID() string {
	var b [8]byte
	_, _ = rand.Read(b[:])
	return "walk-" + hex.EncodeToString(b[:])
}

func (s *syncStarter) fail(kind string, err error) {
	if err != nil {
		s.mu.Lock()
		s.errs = append(s.errs, kind+": "+err.Error())
		s.mu.Unlock()
	}
}

func (s *syncStarter) StartTurn(projectID, text string) (string, error) {
	id := turnID()
	s.fail("turn", s.runner.RunTurn(context.Background(), projectID, id, text))
	return id, nil
}

func (s *syncStarter) StartUnderstandingTurn(projectID string) (string, error) {
	id := turnID()
	s.fail("understanding", s.runner.RunUnderstandingTurn(context.Background(), projectID, id))
	return id, nil
}

func (s *syncStarter) round(projectID, sign string, record func(context.Context) error) (string, error) {
	ctx := context.Background()
	if err := record(ctx); err != nil {
		return "", err
	}
	view, err := graph.SignoffDetail(ctx, s.db, projectID, sign)
	if err != nil || view.Brief {
		return "", err
	}
	id := turnID()
	s.fail("signoff", s.runner.RunSignoffRound(ctx, projectID, id, sign))
	return id, nil
}

func (s *syncStarter) SignoffExplanation(projectID, sign, text string) (string, error) {
	return s.round(projectID, sign, func(ctx context.Context) error {
		return s.gate.RecordExplanation(ctx, projectID, sign, text)
	})
}

func (s *syncStarter) SignoffAnswers(projectID, sign, origin, answer string) (string, error) {
	return s.round(projectID, sign, func(ctx context.Context) error {
		return s.gate.RecordAnswers(ctx, projectID, sign, origin, answer)
	})
}

func (s *syncStarter) SignoffAsk(string, string, string) (string, error) {
	return "", fmt.Errorf("the walkthrough does not ask the agent")
}

// walk is one run of the walkthrough over a fresh database.
type walk struct {
	t       *testing.T
	h       http.Handler
	db      *store.DB
	dir     string
	starter *syncStarter
	project string
	// lastOutput gives the model's last output for the report when a
	// step fails (backlog-r1 1.8: «en qué paso y con qué salida»).
	lastOutput func() string
}

// newWalk opens a fresh database and builds the provider over it (the
// Meta provider records its calls in that database).
func newWalk(t *testing.T, build func(*store.DB) (provider.Provider, memory.TokenCounter), lastOutput func() string) *walk {
	t.Helper()
	// The database stays when the walkthrough fails, so its run can be
	// inspected (P-50); a passing run removes it.
	dir, err := os.MkdirTemp("", "tupaqnin-walk-")
	if err != nil {
		t.Fatalf("temp dir: %v", err)
	}
	db, err := store.Open(context.Background(), dir)
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	t.Cleanup(func() {
		_ = db.Close()
		if !t.Failed() {
			_ = os.RemoveAll(dir)
		}
	})
	prov, count := build(db)
	starter := &syncStarter{runner: cycle.NewRunner(db, prov, count, nil), gate: cycle.NewGate(db), db: db}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	h := server.NewWithAPI(logger, server.NewHub(), db, starter)
	return &walk{t: t, h: h, db: db, dir: dir, starter: starter, lastOutput: lastOutput}
}

// fail stops the walkthrough naming the step, the turns' errors and
// the model's last output.
func (w *walk) fail(step, format string, args ...any) {
	w.t.Helper()
	msg := fmt.Sprintf(format, args...)
	if len(w.starter.errs) > 0 {
		msg += "\nturn errors: " + strings.Join(w.starter.errs, " · ")
	}
	if w.lastOutput != nil {
		msg += "\nlast model output: " + w.lastOutput()
	}
	msg += w.diagnosis()
	w.t.Fatalf("walkthrough step %s: %s", step, msg)
}

// diagnosis is what the report adds on failure (P-50): each section's
// pendings with their kind and object, what each step_ended did not
// apply and each output_retry's errors, and the kept database.
func (w *walk) diagnosis() string {
	var sb strings.Builder
	ctx := context.Background()
	if w.project != "" {
		sections, _ := graph.OrderedSections(ctx, w.db, w.project)
		sb.WriteString("\npendings:")
		for _, sec := range sections {
			_, pendings, err := graph.SectionStatus(ctx, w.db, w.project, sec.Label)
			if err != nil {
				fmt.Fprintf(&sb, "\n  %s: %v", sec.Label, err)
				continue
			}
			fmt.Fprintf(&sb, "\n  %s: %d", sec.Label, len(pendings))
			for _, p := range pendings {
				fmt.Fprintf(&sb, "\n    - %s %s", p.Kind, p.About)
			}
		}
		events, _ := w.db.EventsSince(ctx, w.project, 0)
		sb.WriteString("\nsteps:")
		for _, ev := range events {
			d := eventData(ev)
			switch ev.Type {
			case store.EventStepEnded:
				na, _ := json.Marshal(d["not_applied"])
				fmt.Fprintf(&sb, "\n  step_ended #%d %v: stop_reason=%v not_applied=%s", ev.Seq, d["turn_kind"], d["stop_reason"], na)
			case store.EventOutputRetry:
				errs, _ := json.Marshal(d["errors"])
				fmt.Fprintf(&sb, "\n  output_retry #%d %v: errors=%s", ev.Seq, d["result"], errs)
			}
		}
	}
	fmt.Fprintf(&sb, "\ndatabase kept in: %s", w.dir)
	return sb.String()
}

func (w *walk) do(step, method, path string, body any, want int, out any) {
	w.t.Helper()
	var r io.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		r = bytes.NewReader(b)
	}
	req := httptest.NewRequest(method, path, r)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	rec := httptest.NewRecorder()
	w.h.ServeHTTP(rec, req)
	if rec.Code != want {
		w.fail(step, "%s %s: got %d %s, want %d", method, path, rec.Code, rec.Body.String(), want)
	}
	if out != nil {
		if err := json.Unmarshal(rec.Body.Bytes(), out); err != nil {
			w.fail(step, "%s %s: decode %q: %v", method, path, rec.Body.String(), err)
		}
	}
}

func (w *walk) api(path string) string { return "/api/projects/" + w.project + path }

// run walks steps 1 to 6 and returns the signed section's label.
func (w *walk) run() string {
	w.t.Helper()
	// 1. «Probar con el caso de demo» (B3-19).
	var created server.CreateProjectResponse
	w.do("1 demo case", http.MethodPost, "/api/projects", map[string]any{"template": "case-5"}, http.StatusCreated, &created)
	w.project = created.ProjectID

	// 2. Paula's request, already filled; sending it starts the
	// understanding and, after it, the chain of steps (B-17).
	var req map[string]any
	w.do("2 request", http.MethodGet, w.api("/request"), nil, http.StatusOK, &req)
	if text, _ := req["text"].(string); !strings.Contains(text, "microplásticos") {
		w.fail("2 request", "the demo request is not filled: %v", req)
	}
	w.do("2 request", http.MethodPost, w.api("/request/submit"), nil, http.StatusAccepted, nil)

	// 3. The understanding: how it applies «campo», a case that counts
	// and one that does not; the jury confirms each case.
	var und server.UnderstandingView
	w.do("3 understanding", http.MethodGet, w.api("/understanding"), nil, http.StatusOK, &und)
	if und.Status != "accepted" || len(und.Cases) == 0 {
		w.fail("3 understanding", "status %q with %d cases, want accepted with cases", und.Status, len(und.Cases))
	}
	counts, notCounts := false, false
	for _, c := range und.Cases {
		if c.Counts != nil && *c.Counts {
			counts = true
		} else if c.Counts != nil {
			notCounts = true
		}
		w.do("3 understanding", http.MethodPost, w.api("/cases/"+c.Label+"/confirm"), nil, http.StatusOK, nil)
	}
	if !counts || !notCounts {
		w.fail("3 understanding", "want a case that counts and one that does not: %+v", und.Cases)
	}

	// 4. The agent worked: the comparison rule is a critical decision,
	// with its card, used in the section whose text has the datum 1,5
	// copied from the calculation (docs/demo/caso-5: «La cifra 1,5 es un
	// dato copiado del cálculo (A-3)»; P-53). A copied datum has no
	// covers edge, so the frame's block is not the way to find it.
	var sections []server.SectionView
	w.do("4 agent works", http.MethodGet, w.api("/sections"), nil, http.StatusOK, &sections)
	var ruleSection server.SectionView
	block := 0
	for _, sec := range sections {
		for i, b := range graph.Blocks(sec.Text) {
			if strings.Contains(b, caseFigure) && block == 0 {
				ruleSection, block = sec, i+1
			}
		}
	}
	if block == 0 {
		w.fail("4 agent works", "the agent wrote %q in no section (the comparison of case 5 is missing from the text)", caseFigure)
	}
	var frame graph.Frame
	w.do("4 agent works", http.MethodGet, w.api("/frame"), nil, http.StatusOK, &frame)
	var rule graph.FrameDecision
	var critical []string
	for _, s := range frame.Sections {
		if s.Label != ruleSection.Label {
			continue
		}
		for _, d := range s.Decisions {
			if d.Card == "" {
				continue
			}
			critical = append(critical, d.Label+": "+d.Statement+" / instead of: "+d.InsteadOf)
			// The comparison rule of case 5 names the particle size
			// (P-55): «micra» in its statement or its contrast.
			if rule.Label == "" && (hasRuleWord(d.Statement) || hasRuleWord(d.InsteadOf)) {
				rule = d
			}
		}
	}
	if rule.Label == "" {
		w.fail("4 agent works", "%s has %q but none of its critical decisions with a card is the comparison rule "+
			"(no %q in its statement or instead_of); critical decisions there: %q", ruleSection.Label, caseFigure, ruleWord, critical)
	}

	// 5. The card: an option and a «porque»; then the history of the
	// block with the figure (pantallas, step 5: «hace clic en la cifra y
	// ve su historia»).
	w.do("5 card", http.MethodPost, w.api("/cards/"+rule.Card+"/answer"),
		map[string]any{"option": "keep", "reason": cardReason}, http.StatusOK, nil)
	var history server.BlockHistoryView
	w.do("5 history", http.MethodGet, w.api(fmt.Sprintf("/sections/%s/history?block=%d", ruleSection.Label, block)),
		nil, http.StatusOK, &history)
	if len(history.Entries) == 0 {
		w.fail("5 history", "the history of block %d of %s is empty", block, ruleSection.Label)
	}

	// 6. «Firmar sección»: an explanation without the why of the rule;
	// Wonder names what is missing and brings a case; the jury answers
	// until the signoff coincides, and signs.
	// Every other open card that blocks the section is answered first,
	// as the jury would from «Te toca» (P-56): the app blocks the
	// signoff while a decision the section uses has an open card.
	var detail server.SectionDetail
	w.do("6 open cards", http.MethodGet, w.api("/sections/"+ruleSection.Label), nil, http.StatusOK, &detail)
	for _, p := range detail.Pendings {
		if p.Kind == graph.PendingOpenCard {
			w.do("6 open cards", http.MethodPost, w.api("/cards/"+p.About+"/answer"),
				map[string]any{"option": "keep", "reason": otherCardReason}, http.StatusOK, nil)
		}
	}
	var view graph.SignoffView
	w.do("6 signoff", http.MethodPost, w.api("/sections/"+ruleSection.Label+"/signoff"), nil, http.StatusCreated, &view)
	sign := view.Label
	w.do("6 signoff", http.MethodPost, w.api("/signoffs/"+sign+"/explanation"), map[string]any{"text": explanation}, http.StatusAccepted, nil)
	for round := 1; ; round++ {
		w.do("6 signoff", http.MethodGet, w.api("/signoffs/"+sign), nil, http.StatusOK, &view)
		if view.CanSign {
			break
		}
		if view.Phase != graph.SignoffInProgress || round >= maxRounds {
			w.fail("6 signoff", "round %d: phase %q, cannot sign: %+v", round, view.Phase, view)
		}
		w.do("6 signoff", http.MethodPost, w.api("/signoffs/"+sign+"/answers"),
			map[string]any{"origin_answer": originAnswer, "case_answer": caseAnswer}, http.StatusAccepted, nil)
	}
	w.do("6 sign", http.MethodPost, w.api("/signoffs/"+sign+"/sign"), nil, http.StatusOK, nil)
	var section server.SectionDetail
	w.do("6 sign", http.MethodGet, w.api("/sections/"+ruleSection.Label), nil, http.StatusOK, &section)
	if section.Status != "signed" {
		w.fail("6 sign", "section %s: status %q, want signed", ruleSection.Label, section.Status)
	}
	return ruleSection.Label
}

// orderedEvents checks that the history holds, in order, the events
// of each step of the walkthrough.
func (w *walk) orderedEvents(want []func(store.Event) bool, names []string) {
	w.t.Helper()
	events, err := w.db.EventsSince(context.Background(), w.project, 0)
	if err != nil {
		w.t.Fatalf("EventsSince: %v", err)
	}
	i := 0
	for _, ev := range events {
		if i < len(want) && want[i](ev) {
			i++
		}
	}
	if i < len(want) {
		var got []string
		for _, ev := range events {
			got = append(got, string(ev.Type))
		}
		w.t.Fatalf("history: missing %q in order (found %d of %d); events: %s", names[i], i, len(want), strings.Join(got, " "))
	}
}

func eventData(ev store.Event) map[string]any {
	var d map[string]any
	_ = json.Unmarshal(ev.Data, &d)
	return d
}

func is(t store.EventType, actor string, match func(map[string]any) bool) func(store.Event) bool {
	return func(ev store.Event) bool {
		return ev.Type == t && (actor == "" || ev.Actor == actor) && (match == nil || match(eventData(ev)))
	}
}

func field(key string, want any) func(map[string]any) bool {
	return func(d map[string]any) bool { return d[key] == want }
}

// stepEvents are the events each step of the walkthrough leaves, in
// order: the request, the understanding, the agent's text and its card,
// the end of the chain, the answered card, the history seen, the
// signoff with a round that names what is missing, a round that
// coincides, and the signed section.
func stepEvents(signed string) ([]func(store.Event) bool, []string) {
	return []func(store.Event) bool{
		is(store.EventHumanMessage, "human", nil),
		is(store.EventStepEnded, "agent", field("turn_kind", "understanding")),
		is(store.EventEdit, "agent", nil),
		is(store.EventStateChanged, "app", field("to", "open")),
		is(store.EventStepEnded, "agent", func(d map[string]any) bool { return d["stop_reason"] != nil }),
		is(store.EventDecisionSeen, "human", field("seen_via", "card")),
		is(store.EventDecisionSeen, "human", field("seen_via", "history")),
		is(store.EventStateChanged, "human", field("signoff_step", "requested")),
		is(store.EventSignoffCase, "agent", field("coincides", false)),
		is(store.EventSignoffCase, "agent", field("coincides", true)),
		is(store.EventStateChanged, "human", func(d map[string]any) bool { return d["label"] == signed && d["to"] == "signed" }),
	}, []string{
		"1-2 request (human_message)", "3 understanding (step_ended)", "4 agent text (edit)", "4 card opened (state_changed)",
		"4 chain stopped (step_ended with stop_reason)", "5 card answered (decision_seen card)", "5 history (decision_seen history)",
		"6 signoff requested", "6 Wonder names what is missing (signoff_case, not coinciding)",
		"6 signoff coincides (signoff_case)", "6 section signed (state_changed)",
	}
}
