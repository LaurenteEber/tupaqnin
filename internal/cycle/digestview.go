package cycle

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/LaurenteEber/tupaqnin/internal/checks"
	"github.com/LaurenteEber/tupaqnin/internal/graph"
	"github.com/LaurenteEber/tupaqnin/internal/store"
)

// What the round card shows of the digest (backlog-r3 3.4; pantallas
// «Dónde va cada vista de la rebanada 3»): first the answer, the outline
// with its findings, «Descartados (n)» and «Sin revisar» in sight,
// «¿se redujo la duda?», and «Repetir con otra pregunta» with its cost.
// Go computes it; the page shows it. Labels only, never ids.

// FindingView is one finding as the card shows it: its number in the
// digest's list (0 when it is not in it), its reader and number there,
// kind, the source's own stretch (the reader's passage until one is
// recorded) and the note as it wrote it, its source by label
// (or its hit, or the reference's title) with its URL when external,
// and its verification with why.
type FindingView struct {
	N            int    `json:"n"`
	Part         int    `json:"part"`
	Finding      int    `json:"finding"`
	Kind         string `json:"kind"`
	Passage      string `json:"passage"`
	Note         string `json:"note,omitempty"`
	Source       string `json:"source,omitempty"`
	SourceTitle  string `json:"source_title,omitempty"`
	SourceURL    string `json:"source_url,omitempty"`
	Hit          string `json:"hit,omitempty"`
	Reference    string `json:"reference,omitempty"`
	Verification string `json:"verification"`
	Reason       string `json:"reason,omitempty"`
	Access       string `json:"access,omitempty"`
	// OpenQuestion is the «por aclarar» a reader's contradiction opened
	// (a label), if any.
	OpenQuestion string `json:"open_question,omitempty"`
}

// DigestView is the principal's part of the digest: the answer, the
// outline with the findings each line uses, the contrast with the
// expectation and the contradictions, each with both findings and the
// «por aclarar» it opened (a label), if any.
type DigestView struct {
	Answer              string                  `json:"answer"`
	Outline             []DigestLineView        `json:"outline"`
	ExpectationContrast string                  `json:"expectation_contrast,omitempty"`
	Contradictions      []ContradictionViewItem `json:"contradictions"`
}

// DigestLineView is one outline line and the findings it uses.
type DigestLineView struct {
	Text     string        `json:"text"`
	Findings []FindingView `json:"findings"`
}

// ContradictionViewItem is one contradiction of the digest: shown, never
// resolved (flujos §2).
type ContradictionViewItem struct {
	A            FindingView `json:"a"`
	B            FindingView `json:"b"`
	About        string      `json:"about"`
	SameSource   bool        `json:"same_source"`
	OpenQuestion string      `json:"open_question,omitempty"`
}

// CoverageView is one reader's coverage (A-5), by label: what it was
// assigned, what of it was left unread, and its gaps. Go counts; the
// page writes «2 de 5 resultados sin leer».
type CoverageView struct {
	Part          int      `json:"part"`
	Status        string   `json:"status"`
	Reason        string   `json:"reason,omitempty"`
	Sources       int      `json:"sources"`
	SourcesUnread []string `json:"sources_unread"`
	Hits          int      `json:"hits"`
	HitsUnread    []string `json:"hits_unread"`
	Gaps          []string `json:"gaps"`
}

// RepeatOffer is «Repetir con otra pregunta» with its estimated cost
// (fichas §2: «con el costo a la vista»), offered when the doubt was not
// reduced or only in part (flujos §2). It never runs on its own («No
// hacer»: repetir una ronda sin tu clic).
type RepeatOffer struct {
	EstimatedCostUSD float64 `json:"estimated_cost_usd"`
}

