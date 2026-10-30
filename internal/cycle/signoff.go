package cycle

import (
	"bytes"
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/santhosh-tekuri/jsonschema/v6"

	"github.com/LaurenteEber/tupaqnin/internal/fingerprint"
	"github.com/LaurenteEber/tupaqnin/internal/graph"
	"github.com/LaurenteEber/tupaqnin/internal/provider"
	"github.com/LaurenteEber/tupaqnin/internal/store"
)

// The signoff (backlog-r1 1.7; flujos §3; fichas §2 «Firma»; B-13,
// B-14; specification §4). Go decides «coincide» from Wonder's marks;
// Wonder never signs or confirms anything.

//go:embed schema/wonder_output.schema.json
var wonderSchema []byte

//go:embed prompts/contraster.en.md
var contrasterInstructions string

// Errors of the signoff routes.
var (
	ErrSectionNotFound = errors.New("cycle: section not found")
	ErrSectionSigned   = errors.New("cycle: the section is already signed")
	// ErrSectionEmpty: a section without text is not offered for signing
	// (B-21).
	ErrSectionEmpty           = errors.New("cycle: the section has no text")
	ErrSignoffNotFound        = errors.New("cycle: signoff not found")
	ErrSignoffNotInProgress   = errors.New("cycle: the signoff is not in progress")
	ErrEmptyAnswer            = errors.New("cycle: empty explanation or answer")
	ErrExplanationGiven       = errors.New("cycle: the explanation was already contrasted")
	ErrNoRoundYet             = errors.New("cycle: Wonder has not contrasted the explanation yet")
	ErrCaseAnswerRequired     = errors.New("cycle: the new case needs your answer")
	ErrSignoffDoesNotCoincide = errors.New("cycle: the signoff does not coincide")
	ErrSignoffOutdated        = errors.New("cycle: the section or its critical decisions changed since the signoff opened")
	ErrBriefSignoff           = errors.New("cycle: a brief signoff has no Wonder round")
)

// PendingsError is a signing refused because the section has pendings
// (flujos §3: «No se puede firmar todavía»).
type PendingsError struct{ Pendings []graph.Pending }

func (e *PendingsError) Error() string { return "cycle: the section has pendings" }

// Outcomes of one Wonder round (flujos §3).
const (
	OutcomeCoincides     = "coincides"
	OutcomeCase          = "case"
	OutcomeOpen          = "open"
	OutcomePaused        = "paused"
	OutcomeMissingInText = "missing_in_text"
)

// StatusFailed is a Wonder round whose output missed its shape twice:
// visible, without stopping the work and without an agent pending
// (fichas §7).
// [EL DISEÑO NO LO DICE]: fichas §7 names what happens to a reader's
// and the principal's return; Wonder's is a turn error, and the
// signoff stays in progress.
const StatusFailed StepStatus = "failed"

// SignoffOpen is what asking to sign returns: the signoff in progress,
// or the pendings and open changes that keep it from opening.
type SignoffOpen struct {
	Opened      bool
	Label       string
	Brief       bool
	Pendings    []graph.Pending
	OpenChanges []string
}

// SignoffRoundResult is the outcome of EndSignoffRound.
type SignoffRoundResult struct {
	Status  StepStatus
	Errors  []string
	Outcome string
}

func (g *Gate) section(ctx context.Context, projectID, label string) (store.Object, error) {
	obj, err := g.db.GetObject(ctx, projectID, label)
	if err != nil || obj.Type != store.ObjectSection {
		return store.Object{}, ErrSectionNotFound
	}
	return obj, nil
}

// signoff loads a sign-n in progress with its body.
func (g *Gate) signoff(ctx context.Context, projectID, label string) (store.Object, graph.SignoffBody, error) {
	sign, err := g.db.GetObject(ctx, projectID, label)
	if err != nil || sign.Type != store.ObjectSignoff {
		return store.Object{}, graph.SignoffBody{}, ErrSignoffNotFound
	}
	body, err := graph.ParseSignoff(sign)
	if err != nil {
		return store.Object{}, graph.SignoffBody{}, err
	}
	phase, _, err := graph.SignoffPhaseOf(ctx, g.db, projectID, sign.ID)
	if err != nil {
		return store.Object{}, graph.SignoffBody{}, err
	}
	if phase != graph.SignoffInProgress {
		return sign, body, ErrSignoffNotInProgress
	}
	return sign, body, nil
}

// openChanges are the labels of the section's open changes (A-1): they
// are not pendings, and they also block the signoff (estados §1).
func openChanges(section store.Object) []string {
	var out []string
	changes, _ := bodyOf(section)["open_changes"].([]any)
	for _, c := range changes {
		if m, ok := c.(map[string]any); ok {
			if l, _ := m["label"].(string); l != "" {
				out = append(out, l)
			}
		}
	}
	return out
}

