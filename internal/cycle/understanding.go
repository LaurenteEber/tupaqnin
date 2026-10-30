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

	"github.com/santhosh-tekuri/jsonschema/v6"

	"github.com/LaurenteEber/tupaqnin/internal/store"
)

//go:embed schema/understanding_output.schema.json
var understandingSchema []byte

//go:embed prompts/understanding.en.md
var understandingInstructions string

// CriterionRef is one criterion of the request, by label and
// internal id (the id never reaches the model, rule 7).
type CriterionRef struct {
	Label string
	ID    string
}

// UnderstandingCriterion is one entry of the fixed understanding
// output: how a criterion will be applied, with its two cases.
type UnderstandingCriterion struct {
	Criterion    string `json:"criterion"`
	Counts       string `json:"counts"`
	DoesNotCount string `json:"does_not_count"`
}

// UnderstandingSection is one proposed section of the output.
type UnderstandingSection struct {
	ID    string `json:"id"`
	Title string `json:"title"`
	Role  string `json:"role"`
}

// UnderstandingOutput is the principal's fixed understanding shape
// (fichas §7, salida-entendi.schema.json).
type UnderstandingOutput struct {
	Understood string `json:"understood"`
	// ProjectTitle is the title proposed for the project (U-1).
	ProjectTitle string                   `json:"project_title"`
	Criteria     []UnderstandingCriterion `json:"criteria"`
	Material     []struct {
		Case string `json:"case"`
	} `json:"material_readings"`
	Sections []UnderstandingSection `json:"sections"`
}

// UnderstandingResult is the outcome of one EndUnderstanding call:
// accepted, retry or agent_pending, like EndStep (fichas §7).
type UnderstandingResult struct {
	Status StepStatus
	Errors []string
	Output UnderstandingOutput
}

// EndUnderstanding validates one understanding output and, when it
// matches, applies it in a single transaction: two unconfirmed
// cases per criterion, the material readings as cases without a
// criterion, and the proposed sections in draft with their role
// proposed (backlog 1.2). Rule 4 of specification §3 also applies
// here: a new-n in criterion is rejected in Go, as is a criterion
// that is missing, repeated, or not of this request.
// [EL DISEÑO NO LO DICE] the end of the understanding has no event
// type of its own in table 1.3, so it is recorded as step_ended
// with turn_kind "understanding" and the understood text, and the
// material cases carry no counts field.
func (g *Gate) EndUnderstanding(ctx context.Context, projectID, turnID string, attempt int, output json.RawMessage, criteria []CriterionRef) (UnderstandingResult, error) {
	if attempt < 1 {
		return UnderstandingResult{}, fmt.Errorf("cycle: attempt must be 1 or more, got %d", attempt)
	}
	errs, parsed, err := validateUnderstanding(output)
	if err != nil {
		return UnderstandingResult{}, err
	}
	if len(errs) == 0 {
		errs = checkUnderstanding(parsed, criteria, g, ctx, projectID)
	}
	if len(errs) > 0 {
		status := StatusRetry
		data := map[string]any{"attempt": attempt, "errors": errs, "turn_kind": "understanding"}
		if attempt >= 2 {
			status = StatusAgentPending
			data["result"] = "agent_pending"
		}
		if err := g.append(ctx, projectID, store.EventInput{
			Type: store.EventOutputRetry, Actor: "agent", AgentRole: "principal", TurnID: turnID,
			Data: data,
		}); err != nil {
			return UnderstandingResult{}, err
		}
		return UnderstandingResult{Status: status, Errors: errs}, nil
	}

	err = g.db.Write(ctx, projectID, func(tx *store.Tx) error {
		// Proposing a role can make a decision critical (T-5).
		return TrackCriticality(ctx, tx, turnID, func() error {
			var rolesNotApplied []any
			ev := store.EventInput{Type: store.EventStateChanged, Actor: "agent", AgentRole: "principal", TurnID: turnID}
			byLabel := make(map[string]CriterionRef, len(criteria))
			for _, c := range criteria {
				byLabel[c.Label] = c
			}
			for _, entry := range parsed.Criteria {
				crit := byLabel[entry.Criterion]
				for _, kase := range []struct {
					text   string
					counts bool
				}{{entry.Counts, true}, {entry.DoesNotCount, false}} {
					if _, err := tx.CreateObject(store.ObjectUnderstandingCase, "unconfirmed",
						map[string]any{"text": kase.text, "counts": kase.counts, "criterion_id": crit.ID}, ev); err != nil {
						return err
					}
				}
			}
			for _, reading := range parsed.Material {
				if _, err := tx.CreateObject(store.ObjectUnderstandingCase, "unconfirmed",
					map[string]any{"text": reading.Case, "criterion_id": ""}, ev); err != nil {
					return err
				}
			}
			for _, sec := range parsed.Sections {
				if strings.HasPrefix(sec.ID, "new-") {
					if _, err := tx.CreateObject(store.ObjectSection, "draft",
						map[string]any{
							"title": sec.Title, "text": "",
							"role":         map[string]any{"value": sec.Role, "state": "proposed"},
							"open_changes": []any{},
						}, ev); err != nil {
						return err
					}
					continue
				}
				obj, err := g.db.GetObject(ctx, projectID, sec.ID)
				if err != nil {
					return err
				}
				applied, err := proposeRole(tx, sec.ID, obj.ID, sec.Role, turnID)
				if err != nil {
					return err
				}
				if !applied { // P-32: a confirmed role stays
					rolesNotApplied = append(rolesNotApplied, map[string]any{"section_id": sec.ID, "role": sec.Role})
				}
			}
			// U-1: the proposed title names the project unless the
			// person already wrote a name.
			if title := strings.TrimSpace(parsed.ProjectTitle); title != "" {
				if _, err := tx.ProposeProjectName(title); err != nil {
					return err
				}
			}
			data := map[string]any{"turn_kind": "understanding", "understood": parsed.Understood}
			if rolesNotApplied != nil {
				data["not_applied"] = map[string]any{"proposed_roles": rolesNotApplied}
			}
			_, err := tx.AppendEvent(store.EventInput{
				Type: store.EventStepEnded, Actor: "agent", AgentRole: "principal", TurnID: turnID,
				Data: data,
			})
			return err
		})
	})
	if err != nil {
		return UnderstandingResult{}, err
	}
	return UnderstandingResult{Status: StatusAccepted, Output: parsed}, nil
}

