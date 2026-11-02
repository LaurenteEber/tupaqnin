package walkthrough

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/LaurenteEber/tupaqnin/internal/cycle"
	"github.com/LaurenteEber/tupaqnin/internal/graph"
	"github.com/LaurenteEber/tupaqnin/internal/provider"
	"github.com/LaurenteEber/tupaqnin/internal/server"
	"github.com/LaurenteEber/tupaqnin/internal/store"
)

// The rushed profile's walkthroughs (backlog-r2 2.10): one test per
// script of profile B, with the fake provider and hand-made outputs that
// reproduce what the agent does in the script, the human answering as in
// the script. Each test asserts the brake of the table «Frenos esperados»
// and is cut where the table says. The outputs live in
// testdata/rushed/<case>/, one file per call in the order the calls
// happen, and are never Meta's.

const rushedDir = "testdata/rushed"

// handMade is one hand-made output: the fixed output of a principal
// step, the understanding, Wonder or the checker (output), or the tool
// calls of a step (tool_calls).
type handMade struct {
	TurnKind provider.TurnKind `json:"turn_kind"`
	// Script names the row of the script the output reproduces.
	Script string `json:"script"`
	// When, on a checker output: a text the checker's brief must hold for
	// the output to answer that call. A checker call whose brief does not
	// hold it gets the fake's «no» on every probe (provider.Fake) and the
	// output waits for its call.
	When string `json:"when,omitempty"`
	// Part, on a reader's output: the part of the round it answers, so two
	// readers running at once each get theirs (0: whichever calls first).
	Part      int             `json:"part,omitempty"`
	Output    json.RawMessage `json:"output,omitempty"`
	ToolCalls []struct {
		Name      string          `json:"name"`
		Arguments json.RawMessage `json:"arguments"`
	} `json:"tool_calls,omitempty"`
	name string
}

// scripted serves the hand-made outputs of one walkthrough in order, each
// kind of turn with its own queue. A step, the understanding or a Wonder
// round without an output left fails the call (the walkthrough reports
// it); a checker, stop report or defense call without one gets the fake's
// plain answer. On a checker output, every probe of the call it does not
// judge gets «no», as the fake's plain answer does.
type scripted struct {
	t      *testing.T
	db     *store.DB
	mu     sync.Mutex
	queue  map[provider.TurnKind][]handMade
	last   string
	served []string
	calls  int
	// before runs a human act while the call of the named output is in
	// flight, as when the person edits while the agent writes (9B, F9).
	before map[string]func()
}

func loadScript(t *testing.T, name string) *scripted {
	t.Helper()
	files, err := filepath.Glob(filepath.Join(rushedDir, name, "*.json"))
	if err != nil || len(files) == 0 {
		t.Fatalf("hand-made outputs of %s: %v (%d files)", name, err, len(files))
	}
	sort.Strings(files)
	s := &scripted{t: t, queue: map[provider.TurnKind][]handMade{}, before: map[string]func(){}}
	for _, f := range files {
		if filepath.Base(f) == "request.json" {
			continue
		}
		b, err := os.ReadFile(f) //nolint:gosec // test data path
		if err != nil {
			t.Fatalf("read %s: %v", f, err)
		}
		var hm handMade
		if err := json.Unmarshal(b, &hm); err != nil {
			t.Fatalf("parse %s: %v", f, err)
		}
		if hm.TurnKind == "" || (len(hm.Output) == 0) == (len(hm.ToolCalls) == 0) {
			t.Fatalf("%s: a hand-made output needs turn_kind and either output or tool_calls", f)
		}
		hm.name = strings.TrimSuffix(filepath.Base(f), ".json")
		s.queue[hm.TurnKind] = append(s.queue[hm.TurnKind], hm)
	}
	return s
}

// inputText joins every string of a request's input items, so When is
// matched against the brief as the model reads it.
func inputText(input []json.RawMessage) string {
	var sb strings.Builder
	var walk func(v any)
	walk = func(v any) {
		switch x := v.(type) {
		case string:
			sb.WriteString(x)
			sb.WriteString("\n")
		case []any:
			for _, e := range x {
				walk(e)
			}
		case map[string]any:
			for _, e := range x {
				walk(e)
			}
		}
	}
	for _, item := range input {
		var v any
		if json.Unmarshal(item, &v) == nil {
			walk(v)
		}
	}
	return sb.String()
}