// RequestSignoff is 7a: only you ask to sign. With pendings or open
// changes it lists them and opens nothing; with a signoff in progress
// it returns that one; otherwise it opens a new sign-n, brief when the
// section has no critical decisions (A-4), with the origin question's
// assumption chosen in the graph (fichas §9).
func (g *Gate) RequestSignoff(ctx context.Context, projectID, sectionLabel string) (SignoffOpen, error) {
	section, err := g.section(ctx, projectID, sectionLabel)
	if err != nil {
		return SignoffOpen{}, err
	}
	if section.State == "signed" {
		return SignoffOpen{}, ErrSectionSigned
	}
	if strings.TrimSpace(bodyStr(bodyOf(section), "text")) == "" {
		return SignoffOpen{}, ErrSectionEmpty
	}
	_, pendings, err := graph.SectionStatus(ctx, g.db, projectID, sectionLabel)
	if err != nil {
		return SignoffOpen{}, err
	}
	changes := openChanges(section)
	// [EL DISEÑO NO LO DICE]: a stale section whose list carries no
	// label still blocks; it is reported with an empty list.
	if len(pendings) > 0 || len(changes) > 0 || section.State == "stale" {
		return SignoffOpen{Pendings: pendings, OpenChanges: changes}, nil
	}
	if active, ok, err := graph.ActiveSignoff(ctx, g.db, projectID, section.ID); err != nil {
		return SignoffOpen{}, err
	} else if ok {
		body, err := graph.ParseSignoff(active)
		if err != nil {
			return SignoffOpen{}, err
		}
		return SignoffOpen{Opened: true, Label: active.Label, Brief: body.Brief, Pendings: []graph.Pending{}}, nil
	}

	critical, err := graph.SectionCriticalDecisions(ctx, g.db, projectID, section.ID)
	if err != nil {
		return SignoffOpen{}, err
	}
	origin, err := graph.OriginAssumption(ctx, g.db, projectID, critical)
	if err != nil {
		return SignoffOpen{}, err
	}
	body := graph.SignoffBody{SectionID: section.ID, SectionVersion: section.CurrentVersion,
		Brief: len(critical) == 0, Critical: []graph.SignedRef{}, OriginAssumptionID: origin,
		Cases: []graph.SignoffCase{}, Rounds: []graph.SignoffRound{}}
	for _, d := range critical {
		body.Critical = append(body.Critical, graph.SignedRef{ID: d.ID, Version: d.CurrentVersion})
	}
	var sign store.Object
	err = g.db.Write(ctx, projectID, func(tx *store.Tx) error {
		var err error
		// [EL DISEÑO NO LO DICE]: the event that opens a sign-n; the
		// routes table lists state_changed, with signoff_step in its data.
		sign, err = tx.CreateObject(store.ObjectSignoff, "", body, store.EventInput{
			Type: store.EventStateChanged, Actor: "human",
			Data: map[string]any{"signoff_step": graph.StepRequested, "section": section.Label, "brief": body.Brief},
		})
		return err
	})
	if err != nil {
		return SignoffOpen{}, err
	}
	return SignoffOpen{Opened: true, Label: sign.Label, Brief: body.Brief, Pendings: []graph.Pending{}}, nil
}

// lastEventAt is when the signoff last asked you something: the time of
// its last event.
func (g *Gate) lastEventAt(ctx context.Context, projectID, objectID string) (time.Time, error) {
	events, err := g.db.EventsSince(ctx, projectID, 0)
	if err != nil {
		return time.Time{}, err
	}
	at := time.Now()
	for _, ev := range events {
		if ev.ObjectID == objectID {
			at = parseStoredTime(ev.CreatedAt)
		}
	}
	return at, nil
}

func actTimed(sign store.Object, part string, since time.Time) store.EventInput {
	return store.EventInput{Type: store.EventActTimed, Actor: "human", ObjectID: sign.ID,
		Data: map[string]any{"act": "signoff", "label": sign.Label, "part": part,
			"duration_ms": time.Since(since).Milliseconds()}}
}

// humanUpdate writes your input on the sign-n, conditioned on the
// version read, with act_timed for each part you answered (K-11).
// [EL DISEÑO NO LO DICE]: the event of your input on the sign-n;
// state_changed with signoff_step, like its opening.
func (g *Gate) humanUpdate(ctx context.Context, projectID string, sign store.Object, body graph.SignoffBody, step string, parts []string) error {
	since, err := g.lastEventAt(ctx, projectID, sign.ID)
	if err != nil {
		return err
	}
	return g.db.Write(ctx, projectID, func(tx *store.Tx) error {
		if _, err := tx.UpdateObject(sign.ID, sign.CurrentVersion, "", body, store.EventInput{
			Type: store.EventStateChanged, Actor: "human", Data: map[string]any{"signoff_step": step},
		}); err != nil {
			return err
		}
		for _, part := range parts {
			if _, err := tx.AppendEvent(actTimed(sign, part, since)); err != nil {
				return err
			}
		}
		return nil
	})
}

