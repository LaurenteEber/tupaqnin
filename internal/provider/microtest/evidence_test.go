//go:build meta

// Micro-test of evidence (backlog-r3 3.0, B-55, B-60, B-65, B-71):
// before slice 3 builds the round, it measures with Meta what slice 3
// assumes. (1) A reader call with Meta's hosted {"type":"web_search"}
// and the reader's strict fixed output (salida-lector): is it admitted,
// what items come back (web_search_call, URL annotations), tokens and
// cost. (2) The same reader with search_literature and read_hit as
// strict function tools over real OpenAlex and arXiv answers. Five runs
// each. (3) Without a model: OpenAlex and arXiv queries for case 5 (the
// minimum particle size, B-71) and «pobreza y desarrollo cerebral»,
// saved as recordings for the tests of 3.1, with no key in them.
//
// It runs only with -tags=meta; parts 1 and 2 need META_API_KEY, every
// part needs TQ_CONTACT_EMAIL (B-60: OpenAlex's polite pool):
//
//	make microtest-evidence
//
// No Meta, OpenAlex or arXiv answer fails this test: everything is
// logged (-v) and the report is written from that output into
// docs/casos-prueba/micro-prueba-evidencia/informe.md. The test only
// calls the APIs it builds the URL for; it never downloads an address a
// model wrote (B-58).
package microtest

import (
	"context"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/LaurenteEber/tupaqnin/internal/provider"
)

const (
	readerSchemaPath = "../../../docs/diseno/bloque-3/salida-lector.schema.json"
	// The recordings for 3.1's tests (httptest.Server) and, embedded, for
	// the fake provider's evidence (3.2).
	// [EL DISEÑO NO LO DICE]: where they live; the testdata of the
	// package 3.1 creates, internal/evidence.
	evidenceRecordingDir = "../../evidence/testdata/recordings"
	// The model calls of the first valid run of part 2, for 3.2's fake
	// reader (P-96), and the API answers that run used, beside the
	// recordings of part 3.
	readerFixtureDir = "../testdata/reader"
	evidenceRuns     = 5
	// B-65: up to 8 tool calls per reader; the model rounds are capped
	// apart, so a run always ends.
	readerToolCap     = 8
	readerModelRounds = 10
	// B-60: 5 s and 5 MB per resource; arXiv, one request every 3 s.
	evidenceTimeout  = 5 * time.Second
	evidenceMaxBytes = 5 << 20
	arxivSpacing     = 3 * time.Second
	// readHitChunk is how much of a result's text read_hit returns from
	// start.
	readHitChunk = 3000
)

// The reader runs with Spark 1.3 low (fichas §1, B-65).
var readerModel, readerEffort = provider.ModelSpark13, provider.EffortLow

const readerBaseInstructions = `You are a reader of tupaqnin. You search evidence for one part of a research request and return findings; you never write the document.
Every finding carries a passage copied literally from what you read, never a paraphrase or a translation, and names its source in exactly one way, the other two null:
- hit: a search result one of your tools returned, by its label (hit-1, hit-2...);
- reference: a work you name yourself that did not come from a search result (title, authors, year, and its DOI if you know it);
- source_id: a source of the human's project (this request has none).
A finding of kind gap says in note what you looked for and did not find; its passage and source are null.
covered lists the criteria labels of the request that your findings cover. sources_read lists the hit labels you actually read.
Return only the fixed JSON output.`

const webSearchInstructions = readerBaseInstructions + `
You can search the web with web_search.`

const literatureInstructions = readerBaseInstructions + `
You have two tools: search_literature searches OpenAlex or arXiv and numbers each result as hit-n; read_hit returns the accessible text of a result from a character offset. Use at most 8 tool calls in total.`

// readerTask is the request of the part, as the app will assemble it
// (bloque-1 §4), with case 5's request, criteria and premise in literal
// and the part's question on the minimum particle size (B-71).
const readerTask = `Request of the human (literal): «Escribe conmigo el capítulo 2 de mi tesis doctoral: el estado del arte sobre microplásticos en agua potable. El capítulo compara los conteos y los métodos de los estudios publicados entre el agua de grifo y la embotellada. Tiene tres secciones: «Estudios incluidos», «Síntesis» y «Conclusiones». No evalúa efectos en la salud ni recomienda qué agua tomar. Usa los artículos y mi cálculo de la comparación que subí como fuentes.»

Question of your part: which methods detect microplastics in drinking water, what minimum particle size each method detects, and whether studies that used different minimum sizes can be compared.

Criteria (literal):
- crit-1: «Solo estudios de campo, no de laboratorio.»
- crit-2: «El capítulo compara conteos y métodos; no evalúa efectos en la salud ni recomienda qué agua tomar.»

Expectation of the human (the premise, literal): «El agua embotellada trae más partículas que la del grifo.»

Scope: published literature on detection methods and minimum particle size. The human's own figures are not searched.`

