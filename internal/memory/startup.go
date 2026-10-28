package memory

import (
	"context"
	"crypto/rand"
	"encoding/base32"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/LaurenteEber/tupaqnin/internal/store"
)

// StartupReason is why a new principal instance starts (B2-2,
// specification table 1.3).
type StartupReason string

const (
	StartupNewSession  StartupReason = "new_session"
	StartupModelChange StartupReason = "model_change"
	StartupThreshold   StartupReason = "threshold"
)

// TokenCounter counts the tokens of input items for a model already
// bound by the caller (B-5; backlog 0.9).
type TokenCounter func(ctx context.Context, input []json.RawMessage) (int, error)

// Brief is one startup: the input items for the new instance, in the
// design order (core, current task, trimmed index, what changed,
// your messages), its measured tokens, and whether it still passes
// the cap after trimming (block 2 §3: then the app warns).
// [EL DISEÑO NO LO DICE]: the Brief shape is not in the design; this
// is the smallest form the turn runner of 0.10 needs.
type Brief struct {
	InstanceID string
	Reason     StartupReason
	Input      []json.RawMessage
	Tokens     int
	OverCap    bool
}

// Item is one object as read(id) returns it: its current text with
// its version (block 2 §3).
type Item struct {
	Label   string `json:"label"`
	Type    string `json:"type"`
	Version int    `json:"version"`
	State   string `json:"state"`
	Text    string `json:"text"`
}

// Change is one object whose current version was not yet delivered.
type Change struct {
	Label     string `json:"label"`
	Type      string `json:"type"`
	Version   int    `json:"version"`
	State     string `json:"state"`
	FirstLine string `json:"first_line"`
}

func newInstanceID() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", fmt.Errorf("generate instance id: %w", err)
	}
	return strings.ToLower(base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(b[:])), nil
}

func inputItem(text string) json.RawMessage {
	raw, _ := json.Marshal(map[string]any{
		"type": "message", "role": "user",
		"content": []map[string]string{{"type": "input_text", "text": text}},
	})
	return raw
}

func renderItems(parts []string) []json.RawMessage {
	var items []json.RawMessage
	for _, p := range parts {
		if strings.TrimSpace(p) != "" {
			items = append(items, inputItem(p))
		}
	}
	return items
}

func (m *Memory) renderCore(st State) string {
	var sb strings.Builder
	sb.WriteString("Core\n")
	if st.Essentials.RequestText != "" {
		fmt.Fprintf(&sb, "Request: %s\n", st.Essentials.RequestText)
	}
	if st.Essentials.Audience != "" {
		fmt.Fprintf(&sb, "Audience: %s\n", st.Essentials.Audience)
	}
	for _, c := range st.Essentials.Criteria {
		fmt.Fprintf(&sb, "Criterion %s: %s\n", c.Label, c.Text)
		for _, cs := range c.Cases {
			fmt.Fprintf(&sb, "- case %s (counts %v): %s\n", cs.Label, cs.Counts, cs.Text)
		}
	}
	if st.Essentials.PremiseText != "" {
		fmt.Fprintf(&sb, "Premise: %s\n", st.Essentials.PremiseText)
	}
	for _, r := range st.Essentials.SectionRoles {
		fmt.Fprintf(&sb, "Section %s role: %s (%s)\n", r.Label, r.Role, r.State)
	}
	for _, d := range st.Essentials.Decisions {
		if d.Critical {
			continue // critical decisions go with the current task
		}
		sb.WriteString("Decision " + compactDecision(d) + "\n")
	}
	if len(st.Essentials.OpenCards) > 0 {
		fmt.Fprintf(&sb, "Open cards: %s\n", strings.Join(st.Essentials.OpenCards, ", "))
	}
	if len(st.Essentials.OpenQuestions) > 0 {
		fmt.Fprintf(&sb, "Open questions: %s\n", strings.Join(st.Essentials.OpenQuestions, ", "))
	}
	if len(st.Essentials.Proposals) > 0 {
		fmt.Fprintf(&sb, "Open proposals: %s\n", strings.Join(st.Essentials.Proposals, ", "))
	}
	if len(st.Essentials.UnconfirmedCases) > 0 {
		fmt.Fprintf(&sb, "Unconfirmed cases: %s\n", strings.Join(st.Essentials.UnconfirmedCases, ", "))
	}
	// In flight (layer 3), complete; the agent note enters as its
	// first 80 words, marked truncated (T-6).
	f := st.InFlight
	fmt.Fprintf(&sb, "Mode: %s\n", f.Mode)
	if f.LastTriage != "" {
		fmt.Fprintf(&sb, "Last triage: %s\n", f.LastTriage)
	}
	for _, p := range f.AgentPending {
		fmt.Fprintf(&sb, "Agent pending: %s\n", p)
	}
	for _, item := range f.Deferred {
		mark := ""
		if item.Long {
			mark = " [deferred item over 30 words, registered]"
		}
		fmt.Fprintf(&sb, "Deferred: %s — %s%s\n", item.What, item.Why, mark)
	}
	if f.AgentNote != "" {
		note, truncated := firstWords(f.AgentNote, 80)
		mark := ""
		if truncated {
			mark = " [truncated]"
		}
		fmt.Fprintf(&sb, "Agent note: %s%s\n", note, mark)
	}
	return sb.String()
}