// RecordExplanation is 7b: your explanation without looking. In a full
// signoff it is contrasted once (B-13 (1)).
// [EL DISEÑO NO LO DICE]: whether it can be rewritten; only while no
// round has come back (a failed call), so it can be sent again.
func (g *Gate) RecordExplanation(ctx context.Context, projectID, signLabel, text string) error {
	sign, body, err := g.signoff(ctx, projectID, signLabel)
	if err != nil {
		return err
	}
	if strings.TrimSpace(text) == "" {
		return ErrEmptyAnswer
	}
	if len(body.Rounds) > 0 {
		return ErrExplanationGiven
	}
	body.Explanation = text
	return g.humanUpdate(ctx, projectID, sign, body, graph.StepExplanation, []string{"explanation"})
}

// RecordAnswers is 7e: your answers to the origin question and to the
// new case (B-13 (2)). The case answer is required while a case waits;
// an empty origin answer keeps the one you gave.
func (g *Gate) RecordAnswers(ctx context.Context, projectID, signLabel, originAnswer, caseAnswer string) error {
	sign, body, err := g.signoff(ctx, projectID, signLabel)
	if err != nil {
		return err
	}
	if body.Brief {
		return ErrBriefSignoff
	}
	if len(body.Rounds) == 0 {
		return ErrNoRoundYet
	}
	var parts []string
	i, open := body.OpenCase()
	if open && strings.TrimSpace(caseAnswer) == "" {
		return ErrCaseAnswerRequired
	}
	if strings.TrimSpace(originAnswer) != "" {
		body.OriginAnswer = originAnswer
		parts = append(parts, "origin")
	}
	if open {
		body.Cases[i].Answer = caseAnswer
		parts = append(parts, "case")
	}
	if len(parts) == 0 {
		return ErrEmptyAnswer
	}
	return g.humanUpdate(ctx, projectID, sign, body, graph.StepAnswers, parts)
}

// CancelSignoff ends a signoff in progress inside tx with
// signoff_cancelled and its reason (B-14): left, edited or
// missing_in_text. data adds to the event.
func CancelSignoff(tx *store.Tx, sign store.Object, reason, actor string, data map[string]any) error {
	body, err := graph.ParseSignoff(sign)
	if err != nil {
		return err
	}
	body.Ended = reason
	if data == nil {
		data = map[string]any{}
	}
	data["reason"] = reason
	_, err = tx.UpdateObject(sign.ID, sign.CurrentVersion, "", body,
		store.EventInput{Type: store.EventSignoffCancelled, Actor: actor, Data: data})
	return err
}

// LeaveSignoff is 7g: leaving it for later cancels it (B-14, left).
func (g *Gate) LeaveSignoff(ctx context.Context, projectID, signLabel string) error {
	sign, _, err := g.signoff(ctx, projectID, signLabel)
	if err != nil {
		return err
	}
	return g.db.Write(ctx, projectID, func(tx *store.Tx) error {
		return CancelSignoff(tx, sign, graph.CancelLeft, "human", nil)
	})
}

