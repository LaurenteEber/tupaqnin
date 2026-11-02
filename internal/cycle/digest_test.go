package cycle_test

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/LaurenteEber/tupaqnin/internal/checks"
	"github.com/LaurenteEber/tupaqnin/internal/cycle"
	"github.com/LaurenteEber/tupaqnin/internal/evidence"
	"github.com/LaurenteEber/tupaqnin/internal/graph"
	"github.com/LaurenteEber/tupaqnin/internal/memory"
	"github.com/LaurenteEber/tupaqnin/internal/provider"
	"github.com/LaurenteEber/tupaqnin/internal/store"
)

// Backlog-r3 3.4: the digest, the coverage inventory and the conflicts
// between sources (fichas §2 and §7, flujos §2, B-63, B-66, estados §7).

const (
	// The passage of src-1 (roundSource) a reader quotes, and the one of
	// src-2 that clashes with it.
	passageOne  = "En 2023 los conteos fueron 60 veces mayores."
	sourceTwo   = "Un segundo estudio. En 2023 los conteos fueron similares en ambos tipos de agua."
	passageTwo  = "En 2023 los conteos fueron similares en ambos tipos de agua."
	digestQuery = `{"query":"microplastics","where":"openalex","year_from":null}`
)

// projectPart is a round part that reads the given project sources.
func projectPart(sources ...string) string {
	var list []string
	for _, s := range sources {
		list = append(list, `{"source_id":"`+s+`","what_for":"Las cifras."}`)
	}
	return `{"question":"¿Qué dicen las cifras?","sources":[` + strings.Join(list, ",") + `],"where":["project"],"queries":[]}`
}

// projectFinding is a salida-lector finding on a project source.
func projectFinding(kind, passage, source string) map[string]any {
	return finding(kind, ptr(passage), ptr(source), nil, nil)
}

// digestFixture is a hand-made digest call answering out.
func digestFixture(t *testing.T, out map[string]any) provider.Fixture {
	t.Helper()
	text, err := json.Marshal(out)
	if err != nil {
		t.Fatal(err)
	}
	item, _ := json.Marshal(map[string]any{"type": "message", "role": "assistant",
		"content": []map[string]string{{"type": "output_text", "text": string(text)}}})
	return provider.Fixture{TurnKind: provider.TurnDigest, Items: []json.RawMessage{item}, Usage: provider.Usage{CostUSD: 0.002}}
}

// recordedDigest is a digest output from internal/provider/testdata/
// digest: hand-made until `make microtest-digest` records Meta's.
func recordedDigest(t *testing.T, name string) provider.Fixture {
	t.Helper()
	fx, err := provider.LoadFixture(filepath.Join("..", "provider", "testdata", "digest", name+".json"))
	if err != nil {
		t.Fatalf("digest recording %s: %v", name, err)
	}
	return fx
}

// plainDigest is a digest that uses finding 1 with the doubt as given.
func plainDigest(t *testing.T, reduced string) provider.Fixture {
	return digestFixture(t, map[string]any{"answer": "Los conteos de 2023 fueron mucho mayores.",
		"outline": []string{"Conteos de 2023."}, "findings": []map[string]int{{"finding": 1, "outline_line": 1}},
		"expectation_contrast": "Coincide en parte.", "reduced_doubt": reduced, "contradictions": []any{}})
}

