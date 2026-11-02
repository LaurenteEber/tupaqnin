package cycle

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/LaurenteEber/tupaqnin/internal/checks"
	"github.com/LaurenteEber/tupaqnin/internal/fingerprint"
	"github.com/LaurenteEber/tupaqnin/internal/graph"
	"github.com/LaurenteEber/tupaqnin/internal/store"
)

// Proposals (backlog-r4 4.1, B-73 to B-77): what the agent wanted to write
// and you decide. Born in the step's transaction, or in your save's when it
// displaces the agent's text (B-73 d); they never block the chain and are
// never merged: accepting applies them as they are, or asks for a new step.

var (
	// ErrProposalNotFound: no proposal has that label.
	ErrProposalNotFound = errors.New("cycle: proposal not found")
	// ErrProposalNotOpen: it was already accepted or discarded (estados §5).
	ErrProposalNotOpen = errors.New("cycle: proposal not open")
)

// ProposalOutcome is what accepting or discarding left. NewStep says the
// app asks the agent for a new step (B-75); Section is the proposal's
// section, or the one accepting created (B-77).
type ProposalOutcome struct {
	Label   string
	State   string
	Why     string
	Section string
	NewStep bool
}

// createProposals writes the step's proposals, each with its contrast
// (B-74), and returns the handled_changes that apply now: the «actualizado»
// closes of a change of a proposed section go with its proposal (B-73).
func (p *project) createProposals(tx *store.Tx, turnID string, out stepOutput, pl *plan,
	ids, labels map[string]string) (json.RawMessage, error) {
	ctx := context.Background()
	var handled []handledChange
	_ = json.Unmarshal(out.HandledChanges, &handled)
	closes := map[*sectionPlan][]graph.ProposalClose{}
	var now []handledChange
	for _, h := range handled {
		var into *sectionPlan
		if h.Result == "updated" {
			sec, _, ok, err := sectionOfChange(ctx, tx, h.ChangeID)
			if err != nil {
				return nil, err
			}
			for _, sp := range pl.proposals {
				if ok && sp.sectionID == sec.ID {
					into = sp
				}
			}
		}
		if into == nil {
			now = append(now, h)
			continue
		}
		closes[into] = append(closes[into], graph.ProposalClose{ChangeID: h.ChangeID, Result: h.Result, Reason: h.Reason})
	}
	remaining := out.HandledChanges
	if len(now) != len(handled) {
		raw, err := json.Marshal(now)
		if err != nil {
			return nil, err
		}
		remaining = raw
	}

	for _, sp := range pl.proposals {
		b := graph.ProposalBody{SectionID: sp.sectionID, BaseVersion: sp.baseVersion, Origin: sp.why, TurnID: turnID,
			Figures: []graph.ProposalFigure{}, Citations: []graph.ProposalCitation{}, HandledChanges: closes[sp],
			Contrast: graph.ProposalContrast{Removed: []string{}, Added: []string{}, Decisions: []graph.ProposalDecision{},
				TriageLine: out.TriageLine, Note: out.Note}}
		if b.HandledChanges == nil {
			b.HandledChanges = []graph.ProposalClose{}
		}
		for i, e := range sp.edits {
			edit := graph.ProposalEdit{Anchor: e.anchor, Operation: e.operation, Text: e.text, ClaimOrigin: e.origin}
			if i == 0 {
				b.Anchor, b.Operation, b.Text, b.ClaimOrigin = edit.Anchor, edit.Operation, edit.Text, edit.ClaimOrigin
			} else {
				b.MoreEdits = append(b.MoreEdits, edit)
			}
			if e.operation == "replace" {
				b.Contrast.Removed = append(b.Contrast.Removed, e.anchor)
			}
			b.Contrast.Added = append(b.Contrast.Added, e.text)
		}
		if sp.newRole != nil {
			b.NewSection = &graph.ProposalNewSection{Title: sp.newRole.Title, Role: sp.newRole.Role}
			if b.Operation == "" {
				b.Operation = "insert_after"
			}
		} else {
			// The contrast is against the current text, read in the
			// transaction (B-74).
			sec, err := tx.Object(sp.sectionID)
			if err != nil {
				return nil, err
			}
			b.Contrast.Version = sec.CurrentVersion
			b.Contrast.Fingerprint = fingerprint.Section(bodyStr(bodyOf(sec), "text"))
		}
		for _, d := range out.NewDecisions {
			for _, used := range d.UsedInSections {
				if used == sp.ref {
					b.Contrast.Decisions = append(b.Contrast.Decisions, graph.ProposalDecision{
						Label: labels[d.ID], Statement: d.Statement, InsteadOf: d.InsteadOf})
					if sp.newRole != nil { // no section to link it to yet
						b.UsedDecisions = append(b.UsedDecisions, ids[d.ID])
					}
				}
			}
		}
		resolve := func(value string) string {
			if isNew(value) {
				return ids[value]
			}
			obj, _ := p.object(value)
			return obj.ID
		}
		for _, d := range out.UsedDecisions {
			b.UsedDecisions = append(b.UsedDecisions, resolve(d))
		}
		for _, s := range out.UsedSources {
			b.UsedSources = append(b.UsedSources, resolve(s))
		}
		for _, f := range sp.figures {
			b.Figures = append(b.Figures, graph.ProposalFigure{Anchor: f.FigureAnchor, DecisionID: resolve(f.DecisionID)})
		}
		for _, c := range sp.citations {
			b.Citations = append(b.Citations, graph.ProposalCitation{Anchor: c.ClaimAnchor, SourceID: resolve(c.SourceID), Passage: c.Passage})
		}
		repeated, err := repeatsProposal(ctx, tx, b)
		if err != nil {
			return nil, err
		}
		if repeated {
			p.repeatedProposals++
		}
		obj, err := tx.CreateObject(store.ObjectProposal, "open", b, store.EventInput{
			Type: store.EventStateChanged, Actor: "agent", AgentRole: "principal", TurnID: turnID,
			Data: map[string]any{"section": sp.ref, "to": "open", "why": sp.why}})
		if err != nil {
			return nil, err
		}
		p.proposals = append(p.proposals, obj.Label)
	}
	return remaining, nil
}

