package checks

import (
	"context"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/LaurenteEber/tupaqnin/internal/graph"
	"github.com/LaurenteEber/tupaqnin/internal/store"
)

// Rule 6 (flujos §1, fichas §3, G-1, B-80): the app warns of an edit of yours
// that looks accidental, by rules, without a model and without blocking. If you
// do not answer, your edit rules and the alert stays unanswered.

// The thresholds of B-80, all [inicial].
const (
	// signedRemoved: «20 caracteres o más quitados» of a signed section, and
	// what an edit of another section must remove (rules 2 and 3).
	removedAtLeast = 20
	// otherSectionWithin: «tu edición anterior, de hace 10 min o menos».
	otherSectionWithin = 10 * time.Minute
	// pastedAtLeast: «un fragmento nuevo de 80 caracteres o más».
	pastedAtLeast = 80
	// maxDiffCells bounds the comparison: past it, what the common start and
	// end leave counts as replaced.
	maxDiffCells = 4_000_000
)

// SuspectEdit is your save as rule 6 reads it, in its transaction: the section
// after it, the text it was over (what your tab read), whether the section was
// signed before it, and now.
type SuspectEdit struct {
	Section   store.Object
	BaseText  string
	WasSigned bool
	Now       time.Time
}

// AlertSuspiciousEdit appends the suspicious_edit_alert of your save, if any.
func AlertSuspiciousEdit(ctx context.Context, tx *store.Tx, edit SuspectEdit) error {
	reasons, err := SuspiciousReasons(ctx, tx, tx.ProjectID(), edit)
	if err != nil || len(reasons) == 0 {
		return err
	}
	_, err = tx.AppendEvent(store.EventInput{Type: store.EventSuspiciousEditAlert, Actor: "app", ObjectID: edit.Section.ID,
		Data: graph.EditAlertData{Section: edit.Section.Label, Version: edit.Section.CurrentVersion, Reasons: reasons}})
	return err
}

// SuspiciousReasons runs B-80's four rules in order.
func SuspiciousReasons(ctx context.Context, q store.Querier, projectID string, edit SuspectEdit) ([]graph.SuspiciousReason, error) {
	base, next := graph.Compare(edit.BaseText), graph.Compare(sectionText(edit.Section))
	d := diffTexts(base.Text, next.Text)
	deleted := deletedBlocks(edit.BaseText, sectionText(edit.Section))
	var out []graph.SuspiciousReason

	// Rule 1: a whole paragraph (a block, B-10), or more than half of the text.
	if len(deleted) > 0 {
		out = append(out, graph.SuspiciousReason{Rule: 1, Kind: graph.SuspiciousParagraphDeleted})
	}
	if d.removed*2 > utf8.RuneCountInString(base.Text) {
		out = append(out, graph.SuspiciousReason{Rule: 1, Kind: graph.SuspiciousOverHalf})
	}

	// Rule 2: what holds a decision (an anchor, a block), or something signed.
	anchors, err := removedAnchors(ctx, q, projectID, edit.Section.ID, base, next)
	if err != nil {
		return nil, err
	}
	out = append(out, anchors...)
	seen := map[string]bool{}
	for _, r := range anchors {
		seen[r.Decision] = true
	}
	for _, block := range deleted {
		decisions, err := graph.BlockDecisions(ctx, q, projectID, edit.Section.ID, block)
		if err != nil {
			return nil, err
		}
		for _, dec := range decisions {
			if !seen[dec.Label] {
				seen[dec.Label] = true
				out = append(out, graph.SuspiciousReason{Rule: 2, Kind: graph.SuspiciousDecisionText, Decision: dec.Label, Statement: dec.Statement})
			}
		}
	}
	if edit.WasSigned && d.removed >= removedAtLeast {
		out = append(out, graph.SuspiciousReason{Rule: 2, Kind: graph.SuspiciousSignedText})
	}

	// Rule 3: another section, 10 min or less after your previous edit, and it
	// removes text.
	if d.removed >= removedAtLeast {
		title, other, err := previousOtherSection(ctx, q, projectID, edit)
		if err != nil {
			return nil, err
		}
		if other {
			out = append(out, graph.SuspiciousReason{Rule: 3, Kind: graph.SuspiciousOtherSection, About: title})
		}
	}

	// Rule 4: a new fragment already literal in another section or a source.
	where, pasted, err := pastedFrom(ctx, q, projectID, edit.Section.ID, d.added)
	if err != nil {
		return nil, err
	}
	if pasted {
		out = append(out, graph.SuspiciousReason{Rule: 4, Kind: graph.SuspiciousPasted, About: where})
	}
	return out, nil
}

