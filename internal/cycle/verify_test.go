package cycle_test

import (
	"context"
	"encoding/json"
	"strings"
	"sync"
	"testing"

	"github.com/LaurenteEber/tupaqnin/internal/cycle"
	"github.com/LaurenteEber/tupaqnin/internal/evidence"
	"github.com/LaurenteEber/tupaqnin/internal/graph"
	"github.com/LaurenteEber/tupaqnin/internal/provider"
	"github.com/LaurenteEber/tupaqnin/internal/store"
)

// Backlog-r3 3.3: the app verifies each finding on its own (flujos §2,
// B-59, B-61).

// countingEvidence wraps an Evidence and records every URL FetchText
// got and every reference Lookup got.
type countingEvidence struct {
	inner   cycle.Evidence
	mu      sync.Mutex
	fetched []string
	lookups []evidence.Reference
}

func (c *countingEvidence) SearchLiterature(ctx context.Context, q evidence.Query) ([]evidence.Hit, error) {
	return c.inner.SearchLiterature(ctx, q)
}

func (c *countingEvidence) FetchText(ctx context.Context, url string) (evidence.Text, error) {
	c.mu.Lock()
	c.fetched = append(c.fetched, url)
	c.mu.Unlock()
	return c.inner.FetchText(ctx, url)
}

func (c *countingEvidence) Lookup(ctx context.Context, ref evidence.Reference) (evidence.Record, error) {
	c.mu.Lock()
	c.lookups = append(c.lookups, ref)
	c.mu.Unlock()
	return c.inner.Lookup(ctx, ref)
}

func (c *countingEvidence) urls() []string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]string{}, c.fetched...)
}

// handEvidence is hand-made evidence in the shape of internal/evidence:
// each search returns the next list of hits, each page its text or
// error, each DOI its record or error. It never reaches the network.
type handEvidence struct {
	mu       sync.Mutex
	searches [][]evidence.Hit
	pages    map[string]evidence.Text
	pageErrs map[string]error
	records  map[string]evidence.Record
}

func (h *handEvidence) SearchLiterature(context.Context, evidence.Query) ([]evidence.Hit, error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if len(h.searches) == 0 {
		return nil, nil
	}
	out := h.searches[0]
	h.searches = h.searches[1:]
	return out, nil
}

func (h *handEvidence) FetchText(_ context.Context, url string) (evidence.Text, error) {
	if err, ok := h.pageErrs[url]; ok {
		return evidence.Text{}, err
	}
	if t, ok := h.pages[url]; ok {
		return t, nil
	}
	return evidence.Text{}, evidence.ErrUnavailable
}

func (h *handEvidence) Lookup(_ context.Context, ref evidence.Reference) (evidence.Record, error) {
	if rec, ok := h.records[strings.ToLower(ref.DOI)]; ok && ref.DOI != "" {
		return rec, nil
	}
	return evidence.Record{}, evidence.ErrNotFound
}

// finding is one salida-lector finding.
func finding(kind string, passage *string, sourceID, hit *string, reference map[string]any) map[string]any {
	return map[string]any{"kind": kind, "passage": passage, "source_id": sourceID, "hit": hit,
		"reference": reference, "note": nil}
}

// readerOutput is a reader's fixed output with these findings.
func readerOutput(t *testing.T, findings ...map[string]any) string {
	t.Helper()
	raw, err := json.Marshal(map[string]any{"findings": findings, "covered": []string{"crit-1"}, "sources_read": []string{}})
	if err != nil {
		t.Fatal(err)
	}
	if errs := cycle.ValidateReaderOutput(string(raw)); len(errs) > 0 {
		t.Fatalf("hand-made reader output does not have the fixed shape: %v", errs)
	}
	return string(raw)
}

// The 3.0 recording of OpenAlex the readers search in these tests.
const (
	recordedQuery = `{"query":"microplastics drinking water detection minimum particle size","where":"openalex","year_from":null}`
	missingDOI    = "10.9999/does-not-exist"
)

func recordedHits(t *testing.T) []evidence.Hit {
	t.Helper()
	hits, err := evidence.NewReplay().SearchLiterature(context.Background(), evidence.Query{
		Text: "microplastics drinking water detection minimum particle size", Where: "openalex"})
	if err != nil || len(hits) == 0 || hits[0].Abstract == "" || hits[0].DOI == "" {
		t.Fatalf("recorded hits: %d, %v", len(hits), err)
	}
	return hits
}

