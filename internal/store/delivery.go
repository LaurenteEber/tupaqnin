package store

import (
	"context"
	"fmt"
)

// How something reached the principal (specification §1.4).
const (
	ViaStartup = "startup"
	ViaRead    = "read"
	ViaNotice  = "notice"
)

// Delivery is one delivered record: an object version handed to a
// principal instance (specification §1.4).
type Delivery struct {
	ProjectID  string
	InstanceID string
	Turn       int
	Via        string
	ObjectID   string
	Version    int
}

// RecordDelivery appends one delivery row. Content is never logged
// here (T-10); only ids and versions are stored.
func (db *DB) RecordDelivery(ctx context.Context, d Delivery) error {
	if _, err := db.Writer.ExecContext(ctx,
		`INSERT INTO delivery (project_id, instance_id, turn, via, object_id, version, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		d.ProjectID, nullStr(d.InstanceID), nullInt(d.Turn), d.Via, d.ObjectID, d.Version, nowText()); err != nil {
		return fmt.Errorf("record delivery: %w", err)
	}
	return nil
}

// RecordDelivery appends one delivery row inside this write
// transaction, so a startup and its deliveries commit together.
func (tx *Tx) RecordDelivery(d Delivery) error {
	if _, err := tx.tx.ExecContext(tx.ctx,
		`INSERT INTO delivery (project_id, instance_id, turn, via, object_id, version, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		tx.projectID, nullStr(d.InstanceID), nullInt(d.Turn), d.Via, d.ObjectID, d.Version, nowText()); err != nil {
		return fmt.Errorf("record delivery: %w", err)
	}
	return nil
}

// LatestDeliveries returns, per object id, the highest version ever
// delivered to any principal instance (B2-2: whatever the instance).
func (db *DB) LatestDeliveries(ctx context.Context, projectID string) (map[string]int, error) {
	rows, err := db.Reader.QueryContext(ctx,
		`SELECT object_id, MAX(version) FROM delivery
		 WHERE project_id = ? GROUP BY object_id`, projectID)
	if err != nil {
		return nil, fmt.Errorf("latest deliveries: %w", err)
	}
	defer func() { _ = rows.Close() }()
	out := make(map[string]int)
	for rows.Next() {
		var id string
		var version int
		if err := rows.Scan(&id, &version); err != nil {
			return nil, fmt.Errorf("latest deliveries: %w", err)
		}
		out[id] = version
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("latest deliveries: %w", err)
	}
	return out, nil
}
