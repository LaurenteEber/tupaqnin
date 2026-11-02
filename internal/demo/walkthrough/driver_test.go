package walkthrough

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
	"time"
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
	// keepReason answers a «por aclarar» keeping what was decided (P-79).
	keepReason = "Porque la regla de comparación ya está decidida y el texto debe seguirla."
	// acceptReason accepts a paragraph «sin verificar» (R-1, 2.11).
	acceptReason = "Porque lo comparé con las fuentes del caso y lo sostengo yo."
	explanation  = "La embotellada trae 1,5 veces más partículas que la de grifo, comparando los estudios elegidos."
	originAnswer = "Del supuesto de que el conteo depende del tamaño mínimo que cada estudio puede ver: con filtros distintos no se cuenta lo mismo."
	caseAnswer   = "Solo los comparo desde el tamaño mínimo que ambos ven; si no, la diferencia sale del filtro y no del agua."
	maxRounds    = 4
	// turnWait bounds how long an act waits for its background turn; real
	// model calls take minutes (make demo-check).
	turnWait = 20 * time.Minute
	// caseFigure is the datum of case 5: 1,5 times, not 60.
	caseFigure = "1,5"
	// ruleWord marks the comparison rule (P-55), compared without case
	// or accents.
	ruleWord = "micra"
)

// hasRuleWord reports whether text names ruleWord, ignoring case and
// accents.
func hasRuleWord(text string) bool { return namesWord(text, ruleWord) }

// namesWord reports whether text contains word, ignoring case and
// accents (P-82).
func namesWord(text, word string) bool {
	fold := func(s string) string {
		var b strings.Builder
		for _, r := range norm.NFD.String(strings.ToLower(s)) {
			if !unicode.Is(unicode.Mn, r) {
				b.WriteRune(r)
			}
		}
		return b.String()
	}
	return strings.Contains(fold(text), fold(word))
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

// waitTurn waits for the turn a round or an audit left running in the
// background, so the next act sees the state it left. The runner has no
// «is a turn running» to ask: a «Buscarla» on candidate 0 needs the turn
// slot first, answers ErrTurnRunning while one runs and otherwise refuses
// without writing anything.
func (s *syncStarter) waitTurn(projectID string) error {
	deadline := time.Now().Add(turnWait)
	for time.Now().Before(deadline) {
		_, _, err := s.runner.StartCandidateRound(projectID, []int{0})
		if !errors.Is(err, cycle.ErrTurnRunning) {
			return nil
		}
		time.Sleep(5 * time.Millisecond)
	}
	return fmt.Errorf("the turn did not end in %s", turnWait)
}

// The round acts (server.RoundRunner): the runner's, each one waiting for
// its turn, as StartTurn does.

func (s *syncStarter) Round(ctx context.Context, projectID, label string) (cycle.RoundView, error) {
	return s.runner.Round(ctx, projectID, label)
}

func (s *syncStarter) StartRound(projectID string) (string, string, error) {
	label, turn, err := s.runner.StartRound(projectID)
	if err == nil {
		err = s.waitTurn(projectID)
	}
	return label, turn, err
}

func (s *syncStarter) StartCandidateRound(projectID string, references []int) (string, string, error) {
	label, turn, err := s.runner.StartCandidateRound(projectID, references)
	if err == nil {
		err = s.waitTurn(projectID)
	}
	return label, turn, err
}

func (s *syncStarter) RepeatRound(projectID, label, question string) (string, string, float64, error) {
	newLabel, turn, cost, err := s.runner.RepeatRound(projectID, label, question)
	if err == nil {
		err = s.waitTurn(projectID)
	}
	return newLabel, turn, cost, err
}

func (s *syncStarter) LeaveRoundOpen(ctx context.Context, projectID, label string) (string, error) {
	return s.runner.LeaveRoundOpen(ctx, projectID, label)
}

func (s *syncStarter) AuditSection(projectID, section string) (string, string, error) {
	label, turn, err := s.runner.AuditSection(projectID, section)
	if err == nil {
		err = s.waitTurn(projectID)
	}
	return label, turn, err
}

func (s *syncStarter) DiscardFlaw(ctx context.Context, projectID, label string, n int) error {
	return s.runner.DiscardFlaw(ctx, projectID, label, n)
}

// withVisitor is h with one visitor of db: a link and a session in the
// store, and the cookie on every request. The owner check runs as ever
// (B-87); the session lasts a year because a test's clock moves.
func withVisitor(t *testing.T, db *store.DB, h http.Handler) http.Handler {
	t.Helper()
	ctx := context.Background()
	now := time.Now()
	_, tokenHash, err := store.NewSecret()
	if err != nil {
		t.Fatalf("NewSecret: %v", err)
	}
	link, err := db.CreateInvitation(ctx, store.InvitationInput{Label: "test", BudgetUSD: 30, MaxAccounts: 10, MaxAnonProjects: 1000}, tokenHash, now)
	if err != nil {
		t.Fatalf("CreateInvitation: %v", err)
	}
	cookie, hash, err := store.NewSecret()
	if err != nil {
		t.Fatalf("NewSecret: %v", err)
	}
	if err := db.CreateSession(ctx, hash, link, now.Add(365*24*time.Hour), now); err != nil {
		t.Fatalf("CreateSession: %v", err)
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.AddCookie(&http.Cookie{Name: "__Host-tq_session", Value: cookie})
		h.ServeHTTP(w, r)
	})
}