// abstractPassage is a literal stretch of the hit's abstract.
func abstractPassage(t *testing.T, h evidence.Hit) string {
	t.Helper()
	words := strings.Fields(h.Abstract)
	if len(words) < 14 {
		t.Fatalf("short abstract: %q", h.Abstract)
	}
	return strings.Join(words[2:14], " ")
}

// oneReaderRound runs a discover step with one openalex part whose
// reader searches the recorded query (when search) and returns output.
func oneReaderRound(t *testing.T, ev cycle.Evidence, search bool, output string) (*store.DB, *cycle.Runner) {
	t.Helper()
	step := discoverStep("too_much_to_read", "["+openAlexPart+"]")
	fixtures := principalSteps(t, step)
	if search {
		fixtures = append(fixtures, readerCalls(0, 0.001, [2]string{"search_literature", recordedQuery}))
	}
	fixtures = append(fixtures, readerText(0, output, 0.001))
	db, runner, _ := newRoundRunner(t, false, fixtures...)
	runner.Evidence = ev
	runDiscover(t, runner, step)
	return db, runner
}

func checkOf(t *testing.T, body cycle.RoundBody, part, n int) cycle.FindingCheck {
	t.Helper()
	for _, c := range body.Findings {
		if c.Part == part && c.Finding == n {
			return c
		}
	}
	t.Fatalf("no check for part %d finding %d: %+v", part, n, body.Findings)
	return cycle.FindingCheck{}
}

func webSources(t *testing.T, db *store.DB) []store.Object {
	t.Helper()
	all, err := db.ListObjects(context.Background(), "p1", store.ObjectSource)
	if err != nil {
		t.Fatal(err)
	}
	var out []store.Object
	for _, s := range all {
		var b map[string]any
		_ = json.Unmarshal(s.Body, &b)
		if b["kind"] == "web" {
			out = append(out, s)
		}
	}
	return out
}

func bodyMap(t *testing.T, obj store.Object) map[string]any {
	t.Helper()
	var b map[string]any
	if err := json.Unmarshal(obj.Body, &b); err != nil {
		t.Fatal(err)
	}
	return b
}

// Listo cuando 1: a reader with three findings (one good, one with a
// DOI that does not exist, one with its passage changed) leaves one
// entered, one discarded with source_not_found and one discarded with
// passage_not_found; each finding on its own. The one that enters from
// outside becomes a web source (B-61), with the round's verification
// event.
func TestB59_EachFindingIsVerifiedOnItsOwn(t *testing.T) {
	hits := recordedHits(t)
	good := abstractPassage(t, hits[0])
	out := readerOutput(t,
		finding("fact", ptr(good), nil, ptr("hit-1"), nil),
		finding("fact", ptr("Los conteos se duplicaron en Lima."), nil, nil,
			map[string]any{"title": "Conteos en Lima", "authors": "Valdivia y Ortiz", "year": 2022, "doi": missingDOI}),
		finding("fact", ptr("En 2023 los conteos fueron 80 veces mayores."), ptr("src-1"), nil, nil))
	db, _ := oneReaderRound(t, evidence.NewReplay(), true, out)
	round, body := onlyRound(t, db)
	if round.State != "closed" || len(body.Findings) != 3 {
		t.Fatalf("round %s, findings %+v", round.State, body.Findings)
	}
	webs := webSources(t, db)
	if len(webs) != 1 {
		t.Fatalf("web sources: %d", len(webs))
	}
	if c := checkOf(t, body, 1, 1); c.Verification != cycle.VerificationEntered || c.Reason != "" ||
		c.Access != "abstract" || c.SourceID != webs[0].ID {
		t.Fatalf("good finding: %+v", c)
	}
	if c := checkOf(t, body, 1, 2); c.Verification != cycle.VerificationDiscarded || c.Reason != cycle.ReasonSourceNotFound || c.SourceID != "" {
		t.Fatalf("missing DOI: %+v", c)
	}
	if c := checkOf(t, body, 1, 3); c.Verification != cycle.VerificationDiscarded || c.Reason != cycle.ReasonPassageNotFound {
		t.Fatalf("changed passage: %+v", c)
	}
	b := bodyMap(t, webs[0])
	if b["doi"] != hits[0].DOI || b["url"] != hits[0].URL || b["title"] != hits[0].Title || b["access"] != "abstract" ||
		b["text"] != hits[0].Abstract || b["fingerprint"] == "" || webs[0].Label != "src-2" {
		t.Fatalf("web source %s: %+v", webs[0].Label, b)
	}
	// The round's verification is one write with its event, after the
	// readers' end, by the app.
	events, err := db.EventsSince(context.Background(), "p1", 0)
	if err != nil {
		t.Fatal(err)
	}
	var verified, created int
	for _, ev := range events {
		var data map[string]any
		_ = json.Unmarshal(ev.Data, &data)
		if ev.Type == store.EventStateChanged && ev.Actor == "app" && data["findings_verified"] == float64(3) &&
			data["entered"] == float64(1) && data["discarded"] == float64(2) {
			verified++
		}
		if ev.Type == store.EventStateChanged && ev.Actor == "app" && data["kind"] == "web" {
			created++
		}
	}
	if verified != 1 || created != 1 {
		t.Fatalf("events: verification %d, web source created %d", verified, created)
	}
}

