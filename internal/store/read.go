package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
)

// queryer is what the read pool and a write transaction share.
type queryer interface {
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

// Querier reads the current graph: the read pool, or a write
// transaction that must see its own writes, as the criticality does
// in each transaction that changes edges or a role (T-5).
type Querier interface {
	GetObject(ctx context.Context, projectID, idOrLabel string) (Object, error)
	ListObjects(ctx context.Context, projectID string, t ObjectType) ([]Object, error)
	AllObjects(ctx context.Context, projectID string) ([]Object, error)
	EdgesFrom(ctx context.Context, projectID, fromID string) ([]Edge, error)
	EdgesTo(ctx context.Context, projectID, toID string) ([]Edge, error)
}

var (
	_ Querier = (*DB)(nil)
	_ Querier = (*Tx)(nil)
)

// GetObject, ListObjects, AllObjects, EdgesFrom and EdgesTo read inside
// the transaction, its own writes included.
func (tx *Tx) GetObject(ctx context.Context, projectID, idOrLabel string) (Object, error) {
	return getObject(ctx, tx.tx, projectID, idOrLabel)
}

func (tx *Tx) ListObjects(ctx context.Context, projectID string, t ObjectType) ([]Object, error) {
	return listObjects(ctx, tx.tx, projectID, t)
}

func (tx *Tx) AllObjects(ctx context.Context, projectID string) ([]Object, error) {
	return allObjects(ctx, tx.tx, projectID)
}

func (tx *Tx) EdgesFrom(ctx context.Context, projectID, fromID string) ([]Edge, error) {
	return edges(ctx, tx.tx, projectID, "from_id", fromID)
}

func (tx *Tx) EdgesTo(ctx context.Context, projectID, toID string) ([]Edge, error) {
	return edges(ctx, tx.tx, projectID, "to_id", toID)
}

// ProjectID is the project the transaction writes to.
func (tx *Tx) ProjectID() string { return tx.projectID }

// GetObject returns the object by internal id or short label within
// the project, with the body of its current version.
func (db *DB) GetObject(ctx context.Context, projectID, idOrLabel string) (Object, error) {
	return getObject(ctx, db.Reader, projectID, idOrLabel)
}

func getObject(ctx context.Context, q queryer, projectID, idOrLabel string) (Object, error) {
	var obj Object
	var state sql.NullString
	var body string
	err := q.QueryRowContext(ctx,
		`SELECT o.id, o.label, o.type, o.state, o.current_version, v.body
		 FROM object o
		 JOIN version v ON v.project_id = o.project_id AND v.object_id = o.id
			AND v.version = o.current_version
		 WHERE o.project_id = ? AND (o.id = ? OR o.label = ?)`,
		projectID, idOrLabel, idOrLabel).
		Scan(&obj.ID, &obj.Label, &obj.Type, &state, &obj.CurrentVersion, &body)
	if errors.Is(err, sql.ErrNoRows) {
		return Object{}, ErrNotFound
	}
	if err != nil {
		return Object{}, fmt.Errorf("get object: %w", err)
	}
	obj.ProjectID = projectID
	obj.State = state.String
	obj.Body = json.RawMessage(body)
	return obj, nil
}

// ListObjects returns every object of the type in the project,
// ordered by label.
func (db *DB) ListObjects(ctx context.Context, projectID string, t ObjectType) ([]Object, error) {
	return listObjects(ctx, db.Reader, projectID, t)
}

func listObjects(ctx context.Context, q queryer, projectID string, t ObjectType) ([]Object, error) {
	if !t.Valid() {
		return nil, fmt.Errorf("%w: object type %q", ErrInvalidType, t)
	}
	rows, err := q.QueryContext(ctx,
		`SELECT o.id, o.label, o.type, o.state, o.current_version, v.body
		 FROM object o
		 JOIN version v ON v.project_id = o.project_id AND v.object_id = o.id
			AND v.version = o.current_version
		 WHERE o.project_id = ? AND o.type = ? ORDER BY o.label`, projectID, string(t))
	if err != nil {
		return nil, fmt.Errorf("list objects: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var out []Object
	for rows.Next() {
		var obj Object
		var state sql.NullString
		var body string
		if err := rows.Scan(&obj.ID, &obj.Label, &obj.Type, &state, &obj.CurrentVersion, &body); err != nil {
			return nil, fmt.Errorf("list objects: %w", err)
		}
		obj.ProjectID = projectID
		obj.State = state.String
		obj.Body = json.RawMessage(body)
		out = append(out, obj)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list objects: %w", err)
	}
	return out, nil
}

func edges(ctx context.Context, q queryer, projectID, column, id string) ([]Edge, error) {
	// column is one of two fixed literals chosen below, never input.
	rows, err := q.QueryContext(ctx,
		`SELECT id, type, from_id, to_id, anchor, version, current FROM edge
		 WHERE project_id = ? AND `+column+` = ? AND current = 1 ORDER BY id`,
		projectID, id)
	if err != nil {
		return nil, fmt.Errorf("list edges: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var out []Edge
	for rows.Next() {
		var e Edge
		var anchor sql.NullString
		var current int
		if err := rows.Scan(&e.ID, &e.Type, &e.FromID, &e.ToID, &anchor, &e.Version, &current); err != nil {
			return nil, fmt.Errorf("list edges: %w", err)
		}
		e.ProjectID = projectID
		e.Anchor = anchor.String
		e.Current = current == 1
		out = append(out, e)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list edges: %w", err)
	}
	return out, nil
}

// EdgesFrom returns the current edges leaving the object, in the project.
// [EL DISEÑO NO LO DICE]: whether reads return superseded edges is
// not stated; only current edges are returned, as the graph uses them.
func (db *DB) EdgesFrom(ctx context.Context, projectID, fromID string) ([]Edge, error) {
	return edges(ctx, db.Reader, projectID, "from_id", fromID)
}

// EdgesTo returns the current edges entering the object, in the project.
func (db *DB) EdgesTo(ctx context.Context, projectID, toID string) ([]Edge, error) {
	return edges(ctx, db.Reader, projectID, "to_id", toID)
}

// EventsSince returns the history after seq, in seq order (T-7: the
// order of the history is event.seq, not the clock).
func (db *DB) EventsSince(ctx context.Context, projectID string, seq int) ([]Event, error) {
	rows, err := db.Reader.QueryContext(ctx,
		`SELECT seq, type, actor, agent_role, model_and_effort, reason, object_id,
			base_version, new_version, turn_id, data, created_at
		 FROM event WHERE project_id = ? AND seq > ? ORDER BY seq`, projectID, seq)
	if err != nil {
		return nil, fmt.Errorf("list events: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var out []Event
	for rows.Next() {
		var e Event
		var actor, role, model, reason, objectID, turnID, data sql.NullString
		var base, newV sql.NullInt64
		if err := rows.Scan(&e.Seq, &e.Type, &actor, &role, &model, &reason,
			&objectID, &base, &newV, &turnID, &data, &e.CreatedAt); err != nil {
			return nil, fmt.Errorf("list events: %w", err)
		}
		e.ProjectID = projectID
		e.Actor, e.AgentRole, e.ModelAndEffort = actor.String, role.String, model.String
		e.Reason, e.ObjectID, e.TurnID = reason.String, objectID.String, turnID.String
		e.BaseVersion, e.NewVersion = int(base.Int64), int(newV.Int64)
		if data.Valid {
			e.Data = json.RawMessage(data.String)
		}
		out = append(out, e)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list events: %w", err)
	}
	return out, nil
}

// AllObjects returns every object of the project with the body of its
// current version, ordered by label. It backs the computed working
// state (specification §5: the memory is a query, never a source).
func (db *DB) AllObjects(ctx context.Context, projectID string) ([]Object, error) {
	return allObjects(ctx, db.Reader, projectID)
}

func allObjects(ctx context.Context, q queryer, projectID string) ([]Object, error) {
	rows, err := q.QueryContext(ctx,
		`SELECT o.id, o.label, o.type, o.state, o.current_version, v.body
		 FROM object o
		 JOIN version v ON v.project_id = o.project_id AND v.object_id = o.id
			AND v.version = o.current_version
		 WHERE o.project_id = ? ORDER BY o.label`, projectID)
	if err != nil {
		return nil, fmt.Errorf("list all objects: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var out []Object
	for rows.Next() {
		var obj Object
		var state sql.NullString
		var body string
		if err := rows.Scan(&obj.ID, &obj.Label, &obj.Type, &state, &obj.CurrentVersion, &body); err != nil {
			return nil, fmt.Errorf("list all objects: %w", err)
		}
		obj.ProjectID = projectID
		obj.State = state.String
		obj.Body = json.RawMessage(body)
		out = append(out, obj)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list all objects: %w", err)
	}
	return out, nil
}