func messageItem(text string) json.RawMessage {
	b, _ := json.Marshal(map[string]any{"type": "message", "role": "assistant",
		"content": []map[string]string{{"type": "output_text", "text": text}}})
	return b
}

// fixture turns a hand-made output into the fake's recording for req.
func (s *scripted) fixture(hm handMade, req provider.Request) (provider.Fixture, error) {
	fx := provider.Fixture{Name: hm.name, TurnKind: hm.TurnKind}
	if len(hm.ToolCalls) > 0 {
		for i, c := range hm.ToolCalls {
			s.calls++
			item, _ := json.Marshal(map[string]any{"type": "function_call", "call_id": fmt.Sprintf("call-%d", s.calls),
				"name": c.Name, "arguments": string(c.Arguments)})
			_ = i
			fx.Items = append(fx.Items, item)
		}
		return fx, nil
	}
	out := hm.Output
	if hm.TurnKind == provider.TurnChecker {
		var judged struct {
			Probes     []map[string]any `json:"probes"`
			Candidates json.RawMessage  `json:"candidates"`
		}
		if err := json.Unmarshal(out, &judged); err != nil {
			return fx, fmt.Errorf("%s: %w", hm.name, err)
		}
		seen := map[string]bool{}
		for _, p := range judged.Probes {
			seen[fmt.Sprint(p["protected"])] = true
		}
		for _, l := range req.ProbeLabels {
			if !seen[l] {
				judged.Probes = append(judged.Probes, map[string]any{"protected": l, "verdict": "no", "passage": nil})
			}
		}
		out, _ = json.Marshal(judged)
	}
	var compact bytes.Buffer
	if err := json.Compact(&compact, out); err != nil {
		return fx, fmt.Errorf("%s: %w", hm.name, err)
	}
	fx.Items = []json.RawMessage{messageItem(compact.String())}
	return fx, nil
}

func (s *scripted) Respond(ctx context.Context, req provider.Request, onDelta func(provider.Delta)) (provider.Response, error) {
	s.mu.Lock()
	var fake *provider.Fake
	var hook func()
	q := s.queue[req.TurnKind]
	next := 0
	for next < len(q) && q[next].Part != 0 && q[next].Part != req.Part {
		next++
	}
	if next < len(q) && (q[next].When == "" || strings.Contains(inputText(req.Input), q[next].When)) {
		hm := q[next]
		fx, err := s.fixture(hm, req)
		if err != nil {
			s.mu.Unlock()
			return provider.Response{}, err
		}
		s.queue[req.TurnKind] = append(append([]handMade{}, q[:next]...), q[next+1:]...)
		s.served = append(s.served, hm.name)
		s.last = fmt.Sprintf("%s (%s)", hm.name, hm.Script)
		hook = s.before[hm.name]
		fake = provider.NewFake(fx)
	} else {
		// The fake's plain answer for a checker, stop report or defense
		// call; any other kind fails with no_fixtures.
		fake = provider.NewFake()
		s.last = fmt.Sprintf("%s call with no hand-made output left (plain fake answer)", req.TurnKind)
	}
	fake.Recorder = s.db
	s.mu.Unlock()
	if hook != nil {
		hook()
	}
	return fake.Respond(ctx, req, onDelta)
}

func (s *scripted) lastOutput() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return fmt.Sprintf("%s\nhand-made outputs served: %q", s.last, s.served)
}

// left names the hand-made outputs no call used.
func (s *scripted) left() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []string
	for _, q := range s.queue {
		for _, hm := range q {
			out = append(out, hm.name)
		}
	}
	sort.Strings(out)
	return out
}

// rushedStarter is the walkthrough's synchronous starter with what the
// runner adds in the app: asking to sign runs the checker's pending batch
// first (B-36, P-67), and answering a «por aclarar» probes a changed
// premise (B-46).
type rushedStarter struct {
	*syncStarter
}

func (s rushedStarter) RequestSignoff(ctx context.Context, projectID, sectionLabel string) (cycle.SignoffOpen, error) {
	return s.runner.RequestSignoff(ctx, projectID, sectionLabel)
}

func (s rushedStarter) AnswerOpenQuestion(ctx context.Context, projectID, label string, option *int, reason string) (cycle.OpenQuestionAnswer, error) {
	return s.runner.AnswerOpenQuestion(ctx, projectID, label, option, reason)
}

