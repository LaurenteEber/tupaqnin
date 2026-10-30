package graph

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/LaurenteEber/tupaqnin/internal/fingerprint"
	"github.com/LaurenteEber/tupaqnin/internal/store"
)

// The signoff (flujos §3, fichas §2 «Firma», B-13, B-14). A signoff in
// progress is a signoff object (sign-n) that is not signed yet; its
// phase is derived from the history, never stored (B-14 precision).

// SignoffPhase is where a signoff is, read from its events (B-14).
type SignoffPhase string

const (
	// SignoffInProgress: a sign-n without signoff_cancelled, without the
	// signing and without a signoff_paused after its last round.
	SignoffInProgress SignoffPhase = "in_progress"
	// SignoffPaused: its last event is signoff_paused (E-2); asking to
	// sign again creates another sign-n.
	SignoffPaused SignoffPhase = "paused"
	// SignoffCancelled: it has a signoff_cancelled (left · edited ·
	// missing_in_text).
	SignoffCancelled SignoffPhase = "cancelled"
	// SignoffSigned: the section moved to signed with this signoff.
	SignoffSigned SignoffPhase = "signed"
)

// Reasons of signoff_cancelled (glossary §4, B-14).
const (
	CancelLeft          = "left"
	CancelEdited        = "edited"
	CancelMissingInText = "missing_in_text"
)

// MaxFailedCases is how many failed cases pause a signoff (E-2
// [inicial]).
const MaxFailedCases = 2

// PendingMissingInText is «falta en el texto» señalado por Wonder
// (glossary §9): an agent pending, with Wonder's passage, that closes
// when the section's fingerprint changes (B-15).
const PendingMissingInText = "missing_in_text"

// The values of signoff_step in the data of a sign-n's events: the
// request, your explanation, your answers and the signing.
const (
	StepRequested   = "requested"
	StepExplanation = "explanation"
	StepAnswers     = "answers"
	StepSigned      = "signed"
)

// SignoffBody is what a sign-n stores (specification §4, «lo que guarda
// la firma»; B-14: the section version, your explanation, the answers
// and the result of each round). Ids are internal: the view and the
// brief for Wonder translate them to labels, here only.
type SignoffBody struct {
	SectionID      string      `json:"section_id"`
	SectionVersion int         `json:"section_version"`
	Brief          bool        `json:"brief"`
	Critical       []SignedRef `json:"critical_decisions"`
	// OriginAssumptionID is the assumption the app chose in the graph
	// (fichas §9); empty when there is no edge and Wonder chooses.
	OriginAssumptionID string        `json:"origin_assumption_id"`
	Explanation        string        `json:"explanation"`
	OriginQuestion     string        `json:"origin_question"`
	OriginAnswer       string        `json:"origin_answer"`
	Cases              []SignoffCase `json:"cases"`
	FailedCases        int           `json:"failed_cases"`
	// Coincides is decided in Go from Wonder's marks of the last round
	// (backlog-r1 1.7).
	Coincides         bool   `json:"coincides"`
	ContrastedBy      string `json:"contrasted_by"`
	WonderUnavailable bool   `json:"wonder_unavailable"`
	// The commitment, written when signing (specification §4).
	SectionFingerprint string         `json:"section_fingerprint,omitempty"`
	Criteria           []SignedRef    `json:"criteria,omitempty"`
	Sources            []SignedSource `json:"sources,omitempty"`
	AnsweredCards      []string       `json:"answered_cards,omitempty"`
	// Rounds are Wonder's results, beside the commitment, not inside it
	// (specification §4).
	Rounds []SignoffRound `json:"rounds"`
	// Ended mirrors the event that ended the signoff, so its last
	// version differs (left · edited · missing_in_text · paused ·
	// signed). The phase is still read from the history.
	Ended string `json:"ended,omitempty"`
}

// SignedRef is an object by internal id and version.
type SignedRef struct {
	ID      string `json:"id"`
	Version int    `json:"version"`
}

