package cycle

import (
	"strings"
	"unicode/utf8"

	"github.com/LaurenteEber/tupaqnin/internal/graph"
	"github.com/LaurenteEber/tupaqnin/internal/store"
)

// B-99: a change the agent should not make without you seeing it first goes to a
// proposal (B-73), by a rule of the app, without a model. Eber, 3-nov: on every step
// of the agent that changes existing text, not only on the judge's batch, because
// the real cases were an ordinary step and a batch's sentence ending (C.6 diagnosis).

// removesHalfOf is (a): more than half of the section's text goes.
func removesHalfOf(base, result string) bool {
	return utf8.RuneCountInString(result)*2 < utf8.RuneCountInString(base)
}

// sentenceClosers end a sentence; a closing quote or bracket may follow the period.
const sentenceClosers = ".!?:;…»”\"')]"

func closesSentence(s string) bool {
	s = strings.TrimSpace(s)
	if s == "" {
		return true
	}
	r, _ := utf8.DecodeLastRuneInString(s)
	return strings.ContainsRune(sentenceClosers, r)
}

// endsSentenceAt says position end of text is the end of a sentence: what comes
// before closes it and what follows is a space or nothing, or the paragraph ends
// there. «3.14» does not end at its period.
func endsSentenceAt(text string, end int) bool {
	rest := text[end:]
	if strings.TrimSpace(rest) == "" || strings.HasPrefix(rest, "\n") {
		return true
	}
	if !closesSentence(text[:end]) {
		return false
	}
	// An anchor may bring the space after the period.
	return rest[0] == ' ' || rest[0] == '\t' || text[end-1] == ' '
}

// startsSentenceAt says position start of text begins a sentence or a paragraph.
func startsSentenceAt(text string, start int) bool {
	before := strings.TrimRight(text[:start], " \t")
	return before == "" || strings.HasSuffix(before, "\n") || closesSentence(before)
}

// isProse: a heading, a list item, a quote or a table row does not close with a period.
func isProse(block string) bool {
	b := strings.TrimSpace(block)
	if b == "" {
		return false
	}
	switch b[0] {
	case '#', '-', '*', '>', '|', '+', '`':
		return false
	}
	digits := strings.TrimLeft(b, "0123456789")
	return len(digits) == len(b) || !strings.HasPrefix(digits, ".")
}

// breaksASentence is (c): the step leaves more unfinished paragraphs than it found.
// A paragraph is judged by its last line, so one that leads into a list or a table
// is not unfinished, and one that was already open is not this step's.
func breaksASentence(base, result string) bool {
	return unfinishedParagraphs(result) > unfinishedParagraphs(base)
}

func unfinishedParagraphs(text string) int {
	n := 0
	for _, b := range strings.Split(text, "\n\n") {
		b = strings.TrimSpace(b)
		last := b[strings.LastIndex(b, "\n")+1:]
		if isProse(last) && !closesSentence(last) {
			n++
		}
	}
	return n
}

// rewritesASentenceEnd is what the case-5 batch did: a «replace» of the end of a
// sentence, from its middle to its period, so the new words hang from the old ones.
func rewritesASentenceEnd(base string, edits []edit) bool {
	for _, e := range edits {
		if e.operation != "replace" || e.text == "" || e.anchor == "" {
			continue
		}
		if !startsSentenceAt(base, e.start) && endsSentenceAt(base, e.end) && closesSentence(base[e.start:e.end]) {
			return true
		}
	}
	return false
}

// paragraphGone says edits that delete, with no text in its place, cover the whole
// paragraph that holds anchor, and the anchor is no longer in the result: (b) is
// about deleting a paragraph, not a word, and not rewriting it (a recalculation).
func paragraphGone(base, result, anchor string, edits []edit) bool {
	if strings.Contains(result, anchor) {
		return false
	}
	at := strings.Index(base, anchor)
	if at < 0 {
		return false
	}
	start := strings.LastIndex(base[:at], "\n\n") + 1
	if start > 0 {
		start++
	}
	end := len(base)
	if next := strings.Index(base[at:], "\n\n"); next >= 0 {
		end = at + next
	}
	for i := start; i < end; i++ {
		if base[i] == ' ' || base[i] == '\n' || base[i] == '\t' {
			continue
		}
		covered := false
		for _, e := range edits {
			covered = covered || strings.TrimSpace(e.text) == "" && i >= e.start && i < e.end
		}
		if !covered {
			return false
		}
	}
	return true
}

// supportingAnchors are the passages of a section that hold a decision of yours or
// a confirmed one, or a linked citation: figures covered by such a decision, its
// own anchored use, and citations with their source.
func (p *project) supportingAnchors(sectionID string) []string {
	var out []string
	holds := func(id string) bool {
		obj, ok := p.byID[id]
		return ok && obj.Type == store.ObjectDecision && (obj.State == "human" || obj.State == "confirmed")
	}
	for id, obj := range p.byID {
		b := bodyOf(obj)
		switch obj.Type {
		case store.ObjectFigure, store.ObjectCitation:
			if bodyStr(b, "section_id") != sectionID || bodyStr(b, "anchor") == "" {
				continue
			}
			for _, e := range p.edgesOut[id] {
				if e.Type == store.EdgeCovers && holds(e.ToID) || e.Type == store.EdgeComesFrom {
					out = append(out, bodyStr(b, "anchor"))
					break
				}
			}
		case store.ObjectDecision:
			if !holds(id) {
				continue
			}
			for _, e := range p.edgesOut[id] {
				if e.Type == store.EdgeUsedIn && e.ToID == sectionID && e.Anchor != "" {
					out = append(out, e.Anchor)
				}
			}
		}
	}
	return out
}

// drasticWhy says why a step's text on existing text goes to a proposal (B-99), or "".
func (p *project) drasticWhy(sp *sectionPlan) string {
	if sp.base == "" {
		return ""
	}
	switch {
	case removesHalfOf(sp.base, sp.result):
		return graph.ProposalRemovesHalf
	case p.removesSupport(sp):
		return graph.ProposalRemovesSupport
	case breaksASentence(sp.base, sp.result):
		return graph.ProposalBreaksSentence
	case p.inJudgeBatch && rewritesASentenceEnd(sp.base, sp.edits):
		return graph.ProposalRewritesSentenceEnd
	}
	return ""
}

func (p *project) removesSupport(sp *sectionPlan) bool {
	for _, anchor := range p.supportingAnchors(sp.sectionID) {
		if paragraphGone(sp.base, sp.result, anchor, sp.edits) {
			return true
		}
	}
	return false
}