// Listo cuando 2: a result with only its abstract and the passage
// outside it is «no verificado» (a paywall): kept as a web source with
// access none and no text (B-61).
func TestB59_AbstractOnlyWithThePassageOutsideIsUnverified(t *testing.T) {
	out := readerOutput(t, finding("fact", ptr("Este pasaje no está en el resumen."), nil, ptr("hit-1"), nil))
	db, _ := oneReaderRound(t, evidence.NewReplay(), true, out)
	_, body := onlyRound(t, db)
	c := checkOf(t, body, 1, 1)
	webs := webSources(t, db)
	if c.Verification != cycle.VerificationUnverified || c.Reason != cycle.ReasonInaccessible || len(webs) != 1 || c.SourceID != webs[0].ID {
		t.Fatalf("check %+v, web sources %d", c, len(webs))
	}
	if b := bodyMap(t, webs[0]); b["access"] != "none" || b["text"] != "" {
		t.Fatalf("web source: %+v", b)
	}
}

// Listo cuando 3: the fictitious DOI in a source of your material is
// never checked: the finding on it is checked against its text only,
// with no lookup and no download.
func TestB59_FictitiousDOIOfYourMaterialIsNotChecked(t *testing.T) {
	out := readerOutput(t, finding("fact", ptr("doi:"+missingDOI+", midieron hasta 20 micras"), ptr("src-2"), nil, nil))
	step := discoverStep("too_much_to_read", "["+openAlexPart+"]")
	db, runner, _ := newRoundRunner(t, false, append(principalSteps(t, step), readerText(0, out, 0.001))...)
	if err := db.Write(context.Background(), "p1", func(tx *store.Tx) error {
		_, err := tx.CreateObject(store.ObjectSource, "", map[string]any{"kind": "file", "filename": "nota.md",
			"text": "Valdivia y Ortiz (2022), doi:" + missingDOI + ", midieron hasta 20 micras."},
			store.EventInput{Type: store.EventStateChanged, Actor: "human"})
		return err
	}); err != nil {
		t.Fatal(err)
	}
	ev := &countingEvidence{inner: evidence.NewReplay()}
	runner.Evidence = ev
	runDiscover(t, runner, step)
	_, body := onlyRound(t, db)
	src, _ := db.GetObject(context.Background(), "p1", "src-2")
	if c := checkOf(t, body, 1, 1); c.Verification != cycle.VerificationEntered || c.SourceID != src.ID || c.Access != "" {
		t.Fatalf("material finding: %+v", c)
	}
	if len(ev.lookups) != 0 || len(ev.urls()) != 0 || len(webSources(t, db)) != 0 {
		t.Fatalf("lookups %v, fetched %v", ev.lookups, ev.urls())
	}
}