// The two tools of B-55 over OpenAlex and arXiv. Their argument shape
// here is the micro-test's; 3.1 fixes the real one.
var literatureTools = []map[string]any{
	{
		"type": "function", "name": "search_literature", "strict": true,
		"description": "Search published literature. Returns up to 10 results, each numbered hit-n, with title, authors, year, DOI and whether its text is open.",
		"parameters": map[string]any{
			"type": "object", "additionalProperties": false,
			"required": []string{"query", "where", "year_from"},
			"properties": map[string]any{
				"query":     map[string]any{"type": "string"},
				"where":     map[string]any{"type": "string", "enum": []string{"openalex", "arxiv"}},
				"year_from": map[string]any{"type": []string{"integer", "null"}},
			},
		},
	},
	{
		"type": "function", "name": "read_hit", "strict": true,
		"description": "Read the accessible text of a search result from a character offset (null for the beginning).",
		"parameters": map[string]any{
			"type": "object", "additionalProperties": false,
			"required": []string{"hit", "start"},
			"properties": map[string]any{
				"hit":   map[string]any{"type": "string", "description": "The result's label, for example hit-3."},
				"start": map[string]any{"type": []string{"integer", "null"}},
			},
		},
	},
}

// evidenceClient is the micro-test's own HTTP client (B-60): no Meta
// key, no cookies, 5 s, at most 3 redirects.
var evidenceClient = &http.Client{
	Timeout: evidenceTimeout,
	CheckRedirect: func(req *http.Request, via []*http.Request) error {
		if len(via) >= 3 {
			return fmt.Errorf("too many redirects")
		}
		return nil
	},
}

var (
	arxivMu   sync.Mutex
	arxivLast time.Time
)

// apiResponse is one OpenAlex or arXiv answer, as recorded.
type apiResponse struct {
	Service     string        `json:"service"`
	Query       string        `json:"query"`
	URL         string        `json:"url"` // without the contact
	Status      int           `json:"status"`
	ContentType string        `json:"content_type"`
	Body        string        `json:"body"`
	Elapsed     time.Duration `json:"-"`
	Err         string        `json:"-"`
}

// fetchAPI calls an OpenAlex or arXiv URL the test built, with the
// contact in the User-Agent (B-60), the size cap measured after
// decompression and arXiv spaced by 3 s. One retry after 3 s on a
// transport error or a 5xx.
func fetchAPI(service, query, rawURL, contact string) apiResponse {
	out := apiResponse{Service: service, Query: query, URL: rawURL}
	for attempt := 1; attempt <= 2; attempt++ {
		if service == "arxiv" {
			arxivMu.Lock()
			if wait := arxivSpacing - time.Since(arxivLast); wait > 0 {
				time.Sleep(wait)
			}
			arxivLast = time.Now()
			arxivMu.Unlock()
		}
		start := time.Now()
		req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, rawURL, nil)
		if err != nil {
			out.Err = "build_request"
			return out
		}
		req.Header.Set("User-Agent", "tupaqnin-microtest/0.3 (mailto:"+contact+")")
		resp, err := evidenceClient.Do(req)
		if err != nil {
			out.Err = "transport"
			out.Elapsed = time.Since(start)
			time.Sleep(3 * time.Second)
			continue
		}
		body, rerr := io.ReadAll(io.LimitReader(resp.Body, evidenceMaxBytes+1))
		_ = resp.Body.Close()
		out.Status, out.ContentType, out.Elapsed = resp.StatusCode, resp.Header.Get("Content-Type"), time.Since(start)
		switch {
		case rerr != nil:
			out.Err = "read"
		case len(body) > evidenceMaxBytes:
			out.Err = "over_5mb"
		case resp.StatusCode >= 500 && attempt == 1:
			out.Err = fmt.Sprintf("http_%d", resp.StatusCode)
			time.Sleep(3 * time.Second)
			continue
		case resp.StatusCode != http.StatusOK:
			out.Err = fmt.Sprintf("http_%d", resp.StatusCode)
		default:
			out.Err = ""
			out.Body = string(body)
		}
		return out
	}
	return out
}