// Sign is the signing (fichas §2, specification §4): only when it
// coincides (or, brief, with your explanation), without pendings, on the
// version that was explained. It stores the commitment, creates an
// explains edge from the signoff to each critical decision and leaves
// the section signed, all in one transaction.
func (g *Gate) Sign(ctx context.Context, projectID, signLabel string) error {
	sign, body, err := g.signoff(ctx, projectID, signLabel)
	if err != nil {
		return err
	}
	if body.Brief && strings.TrimSpace(body.Explanation) == "" || !body.Brief && !body.Coincides {
		return ErrSignoffDoesNotCoincide
	}
	section, err := g.db.GetObject(ctx, projectID, body.SectionID)
	if err != nil {
		return err
	}
	if section.CurrentVersion != body.SectionVersion || section.State == "stale" {
		return ErrSignoffOutdated
	}
	if _, pendings, err := graph.SectionStatus(ctx, g.db, projectID, section.Label); err != nil {
		return err
	} else if len(pendings) > 0 {
		return &PendingsError{Pendings: pendings}
	}
	critical, err := graph.SectionCriticalDecisions(ctx, g.db, projectID, section.ID)
	if err != nil {
		return err
	}
	if len(critical) != len(body.Critical) {
		return ErrSignoffOutdated
	}
	for i, d := range critical {
		if body.Critical[i].ID != d.ID {
			return ErrSignoffOutdated
		}
	}
	opened, err := g.signoffOpenedAt(ctx, projectID, sign.ID)
	if err != nil {
		return err
	}

	return g.db.Write(ctx, projectID, func(tx *store.Tx) error {
		return TrackCriticality(ctx, tx, "", func() error {
			body.SectionFingerprint = fingerprint.Section(bodyStr(bodyOf(section), "text"))
			body.Criteria, body.Sources, body.AnsweredCards = []graph.SignedRef{}, []graph.SignedSource{}, []string{}
			critIDs := map[string]bool{}
			for i, d := range critical {
				body.Critical[i].Version = d.CurrentVersion
				critIDs[d.ID] = true
				edges, err := tx.EdgesFrom(ctx, projectID, d.ID)
				if err != nil {
					return err
				}
				for _, e := range edges {
					if e.Type != store.EdgeApplies {
						continue
					}
					crit, err := tx.Object(e.ToID)
					if err != nil {
						return err
					}
					body.Criteria = append(body.Criteria, graph.SignedRef{ID: crit.ID, Version: crit.CurrentVersion})
				}
			}
			uses, err := tx.EdgesFrom(ctx, projectID, section.ID)
			if err != nil {
				return err
			}
			for _, e := range uses {
				if e.Type != store.EdgeUses {
					continue
				}
				src, err := tx.Object(e.ToID)
				if err != nil {
					return err
				}
				body.Sources = append(body.Sources, graph.SignedSource{ID: src.ID, Fingerprint: bodyStr(bodyOf(src), "fingerprint")})
			}
			cards, err := tx.ListObjects(ctx, projectID, store.ObjectCard)
			if err != nil {
				return err
			}
			for _, c := range cards {
				if c.State == "answered" && critIDs[bodyStr(bodyOf(c), "decision_id")] {
					body.AnsweredCards = append(body.AnsweredCards, c.ID)
				}
			}
			body.Ended = graph.StepSigned
			ev := store.EventInput{Type: store.EventStateChanged, Actor: "human",
				Data: map[string]any{"signoff_step": graph.StepSigned, "section": section.Label}}
			if body.Brief {
				// A-4: the history records «pregunta de origen: no aplica».
				ev.Type = store.EventBriefSignoff
				ev.Data = map[string]any{"signoff_step": graph.StepSigned, "section": section.Label,
					"origin_question": "not_applicable"}
			}
			if _, err := tx.UpdateObject(sign.ID, sign.CurrentVersion, "", body, ev); err != nil {
				return err
			}
			for _, d := range critical {
				if _, err := tx.AddEdge(store.EdgeExplains, sign.ID, d.ID, "",
					store.EventInput{Type: store.EventEdge, Actor: "app"}); err != nil {
					return err
				}
			}
			secBody := bodyOf(section)
			if _, err := tx.UpdateObject(section.ID, section.CurrentVersion, "signed", secBody, store.EventInput{
				Type: store.EventStateChanged, Actor: "human",
				Data: map[string]any{"label": section.Label, "from": section.State, "to": "signed", "signoff": sign.Label},
			}); err != nil {
				if errors.Is(err, store.ErrVersionConflict) {
					return ErrSignoffOutdated
				}
				return err
			}
			_, err = tx.AppendEvent(actTimed(sign, "signed", opened))
			return err
		})
	})
}

func (g *Gate) signoffOpenedAt(ctx context.Context, projectID, signID string) (time.Time, error) {
	events, err := g.db.EventsSince(ctx, projectID, 0)
	if err != nil {
		return time.Time{}, err
	}
	for _, ev := range events {
		if ev.ObjectID == signID {
			return parseStoredTime(ev.CreatedAt), nil
		}
	}
	return time.Now(), nil
}