// closeProposal moves an open proposal to accepted or discarded (estados
// §5), with why in its body and in the event.
func closeProposal(tx *store.Tx, obj store.Object, b graph.ProposalBody, state, why, section, signoff string) error {
	return closeProposalWith(tx, obj, b, state, why, section, signoff, nil)
}

// closeProposalWith is closeProposal with what the acceptance's rules found.
func closeProposalWith(tx *store.Tx, obj store.Object, b graph.ProposalBody, state, why, section, signoff string, extra map[string]any) error {
	b.Why = why
	data := map[string]any{"label": obj.Label, "section": section, "from": "open", "to": state}
	for k, v := range extra {
		data[k] = v
	}
	if why != "" {
		data["why"] = why
	}
	if signoff != "" {
		data["signoff"] = signoff
	}
	_, err := tx.UpdateObject(obj.ID, obj.CurrentVersion, state, b,
		store.EventInput{Type: store.EventStateChanged, Actor: "human", Data: data})
	return err
}

// repeatsProposal says an identical proposal is already open or was discarded:
// same section, same texts, same new section (P-115).
func repeatsProposal(ctx context.Context, tx *store.Tx, b graph.ProposalBody) (bool, error) {
	all, err := tx.ListObjects(ctx, tx.ProjectID(), store.ObjectProposal)
	if err != nil {
		return false, err
	}
	key := func(p graph.ProposalBody) string {
		raw, _ := json.Marshal([]any{p.SectionID, p.Edits(), p.NewSection})
		return string(raw)
	}
	for _, obj := range all {
		if obj.State != "open" && obj.State != "discarded" {
			continue
		}
		other, err := graph.ParseProposal(obj)
		if err != nil {
			return false, err
		}
		if key(other) == key(b) {
			return true, nil
		}
	}
	return false, nil
}

