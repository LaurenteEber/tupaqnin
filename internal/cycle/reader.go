package cycle

import (
	"bytes"
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"

	"github.com/santhosh-tekuri/jsonschema/v6"

	"github.com/LaurenteEber/tupaqnin/internal/evidence"
	"github.com/LaurenteEber/tupaqnin/internal/graph"
	"github.com/LaurenteEber/tupaqnin/internal/provider"
	"github.com/LaurenteEber/tupaqnin/internal/store"
)

// The reader's instructions (a draft Eber reviews) and its fixed output,
// a copy of salida-lector.schema.json (backlog-r3 3.2).
//
//go:embed prompts/reader.en.md
var readerInstructions string

//go:embed schema/reader_output.schema.json
var readerSchema []byte

// The reader's model and effort (fichas §1, B-65: Spark 1.3 low).
const (
	ReaderModel  = provider.ModelSpark13
	ReaderEffort = provider.EffortLow
)

// readerMaxModelCalls caps the model calls of one reader answer (the
// first or its retry). [EL DISEÑO NO LO DICE]: the design caps tool
// calls (8), not model calls; the evidence micro-test allowed 10.
const readerMaxModelCalls = 10

// readHitChunk is how many characters of a result's text read_hit
// returns from start. [EL DISEÑO NO LO DICE]: «por tramos» without a
// size; the micro-test's 3000.
const readHitChunk = 3000

// Evidence is what the readers search and read through: one shared
// internal/evidence Client (B-60), or its replay (TQ_PROVIDER=fake).
type Evidence interface {
	SearchLiterature(ctx context.Context, q evidence.Query) ([]evidence.Hit, error)
	FetchText(ctx context.Context, rawURL string) (evidence.Text, error)
	Lookup(ctx context.Context, ref evidence.Reference) (evidence.Record, error)
}

// Errors that cut a reader before its next call (B-65).
var (
	errRoundCap   = errors.New("cycle: round cap reached")
	errProjectCap = errors.New("cycle: project cap reached")
)

// roundBudget is a round's spend, checked before each call, model or
// tool (B-65): a model call reserves before it starts («reserva del
// máximo antes de cada llamada»), so the calls in flight, together with
// what was spent, never take the round or the project past its cap; a
// reader already in a call ends it.
// [EL DISEÑO NO LO DICE]: the maximum of one call, which no call fixes
// (there is no output cap): provider.ReaderReserveUSD, above every reader
// call the 3.0 measured, or the dearest call this round has seen if it
// cost more.
type roundBudget struct {
	mu           sync.Mutex
	capUSD       float64
	projectCap   float64
	projectSpent float64
	spent        float64
	reserved     float64 // reserved by the model calls in flight
	dearest      float64 // the dearest call of the round so far
}

// check is the budget before a tool call, which costs nothing itself
// but is only of use if one more model call can read its result: it
// passes while that call's reservation still fits.
func (b *roundBudget) check() error {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.overLocked(b.perCallLocked())
}

func (b *roundBudget) perCallLocked() float64 { return max(provider.ReaderReserveUSD, b.dearest) }

// overLocked reports the cap that spent, reserved and more would pass;
// b.mu is held.
func (b *roundBudget) overLocked(more float64) error {
	const eps = 1e-9
	if b.spent+b.reserved+more > b.capUSD+eps {
		return errRoundCap
	}
	if b.projectSpent+b.spent+b.reserved+more > b.projectCap+eps {
		return errProjectCap
	}
	return nil
}

// reserve sets aside the maximum of one model call before it starts.
func (b *roundBudget) reserve() (float64, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	r := b.perCallLocked()
	if err := b.overLocked(r); err != nil {
		return 0, err
	}
	b.reserved += r
	return r, nil
}

// settle releases a call's reservation and adds what it cost.
func (b *roundBudget) settle(reserved, usd float64) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.reserved -= reserved
	b.spent += usd
	b.dearest = max(b.dearest, usd)
}

// add counts a cost spent outside a reserved call.
func (b *roundBudget) add(usd float64) { b.settle(0, usd) }

// budgetProvider reserves the round's budget before each model call and
// settles what it cost after.
type budgetProvider struct {
	inner  provider.Provider
	budget *roundBudget
}