// digestView fills the card's digest: the principal's part when there
// is one, and the app's (B-63) always once the round digested.
func (r *Runner) digestView(ctx context.Context, projectID string, body RoundBody, labeler *graph.Labeler, v *RoundView) error {
	v.ReducedDoubt, v.DigestStatus = body.ReducedDoubt, body.DigestStatus
	v.Discarded, v.Unverified, v.Coverage = []FindingView{}, []FindingView{}, []CoverageView{}
	v.Unused, v.ReaderContradictions = []FindingView{}, []FindingView{}
	if body.DigestStatus == "" {
		return nil
	}
	sourceInfo := func(id string, fv *FindingView) {
		if id == "" {
			return
		}
		if l, err := labeler.ToLabel(id); err == nil {
			fv.Source = l
		}
		if src, err := r.db.GetObject(ctx, projectID, id); err == nil {
			fv.SourceTitle = graph.SourceTitle(src)
			fv.SourceURL = bodyStr(bodyOf(src), "url")
		}
	}
	numbered := map[[2]int]int{}
	for _, f := range body.DigestFindings {
		numbered[[2]int{f.Part, f.Finding}] = f.N
	}
	view := func(c FindingCheck) FindingView {
		fv := FindingView{N: numbered[[2]int{c.Part, c.Finding}], Part: c.Part, Finding: c.Finding,
			Verification: c.Verification, Reason: c.Reason, Access: c.Access}
		if c.Part == 0 {
			// The app's own: a reference the round asked for (B-59 d).
			if c.Finding >= 1 && c.Finding <= len(body.References) {
				fv.Reference = body.References[c.Finding-1].Text
			}
		} else if f, ok := readerFindingAt(body, c.Part, c.Finding); ok {
			fv.Kind, fv.Passage, fv.Note, fv.Hit = f.Kind, findingPassage(c, strings.TrimSpace(deref(f.Passage))), deref(f.Note), deref(f.Hit)
			if f.Reference != nil {
				fv.Reference = f.Reference.Title
			}
		}
		sourceInfo(c.SourceID, &fv)
		if fv.SourceURL == "" && fv.Hit != "" {
			for _, h := range body.Hits {
				if h.Label == fv.Hit {
					fv.SourceURL = h.URL
				}
			}
		}
		return fv
	}
	byNumber := map[int]FindingView{}
	for _, c := range body.Findings {
		fv := view(c)
		switch c.Verification {
		case VerificationDiscarded:
			v.Discarded = append(v.Discarded, fv)
		case VerificationUnverified:
			v.Unverified = append(v.Unverified, fv)
		}
		if fv.N > 0 {
			byNumber[fv.N] = fv
		}
	}
	// Flujos §2: no finding the principal received is hidden, and no
	// reader's contradiction, whatever its digest returned.
	used := map[int]bool{}
	if body.Digest != nil {
		for _, u := range body.Digest.Findings {
			used[u.Finding] = true
		}
	}
	for _, f := range body.DigestFindings {
		if !used[f.N] {
			if fv, ok := byNumber[f.N]; ok {
				v.Unused = append(v.Unused, fv)
			}
		}
	}
	readerOQ := map[int]string{}
	for _, c := range body.Conflicts {
		if c.FindingB == 0 && c.OpenQuestionID != "" {
			if l, err := labeler.ToLabel(c.OpenQuestionID); err == nil {
				readerOQ[c.FindingA] = l
			}
		}
	}
	for _, c := range body.Findings {
		if c.Part == 0 {
			continue
		}
		if fv := view(c); fv.Kind == "contradiction" {
			fv.OpenQuestion = readerOQ[fv.N]
			v.ReaderContradictions = append(v.ReaderContradictions, fv)
		}
	}
	for _, e := range body.CoverageInventory {
		cv := CoverageView{Part: e.Part, Status: e.Status, Reason: e.Reason, Sources: len(e.Sources),
			Hits: len(e.Hits), SourcesUnread: []string{}, HitsUnread: []string{}, Gaps: e.Gaps}
		read := map[string]bool{}
		for _, id := range append(append([]string{}, e.SourcesRead...), e.HitsRead...) {
			read[id] = true
		}
		for _, id := range e.Sources {
			if !read[id] {
				if l, err := labeler.ToLabel(id); err == nil {
					cv.SourcesUnread = append(cv.SourcesUnread, l)
				}
			}
		}
		for _, h := range e.Hits {
			if !read[h] {
				cv.HitsUnread = append(cv.HitsUnread, h)
			}
		}
		if cv.Gaps == nil {
			cv.Gaps = []string{}
		}
		v.Coverage = append(v.Coverage, cv)
	}
	if body.Digest != nil {
		d := DigestView{Answer: body.Digest.Answer, ExpectationContrast: deref(body.Digest.ExpectationContrast),
			Outline: []DigestLineView{}, Contradictions: []ContradictionViewItem{}}
		for i, text := range body.Digest.Outline {
			line := DigestLineView{Text: text, Findings: []FindingView{}}
			for _, u := range body.Digest.Findings {
				if fv, ok := byNumber[u.Finding]; ok && u.OutlineLine == i+1 {
					line.Findings = append(line.Findings, fv)
				}
			}
			d.Outline = append(d.Outline, line)
		}
		for _, c := range body.Conflicts {
			if c.FindingB == 0 {
				continue // a reader's contradiction: in ReaderContradictions
			}
			item := ContradictionViewItem{A: byNumber[c.FindingA], B: byNumber[c.FindingB], About: c.About,
				SameSource: c.SameSource}
			if c.OpenQuestionID != "" {
				if l, err := labeler.ToLabel(c.OpenQuestionID); err == nil {
					item.OpenQuestion = l
				}
			}
			d.Contradictions = append(d.Contradictions, item)
		}
		v.Digest = &d
	}
	if body.ScopeQuestionID != "" {
		if l, err := labeler.ToLabel(body.ScopeQuestionID); err == nil {
			v.ScopeQuestion = l
		}
	}
	if repeatable(body) {
		v.Repeat = &RepeatOffer{EstimatedCostUSD: EstimatedRoundCost(len(body.Parts))}
	}
	return nil
}