func openAlexURL(query string, yearFrom *int) string {
	v := url.Values{"search": {query}, "per-page": {"10"}}
	if yearFrom != nil {
		v.Set("filter", fmt.Sprintf("from_publication_date:%d-01-01", *yearFrom))
	}
	return "https://api.openalex.org/works?" + v.Encode()
}

func arxivURL(query string) string {
	v := url.Values{"search_query": {"all:" + query}, "start": {"0"}, "max_results": {"10"}}
	return "https://export.arxiv.org/api/query?" + v.Encode()
}

// literatureHit is one result as the tools return it.
type literatureHit struct {
	Title, Authors, DOI, Text string
	Year                      int
	Open                      bool
}

// parseOpenAlex reads an OpenAlex /works answer; the abstract is
// rebuilt from its inverted index.
func parseOpenAlex(body string) ([]literatureHit, error) {
	var page struct {
		Results []struct {
			DisplayName     string `json:"display_name"`
			PublicationYear int    `json:"publication_year"`
			DOI             string `json:"doi"`
			OpenAccess      struct {
				IsOA bool `json:"is_oa"`
			} `json:"open_access"`
			Authorships []struct {
				Author struct {
					DisplayName string `json:"display_name"`
				} `json:"author"`
			} `json:"authorships"`
			Abstract map[string][]int `json:"abstract_inverted_index"`
		} `json:"results"`
	}
	if err := json.Unmarshal([]byte(body), &page); err != nil {
		return nil, err
	}
	var hits []literatureHit
	for _, r := range page.Results {
		var names []string
		for _, a := range r.Authorships {
			names = append(names, a.Author.DisplayName)
		}
		hits = append(hits, literatureHit{Title: r.DisplayName, Authors: shortAuthors(names), Year: r.PublicationYear,
			DOI: strings.TrimPrefix(r.DOI, "https://doi.org/"), Open: r.OpenAccess.IsOA, Text: invertedAbstract(r.Abstract)})
	}
	return hits, nil
}

func invertedAbstract(index map[string][]int) string {
	words := map[int]string{}
	last := -1
	for w, positions := range index {
		for _, p := range positions {
			words[p] = w
			if p > last {
				last = p
			}
		}
	}
	parts := make([]string, 0, last+1)
	for i := 0; i <= last; i++ {
		if w, ok := words[i]; ok {
			parts = append(parts, w)
		}
	}
	return strings.Join(parts, " ")
}

// parseArxiv reads an arXiv Atom answer; every arXiv result is open.
func parseArxiv(body string) ([]literatureHit, error) {
	var feed struct {
		Entries []struct {
			Title     string `xml:"title"`
			Summary   string `xml:"summary"`
			Published string `xml:"published"`
			DOI       string `xml:"http://arxiv.org/schemas/atom doi"`
			Authors   []struct {
				Name string `xml:"name"`
			} `xml:"author"`
		} `xml:"entry"`
	}
	if err := xml.Unmarshal([]byte(body), &feed); err != nil {
		return nil, err
	}
	var hits []literatureHit
	for _, e := range feed.Entries {
		var names []string
		for _, a := range e.Authors {
			names = append(names, a.Name)
		}
		year := 0
		if len(e.Published) >= 4 {
			_, _ = fmt.Sscanf(e.Published[:4], "%d", &year)
		}
		hits = append(hits, literatureHit{Title: collapse(e.Title), Authors: shortAuthors(names), Year: year,
			DOI: e.DOI, Open: true, Text: collapse(e.Summary)})
	}
	return hits, nil
}

func shortAuthors(names []string) string {
	if len(names) > 3 {
		return strings.Join(names[:3], ", ") + " et al."
	}
	return strings.Join(names, ", ")
}

var spaces = regexp.MustCompile(`\s+`)

func collapse(s string) string { return strings.TrimSpace(spaces.ReplaceAllString(s, " ")) }

// literatureRun answers one reader's tool calls and numbers its hits,
// as the round will (B-58).
type literatureRun struct {
	contact   string
	responses []apiResponse // the API answers, in order, for the recordings
	hits      []literatureHit
	searches  int
	reads     int
	failures  []string
}