// EndSignoffRound validates one Wonder output (salida-wonder.schema.json
// and the Go checks: rule 4 of specification §3 rejects a new-n in
// decision here too) and applies the round in one transaction
// conditioned on the version read (B-13, B-14). Go decides «coincide»:
// every decision match, the origin question answers and, if there was a
// case, the last one applies_well. missing_in_text cancels the signoff
// with its passages; 2 cases that do not apply pause it (E-2).
func (g *Gate) EndSignoffRound(ctx context.Context, projectID, turnID, signLabel string, attempt int,
	output json.RawMessage, contrastedBy string, wonderUnavailable bool) (SignoffRoundResult, error) {
	sign, body, err := g.signoff(ctx, projectID, signLabel)
	if err != nil {
		return SignoffRoundResult{}, err
	}
	section, err := g.db.GetObject(ctx, projectID, body.SectionID)
	if err != nil {
		return SignoffRoundResult{}, err
	}
	errs, out, err := validateWonder(output)
	if err != nil {
		return SignoffRoundResult{}, err
	}
	if len(errs) == 0 {
		errs, err = g.checkWonder(ctx, projectID, out, body, section)
		if err != nil {
			return SignoffRoundResult{}, err
		}
	}
	if len(errs) > 0 {
		status := StatusRetry
		data := map[string]any{"attempt": attempt, "errors": errs, "turn_kind": "signoff", "signoff": sign.Label}
		if attempt >= 2 {
			status = StatusFailed
			data["result"] = "failed"
		}
		if err := g.append(ctx, projectID, store.EventInput{
			Type: store.EventOutputRetry, Actor: "agent", AgentRole: string(provider.RoleContraster), TurnID: turnID,
			ModelAndEffort: contrastedBy, ObjectID: sign.ID, Data: data,
		}); err != nil {
			return SignoffRoundResult{}, err
		}
		return SignoffRoundResult{Status: status, Errors: errs}, nil
	}

	allMatch := true
	var passages []string
	for _, d := range out.Decisions {
		if d.Verdict != "match" {
			allMatch = false
		}
		if d.Verdict == "missing_in_text" {
			passages = append(passages, *d.Passage)
		}
	}
	var previous any
	if i, judged := pendingCase(body); judged {
		body.Cases[i].Result = *out.PreviousCase
		previous = *out.PreviousCase
		if *out.PreviousCase == "does_not_apply" {
			body.FailedCases++
		}
	}
	lastCaseWell := true
	for i := len(body.Cases) - 1; i >= 0; i-- {
		if body.Cases[i].Result != "" {
			lastCaseWell = body.Cases[i].Result == "applies_well"
			break
		}
	}
	answers := out.OriginQuestion.Answer != nil && *out.OriginQuestion.Answer == "answers"
	coincides := allMatch && answers && lastCaseWell
	outcome := OutcomeOpen
	switch {
	case len(passages) > 0:
		outcome, coincides = OutcomeMissingInText, false
	case body.FailedCases >= graph.MaxFailedCases:
		outcome, coincides = OutcomePaused, false
	case coincides:
		outcome = OutcomeCoincides
	case out.NewCase != nil:
		outcome = OutcomeCase
		body.Cases = append(body.Cases, graph.SignoffCase{Case: *out.NewCase})
	}
	body.OriginQuestion = out.OriginQuestion.Question
	body.Coincides = coincides
	body.ContrastedBy = contrastedBy
	if wonderUnavailable {
		body.WonderUnavailable = true
	}
	clean, err := json.Marshal(out)
	if err != nil {
		return SignoffRoundResult{}, err
	}
	body.Rounds = append(body.Rounds, graph.SignoffRound{ContrastedBy: contrastedBy,
		WonderUnavailable: wonderUnavailable, Output: clean, Coincides: coincides})

	err = g.db.Write(ctx, projectID, func(tx *store.Tx) error {
		wonder := func(t store.EventType, data map[string]any) store.EventInput {
			return store.EventInput{Type: t, Actor: "agent", AgentRole: string(provider.RoleContraster),
				ModelAndEffort: contrastedBy, TurnID: turnID, Data: data}
		}
		// [EL DISEÑO NO LO DICE]: each round is a signoff_case (the case
		// result, when there was a case), with the round's outcome.
		updated, err := tx.UpdateObject(sign.ID, sign.CurrentVersion, "", body, wonder(store.EventSignoffCase, map[string]any{
			"round": len(body.Rounds), "previous_case": previous, "coincides": coincides, "outcome": outcome,
			"wonder_unavailable": wonderUnavailable,
		}))
		if err != nil {
			return err
		}
		switch outcome {
		case OutcomePaused:
			body.Ended = "paused"
			_, err = tx.UpdateObject(sign.ID, updated.CurrentVersion, "", body, wonder(store.EventSignoffPaused,
				map[string]any{"failed_cases": body.FailedCases, "section": section.Label}))
		case OutcomeMissingInText:
			err = CancelSignoff(tx, updated, graph.CancelMissingInText, "agent", map[string]any{
				"section_id": section.ID, "section": section.Label, "passages": passages,
				"fingerprint": fingerprint.Section(bodyStr(bodyOf(section), "text")),
			})
		}
		return err
	})
	if errors.Is(err, store.ErrVersionConflict) {
		return SignoffRoundResult{}, ErrSignoffNotInProgress
	}
	if err != nil {
		return SignoffRoundResult{}, err
	}
	return SignoffRoundResult{Status: StatusAccepted, Outcome: outcome}, nil
}

// pendingCase is the last case, answered and not judged yet.
func pendingCase(body graph.SignoffBody) (int, bool) {
	n := len(body.Cases)
	if n == 0 || body.Cases[n-1].Answer == "" || body.Cases[n-1].Result != "" {
		return 0, false
	}
	return n - 1, true
}

