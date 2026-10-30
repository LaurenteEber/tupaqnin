package graph

import (
	"context"
	"strings"
	"unicode"

	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/text"

	"github.com/LaurenteEber/tupaqnin/internal/store"
)

// firstLineMax is M-4's cap for a section's first line [inicial].
const firstLineMax = 200

// FramePurpose is «Para qué»: the request, its audience and the premise,
// in literal (layer 1).
type FramePurpose struct {
	RequestText        string `json:"request_text"`
	Audience           string `json:"audience"`
	PremiseText        string `json:"premise_text"`
	PremiseExploratory bool   `json:"premise_exploratory"`
}

// FrameCase is a case of a criterion: one that counts or one that does
// not, in literal.
type FrameCase struct {
	Label  string `json:"label"`
	Text   string `json:"text"`
	Counts bool   `json:"counts"`
	State  string `json:"state"`
}

// FrameCriterion is «Cómo se juzga»: a criterion with its cases.
type FrameCriterion struct {
	Label string      `json:"label"`
	Text  string      `json:"text"`
	Cases []FrameCase `json:"cases"`
}

// FrameLayers is what the frame takes from layers 1 and 2 of the
// working state (memory.WorkingState, specification §9). The memory
// package imports this one, so the caller computes the working state and
// passes it here.
// [EL DISEÑO NO LO DICE]: the backlog's Frame(ctx, projectID) has no
// way to reach the working state without an import cycle; the layers and
// the reader are passed in, as Critical takes its q.
type FrameLayers struct {
	Purpose          FramePurpose     `json:"purpose"`
	Criteria         []FrameCriterion `json:"criteria"`
	UnconfirmedCases []string         `json:"unconfirmed_cases"`
	StopReportUnseen bool             `json:"stop_report_unseen"`
}

// FrameDecision is a current critical decision under a section: the
// item and what its card shows when expanded (§9).
type FrameDecision struct {
	Label         string `json:"label"`
	Statement     string `json:"statement"`
	InsteadOf     string `json:"instead_of"`
	WouldChooseIf string `json:"would_choose_if"`
	// Reason is your «porque»: the one of its answered card, or the one
	// you wrote when you chose the other option.
	Reason string `json:"reason,omitempty"`
	// SignoffExplanation is your explanation in the section's current
	// signature, on each critical decision that signature explains
	// (spec §9).
	SignoffExplanation string `json:"signoff_explanation,omitempty"`
	// Icon is the first criticality reason it meets, in the order
	// figure · conclusion or recommendation · several sections · marked
	// by you.
	Icon Reason `json:"icon"`
	// Sections are the labels of the sections that use it, in document
	// order.
	Sections []string `json:"sections"`
	// Card is its open card, if any: «Te toca» leads there.
	Card string `json:"card,omitempty"`
	// Block is the block of this section where a figure it covers
	// appears, for the link to that paragraph's history (D7); 0 when no
	// figure of it is in the text.
	// [EL DISEÑO NO LO DICE]: which paragraph a decision without a
	// figure opens; none, the link goes to the text only.
	Block int `json:"block"`
	// Seen: a decision_seen event exists for it.
	// [EL DISEÑO NO LO DICE]: the exact «sin ver» is slice 2; here, seen
	// once ever, through any view.
	Seen bool `json:"seen"`
}

// FrameSection is a section of the skeleton: its state, its first line
// (M-4) and its current critical decisions.
type FrameSection struct {
	Label     string          `json:"label"`
	Title     string          `json:"title"`
	Status    Status          `json:"status"`
	Pendings  int             `json:"pendings"`
	FirstLine string          `json:"first_line"`
	Decisions []FrameDecision `json:"decisions"`
}

// FrameCard is an open card in «Lo que te toca».
type FrameCard struct {
	Label     string `json:"label"`
	Decision  string `json:"decision"`
	Statement string `json:"statement"`
}