// discardAtSigning is B-76: the section's open proposals go to discarded
// (signed_without), in the signing's transaction.
func discardAtSigning(ctx context.Context, tx *store.Tx, section store.Object, signLabel string) error {
	proposals, err := tx.ListObjects(ctx, tx.ProjectID(), store.ObjectProposal)
	if err != nil {
		return err
	}
	for _, obj := range proposals {
		b, err := graph.ParseProposal(obj)
		if err != nil {
			return err
		}
		if obj.State != "open" || b.SectionID != section.ID {
			continue
		}
		if err := closeProposal(tx, obj, b, "discarded", graph.ProposalSignedWithout, section.Label, signLabel); err != nil {
			return err
		}
	}
	return nil
}

func (g *Gate) openProposal(ctx context.Context, q store.Querier, projectID, label string) (store.Object, graph.ProposalBody, error) {
	obj, err := q.GetObject(ctx, projectID, label)
	if errors.Is(err, store.ErrNotFound) || err == nil && obj.Type != store.ObjectProposal {
		return store.Object{}, graph.ProposalBody{}, ErrProposalNotFound
	}
	if err != nil {
		return store.Object{}, graph.ProposalBody{}, err
	}
	if obj.State != "open" {
		return store.Object{}, graph.ProposalBody{}, ErrProposalNotOpen
	}
	b, err := graph.ParseProposal(obj)
	return obj, b, err
}

// proposalSectionLabel is the section's label, or "" for a section proposal.
func proposalSectionLabel(ctx context.Context, q store.Querier, projectID string, b graph.ProposalBody) (string, error) {
	if b.SectionID == "" {
		return "", nil
	}
	sec, err := q.GetObject(ctx, projectID, b.SectionID)
	if err != nil {
		return "", err
	}
	return sec.Label, nil
}

// DiscardProposal is «Descartar»: no «porque» (B-75), and only the proposal
// moves; a section proposal creates nothing (B-77).
func (g *Gate) DiscardProposal(ctx context.Context, projectID, label string) (ProposalOutcome, error) {
	out := ProposalOutcome{Label: label, State: "discarded", Why: graph.ProposalDiscarded}
	err := g.db.Write(ctx, projectID, func(tx *store.Tx) error {
		obj, b, err := g.openProposal(ctx, tx, projectID, label)
		if err != nil {
			return err
		}
		if out.Section, err = proposalSectionLabel(ctx, tx, projectID, b); err != nil {
			return err
		}
		return closeProposal(tx, obj, b, "discarded", graph.ProposalDiscarded, out.Section, "")
	})
	return out, err
}

// AcceptProposal is «Aceptar» (B-75, T-4): it applies the proposal with its
// anchor and operation on the current text. When it cannot be applied as it
// is (the anchor no longer appears once, or, for a replace_all, the section
// changed since your save and that save was not what you undid, B-79) nothing
// is applied: the proposal goes to discarded with that reason and the agent is
// asked for a new step. The text is the
// agent's: an edit with actor agent and accepted_by human. It never merges
// texts and never calls a model.
func (g *Gate) AcceptProposal(ctx context.Context, projectID, label string) (ProposalOutcome, error) {
	out := ProposalOutcome{Label: label}
	p, err := g.loadProject(ctx, projectID)
	if err != nil {
		return out, err
	}
	// Versions only grow, so the text your save left is read outside the
	// write's transaction (replace_all).
	var saved string
	obj, b, err := g.openProposal(ctx, g.db, projectID, label)
	if err != nil {
		return out, err
	}
	if b.Operation == graph.OperationReplaceAll {
		if text, ok := p.versionText(b.SectionID, b.BaseVersion); ok {
			saved = text
		}
		if p.versionErr != nil {
			return out, p.versionErr
		}
	}
	err = g.db.Write(ctx, projectID, func(tx *store.Tx) error {
		// Read again in the transaction: it may have closed since.
		if obj, b, err = g.openProposal(ctx, tx, projectID, label); err != nil {
			return err
		}
		return TrackCriticality(ctx, tx, "", func() error {
			if b.NewSection != nil {
				return p.acceptSectionProposal(ctx, tx, obj, b, &out)
			}
			return p.acceptTextProposal(ctx, tx, obj, b, saved, &out)
		})
	})
	return out, err
}