func (lr *literatureRun) answer(fc functionCall) string {
	var args struct {
		Query    string `json:"query"`
		Where    string `json:"where"`
		YearFrom *int   `json:"year_from"`
		Hit      string `json:"hit"`
		Start    *int   `json:"start"`
	}
	if err := json.Unmarshal([]byte(fc.Arguments), &args); err != nil {
		return "error: arguments are not JSON"
	}
	switch fc.Name {
	case "search_literature":
		lr.searches++
		var res apiResponse
		var hits []literatureHit
		var err error
		switch args.Where {
		case "openalex":
			res = fetchAPI("openalex", args.Query, openAlexURL(args.Query, args.YearFrom), lr.contact)
			if res.Err == "" {
				hits, err = parseOpenAlex(res.Body)
			}
		case "arxiv":
			res = fetchAPI("arxiv", args.Query, arxivURL(args.Query), lr.contact)
			if res.Err == "" {
				hits, err = parseArxiv(res.Body)
			}
		default:
			return "error: where is openalex or arxiv"
		}
		if res.Err == "" {
			lr.responses = append(lr.responses, res)
		}
		if res.Err != "" || err != nil {
			lr.failures = append(lr.failures, args.Where+" "+res.Err)
			return "error: the search did not answer; try again or another query"
		}
		var b strings.Builder
		n := 0
		for _, h := range hits {
			if args.YearFrom != nil && h.Year < *args.YearFrom {
				continue
			}
			lr.hits = append(lr.hits, h)
			n++
			doi := h.DOI
			if doi == "" {
				doi = "none"
			}
			fmt.Fprintf(&b, "hit-%d · %s · %s · %d · DOI %s · open text: %s\n", len(lr.hits), h.Title, h.Authors, h.Year, doi,
				map[bool]string{true: "yes", false: "no"}[h.Open])
		}
		if n == 0 {
			return "no results"
		}
		return b.String()
	case "read_hit":
		lr.reads++
		h, ok := lr.hit(args.Hit)
		if !ok {
			return "error: no result " + args.Hit
		}
		// [EL DISEÑO NO LO DICE]: in 3.0 read_hit returns the abstract
		// the API gave; downloading the open text is 3.1 (FetchText, B-60).
		text := h.Text
		if text == "" {
			return "this result has no accessible text"
		}
		from := 0
		if args.Start != nil && *args.Start > 0 {
			from = min(*args.Start, len(text))
		}
		to := min(from+readHitChunk, len(text))
		return fmt.Sprintf("%s (%s, %d), characters %d-%d of %d:\n%s", h.Title, h.Authors, h.Year, from, to, len(text), text[from:to])
	}
	return "error: unknown tool " + fc.Name
}

func (lr *literatureRun) hit(label string) (literatureHit, bool) {
	var n int
	if _, err := fmt.Sscanf(label, "hit-%d", &n); err != nil || n < 1 || n > len(lr.hits) {
		return literatureHit{}, false
	}
	return lr.hits[n-1], true
}

// readerFinding is one finding of the reader's output.
type readerFinding struct {
	Kind      string  `json:"kind"`
	Passage   *string `json:"passage"`
	SourceID  *string `json:"source_id"`
	Hit       *string `json:"hit"`
	Reference *struct {
		Title   string  `json:"title"`
		Authors string  `json:"authors"`
		Year    *int    `json:"year"`
		DOI     *string `json:"doi"`
	} `json:"reference"`
	Note *string `json:"note"`
}

// readerJudgment is what one run's final output says, counted.
type readerJudgment struct {
	valid      bool
	err        string
	kinds      map[string]int
	byHit      int
	byRef      int
	refWithDOI int
	bySourceID int // the request has no project source: an invented one
	unknownHit int // a hit-n the tools never returned
	literal    int // hit passages found literally in that hit's text
	withHit    int
	covered    []string
}