// walk is one run of the walkthrough over a fresh database.
type walk struct {
	t       *testing.T
	h       http.Handler
	db      *store.DB
	dir     string
	starter *syncStarter
	project string
	// byGoal is the Meta walkthrough: its steps go by goal (P-110). The fake's
	// recordings fix the order, so its walkthrough keeps its fixed steps.
	byGoal bool
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
	h := withVisitor(t, db, server.NewWithAPI(logger, server.NewHub(), db, starter))
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

// toSignoff walks steps 1 to 5, opens the signoff of the section with
// the comparison rule and sends the explanation (step 6 up to Wonder's
// first round). It returns the section's and the signoff's labels, and
// whether the history of the block with the figure showed a decision.
func (w *walk) toSignoff() (string, string, bool) {
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
	if w.byGoal {
		// P-110: with Meta the model sets the order: go on until the goal.
		// The rule's own card waits for step 5.
		w.reachGoal("4 agent works", func() bool {
			_, block, rule, _, _ := w.findRuleByRole()
			return block != 0 && rule.Label != ""
		}, func(c server.CardView) bool { return isComparisonRule(c.Statement, c.InsteadOf) })
	}
	figureSection, block, rule, critical := w.findRule()
	signSection := figureSection.Label
	if w.byGoal {
		// P-111: the rule is found by its role, in any section.
		figureSection, block, rule, signSection, critical = w.findRuleByRole()
	}
	if block == 0 {
		w.fail("4 agent works", "the agent wrote %q in no section (the comparison of case 5 is missing from the text)", caseFigure)
	}
	if rule.Label == "" {
		w.fail("4 agent works", "%s has %q but none of the critical decisions with a card is the comparison rule; "+
			"critical decisions there: %q", figureSection.Label, caseFigure, critical)
	}

	// 5. The card: an option and a «porque»; then the history of the
	// block with the figure (pantallas, step 5: «hace clic en la cifra y
	// ve su historia»).
	w.do("5 card", http.MethodPost, w.api("/cards/"+rule.Card+"/answer"),
		map[string]any{"option": "keep", "reason": cardReason}, http.StatusOK, nil)
	var history server.BlockHistoryView
	w.do("5 history", http.MethodGet, w.api(fmt.Sprintf("/sections/%s/history?block=%d", figureSection.Label, block)),
		nil, http.StatusOK, &history)
	if len(history.Entries) == 0 {
		w.fail("5 history", "the history of block %d of %s is empty", block, figureSection.Label)
	}
	// decision_seen with history is recorded only for each decision the
	// block's history shows (M-1); a copied datum has none (A-3, P-58).
	historyDecision := false
	for _, e := range history.Entries {
		if e.Kind == graph.EntryDecision {
			historyDecision = true
		}
	}

	// 6. «Firmar sección»: an explanation without the why of the rule;
	// Wonder names what is missing and brings a case.
	// Every other open card that blocks the section is answered first,
	// as the jury would from «Te toca» (P-56): the app blocks the
	// signoff while a decision the section uses has an open card.
	if w.byGoal {
		// P-110: the agent's own pendings on the section block the signoff
		// and only the agent closes them.
		w.reachGoal("6 agent pendings", func() bool {
			var d server.SectionDetail
			w.do("6 agent pendings", http.MethodGet, w.api("/sections/"+signSection), nil, http.StatusOK, &d)
			for _, p := range d.Pendings {
				if !yoursToAnswer(p.Kind) {
					return false
				}
			}
			return true
		}, nil)
	}
	var detail server.SectionDetail
	w.do("6 open cards", http.MethodGet, w.api("/sections/"+signSection), nil, http.StatusOK, &detail)
	for _, p := range detail.Pendings {
		switch p.Kind {
		case graph.PendingOpenCard:
			w.do("6 open cards", http.MethodPost, w.api("/cards/"+p.About+"/answer"),
				map[string]any{"option": "keep", "reason": otherCardReason}, http.StatusOK, nil)
		case graph.PendingOpenQuestion:
			// P-79: an open «por aclarar» blocks the signoff too (2.4); the
			// jury answers it from «Te toca», keeping what was decided or
			// the confirmed criterion, with a «porque» (P-109).
			w.answerOpenQuestion(p.About)
		case graph.PendingUnsourcedClaim:
			// [EL DISEÑO NO LO DICE] (2.11): the slice-1 recording of case 5
			// proposes evidence_required but writes without reading or
			// citing, so R-1 leaves its paragraphs «sin verificar». The
			// jury accepts them with a «porque» (R-1); piece 2.10 records
			// the slice-2 stretch of case 5 again.
			w.do("6 unsourced claims", http.MethodPost, w.api("/sections/"+signSection+"/claims/accept"),
				map[string]any{"passage": p.About, "reason": acceptReason}, http.StatusNoContent, nil)
		}
	}
	var view graph.SignoffView
	w.do("6 signoff", http.MethodPost, w.api("/sections/"+signSection+"/signoff"), nil, http.StatusCreated, &view)
	w.do("6 signoff", http.MethodPost, w.api("/signoffs/"+view.Label+"/explanation"), map[string]any{"text": explanation}, http.StatusAccepted, nil)
	return signSection, view.Label, historyDecision
}

// wonderFirstRound checks Wonder's first round (P-57): at least one
// critical decision is missing in the explanation and a new case waits
// for an answer. Meta's run stops here; a person signs in the browser.
func (w *walk) wonderFirstRound(sign string) {
	w.t.Helper()
	var view graph.SignoffView
	w.do("6 Wonder first round", http.MethodGet, w.api("/signoffs/"+sign), nil, http.StatusOK, &view)
	if view.Phase != graph.SignoffInProgress || view.Rounds != 1 {
		w.fail("6 Wonder first round", "phase %q after %d rounds, want in progress after 1: %+v", view.Phase, view.Rounds, view)
	}
	critical := map[string]bool{}
	for _, d := range view.CriticalDecisions {
		critical[d] = true
	}
	missing := false
	for _, v := range view.Verdicts {
		if v.Verdict == "missing_in_explanation" && critical[v.Decision] {
			missing = true
		}
	}
	if !missing || view.OpenCase == "" {
		w.fail("6 Wonder first round", "want a critical decision missing_in_explanation and a new case; "+
			"critical %q, verdicts %+v, open case %q", view.CriticalDecisions, view.Verdicts, view.OpenCase)
	}
}

// findRule looks for step 4's goal: the section whose text has the datum
// 1,5, its block, and the comparison rule among its critical decisions with
// a card, whose statements it returns for the report.
func (w *walk) findRule() (server.SectionView, int, graph.FrameDecision, []string) {
	w.t.Helper()
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
		return ruleSection, 0, graph.FrameDecision{}, nil
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
	return ruleSection, block, rule, critical
}

// isComparisonRule says a critical decision is case 5's comparison rule by
// what it says, in the words a model may use (P-111): it compares, and it names
// the particle size. Accents and case do not count.
func isComparisonRule(statement, insteadOf string) bool {
	text := statement + " " + insteadOf
	if !namesWord(text, "compar") {
		return false
	}
	for _, size := range []string{"micra", "mínimo", "µm", "μm"} {
		if namesWord(text, size) {
			return true
		}
	}
	return false
}

// ruleByRole is the comparison rule among the decisions with a card of any
// section, the section that uses it, and the statements it looked at.
func ruleByRole(frame graph.Frame) (graph.FrameDecision, string, []string) {
	var rule graph.FrameDecision
	section := ""
	critical := []string{}
	for _, s := range frame.Sections {
		for _, d := range s.Decisions {
			if d.Card == "" {
				continue
			}
			critical = append(critical, s.Label+" "+d.Label+": "+d.Statement+" / instead of: "+d.InsteadOf)
			if rule.Label == "" && isComparisonRule(d.Statement, d.InsteadOf) {
				rule, section = d, s.Label
			}
		}
	}
	return rule, section, critical
}

// findRuleByRole is step 4's goal on the Meta walkthrough (P-111): the block
// with 1,5, a copied datum that needs no decision there (A-3), and the rule
// wherever the model used it, with the section to sign.
func (w *walk) findRuleByRole() (server.SectionView, int, graph.FrameDecision, string, []string) {
	w.t.Helper()
	figureSection, block, _, _ := w.findRule()
	var frame graph.Frame
	w.do("4 agent works", http.MethodGet, w.api("/frame"), nil, http.StatusOK, &frame)
	rule, section, critical := ruleByRole(frame)
	return figureSection, block, rule, section, critical
}

// maxGoalRounds and goOnMessage are the Meta walkthrough's way forward
// (P-110): the model sets the order of the work, so a step answers what is
// in «Te toca» and asks to go on until its goal holds.
const (
	maxGoalRounds = 4
	goOnMessage   = "Sigue"
)

// reachGoal runs up to maxGoalRounds rounds of «answer what is yours, send
// Sigue, wait for the chain» until goal holds, logging each round, and says
// whether it held. A card skipCard names stays open for its own step. The
// step that needs the goal decides what a miss means.
func (w *walk) reachGoal(step string, goal func() bool, skipCard func(server.CardView) bool) bool {
	w.t.Helper()
	for round := 1; round <= maxGoalRounds; round++ {
		if goal() {
			w.t.Logf("%s: goal holds before round %d", step, round)
			return true
		}
		answered := w.answerTodo(step, skipCard)
		w.say(step, goOnMessage)
		w.t.Logf("%s: round %d of %d: answered %v, sent %q", step, round, maxGoalRounds, answered, goOnMessage)
	}
	held := goal()
	w.t.Logf("%s: goal holds after %d rounds: %v", step, maxGoalRounds, held)
	return held
}

// say sends a message once no turn runs (a background defense takes the
// slot, P-90) and returns when its chain ends.
func (w *walk) say(step, text string) {
	w.t.Helper()
	if err := w.starter.waitTurn(w.project); err != nil {
		w.fail(step, "%v", err)
	}
	w.do(step, http.MethodPost, w.api("/messages"), map[string]any{"text": text}, http.StatusAccepted, nil)
}

// answerTodo answers what «Te toca» lists: each open question with the
// P-109 chooser, each card with an option and a «porque». It returns the
// labels it answered, for the log.
func (w *walk) answerTodo(step string, skipCard func(server.CardView) bool) []string {
	w.t.Helper()
	var todo server.TodoView
	w.do(step, http.MethodGet, w.api("/todo"), nil, http.StatusOK, &todo)
	answered := []string{}
	for _, q := range todo.OpenQuestions {
		option, reason := openQuestionAnswer(q.Options)
		w.do(step, http.MethodPost, w.api("/open-questions/"+q.Label+"/answer"),
			map[string]any{"option": option, "reason": reason}, http.StatusOK, nil)
		answered = append(answered, q.Label)
	}
	for _, c := range todo.Cards {
		if skipCard != nil && skipCard(c) {
			continue
		}
		var option any = "keep"
		if c.OwnEdit { // it asks only for the «porque» (B-49)
			option = nil
		}
		w.do(step, http.MethodPost, w.api("/cards/"+c.Label+"/answer"),
			map[string]any{"option": option, "reason": otherCardReason}, http.StatusOK, nil)
		answered = append(answered, c.Label)
	}
	return answered
}

// yoursToAnswer says a pending is one the jury answers on its own at step 6,
// not one only the agent can close.
func yoursToAnswer(kind string) bool {
	switch kind {
	case graph.PendingOpenCard, graph.PendingOpenQuestion, graph.PendingUnsourcedClaim:
		return true
	}
	return false
}

// answerOpenQuestion answers an open «por aclarar» from «Te toca» with a
// «porque» (P-79, P-109).
func (w *walk) answerOpenQuestion(label string) {
	var todo server.TodoView
	w.do("6 open questions", http.MethodGet, w.api("/todo"), nil, http.StatusOK, &todo)
	for _, q := range todo.OpenQuestions {
		if q.Label != label {
			continue
		}
		option, reason := openQuestionAnswer(q.Options)
		w.do("6 open questions", http.MethodPost, w.api("/open-questions/"+label+"/answer"),
			map[string]any{"option": option, "reason": reason}, http.StatusOK, nil)
		return
	}
	w.fail("6 open questions", "«por aclarar» %s blocks the signoff but is not in «Te toca»", label)
}

// keepsCriterion are the words by which a variant says it keeps the confirmed
// criterion, «Solo estudios de campo, no de laboratorio»: it leaves out what
// is not of the field. The variants are the model's own, so they vary.
var keepsCriterion = []string{"fuera", "excluy", "excluir", "excluí", "no cuenta", "no contar", "no incluir",
	"no es de campo", "no son de campo", "solo de campo", "solo estudios de campo", "solo campo", "sin el piloto", "sin pilotos"}

// openQuestionAnswer picks the option the walkthrough answers with (P-109).
// The app adds «mantener» only when the question protects the premise or a
// decision (2.4); one about the sense of your own term carries variants:
// then the variant that keeps the criterion, by its text, or else the first.
func openQuestionAnswer(options []server.OpenQuestionOption) (int, string) {
	for i, opt := range options {
		if opt.Keep {
			return i, keepReason
		}
	}
	for i, opt := range options {
		for _, word := range keepsCriterion {
			if namesWord(opt.Option, word) {
				return i, otherCardReason
			}
		}
	}
	return 0, otherCardReason
}

// signAfterRounds answers Wonder until the signoff coincides, signs
// and checks the section is signed (the fake provider's full run).
func (w *walk) signAfterRounds(section, sign string) {
	w.t.Helper()
	var view graph.SignoffView
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
	var detail server.SectionDetail
	w.do("6 sign", http.MethodGet, w.api("/sections/"+section), nil, http.StatusOK, &detail)
	if detail.Status != "signed" {
		w.fail("6 sign", "section %s: status %q, want signed", section, detail.Status)
	}
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

// afterFirstRound is how many of stepEvents come after Wonder's first
// round: the round that coincides and the signed section.
const afterFirstRound = 2

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
// coincides, and the signed section. The history seen counts only when
// the block's history showed a decision (P-58). The run with Meta stops
// after Wonder's first round, so it leaves out the last afterFirstRound.
// chainStopped is where the chain stops (P-82): since slice 2 a stop
// ends with a stop_report, which carries the reason (B-37, refined on
// 31-oct); a step_ended with stop_reason (no_work, step_limit) still
// counts.
func chainStopped(ev store.Event) bool {
	if ev.Type == store.EventStopReport {
		return true
	}
	return is(store.EventStepEnded, "agent", func(d map[string]any) bool { return d["stop_reason"] != nil })(ev)
}

func stepEvents(signed string, historyDecision bool) ([]func(store.Event) bool, []string) {
	want := []func(store.Event) bool{
		is(store.EventHumanMessage, "human", nil),
		is(store.EventStepEnded, "agent", field("turn_kind", "understanding")),
		is(store.EventEdit, "agent", nil),
		is(store.EventStateChanged, "app", field("to", "open")),
		chainStopped,
		is(store.EventDecisionSeen, "human", field("seen_via", "card")),
		is(store.EventDecisionSeen, "human", field("seen_via", "history")),
		is(store.EventStateChanged, "human", field("signoff_step", "requested")),
		is(store.EventSignoffCase, "agent", field("coincides", false)),
		is(store.EventSignoffCase, "agent", field("coincides", true)),
		is(store.EventStateChanged, "human", func(d map[string]any) bool { return d["label"] == signed && d["to"] == "signed" }),
	}
	names := []string{
		"1-2 request (human_message)", "3 understanding (step_ended)", "4 agent text (edit)", "4 card opened (state_changed)",
		"4 chain stopped (stop_report, or step_ended with stop_reason)", "5 card answered (decision_seen card)", "5 history (decision_seen history)",
		"6 signoff requested", "6 Wonder names what is missing (signoff_case, not coinciding)",
		"6 signoff coincides (signoff_case)", "6 section signed (state_changed)",
	}
	if !historyDecision {
		const seenHistory = 6
		want = append(want[:seenHistory], want[seenHistory+1:]...)
		names = append(names[:seenHistory], names[seenHistory+1:]...)
	}
	return want, names
}
