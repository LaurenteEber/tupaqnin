package graph

import (
	"context"
	"regexp"
	"unicode/utf16"

	"golang.org/x/text/unicode/norm"

	"github.com/LaurenteEber/tupaqnin/internal/store"
)

// Kinds of a warning in the text (backlog-r2 2.9, specification §8,
// pantallas «Avisos en el texto»).
const (
	// WarningChanged is an open change of a signed section (it moved it
	// to changed_since_signoff, estados §1).
	WarningChanged = "changed"
	// WarningStale is an open change of a section «escrita con lo
	// anterior» (A-1, E-4).
	WarningStale = "stale"
	// WarningOpenQuestion is an open «por aclarar» that blocks the
	// section (2.4).
	WarningOpenQuestion = "open_question"
	// WarningAgentPending is one of the agent's pendings (estados §1).
	WarningAgentPending = "agent_pending"
)

// Warning is one thing in a section that asks for your attention
// (backlog-r2 2.9): its kind, the literal passage where it goes, if any,
// and the label it leads to. Go places it: Block is the paragraph, from
// 1, as the history numbers it (B-10), and Start and End are the
// passage's offsets inside that block in UTF-16 code units, the unit of
// the page's editor. The page only underlines and links it; it never
// sees an id (rule 7 of AGENTS: Target is a short label, for the link,
// never shown).
type Warning struct {
	Kind string `json:"kind"`
	// Pending is the agent pending's kind (glossary §9), on agent_pending.
	Pending string `json:"pending,omitempty"`
	// Cause tells the two causes of source_not_supporting apart.
	Cause string `json:"cause,omitempty"`
	// ChangeKind is what changed, on changed and stale: decision, figure
	// or source.
	ChangeKind string `json:"change_kind,omitempty"`
	// What is what the warning is about in words, literal: what changed
	// (a decision's statement, a source's file name, a figure's anchor)
	// or the «por aclarar»'s question. Never a label.
	What string `json:"what,omitempty"`
	// Passage is literal in the section's current text, or empty.
	Passage string `json:"passage,omitempty"`
	Block   int    `json:"block,omitempty"`
	Start   int    `json:"start"`
	End     int    `json:"end"`
	// Target is the label the warning leads to: the change (chg-n), the
	// «por aclarar» (oq-n) or, for an agent pending, the section itself
	// (its history at Block).
	Target string `json:"target"`
}

// SectionWarnings returns a section's warnings in the text (backlog-r2
// 2.9): its open changes first, then its open «por aclarar», then the
// agent's pendings, each in the order SectionStatus lists them. They
// come from the same functions as the state and the pendings, so a
// warning closes when its change, question or pending does.
// [EL DISEÑO NO LO DICE]: an open card is a pending but not one of the
// four kinds; it stays in «Lo que te toca» and in the signoff's list,
// without a warning in the text.
// [EL DISEÑO NO LO DICE]: «sin comprobar» (B-50) is listed as an agent
// pending: estados §1 has no other place for it.
func SectionWarnings(ctx context.Context, q store.Querier, projectID, label string) ([]Warning, error) {
	section, err := q.GetObject(ctx, projectID, label)
	if err != nil {
		return nil, err
	}
	status, pendings, err := SectionStatus(ctx, q, projectID, label)
	if err != nil {
		return nil, err
	}
	// NFC, so a block's compared offsets are offsets of the text (T-4).
	text := norm.NFC.String(bodyString(section, "text"))
	out := []Warning{}

	changeKind := WarningStale
	if status == StatusChangedSinceSignoff {
		changeKind = WarningChanged
	}
	for _, c := range OpenChanges(section) {
		w := Warning{Kind: changeKind, ChangeKind: c.Kind, Target: c.Label}
		if obj, err := q.GetObject(ctx, projectID, c.About); err == nil {
			w.What = readable(obj, nil)
		}
		out = append(out, w)
	}

	for _, p := range pendings {
		switch p.Kind {
		case PendingOpenCard:
			continue
		case PendingOpenQuestion:
			w := Warning{Kind: WarningOpenQuestion, Target: p.About}
			if oq, err := q.GetObject(ctx, projectID, p.About); err == nil {
				w.What = bodyString(oq, "question")
				w.place(text, bodyString(oq, "new_passage"))
			}
			out = append(out, w)
		default:
			w := Warning{Kind: WarningAgentPending, Pending: p.Kind, Cause: p.Cause, Target: section.Label}
			passage, err := pendingPassage(ctx, q, projectID, text, p)
			if err != nil {
				return nil, err
			}
			w.place(text, passage)
			out = append(out, w)
		}
	}
	return out, nil
}

// pendingPassage is the literal passage of the text an agent pending is
// about, or "" when it has none: the number of «cifra sin decisión», the
// code as written of «códigos internos», the citation's claim of «fuente
// que no sostiene», and the passage the others keep (relato del proceso,
// escrito sin decisión from the checker, afirmación sin fuente, falta en
// el texto).
func pendingPassage(ctx context.Context, q store.Querier, projectID, text string, p Pending) (string, error) {
	switch p.Kind {
	case PendingInternalCodes:
		return codeAsWritten(text, p.About), nil
	case PendingSourceNotSupporting:
		cit, err := q.GetObject(ctx, projectID, p.About)
		if err != nil {
			return "", nil
		}
		return bodyString(cit, "anchor"), nil
	case PendingFigureWithoutDecision, PendingProcessNarrative, PendingTextWithoutDecision,
		PendingUnsourcedClaim, PendingMissingInText:
		return p.About, nil
	}
	return "", nil
}

// codeLike is a token that NormalizeLabel can turn into a short label:
// letters (look-alikes included), any dash, digits, with format
// characters allowed between them.
var codeLike = regexp.MustCompile(`[\p{L}\p{Cf}]+[\p{Pd}\x{2212}\p{Cf}]+[\p{N}\p{Cf}]+`)

// codeAsWritten is the first spelling in the text of an internal code,
// which InternalCodes reports normalized (rule 4: «DEC‑1» is dec-1).
func codeAsWritten(text, code string) string {
	for _, m := range codeLike.FindAllString(text, -1) {
		if NormalizeLabel(m) == code {
			return m
		}
	}
	return ""
}

// place puts the warning at the first block where passage is literal
// (T-4 comparison), with the passage as the text spells it. Start and
// End count from the start of the paragraph the editor shows for that
// block, one per block (P-78, Paragraphs): from its first line, so a
// heading keeps its «## » and a list item its marker. A passage that
// is not in one block leaves the warning without a place and without a
// passage: it goes only in the band.
// [EL DISEÑO NO LO DICE]: the offsets' unit; UTF-16 code units, the
// editor's.
func (w *Warning) place(text, passage string) {
	if passage == "" {
		return
	}
	starts := paragraphStarts(text)
	for i, r := range blockRanges(text) {
		c := Compare(text[r[0]:r[1]])
		found := c.Find(passage)
		if len(found) == 0 {
			continue
		}
		from, to := r[0]+found[0][0], r[0]+found[0][1]
		para := starts[i]
		w.Passage = text[from:to]
		w.Block = i + 1
		w.Start = utf16Len(text[para:from])
		w.End = utf16Len(text[para:to])
		return
	}
}

func utf16Len(s string) int { return len(utf16.Encode([]rune(s))) }
