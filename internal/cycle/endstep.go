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

//go:embed schema/principal_output.schema.json
var principalSchema []byte

// StepStatus is the outcome of one EndStep call (backlog 0.7).
type StepStatus string

const (
	StatusAccepted     StepStatus = "accepted"
	StatusRetry        StepStatus = "retry"
	StatusAgentPending StepStatus = "agent_pending"
)

// StepResult is what EndStep reports: the status and, when the output
// did not match the fixed shape, the validation errors as a JSON path
// and a message each, without internal ids (backlog 0.7).
type StepResult struct {
	Status StepStatus
	Errors []string
}

// Gate validates the principal's fixed-shape step output and records
// the end of step in the history (specification §3, rule 1 only).
// [EL DISEÑO NO LO DICE]: the backlog names EndStep without a
// receiver; the gate holding the store is the smallest choice.
type Gate struct {
	db *store.DB
}

// NewGate returns a Gate over db.
func NewGate(db *store.DB) *Gate { return &Gate{db: db} }

// RejectionMessage is the message the turn runner sends when rules 2
// to 6 rejected a step that had the fixed shape (specification §3).
func RejectionMessage(errs []string) string {
	return "Your step was not applied: " + strings.Join(errs, "; ") +
		". Return the complete output again, corrected."
}

// RetryMessage is the message the turn runner sends when it calls the
// principal once more after a retry (backlog 0.7, verbatim shape).
func RetryMessage(errs []string) string {
	return "Your output did not match the required schema: " +
		strings.Join(errs, "; ") +
		". Return the complete output again."
}

type principalOutput struct {
	TriageLine  string          `json:"triage_line"`
	ClosingRisk string          `json:"closing_risk"`
	Note        string          `json:"note"`
	Deferred    json.RawMessage `json:"deferred"`
}

// EndStep validates output against the full principal schema in Go
// (B-2; specification §3, rule 1). It never calls the model and writes
// nothing for a valid output: ApplyStep checks rules 2 to 6 and writes
// step_ended with the step (backlog-r1 1.3). An invalid one appends
// output_retry: at attempt 1 the result is retry; at attempt 2 it is
// agent_pending (fichas §7).
func (g *Gate) EndStep(ctx context.Context, projectID, turnID string, attempt int, output json.RawMessage) (StepResult, error) {
	if attempt < 1 {
		return StepResult{}, fmt.Errorf("cycle: attempt must be 1 or more, got %d", attempt)
	}
	errs, _, err := validateOutput(output)
	if err != nil {
		return StepResult{}, err
	}
	if len(errs) == 0 {
		return StepResult{Status: StatusAccepted}, nil
	}
	return g.Reject(ctx, projectID, turnID, attempt, errs)
}

// Reject records an output that did not pass (the fixed shape, or rules
// 2 to 6, which count as the retry): output_retry, then retry at
// attempt 1 and agent_pending at attempt 2 (fichas §7).
func (g *Gate) Reject(ctx context.Context, projectID, turnID string, attempt int, errs []string) (StepResult, error) {
	status := StatusRetry
	data := map[string]any{"attempt": attempt, "errors": errs}
	if attempt >= 2 {
		status = StatusAgentPending
		data["result"] = "agent_pending"
	}
	if err := g.append(ctx, projectID, store.EventInput{
		Type: store.EventOutputRetry, Actor: "agent", AgentRole: "principal", TurnID: turnID,
		Data: data,
	}); err != nil {
		return StepResult{}, err
	}
	return StepResult{Status: status, Errors: errs}, nil
}

// cleanOutput is output without the nulls of optional properties (the
// transport schema sends them, B-2), re-encoded.
func cleanOutput(output json.RawMessage) (json.RawMessage, error) {
	inst, err := jsonschema.UnmarshalJSON(bytes.NewReader(output))
	if err != nil {
		return nil, fmt.Errorf("cycle: parse output: %w", err)
	}
	_, doc, err := compiledSchema()
	if err != nil {
		return nil, err
	}
	if obj, ok := inst.(map[string]any); ok {
		stripOptionalNulls(obj, doc, doc)
	}
	raw, err := json.Marshal(inst)
	if err != nil {
		return nil, fmt.Errorf("cycle: re-encode output: %w", err)
	}
	return raw, nil
}

func (g *Gate) append(ctx context.Context, projectID string, ev store.EventInput) error {
	return g.db.Write(ctx, projectID, func(tx *store.Tx) error {
		_, err := tx.AppendEvent(ev)
		return err
	})
}