// Listo cuando 4: the same DOI in two rounds does not create two
// sources; the second round reuses it, and a changed text is its new
// version (source_new_version).
func TestB61_SameDOIInTwoRoundsIsOneSource(t *testing.T) {
	hit := func(abstract string) []evidence.Hit {
		return []evidence.Hit{{Service: "openalex", ID: "W1", Title: "Detection limits", Year: 2021,
			DOI: "10.1/limits", URL: "https://openalex.example/W1", Abstract: abstract}}
	}
	const passage = "Micro-FTIR detects particles down to 20 micrometres."
	ev := &handEvidence{searches: [][]evidence.Hit{hit("First: " + passage), hit("Second: " + passage)}}
	out := readerOutput(t, finding("fact", ptr(passage), nil, ptr("hit-1"), nil))
	step := discoverStep("too_much_to_read", "["+openAlexPart+"]")
	fixtures := func() []provider.Fixture {
		return append(principalSteps(t, step), readerCalls(0, 0.001, [2]string{"search_literature", recordedQuery}),
			readerText(0, out, 0.001))
	}
	db, runner, _ := newRoundRunner(t, false, fixtures()...)
	runner.Evidence = ev
	runDiscover(t, runner, step)
	// The second round, in a later session of the same project.
	fake := provider.NewFake(fixtures()...)
	fake.Recorder = db
	count := func(ctx context.Context, input []json.RawMessage) (int, error) {
		return fake.CountTokens(ctx, provider.PrincipalModel, input)
	}
	runner2 := cycle.NewRunner(db, fake, count, &fakePublisher{})
	runner2.Evidence = ev
	t.Cleanup(runner2.Close)
	if err := runner2.RunTurn(context.Background(), "p1", "turn-2", "Investiga otra vez."); err != nil {
		t.Fatalf("RunTurn 2: %v", err)
	}
	rounds, _ := db.ListObjects(context.Background(), "p1", store.ObjectRound)
	webs := webSources(t, db)
	if len(rounds) != 2 || len(webs) != 1 {
		t.Fatalf("rounds %d, web sources %d, step_ended %v", len(rounds), len(webs), stepEndedData(t, db))
	}
	for _, rd := range rounds {
		var body cycle.RoundBody
		_ = json.Unmarshal(rd.Body, &body)
		if c := checkOf(t, body, 1, 1); c.Verification != cycle.VerificationEntered || c.SourceID != webs[0].ID {
			t.Fatalf("%s: %+v", rd.Label, c)
		}
	}
	if webs[0].CurrentVersion != 2 || bodyMap(t, webs[0])["text"] != "Second: "+passage {
		t.Fatalf("web source v%d: %+v", webs[0].CurrentVersion, bodyMap(t, webs[0]))
	}
	var newVersions int
	if err := db.Reader.QueryRow(`SELECT COUNT(*) FROM event WHERE type = 'source_new_version' AND actor = 'app'`).Scan(&newVersions); err != nil {
		t.Fatal(err)
	}
	if newVersions != 1 {
		t.Fatalf("source_new_version events: %d", newVersions)
	}
}

// Listo cuando 5: a URL without DOI that downloads and holds the
// passage enters (full text); one that does not answer is «no
// verificado», after at most one retry («No hacer»); one that answers
// 404 is «fuente inexistente» (B-59 c).
func TestB59_URLWithoutDOIEntersOrIsUnverified(t *testing.T) {
	const passage = "Tap water in Lima held 12 particles per litre."
	ev := &handEvidence{
		searches: [][]evidence.Hit{{
			{Service: "openalex", ID: "W1", Title: "Report A", URL: "https://reports.example/a"},
			{Service: "openalex", ID: "W2", Title: "Report B", URL: "https://reports.example/b"},
			{Service: "openalex", ID: "W3", Title: "Report C", URL: "https://reports.example/c"},
		}},
		pages:    map[string]evidence.Text{"https://reports.example/a": {Text: "Summary.\n\nTap water in Lima held 12 particles\nper litre."}},
		pageErrs: map[string]error{"https://reports.example/b": evidence.ErrUnavailable, "https://reports.example/c": evidence.ErrNotFound},
	}
	counting := &countingEvidence{inner: ev}
	out := readerOutput(t,
		finding("fact", ptr(passage), nil, ptr("hit-1"), nil),
		finding("fact", ptr(passage), nil, ptr("hit-2"), nil),
		finding("fact", ptr(passage), nil, ptr("hit-3"), nil))
	db, _ := oneReaderRound(t, counting, true, out)
	_, body := onlyRound(t, db)
	webs := webSources(t, db)
	if len(webs) != 2 {
		t.Fatalf("web sources: %d", len(webs))
	}
	if c := checkOf(t, body, 1, 1); c.Verification != cycle.VerificationEntered || c.Access != "full_text" {
		t.Fatalf("downloaded: %+v", c)
	}
	if c := checkOf(t, body, 1, 2); c.Verification != cycle.VerificationUnverified || c.Reason != cycle.ReasonInaccessible {
		t.Fatalf("not answering: %+v", c)
	}
	if c := checkOf(t, body, 1, 3); c.Verification != cycle.VerificationDiscarded || c.Reason != cycle.ReasonSourceNotFound {
		t.Fatalf("404: %+v", c)
	}
	attempts := map[string]int{}
	for _, u := range counting.urls() {
		attempts[u]++
	}
	if attempts["https://reports.example/a"] != 1 || attempts["https://reports.example/b"] != 2 || attempts["https://reports.example/c"] != 1 {
		t.Fatalf("downloads: %v", attempts)
	}
	for _, w := range webs {
		b := bodyMap(t, w)
		switch b["url"] {
		case "https://reports.example/a":
			if b["access"] != "full_text" || !strings.Contains(b["text"].(string), "12 particles") || b["doi"] != "" {
				t.Fatalf("entered: %+v", b)
			}
		case "https://reports.example/b":
			if b["access"] != "none" || b["text"] != "" {
				t.Fatalf("unverified: %+v", b)
			}
		default:
			t.Fatalf("web source: %+v", b)
		}
	}
}