// textAfterProposal is what the current text becomes, or why it cannot be
// applied.
func textAfterProposal(current string, b graph.ProposalBody, saved string) (result, why string) {
	if b.Operation == graph.OperationReplaceAll {
		if current != saved {
			return "", graph.ProposalSectionChanged
		}
		return b.Text, ""
	}
	sp := &sectionPlan{ref: "proposal", base: graph.Compare(current).Orig}
	for i, e := range b.Edits() {
		ed := edit{index: i, text: e.Text, anchor: e.Anchor, origin: e.ClaimOrigin, operation: e.Operation}
		if e.Anchor != "" {
			found := graph.Compare(sp.base).Find(e.Anchor)
			if len(found) != 1 {
				return "", graph.ProposalAnchorNotUnique
			}
			ed.start, ed.end = found[0][0], found[0][1]
			if e.Operation == "insert_after" {
				ed.start = ed.end
			}
		}
		sp.edits = append(sp.edits, ed)
	}
	if overlap(sp) != "" {
		return "", graph.ProposalAnchorNotUnique
	}
	return applyEdits(sp.base, sp.edits), ""
}

// acceptedEvent is the agent's text you accepted (B-75): the same data as a
// step's, so the history by block finds it (B-10). A replace_all carries no
// texts: the steps it displaced keep theirs, with their claim_origin (R-1).
func acceptedEvent(t store.EventType, obj store.Object, b graph.ProposalBody) store.EventInput {
	texts, anchors, origins := []string{}, []string{}, []string{}
	if b.Operation != graph.OperationReplaceAll {
		for _, e := range b.Edits() {
			texts = append(texts, e.Text)
			origins = append(origins, e.ClaimOrigin)
			if e.Anchor != "" {
				anchors = append(anchors, e.Anchor)
			}
		}
	}
	return store.EventInput{Type: t, Actor: "agent", AgentRole: "principal", TurnID: b.TurnID,
		Data: map[string]any{"texts": texts, "anchors": anchors, "claim_origins": origins,
			"accepted_by": "human", "proposal": obj.Label}}
}

// citesRevoked: the proposal's step cited a decision revoked since (B-82).
func citesRevoked(ctx context.Context, tx *store.Tx, b graph.ProposalBody) (bool, error) {
	ids := append([]string{}, b.UsedDecisions...)
	for _, f := range b.Figures {
		ids = append(ids, f.DecisionID)
	}
	for _, id := range ids {
		dec, err := tx.GetObject(ctx, tx.ProjectID(), id)
		if err != nil {
			return false, err
		}
		if dec.State == "revoked" {
			return true, nil
		}
	}
	return false, nil
}

