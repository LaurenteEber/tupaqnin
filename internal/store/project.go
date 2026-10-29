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

// CreateProject creates an empty project and returns its internal
// id (specification §1). Access control arrives with the delivery
// slice (§1.7); in slice 0 a project is reached by its id alone.
// The name goes in project.name (B-18, migration 0003).
func (db *DB) CreateProject(ctx context.Context, name string) (string, error) {
	id, err := newID()
	if err != nil {
		return "", err
	}
	if _, err := db.Writer.ExecContext(ctx,
		`INSERT INTO project (id, name, created_at) VALUES (?, ?, ?)`, id, name, nowText()); err != nil {
		return "", fmt.Errorf("create project: %w", err)
	}
	return id, nil
}

// ListProjects returns every project, newest first (P1 minimum).
func (db *DB) ListProjects(ctx context.Context) ([]Project, error) {
	rows, err := db.Reader.QueryContext(ctx,
		`SELECT id, COALESCE(name, ''), created_at FROM project ORDER BY created_at DESC, id`)
	if err != nil {
		return nil, fmt.Errorf("list projects: %w", err)
	}
	defer func() { _ = rows.Close() }()
	out := []Project{}
	for rows.Next() {
		var p Project
		if err := rows.Scan(&p.ID, &p.Name, &p.CreatedAt); err != nil {
			return nil, fmt.Errorf("list projects: %w", err)
		}
		out = append(out, p)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list projects: %w", err)
	}
	return out, nil
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