// SignedSource is a source by internal id and fingerprint.
type SignedSource struct {
	ID          string `json:"id"`
	Fingerprint string `json:"fingerprint"`
}

// SignoffCase is one application case of the signoff (glossary §7,
// application_case) with your answer and Wonder's judgment of it.
type SignoffCase struct {
	Case   string `json:"case"`
	Answer string `json:"answer"`
	Result string `json:"result"` // applies_well · does_not_apply · "" while not judged
}

// SignoffRound is one Wonder round: who contrasted and its output, as
// it came (labels only).
type SignoffRound struct {
	ContrastedBy      string          `json:"contrasted_by"`
	WonderUnavailable bool            `json:"wonder_unavailable"`
	Output            json.RawMessage `json:"output"`
	Coincides         bool            `json:"coincides"`
}

// ParseSignoff reads a sign-n's body.
func ParseSignoff(obj store.Object) (SignoffBody, error) {
	var b SignoffBody
	if err := json.Unmarshal(obj.Body, &b); err != nil {
		return SignoffBody{}, fmt.Errorf("graph: parse signoff: %w", err)
	}
	return b, nil
}

// phaseOf derives a signoff's phase from its own events, in order
// (B-14 precision).
func phaseOf(events []store.Event) (SignoffPhase, string) {
	phase := SignoffInProgress
	for _, ev := range events {
		data := eventData(ev)
		switch {
		case ev.Type == store.EventSignoffCancelled:
			reason, _ := data["reason"].(string)
			return SignoffCancelled, reason
		case ev.Type == store.EventBriefSignoff || data["signoff_step"] == StepSigned:
			return SignoffSigned, ""
		case ev.Type == store.EventSignoffPaused:
			phase = SignoffPaused
		default:
			phase = SignoffInProgress
		}
	}
	return phase, ""
}

// signoffIndex is every signoff of the project with its events.
type signoffIndex struct {
	signoffs []store.Object
	events   map[string][]store.Event
	created  map[string]int
}

func loadSignoffs(ctx context.Context, db *store.DB, projectID string) (*signoffIndex, []store.Event, error) {
	objects, err := db.ListObjects(ctx, projectID, store.ObjectSignoff)
	if err != nil {
		return nil, nil, err
	}
	events, err := db.EventsSince(ctx, projectID, 0)
	if err != nil {
		return nil, nil, err
	}
	idx := &signoffIndex{signoffs: objects, events: map[string][]store.Event{}, created: map[string]int{}}
	ids := map[string]bool{}
	for _, o := range objects {
		ids[o.ID] = true
	}
	for _, ev := range events {
		if !ids[ev.ObjectID] {
			continue
		}
		if _, ok := idx.created[ev.ObjectID]; !ok {
			idx.created[ev.ObjectID] = ev.Seq
		}
		idx.events[ev.ObjectID] = append(idx.events[ev.ObjectID], ev)
	}
	return idx, events, nil
}

// latest returns the newest signoff of a section, if any.
func (idx *signoffIndex) latest(sectionID string) (store.Object, bool) {
	var best store.Object
	found := false
	for _, o := range idx.signoffs {
		if bodyString(o, "section_id") != sectionID {
			continue
		}
		if !found || idx.created[o.ID] > idx.created[best.ID] {
			best, found = o, true
		}
	}
	return best, found
}

// CurrentSignoff is the section's current signature: its newest signoff
// that was signed (spec §9: «la explicación de firma vigente»). A
// signoff in progress, paused or cancelled is not a signature.
func CurrentSignoff(ctx context.Context, db *store.DB, projectID, sectionID string) (SignoffBody, bool, error) {
	idx, _, err := loadSignoffs(ctx, db, projectID)
	if err != nil {
		return SignoffBody{}, false, err
	}
	var best store.Object
	found := false
	for _, o := range idx.signoffs {
		if bodyString(o, "section_id") != sectionID {
			continue
		}
		if phase, _ := phaseOf(idx.events[o.ID]); phase != SignoffSigned {
			continue
		}
		if !found || idx.created[o.ID] > idx.created[best.ID] {
			best, found = o, true
		}
	}
	if !found {
		return SignoffBody{}, false, nil
	}
	body, err := ParseSignoff(best)
	return body, err == nil, err
}