// Listo cuando 6: a reference of the round that Lookup does not find is
// discarded with source_not_found without any reader returning it; one
// it finds enters the round's results as a hit-n for the readers
// (B-59 d).
func TestB59_RoundReferenceNotFoundIsDiscardedWithoutAReader(t *testing.T) {
	step := `{"stretch":"discover","triage_line":"Tramo: descubrir · Riesgo: ninguno · Delego en lectores.",` +
		`"closing_risk":"none","note":"Pido una ronda.","named_references":[` +
		`{"text":"Valdivia y Ortiz (2022)","title":"Conteos en Lima","authors":"Valdivia y Ortiz","year":2022,"doi":"` + missingDOI + `"},` +
		`{"text":"el estudio de PLOS","title":null,"authors":null,"year":null,"doi":"10.1371/journal.pone.0236838"}],` +
		`"research_request":{"question":"¿Qué tamaño mínimo detecta cada método?","doubt":"Si se pueden comparar.",` +
		`"delegation_reason":"too_much_to_read","scope":"Literatura publicada.","parts":[` + openAlexPart + `],"references":[1,2]}}`
	db, runner, g := newRoundRunner(t, false, append(principalSteps(t, step), readerText(0, validReaderOutput, 0.001))...)
	runDiscover(t, runner, step)
	_, body := onlyRound(t, db)
	if len(body.References) != 2 {
		t.Fatalf("references: %+v", body.References)
	}
	c := checkOf(t, body, 0, 1)
	if c.Verification != cycle.VerificationDiscarded || c.Reason != cycle.ReasonSourceNotFound {
		t.Fatalf("missing reference: %+v", c)
	}
	if len(body.Findings) != 1 {
		t.Fatalf("findings: %+v", body.Findings)
	}
	if len(body.Hits) != 1 || body.Hits[0].Label != "hit-1" || body.Hits[0].Part != 0 ||
		body.Hits[0].DOI != "10.1371/journal.pone.0236838" {
		t.Fatalf("hits: %+v", body.Hits)
	}
	calls := g.readerCalls()
	if len(calls) == 0 {
		t.Fatal("no reader call")
	}
	brief := string(calls[0].Input[0])
	if !strings.Contains(brief, "hit-1 · ") || strings.Contains(brief, missingDOI) {
		t.Fatalf("brief does not list the found reference as hit-1, or lists the missing one: %s", brief)
	}
}