// rushed is one walkthrough of the rushed profile.
type rushed struct {
	*walk
	script *scripted
	name   string
	// now is the clock the return line reads (server.WithClock).
	now time.Time
}

func newRushed(t *testing.T, name string) *rushed {
	t.Helper()
	dir := t.TempDir()
	db, err := store.Open(context.Background(), dir)
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	script := loadScript(t, name)
	script.db = db
	count := func(ctx context.Context, input []json.RawMessage) (int, error) {
		return provider.NewFake().CountTokens(ctx, provider.PrincipalModel, input)
	}
	starter := &syncStarter{runner: cycle.NewRunner(db, script, count, nil), gate: cycle.NewGate(db), db: db}
	r := &rushed{script: script, name: name, now: time.Now()}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	h := withVisitor(t, db, server.NewWithAPI(logger, server.NewHub(), db, rushedStarter{starter},
		server.WithClock(func() time.Time { return r.now })))
	r.walk = &walk{t: t, h: h, db: db, dir: dir, starter: starter, lastOutput: script.lastOutput}
	return r
}

// rushedRequest is a walkthrough's request (request.json): what the
// human writes at F1 and the material the readers would have read.
type rushedRequest struct {
	Name        string   `json:"name"`
	Text        string   `json:"text"`
	Audience    string   `json:"audience"`
	Criteria    []string `json:"criteria"`
	Premise     string   `json:"premise"`
	Exploratory bool     `json:"exploratory"`
	Sources     []struct {
		Filename string `json:"filename"`
		Kind     string `json:"kind"`
	} `json:"sources"`
}

// open is F1: a new project with the script's request and material, not
// yet submitted.
func (r *rushed) open(step string) {
	r.t.Helper()
	path := filepath.Join(rushedDir, r.name, "request.json")
	b, err := os.ReadFile(path) //nolint:gosec // test data path
	if err != nil {
		r.t.Fatalf("read %s: %v", path, err)
	}
	var req rushedRequest
	if err := json.Unmarshal(b, &req); err != nil {
		r.t.Fatalf("parse %s: %v", path, err)
	}
	var created server.CreateProjectResponse
	r.do(step, http.MethodPost, "/api/projects", map[string]any{"name": req.Name}, http.StatusCreated, &created)
	r.project = created.ProjectID
	r.do(step, http.MethodPut, r.api("/request"), map[string]any{"text": req.Text, "audience": req.Audience,
		"criteria": req.Criteria, "premise": req.Premise, "exploratory": req.Exploratory}, http.StatusOK, nil)
	for _, src := range req.Sources {
		data, err := os.ReadFile(filepath.Join(rushedDir, r.name, "sources", src.Filename)) //nolint:gosec // test data path
		if err != nil {
			r.t.Fatalf("read source %s: %v", src.Filename, err)
		}
		r.upload(step, src.Filename, src.Kind, data)
	}
}

func (r *rushed) upload(step, filename, kind string, data []byte) {
	r.t.Helper()
	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	fw, _ := mw.CreateFormFile("file", filename)
	_, _ = fw.Write(data)
	_ = mw.WriteField("kind", kind)
	_ = mw.Close()
	req := httptest.NewRequest(http.MethodPost, r.api("/sources"), &body)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	rec := httptest.NewRecorder()
	r.h.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		r.fail(step, "upload %s: got %d %s", filename, rec.Code, rec.Body.String())
	}
}

// submit sends the request: the understanding and, after it, the chain
// of steps (B-17) run before it returns.
func (r *rushed) submit(step string) {
	r.t.Helper()
	r.do(step, http.MethodPost, r.api("/request/submit"), nil, http.StatusAccepted, nil)
	r.turnsOK(step)
}

// say sends a message of the human; its turn runs before it returns.
func (r *rushed) say(step, text string) {
	r.t.Helper()
	r.do(step, http.MethodPost, r.api("/messages"), map[string]any{"text": text}, http.StatusAccepted, nil)
	r.turnsOK(step)
}

// turnsOK fails the step when a turn failed.
func (r *rushed) turnsOK(step string) {
	r.t.Helper()
	if len(r.starter.errs) > 0 {
		r.fail(step, "a turn failed")
	}
}