func (p budgetProvider) Respond(ctx context.Context, req provider.Request, onDelta func(provider.Delta)) (provider.Response, error) {
	r, err := p.budget.reserve()
	if err != nil {
		return provider.Response{}, err
	}
	resp, err := p.inner.Respond(ctx, req, onDelta)
	cost := 0.0
	if err == nil {
		cost = resp.Usage.CostUSD
	}
	p.budget.settle(r, cost)
	return resp, err
}

// roundRun is one round in flight: the hit registry, numbered within
// the round with a counter guarded by mu (the readers run in parallel,
// B-58), and each reader's progress for the page.
type roundRun struct {
	mu        sync.Mutex
	hits      []HitRecord
	abstracts []string
	texts     map[int]string // read_hit's text by hit number, fetched once
	// downloads are the round's downloads by URL, shared by read_hit and
	// VerifyFindings, so a page is fetched once and a URL never gets
	// more than one retry in the whole round («No hacer», 3.3).
	downloads map[string]*download
	status    []string
	reasons   []string
	toolCalls []int
}

func newRoundRun(parts int) *roundRun {
	rr := &roundRun{texts: map[int]string{}, downloads: map[string]*download{}, status: make([]string, parts),
		reasons: make([]string, parts), toolCalls: make([]int, parts)}
	for i := range rr.status {
		rr.status[i] = ReaderPending
	}
	return rr
}

// maxDownloadAttempts is a URL's first attempt and its one retry, in the
// whole round (backlog-r3 3.3, «No hacer»).
const maxDownloadAttempts = 2

// download is one URL an API gave, downloaded in the round: its text,
// or that the page does not exist, or that it failed after its
// attempts. mu is held while it downloads, so a second caller waits for
// the first's result instead of downloading again.
type download struct {
	mu       sync.Mutex
	attempts int
	done     bool
	text     string
	notFound bool
}

// download returns the round's download of url.
func (rr *roundRun) download(url string) *download {
	rr.mu.Lock()
	defer rr.mu.Unlock()
	d, ok := rr.downloads[url]
	if !ok {
		d = &download{}
		rr.downloads[url] = d
	}
	return d
}

// fetch makes at most perCall attempts of this call, and never more
// than maxDownloadAttempts in the round; a successful download, a page
// that does not exist, or a failure that will not pass is final and
// reused.
func (d *download) fetch(ctx context.Context, ev Evidence, url string, perCall int) fetched {
	d.mu.Lock()
	defer d.mu.Unlock()
	for i := 0; i < perCall && !d.done && d.attempts < maxDownloadAttempts; i++ {
		d.attempts++
		t, err := ev.FetchText(ctx, url)
		switch {
		case err == nil:
			d.text, d.done = strings.TrimSpace(t.Text), true
		case errors.Is(err, evidence.ErrNotFound):
			d.notFound, d.done = true, true
		case errors.Is(err, evidence.ErrUnavailable), errors.Is(err, evidence.ErrTimeout):
			// May pass: a later attempt, within the budget, can try.
		default:
			d.done = true
		}
	}
	return fetched{text: d.text, notFound: d.notFound}
}

func (rr *roundRun) set(part int, status, reason string, calls int) {
	rr.mu.Lock()
	defer rr.mu.Unlock()
	rr.status[part-1], rr.reasons[part-1], rr.toolCalls[part-1] = status, reason, calls
}

// register numbers hits hit-n in the round, in arrival order.
func (rr *roundRun) register(part int, hits []evidence.Hit) []HitRecord {
	rr.mu.Lock()
	defer rr.mu.Unlock()
	out := make([]HitRecord, 0, len(hits))
	for _, h := range hits {
		rec := HitRecord{Label: fmt.Sprintf("hit-%d", len(rr.hits)+1), Part: part, Service: h.Service, ID: h.ID,
			Title: h.Title, Authors: h.Authors, Year: h.Year, DOI: h.DOI, URL: h.URL, OpenText: h.OpenText,
			Abstract: h.Abstract}
		if rec.Authors == nil {
			rec.Authors = []string{}
		}
		rr.hits = append(rr.hits, rec)
		rr.abstracts = append(rr.abstracts, h.Abstract)
		out = append(out, rec)
	}
	return out
}