// SignoffPhaseOf is the phase of one signoff and, when cancelled, its
// reason.
func SignoffPhaseOf(ctx context.Context, db *store.DB, projectID, signID string) (SignoffPhase, string, error) {
	idx, _, err := loadSignoffs(ctx, db, projectID)
	if err != nil {
		return "", "", err
	}
	phase, reason := phaseOf(idx.events[signID])
	return phase, reason, nil
}

// ActiveSignoff returns the signoff in progress of a section, if there
// is one.
func ActiveSignoff(ctx context.Context, db *store.DB, projectID, sectionID string) (store.Object, bool, error) {
	idx, _, err := loadSignoffs(ctx, db, projectID)
	if err != nil {
		return store.Object{}, false, err
	}
	o, ok := idx.latest(sectionID)
	if !ok {
		return store.Object{}, false, nil
	}
	if phase, _ := phaseOf(idx.events[o.ID]); phase != SignoffInProgress {
		return store.Object{}, false, nil
	}
	return o, true, nil
}

// SectionsInSignoff returns the ids of the sections with a signoff in
// progress (B-14: an agent step that touches one is rejected there).
func SectionsInSignoff(ctx context.Context, db *store.DB, projectID string) (map[string]bool, error) {
	idx, _, err := loadSignoffs(ctx, db, projectID)
	if err != nil {
		return nil, err
	}
	out := map[string]bool{}
	for _, o := range idx.signoffs {
		sec := bodyString(o, "section_id")
		if latest, _ := idx.latest(sec); latest.ID != o.ID {
			continue
		}
		if phase, _ := phaseOf(idx.events[o.ID]); phase == SignoffInProgress {
			out[sec] = true
		}
	}
	return out, nil
}

// labelNumber is the number of a label (dec-12 → 12), for the order in
// which objects arrived.
func labelNumber(label string) int {
	i := strings.LastIndex(label, "-")
	n, _ := strconv.Atoi(label[i+1:])
	return n
}

// SectionCriticalDecisions returns the current critical decisions of a
// section (T-5), in the order they arrived: those used_in the section
// and those covering a figure in it, as the frame reads them.
func SectionCriticalDecisions(ctx context.Context, q store.Querier, projectID, sectionID string) ([]store.Object, error) {
	critical, err := CriticalAll(ctx, q, projectID)
	if err != nil {
		return nil, err
	}
	var out []store.Object
	for id := range critical {
		dec, err := q.GetObject(ctx, projectID, id)
		if err != nil {
			return nil, err
		}
		if dec.State == "superseded" || dec.State == "revoked" {
			continue
		}
		in, err := decisionInSection(ctx, q, projectID, dec.ID, sectionID)
		if err != nil {
			return nil, err
		}
		if in {
			out = append(out, dec)
		}
	}
	sort.Slice(out, func(i, j int) bool { return labelNumber(out[i].Label) < labelNumber(out[j].Label) })
	return out, nil
}

func decisionInSection(ctx context.Context, q store.Querier, projectID, decisionID, sectionID string) (bool, error) {
	outEdges, err := q.EdgesFrom(ctx, projectID, decisionID)
	if err != nil {
		return false, err
	}
	for _, e := range outEdges {
		if e.Type == store.EdgeUsedIn && e.ToID == sectionID {
			return true, nil
		}
	}
	inEdges, err := q.EdgesTo(ctx, projectID, decisionID)
	if err != nil {
		return false, err
	}
	for _, e := range inEdges {
		if e.Type != store.EdgeCovers {
			continue
		}
		fig, err := q.GetObject(ctx, projectID, e.FromID)
		if err != nil {
			return false, err
		}
		if bodyString(fig, "section_id") == sectionID {
			return true, nil
		}
	}
	return false, nil
}

