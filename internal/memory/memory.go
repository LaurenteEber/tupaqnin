package memory

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/LaurenteEber/tupaqnin/internal/graph"
	"github.com/LaurenteEber/tupaqnin/internal/store"
)

// Memory computes the principal's working state from the project
// graph and the history (B2-1). It has no store of its own: every
// call is a fresh query (specification §5).
type Memory struct {
	db *store.DB
}

// New returns a Memory over db.
func New(db *store.DB) *Memory { return &Memory{db: db} }

// State is the working state in its three layers (block 2 §2,
// glossary §7).
type State struct {
	Essentials Essentials `json:"essentials"`
	Index      []Line     `json:"index"`
	InFlight   InFlight   `json:"in_flight"`
}

// Essentials is layer 1: what the human said, verbatim, the section
// roles, your decisions and the critical ones in compact form, and
// what is yours to do.
type Essentials struct {
	RequestText          string        `json:"request_text"`
	Audience             string        `json:"audience"`
	Criteria             []Criterion   `json:"criteria"`
	PremiseText          string        `json:"premise_text"`
	PremiseExploratory   bool          `json:"premise_exploratory"`
	SectionRoles         []SectionRole `json:"section_roles"`
	Decisions            []Decision    `json:"decisions"`
	OpenCards            []string      `json:"open_cards"`
	OpenQuestions        []string      `json:"open_questions"`
	Proposals            []string      `json:"proposals"`
	UnconfirmedCases     []string      `json:"unconfirmed_cases"`
	OpenAudienceQuestion []string      `json:"open_audience_questions"`
	SignoffPaused        bool          `json:"signoff_paused"`
	StopReportUnseen     bool          `json:"stop_report_unseen"`
	UnansweredMessages   []string      `json:"unanswered_messages"`
}

// Criterion is one current criterion with its understanding cases.
type Criterion struct {
	Label string `json:"label"`
	Text  string `json:"text"`
	State string `json:"state"`
	Cases []Case `json:"cases"`
}

// Case is one understanding case of a criterion.
type Case struct {
	Label  string `json:"label"`
	Text   string `json:"text"`
	Counts bool   `json:"counts"`
	State  string `json:"state"`
}

// SectionRole is the proposed or confirmed role of one section.
type SectionRole struct {
	Label string `json:"label"`
	Role  string `json:"role"`
	State string `json:"state"`
	// Title and Focus («qué debe abordar», yours and in literal, K-17)
	// go with the current task when the task is this section.
	Title string `json:"title,omitempty"`
	Focus string `json:"focus,omitempty"`
}

// Decision is the compact form of a decision (B2-1): label and
// version, statement, reason (the human's «porque», from the
// history), instead of, would choose if, and its edges by label.
// Criticality is computed, never stored (§1.5).
type Decision struct {
	Label         string   `json:"label"`
	Version       int      `json:"version"`
	Statement     string   `json:"statement"`
	Reason        string   `json:"reason"`
	InsteadOf     string   `json:"instead_of"`
	WouldChooseIf string   `json:"would_choose_if"`
	Assumption    bool     `json:"assumption"`
	Origin        string   `json:"origin"`
	State         string   `json:"state"`
	Critical      bool     `json:"critical"`
	Edges         []string `json:"edges"`
}

// Line is one index line (layer 2): label, version, state, first line.
type Line struct {
	Label       string `json:"label"`
	Type        string `json:"type"`
	Version     int    `json:"version"`
	State       string `json:"state"`
	FirstLine   string `json:"first_line"`
	OpenChanges int    `json:"open_changes"`
}

// DeferredItem is one deferred item carried in flight (B2-6). Long
// registers that the item is over 30 words (specification §3, T-6):
// it is never trimmed; the flag is the signal.
type DeferredItem struct {
	What string `json:"what"`
	Why  string `json:"why"`
	Long bool   `json:"long"`
}