func (rr *roundRun) hit(label string) (int, HitRecord, string, bool) {
	var n int
	if _, err := fmt.Sscanf(label, "hit-%d", &n); err != nil || fmt.Sprintf("hit-%d", n) != label {
		return 0, HitRecord{}, "", false
	}
	rr.mu.Lock()
	defer rr.mu.Unlock()
	if n < 1 || n > len(rr.hits) {
		return 0, HitRecord{}, "", false
	}
	return n, rr.hits[n-1], rr.abstracts[n-1], true
}

// evidenceClient is the runner's evidence: the one set by main, or the
// replay, which never reaches the network.
func (r *Runner) evidenceClient() Evidence {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.Evidence == nil {
		r.Evidence = evidence.NewReplay()
	}
	return r.Evidence
}

func (r *Runner) projectCap() float64 {
	if r.ProjectCapUSD > 0 {
		return r.ProjectCapUSD
	}
	return DefaultProjectCapUSD
}

func (r *Runner) roundCap() float64 {
	if r.RoundCapUSD > 0 {
		return r.RoundCapUSD
	}
	return RoundCapUSD
}

// runRound runs an open round's readers (B-64: on its own, without
// waiting for a click). The caller holds the project's turn slot (P-20):
// your messages get 409 turn_running until it ends. First the project
// cap: if what the project spent plus the round's estimate passes it,
// no reader is launched and the round closes (B-65). Then one reader
// turn per part, in parallel, and the readers' returns and the hit
// registry go to the round in one write when they end.
func (r *Runner) runRound(ctx context.Context, projectID, roundID, publishTurn string) error {
	round, err := r.db.GetObject(ctx, projectID, roundID)
	if err != nil {
		return err
	}
	var body RoundBody
	if err := json.Unmarshal(round.Body, &body); err != nil {
		return fmt.Errorf("cycle: round body: %w", err)
	}
	// The page learns a round started and shows its card (B-64).
	r.publishRound(projectID, publishTurn, round.Label)
	spent, err := r.db.ProjectSpend(ctx, projectID)
	if err != nil {
		return err
	}
	capErr, err := r.spend.fits(ctx, projectID, provider.RoleReader, body.EstimatedCostUSD)
	if err != nil {
		return err
	}
	if capErr != nil {
		// [EL DISEÑO NO LO DICE]: a round a cap stops before it launches
		// closes, with no readers, as B-64 closed a round never launched
		// (reduced_doubt stays null).
		body.Stopped = capErr.Reason
		body.Readers = make([]ReaderResult, len(body.Parts))
		for i := range body.Readers {
			body.Readers[i] = ReaderResult{Part: i + 1, Status: ReaderEmpty, Reason: capErr.Reason}
		}
		body.Hits = []HitRecord{}
		return r.writeRoundEnd(ctx, projectID, round, body, "closed")
	}

	rr := newRoundRun(len(body.Parts))
	r.mu.Lock()
	if r.rounds == nil {
		r.rounds = map[string]*roundRun{}
	}
	r.rounds[roundID] = rr
	r.mu.Unlock()
	defer func() {
		r.mu.Lock()
		delete(r.rounds, roundID)
		r.mu.Unlock()
	}()

	// B-59 (d): the round's references are looked up by the app, without
	// a reader, before the readers start.
	body.Findings = r.lookupRoundReferences(ctx, body.References, rr)
	rr.mu.Lock()
	body.Hits = append([]HitRecord{}, rr.hits...)
	rr.mu.Unlock()

	budget := &roundBudget{capUSD: r.roundCap(), projectCap: r.projectCap() - provider.SignoffReserveUSD, projectSpent: spent}
	briefs := make([]string, len(body.Parts))
	for i := range body.Parts {
		if briefs[i], err = r.readerBrief(ctx, projectID, round.Label, body, i+1); err != nil {
			return err
		}
	}
	results := make([]ReaderResult, len(body.Parts))
	var wg sync.WaitGroup
	for i := range body.Parts {
		wg.Add(1)
		go func(part int) {
			defer wg.Done()
			results[part-1] = r.runReader(ctx, projectID, body, part, briefs[part-1], rr, budget)
		}(i + 1)
	}
	wg.Wait()
	body.Readers = results
	rr.mu.Lock()
	body.Hits = append([]HitRecord{}, rr.hits...)
	rr.mu.Unlock()
	if err := r.writeRoundEnd(ctx, projectID, round, body, "open"); err != nil {
		return err
	}
	// Backlog-r3 3.3: when the readers end, the app checks each finding;
	// 3.4: then the principal digests them and the round closes, while
	// the round still shows running.
	if err := r.VerifyFindings(ctx, projectID, round.Label); err != nil {
		return err
	}
	return r.Digest(ctx, projectID, round.Label)
}