func compactDecision(d Decision) string {
	edges := ""
	if len(d.Edges) > 0 {
		edges = " · " + strings.Join(d.Edges, ", ")
	}
	return fmt.Sprintf("%s v%d: %s · instead of: %s · would choose if: %s%s",
		d.Label, d.Version, d.Statement, d.InsteadOf, d.WouldChooseIf, edges)
}

func (m *Memory) renderTask(st State) string {
	var sb strings.Builder
	sb.WriteString("Current task\n")
	switch {
	case len(st.Essentials.UnansweredMessages) > 0:
		fmt.Fprintf(&sb, "Your unanswered message: %s\n", st.Essentials.UnansweredMessages[0])
	case len(st.InFlight.AgentPending) > 0:
		fmt.Fprintf(&sb, "First agent pending: %s\n", st.InFlight.AgentPending[0])
	case len(st.InFlight.Deferred) > 0:
		fmt.Fprintf(&sb, "First deferred item: %s — %s\n", st.InFlight.Deferred[0].What, st.InFlight.Deferred[0].Why)
	default:
		sb.WriteString("Stop: only your items remain.\n")
	}
	for _, d := range st.Essentials.Decisions {
		if d.Critical {
			sb.WriteString("Decision " + compactDecision(d) + "\n")
		}
	}
	return sb.String()
}

func renderIndex(lines []Line, sectionsOnly bool) string {
	var sb strings.Builder
	sb.WriteString("Index\n")
	if !sectionsOnly {
		for _, l := range lines {
			fmt.Fprintf(&sb, "- %s · v%d · %s · %s\n", l.Label, l.Version, l.State, l.FirstLine)
		}
		return sb.String()
	}
	counts := make(map[string]int)
	for _, l := range lines {
		if l.Type == "section" {
			fmt.Fprintf(&sb, "- %s · v%d · %s · %s\n", l.Label, l.Version, l.State, l.FirstLine)
		} else {
			counts[l.Type]++
		}
	}
	types := make([]string, 0, len(counts))
	for t := range counts {
		types = append(types, t)
	}
	sort.Strings(types)
	for _, t := range types {
		fmt.Fprintf(&sb, "%d %s → read(id)\n", counts[t], t)
	}
	return sb.String()
}

func renderChanges(changes []Change, oneLine bool) string {
	var sb strings.Builder
	sb.WriteString("What changed\n")
	for _, c := range changes {
		if oneLine {
			fmt.Fprintf(&sb, "- %s v%d\n", c.Label, c.Version)
		} else {
			fmt.Fprintf(&sb, "- %s (%s) v%d · %s · %s\n", c.Label, c.Type, c.Version, c.State, c.FirstLine)
		}
	}
	return sb.String()
}

func renderMessages(messages []humanMessage) string {
	var sb strings.Builder
	sb.WriteString("Your messages\n")
	for _, msg := range messages {
		fmt.Fprintf(&sb, "You: %s\nPrincipal: %s\n", msg.Text, msg.Response)
	}
	return sb.String()
}