// checkWonder applies the Go checks beyond the schema: rule 4 (no new-n
// and no unknown label in decision), one entry per critical decision,
// what is missing named and the passage literal in the section, and the
// judgments only of what you answered.
// [EL DISEÑO NO LO DICE]: beyond rule 4, the schema's descriptions
// (missing and passage by verdict, null while unanswered) are checked in
// Go, and a passage not literal in the section is rejected.
func (g *Gate) checkWonder(ctx context.Context, projectID string, out graph.WonderOutput, body graph.SignoffBody, section store.Object) ([]string, error) {
	labels, err := graph.NewLabeler(ctx, g.db, projectID)
	if err != nil {
		return nil, err
	}
	critical := map[string]bool{}
	var order []string
	for _, d := range body.Critical {
		l, err := labels.ToLabel(d.ID)
		if err != nil {
			return nil, err
		}
		critical[l] = true
		order = append(order, l)
	}
	var errs []string
	fail := func(path, format string, args ...any) { errs = append(errs, path+": "+fmt.Sprintf(format, args...)) }
	seen := map[string]bool{}
	text := graph.Compare(bodyStr(bodyOf(section), "text"))
	for i, d := range out.Decisions {
		path := fmt.Sprintf("/decisions/%d", i)
		switch {
		case isNew(d.Decision):
			fail(path+"/decision", "%s is a new-n; name each critical decision by its label as given", d.Decision)
		case !critical[d.Decision]:
			fail(path+"/decision", "%s is not a critical decision of this section", d.Decision)
		case seen[d.Decision]:
			fail(path+"/decision", "%s appears more than once", d.Decision)
		}
		seen[d.Decision] = true
		if d.Verdict == "missing_in_explanation" && (d.Missing == nil || strings.TrimSpace(*d.Missing) == "") {
			fail(path+"/missing", "name what is missing, without the answer")
		}
		if d.Verdict == "missing_in_text" {
			if d.Passage == nil {
				fail(path+"/passage", "quote the passage of the section where it is missing")
			} else if len(text.Find(*d.Passage)) == 0 {
				fail(path+"/passage", "the passage does not appear literally in the section")
			}
		}
	}
	for _, l := range order {
		if !seen[l] {
			fail("/decisions", "missing an entry for %s", l)
		}
	}
	answer := out.OriginQuestion.Answer
	if body.OriginAnswer == "" && answer != nil {
		fail("/origin_question/answer", "must be null: the person has not answered the origin question")
	}
	if body.OriginAnswer != "" && answer == nil {
		fail("/origin_question/answer", "judge the person's answer to the origin question")
	}
	_, judged := pendingCase(body)
	if judged && out.PreviousCase == nil {
		fail("/previous_case", "judge the person's answer to the last case")
	}
	if !judged && out.PreviousCase != nil {
		fail("/previous_case", "must be null: there is no answered case to judge")
	}
	return errs, nil
}

func validateWonder(output json.RawMessage) ([]string, graph.WonderOutput, error) {
	var parsed graph.WonderOutput
	inst, err := jsonschema.UnmarshalJSON(bytes.NewReader(output))
	if err != nil {
		return []string{"/: output is not valid JSON"}, parsed, nil
	}
	sch, err := compiledWonderSchema()
	if err != nil {
		return nil, parsed, err
	}
	if err := sch.Validate(inst); err != nil {
		var verr *jsonschema.ValidationError
		if !errors.As(err, &verr) {
			return nil, parsed, fmt.Errorf("cycle: validate Wonder output: %w", err)
		}
		return flattenErrors(verr), parsed, nil
	}
	if err := json.Unmarshal(output, &parsed); err != nil {
		return nil, parsed, fmt.Errorf("cycle: parse Wonder output: %w", err)
	}
	return nil, parsed, nil
}

var (
	wonderSchemaOnce sync.Once
	wonderCompiled   *jsonschema.Schema
	wonderErr        error
)

func compiledWonderSchema() (*jsonschema.Schema, error) {
	wonderSchemaOnce.Do(func() {
		doc, err := jsonschema.UnmarshalJSON(bytes.NewReader(wonderSchema))
		if err != nil {
			wonderErr = fmt.Errorf("cycle: parse Wonder schema: %w", err)
			return
		}
		c := jsonschema.NewCompiler()
		const loc = "https://tupaqnin.invalid/wonder_output.schema.json"
		if err := c.AddResource(loc, doc); err != nil {
			wonderErr = fmt.Errorf("cycle: add Wonder schema: %w", err)
			return
		}
		wonderCompiled, wonderErr = c.Compile(loc)
	})
	return wonderCompiled, wonderErr
}

// The signoff's Wonder turns take their own slot, so a signoff does not
// wait for the agent's chain (and the other way round).
// [EL DISEÑO NO LO DICE]: one Wonder turn per project at a time, apart
// from the principal's turn.
func signoffSlot(projectID string) string { return projectID + "#signoff" }

// SignoffExplanation records your explanation and, in a full signoff,
// starts Wonder's first call in the background (B-13 (1)), returning
// its turn id. A brief signoff calls no model and returns no turn
// (A-4).
func (r *Runner) SignoffExplanation(projectID, signLabel, text string) (string, error) {
	return r.startSignoffRound(projectID, signLabel, func(ctx context.Context) error {
		return r.gate.RecordExplanation(ctx, projectID, signLabel, text)
	})
}

// SignoffAnswers records your answers and starts Wonder's next call
// (B-13 (2)).
func (r *Runner) SignoffAnswers(projectID, signLabel, originAnswer, caseAnswer string) (string, error) {
	return r.startSignoffRound(projectID, signLabel, func(ctx context.Context) error {
		return r.gate.RecordAnswers(ctx, projectID, signLabel, originAnswer, caseAnswer)
	})
}