// writeRoundEnd is the round's one write when its readers end: their
// returns and the hit registry (backlog-r3 3.2), conditioned on the
// version read.
func (r *Runner) writeRoundEnd(ctx context.Context, projectID string, round store.Object, body RoundBody, state string) error {
	return r.db.Write(ctx, projectID, func(tx *store.Tx) error {
		data := map[string]any{"to": state, "readers_ended": len(body.Readers)}
		if body.Stopped != "" {
			data["stopped"] = body.Stopped
		}
		_, err := tx.UpdateObject(round.ID, round.CurrentVersion, state, body, store.EventInput{
			Type: store.EventStateChanged, Actor: "app", TurnID: body.TurnID, Data: data,
		})
		return err
	})
}

// runReader runs one reader: its brief, its tools, the fixed output and
// one retry; if it fails the shape twice it is «vacío» (fichas §7). Its
// calls carry the round's turn id and no principal instance (B-62).
func (r *Runner) runReader(ctx context.Context, projectID string, body RoundBody, part int, brief string,
	rr *roundRun, budget *roundBudget) ReaderResult {
	calls := 0
	rr.set(part, ReaderRunning, "", 0)
	end := func(status, reason string, output json.RawMessage) ReaderResult {
		rr.set(part, status, reason, calls)
		return ReaderResult{Part: part, Status: status, Reason: reason, ToolCalls: calls, Output: output}
	}
	schema, err := provider.TransportSchema(readerSchema)
	if err != nil {
		return end(ReaderEmpty, EmptyProviderError, nil)
	}
	spec := body.Parts[part-1]
	ts := r.readerToolSet(projectID, spec, part, rr, budget, &calls)
	req := provider.Request{ProjectID: projectID, TurnID: body.TurnID, Role: provider.RoleReader,
		TurnKind: provider.TurnReader, Model: ReaderModel, Effort: ReaderEffort,
		Instructions: readerInstructions, Input: []json.RawMessage{userItem(brief)},
		Schema: schema, SchemaStrict: true, Part: part}
	prov := budgetProvider{inner: r.prov, budget: budget}
	var capErr *provider.CapError
	for attempt := 1; attempt <= 2; attempt++ {
		resp, sent, err := respondWith(ctx, prov, req, nil, ts)
		switch {
		case errors.Is(err, errRoundCap):
			return end(ReaderEmpty, EmptyRoundCap, nil)
		case errors.Is(err, errProjectCap):
			return end(ReaderEmpty, EmptyProjectCap, nil)
		case errors.As(err, &capErr):
			return end(ReaderEmpty, capErr.Reason, nil)
		case err != nil:
			return end(ReaderEmpty, EmptyProviderError, nil)
		}
		var errs []string
		if len(resp.ToolCalls) > 0 {
			errs = []string{"/: no fixed output: return it now, without calling tools"}
		} else {
			errs = validateReaderOutput(resp.Text)
		}
		if len(errs) == 0 {
			return end(ReaderReturned, "", json.RawMessage(resp.Text))
		}
		if attempt == 2 {
			break
		}
		req.Input = append(append(append([]json.RawMessage{}, sent...), resp.RawItems...),
			userItem(RetryMessage(errs)))
	}
	return end(ReaderEmpty, EmptyInvalidOutput, nil)
}

var (
	readerSchemaOnce sync.Once
	readerSchemaC    *jsonschema.Schema
	readerSchemaErr  error
)