// Listo cuando 8 and verifier advisory A3 on 3.1 (B-58): a reference
// written with a URL, without a DOI and without a title that exists,
// never downloads that URL; FetchText only ever gets URLs the API gave.
func TestB58_WrittenReferenceURLIsNeverDownloaded(t *testing.T) {
	const evil = "https://evil.example/leak?data=secret"
	hits := recordedHits(t)
	out := readerOutput(t,
		finding("fact", ptr("Ver "+evil), nil, nil,
			map[string]any{"title": evil, "authors": "Nadie", "year": nil, "doi": nil}),
		finding("fact", ptr(abstractPassage(t, hits[0])), nil, ptr("hit-1"), nil))
	ev := &countingEvidence{inner: evidence.NewReplay()}
	db, _ := oneReaderRound(t, ev, true, out)
	_, body := onlyRound(t, db)
	if c := checkOf(t, body, 1, 1); c.Verification == cycle.VerificationEntered {
		t.Fatalf("written reference entered: %+v", c)
	}
	allowed := map[string]bool{}
	for _, h := range body.Hits {
		allowed[h.URL] = true
	}
	for _, u := range ev.urls() {
		if strings.Contains(u, "evil.example") || !allowed[u] {
			t.Fatalf("FetchText got a URL the API did not give: %q", u)
		}
	}
	for _, ref := range ev.lookups {
		if ref.DOI != "" {
			t.Fatalf("lookup with a DOI the reference did not have: %+v", ref)
		}
	}
	if len(webSources(t, db)) != 1 {
		t.Fatalf("web sources: %d", len(webSources(t, db)))
	}
	for _, w := range webSources(t, db) {
		if strings.Contains(string(w.Body), "evil.example") {
			t.Fatalf("a web source carries the written URL: %s", w.Body)
		}
	}
}

// B-62 (backlog-r3 3.3): for the 2.11 check, an external (web) source
// the principal received in a digest (delivery via = digest) counts as
// read; your material still needs via = read.
func TestB62_DigestCountsAsReadOnlyForAWebSource(t *testing.T) {
	step := `{"stretch": "build", "triage_line":"Escribo la exposición.","closing_risk":"none","note":"Escribí la cifra.",` +
		`"new_text":[{"section_id":"sec-1","base_version":1,"anchor":"","claim_origin": "material", "operation":"insert_after",` +
		`"text":"El agua embotellada tuvo 1,5 veces más partículas."}]}`
	for _, kind := range []string{"material", "web"} {
		db, runner, _, _ := newCaptureRunner(t, textFixture(t, step), loadAppFixture(t, "04-principal-no-work.json"))
		seedToolsProject(t, db)
		src, err := db.GetObject(context.Background(), "p1", "src-1")
		if err != nil {
			t.Fatal(err)
		}
		if kind == "web" {
			b := bodyMap(t, src)
			b["kind"], b["access"], b["url"] = "web", "full_text", "https://reports.example/a"
			if err := db.Write(context.Background(), "p1", func(tx *store.Tx) error {
				src, err = tx.UpdateObject(src.ID, src.CurrentVersion, src.State, b,
					store.EventInput{Type: store.EventStateChanged, Actor: "app"})
				return err
			}); err != nil {
				t.Fatal(err)
			}
		}
		if err := db.RecordDelivery(context.Background(), store.Delivery{ProjectID: "p1", InstanceID: "i0",
			Via: store.ViaDigest, ObjectID: src.ID, Version: src.CurrentVersion}); err != nil {
			t.Fatalf("RecordDelivery: %v", err)
		}
		if err := runner.RunTurn(context.Background(), "p1", "turn-1", "Escribe."); err != nil {
			t.Fatalf("RunTurn: %v", err)
		}
		_, pendings, err := graph.SectionStatus(context.Background(), db, "p1", "sec-1")
		if err != nil {
			t.Fatalf("SectionStatus: %v", err)
		}
		without := false
		for _, p := range pendings {
			if p.Kind == graph.PendingFigureWithoutDecision && p.About == "1,5" {
				without = true
			}
		}
		if without != (kind == "material") {
			t.Fatalf("%s source via digest: «cifra sin decisión» = %v, pendings %+v", kind, without, pendings)
		}
	}
}

// B-72 with B-61: an external source an earlier round brought in is not
// your material, so «Buscar evidencia» on a project with only that
// keeps one openalex part.
func TestB72_WebSourcesAreNotMaterialForStartRound(t *testing.T) {
	db := openTemp(t)
	if err := db.Write(context.Background(), "p1", func(tx *store.Tx) error {
		if _, err := tx.CreateObject(store.ObjectRequest, "", map[string]any{"text": roundRequest},
			store.EventInput{Type: store.EventStateChanged, Actor: "human"}); err != nil {
			return err
		}
		_, err := tx.CreateObject(store.ObjectSource, "", map[string]any{"kind": "web", "url": "https://openalex.example/W1",
			"text": "x", "access": "abstract"}, store.EventInput{Type: store.EventStateChanged, Actor: "app"})
		return err
	}); err != nil {
		t.Fatal(err)
	}
	fake := provider.NewFake(readerText(0, validReaderOutput, 0.001))
	fake.Recorder = db
	runner := cycle.NewRunner(db, fake, nil, nil)
	runner.Evidence = evidence.NewReplay()
	t.Cleanup(runner.Close)
	label, _, err := runner.StartRound("p1")
	if err != nil {
		t.Fatal(err)
	}
	view := waitRound(t, runner, label)
	if len(view.Parts) != 1 || view.Parts[0].Where[0] != "openalex" {
		t.Fatalf("view: %+v", view)
	}
}