// InFlight is layer 3: mode, the current step, the last triage,
// agent pendings, open changes, open rounds, the deferred items and
// the agent note of the last step, complete.
type InFlight struct {
	Mode              string              `json:"mode"`
	ModeMessage       string              `json:"mode_message"`
	CurrentStep       string              `json:"current_step"`
	LastTriage        string              `json:"last_triage"`
	AgentPending      []string            `json:"agent_pending"`
	OpenChanges       map[string][]string `json:"open_changes"`
	CoverageInventory string              `json:"coverage_inventory"`
	OpenRounds        []string            `json:"open_rounds"`
	Deferred          []DeferredItem      `json:"deferred"`
	AgentNote         string              `json:"agent_note"`
}

type body map[string]any

func decodeBody(obj store.Object) body {
	var b body
	_ = json.Unmarshal(obj.Body, &b)
	return b
}

func (b body) str(key string) string {
	s, _ := b[key].(string)
	return s
}

func (b body) boolean(key string) bool {
	v, _ := b[key].(bool)
	return v
}

func firstLine(text string) string {
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(strings.TrimLeft(line, "# "))
		if line != "" {
			return line
		}
	}
	return ""
}

func openChangeLabels(b body) []string {
	raw, _ := b["open_changes"].([]any)
	var out []string
	for _, item := range raw {
		switch v := item.(type) {
		case string:
			out = append(out, v)
		case map[string]any:
			if s, _ := v["label"].(string); s != "" {
				out = append(out, s)
			} else if s, _ := v["id"].(string); s != "" {
				out = append(out, s)
			}
		}
	}
	return out
}

// WorkingState computes the three layers from the graph and the
// history. Nothing is cached as a source (specification §5).
func (m *Memory) WorkingState(ctx context.Context, projectID string) (State, error) {
	st, _, _, err := m.snapshot(ctx, projectID)
	return st, err
}