// validateReaderOutput checks a reader's return against salida-lector:
// "path: message" per error, nil when it has the fixed shape.
func validateReaderOutput(text string) []string {
	readerSchemaOnce.Do(func() {
		doc, err := jsonschema.UnmarshalJSON(bytes.NewReader(readerSchema))
		if err != nil {
			readerSchemaErr = err
			return
		}
		c := jsonschema.NewCompiler()
		const loc = "https://tupaqnin.invalid/reader_output.schema.json"
		if err := c.AddResource(loc, doc); err != nil {
			readerSchemaErr = err
			return
		}
		readerSchemaC, readerSchemaErr = c.Compile(loc)
	})
	if readerSchemaErr != nil {
		return []string{"/: reader schema unavailable"}
	}
	inst, err := jsonschema.UnmarshalJSON(strings.NewReader(text))
	if err != nil {
		return []string{"/: output is not valid JSON"}
	}
	if err := readerSchemaC.Validate(inst); err != nil {
		var verr *jsonschema.ValidationError
		if errors.As(err, &verr) {
			return flattenErrors(verr)
		}
		return []string{"/: " + err.Error()}
	}
	return nil
}

// Tool definitions of a reader (B-55, P-97: no web).
func readerToolDefs(spec RoundPart) json.RawMessage {
	var defs []any
	services := []string{}
	project := false
	for _, w := range spec.Where {
		switch w {
		case "project":
			project = true
		case evidence.ServiceOpenAlex, evidence.ServiceArxiv:
			services = append(services, w)
		}
	}
	if project {
		var principal []any
		_ = json.Unmarshal(principalTools, &principal)
		defs = append(defs, principal...)
	}
	if len(services) > 0 {
		defs = append(defs,
			map[string]any{"type": "function", "name": "search_literature", "strict": true,
				"description": "Search published literature. Returns up to 10 results, each numbered hit-n, with title, authors, year, DOI and whether its text is open.",
				"parameters": map[string]any{"type": "object", "additionalProperties": false,
					"required": []string{"query", "where", "year_from"},
					"properties": map[string]any{
						"query":     map[string]any{"type": "string"},
						"where":     map[string]any{"type": "string", "enum": services},
						"year_from": map[string]any{"type": []string{"integer", "null"}},
					}}},
			map[string]any{"type": "function", "name": "read_hit", "strict": true,
				"description": "Read the accessible text of a search result from a character offset (null for the beginning).",
				"parameters": map[string]any{"type": "object", "additionalProperties": false,
					"required": []string{"hit", "start"},
					"properties": map[string]any{
						"hit":   map[string]any{"type": "string", "description": "The result's label, for example hit-3."},
						"start": map[string]any{"type": []string{"integer", "null"}},
					}}})
	}
	raw, _ := json.Marshal(defs)
	return raw
}

// readerToolSet is a reader's tools and cap: 8 tool calls, every call
// counted, the round's budget checked before each one (B-65).
func (r *Runner) readerToolSet(projectID string, spec RoundPart, part int, rr *roundRun, budget *roundBudget, calls *int) toolSet {
	allowed := map[string]bool{}
	for _, w := range spec.Where {
		allowed[w] = true
	}
	return toolSet{defs: readerToolDefs(spec), maxCalls: MaxReaderToolCalls, calls: calls,
		maxModelCalls: readerMaxModelCalls, beforeTool: budget.check,
		run: func(ctx context.Context, call provider.ToolCall) (string, error) {
			defer rr.set(part, ReaderRunning, "", *calls)
			return r.runReaderTool(ctx, projectID, call, allowed, part, rr)
		}}
}