// FrameTodo is «Lo que te toca»: your pendings (layer 1).
// [EL DISEÑO NO LO DICE]: the sections ready to sign count as yours to
// do, as in the sketch («Firmar «Síntesis»»).
type FrameTodo struct {
	Cards            []FrameCard `json:"cards"`
	UnconfirmedCases int         `json:"unconfirmed_cases"`
	ReadyToSign      []string    `json:"ready_to_sign"`
	StopReportUnseen bool        `json:"stop_report_unseen"`
}

// Frame is the project's frame (M-1, specification §9), in its order:
// what it is for, how it is judged, its skeleton and what is yours to do.
type Frame struct {
	Purpose  FramePurpose     `json:"purpose"`
	Criteria []FrameCriterion `json:"criteria"`
	Sections []FrameSection   `json:"sections"`
	Todo     FrameTodo        `json:"todo"`
}

// ComputeFrame is frame(project): the frame from the working state's
// layers and the graph, without a model and without writing (§9): criticality from the
// one function of T-5, each section's state and pendings from
// SectionStatus, plus the «porque» of each answered card. It shows only
// what is current: superseded and revoked decisions are left out.
// [EL DISEÑO NO LO DICE]: the backlog names it Frame, which is the type;
// the function is ComputeFrame.
func ComputeFrame(ctx context.Context, db *store.DB, projectID string, layers FrameLayers) (Frame, error) {
	f := Frame{Purpose: layers.Purpose, Criteria: layers.Criteria, Sections: []FrameSection{},
		Todo: FrameTodo{Cards: []FrameCard{}, ReadyToSign: []string{},
			UnconfirmedCases: len(layers.UnconfirmedCases), StopReportUnseen: layers.StopReportUnseen}}
	if f.Criteria == nil {
		f.Criteria = []FrameCriterion{}
	}

	objects, err := db.AllObjects(ctx, projectID)
	if err != nil {
		return Frame{}, err
	}
	byID := make(map[string]store.Object, len(objects))
	var sections, decisions, cards []store.Object
	for _, obj := range objects {
		byID[obj.ID] = obj
		switch obj.Type {
		case store.ObjectSection:
			sections = append(sections, obj)
		case store.ObjectDecision:
			decisions = append(decisions, obj)
		case store.ObjectCard:
			cards = append(cards, obj)
		}
	}

	SortSections(sections) // the skeleton follows position (K-17)

	critical, err := CriticalAll(ctx, db, projectID)
	if err != nil {
		return Frame{}, err
	}
	reasons, err := cardReasons(ctx, db, projectID)
	if err != nil {
		return Frame{}, err
	}
	openCard := map[string]string{}
	for _, c := range cards {
		if c.State != "open" {
			continue
		}
		decID := bodyString(c, "decision_id")
		openCard[decID] = c.Label
		f.Todo.Cards = append(f.Todo.Cards, FrameCard{Label: c.Label,
			Decision: byID[decID].Label, Statement: bodyString(byID[decID], "statement")})
	}
	events, err := db.EventsSince(ctx, projectID, 0)
	if err != nil {
		return Frame{}, err
	}
	seen := map[string]bool{}
	for _, ev := range events {
		if ev.Type == store.EventDecisionSeen {
			seen[ev.ObjectID] = true
		}
	}

	// Each current critical decision, with the sections that use it:
	// those it is used_in and those where a figure it covers is.
	type item struct {
		id       string
		decision FrameDecision
		anchors  map[string][]string // section id → anchors of its figures
		in       map[string]bool
	}
	var items []*item
	for _, dec := range decisions {
		why, ok := critical[dec.ID]
		if !ok || dec.State == "superseded" || dec.State == "revoked" {
			continue
		}
		it := &item{id: dec.ID, anchors: map[string][]string{}, in: map[string]bool{}}
		out, err := db.EdgesFrom(ctx, projectID, dec.ID)
		if err != nil {
			return Frame{}, err
		}
		for _, e := range out {
			if e.Type == store.EdgeUsedIn && byID[e.ToID].Type == store.ObjectSection {
				it.in[e.ToID] = true
			}
		}
		in, err := db.EdgesTo(ctx, projectID, dec.ID)
		if err != nil {
			return Frame{}, err
		}
		for _, e := range in {
			fig := byID[e.FromID]
			if e.Type != store.EdgeCovers || fig.Type != store.ObjectFigure {
				continue
			}
			sec := bodyString(fig, "section_id")
			if byID[sec].Type == store.ObjectSection {
				it.in[sec] = true
				it.anchors[sec] = append(it.anchors[sec], bodyString(fig, "anchor"))
			}
		}
		reason := reasons[dec.ID]
		if reason == "" {
			reason = bodyString(dec, "reason") // the decision you chose on a card (B-16)
		}
		it.decision = FrameDecision{Label: dec.Label, Statement: bodyString(dec, "statement"),
			InsteadOf: bodyString(dec, "instead_of"), WouldChooseIf: bodyString(dec, "would_choose_if"),
			Reason: reason, Icon: icon(why), Card: openCard[dec.ID], Seen: seen[dec.ID], Sections: []string{}}
		for _, sec := range sections {
			if it.in[sec.ID] {
				it.decision.Sections = append(it.decision.Sections, sec.Label)
			}
		}
		items = append(items, it)
	}

	// The skeleton, in document order; a decision several sections use
	// appears under each one, with the same label.
	for _, sec := range sections {
		status, pendings, err := SectionStatus(ctx, db, projectID, sec.Label)
		if err != nil {
			return Frame{}, err
		}
		text := bodyString(sec, "text")
		fs := FrameSection{Label: sec.Label, Title: bodyString(sec, "title"), Status: status,
			Pendings: len(pendings), FirstLine: firstSentence(text), Decisions: []FrameDecision{}}
		blocks := Blocks(text)
		signed, hasSignature, err := CurrentSignoff(ctx, db, projectID, sec.ID)
		if err != nil {
			return Frame{}, err
		}
		explained := map[string]bool{}
		for _, ref := range signed.Critical {
			explained[ref.ID] = true
		}
		for _, it := range items {
			if !it.in[sec.ID] {
				continue
			}
			d := it.decision
			d.Block = blockOf(blocks, it.anchors[sec.ID])
			if hasSignature && explained[it.id] {
				d.SignoffExplanation = signed.Explanation
			}
			fs.Decisions = append(fs.Decisions, d)
		}
		f.Sections = append(f.Sections, fs)
		if status == StatusReady {
			f.Todo.ReadyToSign = append(f.Todo.ReadyToSign, sec.Label)
		}
	}
	return f, nil
}

