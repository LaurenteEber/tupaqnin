package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

// Project is one row of the project list (backlog-r1 1.1, P1).
type Project struct {
	ID        string `json:"project_id"`
	Name      string `json:"name"`
	CreatedAt string `json:"created_at"`
}

// ProjectExists reports whether the project id exists.
func (db *DB) ProjectExists(ctx context.Context, projectID string) (bool, error) {
	var one int
	err := db.Reader.QueryRowContext(ctx,
		`SELECT 1 FROM project WHERE id = ?`, projectID).Scan(&one)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("project exists: %w", err)
	}
	return true, nil
}

// ProposeProjectName sets the project's name only while it is empty:
// the understanding proposes a title (U-1) and never replaces one the
// person wrote. It reports whether the name was set.
// [EL DISEÑO NO LO DICE]: the name has no event; it is a column of
// project, not an object of the graph.
func (tx *Tx) ProposeProjectName(name string) (bool, error) {
	res, err := tx.tx.ExecContext(tx.ctx,
		`UPDATE project SET name = ? WHERE id = ? AND COALESCE(name, '') = ''`, name, tx.projectID)
	if err != nil {
		return false, fmt.Errorf("propose project name: %w", err)
	}
	n, err := res.RowsAffected()
	return n > 0, err
}

// RenameProject sets the name the person wrote in the header (U-1).
func (db *DB) RenameProject(ctx context.Context, projectID, name string) error {
	if _, err := db.Writer.ExecContext(ctx,
		`UPDATE project SET name = ? WHERE id = ?`, name, projectID); err != nil {
		return fmt.Errorf("rename project: %w", err)
	}
	return nil
}

// ProjectName is the project's name (B-18, U-1); empty when it has none.
func (db *DB) ProjectName(ctx context.Context, projectID string) (string, error) {
	var name string
	err := db.Reader.QueryRowContext(ctx,
		`SELECT COALESCE(name, '') FROM project WHERE id = ?`, projectID).Scan(&name)
	if err != nil {
		return "", fmt.Errorf("project name: %w", err)
	}
	return name, nil
}

// ProjectsWithMode are the projects with a mode_set in their history, for the
// sweep of the modes a restart left on (P-114).
func (db *DB) ProjectsWithMode(ctx context.Context) ([]string, error) {
	rows, err := db.Reader.QueryContext(ctx, `SELECT DISTINCT project_id FROM event WHERE type = ?`, string(EventModeSet))
	if err != nil {
		return nil, fmt.Errorf("store: projects with mode: %w", err)
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("store: projects with mode: %w", err)
		}
		out = append(out, id)
	}
	return out, rows.Err()
}