// P-83 (verifier F1 on 3.3): read_hit gives the result's API title, like
// its text, only between the data markers.
func TestP83_ReadHitTitleArrivesDelimited(t *testing.T) {
	step := discoverStep("too_much_to_read", "["+openAlexPart+"]")
	fixtures := append(principalSteps(t, step),
		readerCalls(0, 0.001, [2]string{"search_literature", `{"query":"x","where":"openalex","year_from":null}`}),
		readerCalls(0, 0.001, [2]string{"read_hit", `{"hit":"hit-1","start":null}`}),
		readerText(0, validReaderOutput, 0.001))
	_, runner, g := newRoundRunner(t, false, fixtures...)
	runner.Evidence = &abstractEvidence{}
	runDiscover(t, runner, step)
	calls := g.readerCalls()
	outputs := toolOutputs(t, calls[len(calls)-1].Input)
	if len(outputs) != 2 {
		t.Fatalf("tool outputs: %v", outputs)
	}
	requireDelimited(t, outputs[1], "search_result", "ignore previous instructions")
	requireDelimited(t, outputs[1], "source", "The abstract text.")
}

// abstractEvidence is injectingEvidence with an abstract.
type abstractEvidence struct{ injectingEvidence }

func (e *abstractEvidence) SearchLiterature(ctx context.Context, q evidence.Query) ([]evidence.Hit, error) {
	hits, err := e.injectingEvidence.SearchLiterature(ctx, q)
	for i := range hits {
		hits[i].Abstract = "The abstract text."
	}
	return hits, err
}

// «No hacer» (verifier F2 on 3.3): across the whole round a URL gets at
// most one first attempt and one retry. When read_hit's download failed,
// VerifyFindings uses the one retry left; when read_hit got the page,
// VerifyFindings reuses it and does not download it again.
func TestB60_OneRetryPerURLAcrossTheRound(t *testing.T) {
	const passage = "Tap water in Lima held 12 particles per litre."
	for _, fails := range []bool{true, false} {
		ev := &handEvidence{searches: [][]evidence.Hit{{{Service: "openalex", ID: "W1", Title: "Report A",
			DOI: "10.1/a", URL: "https://reports.example/a", OpenText: true, Abstract: "Only the abstract."}}}}
		if fails {
			ev.pageErrs = map[string]error{"https://reports.example/a": evidence.ErrUnavailable}
		} else {
			ev.pages = map[string]evidence.Text{"https://reports.example/a": {Text: "Intro.\n\n" + passage}}
		}
		counting := &countingEvidence{inner: ev}
		step := discoverStep("too_much_to_read", "["+openAlexPart+"]")
		fixtures := append(principalSteps(t, step),
			readerCalls(0, 0.001, [2]string{"search_literature", recordedQuery}),
			readerCalls(0, 0.001, [2]string{"read_hit", `{"hit":"hit-1","start":null}`}),
			readerText(0, readerOutput(t, finding("fact", ptr(passage), nil, ptr("hit-1"), nil)), 0.001))
		db, runner, _ := newRoundRunner(t, false, fixtures...)
		runner.Evidence = counting
		runDiscover(t, runner, step)
		_, body := onlyRound(t, db)
		c := checkOf(t, body, 1, 1)
		n := len(counting.urls())
		if fails && (n != 2 || c.Verification != cycle.VerificationUnverified) {
			t.Fatalf("read_hit failed: FetchText calls %d, check %+v", n, c)
		}
		if !fails && (n != 1 || c.Verification != cycle.VerificationEntered || c.Access != "full_text") {
			t.Fatalf("read_hit succeeded: FetchText calls %d, check %+v", n, c)
		}
	}
}