func (p *project) acceptTextProposal(ctx context.Context, tx *store.Tx, obj store.Object, b graph.ProposalBody, saved string, out *ProposalOutcome) error {
	sec, err := tx.Object(b.SectionID)
	if err != nil {
		return err
	}
	out.Section = sec.Label
	if b.Operation == graph.OperationReplaceAll {
		// B-79: you undid the edit that displaced it and nothing moved since:
		// it still replaces the section's text.
		undid, err := graph.LastWriteUndid(ctx, tx, tx.ProjectID(), sec.ID, b.BaseVersion)
		if err != nil {
			return err
		}
		if undid {
			saved = bodyStr(bodyOf(sec), "text")
		}
	}
	result, why := textAfterProposal(bodyStr(bodyOf(sec), "text"), b, saved)
	revoked, err := citesRevoked(ctx, tx, b)
	if err != nil {
		return err
	}
	if revoked {
		why = graph.ProposalDecisionRevoked
	}
	if why != "" {
		out.State, out.Why, out.NewStep = "discarded", why, true
		return closeProposal(tx, obj, b, "discarded", why, sec.Label, "")
	}
	body := bodyOf(sec)
	body["text"] = result
	// Estados §5: accepting on a signed section moves it to «cambió».
	state := sec.State
	if state == "signed" {
		state = "changed_since_signoff"
	}
	if _, err := tx.UpdateObject(sec.ID, sec.CurrentVersion, state, body, acceptedEvent(store.EventEdit, obj, b)); err != nil && !errors.Is(err, store.ErrNoChange) {
		return err
	}
	if err := p.linkAccepted(ctx, tx, obj, b, sec, result); err != nil {
		return err
	}
	// [EL DISEÑO NO LO DICE]: accepting on a section in signoff changes what
	// is being signed; it cancels the signoff, like your edit (K-16).
	signing, inSignoff, err := graph.ActiveSignoff(ctx, tx, tx.ProjectID(), sec.ID)
	if err != nil {
		return err
	}
	if inSignoff {
		if err := CancelSignoff(tx, signing, graph.CancelEdited, "human", map[string]any{"section": sec.Label}); err != nil {
			return err
		}
	}
	found, err := rulesOnAccepted(ctx, tx, b, sec)
	if err != nil {
		return err
	}
	out.State = "accepted"
	return closeProposalWith(tx, obj, b, "accepted", "", sec.Label, "", found)
}

// rulesOnAccepted runs the app's rules 1 to 5 on an accepted proposal's text, as
// on any agent text (P-114), in the acceptance's transaction. Rules 1, 3 and 5 are
// read from the current state; what rule 2 finds goes on the proposal's close, the
// way a step's goes on its step_ended. Rule 0 does not apply: an accepted text is
// no direct step.
func rulesOnAccepted(ctx context.Context, tx *store.Tx, b graph.ProposalBody, sec store.Object) (map[string]any, error) {
	decided := len(b.UsedDecisions) > 0 || len(b.Contrast.Decisions) > 0
	texts := []string{}
	for _, e := range b.Edits() {
		texts = append(texts, e.Text)
	}
	findings, err := checks.RunRules(ctx, tx, tx.ProjectID(), checks.Change{Kind: checks.AgentStep, Stretch: checks.StretchBuild,
		Sections: []string{sec.ID}, Texts: texts, Figures: len(b.Figures) > 0, Citations: len(b.Citations) > 0,
		Decided: map[string]bool{sec.ID: decided}, Actor: "agent", TurnID: b.TurnID})
	if err != nil {
		return nil, err
	}
	withoutDecision, closed := []string{}, []string{}
	for _, f := range findings {
		if f.Kind == graph.PendingTextWithoutDecision {
			withoutDecision = append(withoutDecision, f.Section)
		}
	}
	if decided {
		closed = append(closed, sec.Label)
	}
	return map[string]any{"accepted_by": "human", "text_without_decision": withoutDecision, "decided_sections": closed}, nil
}

// acceptSectionProposal creates the section as a step creates one before the
// plan (B-77), last in the order (K-17).
func (p *project) acceptSectionProposal(ctx context.Context, tx *store.Tx, obj store.Object, b graph.ProposalBody, out *ProposalOutcome) error {
	result, _ := textAfterProposal("", b, "")
	position, err := graph.NextSectionPosition(ctx, tx, tx.ProjectID())
	if err != nil {
		return err
	}
	sec, err := tx.CreateObject(store.ObjectSection, "draft", map[string]any{
		"title": b.NewSection.Title, "position": position, "text": result,
		"role":         map[string]any{"value": b.NewSection.Role, "state": "proposed"},
		"open_changes": []any{},
	}, acceptedEvent(store.EventStateChanged, obj, b))
	if err != nil {
		return err
	}
	p.byID[sec.ID] = sec
	if err := p.linkAccepted(ctx, tx, obj, b, sec, result); err != nil {
		return err
	}
	found, err := rulesOnAccepted(ctx, tx, b, sec)
	if err != nil {
		return err
	}
	out.State, out.Section = "accepted", sec.Label
	return closeProposalWith(tx, obj, b, "accepted", "", sec.Label, "", found)
}