func judgeReader(t *testing.T, final string, readerDoc map[string]any, readerRaw []byte, run *literatureRun) readerJudgment {
	t.Helper()
	j := readerJudgment{kinds: map[string]int{}}
	if final == "" {
		j.err = "sin salida final"
		return j
	}
	inst, err := decodeInstance(final)
	if err != nil {
		j.err = "no es JSON"
		return j
	}
	if obj, ok := inst.(map[string]any); ok {
		stripOptionalNulls(obj, readerDoc, readerDoc)
	}
	if verr := validateInstance(compileSchema(t, readerRaw), inst); verr != nil {
		j.err = strings.ReplaceAll(verr.Error(), "\n", " ")
	} else {
		j.valid = true
	}
	var out struct {
		Findings []readerFinding `json:"findings"`
		Covered  []string        `json:"covered"`
	}
	_ = json.Unmarshal([]byte(final), &out)
	j.covered = out.Covered
	for _, f := range out.Findings {
		j.kinds[f.Kind]++
		switch {
		case f.Hit != nil:
			j.byHit++
			if run == nil {
				j.unknownHit++
				continue
			}
			h, ok := run.hit(*f.Hit)
			if !ok {
				j.unknownHit++
				continue
			}
			if f.Passage != nil {
				j.withHit++
				if strings.Contains(collapse(h.Text), collapse(*f.Passage)) || strings.Contains(collapse(h.Title), collapse(*f.Passage)) {
					j.literal++
				}
			}
		case f.Reference != nil:
			j.byRef++
			if f.Reference.DOI != nil && *f.Reference.DOI != "" {
				j.refWithDOI++
			}
		case f.SourceID != nil:
			j.bySourceID++
		}
	}
	return j
}

func (j readerJudgment) String() string {
	var kinds []string
	for _, k := range []string{"fact", "assumption", "contradiction", "gap"} {
		if j.kinds[k] > 0 {
			kinds = append(kinds, fmt.Sprintf("%s ×%d", k, j.kinds[k]))
		}
	}
	return fmt.Sprintf("valid=%v err=%q findings=[%s] by_hit=%d (unknown %d, literal %d of %d) by_reference=%d (with DOI %d) by_source_id=%d covered=%v",
		j.valid, j.err, strings.Join(kinds, ", "), j.byHit, j.unknownHit, j.literal, j.withHit, j.byRef, j.refWithDOI, j.bySourceID, j.covered)
}

// readerBody is a reader's Responses request.
func readerBody(instructions string, tools []map[string]any, format json.RawMessage, input []json.RawMessage) map[string]any {
	body := map[string]any{
		"model": readerModel, "instructions": instructions,
		"stream": true, "store": false, "include": []string{"reasoning.encrypted_content"},
		"reasoning": map[string]string{"effort": readerEffort},
		"input":     input,
		"tools":     tools, "tool_choice": "auto",
		"text": map[string]any{"format": map[string]any{
			"type": "json_schema", "name": "reader_output", "strict": true, "schema": format}},
	}
	return body
}

// webItems counts what a web_search call returned: the hosted calls,
// the URL annotations and their domains (never the page content).
func webItems(output []json.RawMessage) (types map[string]int, annotations int, domains []string) {
	types = map[string]int{}
	seen := map[string]bool{}
	for _, item := range output {
		var it struct {
			Type    string `json:"type"`
			Content []struct {
				Annotations []struct {
					Type string `json:"type"`
					URL  string `json:"url"`
				} `json:"annotations"`
			} `json:"content"`
		}
		if json.Unmarshal(item, &it) != nil {
			continue
		}
		types[it.Type]++
		for _, c := range it.Content {
			for _, a := range c.Annotations {
				annotations++
				if u, err := url.Parse(a.URL); err == nil && u.Host != "" && !seen[u.Host] {
					seen[u.Host] = true
					domains = append(domains, u.Host)
				}
			}
		}
	}
	sort.Strings(domains)
	return types, annotations, domains
}

