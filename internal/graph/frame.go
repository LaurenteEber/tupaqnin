package graph

import (
	"context"
	"sort"
	"strings"
	"time"
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
	// Now and ReturnAfter decide the return line (M-2, B-48 a): it is
	// there when ReturnAfter or more passed between your last event and
	// Now. A zero Now computes no line. Injected, so tests never sleep.
	Now         time.Time     `json:"-"`
	ReturnAfter time.Duration `json:"-"`
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
	// Seen: a decision_seen event exists for it, or it is yours (P-77,
	// B-39: «tus decisiones las viste al tomarlas»): of origin human, or
	// born from your act (a card's other option, an open question's).
	Seen bool `json:"seen"`
	// Sources are the searched sources with a verified citation in this
	// section (backlog-r3 3.5, pantallas: «en la decisión desplegada, las
	// fuentes buscadas con su enlace»).
	Sources []SearchedSource `json:"sources"`
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
	// Unverified is how many paragraphs have no verified origin (R-1,
	// R-2): «¿en qué te basaste?» from the graph.
	// [EL DISEÑO NO LO DICE]: what the frame shows of the provenance; a
	// count per section, the detail is in the paragraph's history.
	Unverified int `json:"unverified"`
}

// FrameCard is an open card in «Lo que te toca».
type FrameCard struct {
	Label     string `json:"label"`
	Decision  string `json:"decision"`
	Statement string `json:"statement"`
}

// FrameOpenQuestion is an open «por aclarar» in «Lo que te toca»: one
// line that leads to its card (pantallas, rebanada 2).
type FrameOpenQuestion struct {
	Label    string `json:"label"`
	Question string `json:"question"`
}

// FrameFlaws is «Fallas por revisar» in «Lo que te toca» (pantallas,
// backlog-r3 3.6): a judge round whose batch has not started, how many
// flaws it has not discarded, and its section.
type FrameFlaws struct {
	Round   string `json:"round"`
	Section string `json:"section"`
	Flaws   int    `json:"flaws"`
}

// FrameTodo is «Lo que te toca»: your pendings (layer 1).
// [EL DISEÑO NO LO DICE]: the sections ready to sign count as yours to
// do, as in the sketch («Firmar «Síntesis»»).
type FrameTodo struct {
	Cards            []FrameCard         `json:"cards"`
	OpenQuestions    []FrameOpenQuestion `json:"open_questions"`
	FlawsToReview    []FrameFlaws        `json:"flaws_to_review"`
	UnconfirmedCases int                 `json:"unconfirmed_cases"`
	ReadyToSign      []string            `json:"ready_to_sign"`
	StopReportUnseen bool                `json:"stop_report_unseen"`
	// Proposals are the open proposals (backlog-r4 4.1): one line «Propuestas
	// (n)» in the page; they are not pendings and block nothing (B-76).
	Proposals []FrameProposal `json:"proposals"`
	// Count is how many lines «Lo que te toca» shows: each card, each
	// «por aclarar», each section to sign, the unseen report, and the
	// understanding's unconfirmed cases as one line (backlog-r2 2.7).
	Count int `json:"count"`
}

// FrameProposal is an open proposal in «Lo que te toca»: its section, or
// the title of the section it would create (B-77).
type FrameProposal struct {
	Label   string `json:"label"`
	Section string `json:"section,omitempty"`
	Title   string `json:"title,omitempty"`
}

// FrameUnseen is one item of the filter «solo lo que no viste» (§9):
// a critical decision with no decision_seen, under the first section
// of the skeleton that shows it (its Block is that section's).
type FrameUnseen struct {
	Section  string        `json:"section"`
	Decision FrameDecision `json:"decision"`
}

