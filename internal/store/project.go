package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

// CreateProject creates an empty project and returns its internal
// id (specification §1). Access control arrives with the delivery
// slice (§1.7); in slice 0 a project is reached by its id alone.
func (db *DB) CreateProject(ctx context.Context) (string, error) {
	id, err := newID()
	if err != nil {
		return "", err
	}
	if _, err := db.Writer.ExecContext(ctx,
		`INSERT INTO project (id, created_at) VALUES (?, ?)`, id, nowText()); err != nil {
		return "", fmt.Errorf("create project: %w", err)
	}
	return id, nil
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
