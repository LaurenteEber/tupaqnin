package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// SpendState is what the spend gate reads before a model call (B-89).
// InvitationID is empty for a project without a link (an older local
// database): it has the project's cap only.
type SpendState struct {
	ProjectUSD float64
	// DearestUSD is the dearest call of the asked role in the project.
	DearestUSD float64
	// ContrasterUSD is what the project's contraster calls cost: the part of
	// its signoff balance already used (B-89).
	ContrasterUSD float64
	LinkState
}

// LinkState is a link's side of the caps (B-89).
type LinkState struct {
	InvitationID string
	Label        string
	BudgetUSD    float64
	SpentUSD     float64
	// DayUSD is model_call.cost of the UTC day of the clock given, by
	// created_at, of the projects whose link is not the jury's (B-89).
	DayUSD float64
	// TotalUSD is the sum of spent_usd of every link.
	TotalUSD float64
}

// LinkSpendState reads a link's side of the caps.
func (db *DB) LinkSpendState(ctx context.Context, invitationID string, now time.Time) (LinkState, error) {
	day := now.UTC().Truncate(24 * time.Hour)
	s := LinkState{InvitationID: invitationID}
	err := db.Reader.QueryRowContext(ctx,
		`SELECT i.label, i.budget_usd, i.spent_usd,
		        (SELECT COALESCE(SUM(spent_usd), 0) FROM invitation),
		        (SELECT COALESCE(SUM(m.cost), 0) FROM model_call m
		         JOIN project p ON p.id = m.project_id
		         JOIN invitation l ON l.id = p.invitation_id
		         WHERE l.label <> ? AND m.created_at >= ? AND m.created_at < ?)
		 FROM invitation i WHERE i.id = ?`,
		LabelJury, timeText(day), timeText(day.Add(24*time.Hour)), invitationID).
		Scan(&s.Label, &s.BudgetUSD, &s.SpentUSD, &s.TotalUSD, &s.DayUSD)
	if err != nil {
		return LinkState{}, fmt.Errorf("link spend: %w", err)
	}
	return s, nil
}

// ProjectSpendState reads the caps' figures of one project for one role.
func (db *DB) ProjectSpendState(ctx context.Context, projectID, role string, now time.Time) (SpendState, error) {
	var s SpendState
	var invitation sql.NullString
	err := db.Reader.QueryRowContext(ctx,
		`SELECT p.invitation_id,
		        (SELECT COALESCE(SUM(cost), 0) FROM model_call WHERE project_id = p.id),
		        (SELECT COALESCE(MAX(cost), 0) FROM model_call WHERE project_id = p.id AND role = ?),
		        (SELECT COALESCE(SUM(cost), 0) FROM model_call WHERE project_id = p.id AND role = 'contraster')
		 FROM project p WHERE p.id = ?`, role, projectID).
		Scan(&invitation, &s.ProjectUSD, &s.DearestUSD, &s.ContrasterUSD)
	if errors.Is(err, sql.ErrNoRows) {
		return SpendState{}, ErrNotFound
	}
	if err != nil {
		return SpendState{}, fmt.Errorf("project spend state: %w", err)
	}
	if invitation.Valid {
		if s.LinkState, err = db.LinkSpendState(ctx, invitation.String, now); err != nil {
			return SpendState{}, err
		}
	}
	return s, nil
}