func countTypes(types map[string]int) string {
	var keys []string
	for k := range types {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var parts []string
	for _, k := range keys {
		parts = append(parts, fmt.Sprintf("%s ×%d", k, types[k]))
	}
	return strings.Join(parts, ", ")
}

// saveRecording keeps one API answer for 3.1 (no key, no contact).
func saveRecording(t *testing.T, name string, res apiResponse) {
	t.Helper()
	if c := os.Getenv("TQ_CONTACT_EMAIL"); c != "" && (strings.Contains(res.Body, c) || strings.Contains(res.URL, c)) {
		t.Errorf("recording %s holds the contact: not saved", name)
		return
	}
	b, err := json.MarshalIndent(res, "", "  ")
	if err != nil {
		t.Logf("recording %s: marshal: %v", name, err)
		return
	}
	if err := os.MkdirAll(evidenceRecordingDir, 0o755); err != nil {
		t.Logf("recording %s: mkdir: %v", name, err)
		return
	}
	if err := os.WriteFile(filepath.Join(evidenceRecordingDir, name+".json"), b, 0o644); err != nil {
		t.Logf("recording %s: write: %v", name, err)
	}
}

func TestMicroTestEvidence(t *testing.T) {
	contact := os.Getenv("TQ_CONTACT_EMAIL")
	if contact == "" {
		t.Fatal("TQ_CONTACT_EMAIL not set: OpenAlex and arXiv are called with a contact (B-60)")
	}
	apiKey := os.Getenv("META_API_KEY")

	readerRaw, err := os.ReadFile(readerSchemaPath)
	if err != nil {
		t.Fatalf("read reader schema: %v", err)
	}
	var readerDoc map[string]any
	if err := json.Unmarshal(readerRaw, &readerDoc); err != nil {
		t.Fatalf("decode reader schema: %v", err)
	}
	// B-58: the app never downloads an address the model wrote, so the
	// written reference carries no URL.
	if props, _ := readerDoc["$defs"].(map[string]any)["extref"].(map[string]any)["properties"].(map[string]any); props["url"] != nil {
		t.Fatal("salida-lector.schema.json still has extref.url: sync docs/diseno from the design (B-58: no URL)")
	}
	transport, err := provider.TransportSchema(readerRaw)
	if err != nil {
		t.Fatalf("TransportSchema: %v", err)
	}
	t.Logf("run: %s · reader %s %s · store false, streaming", time.Now().UTC().Format("2006-01-02 15:04 UTC"), readerModel, readerEffort)

	t.Run("1-web-search", func(t *testing.T) {
		if apiKey == "" {
			t.Skip("META_API_KEY not set")
		}
		tools := []map[string]any{{"type": "web_search"}}
		var valid int
		var costs []float64
		for i := 1; i <= evidenceRuns; i++ {
			c := postResponses(apiKey, readerBody(webSearchInstructions, tools, transport, []json.RawMessage{userItem(readerTask)}))
			if c.failed() {
				t.Logf("web run %d: rejected %s in %.1f s", i, c.errText(), c.elapsed.Seconds())
				continue
			}
			types, annotations, domains := webItems(c.output)
			j := judgeReader(t, c.text, readerDoc, readerRaw, nil)
			if j.valid {
				valid++
			}
			costs = append(costs, c.cost())
			t.Logf("web run %d: admitted · items [%s] · url annotations %d · domains %v · %s · tokens in %d out %d · token cost %.6f USD · %.1f s · usage %s",
				i, countTypes(types), annotations, domains, j, c.inTokens, c.outTokens, c.cost(), c.elapsed.Seconds(), c.usage)
		}
		t.Logf("web summary: %d of %d valid · mean token cost per reader %.6f USD", valid, evidenceRuns, mean(costs))
	})

	t.Run("2-literature-tools", func(t *testing.T) {
		if apiKey == "" {
			t.Skip("META_API_KEY not set")
		}
		var valid int
		var costs []float64
		saved := false
		for i := 1; i <= evidenceRuns; i++ {
			run := &literatureRun{contact: contact}
			input := []json.RawMessage{userItem(readerTask)}
			var calls []toolCall
			var final, failure string
			var toolCalls, badArgs, streamedArgs int
			names := map[string]int{}
			for round := 1; round <= readerModelRounds; round++ {
				c := postResponses(apiKey, readerBody(literatureInstructions, literatureTools, transport, input))
				calls = append(calls, c)
				if c.failed() {
					failure = c.errText()
					break
				}
				if c.argDeltas > 0 {
					streamedArgs++
				}
				fcs := functionCalls(c.output)
				if len(fcs) == 0 {
					final = c.text
					break
				}
				// store: false: everything the model returned goes back,
				// then one output per call.
				input = append(input, c.output...)
				for _, fc := range fcs {
					toolCalls++
					names[fc.Name]++
					if !literatureArgsValid(fc) {
						badArgs++
					}
					out := "error: the limit of 8 tool calls is reached; return your fixed output now"
					if toolCalls <= readerToolCap {
						out = run.answer(fc)
					}
					input = append(input, toolOutputItem(fc.CallID, out))
				}
			}
			var cost, secs float64
			var in, out int
			for _, c := range calls {
				cost += c.cost()
				secs += c.elapsed.Seconds()
				in += c.inTokens
				out += c.outTokens
			}
			costs = append(costs, cost)
			j := judgeReader(t, final, readerDoc, readerRaw, run)
			if j.valid {
				valid++
			}
			var open int
			for _, h := range run.hits {
				if h.Open {
					open++
				}
			}
			// P-96: the first valid run that used the tools is recorded for
			// 3.2's fake reader: its model calls, then the API answers it
			// read, in order. No key, no contact.
			if !saved && j.valid && toolCalls > 0 {
				for n, c := range calls {
					saveFixtureIn(t, readerFixtureDir, fmt.Sprintf("reader-call-%d", n+1), c)
				}
				for n, res := range run.responses {
					saveRecording(t, fmt.Sprintf("reader-run-%d-%s", n+1, res.Service), res)
				}
				saved = true
				t.Logf("literature run %d recorded: %d model calls in internal/provider/testdata/reader, %d API answers in internal/evidence/testdata/recordings", i, len(calls), len(run.responses))
			}
			t.Logf("literature run %d: model calls %d · tool calls %d (search_literature ×%d, read_hit ×%d, bad args %d, args streamed in %d calls) · hits %d (open %d) · api failures %v · error %s · %s · tokens in %d out %d · cost %.6f USD · %.1f s",
				i, len(calls), toolCalls, names["search_literature"], names["read_hit"], badArgs, streamedArgs,
				len(run.hits), open, run.failures, failure, j, in, out, cost, secs)
		}
		t.Logf("literature summary: %d of %d valid · mean cost per reader %.6f USD · recorded %v", valid, evidenceRuns, mean(costs), saved)
	})

	t.Run("3-recordings", func(t *testing.T) {
		queries := []struct {
			name, service, kase, query string
		}{
			{"openalex-microplastics-detection-size", "openalex", "caso 5", "microplastics drinking water detection minimum particle size"},
			{"openalex-microplastics-spectroscopy-limit", "openalex", "caso 5", "microplastic spectroscopy size detection limit"},
			{"openalex-poverty-brain-development", "openalex", "pobreza", "poverty brain development children"},
			{"arxiv-microplastics-detection", "arxiv", "caso 5", "microplastics detection"},
			{"arxiv-poverty-brain-development", "arxiv", "pobreza", "poverty brain development"},
		}
		for _, q := range queries {
			var res apiResponse
			var hits []literatureHit
			var perr error
			if q.service == "openalex" {
				res = fetchAPI(q.service, q.query, openAlexURL(q.query, nil), contact)
				if res.Err == "" {
					hits, perr = parseOpenAlex(res.Body)
				}
			} else {
				res = fetchAPI(q.service, q.query, arxivURL(q.query), contact)
				if res.Err == "" {
					hits, perr = parseArxiv(res.Body)
				}
			}
			if res.Err != "" || perr != nil {
				t.Logf("recording %s (%s): failed %s %v in %.1f s", q.name, q.kase, res.Err, perr, res.Elapsed.Seconds())
				continue
			}
			var open, withAbstract, withDOI int
			for _, h := range hits {
				if h.Open {
					open++
				}
				if h.Text != "" {
					withAbstract++
				}
				if h.DOI != "" {
					withDOI++
				}
			}
			saveRecording(t, q.name, res)
			var titles []string
			for _, h := range hits {
				titles = append(titles, fmt.Sprintf("%d %s", h.Year, h.Title))
			}
			t.Logf("recording %s (%s, %q): status %d · %s · %d bytes · %.1f s · results %d · open text %d · with abstract %d · with DOI %d · titles %q",
				q.name, q.kase, q.query, res.Status, res.ContentType, len(res.Body), res.Elapsed.Seconds(), len(hits), open, withAbstract, withDOI, titles)
		}
	})
}

// literatureArgsValid reports whether a call's arguments decode and
// carry exactly the tool's required properties.
func literatureArgsValid(fc functionCall) bool {
	var args map[string]any
	if json.Unmarshal([]byte(fc.Arguments), &args) != nil {
		return false
	}
	for _, def := range literatureTools {
		if def["name"] != fc.Name {
			continue
		}
		required := def["parameters"].(map[string]any)["required"].([]string)
		if len(args) != len(required) {
			return false
		}
		for _, name := range required {
			if _, ok := args[name]; !ok {
				return false
			}
		}
		return true
	}
	return false
}

func mean(xs []float64) float64 {
	if len(xs) == 0 {
		return 0
	}
	var sum float64
	for _, x := range xs {
		sum += x
	}
	return sum / float64(len(xs))
}