// Startup assembles one startup for a new principal instance, in the
// design order, measures it with count against 15% of window, and
// trims in the design order (index, then changes, then the oldest
// answered messages; core and current task never). It writes the
// delivery rows (via = startup) and the startup event with the new
// instance (B2-2, specification §1.4).
func (m *Memory) Startup(ctx context.Context, projectID string, reason StartupReason, window int, count TokenCounter) (Brief, error) {
	switch reason {
	case StartupNewSession, StartupModelChange, StartupThreshold:
	default:
		return Brief{}, fmt.Errorf("memory: invalid startup reason %q", reason)
	}
	if count == nil {
		return Brief{}, fmt.Errorf("memory: a token counter is required")
	}
	st, err := m.WorkingState(ctx, projectID)
	if err != nil {
		return Brief{}, err
	}
	events, err := m.db.EventsSince(ctx, projectID, 0)
	if err != nil {
		return Brief{}, err
	}
	answered, _ := humanMessages(events)
	if len(answered) > 10 {
		answered = answered[len(answered)-10:]
	}
	changes, err := m.ChangesSinceDelivery(ctx, projectID)
	if err != nil {
		return Brief{}, err
	}

	core := m.renderCore(st)
	task := m.renderTask(st)
	indexSectionsOnly := false
	changesOneLine := false

	build := func() []json.RawMessage {
		return renderItems([]string{
			core, task,
			renderIndex(st.Index, indexSectionsOnly),
			renderChanges(changes, changesOneLine),
			renderMessages(answered),
		})
	}
	items := build()
	tokens, err := count(ctx, items)
	if err != nil {
		return Brief{}, fmt.Errorf("memory: count startup tokens: %w", err)
	}
	capTokens := window * 15 / 100
	// Trimming order of block 2 §3; re-measure after each step.
	if tokens > capTokens {
		indexSectionsOnly = true
		items = build()
		if tokens, err = count(ctx, items); err != nil {
			return Brief{}, fmt.Errorf("memory: count startup tokens: %w", err)
		}
	}
	if tokens > capTokens {
		changesOneLine = true
		items = build()
		if tokens, err = count(ctx, items); err != nil {
			return Brief{}, fmt.Errorf("memory: count startup tokens: %w", err)
		}
	}
	for tokens > capTokens && len(answered) > 0 {
		answered = answered[1:]
		items = build()
		if tokens, err = count(ctx, items); err != nil {
			return Brief{}, fmt.Errorf("memory: count startup tokens: %w", err)
		}
	}

	instanceID, err := newInstanceID()
	if err != nil {
		return Brief{}, err
	}
	var startupSeq int
	err = m.db.Write(ctx, projectID, func(tx *store.Tx) error {
		ev, err := tx.AppendEvent(store.EventInput{
			Type: store.EventStartup, Actor: "app",
			Data: map[string]any{"reason": string(reason), "instance_id": instanceID},
		})
		if err != nil {
			return err
		}
		startupSeq = ev.Seq
		return nil
	})
	if err != nil {
		return Brief{}, err
	}
	objects, err := m.db.AllObjects(ctx, projectID)
	if err != nil {
		return Brief{}, err
	}
	for _, obj := range objects {
		if err := m.db.RecordDelivery(ctx, store.Delivery{
			ProjectID: projectID, InstanceID: instanceID, Turn: startupSeq,
			Via: store.ViaStartup, ObjectID: obj.ID, Version: obj.CurrentVersion,
		}); err != nil {
			return Brief{}, err
		}
	}
	return Brief{
		InstanceID: instanceID, Reason: reason, Input: items,
		Tokens: tokens, OverCap: tokens > capTokens,
	}, nil
}

// ChangesSinceDelivery returns what is current against the last
// delivered version of each object, whatever the instance (B2-2).
func (m *Memory) ChangesSinceDelivery(ctx context.Context, projectID string) ([]Change, error) {
	objects, err := m.db.AllObjects(ctx, projectID)
	if err != nil {
		return nil, err
	}
	delivered, err := m.db.LatestDeliveries(ctx, projectID)
	if err != nil {
		return nil, err
	}
	var out []Change
	for _, obj := range objects {
		if v, ok := delivered[obj.ID]; ok && v == obj.CurrentVersion {
			continue
		}
		b := decodeBody(obj)
		text := b.str("text")
		if text == "" {
			text = b.str("statement")
		}
		out = append(out, Change{Label: obj.Label, Type: string(obj.Type),
			Version: obj.CurrentVersion, State: obj.State, FirstLine: firstLine(text)})
	}
	return out, nil
}

// Read returns the current text of one object by label, with its
// version, and records the delivery (via = read) for the instance.
func (m *Memory) Read(ctx context.Context, projectID, instanceID, label string) (Item, error) {
	obj, err := m.db.GetObject(ctx, projectID, label)
	if err != nil {
		return Item{}, err
	}
	b := decodeBody(obj)
	text := b.str("text")
	if text == "" {
		text = b.str("statement")
	}
	if text == "" {
		raw, err := json.Marshal(b)
		if err != nil {
			return Item{}, fmt.Errorf("memory: encode body: %w", err)
		}
		text = string(raw)
	}
	if err := m.db.RecordDelivery(ctx, store.Delivery{
		ProjectID: projectID, InstanceID: instanceID, Via: store.ViaRead,
		ObjectID: obj.ID, Version: obj.CurrentVersion,
	}); err != nil {
		return Item{}, err
	}
	return Item{Label: obj.Label, Type: string(obj.Type), Version: obj.CurrentVersion,
		State: obj.State, Text: text}, nil
}