// repeatable: the round digested and its doubt was not reduced, or only
// in part (flujos §2: «no o en parte»).
func repeatable(body RoundBody) bool {
	return body.ReducedDoubt == "no" || body.ReducedDoubt == "partly"
}

// Errors of repeating a round or leaving it «por aclarar».
var (
	// ErrRoundNotRepeatable: the round is not closed with its doubt
	// unreduced (no or partly).
	ErrRoundNotRepeatable = errors.New("cycle: the round's doubt was reduced or it is not closed")
	// ErrQuestionRequired: «otra pregunta» is empty or the same one.
	ErrQuestionRequired = errors.New("cycle: another question is required")
	// ErrScopeAlreadyOpen: the round was already left «por aclarar».
	ErrScopeAlreadyOpen = errors.New("cycle: the round was already left to clarify")
)

// RepeatRound repeats a closed round whose doubt was not reduced with
// another question (flujos §2: «repetir con otra pregunta, con su costo
// (decides tú, cuando quieras)»): your click creates the new round open,
// with the same parts and reason and your question, and runs it in the
// background like «Buscar evidencia» (B-72), in the project's turn slot.
// It returns the new round's label, its turn and its estimated cost.
// [EL DISEÑO NO LO DICE]: who writes «otra pregunta»; you, in the
// request, since the digest's output has no field for one. The same
// question (as the app compares text) is refused.
func (r *Runner) RepeatRound(projectID, label, question string) (newLabel, turnID string, cost float64, err error) {
	question = strings.TrimSpace(question)
	newLabel, turnID, err = r.startRoundWith(projectID, map[string]any{"repeats": label},
		func(ctx context.Context) (RoundBody, error) {
			old, err := r.db.GetObject(ctx, projectID, label)
			if errors.Is(err, store.ErrNotFound) || (err == nil && old.Type != store.ObjectRound) {
				return RoundBody{}, ErrNotRound
			}
			if err != nil {
				return RoundBody{}, err
			}
			var body RoundBody
			if err := json.Unmarshal(old.Body, &body); err != nil {
				return RoundBody{}, fmt.Errorf("cycle: round body: %w", err)
			}
			if old.State != "closed" || !repeatable(body) {
				return RoundBody{}, ErrRoundNotRepeatable
			}
			if question == "" || graph.CompareString(question) == graph.CompareString(body.Question) {
				return RoundBody{}, ErrQuestionRequired
			}
			parts := make([]RoundPart, len(body.Parts))
			copy(parts, body.Parts)
			cost = EstimatedRoundCost(len(parts))
			return RoundBody{Kind: body.Kind, Question: question, Doubt: body.Doubt,
				DelegationReason: body.DelegationReason, Scope: body.Scope, Parts: parts,
				References: []namedReference{}, Repeats: old.ID, EstimatedCostUSD: cost,
				FromCandidate: body.FromCandidate}, nil
		})
	if err != nil {
		return "", "", 0, err
	}
	return newLabel, turnID, cost, nil
}