// linkAccepted applies what was kept with the proposal: the step's edges to
// the section, its figures and citations whose anchor is in the text, its
// data, and the closes it declared (A-1).
// [EL DISEÑO NO LO DICE]: a figure or citation whose anchor is no longer in
// the text at accepting, and a close no longer in the list, are not applied
// and stay in the proposal's event.
func (p *project) linkAccepted(ctx context.Context, tx *store.Tx, obj store.Object, b graph.ProposalBody, sec store.Object, result string) error {
	ev := acceptedEvent(store.EventStateChanged, obj, b)
	ev.Data = map[string]any{"accepted_by": "human", "proposal": obj.Label}
	edges := newEdgeSet(p, tx, store.EventInput{Type: store.EventEdge, Actor: "agent", AgentRole: "principal", TurnID: b.TurnID,
		Data: map[string]any{"accepted_by": "human", "proposal": obj.Label}})
	for _, d := range b.UsedDecisions {
		if err := edges.add(store.EdgeUsedIn, d, sec.ID, ""); err != nil {
			return err
		}
	}
	for _, s := range b.UsedSources {
		if err := edges.add(store.EdgeUses, sec.ID, s, ""); err != nil {
			return err
		}
	}
	text := graph.Compare(result)
	linked := map[string]bool{}
	var missing []string
	for _, f := range b.Figures {
		if len(text.Find(f.Anchor)) == 0 || len(graph.Numbers(f.Anchor)) == 0 {
			missing = append(missing, f.Anchor)
			continue
		}
		if err := linkFigure(tx, edges, sec.ID, f.Anchor, f.DecisionID, ev); err != nil {
			return err
		}
		for v := range graph.NumberValues(f.Anchor) {
			linked[v] = true
		}
	}
	for _, c := range b.Citations {
		if len(text.Find(c.Anchor)) == 0 {
			missing = append(missing, c.Anchor)
			continue
		}
		src := store.Object{}
		if c.SourceID != "" {
			var err error
			if src, err = tx.Object(c.SourceID); err != nil {
				return err
			}
		}
		if _, err := p.linkCitation(tx, edges, sec.ID, c.Anchor, c.Passage, src, ev); err != nil {
			return err
		}
	}
	// A replace_all's data never left the graph: only figures and citations do.
	if b.Operation != graph.OperationReplaceAll {
		step := stepOutput{}
		for _, s := range b.UsedSources {
			if src, ok := p.byID[s]; ok {
				step.UsedSources = append(step.UsedSources, src.Label)
			}
		}
		const ref = "new-accepted"
		for _, e := range b.Edits() {
			step.NewText = append(step.NewText, newTextItem{SectionID: ref, Text: e.Text})
		}
		if err := p.linkData(tx, step, &plan{}, map[string]string{ref: sec.ID}, linked, edges, ev); err != nil {
			return err
		}
	}
	closes, err := json.Marshal(b.HandledChanges)
	if err != nil {
		return err
	}
	written := map[string]bool{sec.ID: true}
	rejected, err := applyHandledChanges(ctx, tx, b.TurnID, closes, written)
	if err != nil {
		return err
	}
	if len(missing) == 0 && len(rejected) == 0 {
		return nil
	}
	_, err = tx.AppendEvent(store.EventInput{Type: store.EventStateChanged, Actor: "app", ObjectID: obj.ID, Data: map[string]any{
		"proposal": obj.Label, "accepted_without": missing, "closes_not_applied": len(rejected)}})
	return err
}

