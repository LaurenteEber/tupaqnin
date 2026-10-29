package store

import (
	"context"
	"database/sql"
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
	CreatedAt      string
}

// ModelCalls returns every recorded call of one principal instance,
// in the order they were made. The conversation of a live instance
// is rebuilt from these rows (B-7).
func (db *DB) ModelCalls(ctx context.Context, projectID, instanceID string) ([]ModelCall, error) {
	rows, err := db.Reader.QueryContext(ctx,
		`SELECT instance_id, turn_id, role, model_and_effort, input_tokens,
			output_tokens, context_window, input_items, raw_items, cost, created_at
		 FROM model_call WHERE project_id = ? AND instance_id = ? ORDER BY rowid`,
		projectID, instanceID)
	if err != nil {
		return nil, fmt.Errorf("model calls: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var out []ModelCall
	for rows.Next() {
		var c ModelCall
		var input, raw []byte
		var turn, role, model sql.NullString
		c.ProjectID = projectID
		if err := rows.Scan(&c.InstanceID, &turn, &role, &model,
			&c.InputTokens, &c.OutputTokens, &c.ContextWindow, &input, &raw,
			&c.CostUSD, &c.CreatedAt); err != nil {
			return nil, fmt.Errorf("model calls: %w", err)
		}
		c.TurnID, c.Role, c.ModelAndEffort = turn.String, role.String, model.String
		c.InputItems = input
		c.RawItems = raw
		out = append(out, c)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("model calls: %w", err)
	}
	return out, nil
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

// ProjectSpend is what the project's model calls cost, in USD (B3-20,
// P3: the sum of model_call.cost).
func (db *DB) ProjectSpend(ctx context.Context, projectID string) (float64, error) {
	var spent sql.NullFloat64
	if err := db.Reader.QueryRowContext(ctx,
		`SELECT SUM(cost) FROM model_call WHERE project_id = ?`, projectID).Scan(&spent); err != nil {
		return 0, fmt.Errorf("project spend: %w", err)
	}
	return spent.Float64, nil
}