// Frame is the project's frame (M-1, specification §9), in its order:
// what it is for, how it is judged, its skeleton and what is yours to do.
type Frame struct {
	Purpose  FramePurpose     `json:"purpose"`
	Criteria []FrameCriterion `json:"criteria"`
	Sections []FrameSection   `json:"sections"`
	Todo     FrameTodo        `json:"todo"`
	// ReturnLine is M-2's line above the frame; nil on shorter returns.
	ReturnLine *ReturnLine `json:"return_line,omitempty"`
	// Unseen is the filter «solo lo que no viste», already ordered.
	Unseen []FrameUnseen `json:"unseen"`
	// Candidates are what the agent named without a citation (R-2, B-69):
	// «candidata, sin verificar», with «Buscarla».
	Candidates []Candidate `json:"candidates"`
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
		Todo: FrameTodo{Cards: []FrameCard{}, OpenQuestions: []FrameOpenQuestion{}, FlawsToReview: []FrameFlaws{}, ReadyToSign: []string{},
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
		case store.ObjectOpenQuestion:
			if obj.State == "open" {
				f.Todo.OpenQuestions = append(f.Todo.OpenQuestions,
					FrameOpenQuestion{Label: obj.Label, Question: bodyString(obj, "question")})
			}
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
	seen := seenDecisions(events, decisions)
	f.Candidates = Candidates(events)

	// Each current critical decision, with the sections that use it:
	// those it is used_in and those where a figure it covers is.
	type item struct {
		id       string
		decision FrameDecision
		anchors  map[string][]string // section id → anchors of its figures
		in       map[string]bool
		usedIn   int // sections it is used_in (the filter's order, §9)
		first    int // index of the first skeleton section showing it
		shown    *FrameDecision
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
				if !it.in[e.ToID] {
					it.usedIn++
				}
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
		provenance, err := Provenances(ctx, db, projectID, sec.ID)
		if err != nil {
			return Frame{}, err
		}
		for _, p := range provenance {
			if !p.Verified {
				fs.Unverified++
			}
		}
		// [EL DISEÑO NO LO DICE]: which searched sources a decision shows
		// (pantallas: «en la decisión desplegada, las fuentes buscadas con
		// su enlace»); no edge joins them, so those of its section.
		searched := SearchedSources(provenance)
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
			d.Sources = searched
			if hasSignature && explained[it.id] {
				d.SignoffExplanation = signed.Explanation
			}
			fs.Decisions = append(fs.Decisions, d)
			if it.shown == nil {
				it.first, it.shown = len(f.Sections), &d
			}
		}
		f.Sections = append(f.Sections, fs)
		if status == StatusReady {
			f.Todo.ReadyToSign = append(f.Todo.ReadyToSign, sec.Label)
		}
	}

	// The filter «solo lo que no viste» (§9): the critical decisions
	// with no decision_seen, more used_in edges first and, at equal
	// number, in document order (the first section showing each, then
	// the order of the skeleton under it).
	var unseen []*item
	for _, it := range items {
		if !it.decision.Seen && it.shown != nil {
			unseen = append(unseen, it)
		}
	}
	sort.SliceStable(unseen, func(i, j int) bool {
		if unseen[i].usedIn != unseen[j].usedIn {
			return unseen[i].usedIn > unseen[j].usedIn
		}
		return unseen[i].first < unseen[j].first
	})
	f.Unseen = []FrameUnseen{}
	for _, it := range unseen {
		f.Unseen = append(f.Unseen, FrameUnseen{Section: f.Sections[it.first].Label, Decision: *it.shown})
	}

	judgeRounds, err := OpenJudgeRounds(ctx, db, projectID)
	if err != nil {
		return Frame{}, err
	}
	for _, jr := range judgeRounds {
		if jr.BatchStarted {
			continue
		}
		section := ""
		if sec, err := db.GetObject(ctx, projectID, jr.SectionID); err == nil {
			section = sec.Label
		}
		f.Todo.FlawsToReview = append(f.Todo.FlawsToReview, FrameFlaws{Round: jr.Label, Section: section, Flaws: len(jr.Flaws)})
	}
	open, err := OpenProposals(ctx, db, projectID, "")
	if err != nil {
		return Frame{}, err
	}
	f.Todo.Proposals = []FrameProposal{}
	for _, p := range open {
		line := FrameProposal{Label: p.Label, Section: p.Section}
		if p.NewSection != nil {
			line.Title = p.NewSection.Title
		}
		f.Todo.Proposals = append(f.Todo.Proposals, line)
	}
	f.Todo.Count = len(f.Todo.Cards) + len(f.Todo.OpenQuestions) + len(f.Todo.FlawsToReview) + len(f.Todo.ReadyToSign) + len(f.Todo.Proposals)
	if f.Todo.UnconfirmedCases > 0 {
		f.Todo.Count++
	}
	if f.Todo.StopReportUnseen {
		f.Todo.Count++
	}
	if !layers.Now.IsZero() {
		after := layers.ReturnAfter
		if after <= 0 {
			after = ReturnAfter()
		}
		line, err := returnLine(ctx, db, projectID, events, byID, sections, layers.Now, after)
		if err != nil {
			return Frame{}, err
		}
		if line != nil {
			line.Todo = f.Todo.Count
		}
		f.ReturnLine = line
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
		r := span(n)
		block = strings.TrimSpace(string(src[r[0]:r[1]]))
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

// seenDecisions are the decisions, by internal id, that do not carry the
// «sin ver» dot (B3-9, P-77): those with a decision_seen event, and
// yours, which you saw when you made them (B-39): of origin human, or
// whose first event is an act of yours (a card's other option, an open
// question's option that replaced one).
func seenDecisions(events []store.Event, decisions []store.Object) map[string]bool {
	seen := map[string]bool{}
	created := map[string]bool{}
	for _, ev := range events {
		if ev.ObjectID == "" {
			continue
		}
		if ev.Type == store.EventDecisionSeen {
			seen[ev.ObjectID] = true
		}
		if !created[ev.ObjectID] {
			created[ev.ObjectID] = true // an object's first event creates it
			// Keyed by id: only decisions are read from it.
			seen[ev.ObjectID] = seen[ev.ObjectID] || ev.Actor == "human"
		}
	}
	for _, dec := range decisions {
		if bodyString(dec, "origin") == "human" {
			seen[dec.ID] = true
		}
	}
	return seen
}