// icon is the first reason it meets, in the order of §9.
func icon(reasons []Reason) Reason {
	for _, want := range []Reason{ReasonCoversFigure, ReasonConclusionSection, ReasonTwoOrMoreSections, ReasonHumanMarked} {
		for _, r := range reasons {
			if r == want {
				return want
			}
		}
	}
	return ""
}

// blockOf is the first block (1-based) where one of the anchors appears.
func blockOf(blocks []string, anchors []string) int {
	for i, b := range blocks {
		c := Compare(b)
		for _, a := range anchors {
			if len(c.Find(a)) > 0 {
				return i + 1
			}
		}
	}
	return 0
}

// firstSentence is M-4: the first sentence of the text, up to 200
// characters [inicial]. A sentence ends at «.», «?» or «!» followed by a
// space or the end of its block.
// [EL DISEÑO NO LO DICE]: a heading is not a sentence; the first block
// that is not a heading is read.
func firstSentence(markdown string) string {
	src := []byte(markdown)
	doc := blockParser.Parse(text.NewReader(src))
	block := ""
	for n := doc.FirstChild(); n != nil; n = n.NextSibling() {
		if _, heading := n.(*ast.Heading); heading {
			continue
		}
		block = strings.TrimSpace(span(n, src))
		break
	}
	runes := []rune(block)
	end := len(runes)
	for i, r := range runes {
		if (r == '.' || r == '?' || r == '!') && (i+1 == len(runes) || unicode.IsSpace(runes[i+1])) {
			end = i + 1
			break
		}
	}
	if end > firstLineMax {
		end = firstLineMax
	}
	return strings.TrimSpace(string(runes[:end]))
}