// validateOutput returns the validation errors (empty when the output
// matches) and the parsed step fields. The output is never logged.
func validateOutput(output json.RawMessage) ([]string, principalOutput, error) {
	var parsed principalOutput
	inst, err := jsonschema.UnmarshalJSON(bytes.NewReader(output))
	if err != nil {
		// Not JSON, or not an object the schema can judge: it does not
		// match the fixed shape. The raw output stays out of the error.
		return []string{"/: output is not valid JSON"}, parsed, nil
	}
	sch, doc, err := compiledSchema()
	if err != nil {
		return nil, parsed, err
	}
	if obj, ok := inst.(map[string]any); ok {
		stripOptionalNulls(obj, doc, doc)
	}
	if err := sch.Validate(inst); err != nil {
		var verr *jsonschema.ValidationError
		if !errors.As(err, &verr) {
			return nil, parsed, fmt.Errorf("cycle: validate output: %w", err)
		}
		return flattenErrors(verr), parsed, nil
	}
	raw, err := json.Marshal(inst)
	if err != nil {
		return nil, parsed, fmt.Errorf("cycle: re-encode output: %w", err)
	}
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return nil, parsed, fmt.Errorf("cycle: parse output: %w", err)
	}
	return nil, parsed, nil
}

// flattenErrors renders a ValidationError as one "path: message"
// string per leaf cause (backlog 0.7: JSON path and message).
func flattenErrors(verr *jsonschema.ValidationError) []string {
	var out []string
	var walk func(u *jsonschema.OutputUnit)
	walk = func(u *jsonschema.OutputUnit) {
		if len(u.Errors) == 0 {
			if u.Error != nil {
				path := u.InstanceLocation
				if path == "" {
					path = "/"
				}
				out = append(out, path+": "+u.Error.String())
			}
			return
		}
		for i := range u.Errors {
			walk(&u.Errors[i])
		}
	}
	walk(verr.BasicOutput())
	if len(out) == 0 {
		out = append(out, "/: "+verr.Error())
	}
	return out
}

var (
	schemaOnce  sync.Once
	compiledSch *jsonschema.Schema
	compiledDoc map[string]any
	compiledErr error
)

func compiledSchema() (*jsonschema.Schema, map[string]any, error) {
	schemaOnce.Do(func() {
		doc, err := jsonschema.UnmarshalJSON(bytes.NewReader(principalSchema))
		if err != nil {
			compiledErr = fmt.Errorf("cycle: parse principal schema: %w", err)
			return
		}
		// The validator's default (RE2) engine is enough: the design
		// schema carries no lookahead (backlog 0.8 finding).
		c := jsonschema.NewCompiler()
		const loc = "https://tupaqnin.invalid/principal_output.schema.json"
		if err := c.AddResource(loc, doc); err != nil {
			compiledErr = fmt.Errorf("cycle: add principal schema: %w", err)
			return
		}
		sch, err := c.Compile(loc)
		if err != nil {
			compiledErr = fmt.Errorf("cycle: compile principal schema: %w", err)
			return
		}
		var rawDoc map[string]any
		if err := json.Unmarshal(principalSchema, &rawDoc); err != nil {
			compiledErr = fmt.Errorf("cycle: decode principal schema: %w", err)
			return
		}
		compiledSch, compiledDoc = sch, rawDoc
	})
	return compiledSch, compiledDoc, compiledErr
}

// stripOptionalNulls removes, recursively, the null values of
// properties that were optional in the full schema, so an output
// produced against the transport schema (where optionals are
// nullable, provider.TransportSchema) validates against the full
// schema (B-2; proven in micro-test 0.8).
func stripOptionalNulls(inst any, schema map[string]any, root map[string]any) {
	if ref, ok := schema["$ref"].(string); ok {
		if target, ok := resolveRef(ref, root); ok {
			stripOptionalNulls(inst, target, root)
		}
		return
	}
	obj, ok := inst.(map[string]any)
	if !ok {
		return
	}
	props, _ := schema["properties"].(map[string]any)
	required := make(map[string]bool)
	if req, ok := schema["required"].([]any); ok {
		for _, r := range req {
			if name, ok := r.(string); ok {
				required[name] = true
			}
		}
	}
	for name, value := range obj {
		propSchema, _ := props[name].(map[string]any)
		if value == nil && !required[name] && propSchema != nil {
			delete(obj, name)
			continue
		}
		if propSchema != nil {
			stripOptionalNullsInValue(value, propSchema, root)
		}
	}
}

func stripOptionalNullsInValue(inst any, schema map[string]any, root map[string]any) {
	if branches, ok := schema["anyOf"].([]any); ok {
		for _, branch := range branches {
			if b, ok := branch.(map[string]any); ok {
				stripOptionalNulls(inst, b, root)
			}
		}
		return
	}
	if arr, ok := inst.([]any); ok {
		if items, ok := schema["items"].(map[string]any); ok {
			for _, item := range arr {
				stripOptionalNullsInValue(item, items, root)
			}
		}
		return
	}
	stripOptionalNulls(inst, schema, root)
}

func resolveRef(ref string, root map[string]any) (map[string]any, bool) {
	const prefix = "#/$defs/"
	if !strings.HasPrefix(ref, prefix) {
		return nil, false
	}
	defs, _ := root["$defs"].(map[string]any)
	target, ok := defs[strings.TrimPrefix(ref, prefix)].(map[string]any)
	return target, ok
}