// LeaveRoundOpen is «dejarlo por aclarar» (flujos §2, estados §4: «lo
// que eliges dejar por aclarar cuando una ronda no reduce la duda»): it
// opens a scope «por aclarar» (B-68's options: complete the review with
// another round, with its cost, or declare the limit in the text)
// through the one creation path of 2.4, blocking the sections that use
// a source of the round's digest, and keeps it in the round, in one
// write conditioned on the round's version read. It returns the «por
// aclarar»'s label.
// [EL DISEÑO NO LO DICE]: its question is the round's doubt (its
// question if it has none), literal; the options are codes the page
// translates; what it blocks.
func (r *Runner) LeaveRoundOpen(ctx context.Context, projectID, label string) (string, error) {
	round, err := r.db.GetObject(ctx, projectID, label)
	if errors.Is(err, store.ErrNotFound) || (err == nil && round.Type != store.ObjectRound) {
		return "", ErrNotRound
	}
	if err != nil {
		return "", err
	}
	var body RoundBody
	if err := json.Unmarshal(round.Body, &body); err != nil {
		return "", fmt.Errorf("cycle: round body: %w", err)
	}
	if round.State != "closed" || !repeatable(body) {
		return "", ErrRoundNotRepeatable
	}
	if body.ScopeQuestionID != "" {
		return "", ErrScopeAlreadyOpen
	}
	question := body.Doubt
	if strings.TrimSpace(question) == "" {
		question = body.Question
	}
	var sources []string
	for _, f := range body.DigestFindings {
		if f.SourceID != "" {
			sources = append(sources, f.SourceID)
		}
	}
	var oqLabel string
	err = r.db.Write(ctx, projectID, func(tx *store.Tx) error {
		sections, err := checks.SectionsUsing(ctx, tx, projectID, sources)
		if err != nil {
			return err
		}
		extra := map[string]any{"origin": "round", "kind": OpenQuestionScope, "round_id": round.ID,
			"undeclared_by_agent": false}
		if body.FromCandidate && strings.TrimSpace(body.Doubt) == "" {
			extra["question_is_data"] = true // P-83: its question is a candidate's text
		}
		oq, err := checks.Open(ctx, tx, checks.OpenQuestion{Question: question,
			Options: checks.ScopeOptions(), DependsOn: sections, Extra: extra,
			Event: store.EventInput{Type: store.EventStateChanged, Actor: "human", TurnID: body.TurnID,
				Data: map[string]any{"via": "round", "kind": OpenQuestionScope, "round": round.Label}}})
		if err != nil {
			return err
		}
		oqLabel = oq.Label
		body.ScopeQuestionID = oq.ID
		_, err = tx.UpdateObject(round.ID, round.CurrentVersion, round.State, body, store.EventInput{
			Type: store.EventStateChanged, Actor: "human", TurnID: body.TurnID,
			Data: map[string]any{"to": round.State, "open_question": oq.Label},
		})
		return err
	})
	return oqLabel, err
}