// OriginAssumption is the assumption of the origin question (fichas §9,
// E-3): the one the most critical decisions of the section depend on,
// following depends_on_assumption. Empty when there is no edge: then
// Wonder chooses [inicial].
// [EL DISEÑO NO LO DICE]: a tie; the assumption that arrived first.
func OriginAssumption(ctx context.Context, q store.Querier, projectID string, critical []store.Object) (string, error) {
	count := map[string]int{}
	labels := map[string]string{}
	for _, dec := range critical {
		edges, err := q.EdgesFrom(ctx, projectID, dec.ID)
		if err != nil {
			return "", err
		}
		for _, e := range edges {
			if e.Type != store.EdgeDependsOnAssumption {
				continue
			}
			if _, ok := labels[e.ToID]; !ok {
				target, err := q.GetObject(ctx, projectID, e.ToID)
				if err != nil {
					return "", err
				}
				labels[e.ToID] = target.Label
			}
			count[e.ToID]++
		}
	}
	best := ""
	for id, n := range count {
		switch {
		case best == "", n > count[best]:
			best = id
		case n == count[best] && labelNumber(labels[id]) < labelNumber(labels[best]):
			best = id
		}
	}
	return best, nil
}

// missingInText returns, in order and once each, the passages of the
// pending «falta en el texto» of a section: the signoff_cancelled
// events with reason missing_in_text whose section fingerprint is still
// the current one (B-14, B-15).
func missingInText(section store.Object, events []store.Event) []string {
	current := fingerprint.Section(bodyString(section, "text"))
	var out []string
	seen := map[string]bool{}
	for _, ev := range events {
		if ev.Type != store.EventSignoffCancelled {
			continue
		}
		data := eventData(ev)
		if data["reason"] != CancelMissingInText || data["section_id"] != section.ID || data["fingerprint"] != current {
			continue
		}
		passages, _ := data["passages"].([]any)
		for _, p := range passages {
			s, _ := p.(string)
			if s != "" && !seen[s] {
				seen[s] = true
				out = append(out, s)
			}
		}
	}
	return out
}

// WonderVerdict is Wonder's mark of one critical decision in the last
// round, with the block of the section where its point is (P7: «Ver la
// historia de ese punto»).
type WonderVerdict struct {
	Decision string `json:"decision"`
	// Number names the decision on screen: its place among the critical
	// decisions, from 1. Never its label (rule 7) nor its statement,
	// which is the answer (fichas §2 Firma: «sin la respuesta»; P-59).
	Number  int    `json:"number"`
	Verdict string `json:"verdict"`
	Missing string `json:"missing,omitempty"`
	Passage string `json:"passage,omitempty"`
	Block   int    `json:"block"`
	// HistoryOffered: P7 offers «Ver la historia de ese punto» (P-41,
	// corrected 30-oct, flujos §3): after Wonder names what is missing
	// in your explanation, or once the signoff is no longer in
	// progress. Verdicts exist only after your first explanation. With
	// Block 0 (no point in the text) it opens the decision's own
	// history (DecisionHistory).
	HistoryOffered bool `json:"history_offered"`
}