// removedAnchors: the anchor of a linked figure or citation of the section was
// in the text and is not now.
func removedAnchors(ctx context.Context, q store.Querier, projectID, sectionID string, base, next graph.Compared) ([]graph.SuspiciousReason, error) {
	var out []graph.SuspiciousReason
	for _, kind := range []struct {
		t    store.ObjectType
		edge store.EdgeType
		name string
	}{{store.ObjectFigure, store.EdgeCovers, graph.SuspiciousFigureAnchor}, {store.ObjectCitation, store.EdgeComesFrom, graph.SuspiciousCitationAnchor}} {
		objects, err := q.ListObjects(ctx, projectID, kind.t)
		if err != nil {
			return nil, err
		}
		for _, obj := range objects {
			anchor := bodyString(obj, "anchor")
			if bodyString(obj, "section_id") != sectionID || anchor == "" || len(base.Find(anchor)) == 0 || len(next.Find(anchor)) > 0 {
				continue
			}
			// The same figure, written better, was not removed (B-104).
			if kind.t == store.ObjectFigure && graph.FigureStillIn(next.Orig, anchor) {
				continue
			}
			edges, err := q.EdgesFrom(ctx, projectID, obj.ID)
			if err != nil {
				return nil, err
			}
			for _, e := range edges {
				if e.Type != kind.edge {
					continue
				}
				r := graph.SuspiciousReason{Rule: 2, Kind: kind.name, About: anchor}
				if kind.t == store.ObjectFigure {
					dec, err := q.GetObject(ctx, projectID, e.ToID)
					if err != nil {
						return nil, err
					}
					r.Decision, r.Statement = dec.Label, bodyString(dec, "statement")
				}
				out = append(out, r)
				break
			}
		}
	}
	return out, nil
}

// previousOtherSection says your previous edit was in another section, 10 min
// or less ago, with its title.
func previousOtherSection(ctx context.Context, q store.Querier, projectID string, edit SuspectEdit) (string, bool, error) {
	events, err := q.EventsSince(ctx, projectID, 0)
	if err != nil {
		return "", false, err
	}
	var prev *store.Event
	for i := range events {
		ev := &events[i]
		if ev.Type != store.EventEdit || ev.Actor != "human" || ev.ObjectID == edit.Section.ID && ev.NewVersion == edit.Section.CurrentVersion {
			continue
		}
		prev = ev
	}
	if prev == nil || prev.ObjectID == edit.Section.ID {
		return "", false, nil
	}
	at, err := time.Parse("2006-01-02T15:04:05.000Z", prev.CreatedAt)
	if err != nil || edit.Now.Sub(at) > otherSectionWithin {
		return "", false, nil
	}
	sec, err := q.GetObject(ctx, projectID, prev.ObjectID)
	if err != nil {
		return "", false, err
	}
	return bodyString(sec, "title"), true, nil
}

// pastedFrom names the other section or source that already holds literal a
// fragment of pastedAtLeast characters of what this edit added. Only added text
// counts: a window that is mostly old is no new fragment. They are looked up in
// each other text in linear time.
func pastedFrom(ctx context.Context, q store.Querier, projectID, sectionID string, added []string) (string, bool, error) {
	fresh := map[string]bool{}
	for _, text := range added {
		forWindows(text, func(w string) bool { fresh[w] = true; return true })
	}
	if len(fresh) == 0 {
		return "", false, nil
	}
	holds := func(text string) bool {
		found := false
		forWindows(graph.CompareString(text), func(w string) bool {
			found = fresh[w]
			return !found
		})
		return found
	}
	sections, err := q.ListObjects(ctx, projectID, store.ObjectSection)
	if err != nil {
		return "", false, err
	}
	for _, sec := range sections {
		if sec.ID != sectionID && holds(sectionText(sec)) {
			return bodyString(sec, "title"), true, nil
		}
	}
	sources, err := q.ListObjects(ctx, projectID, store.ObjectSource)
	if err != nil {
		return "", false, err
	}
	for _, src := range sources {
		if holds(bodyString(src, "text")) {
			return graph.SourceTitle(src), true, nil
		}
	}
	return "", false, nil
}

