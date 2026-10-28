package store

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/base32"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"golang.org/x/text/unicode/norm"
)

// nowText formats a timestamp as fixed-width UTC text (T-7).
func nowText() string {
	return time.Now().UTC().Format("2006-01-02T15:04:05.000") + "Z"
}

// newID returns a random 128-bit id in base32 (specification §1, T-2).
func newID() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", fmt.Errorf("generate id: %w", err)
	}
	return strings.ToLower(base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(b[:])), nil
}

func marshalBody(body any) (json.RawMessage, error) {
	if body == nil {
		return json.RawMessage("null"), nil
	}
	if raw, ok := body.(json.RawMessage); ok {
		if !json.Valid(raw) {
			return nil, fmt.Errorf("body: invalid JSON")
		}
		return raw, nil
	}
	b, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("marshal body: %w", err)
	}
	return b, nil
}

func marshalData(data any) (sql.NullString, error) {
	if data == nil {
		return sql.NullString{}, nil
	}
	raw, err := marshalBody(data)
	if err != nil {
		return sql.NullString{}, err
	}
	return sql.NullString{String: string(raw), Valid: true}, nil
}

// normalizeForCompare applies B-3: NFC and line endings normalized,
// recursively over every string inside a JSON value.
func normalizeForCompare(v any) any {
	switch x := v.(type) {
	case string:
		s := strings.ReplaceAll(x, "\r\n", "\n")
		s = strings.ReplaceAll(s, "\r", "\n")
		return norm.NFC.String(s)
	case []any:
		for i := range x {
			x[i] = normalizeForCompare(x[i])
		}
		return x
	case map[string]any:
		for k := range x {
			x[k] = normalizeForCompare(x[k])
		}
		return x
	default:
		return v
	}
}

// sameBody reports whether two JSON bodies are canonically equal (B-3).
func sameBody(a, b json.RawMessage) (bool, error) {
	var av, bv any
	if err := json.Unmarshal(a, &av); err != nil {
		return false, fmt.Errorf("compare body: %w", err)
	}
	if err := json.Unmarshal(b, &bv); err != nil {
		return false, fmt.Errorf("compare body: %w", err)
	}
	ab, err := json.Marshal(normalizeForCompare(av))
	if err != nil {
		return false, err
	}
	bb, err := json.Marshal(normalizeForCompare(bv))
	if err != nil {
		return false, err
	}
	return string(ab) == string(bb), nil
}

// Tx is one write transaction on the single writer, scoped to a project.
// Every write and its event commit or roll back together (§2).
type Tx struct {
	ctx       context.Context
	tx        *sql.Tx
	projectID string
}

// Write runs fn in one transaction on the single writer for projectID.
// No transaction is ever left open while waiting for a model (§2).
func (db *DB) Write(ctx context.Context, projectID string, fn func(tx *Tx) error) error {
	sqlTx, err := db.Writer.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin write: %w", err)
	}
	defer func() { _ = sqlTx.Rollback() }()

	tx := &Tx{ctx: ctx, tx: sqlTx, projectID: projectID}
	if err := fn(tx); err != nil {
		return err
	}
	if err := sqlTx.Commit(); err != nil {
		return fmt.Errorf("commit write: %w", err)
	}
	return nil
}

func (tx *Tx) nextLabel(prefix string) (string, error) {
	var n int
	err := tx.tx.QueryRowContext(tx.ctx,
		`INSERT INTO label_counter (project_id, prefix, next) VALUES (?, ?, 2)
		 ON CONFLICT (project_id, prefix) DO UPDATE SET next = next + 1
		 RETURNING next - 1`, tx.projectID, prefix).Scan(&n)
	if err != nil {
		return "", fmt.Errorf("assign label: %w", err)
	}
	return fmt.Sprintf("%s-%d", prefix, n), nil
}

// AppendEvent appends one event with the next seq for the project.
// History only grows: there is no update or delete for events.
func (tx *Tx) AppendEvent(ev EventInput) (Event, error) {
	if !ev.Type.Valid() {
		return Event{}, fmt.Errorf("%w: event type %q", ErrInvalidType, ev.Type)
	}
	data, err := marshalData(ev.Data)
	if err != nil {
		return Event{}, err
	}
	out := Event{
		ProjectID: tx.projectID, Type: ev.Type, Actor: ev.Actor,
		AgentRole: ev.AgentRole, ModelAndEffort: ev.ModelAndEffort,
		Reason: ev.Reason, ObjectID: ev.ObjectID, BaseVersion: ev.BaseVersion,
		NewVersion: ev.NewVersion, TurnID: ev.TurnID, CreatedAt: nowText(),
	}
	if data.Valid {
		out.Data = json.RawMessage(data.String)
	}
	err = tx.tx.QueryRowContext(tx.ctx,
		`INSERT INTO event (project_id, seq, type, actor, agent_role, model_and_effort,
			reason, object_id, base_version, new_version, turn_id, data, created_at)
		 VALUES (?, (SELECT COALESCE(MAX(seq), 0) + 1 FROM event WHERE project_id = ?),
			?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		 RETURNING seq`,
		tx.projectID, tx.projectID, string(ev.Type), nullStr(ev.Actor),
		nullStr(ev.AgentRole), nullStr(ev.ModelAndEffort), nullStr(ev.Reason),
		nullStr(ev.ObjectID), nullInt(ev.BaseVersion), nullInt(ev.NewVersion),
		nullStr(ev.TurnID), data, out.CreatedAt).Scan(&out.Seq)
	if err != nil {
		return Event{}, fmt.Errorf("append event: %w", err)
	}
	return out, nil
}