// runReaderTool answers one reader tool call. What a reader reads never
// writes delivery (2.11, B-62). Every result goes to the model between
// the data markers (P-83); hits are numbered in the round (B-58).
func (r *Runner) runReaderTool(ctx context.Context, projectID string, call provider.ToolCall,
	allowed map[string]bool, part int, rr *roundRun) (string, error) {
	var args struct {
		Source   string  `json:"source"`
		Span     *string `json:"span"`
		Text     string  `json:"text"`
		Query    string  `json:"query"`
		Where    string  `json:"where"`
		YearFrom *int    `json:"year_from"`
		Hit      string  `json:"hit"`
		Start    *int    `json:"start"`
	}
	if err := json.Unmarshal([]byte(call.Arguments), &args); err != nil {
		return "The arguments are not valid JSON.", nil
	}
	switch call.Name {
	case "read_source", "search_sources":
		if !allowed["project"] {
			break
		}
		if call.Name == "search_sources" {
			return r.memory.ReaderSearchSources(ctx, projectID, args.Text)
		}
		span := ""
		if args.Span != nil {
			span = *args.Span
		}
		return r.memory.ReaderReadSource(ctx, projectID, args.Source, span)
	case "search_literature":
		if args.Where != evidence.ServiceOpenAlex && args.Where != evidence.ServiceArxiv || !allowed[args.Where] {
			return "where is not one of the places your part searches.", nil
		}
		q := evidence.Query{Text: args.Query, Where: args.Where}
		if args.YearFrom != nil {
			q.YearFrom = *args.YearFrom
		}
		hits, err := r.evidenceClient().SearchLiterature(ctx, q)
		if err != nil {
			return "The search did not answer; try again or another query.", nil
		}
		if len(hits) == 0 {
			return "No results.", nil
		}
		var b strings.Builder
		for _, h := range rr.register(part, hits) {
			doi := h.DOI
			if doi == "" {
				doi = "none"
			}
			open := "no"
			if h.OpenText {
				open = "yes"
			}
			fmt.Fprintf(&b, "%s · %s · %s · %d · DOI %s · open text: %s\n", h.Label, h.Title,
				strings.Join(h.Authors, ", "), h.Year, doi, open)
		}
		return graph.Delimit(graph.DataSearchResult, b.String()), nil
	case "read_hit":
		return r.readHit(ctx, rr, args.Hit, args.Start), nil
	}
	return "There is no tool " + call.Name + ".", nil
}

// readHit returns a result's accessible text from start: its open text,
// downloaded from the URL the API gave (never one a model wrote, B-58),
// or else its abstract.
func (r *Runner) readHit(ctx context.Context, rr *roundRun, label string, start *int) string {
	n, h, abstract, ok := rr.hit(label)
	if !ok {
		return "There is no result " + label + " in this round."
	}
	rr.mu.Lock()
	text, cached := rr.texts[n]
	rr.mu.Unlock()
	if !cached {
		if h.OpenText && h.URL != "" {
			text = rr.download(h.URL).fetch(ctx, r.evidenceClient(), h.URL, 1).text
		}
		if text == "" {
			text = abstract
		}
		rr.mu.Lock()
		rr.texts[n] = text
		rr.mu.Unlock()
	}
	runes := []rune(text)
	if len(runes) == 0 {
		return label + " has no accessible text."
	}
	from := 0
	if start != nil && *start > 0 {
		from = min(*start, len(runes))
	}
	to := min(from+readHitChunk, len(runes))
	// P-83: the title comes from an API and the text from a page: both
	// go between the markers; the app's header stays outside.
	return fmt.Sprintf("%s, %d, characters %d-%d of %d. Title:\n%s\nText:\n%s", label, h.Year, from, to, len(runes),
		graph.Delimit(graph.DataSearchResult, h.Title), graph.Delimit(graph.DataSource, string(runes[from:to])))
}