// snapshot is WorkingState plus the exact objects and events it was
// computed from, so a startup records deliveries for the versions
// that actually went into its brief (and no later ones).
func (m *Memory) snapshot(ctx context.Context, projectID string) (State, []store.Object, []store.Event, error) {
	objects, err := m.db.AllObjects(ctx, projectID)
	if err != nil {
		return State{}, nil, nil, err
	}
	labeler, err := graph.NewLabeler(ctx, m.db, projectID)
	if err != nil {
		return State{}, nil, nil, err
	}
	events, err := m.db.EventsSince(ctx, projectID, 0)
	if err != nil {
		return State{}, nil, nil, err
	}

	byType := make(map[store.ObjectType][]store.Object)
	sections := make(map[string]store.Object)
	for _, obj := range objects {
		byType[obj.Type] = append(byType[obj.Type], obj)
		if obj.Type == store.ObjectSection {
			sections[obj.ID] = obj
		}
	}
	edgesFrom := make(map[string][]store.Edge)
	edgesTo := make(map[string][]store.Edge)
	for _, obj := range objects {
		from, err := m.db.EdgesFrom(ctx, projectID, obj.ID)
		if err != nil {
			return State{}, nil, nil, err
		}
		edgesFrom[obj.ID] = from
		for _, e := range from {
			edgesTo[e.ToID] = append(edgesTo[e.ToID], e)
		}
	}

	var st State
	st.InFlight.Mode = "normal"
	st.InFlight.OpenChanges = make(map[string][]string)

	if reqs := byType[store.ObjectRequest]; len(reqs) > 0 {
		b := decodeBody(reqs[0])
		st.Essentials.RequestText = b.str("text")
		st.Essentials.Audience = b.str("audience")
	}
	for _, crit := range byType[store.ObjectCriterion] {
		b := decodeBody(crit)
		c := Criterion{Label: crit.Label, Text: b.str("text"), State: crit.State}
		for _, cs := range byType[store.ObjectUnderstandingCase] {
			cb := decodeBody(cs)
			if cb.str("criterion_id") == crit.ID {
				c.Cases = append(c.Cases, Case{Label: cs.Label, Text: cb.str("text"), Counts: cb.boolean("counts"), State: cs.State})
			}
		}
		st.Essentials.Criteria = append(st.Essentials.Criteria, c)
	}
	if prems := byType[store.ObjectPremise]; len(prems) > 0 {
		b := decodeBody(prems[0])
		st.Essentials.PremiseText = b.str("text")
		st.Essentials.PremiseExploratory = b.boolean("exploratory")
	}
	graph.SortSections(byType[store.ObjectSection]) // document order (K-17)
	for _, sec := range byType[store.ObjectSection] {
		b := decodeBody(sec)
		role, _ := b["role"].(map[string]any)
		roleValue, _ := role["value"].(string)
		roleState, _ := role["state"].(string)
		st.Essentials.SectionRoles = append(st.Essentials.SectionRoles,
			SectionRole{Label: sec.Label, Role: roleValue, State: roleState, Title: b.str("title"), Focus: b.str("focus")})
		if changes := openChangeLabels(b); len(changes) > 0 {
			st.InFlight.OpenChanges[sec.Label] = changes
		}
	}

	// The «porque» of a decision is the last reason the history
	// records for it, in literal (event.reason, specification §1).
	lastReason := make(map[string]string)
	for _, ev := range events {
		if ev.ObjectID != "" && ev.Reason != "" {
			lastReason[ev.ObjectID] = ev.Reason
		}
	}

	// Criticality comes from the one function of T-5.
	reasons, err := graph.CriticalAll(ctx, m.db, projectID)
	if err != nil {
		return State{}, nil, nil, err
	}
	critical := make(map[string]bool, len(reasons))
	for id := range reasons {
		critical[id] = true
	}
	for _, dec := range byType[store.ObjectDecision] {
		b := decodeBody(dec)
		isYours := b.str("origin") == "human" || dec.State == "human" || dec.State == "confirmed"
		if !isYours && !critical[dec.ID] {
			continue
		}
		d := Decision{
			Label: dec.Label, Version: dec.CurrentVersion, Statement: b.str("statement"),
			Reason:    lastReason[dec.ID],
			InsteadOf: b.str("instead_of"), WouldChooseIf: b.str("would_choose_if"),
			Assumption: b.boolean("assumption"), Origin: b.str("origin"), State: dec.State,
			Critical: critical[dec.ID],
		}
		for _, e := range edgesFrom[dec.ID] {
			if to, err := labeler.ToLabel(e.ToID); err == nil {
				d.Edges = append(d.Edges, string(e.Type)+" "+to)
			}
		}
		sort.Strings(d.Edges)
		st.Essentials.Decisions = append(st.Essentials.Decisions, d)
	}

	for _, card := range byType[store.ObjectCard] {
		if card.State == "open" {
			st.Essentials.OpenCards = append(st.Essentials.OpenCards, card.Label)
		}
	}
	for _, q := range byType[store.ObjectOpenQuestion] {
		if q.State == "open" {
			st.Essentials.OpenQuestions = append(st.Essentials.OpenQuestions, q.Label)
		}
	}
	for _, p := range byType[store.ObjectProposal] {
		if p.State == "open" {
			st.Essentials.Proposals = append(st.Essentials.Proposals, p.Label)
		}
	}
	for _, cs := range byType[store.ObjectUnderstandingCase] {
		if cs.State == "unconfirmed" {
			st.Essentials.UnconfirmedCases = append(st.Essentials.UnconfirmedCases, cs.Label)
		}
	}
	for _, aq := range byType[store.ObjectAudienceQuestion] {
		if aq.State == "open" {
			st.Essentials.OpenAudienceQuestion = append(st.Essentials.OpenAudienceQuestion, aq.Label)
		}
	}
	for _, rnd := range byType[store.ObjectRound] {
		if rnd.State == "open" {
			st.InFlight.OpenRounds = append(st.InFlight.OpenRounds, rnd.Label)
			if inv := decodeBody(rnd).str("coverage_inventory"); inv != "" {
				st.InFlight.CoverageInventory = inv
			}
		}
	}

	// Layer 2 index: sections, non-critical decisions, rounds and
	// sources, one line each; sections in document order (K-17).
	for _, obj := range sectionsInOrder(objects) {
		b := decodeBody(obj)
		text := b.str("text")
		if text == "" {
			text = b.str("statement")
		}
		if text == "" {
			text = b.str("kind")
		}
		switch obj.Type {
		case store.ObjectSection:
			st.Index = append(st.Index, Line{Label: obj.Label, Type: string(obj.Type),
				Version: obj.CurrentVersion, State: obj.State, FirstLine: firstLine(text),
				OpenChanges: len(openChangeLabels(b))})
		case store.ObjectDecision:
			if !critical[obj.ID] {
				st.Index = append(st.Index, Line{Label: obj.Label, Type: string(obj.Type),
					Version: obj.CurrentVersion, State: obj.State, FirstLine: firstLine(text)})
			}
		case store.ObjectRound:
			st.Index = append(st.Index, Line{Label: obj.Label, Type: string(obj.Type),
				Version: obj.CurrentVersion, State: obj.State, FirstLine: firstLine(text)})
		case store.ObjectSource:
			// R-3: a source's line is its card; the model reads the rest
			// with read_source and search_sources.
			st.Index = append(st.Index, Line{Label: obj.Label, Type: string(obj.Type),
				Version: obj.CurrentVersion, State: obj.State, FirstLine: sourceCard(b)})
		}
	}

	answered, unanswered := humanMessages(events)
	st.Essentials.UnansweredMessages = unanswered
	_ = answered

	lastStepEnded := -1
	lastStopReport := -1
	for i, ev := range events {
		switch ev.Type {
		case store.EventModeSet:
			var data map[string]any
			_ = json.Unmarshal(ev.Data, &data)
			if v, _ := data["mode"].(string); v != "" {
				st.InFlight.Mode = v
			}
			if v, _ := data["message"].(string); v != "" {
				st.InFlight.ModeMessage = v
			}
		case store.EventSignoffPaused:
			st.Essentials.SignoffPaused = true
		case store.EventStopReport:
			lastStopReport = i
			var data map[string]any
			_ = json.Unmarshal(ev.Data, &data)
			seen, _ := data["seen"].(bool)
			st.Essentials.StopReportUnseen = !seen
		case store.EventStepEnded:
			lastStepEnded = i
			var data map[string]any
			_ = json.Unmarshal(ev.Data, &data)
			st.InFlight.CurrentStep = ev.TurnID
			if v, _ := data["triage_line"].(string); v != "" {
				st.InFlight.LastTriage = v
			}
			if v, _ := data["note"].(string); v != "" {
				st.InFlight.AgentNote = v
			}
		}
	}
	// The deferred items are carried since the last stop report
	// (B2-6), complete and never trimmed (T-6).
	for i, ev := range events {
		if ev.Type != store.EventStepEnded || i < lastStopReport {
			continue
		}
		var data struct {
			Deferred []DeferredItem `json:"deferred"`
		}
		_ = json.Unmarshal(ev.Data, &data)
		for _, item := range data.Deferred {
			words := len(strings.Fields(item.What + " " + item.Why))
			item.Long = words > 30
			st.InFlight.Deferred = append(st.InFlight.Deferred, item)
		}
	}
	// An agent pending counts only while no later step has ended.
	for i, ev := range events {
		if ev.Type != store.EventOutputRetry || i < lastStepEnded {
			continue
		}
		var data struct {
			Result string   `json:"result"`
			Errors []string `json:"errors"`
		}
		_ = json.Unmarshal(ev.Data, &data)
		if data.Result == "agent_pending" {
			st.InFlight.AgentPending = data.Errors
		}
	}
	// Every computed pending of the agent (estados §1, B-15), with its
	// section and passage, from the one function every view uses
	// (graph.SectionStatus), in document order (P-51, bloque-2 §2).
	computed, err := m.agentPendings(ctx, projectID, byType[store.ObjectSection])
	if err != nil {
		return State{}, nil, nil, err
	}
	st.InFlight.AgentPending = append(st.InFlight.AgentPending, computed...)
	return st, objects, events, nil
}