// Displacement is B-73 d: your save over a version the agent changed after
// the one your tab read.
type Displacement struct {
	BaseVersion int
}

// AgentOnTop says whether every text written above baseVersion is the
// agent's (B-73 d): only then your save rules and its text goes to a
// proposal. Anything of yours on top keeps the 409 (P-84): a second tab does
// not happen (specification §2), and a stale save of yours has nothing of the
// agent to displace.
func AgentOnTop(ctx context.Context, q store.Querier, projectID, sectionID string, baseVersion int) (bool, error) {
	events, err := q.EventsSince(ctx, projectID, 0)
	if err != nil {
		return false, err
	}
	found := false
	for _, ev := range events {
		if ev.ObjectID != sectionID || ev.Type != store.EventEdit && ev.Type != store.EventEditUndone || ev.NewVersion <= baseVersion {
			continue
		}
		if ev.Actor != "agent" {
			return false, nil
		}
		found = true
	}
	return found, nil
}

// DisplaceAgentText writes the one proposal your save displaced (B-73 d), in
// your save's transaction, after your version: replace_all with the agent's
// whole text, and the figures and citations those steps applied, which leave
// the current text and come back at accepting. One whose anchor is still in
// your text stays: it is still true of what rules.
// [EL DISEÑO NO LO DICE]: a figure or citation «se quita» by ending its edges
// and emptying its section_id, the field every reader filters by; no state is
// added and nothing is deleted (rule 6).
func DisplaceAgentText(ctx context.Context, tx *store.Tx, sec store.Object, agentText string, d Displacement) (string, error) {
	projectID := tx.ProjectID()
	events, err := tx.EventsSince(ctx, projectID, 0)
	if err != nil {
		return "", err
	}
	baseSeq := 0
	created := map[string]int{}
	turnID, triage, note := "", "", ""
	for _, ev := range events {
		if ev.NewVersion == 1 && ev.ObjectID != "" {
			created[ev.ObjectID] = ev.Seq
		}
		if ev.ObjectID == sec.ID && ev.NewVersion == d.BaseVersion && baseSeq == 0 {
			baseSeq = ev.Seq
		}
		if ev.ObjectID == sec.ID && ev.Type == store.EventEdit && ev.Actor == "agent" && ev.NewVersion > d.BaseVersion {
			turnID = ev.TurnID
		}
		if ev.Type == store.EventStepEnded && baseSeq > 0 && ev.TurnID == turnID && turnID != "" {
			var data struct {
				TriageLine string `json:"triage_line"`
				Note       string `json:"note"`
			}
			_ = json.Unmarshal(ev.Data, &data)
			triage, note = data.TriageLine, data.Note
		}
	}
	your := bodyStr(bodyOf(sec), "text")
	yours := graph.Compare(your)
	b := graph.ProposalBody{SectionID: sec.ID, BaseVersion: sec.CurrentVersion, Operation: graph.OperationReplaceAll,
		Text: agentText, Origin: graph.ProposalFromDisplacement, TurnID: turnID,
		Figures: []graph.ProposalFigure{}, Citations: []graph.ProposalCitation{}, HandledChanges: []graph.ProposalClose{},
		Contrast: graph.ProposalContrast{Version: sec.CurrentVersion, Fingerprint: fingerprint.Section(your),
			Removed: []string{your}, Added: []string{agentText}, Decisions: []graph.ProposalDecision{},
			TriageLine: triage, Note: note}}

	var leaving []store.Object
	for _, t := range []store.ObjectType{store.ObjectFigure, store.ObjectCitation} {
		objects, err := tx.ListObjects(ctx, projectID, t)
		if err != nil {
			return "", err
		}
		for _, obj := range objects {
			ob := bodyOf(obj)
			anchor := bodyStr(ob, "anchor")
			if bodyStr(ob, "section_id") != sec.ID || created[obj.ID] <= baseSeq || len(yours.Find(anchor)) > 0 {
				continue
			}
			edges, err := tx.EdgesFrom(ctx, projectID, obj.ID)
			if err != nil {
				return "", err
			}
			target := ""
			for _, e := range edges {
				if e.Type == store.EdgeCovers || e.Type == store.EdgeComesFrom {
					target = e.ToID
				}
			}
			if t == store.ObjectFigure {
				b.Figures = append(b.Figures, graph.ProposalFigure{Anchor: anchor, DecisionID: target})
			} else {
				b.Citations = append(b.Citations, graph.ProposalCitation{Anchor: anchor, SourceID: target, Passage: bodyStr(ob, "passage")})
			}
			leaving = append(leaving, obj)
		}
	}
	// B-74: the decisions of those steps that the section uses and still rule.
	decisions, err := tx.ListObjects(ctx, projectID, store.ObjectDecision)
	if err != nil {
		return "", err
	}
	for _, dec := range decisions {
		if created[dec.ID] <= baseSeq || dec.State == "superseded" || dec.State == "revoked" {
			continue
		}
		edges, err := tx.EdgesFrom(ctx, projectID, dec.ID)
		if err != nil {
			return "", err
		}
		for _, e := range edges {
			if e.Type == store.EdgeUsedIn && e.ToID == sec.ID {
				db := bodyOf(dec)
				b.Contrast.Decisions = append(b.Contrast.Decisions, graph.ProposalDecision{
					Label: dec.Label, Statement: bodyStr(db, "statement"), InsteadOf: bodyStr(db, "instead_of")})
				break
			}
		}
	}
	proposal, err := tx.CreateObject(store.ObjectProposal, "open", b, store.EventInput{
		Type: store.EventStateChanged, Actor: "app",
		Data: map[string]any{"section": sec.Label, "to": "open", "why": graph.ProposalFromDisplacement}})
	if err != nil {
		return "", err
	}
	for _, obj := range leaving {
		ev := store.EventInput{Type: store.EventEdge, Actor: "app", Data: map[string]any{"why": graph.ProposalFromDisplacement, "proposal": proposal.Label}}
		edges, err := tx.EdgesFrom(ctx, projectID, obj.ID)
		if err != nil {
			return "", err
		}
		for _, e := range edges {
			if err := tx.EndEdge(e.ID, ev); err != nil {
				return "", err
			}
		}
		ob := bodyOf(obj)
		ob["section_id"] = ""
		if _, err := tx.UpdateObject(obj.ID, obj.CurrentVersion, obj.State, ob, store.EventInput{Type: store.EventStateChanged, Actor: "app",
			Data: map[string]any{"why": graph.ProposalFromDisplacement, "proposal": proposal.Label}}); err != nil {
			return "", err
		}
	}
	return proposal.Label, nil
}

// StartStepAfterProposal is B-75's «pedir un paso nuevo»: the notice is the
// proposal's line in «lo que cambió»; with no turn running a turn opens for it, passing the spend gate like any step, and with one running the
// notice enters its next step. It returns the turn it opened, or "".
func (r *Runner) StartStepAfterProposal(projectID string) (string, error) {
	if !r.tryAcquireTurn(projectID) {
		return "", nil
	}
	turnID, err := newTurnID()
	if err != nil {
		r.releaseTurn(projectID)
		return "", fmt.Errorf("cycle: new step: %w", err)
	}
	if r.pub != nil {
		r.pub.BeginTurn(projectID, turnID)
	}
	r.answers.Add(1)
	go func() {
		defer r.answers.Done()
		defer r.endTurn(r.ctx, projectID, turnID)
		if err := r.runChain(r.ctx, projectID, turnID, "", false); err != nil {
			r.logTurnFailure(err)
		}
	}()
	return turnID, nil
}