func (r *Runner) startSignoffRound(projectID, signLabel string, record func(context.Context) error) (string, error) {
	slot := signoffSlot(projectID)
	if !r.tryAcquireTurn(slot) {
		return "", ErrTurnRunning
	}
	ctx := context.Background()
	if err := record(ctx); err != nil {
		r.releaseTurn(slot)
		return "", err
	}
	_, body, err := r.gate.signoff(ctx, projectID, signLabel)
	if err != nil || body.Brief {
		r.releaseTurn(slot)
		return "", err
	}
	turnID, err := newTurnID()
	if err != nil {
		r.releaseTurn(slot)
		return "", err
	}
	if r.pub != nil {
		r.pub.BeginTurn(projectID, turnID)
	}
	go func() {
		defer r.releaseTurn(slot)
		if err := r.runSignoffRound(context.Background(), projectID, turnID, signLabel); err != nil && r.Logger != nil {
			r.Logger.Error("signoff turn failed", "error", err.Error())
		}
	}()
	return turnID, nil
}

// RunSignoffRound runs one Wonder round synchronously (tests).
func (r *Runner) RunSignoffRound(ctx context.Context, projectID, turnID, signLabel string) error {
	slot := signoffSlot(projectID)
	if !r.tryAcquireTurn(slot) {
		return ErrTurnRunning
	}
	defer r.releaseTurn(slot)
	return r.runSignoffRound(ctx, projectID, turnID, signLabel)
}

// runSignoffRound is one call of B-13 with its brief (graph.WonderBrief,
// labels only), one retry of the shape (fichas §7), and the round.
func (r *Runner) runSignoffRound(ctx context.Context, projectID, turnID, signLabel string) error {
	sign, _, err := r.gate.signoff(ctx, projectID, signLabel)
	if err != nil {
		r.fail(projectID, turnID, "turn_failed")
		return err
	}
	brief, err := graph.WonderBrief(ctx, r.db, projectID, sign.ID)
	if err != nil {
		r.fail(projectID, turnID, "turn_failed")
		return err
	}
	input := []json.RawMessage{userItem(brief)}
	filter := newFieldFilter(nil) // Wonder's JSON is read on the signoff screen, not streamed
	resp, model, effort, unavailable, err := r.callContraster(ctx, projectID, turnID, input, filter)
	if err != nil {
		r.fail(projectID, turnID, "provider_error")
		return nil
	}
	contrastedBy := model + " " + effort
	result, err := r.gate.EndSignoffRound(ctx, projectID, turnID, signLabel, 1, json.RawMessage(resp.Text), contrastedBy, unavailable)
	if err != nil {
		r.fail(projectID, turnID, "turn_failed")
		return err
	}
	if result.Status == StatusRetry {
		retry := append(append(append([]json.RawMessage{}, input...), resp.RawItems...), userItem(RetryMessage(result.Errors)))
		resp, err = r.contrast(ctx, projectID, turnID, model, effort, retry, filter)
		if err != nil {
			r.fail(projectID, turnID, "provider_error")
			return nil
		}
		result, err = r.gate.EndSignoffRound(ctx, projectID, turnID, signLabel, 2, json.RawMessage(resp.Text), contrastedBy, unavailable)
		if err != nil {
			r.fail(projectID, turnID, "turn_failed")
			return err
		}
	}
	if result.Status != StatusAccepted {
		r.fail(projectID, turnID, "turn_failed")
		return nil
	}
	if r.pub != nil {
		raw, _ := json.Marshal(map[string]any{"status": result.Status, "signoff": signLabel, "outcome": result.Outcome})
		r.pub.PublishStep(projectID, turnID, raw)
	}
	return nil
}

func (r *Runner) contrast(ctx context.Context, projectID, turnID, model, effort string, input []json.RawMessage, f *fieldFilter) (provider.Response, error) {
	schema, err := provider.TransportSchema(wonderSchema)
	if err != nil {
		return provider.Response{}, err
	}
	return r.prov.Respond(ctx, provider.Request{
		ProjectID: projectID, TurnID: turnID,
		Role: provider.RoleContraster, TurnKind: provider.TurnSignoff, Model: model, Effort: effort,
		Instructions: contrasterInstructions, Input: input, Schema: schema, SchemaStrict: true,
	}, r.filtered(projectID, turnID, f))
}

// callContraster calls Wonder 1.0 high, with one retry when it is not
// available; when it still is not (or is not in models.go), Spark 1.3
// xhigh contrasts and the round says so (flujos §3, B-20 (c)). Any other
// error is an error of the turn.
func (r *Runner) callContraster(ctx context.Context, projectID, turnID string, input []json.RawMessage, f *fieldFilter) (provider.Response, string, string, bool, error) {
	if provider.Known(provider.ContrasterModel) {
		for try := 0; try < 2; try++ {
			resp, err := r.contrast(ctx, projectID, turnID, provider.ContrasterModel, provider.ContrasterEffort, input, f)
			if err == nil {
				return resp, provider.ContrasterModel, provider.ContrasterEffort, false, nil
			}
			if !provider.Unavailable(err) {
				return provider.Response{}, "", "", false, err
			}
		}
	}
	resp, err := r.contrast(ctx, projectID, turnID, provider.ContrasterFallbackModel, provider.ContrasterFallbackEffort, input, f)
	return resp, provider.ContrasterFallbackModel, provider.ContrasterFallbackEffort, true, err
}