type humanMessage struct {
	Text     string
	Response string
}

// humanMessages splits the human_message events into answered and
// unanswered, in history order. The human's text is always verbatim.
// [EL DISEÑO NO LO DICE]: the event data fields are text and
// response; table 1.3 names the content, not the field names.
func humanMessages(events []store.Event) (answered []humanMessage, unanswered []string) {
	for _, ev := range events {
		if ev.Type != store.EventHumanMessage {
			continue
		}
		var data map[string]any
		_ = json.Unmarshal(ev.Data, &data)
		text, _ := data["text"].(string)
		response, _ := data["response"].(string)
		if text == "" {
			continue
		}
		if response == "" {
			unanswered = append(unanswered, text)
		} else {
			answered = append(answered, humanMessage{Text: text, Response: response})
		}
	}
	return answered, unanswered
}

// firstWords returns the first n words of s and whether it had more.
func firstWords(s string, n int) (string, bool) {
	words := strings.Fields(s)
	if len(words) <= n {
		return s, false
	}
	return strings.Join(words[:n], " "), true
}

// sectionsInOrder returns a copy of objects where the sections, in
// their own slots, follow document order (K-17).
func sectionsInOrder(objects []store.Object) []store.Object {
	out := append([]store.Object(nil), objects...)
	var slots []int
	var sections []store.Object
	for i, obj := range out {
		if obj.Type == store.ObjectSection {
			slots = append(slots, i)
			sections = append(sections, obj)
		}
	}
	graph.SortSections(sections)
	for k, i := range slots {
		out[i] = sections[k]
	}
	return out
}