// seedSourceAndSection adds src-2 (sourceTwo) and sec-1 (with role)
// using the given sources; with decision, dec-1 is used in sec-1.
func seedSourceAndSection(t *testing.T, db *store.DB, role string, decision bool, uses ...string) {
	t.Helper()
	ctx := context.Background()
	human := store.EventInput{Type: store.EventStateChanged, Actor: "human"}
	edge := store.EventInput{Type: store.EventEdge, Actor: "human"}
	err := db.Write(ctx, "p1", func(tx *store.Tx) error {
		if _, err := tx.CreateObject(store.ObjectSource, "", map[string]any{"kind": "file", "filename": "segundo.md",
			"text": sourceTwo}, human); err != nil {
			return err
		}
		sec, err := tx.CreateObject(store.ObjectSection, "draft", map[string]any{"title": "Conteos",
			"text": "Los conteos de 2023 cambian la comparación.", "role": map[string]any{"value": role, "state": "confirmed"},
			"open_changes": []any{}}, human)
		if err != nil {
			return err
		}
		for _, label := range uses {
			src, err := tx.GetObject(ctx, "p1", label)
			if err != nil {
				return err
			}
			if _, err := tx.AddEdge(store.EdgeUses, sec.ID, src.ID, "", nil, edge); err != nil {
				return err
			}
		}
		if decision {
			dec, err := tx.GetObject(ctx, "p1", "dec-1")
			if err != nil {
				return err
			}
			if _, err := tx.AddEdge(store.EdgeUsedIn, dec.ID, sec.ID, "", nil, edge); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("seed: %v", err)
	}
}

// digestRound runs a discover step whose parts read project sources,
// one reader output per part, then the digest fixtures (and any other).
// seed runs before the step (src-2 and sec-1).
func digestRound(t *testing.T, seed func(*store.DB), parts []string, outputs []string, more ...provider.Fixture) (*store.DB, *cycle.Runner, *gatedProvider) {
	t.Helper()
	step := discoverStep("independent_parts", "["+strings.Join(parts, ",")+"]")
	if len(parts) == 1 {
		step = discoverStep("too_much_to_read", "["+parts[0]+"]")
	}
	fixtures := principalSteps(t, step)
	for i, out := range outputs {
		fixtures = append(fixtures, readerText(i+1, out, 0.001))
	}
	fixtures = append(fixtures, more...)
	db, runner, g := newRoundRunner(t, false, fixtures...)
	runner.Evidence = &handEvidence{}
	if seed != nil {
		seed(db)
	}
	runDiscover(t, runner, step)
	return db, runner, g
}

func roundView(t *testing.T, runner *cycle.Runner, label string) cycle.RoundView {
	t.Helper()
	v, err := runner.Round(context.Background(), "p1", label)
	if err != nil {
		t.Fatalf("Round: %v", err)
	}
	return v
}

// Listo cuando 1 (B-63): with the recorded digest (hand-made until Eber's
// Meta run), the digest brings the discarded with their reason although
// the principal names none of them; the digest is in the principal's
// live instance at its usual effort, records delivery via = digest of
// the source it shows, and goes through the checks with the passages it
// uses (2.3).
func TestB63_DigestBringsTheDiscardedWithTheirReasonEvenIfThePrincipalNamesNone(t *testing.T) {
	out := readerOutput(t,
		projectFinding("fact", passageOne, "src-1"),
		projectFinding("fact", "En 2023 los conteos fueron 90 veces mayores.", "src-1"),
		projectFinding("fact", "Un pasaje cualquiera.", "src-9"))
	db, runner, g := digestRound(t, nil, []string{projectPart("src-1")}, []string{out},
		recordedDigest(t, "discarded-unnamed"))

	round, body := onlyRound(t, db)
	if round.State != "closed" || body.DigestStatus != cycle.DigestDone ||
		body.ReducedDoubt != "yes" && body.ReducedDoubt != "partly" && body.ReducedDoubt != "no" {
		t.Fatalf("round: %s %s %q", round.State, body.DigestStatus, body.ReducedDoubt)
	}
	// The principal received only the finding that entered, numbered.
	if len(body.DigestFindings) != 1 || body.DigestFindings[0].Passage != passageOne {
		t.Fatalf("digest findings: %+v", body.DigestFindings)
	}
	v := roundView(t, runner, round.Label)
	reasons := map[string]string{}
	for _, d := range v.Discarded {
		reasons[d.Passage] = d.Reason
	}
	if len(v.Discarded) != 2 || reasons["En 2023 los conteos fueron 90 veces mayores."] != cycle.ReasonPassageNotFound ||
		reasons["Un pasaje cualquiera."] != cycle.ReasonSourceNotFound {
		t.Fatalf("discarded: %+v", v.Discarded)
	}
	// The recording (hand-made, or Meta's from make microtest-digest)
	// uses finding 1 on some outline line, and contrasts the expectation.
	if v.Digest == nil {
		t.Fatal("no digest in the card")
	}
	used := false
	for _, line := range v.Digest.Outline {
		for _, f := range line.Findings {
			used = used || f.N == 1 && f.Source == "src-1" && f.Passage == passageOne
		}
	}
	if v.Digest == nil || v.Digest.Answer == "" || !used || v.Digest.ExpectationContrast == "" {
		t.Fatalf("digest view: %+v", v.Digest)
	}

	// The digest call: the principal's live instance (the discover
	// step's), its model and its usual effort, the round's turn.
	digests := g.callsOf(provider.TurnDigest)
	if len(digests) != 1 {
		t.Fatalf("digest calls: %d", len(digests))
	}
	d := digests[0]
	var stepInstance string
	if err := db.Reader.QueryRow(`SELECT instance_id FROM model_call WHERE role = 'principal' AND turn_id = 'turn-1' LIMIT 1`).
		Scan(&stepInstance); err != nil {
		t.Fatal(err)
	}
	if d.InstanceID == "" || d.InstanceID != stepInstance || d.Role != provider.RolePrincipal ||
		d.Model != provider.PrincipalModel || d.Effort != provider.PrincipalEffort || d.TurnID != body.TurnID {
		t.Fatalf("digest call: instance %q (step %q) %s %s %s turn %s", d.InstanceID, stepInstance, d.Role, d.Model, d.Effort, d.TurnID)
	}
	// B-62: delivery via = digest of the source it shows.
	src, _ := object(t, db, "src-1")
	var n int
	if err := db.Reader.QueryRow(`SELECT COUNT(*) FROM delivery WHERE via = 'digest' AND object_id = ? AND instance_id = ?`,
		src.ID, stepInstance).Scan(&n); err != nil || n != 1 {
		t.Fatalf("delivery via digest of src-1: %d, %v", n, err)
	}
	// 2.3: one checker call on the digest's change, with the passage it
	// uses, its kind and its source; its probe events name the round.
	checker := g.callsOf(provider.TurnChecker)
	if len(checker) != 1 {
		t.Fatalf("checker calls: %d", len(checker))
	}
	brief := inputText(t, checker[0].Input)
	if !strings.Contains(brief, "digest of round "+round.Label) || !strings.Contains(brief, "Finding of kind fact, source src-1") {
		t.Fatalf("checker brief:\n%s", brief)
	}
	requireDelimited(t, brief, graph.DataPassage, passageOne)
	probes := 0
	for _, ev := range eventsOfType(t, db, store.EventProbe) {
		if secs, _ := eventData(t, ev)["sections"].([]any); len(secs) == 1 && secs[0] == round.Label {
			probes++
		}
	}
	if probes == 0 {
		t.Fatal("no probe event names the round")
	}
}

// inputText joins the text of every input item.
func inputText(t *testing.T, items []json.RawMessage) string {
	t.Helper()
	var b strings.Builder
	for _, raw := range items {
		var item struct {
			Content []struct {
				Text string `json:"text"`
			} `json:"content"`
			Output string `json:"output"`
		}
		_ = json.Unmarshal(raw, &item)
		for _, c := range item.Content {
			b.WriteString(c.Text)
			b.WriteString("\n")
		}
		b.WriteString(item.Output)
	}
	return b.String()
}

// Listo cuando 2 (A-5): a reader that read 3 of its 5 results leaves
// «2 de 5 resultados sin leer» in the inventory, kept in
// round.body.coverage_inventory.
func TestA5_InventorySaysTwoOfFiveResultsUnreadWhenAReaderReadThree(t *testing.T) {
	var hits []evidence.Hit
	for i := 1; i <= 5; i++ {
		n := string(rune('0' + i))
		hits = append(hits, evidence.Hit{Service: "openalex", ID: "W" + n, Title: "Estudio " + n, Year: 2020,
			DOI: "10.1/" + n, URL: "https://example.org/" + n})
	}
	out := `{"findings":[{"kind":"gap","passage":null,"source_id":null,"hit":null,"reference":null,"note":"Nada sobre 20 micras."}],` +
		`"covered":["crit-1"],"sources_read":["hit-1","hit-2","hit-3"]}`
	step := discoverStep("too_much_to_read", "["+openAlexPart+"]")
	fixtures := append(principalSteps(t, step), readerCalls(0, 0.001, [2]string{"search_literature", digestQuery}),
		readerText(0, out, 0.001))
	db, runner, _ := newRoundRunner(t, false, fixtures...)
	runner.Evidence = &handEvidence{searches: [][]evidence.Hit{hits}}
	runDiscover(t, runner, step)

	round, body := onlyRound(t, db)
	if len(body.CoverageInventory) != 1 {
		t.Fatalf("inventory: %+v", body.CoverageInventory)
	}
	inv := body.CoverageInventory[0]
	if len(inv.Hits) != 5 || len(inv.HitsRead) != 3 || len(inv.Sources) != 1 || len(inv.SourcesRead) != 0 ||
		len(inv.Gaps) != 1 || inv.Gaps[0] != "Nada sobre 20 micras." {
		t.Fatalf("inventory: %+v", inv)
	}
	v := roundView(t, runner, round.Label)
	c := v.Coverage[0]
	if c.Hits != 5 || strings.Join(c.HitsUnread, ",") != "hit-4,hit-5" || c.Sources != 1 ||
		strings.Join(c.SourcesUnread, ",") != "src-1" {
		t.Fatalf("coverage view: %+v", c)
	}
}

// Listo cuando 3 (B-66, R-9): a contradiction between two sources opens
// a «por aclarar» source_conflict with the two passages as its options
// and blocks the signoff of the section that uses one of them.
func TestB66_ContradictionBetweenTwoSourcesOpensSourceConflictAndBlocksTheSignoffOfItsSection(t *testing.T) {
	db, runner, _ := digestRound(t, func(db *store.DB) { seedSourceAndSection(t, db, "other", false, "src-2") },
		[]string{projectPart("src-1"), projectPart("src-2")},
		[]string{readerOutput(t, projectFinding("fact", passageOne, "src-1")),
			readerOutput(t, projectFinding("fact", passageTwo, "src-2"))},
		recordedDigest(t, "conflict-two-sources"))

	round, body := onlyRound(t, db)
	if round.State != "closed" || len(body.Conflicts) != 1 || body.Conflicts[0].SameSource || body.Conflicts[0].OpenQuestionID == "" {
		t.Fatalf("round %s conflicts %+v", round.State, body.Conflicts)
	}
	oq, b := object(t, db, "oq-1")
	opts, _ := b["options"].([]any)
	passages := map[any]bool{}
	for _, o := range opts {
		passages[o.(map[string]any)["option"]] = true
	}
	if oq.State != "open" || b["kind"] != cycle.OpenQuestionSourceConflict || b["origin"] != "digest" || len(opts) != 2 ||
		!passages[passageOne] || !passages[passageTwo] || b["question"] != body.Conflicts[0].About {
		t.Fatalf("oq-1: %s %v", oq.State, b)
	}
	if got := currentBlocks(t, db, "oq-1"); len(got) != 1 || got[0] != "sec-1" {
		t.Fatalf("blocks: %v, want sec-1", got)
	}
	open, err := cycle.NewGate(db).RequestSignoff(context.Background(), "p1", "sec-1")
	if err != nil || open.Opened || !hasPending(open.Pendings, graph.PendingOpenQuestion, "oq-1") {
		t.Fatalf("RequestSignoff: %+v, %v; want blocked by oq-1", open, err)
	}
	v := roundView(t, runner, round.Label)
	if v.Digest == nil || len(v.Digest.Contradictions) != 1 || v.Digest.Contradictions[0].OpenQuestion != "oq-1" ||
		v.Digest.Contradictions[0].A.Passage == v.Digest.Contradictions[0].B.Passage {
		t.Fatalf("contradictions: %+v", v.Digest)
	}
}

// Listo cuando 4 (flujos §2, estados §4, B-66): a contradiction between
// readers on the same source opens a «por aclarar» only when it touches
// a critical decision; otherwise it is only shown in the digest.
func TestB66_ContradictionBetweenReadersOnTheSameSourceOpensOnlyIfItTouchesACriticalDecision(t *testing.T) {
	sameSource := func(t *testing.T, role string, decision bool) (*store.DB, cycle.RoundBody, *cycle.Runner) {
		db, runner, _ := digestRound(t, func(db *store.DB) { seedSourceAndSection(t, db, role, decision, "src-1") },
			[]string{projectPart("src-1"), projectPart("src-1")},
			[]string{readerOutput(t, projectFinding("fact", passageOne, "src-1")),
				readerOutput(t, projectFinding("contradiction", "Los filtros finos retienen más partículas.", "src-1"))},
			digestFixture(t, map[string]any{"answer": "Los lectores leen distinto la misma fuente.",
				"outline": []string{"Dos lecturas."}, "findings": []map[string]int{{"finding": 1, "outline_line": 1}},
				"expectation_contrast": "No se contrasta.", "reduced_doubt": "partly",
				"contradictions": []map[string]any{{"finding_a": 1, "finding_b": 2, "about": "Qué explica los conteos"}}}))
		_, body := onlyRound(t, db)
		return db, body, runner
	}
	t.Run("not critical", func(t *testing.T) {
		db, body, runner := sameSource(t, "other", false)
		if len(body.Conflicts) != 1 || !body.Conflicts[0].SameSource || body.Conflicts[0].OpenQuestionID != "" {
			t.Fatalf("conflicts: %+v", body.Conflicts)
		}
		if oqs, _ := db.ListObjects(context.Background(), "p1", store.ObjectOpenQuestion); len(oqs) != 0 {
			t.Fatalf("open questions: %d, want none", len(oqs))
		}
		v := roundView(t, runner, "rnd-1")
		if len(v.Digest.Contradictions) != 1 || !v.Digest.Contradictions[0].SameSource {
			t.Fatalf("shown: %+v", v.Digest.Contradictions)
		}
	})
	t.Run("critical", func(t *testing.T) {
		// dec-1 used in a conclusion section is critical (fichas §3).
		db, body, _ := sameSource(t, "conclusion", true)
		if len(body.Conflicts) != 1 || body.Conflicts[0].OpenQuestionID == "" {
			t.Fatalf("conflicts: %+v", body.Conflicts)
		}
		_, b := object(t, db, "oq-1")
		if b["kind"] != cycle.OpenQuestionReaderContradiction {
			t.Fatalf("oq-1: %v", b)
		}
		if got := currentBlocks(t, db, "oq-1"); len(got) != 1 || got[0] != "sec-1" {
			t.Fatalf("blocks: %v, want sec-1", got)
		}
	})
}

// Listo cuando 5 (flujos §2, 2.3): a reader's classification with a
// term of yours in a doubtful case reaches the checker as a passage of
// the digest, and its doubtful_term opens a «por aclarar».
func TestB44_AReaderClassificationWithYourTermInADoubtfulCaseOpensAnOpenQuestion(t *testing.T) {
	const classification = "Es un estudio de campo: muestreo en una planta piloto."
	reader := readerOutput(t, finding("fact", ptr(passageOne), ptr("src-1"), nil, nil))
	// The reader's note is its classification.
	reader = strings.Replace(reader, `"note":null`, `"note":"`+classification+`"`, 1)
	db0 := openTemp(t)
	seedRound(t, db0, false)
	probes, err := checks.Probes(context.Background(), db0, "p1", checks.Change{})
	if err != nil {
		t.Fatal(err)
	}
	var verdicts []map[string]any
	for _, p := range probes {
		verdicts = append(verdicts, map[string]any{"protected": p.Protected, "verdict": "no", "passage": nil})
	}
	checkerOut, _ := json.Marshal(map[string]any{"probes": verdicts, "candidates": []map[string]any{{
		"type": "doubtful_term", "section": "rnd-1", "new_passage": classification, "clashing_passage": roundCounts,
		"protected": "crit-1", "citation": nil, "question": "¿Una planta piloto cuenta como estudio de campo?",
		"options": []map[string]string{
			{"option": "Cuenta", "contrast": "Agua real.", "consequence": "Entra.", "risk": "Mezcla."},
			{"option": "No cuenta", "contrast": "Piloto.", "consequence": "Sale.", "risk": "Se pierde."}}}}})
	item, _ := json.Marshal(map[string]any{"type": "message", "role": "assistant",
		"content": []map[string]string{{"type": "output_text", "text": string(checkerOut)}}})
	checkerFx := provider.Fixture{TurnKind: provider.TurnChecker, Items: []json.RawMessage{item}}

	db, _, _ := digestRound(t, func(db *store.DB) { seedSourceAndSection(t, db, "other", false, "src-1") },
		[]string{projectPart("src-1")}, []string{reader}, plainDigest(t, "yes"), checkerFx)

	oqs, err := db.ListObjects(context.Background(), "p1", store.ObjectOpenQuestion)
	if err != nil || len(oqs) != 1 {
		t.Fatalf("open questions: %d, %v", len(oqs), err)
	}
	_, b := object(t, db, oqs[0].Label)
	round, _ := object(t, db, "rnd-1")
	if b["origin"] != "checker" || b["kind"] != "doubtful_term" || b["round_id"] != round.ID ||
		b["new_passage"] != classification {
		t.Fatalf("%s: %v", oqs[0].Label, b)
	}
	// It blocks the section that uses the finding's source.
	if got := currentBlocks(t, db, oqs[0].Label); len(got) != 1 || got[0] != "sec-1" {
		t.Fatalf("blocks: %v, want sec-1", got)
	}
}

// Listo cuando 6 (flujos §2, fichas §2): reduced_doubt «no» (or
// «partly») offers repeating with another question, with its cost;
// «yes» offers nothing. Nothing repeats without your click.
func TestK5_ReducedDoubtNoOffersRepeatingWithItsCost(t *testing.T) {
	db, runner, _ := digestRound(t, nil, []string{projectPart("src-1")},
		[]string{readerOutput(t, projectFinding("fact", passageOne, "src-1"))}, plainDigest(t, "no"))
	v := roundView(t, runner, "rnd-1")
	if v.ReducedDoubt != "no" || v.Repeat == nil || v.Repeat.EstimatedCostUSD != cycle.EstimatedRoundCost(1) ||
		v.Repeat.EstimatedCostUSD <= 0 {
		t.Fatalf("view: reduced %q repeat %+v", v.ReducedDoubt, v.Repeat)
	}
	if rounds, _ := db.ListObjects(context.Background(), "p1", store.ObjectRound); len(rounds) != 1 {
		t.Fatalf("rounds: %d; a round never repeats without your click", len(rounds))
	}

	_, yesRunner, _ := digestRound(t, nil, []string{projectPart("src-1")},
		[]string{readerOutput(t, projectFinding("fact", passageOne, "src-1"))}, plainDigest(t, "yes"))
	if v := roundView(t, yesRunner, "rnd-1"); v.Repeat != nil {
		t.Fatalf("yes offers a repeat: %+v", v.Repeat)
	}
}

// Fichas §2 (repetir lo decides tú): your click repeats the round with
// another question, with the same parts, and runs it; the same question
// or a round whose doubt was reduced is refused.
func TestK5_RepeatWithAnotherQuestionCreatesTheRoundOnYourClick(t *testing.T) {
	reader := readerOutput(t, projectFinding("fact", passageOne, "src-1"))
	db, runner, _ := digestRound(t, nil, []string{projectPart("src-1")}, []string{reader},
		plainDigest(t, "no"), readerText(0, reader, 0.001))
	if _, _, _, err := runner.RepeatRound("p1", "rnd-1", "¿Qué tamaño mínimo detecta cada método?"); !errors.Is(err, cycle.ErrQuestionRequired) {
		t.Fatalf("same question: %v", err)
	}
	label, turnID, cost, err := runner.RepeatRound("p1", "rnd-1", "¿Desde qué tamaño se contaron las partículas?")
	if err != nil || label != "rnd-2" || turnID == "" || cost != cycle.EstimatedRoundCost(1) {
		t.Fatalf("RepeatRound: %s %s %v %v", label, turnID, cost, err)
	}
	waitRound(t, runner, "rnd-2")
	runner.Close()
	old, _ := object(t, db, "rnd-1")
	next, _ := object(t, db, "rnd-2")
	var body cycle.RoundBody
	_ = json.Unmarshal(next.Body, &body)
	if body.Question != "¿Desde qué tamaño se contaron las partículas?" || body.Repeats != old.ID || len(body.Parts) != 1 ||
		next.State != "closed" {
		t.Fatalf("rnd-2: %s %+v", next.State, body)
	}
}

// Flujos §2, estados §4: «dejarlo por aclarar» opens a scope «por
// aclarar» through 2.4's path, blocking the section that uses the
// round's source; once.
func TestE4_LeavingTheRoundToClarifyOpensAScopeOpenQuestion(t *testing.T) {
	db, runner, _ := digestRound(t, func(db *store.DB) { seedSourceAndSection(t, db, "other", false, "src-1") },
		[]string{projectPart("src-1")}, []string{readerOutput(t, projectFinding("fact", passageOne, "src-1"))},
		plainDigest(t, "partly"))
	label, err := runner.LeaveRoundOpen(context.Background(), "p1", "rnd-1")
	if err != nil || label != "oq-1" {
		t.Fatalf("LeaveRoundOpen: %s %v", label, err)
	}
	_, b := object(t, db, "oq-1")
	opts, _ := b["options"].([]any)
	if b["kind"] != cycle.OpenQuestionScope || b["question"] != "Si los estudios con distinto tamaño mínimo se pueden comparar." ||
		len(opts) != 2 || opts[0].(map[string]any)["code"] != cycle.ScopeOptionCompleteReview {
		t.Fatalf("oq-1: %v", b)
	}
	if got := currentBlocks(t, db, "oq-1"); len(got) != 1 || got[0] != "sec-1" {
		t.Fatalf("blocks: %v", got)
	}
	if v := roundView(t, runner, "rnd-1"); v.ScopeQuestion != "oq-1" {
		t.Fatalf("view scope question: %q", v.ScopeQuestion)
	}
	if _, err := runner.LeaveRoundOpen(context.Background(), "p1", "rnd-1"); !errors.Is(err, cycle.ErrScopeAlreadyOpen) {
		t.Fatalf("twice: %v", err)
	}
}

// Listo cuando 7 (estados §7): the round is closed in the three cases,
// with reduced_doubt in its body and in its closing event.
func TestE7_TheRoundIsClosedInTheThreeCases(t *testing.T) {
	for _, reduced := range []string{"yes", "partly", "no"} {
		t.Run(reduced, func(t *testing.T) {
			db, _, _ := digestRound(t, nil, []string{projectPart("src-1")},
				[]string{readerOutput(t, projectFinding("fact", passageOne, "src-1"))}, plainDigest(t, reduced))
			round, body := onlyRound(t, db)
			if round.State != "closed" || body.ReducedDoubt != reduced {
				t.Fatalf("round: %s %q", round.State, body.ReducedDoubt)
			}
			closed := 0
			for _, ev := range eventsOfType(t, db, store.EventStateChanged) {
				var data map[string]any
				_ = json.Unmarshal(ev.Data, &data)
				if ev.ObjectID == round.ID && data["to"] == "closed" && data["reduced_doubt"] == reduced {
					closed++
				}
			}
			if closed != 1 {
				t.Fatalf("closing events: %d", closed)
			}
		})
	}
}

// Fichas §7: the digest has a fixed shape and one retry; a finding
// number outside the list is refused, and after two failures the round
// closes without reduced_doubt, as the agent's pending.
func TestB63_DigestWithoutItsFixedShapeIsRetriedOnceThenLeftPending(t *testing.T) {
	bad := digestFixture(t, map[string]any{"answer": "a", "outline": []string{"x"},
		"findings": []map[string]int{{"finding": 9, "outline_line": 1}}, "expectation_contrast": "c",
		"reduced_doubt": "yes", "contradictions": []any{}})
	reader := readerOutput(t, projectFinding("fact", passageOne, "src-1"))

	t.Run("retry passes", func(t *testing.T) {
		db, _, g := digestRound(t, nil, []string{projectPart("src-1")}, []string{reader}, bad, plainDigest(t, "yes"))
		_, body := onlyRound(t, db)
		if body.DigestStatus != cycle.DigestDone || body.ReducedDoubt != "yes" || len(g.callsOf(provider.TurnDigest)) != 2 {
			t.Fatalf("body: %s %q, calls %d", body.DigestStatus, body.ReducedDoubt, len(g.callsOf(provider.TurnDigest)))
		}
		retries := eventsOfType(t, db, store.EventOutputRetry)
		if len(retries) != 1 || eventData(t, retries[0])["turn_kind"] != "digest" || eventData(t, retries[0])["result"] != "retry" {
			t.Fatalf("output_retry: %d", len(retries))
		}
		if last := g.callsOf(provider.TurnDigest)[1]; !strings.Contains(inputText(t, last.Input), "9 is not in the list of findings") {
			t.Fatal("the retry does not say why")
		}
	})
	t.Run("twice", func(t *testing.T) {
		db, _, _ := digestRound(t, nil, []string{projectPart("src-1")}, []string{reader}, bad)
		round, body := onlyRound(t, db)
		if round.State != "closed" || body.DigestStatus != cycle.DigestInvalidOutput || body.ReducedDoubt != "" || body.Digest != nil {
			t.Fatalf("round: %s %+v", round.State, body.DigestStatus)
		}
		retries := eventsOfType(t, db, store.EventOutputRetry)
		if len(retries) != 2 || eventData(t, retries[1])["result"] != "agent_pending" {
			t.Fatalf("output_retry: %d", len(retries))
		}
	})
}

// P-83: every passage and note in the digest's input arrives between
// the source markers, and the round reaches the principal in what
// changed as a line, never with its body (readers' outputs, abstracts).
func TestP83_DigestInputDelimitsPassagesAndTheRoundArrivesWithoutItsBody(t *testing.T) {
	seed := func(db *store.DB) {
		human := store.EventInput{Type: store.EventStateChanged, Actor: "human"}
		if err := db.Write(context.Background(), "p1", func(tx *store.Tx) error {
			_, err := tx.CreateObject(store.ObjectSource, "", map[string]any{"kind": "file", "filename": "x.md",
				"text": "Primero. " + injection}, human)
			return err
		}); err != nil {
			t.Fatal(err)
		}
	}
	db, _, g := digestRound(t, seed, []string{projectPart("src-2")},
		[]string{readerOutput(t, projectFinding("fact", injection, "src-2"))}, plainDigest(t, "yes"))
	digests := g.callsOf(provider.TurnDigest)
	if len(digests) != 1 {
		t.Fatalf("digest calls: %d", len(digests))
	}
	last := inputText(t, digests[0].Input[len(digests[0].Input)-1:])
	requireDelimited(t, last, graph.DataSource, injection)
	// (Your material's first lines in the startup index are outside this
	// piece: P-83 delimited the external sources' there, 3.3.) The round
	// reaches the next step as a line, without its body.
	item, err := memory.New(db).Read(context.Background(), "p1", "", "rnd-1")
	if err != nil || !strings.HasPrefix(item.Text, "Round (closed): ") || strings.Contains(item.Text, injection) ||
		strings.Contains(item.Text, `"readers"`) {
		t.Fatalf("the round as the principal reads it: %q, %v", item.Text, err)
	}
	changes, err := memory.New(db).ChangesSinceDelivery(context.Background(), "p1")
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range changes {
		if c.Label == "rnd-1" && !strings.HasPrefix(c.FirstLine, "Round (closed): ") {
			t.Fatalf("what changed: %q", c.FirstLine)
		}
	}
}

// The embedded digest schema is the design's, byte for byte, and every
// hand-made or recorded digest output has its fixed shape.
func TestB63_EmbeddedDigestSchemaIsTheDesignSchemaAndRecordingsHaveIt(t *testing.T) {
	embedded, err := os.ReadFile(filepath.Join("schema", "digest_output.schema.json"))
	if err != nil {
		t.Fatal(err)
	}
	design, err := os.ReadFile(filepath.Join("..", "..", "docs", "diseno", "bloque-3", "salida-digesto.schema.json"))
	if err != nil {
		t.Fatal(err)
	}
	if string(embedded) != string(design) {
		t.Fatal("embedded schema differs from docs/diseno/bloque-3/salida-digesto.schema.json")
	}
	entries, err := os.ReadDir(filepath.Join("..", "provider", "testdata", "digest"))
	if err != nil || len(entries) == 0 {
		t.Fatalf("digest recordings: %d, %v", len(entries), err)
	}
	for _, e := range entries {
		fx := recordedDigest(t, strings.TrimSuffix(e.Name(), ".json"))
		var text string
		for _, item := range fx.Items {
			text += inputText(t, []json.RawMessage{item})
		}
		if errs := cycle.ValidateDigestOutput(strings.TrimSpace(text)); len(errs) > 0 || fx.TurnKind != provider.TurnDigest {
			t.Fatalf("%s: %v (turn kind %q)", e.Name(), errs, fx.TurnKind)
		}
	}
}

// Verifier F1 (3.4 «No hacer», flujos §2): a reader's contradiction the
// principal leaves out of its digest is still shown with its passage,
// and every entered finding the digest does not use is listed; when it
// touches nothing critical it opens no «por aclarar».
func TestF2_AnIgnoredReaderContradictionIsShownAndUnusedFindingsAreListed(t *testing.T) {
	const clash = "Los filtros finos retienen más partículas."
	reader := readerOutput(t, projectFinding("fact", passageOne, "src-1"), projectFinding("contradiction", clash, "src-1"))
	db, runner, _ := digestRound(t, nil, []string{projectPart("src-1")}, []string{reader}, plainDigest(t, "yes"))
	v := roundView(t, runner, "rnd-1")
	if len(v.Unused) != 1 || v.Unused[0].Passage != clash || v.Unused[0].N != 2 {
		t.Fatalf("unused: %+v", v.Unused)
	}
	if len(v.ReaderContradictions) != 1 || v.ReaderContradictions[0].Passage != clash ||
		v.ReaderContradictions[0].Kind != "contradiction" {
		t.Fatalf("reader contradictions: %+v", v.ReaderContradictions)
	}
	if oqs, _ := db.ListObjects(context.Background(), "p1", store.ObjectOpenQuestion); len(oqs) != 0 {
		t.Fatalf("open questions: %d, want none (nothing critical)", len(oqs))
	}
}

// Same, when it touches a critical decision: it opens a «por aclarar»
// (flujos §2: «si tocan algo crítico, a por aclarar»).
func TestF2_AnIgnoredReaderContradictionThatTouchesACriticalDecisionOpensAnOpenQuestion(t *testing.T) {
	const clash = "Los filtros finos retienen más partículas."
	reader := readerOutput(t, projectFinding("fact", passageOne, "src-1"), projectFinding("contradiction", clash, "src-1"))
	db, _, _ := digestRound(t, func(db *store.DB) { seedSourceAndSection(t, db, "conclusion", true, "src-1") },
		[]string{projectPart("src-1")}, []string{reader}, plainDigest(t, "yes"))
	_, b := object(t, db, "oq-1")
	if b["kind"] != cycle.OpenQuestionReaderContradiction || b["new_passage"] != clash {
		t.Fatalf("oq-1: %v", b)
	}
	if got := currentBlocks(t, db, "oq-1"); len(got) != 1 || got[0] != "sec-1" {
		t.Fatalf("blocks: %v", got)
	}
}

// When the digest fails its shape twice, the card still lists every
// entered finding and the reader's contradiction.
func TestF2_WithAnInvalidDigestTheEnteredFindingsAreListed(t *testing.T) {
	const clash = "Los filtros finos retienen más partículas."
	bad := digestFixture(t, map[string]any{"answer": "a", "outline": []string{"x"},
		"findings": []map[string]int{{"finding": 9, "outline_line": 1}}, "expectation_contrast": "c",
		"reduced_doubt": "yes", "contradictions": []any{}})
	reader := readerOutput(t, projectFinding("fact", passageOne, "src-1"), projectFinding("contradiction", clash, "src-1"))
	_, runner, _ := digestRound(t, nil, []string{projectPart("src-1")}, []string{reader}, bad)
	v := roundView(t, runner, "rnd-1")
	if v.DigestStatus != cycle.DigestInvalidOutput || len(v.Unused) != 2 || len(v.ReaderContradictions) != 1 {
		t.Fatalf("view: %s unused %+v contradictions %+v", v.DigestStatus, v.Unused, v.ReaderContradictions)
	}
}

// B-65 (verifier A1): the digest reserves before each call, its retry
// too, so it never takes the project past its cap.
func TestB65_TheDigestReservesBeforeEachCallAgainstTheProjectCap(t *testing.T) {
	reader := readerOutput(t, projectFinding("fact", passageOne, "src-1"))
	t.Run("before the first call", func(t *testing.T) {
		step := discoverStep("too_much_to_read", "["+projectPart("src-1")+"]")
		fixtures := append(principalSteps(t, step), readerText(0, reader, 0.12), plainDigest(t, "yes"))
		db, runner, g := newRoundRunner(t, false, fixtures...)
		runner.Evidence = &handEvidence{}
		runner.ProjectCapUSD = 0.15 + provider.SignoffReserveUSD // the principal's reserve fits; the digest's, after the reader, does not
		runDiscover(t, runner, step)
		round, body := onlyRound(t, db)
		if n := len(g.callsOf(provider.TurnDigest)); n != 0 || body.DigestStatus != cycle.DigestProjectCap || round.State != "closed" {
			t.Fatalf("digest calls %d, status %q, %s", n, body.DigestStatus, round.State)
		}
	})
	t.Run("before the retry", func(t *testing.T) {
		bad := digestFixture(t, map[string]any{"answer": "a", "outline": []string{"x"},
			"findings": []map[string]int{{"finding": 9, "outline_line": 1}}, "expectation_contrast": "c",
			"reduced_doubt": "yes", "contradictions": []any{}})
		bad.Usage.CostUSD = 0.05
		step := discoverStep("too_much_to_read", "["+projectPart("src-1")+"]")
		fixtures := append(principalSteps(t, step), readerText(0, reader, 0.06), bad)
		db, runner, g := newRoundRunner(t, false, fixtures...)
		runner.Evidence = &handEvidence{}
		runner.ProjectCapUSD = 0.15 + provider.SignoffReserveUSD
		runDiscover(t, runner, step)
		_, body := onlyRound(t, db)
		spent, _ := db.ProjectSpend(context.Background(), "p1")
		if n := len(g.callsOf(provider.TurnDigest)); n != 1 || body.DigestStatus != cycle.DigestProjectCap || spent > 0.15+1e-9 {
			t.Fatalf("digest calls %d, status %q, spent %.3f", n, body.DigestStatus, spent)
		}
	})
}