// section finds a section by its title (what the script names).
func (r *rushed) section(step, title string) server.SectionDetail {
	r.t.Helper()
	var sections []server.SectionView
	r.do(step, http.MethodGet, r.api("/sections"), nil, http.StatusOK, &sections)
	var titles []string
	for _, s := range sections {
		if s.Title == title {
			var detail server.SectionDetail
			r.do(step, http.MethodGet, r.api("/sections/"+s.Label), nil, http.StatusOK, &detail)
			return detail
		}
		titles = append(titles, s.Title)
	}
	r.fail(step, "no section %q; sections: %q", title, titles)
	return server.SectionDetail{}
}

func (r *rushed) todo(step string) server.TodoView {
	r.t.Helper()
	var todo server.TodoView
	r.do(step, http.MethodGet, r.api("/todo"), nil, http.StatusOK, &todo)
	return todo
}

// openQuestionBlocking is the one open «por aclarar» of the to-do list
// that blocks the section; it fails the step unless there is exactly one.
func (r *rushed) openQuestionBlocking(step, title string) server.OpenQuestionView {
	r.t.Helper()
	var found []server.OpenQuestionView
	todo := r.todo(step)
	for _, q := range todo.OpenQuestions {
		for _, s := range q.Sections {
			if s.Title == title {
				found = append(found, q)
			}
		}
	}
	if len(found) != 1 {
		r.fail(step, "want one open «por aclarar» blocking %q in GET /todo, got %d: %+v", title, len(found), todo.OpenQuestions)
	}
	return found[0]
}

// openQuestionObject reads the stored «por aclarar» behind a view: what
// it protects is not on screen (never an internal id), only in its body.
func (r *rushed) openQuestionObject(step, label string) (map[string]any, store.Object) {
	r.t.Helper()
	obj, err := r.db.GetObject(context.Background(), r.project, label)
	if err != nil {
		r.fail(step, "open question %s: %v", label, err)
	}
	var body map[string]any
	_ = json.Unmarshal(obj.Body, &body)
	return body, obj
}

// protectedText is the text of what a checker «por aclarar» protects.
func (r *rushed) protectedText(step string, body map[string]any) (store.Object, string) {
	r.t.Helper()
	id, _ := body["protected_id"].(string)
	obj, err := r.db.GetObject(context.Background(), r.project, id)
	if err != nil {
		r.fail(step, "protected object %q of the «por aclarar»: %v", id, err)
	}
	var b map[string]any
	_ = json.Unmarshal(obj.Body, &b)
	text, _ := b["text"].(string)
	if text == "" {
		text, _ = b["statement"].(string)
	}
	return obj, text
}

// openCard is the open card of the decision the section uses, by the
// decision's statement; it fails the step unless there is one.
func (r *rushed) openCardIn(step, title string) server.CardView {
	r.t.Helper()
	todo := r.todo(step)
	for _, c := range todo.Cards {
		for _, s := range c.Sections {
			if s.Title == title {
				return c
			}
		}
	}
	r.fail(step, "no open card used in %q; open cards: %+v", title, todo.Cards)
	return server.CardView{}
}

func (r *rushed) cardState(step, label string) string {
	r.t.Helper()
	var card server.CardView
	r.do(step, http.MethodGet, r.api("/cards/"+label), nil, http.StatusOK, &card)
	return card.State
}

// answerCard answers a card as the rushed human does; reason «ok» and
// the like leave answer_without_reason (422).
func (r *rushed) answerCard(step, label, reason string, want int) {
	r.t.Helper()
	r.do(step, http.MethodPost, r.api("/cards/"+label+"/answer"), map[string]any{"option": "keep", "reason": reason}, want, nil)
}

// answerQuestion answers a «por aclarar»; option nil chooses none.
func (r *rushed) answerQuestion(step, label string, option *int, reason string, want int) {
	r.t.Helper()
	r.do(step, http.MethodPost, r.api("/open-questions/"+label+"/answer"), map[string]any{"option": option, "reason": reason}, want, nil)
}

// withoutReason counts the answer_without_reason events on an object.
func (r *rushed) withoutReason(step, label string) int {
	r.t.Helper()
	obj, err := r.db.GetObject(context.Background(), r.project, label)
	if err != nil {
		r.fail(step, "%s: %v", label, err)
	}
	events, err := r.db.EventsSince(context.Background(), r.project, 0)
	if err != nil {
		r.fail(step, "events: %v", err)
	}
	n := 0
	for _, ev := range events {
		if ev.Type == store.EventAnswerWithoutReason && ev.ObjectID == obj.ID {
			n++
		}
	}
	return n
}