// AgentPendings are the agent's computed pendings of the project, in
// document order, as the startup renders them (P-51); the chain's
// continuation message carries them too (P-54, flujos §1).
func (m *Memory) AgentPendings(ctx context.Context, projectID string) ([]string, error) {
	sections, err := graph.OrderedSections(ctx, m.db, projectID)
	if err != nil {
		return nil, err
	}
	return m.agentPendings(ctx, projectID, sections)
}

// agentPendings renders the agent's computed pendings of each section:
// all of graph.SectionStatus except the open cards, which are yours,
// and the malformed output, which the output_retry errors above carry.
func (m *Memory) agentPendings(ctx context.Context, projectID string, sections []store.Object) ([]string, error) {
	var out []string
	for _, sec := range sections {
		_, pendings, err := graph.SectionStatus(ctx, m.db, projectID, sec.Label)
		if err != nil {
			return nil, err
		}
		b := decodeBody(sec)
		for _, p := range pendings {
			if p.Kind == graph.PendingOpenCard || p.Kind == graph.PendingMalformedOutput {
				continue
			}
			line := fmt.Sprintf("%s «%s» · %s", sec.Label, b.str("title"), p.Kind)
			if p.About != "" {
				line += ": " + p.About
			}
			if passage := pendingPassage(b.str("text"), p); passage != "" {
				line += " · passage: «" + passage + "»"
			}
			out = append(out, line)
		}
	}
	return out, nil
}

// pendingPassage is the block of the section's text where a figure
// without decision appears, so the agent can quote it as an anchor.
// The other kinds already name their passage or their label.
func pendingPassage(text string, p graph.Pending) string {
	if p.Kind != graph.PendingFigureWithoutDecision {
		return ""
	}
	for _, block := range graph.Blocks(text) {
		for _, n := range graph.Numbers(block) {
			if n.Raw == p.About || n.Value == p.About {
				return strings.TrimSpace(block)
			}
		}
	}
	return ""
}