// SignoffView is what P7 reads (pantallas P7): computed in Go, labels
// only.
type SignoffView struct {
	Label        string       `json:"label"`
	Section      string       `json:"section"`
	SectionTitle string       `json:"section_title"`
	Phase        SignoffPhase `json:"phase"`
	Cancelled    string       `json:"cancelled,omitempty"`
	Brief        bool         `json:"brief"`
	// Pendings of the section now (estados §1); signing needs none.
	Pendings          []Pending       `json:"pendings"`
	CriticalDecisions []string        `json:"critical_decisions"`
	OriginAssumption  string          `json:"origin_assumption,omitempty"`
	Explanation       string          `json:"explanation"`
	OriginQuestion    string          `json:"origin_question"`
	OriginAnswer      string          `json:"origin_answer"`
	Verdicts          []WonderVerdict `json:"verdicts"`
	Cases             []SignoffCase   `json:"cases"`
	// OpenCase is the new case waiting for your answer, if any.
	OpenCase          string `json:"open_case,omitempty"`
	FailedCases       int    `json:"failed_cases"`
	MaxFailedCases    int    `json:"max_failed_cases"`
	Rounds            int    `json:"rounds"`
	ContrastedBy      string `json:"contrasted_by,omitempty"`
	WonderUnavailable bool   `json:"wonder_unavailable"`
	Coincides         bool   `json:"coincides"`
	// CanSign: in progress, no pendings, and everything coincides (or,
	// in a brief signoff, your explanation is there).
	CanSign bool `json:"can_sign"`
}

// WonderOutput is Wonder's fixed output (salida-wonder.schema.json,
// glossary §10).
type WonderOutput struct {
	Decisions []struct {
		Decision string  `json:"decision"`
		Verdict  string  `json:"verdict"`
		Missing  *string `json:"missing"`
		Passage  *string `json:"passage"`
	} `json:"decisions"`
	OriginQuestion struct {
		Question string  `json:"question"`
		Answer   *string `json:"answer"`
	} `json:"origin_question"`
	PreviousCase *string `json:"previous_case"`
	NewCase      *string `json:"new_case"`
}

// OpenCase is the case of the signoff waiting for an answer.
func (b SignoffBody) OpenCase() (int, bool) {
	if n := len(b.Cases); n > 0 && b.Cases[n-1].Answer == "" {
		return n - 1, true
	}
	return 0, false
}

// SignoffDetail is the view of one signoff (P7).
func SignoffDetail(ctx context.Context, db *store.DB, projectID, signLabel string) (SignoffView, error) {
	sign, err := db.GetObject(ctx, projectID, signLabel)
	if err != nil || sign.Type != store.ObjectSignoff {
		return SignoffView{}, fmt.Errorf("%w: signoff %s", store.ErrNotFound, signLabel)
	}
	body, err := ParseSignoff(sign)
	if err != nil {
		return SignoffView{}, err
	}
	labels, err := NewLabeler(ctx, db, projectID)
	if err != nil {
		return SignoffView{}, err
	}
	section, err := db.GetObject(ctx, projectID, body.SectionID)
	if err != nil {
		return SignoffView{}, err
	}
	phase, reason, err := SignoffPhaseOf(ctx, db, projectID, sign.ID)
	if err != nil {
		return SignoffView{}, err
	}
	_, pendings, err := SectionStatus(ctx, db, projectID, section.Label)
	if err != nil {
		return SignoffView{}, err
	}
	v := SignoffView{Label: sign.Label, Section: section.Label, SectionTitle: bodyString(section, "title"),
		Phase: phase, Cancelled: reason, Brief: body.Brief, Pendings: pendings,
		CriticalDecisions: []string{}, Explanation: body.Explanation,
		OriginQuestion: body.OriginQuestion, OriginAnswer: body.OriginAnswer,
		Verdicts: []WonderVerdict{}, Cases: append([]SignoffCase{}, body.Cases...),
		FailedCases: body.FailedCases, MaxFailedCases: MaxFailedCases, Rounds: len(body.Rounds),
		ContrastedBy: body.ContrastedBy, WonderUnavailable: body.WonderUnavailable, Coincides: body.Coincides}
	for _, d := range body.Critical {
		if l, err := labels.ToLabel(d.ID); err == nil {
			v.CriticalDecisions = append(v.CriticalDecisions, l)
		}
	}
	for i, p := range v.Pendings {
		if p.Kind != PendingOpenCard {
			continue
		}
		card, err := db.GetObject(ctx, projectID, p.About)
		if err != nil {
			continue
		}
		dec, err := labels.ToLabel(bodyString(card, "decision_id"))
		if err != nil {
			continue
		}
		for j, l := range v.CriticalDecisions {
			if l == dec {
				v.Pendings[i].Number = j + 1
			}
		}
	}
	if body.OriginAssumptionID != "" {
		v.OriginAssumption, _ = labels.ToLabel(body.OriginAssumptionID)
	}
	if i, ok := body.OpenCase(); ok {
		v.OpenCase = body.Cases[i].Case
		v.Cases = v.Cases[:i]
	}
	if n := len(body.Rounds); n > 0 {
		var out WonderOutput
		_ = json.Unmarshal(body.Rounds[n-1].Output, &out)
		blocks := Blocks(bodyString(section, "text"))
		anchors, err := figureAnchors(ctx, db, projectID, section.ID)
		if err != nil {
			return SignoffView{}, err
		}
		for _, d := range out.Decisions {
			w := WonderVerdict{Decision: d.Decision, Verdict: d.Verdict, Block: blockOf(blocks, anchors[d.Decision])}
			for i, l := range v.CriticalDecisions {
				if l == d.Decision {
					w.Number = i + 1
				}
			}
			if d.Missing != nil {
				w.Missing = *d.Missing
			}
			if d.Passage != nil {
				w.Passage = *d.Passage
				if b := blockOf(blocks, []string{*d.Passage}); b > 0 {
					w.Block = b
				}
			}
			w.HistoryOffered = w.Verdict != "match" &&
				(phase != SignoffInProgress || w.Verdict == "missing_in_explanation")
			v.Verdicts = append(v.Verdicts, w)
		}
	}
	v.CanSign = phase == SignoffInProgress && len(pendings) == 0 &&
		((body.Brief && strings.TrimSpace(body.Explanation) != "") || (!body.Brief && body.Coincides))
	return v, nil
}