// forWindows calls f with each window of pastedAtLeast characters of s that
// starts and ends on a sign, until it returns false: a space at its edge makes
// no fragment new.
func forWindows(s string, f func(string) bool) {
	if utf8.RuneCountInString(s) < pastedAtLeast {
		return
	}
	starts := make([]int, 0, len(s))
	for i := range s {
		starts = append(starts, i)
	}
	starts = append(starts, len(s))
	for i := 0; i+pastedAtLeast < len(starts); i++ {
		w := s[starts[i]:starts[i+pastedAtLeast]]
		if w[0] == ' ' || w[len(w)-1] == ' ' {
			continue
		}
		if !f(w) {
			return
		}
	}
}

// token is a word, or any other sign, of a compared text: a comma is its own
// token, so a fixed comma removes one character.
type token struct {
	text       string
	start, end int
	word       bool
}

func tokenize(s string) []token {
	var out []token
	isWord := func(r rune) bool { return unicode.IsLetter(r) || unicode.IsNumber(r) || unicode.IsMark(r) }
	for i := 0; i < len(s); {
		r, size := utf8.DecodeRuneInString(s[i:])
		switch {
		case unicode.IsSpace(r):
			i += size
		case isWord(r):
			j := i + size
			for j < len(s) {
				r2, s2 := utf8.DecodeRuneInString(s[j:])
				if !isWord(r2) {
					break
				}
				j += s2
			}
			out = append(out, token{text: s[i:j], start: i, end: j, word: true})
			i = j
		default:
			out = append(out, token{text: s[i : i+size], start: i, end: i + size})
			i += size
		}
	}
	return out
}

// textDiff is what an edit removed, in characters, and what it added.
type textDiff struct {
	removed int
	// added are the stretches of the new text this edit brought.
	added []string
}

// diffTexts compares two compared texts by word, by longest common subsequence
// after their common start and end. A replaced stretch removes what its old
// form does not share with the new one at either end.
func diffTexts(base, next string) textDiff {
	a, b := tokenize(base), tokenize(next)
	var d textDiff
	keptA, keptB := commonTokens(a, b)
	for i, j := 0, 0; i < len(a) || j < len(b); {
		if i < len(a) && j < len(b) && keptA[i] && keptB[j] {
			i++
			j++
			continue
		}
		from, to := i, j
		for i < len(a) && !keptA[i] {
			i++
		}
		for j < len(b) && !keptB[j] {
			j++
		}
		added := ""
		if j > to {
			added = next[b[to].start:b[j-1].end]
			d.added = append(d.added, added)
		}
		if i > from {
			d.removed += removedRunes(base[a[from].start:a[i-1].end], added)
			continue
		}
		// Pure insertion: the same text may read one word earlier or later, when
		// it starts or ends with the word beside it.
		for l, r := to, j; l > 0 && b[l-1].text == b[r-1].text; {
			l, r = l-1, r-1
			d.added = append(d.added, next[b[l].start:b[r-1].end])
		}
		for l, r := to, j; r < len(b) && b[l].text == b[r].text; {
			l, r = l+1, r+1
			d.added = append(d.added, next[b[l].start:b[r-1].end])
		}
	}
	return d
}

// commonTokens marks the tokens of a and b in their longest common subsequence.
func commonTokens(a, b []token) (keptA, keptB []bool) {
	keptA, keptB = make([]bool, len(a)), make([]bool, len(b))
	pre := 0
	for pre < len(a) && pre < len(b) && a[pre].text == b[pre].text {
		keptA[pre], keptB[pre] = true, true
		pre++
	}
	post := 0
	for post < len(a)-pre && post < len(b)-pre && a[len(a)-1-post].text == b[len(b)-1-post].text {
		keptA[len(a)-1-post], keptB[len(b)-1-post] = true, true
		post++
	}
	n, m := len(a)-pre-post, len(b)-pre-post
	if n == 0 || m == 0 || n*m > maxDiffCells {
		return keptA, keptB
	}
	// table[i*(m+1)+j] is the LCS of a[pre+i:] and b[pre+j:], walked from the
	// start so a repeated word matches its first place.
	table := make([]int32, (n+1)*(m+1))
	for i := n - 1; i >= 0; i-- {
		for j := m - 1; j >= 0; j-- {
			switch {
			case a[pre+i].text == b[pre+j].text:
				table[i*(m+1)+j] = table[(i+1)*(m+1)+j+1] + 1
			case table[(i+1)*(m+1)+j] >= table[i*(m+1)+j+1]:
				table[i*(m+1)+j] = table[(i+1)*(m+1)+j]
			default:
				table[i*(m+1)+j] = table[i*(m+1)+j+1]
			}
		}
	}
	for i, j := 0, 0; i < n && j < m; {
		switch {
		case a[pre+i].text == b[pre+j].text:
			keptA[pre+i], keptB[pre+j] = true, true
			i++
			j++
		case table[(i+1)*(m+1)+j] >= table[i*(m+1)+j+1]:
			i++
		default:
			j++
		}
	}
	return keptA, keptB
}

