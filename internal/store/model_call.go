package store

import (
	"context"
	"encoding/json"
	"fmt"
)

// ModelCall is one recorded model call (specification §1.4): what was
// sent and what came back, byte-for-byte, plus usage and cost. The
// principal's conversation is rebuilt from these rows between turns.
type ModelCall struct {
	ProjectID      string
	InstanceID     string
	TurnID         string
	Role           string
	ModelAndEffort string
	InputTokens    int
	OutputTokens   int
	ContextWindow  int
	InputItems     json.RawMessage
	RawItems       json.RawMessage
	CostUSD        float64
}

// RecordModelCall appends one model_call row. Content is stored here
// and never logged (T-10).
func (db *DB) RecordModelCall(ctx context.Context, c ModelCall) error {
	if _, err := db.Writer.ExecContext(ctx,
		`INSERT INTO model_call (project_id, instance_id, turn_id, role,
			model_and_effort, input_tokens, output_tokens, context_window,
			input_items, raw_items, cost, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		c.ProjectID, nullStr(c.InstanceID), nullStr(c.TurnID), nullStr(c.Role),
		nullStr(c.ModelAndEffort), c.InputTokens, c.OutputTokens, c.ContextWindow,
		nullRaw(c.InputItems), nullRaw(c.RawItems), c.CostUSD, nowText()); err != nil {
		return fmt.Errorf("record model call: %w", err)
	}
	return nil
}

func nullRaw(r json.RawMessage) any {
	if len(r) == 0 {
		return nil
	}
	return string(r)
}