// readerBrief is a reader's brief, assembled by the app with the
// template of bloque-1 §4, in literal: request, question, doubt and
// reason, criteria with their cases, premise, current decisions, the
// expectation (or «sin expectativa», B-64), scope, sources with what
// for, evidence rules and the fixed return. Only labels, never ids.
func (r *Runner) readerBrief(ctx context.Context, projectID, roundLabel string, body RoundBody, part int) (string, error) {
	labeler, err := graph.NewLabeler(ctx, r.db, projectID)
	if err != nil {
		return "", err
	}
	label := func(id string) string {
		l, err := labeler.ToLabel(id)
		if err != nil {
			return ""
		}
		return l
	}
	request, err := r.requestText(ctx, projectID)
	if err != nil {
		return "", err
	}
	spec := body.Parts[part-1]
	var b strings.Builder
	line := func(format string, args ...any) { fmt.Fprintf(&b, format+"\n", args...) }
	line("# Reader brief: round %s, part %d of %d", roundLabel, part, len(body.Parts))
	line("")
	line("## 1. Request of the human (literal)")
	line("«%s»", request)
	line("")
	line("## 2. Question, doubt and why it is delegated")
	line("Round question:%s", body.questionLine(body.Question))
	line("Doubt it reduces: %s", body.Doubt)
	line("Why it is delegated: %s", body.DelegationReason)
	line("Question of your part:%s", body.questionLine(spec.Question))
	line("")
	line("## 3. Criteria, premise, decisions and expectation (literal)")
	if err := r.writeBriefContext(ctx, projectID, line); err != nil {
		return "", err
	}
	if body.Expectation == "" {
		line("Expectation of the human for this round: none (no expectation).")
	} else {
		line("Expectation of the human for this round (the premise, literal): «%s»", body.Expectation)
	}
	line("")
	line("## 4. Scope")
	line("%s", body.Scope)
	line("Search in: %s.", strings.Join(spec.Where, ", "))
	if len(spec.Queries) > 0 {
		line("Suggested queries: %s.", strings.Join(spec.Queries, " · "))
	}
	line("")
	line("## 5. Sources of the project")
	if len(spec.Sources) == 0 {
		line("None assigned.")
	}
	var found []HitRecord
	for _, h := range body.Hits {
		if h.Part == 0 {
			found = append(found, h)
		}
	}
	if len(found) > 0 {
		// B-59 (d): the round's references the app found, as results.
		line("References of the round the app found in the registry (read them with read_hit):")
		var list strings.Builder
		for _, h := range found {
			doi := h.DOI
			if doi == "" {
				doi = "none"
			}
			fmt.Fprintf(&list, "%s · %s · %d · DOI %s\n", h.Label, h.Title, h.Year, doi)
		}
		// P-83: titles come from an API: data, between the markers.
		line("%s", graph.Delimit(graph.DataSearchResult, list.String()))
	}
	for _, s := range spec.Sources {
		name := ""
		if src, err := r.db.GetObject(ctx, projectID, s.SourceID); err == nil {
			name = bodyStr(bodyOf(src), "filename")
		}
		line("- %s %s: %s", label(s.SourceID), name, s.WhatFor)
	}
	line("")
	line("## 6. Evidence rules")
	line("Quote literally, with the source. Write [inferencia] when you reason without a text. Do not fill gaps. Never quote what you did not see.")
	line("")
	line("## 7. Return")
	line("Your fixed output only: findings with a literal passage and their source, separating facts, assumptions, contradictions and gaps; the criteria or questions you covered, by label; the sources and hits you read. No recommendations.")
	return b.String(), nil
}

// writeBriefContext writes the part of a brief bloque-1 §4 fixes for
// readers and judge alike: the criteria with their cases, the premise and
// the current decisions, in literal, with their label and version.
func (r *Runner) writeBriefContext(ctx context.Context, projectID string, line func(string, ...any)) error {
	list := func(t store.ObjectType) ([]store.Object, error) { return r.db.ListObjects(ctx, projectID, t) }
	criteria, err := list(store.ObjectCriterion)
	if err != nil {
		return err
	}
	cases, err := list(store.ObjectUnderstandingCase)
	if err != nil {
		return err
	}
	premises, err := list(store.ObjectPremise)
	if err != nil {
		return err
	}
	decisions, err := list(store.ObjectDecision)
	if err != nil {
		return err
	}
	for _, c := range criteria {
		line("- %s (version %d): «%s»", c.Label, c.CurrentVersion, bodyStr(bodyOf(c), "text"))
		for _, k := range cases {
			var kb struct {
				Text        string `json:"text"`
				Counts      *bool  `json:"counts"`
				CriterionID string `json:"criterion_id"`
			}
			_ = json.Unmarshal(k.Body, &kb)
			if kb.CriterionID != c.ID || kb.Counts == nil {
				continue
			}
			verdict := "does not count"
			if *kb.Counts {
				verdict = "counts"
			}
			line("  - %s (%s): «%s»", k.Label, verdict, kb.Text)
		}
	}
	for _, p := range premises {
		line("- %s, premise (version %d): «%s»", p.Label, p.CurrentVersion, bodyStr(bodyOf(p), "text"))
	}
	for _, d := range decisions {
		if d.State == "superseded" || d.State == "revoked" {
			continue
		}
		line("- %s, decision %s (version %d): «%s»", d.Label, d.State, d.CurrentVersion, bodyStr(bodyOf(d), "statement"))
	}
	return nil
}