// removedRunes counts what old loses against what replaced it, without what
// they share at the start and the end.
func removedRunes(old, added string) int {
	o, a := []rune(old), []rune(added)
	start := 0
	for start < len(o) && start < len(a) && o[start] == a[start] {
		start++
	}
	end := 0
	for end < len(o)-start && end < len(a)-start && o[len(o)-1-end] == a[len(a)-1-end] {
		end++
	}
	return len(o) - start - end
}

// deletedBlocks are the blocks of the old text (B-10) that are gone from the new
// one. Blocks are aligned first, in order, by the words they share, so a block
// that goes is not taken for the next one because they start alike. A block left
// without a partner is deleted unless half its words or more are in what the
// others left of the new text: it was merged into another.
func deletedBlocks(baseText, nextText string) []string {
	var old, now []blockWords
	for _, block := range graph.Blocks(baseText) {
		if words := wordsOf(block); len(words) > 0 {
			old = append(old, blockWords{raw: block, words: words})
		}
	}
	for _, block := range graph.Blocks(nextText) {
		if words := wordsOf(block); len(words) > 0 {
			now = append(now, blockWords{raw: block, words: words})
		}
	}
	partner, leftover := alignBlocks(old, now)
	var out []string
	for i, block := range old {
		if partner[i] {
			continue
		}
		kept, _ := commonTokens(block.words, leftover)
		count := 0
		for _, k := range kept {
			if k {
				count++
			}
		}
		if count*2 < len(block.words) {
			out = append(out, block.raw)
		}
	}
	return out
}

type blockWords struct {
	raw   string
	words []token
}

func wordsOf(block string) []token {
	var out []token
	for _, tok := range tokenize(graph.CompareString(block)) {
		if tok.word {
			out = append(out, tok)
		}
	}
	return out
}

// alignBlocks pairs old blocks with new ones, in order, for the most shared
// words; it says which old blocks have a partner and returns the new text's
// words no partner took. Past maxDiffCells only identical blocks pair.
func alignBlocks(old, now []blockWords) (partner []bool, leftover []token) {
	wordsA, wordsB := 0, 0
	for _, b := range old {
		wordsA += len(b.words)
	}
	for _, b := range now {
		wordsB += len(b.words)
	}
	shared := func(a, b []token) int {
		if len(a) == len(b) {
			same := true
			for i := range a {
				same = same && a[i].text == b[i].text
			}
			if same {
				return len(a)
			}
		}
		if wordsA*wordsB > maxDiffCells {
			return 0
		}
		kept, _ := commonTokens(a, b)
		count := 0
		for _, k := range kept {
			if k {
				count++
			}
		}
		return count
	}
	n, m := len(old), len(now)
	scores := make([][]int, n)
	best := make([][]int, n+1)
	best[n] = make([]int, m+1)
	for i := range scores {
		scores[i] = make([]int, m)
		best[i] = make([]int, m+1)
		for j := range scores[i] {
			scores[i][j] = shared(old[i].words, now[j].words)
		}
	}
	for i := n - 1; i >= 0; i-- {
		for j := m - 1; j >= 0; j-- {
			best[i][j] = max(best[i+1][j], best[i][j+1])
			if s := scores[i][j]; s > 0 {
				best[i][j] = max(best[i][j], s+best[i+1][j+1])
			}
		}
	}
	partner = make([]bool, n)
	taken := make([][]bool, m)
	for i, j := 0, 0; i < n && j < m; {
		switch {
		case scores[i][j] > 0 && best[i][j] == scores[i][j]+best[i+1][j+1]:
			partner[i] = true
			if wordsA*wordsB <= maxDiffCells {
				_, taken[j] = commonTokens(old[i].words, now[j].words)
			} else {
				taken[j] = make([]bool, len(now[j].words))
				for k := range taken[j] {
					taken[j][k] = true
				}
			}
			i++
			j++
		case best[i][j] == best[i+1][j]:
			i++
		default:
			j++
		}
	}
	for j, block := range now {
		for k, tok := range block.words {
			if taken[j] == nil || !taken[j][k] {
				leftover = append(leftover, tok)
			}
		}
	}
	return partner, leftover
}