func nullStr(s string) sql.NullString { return sql.NullString{String: s, Valid: s != ""} }
func nullInt(n int) sql.NullInt64     { return sql.NullInt64{Int64: int64(n), Valid: n != 0} }

// CreateObject creates an object at version 1, assigning its internal
// id and its short label in the same transaction (T-2), and appends ev.
// The event's ObjectID/NewVersion default to the new object and 1.
func (tx *Tx) CreateObject(t ObjectType, state string, body any, ev EventInput) (Object, error) {
	if !t.Valid() {
		return Object{}, fmt.Errorf("%w: object type %q", ErrInvalidType, t)
	}
	if !t.AllowsState(state) {
		return Object{}, fmt.Errorf("%w: type %q does not admit state %q", ErrInvalidState, t, state)
	}
	raw, err := marshalBody(body)
	if err != nil {
		return Object{}, err
	}
	id, err := newID()
	if err != nil {
		return Object{}, err
	}
	label, err := tx.nextLabel(t.LabelPrefix())
	if err != nil {
		return Object{}, err
	}
	obj := Object{ProjectID: tx.projectID, ID: id, Label: label, Type: t,
		State: state, CurrentVersion: 1, Body: raw}
	if _, err := tx.tx.ExecContext(tx.ctx,
		`INSERT INTO object (project_id, id, label, type, state, current_version)
		 VALUES (?, ?, ?, ?, ?, 1)`,
		tx.projectID, id, label, string(t), nullStr(state)); err != nil {
		return Object{}, fmt.Errorf("create object: %w", err)
	}
	if _, err := tx.tx.ExecContext(tx.ctx,
		`INSERT INTO version (project_id, object_id, version, body, created_at)
		 VALUES (?, ?, 1, ?, ?)`, tx.projectID, id, string(raw), nowText()); err != nil {
		return Object{}, fmt.Errorf("create version: %w", err)
	}
	if ev.ObjectID == "" {
		ev.ObjectID = id
	}
	if ev.NewVersion == 0 {
		ev.NewVersion = 1
	}
	if _, err := tx.AppendEvent(ev); err != nil {
		return Object{}, err
	}
	return obj, nil
}

func (tx *Tx) getObject(id string) (Object, error) {
	var obj Object
	var state sql.NullString
	var body string
	err := tx.tx.QueryRowContext(tx.ctx,
		`SELECT o.id, o.label, o.type, o.state, o.current_version, v.body
		 FROM object o
		 JOIN version v ON v.project_id = o.project_id AND v.object_id = o.id
			AND v.version = o.current_version
		 WHERE o.project_id = ? AND o.id = ?`, tx.projectID, id).
		Scan(&obj.ID, &obj.Label, &obj.Type, &state, &obj.CurrentVersion, &body)
	if errors.Is(err, sql.ErrNoRows) {
		return Object{}, ErrNotFound
	}
	if err != nil {
		return Object{}, fmt.Errorf("get object: %w", err)
	}
	obj.ProjectID = tx.projectID
	obj.State = state.String
	obj.Body = json.RawMessage(body)
	return obj, nil
}