// signoffBlocked asks to sign the section and checks the signoff does
// not open: 409 with the pendings, and no sign-n created (1.7). It
// returns the pendings.
func (r *rushed) signoffBlocked(step, title string) []graph.Pending {
	r.t.Helper()
	sec := r.section(step, title)
	before := r.count(store.ObjectSignoff)
	var blocked server.SignoffBlocked
	r.do(step, http.MethodPost, r.api("/sections/"+sec.Label+"/signoff"), nil, http.StatusConflict, &blocked)
	if blocked.Error != "pendings" || len(blocked.Pendings)+len(blocked.OpenChanges) == 0 {
		r.fail(step, "signoff of %q: %+v, want it blocked with its pendings", title, blocked)
	}
	if after := r.count(store.ObjectSignoff); after != before {
		r.fail(step, "signoff of %q created a signoff (%d → %d)", title, before, after)
	}
	return blocked.Pendings
}

func (r *rushed) count(t store.ObjectType) int {
	r.t.Helper()
	objs, err := r.db.ListObjects(context.Background(), r.project, t)
	if err != nil {
		r.t.Fatalf("ListObjects %s: %v", t, err)
	}
	return len(objs)
}

func hasPending(pendings []graph.Pending, kind, about string) bool {
	for _, p := range pendings {
		if p.Kind == kind && (about == "" || p.About == about) {
			return true
		}
	}
	return false
}

// allUsed checks every hand-made output answered a call: the walkthrough
// reproduced the script up to its cut and no further.
func (r *rushed) allUsed() {
	r.t.Helper()
	if left := r.script.left(); len(left) > 0 {
		r.fail("end", "hand-made outputs no call used: %q (served: %q)", left, r.script.served)
	}
	// RUSHED_DEBUG=1 logs what a passing walkthrough left, for review.
	if os.Getenv("RUSHED_DEBUG") != "" {
		r.t.Logf("served: %q%s", r.script.served, r.diagnosis())
	}
}

// openQuestionView reads one «por aclarar» from the to-do list by label.
func (r *rushed) openQuestionView(step, label string) (server.OpenQuestionView, bool) {
	r.t.Helper()
	for _, q := range r.todo(step).OpenQuestions {
		if q.Label == label {
			return q, true
		}
	}
	return server.OpenQuestionView{}, false
}

func intPtr(i int) *int { return &i }

// openTemplate is F1 on the demo case (B3-19): Paula's request and the
// 16 sources of case 5, not yet submitted.
func (r *rushed) openTemplate(step string) {
	r.t.Helper()
	var created server.CreateProjectResponse
	r.do(step, http.MethodPost, "/api/projects", map[string]any{"template": "case-5"}, http.StatusCreated, &created)
	r.project = created.ProjectID
}

// signBrief signs a section without critical decisions: the signoff
// opens brief (no Wonder round), takes the explanation and signs.
func (r *rushed) signBrief(step, title, explanation string) {
	r.t.Helper()
	sec := r.section(step, title)
	var view graph.SignoffView
	r.do(step, http.MethodPost, r.api("/sections/"+sec.Label+"/signoff"), nil, http.StatusCreated, &view)
	if !view.Brief {
		r.fail(step, "signoff of %q is not brief: %+v", title, view)
	}
	r.do(step, http.MethodPost, r.api("/signoffs/"+view.Label+"/explanation"), map[string]any{"text": explanation}, http.StatusOK, nil)
	r.do(step, http.MethodPost, r.api("/signoffs/"+view.Label+"/sign"), nil, http.StatusOK, nil)
}

// checkerQuestionsBlocking are the open «por aclarar» the checker opened
// that block the section.
func (r *rushed) checkerQuestionsBlocking(step, title string) []server.OpenQuestionView {
	r.t.Helper()
	var out []server.OpenQuestionView
	for _, q := range r.todo(step).OpenQuestions {
		body, _ := r.openQuestionObject(step, q.Label)
		if body["origin"] != "checker" {
			continue
		}
		for _, s := range q.Sections {
			if s.Title == title {
				out = append(out, q)
			}
		}
	}
	return out
}