// SignoffAsk is «Preguntar al agente» from the signoff (B-14
// precision): a principal turn that asks for an answer without new
// text. Its answer is streamed and nothing of it is applied.
// [EL DISEÑO NO LO DICE]: the agent's answer is not a step: no EndStep
// and no ApplyStep run, so it writes nothing and closes no pending.
func (r *Runner) SignoffAsk(projectID, signLabel, text string) (string, error) {
	ctx := context.Background()
	sign, body, err := r.gate.signoff(ctx, projectID, signLabel)
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(text) == "" {
		return "", ErrEmptyAnswer
	}
	section, err := r.db.GetObject(ctx, projectID, body.SectionID)
	if err != nil {
		return "", err
	}
	if !r.tryAcquireTurn(projectID) {
		return "", ErrTurnRunning
	}
	turnID, err := newTurnID()
	if err != nil {
		r.releaseTurn(projectID)
		return "", err
	}
	if r.pub != nil {
		r.pub.BeginTurn(projectID, turnID)
	}
	go func() {
		defer r.releaseTurn(projectID)
		if err := r.runSignoffAsk(context.Background(), projectID, turnID, sign.Label, section.Label, text); err != nil && r.Logger != nil {
			r.Logger.Error("signoff ask failed", "error", err.Error())
		}
	}()
	return turnID, nil
}

// RunSignoffAsk runs «Preguntar al agente» synchronously (tests).
func (r *Runner) RunSignoffAsk(ctx context.Context, projectID, turnID, signLabel, text string) error {
	_, body, err := r.gate.signoff(ctx, projectID, signLabel)
	if err != nil {
		return err
	}
	section, err := r.db.GetObject(ctx, projectID, body.SectionID)
	if err != nil {
		return err
	}
	if !r.tryAcquireTurn(projectID) {
		return ErrTurnRunning
	}
	defer r.releaseTurn(projectID)
	return r.runSignoffAsk(ctx, projectID, turnID, signLabel, section.Label, text)
}

// SignoffAskMessage is what the principal receives from «Preguntar al
// agente»: your question in literal, and that it explains, not edits.
func SignoffAskMessage(sectionLabel, text string) string {
	return "The person is signing " + sectionLabel + " and asks you about it. Explain; do not edit: " +
		"return no new_text and no new decisions, and answer in note.\n\n" + text
}

// recordAsk writes your question as human_message with the agent's
// answer, not applied (Eber, 29-oct; spec §1.3: «tu mensaje en literal
// y, después, su respuesta»).
// [EL DISEÑO NO LO DICE]: history only grows, so the event is written
// once the answer is back, with both; a failed call leaves the question
// alone, unanswered.
func (r *Runner) recordAsk(ctx context.Context, projectID, turnID, signLabel, text, answer string) error {
	data := map[string]any{"text": text, "signoff": signLabel, "applied": false}
	if answer != "" {
		data["response"] = answer
	}
	return r.db.Write(ctx, projectID, func(tx *store.Tx) error {
		_, err := tx.AppendEvent(store.EventInput{Type: store.EventHumanMessage, Actor: "human", TurnID: turnID, Data: data})
		return err
	})
}

func (r *Runner) runSignoffAsk(ctx context.Context, projectID, turnID, signLabel, sectionLabel, text string) error {
	instanceID, startupInput, started, err := r.ensureInstance(ctx, projectID)
	if err != nil {
		r.fail(projectID, turnID, "turn_failed")
		return err
	}
	message := userItem(SignoffAskMessage(sectionLabel, text))
	var input []json.RawMessage
	if started {
		input = append(startupInput, message)
	} else {
		conversation, err := r.conversation(ctx, projectID, instanceID)
		if err != nil {
			r.fail(projectID, turnID, "turn_failed")
			return err
		}
		changes, err := r.changeItems(ctx, projectID, instanceID)
		if err != nil {
			r.fail(projectID, turnID, "turn_failed")
			return err
		}
		input = append(append(conversation, changes...), message)
	}
	resp, err := r.call(ctx, projectID, turnID, instanceID, input, newFieldFilter(stepStreamKeys))
	if err != nil {
		if werr := r.recordAsk(ctx, projectID, turnID, signLabel, text, ""); werr != nil {
			return werr
		}
		r.fail(projectID, turnID, "provider_error")
		return nil
	}
	var answer struct {
		Note string `json:"note"`
	}
	_ = json.Unmarshal([]byte(resp.Text), &answer)
	if err := r.recordAsk(ctx, projectID, turnID, signLabel, text, answer.Note); err != nil {
		r.fail(projectID, turnID, "turn_failed")
		return err
	}
	r.publishStep(projectID, turnID, StepResult{Status: StatusAccepted}, resp.Text)
	return nil
}