// figureAnchors maps each decision label to the anchors of the figures
// it covers in the section.
func figureAnchors(ctx context.Context, db *store.DB, projectID, sectionID string) (map[string][]string, error) {
	figures, err := db.ListObjects(ctx, projectID, store.ObjectFigure)
	if err != nil {
		return nil, err
	}
	out := map[string][]string{}
	for _, fig := range figures {
		if bodyString(fig, "section_id") != sectionID {
			continue
		}
		edges, err := db.EdgesFrom(ctx, projectID, fig.ID)
		if err != nil {
			return nil, err
		}
		for _, e := range edges {
			if e.Type != store.EdgeCovers {
				continue
			}
			dec, err := db.GetObject(ctx, projectID, e.ToID)
			if err != nil {
				return nil, err
			}
			out[dec.Label] = append(out[dec.Label], bodyString(fig, "anchor"))
		}
	}
	return out, nil
}

// WonderBrief is what Wonder receives (fichas §7, fila «Wonder»): the
// brief of bloque-1.md §4, reduced. The request, criteria and premise in
// literal; the section's current text; its history (the critical
// decisions with their contrast, your «porque» and what they replaced);
// the list of critical decisions; your explanation; each case of this
// signoff with your answer; and the origin question's assumption, chosen
// by the app, with your answer if you gave it. Labels only (rule 7).
// [EL DISEÑO NO LO DICE]: «su historia» is reduced to the critical
// decisions' contrast, reasons and what they replaced; the block
// history of the text is not sent.
func WonderBrief(ctx context.Context, db *store.DB, projectID, signID string) (string, error) {
	sign, err := db.GetObject(ctx, projectID, signID)
	if err != nil {
		return "", err
	}
	body, err := ParseSignoff(sign)
	if err != nil {
		return "", err
	}
	section, err := db.GetObject(ctx, projectID, body.SectionID)
	if err != nil {
		return "", err
	}
	reasons, err := cardReasons(ctx, db, projectID)
	if err != nil {
		return "", err
	}
	var b strings.Builder
	line := func(format string, args ...any) { fmt.Fprintf(&b, format+"\n", args...) }

	line("# Signoff brief")
	line("")
	if reqs, err := db.ListObjects(ctx, projectID, store.ObjectRequest); err == nil && len(reqs) > 0 {
		line("## Request of the person (literal)")
		line("%s", bodyString(reqs[0], "text"))
		if a := bodyString(reqs[0], "audience"); a != "" {
			line("Audience: %s", a)
		}
		line("")
	}
	if crits, err := db.ListObjects(ctx, projectID, store.ObjectCriterion); err == nil && len(crits) > 0 {
		line("## Criteria (literal)")
		for _, c := range crits {
			line("- %s: %s", c.Label, bodyString(c, "text"))
		}
		line("")
	}
	if prems, err := db.ListObjects(ctx, projectID, store.ObjectPremise); err == nil && len(prems) > 0 {
		line("## Premise (literal)")
		line("%s: %s", prems[0].Label, bodyString(prems[0], "text"))
		line("")
	}
	line("## Section %s «%s», current text", section.Label, bodyString(section, "title"))
	line("<<<")
	line("%s", bodyString(section, "text"))
	line(">>>")
	line("")
	line("## Critical decisions of this section (one entry each in \"decisions\", by label)")
	for _, ref := range body.Critical {
		dec, err := db.GetObject(ctx, projectID, ref.ID)
		if err != nil {
			return "", err
		}
		assumption := "no"
		if v, _ := bodyValue(dec, "assumption").(bool); v {
			assumption = "yes"
		}
		line("- %s: %s", dec.Label, bodyString(dec, "statement"))
		line("  Instead of: %s", bodyString(dec, "instead_of"))
		line("  Would choose the other if: %s", bodyString(dec, "would_choose_if"))
		line("  Assumption: %s · origin: %s", assumption, bodyString(dec, "origin"))
		reason := reasons[dec.ID]
		if reason == "" {
			reason = bodyString(dec, "reason")
		}
		if reason != "" {
			line("  The person's reason: %s", reason)
		}
		edges, err := db.EdgesFrom(ctx, projectID, dec.ID)
		if err != nil {
			return "", err
		}
		for _, e := range edges {
			if e.Type != store.EdgeSupersedes {
				continue
			}
			old, err := db.GetObject(ctx, projectID, e.ToID)
			if err != nil {
				return "", err
			}
			line("  It replaced %s: %s", old.Label, bodyString(old, "statement"))
		}
	}
	line("")
	line("## Origin question")
	if body.OriginAssumptionID != "" {
		as, err := db.GetObject(ctx, projectID, body.OriginAssumptionID)
		if err != nil {
			return "", err
		}
		text := bodyString(as, "statement")
		if as.Type == store.ObjectPremise {
			text = bodyString(as, "text")
		}
		line("The app chose the assumption %s: %s", as.Label, text)
	} else {
		line("No critical decision depends on an assumption: choose the assumption yourself.")
	}
	if body.OriginQuestion != "" {
		line("The question you asked: %s", body.OriginQuestion)
	}
	if body.OriginAnswer != "" {
		line("The person's answer (literal): %s", body.OriginAnswer)
	} else {
		line("The person has not answered it yet: answer must be null.")
	}
	line("")
	line("## The person's explanation, written without looking (literal)")
	line("<<<")
	line("%s", body.Explanation)
	line(">>>")
	line("")
	line("## Application cases of this signoff")
	if len(body.Cases) == 0 {
		line("None yet: previous_case must be null.")
	}
	for i, c := range body.Cases {
		line("%d. Case: %s", i+1, c.Case)
		line("   The person's answer (literal): %s", c.Answer)
		if c.Result != "" {
			line("   Judged: %s", c.Result)
		} else {
			line("   Not judged yet: judge it in previous_case.")
		}
	}
	return b.String(), nil
}