// UpdateObject writes a new version conditioned on expectedVersion (§2).
// It returns ErrVersionConflict when the conditioned update does not
// hit exactly one row, and ErrNoChange when state and body are
// canonically unchanged (B-3), writing nothing in either case.
func (tx *Tx) UpdateObject(id string, expectedVersion int, state string, body any, ev EventInput) (Object, error) {
	cur, err := tx.getObject(id)
	if err != nil {
		return Object{}, err
	}
	if !cur.Type.AllowsState(state) {
		return Object{}, fmt.Errorf("%w: type %q does not admit state %q", ErrInvalidState, cur.Type, state)
	}
	if expectedVersion != cur.CurrentVersion {
		return Object{}, ErrVersionConflict
	}
	raw, err := marshalBody(body)
	if err != nil {
		return Object{}, err
	}
	same, err := sameBody(cur.Body, raw)
	if err != nil {
		return Object{}, err
	}
	if same && state == cur.State {
		return Object{}, ErrNoChange
	}
	res, err := tx.tx.ExecContext(tx.ctx,
		`UPDATE object SET state = ?, current_version = current_version + 1
		 WHERE project_id = ? AND id = ? AND current_version = ?`,
		nullStr(state), tx.projectID, id, expectedVersion)
	if err != nil {
		return Object{}, fmt.Errorf("update object: %w", err)
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return Object{}, fmt.Errorf("update object: %w", err)
	}
	if affected != 1 {
		return Object{}, ErrVersionConflict
	}
	newVersion := expectedVersion + 1
	if _, err := tx.tx.ExecContext(tx.ctx,
		`INSERT INTO version (project_id, object_id, version, body, created_at)
		 VALUES (?, ?, ?, ?, ?)`, tx.projectID, id, newVersion, string(raw), nowText()); err != nil {
		return Object{}, fmt.Errorf("create version: %w", err)
	}
	if ev.ObjectID == "" {
		ev.ObjectID = id
	}
	if ev.BaseVersion == 0 {
		ev.BaseVersion = expectedVersion
	}
	if ev.NewVersion == 0 {
		ev.NewVersion = newVersion
	}
	if _, err := tx.AppendEvent(ev); err != nil {
		return Object{}, err
	}
	cur.State = state
	cur.CurrentVersion = newVersion
	cur.Body = raw
	return cur, nil
}

// AddEdge creates a current edge at version 1 and appends ev.
func (tx *Tx) AddEdge(t EdgeType, fromID, toID, anchor string, ev EventInput) (Edge, error) {
	if !t.Valid() {
		return Edge{}, fmt.Errorf("%w: edge type %q", ErrInvalidType, t)
	}
	for _, id := range []string{fromID, toID} {
		if _, err := tx.getObject(id); err != nil {
			return Edge{}, err
		}
	}
	id, err := newID()
	if err != nil {
		return Edge{}, err
	}
	edge := Edge{ProjectID: tx.projectID, ID: id, Type: t, FromID: fromID,
		ToID: toID, Anchor: anchor, Version: 1, Current: true}
	if _, err := tx.tx.ExecContext(tx.ctx,
		`INSERT INTO edge (project_id, id, type, from_id, to_id, version, anchor, current)
		 VALUES (?, ?, ?, ?, ?, 1, ?, 1)`,
		tx.projectID, id, string(t), fromID, toID, nullStr(anchor)); err != nil {
		return Edge{}, fmt.Errorf("create edge: %w", err)
	}
	if _, err := tx.AppendEvent(ev); err != nil {
		return Edge{}, err
	}
	return edge, nil
}

// SupersedeEdge marks the edge not current and creates its next
// version (a new row with the same type, ends and anchor), and
// appends ev. The superseded row stays in the table.
// [EL DISEÑO NO LO DICE]: the design says a new version is created
// without naming its row identity; a new row is the smallest choice
// because (project_id, id) is the edge key.
func (tx *Tx) SupersedeEdge(edgeID string, ev EventInput) (Edge, error) {
	var old Edge
	var anchor sql.NullString
	err := tx.tx.QueryRowContext(tx.ctx,
		`SELECT id, type, from_id, to_id, anchor, version FROM edge
		 WHERE project_id = ? AND id = ? AND current = 1`, tx.projectID, edgeID).
		Scan(&old.ID, &old.Type, &old.FromID, &old.ToID, &anchor, &old.Version)
	if errors.Is(err, sql.ErrNoRows) {
		return Edge{}, ErrNotFound
	}
	if err != nil {
		return Edge{}, fmt.Errorf("get edge: %w", err)
	}
	if _, err := tx.tx.ExecContext(tx.ctx,
		`UPDATE edge SET current = 0 WHERE project_id = ? AND id = ?`,
		tx.projectID, edgeID); err != nil {
		return Edge{}, fmt.Errorf("supersede edge: %w", err)
	}
	id, err := newID()
	if err != nil {
		return Edge{}, err
	}
	next := Edge{ProjectID: tx.projectID, ID: id, Type: old.Type, FromID: old.FromID,
		ToID: old.ToID, Anchor: anchor.String, Version: old.Version + 1, Current: true}
	if _, err := tx.tx.ExecContext(tx.ctx,
		`INSERT INTO edge (project_id, id, type, from_id, to_id, version, anchor, current)
		 VALUES (?, ?, ?, ?, ?, ?, ?, 1)`,
		tx.projectID, id, string(next.Type), next.FromID, next.ToID, next.Version, anchor); err != nil {
		return Edge{}, fmt.Errorf("create edge version: %w", err)
	}
	if _, err := tx.AppendEvent(ev); err != nil {
		return Edge{}, err
	}
	return next, nil
}