// checkUnderstanding applies the Go checks beyond the schema:
// rule 4 (no new-n and no unknown label in criterion), one entry
// per criterion of the request, and known or new section ids.
func checkUnderstanding(out UnderstandingOutput, criteria []CriterionRef, g *Gate, ctx context.Context, projectID string) []string {
	var errs []string
	known := make(map[string]bool, len(criteria))
	for _, c := range criteria {
		known[c.Label] = true
	}
	seen := make(map[string]bool, len(out.Criteria))
	for _, entry := range out.Criteria {
		switch {
		case strings.HasPrefix(entry.Criterion, "new-"):
			errs = append(errs, fmt.Sprintf("/criteria: %s is a new-n; use the criterion label as given", entry.Criterion))
		case !known[entry.Criterion]:
			errs = append(errs, fmt.Sprintf("/criteria: %s is not a criterion of this request", entry.Criterion))
		case seen[entry.Criterion]:
			errs = append(errs, fmt.Sprintf("/criteria: %s appears more than once", entry.Criterion))
		}
		seen[entry.Criterion] = true
	}
	for _, c := range criteria {
		if !seen[c.Label] {
			errs = append(errs, fmt.Sprintf("/criteria: missing an entry for %s", c.Label))
		}
	}
	newSeen := map[string]bool{}
	for _, sec := range out.Sections {
		if strings.HasPrefix(sec.ID, "new-") {
			if newSeen[sec.ID] {
				errs = append(errs, fmt.Sprintf("/sections: %s is declared more than once", sec.ID))
			}
			newSeen[sec.ID] = true
			continue
		}
		if _, err := g.db.GetObject(ctx, projectID, sec.ID); err != nil {
			errs = append(errs, fmt.Sprintf("/sections: %s is not a section of this project", sec.ID))
		}
	}
	return errs
}

func validateUnderstanding(output json.RawMessage) ([]string, UnderstandingOutput, error) {
	var parsed UnderstandingOutput
	inst, err := jsonschema.UnmarshalJSON(bytes.NewReader(output))
	if err != nil {
		return []string{"/: output is not valid JSON"}, parsed, nil
	}
	sch, doc, err := compiledUnderstandingSchema()
	if err != nil {
		return nil, parsed, err
	}
	if obj, ok := inst.(map[string]any); ok {
		stripOptionalNulls(obj, doc, doc)
	}
	if err := sch.Validate(inst); err != nil {
		var verr *jsonschema.ValidationError
		if !errors.As(err, &verr) {
			return nil, parsed, fmt.Errorf("cycle: validate understanding: %w", err)
		}
		return flattenErrors(verr), parsed, nil
	}
	raw, err := json.Marshal(inst)
	if err != nil {
		return nil, parsed, fmt.Errorf("cycle: re-encode understanding: %w", err)
	}
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return nil, parsed, fmt.Errorf("cycle: parse understanding: %w", err)
	}
	return nil, parsed, nil
}

var (
	undSchemaOnce sync.Once
	undCompiled   *jsonschema.Schema
	undDoc        map[string]any
	undErr        error
)

func compiledUnderstandingSchema() (*jsonschema.Schema, map[string]any, error) {
	undSchemaOnce.Do(func() {
		doc, err := jsonschema.UnmarshalJSON(bytes.NewReader(understandingSchema))
		if err != nil {
			undErr = fmt.Errorf("cycle: parse understanding schema: %w", err)
			return
		}
		c := jsonschema.NewCompiler()
		const loc = "https://tupaqnin.invalid/understanding_output.schema.json"
		if err := c.AddResource(loc, doc); err != nil {
			undErr = fmt.Errorf("cycle: add understanding schema: %w", err)
			return
		}
		sch, err := c.Compile(loc)
		if err != nil {
			undErr = fmt.Errorf("cycle: compile understanding schema: %w", err)
			return
		}
		var rawDoc map[string]any
		if err := json.Unmarshal(understandingSchema, &rawDoc); err != nil {
			undErr = fmt.Errorf("cycle: decode understanding schema: %w", err)
			return
		}
		undCompiled, undDoc = sch, rawDoc
	})
	return undCompiled, undDoc, undErr
}
