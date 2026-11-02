package store

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"time"
)

const (
	// SessionTTL is how long a session with an account lasts after your last
	// activity (§1.7, 14 days [inicial]; B3-18, 2-nov).
	SessionTTL = 14 * 24 * time.Hour
	// AnonSessionTTL is how long the server keeps a session without an
	// account after your last activity, since it cannot see the browser
	// close (B3-18, 2-nov, 24 h [inicial]).
	AnonSessionTTL = 24 * time.Hour
	// AnonProjectTTL is the same for that session's project: they go together.
	AnonProjectTTL = AnonSessionTTL
	// touchStep keeps the moving expiry to one write an hour (B-88).
	touchStep = time.Hour
)

var (
	// ErrAccountRequired is a second project without an account (B3-18).
	ErrAccountRequired = errors.New("account required")
	// ErrLinkFull is a project past the link's max_anon_projects (B3-22 row 2).
	ErrLinkFull = errors.New("link full")
	// ErrInvitationNotFound is a revoke of no link still active.
	ErrInvitationNotFound = errors.New("invitation not found")
)

// Session is one row of session (§1.7): AccountID is empty without an
// account (B-88).
type Session struct {
	IDHash       string
	AccountID    string
	InvitationID string
	ExpiresAt    string
}

// InvitationInput is what `invite create` fixes on a link (B-92).
type InvitationInput struct {
	Label           string
	BudgetUSD       float64
	MaxAccounts     int
	MaxAnonProjects int
}

// Invitation is one line of `invite list`: never the token nor its hash.
type Invitation struct {
	ID              string
	Label           string
	BudgetUSD       float64
	SpentUSD        float64
	MaxAccounts     int
	MaxAnonProjects int
	Accounts        int
	Projects        int
	RevokedAt       string
	CreatedAt       string
}

// NewSecret returns a new link token or session cookie (32 random bytes
// in base64url) and the SHA-256 the database keeps of it (B3-22 row 4).
func NewSecret() (secret, hash string, err error) {
	var b [32]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", "", fmt.Errorf("generate secret: %w", err)
	}
	secret = base64.RawURLEncoding.EncodeToString(b[:])
	return secret, HashSecret(secret), nil
}

// HashSecret is the SHA-256 of a link token or session cookie.
func HashSecret(secret string) string {
	sum := sha256.Sum256([]byte(secret))
	return hex.EncodeToString(sum[:])
}

// timeText writes a moment as T-7's fixed-width UTC text, so the dates
// compare as text.
func timeText(t time.Time) string {
	return t.UTC().Format("2006-01-02T15:04:05.000") + "Z"
}

// CreateInvitation stores a link by its token's hash (B-92).
func (db *DB) CreateInvitation(ctx context.Context, in InvitationInput, tokenHash string, now time.Time) (string, error) {
	id, err := newID()
	if err != nil {
		return "", err
	}
	if _, err := db.Writer.ExecContext(ctx,
		`INSERT INTO invitation (id, token_hash, label, budget_usd, max_accounts, max_anon_projects, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		id, tokenHash, in.Label, in.BudgetUSD, in.MaxAccounts, in.MaxAnonProjects, timeText(now)); err != nil {
		return "", fmt.Errorf("create invitation: %w", err)
	}
	return id, nil
}

// ActiveInvitationByTokenHash finds a link that is not revoked; ok is
// false when there is none.
func (db *DB) ActiveInvitationByTokenHash(ctx context.Context, tokenHash string) (id string, ok bool, err error) {
	err = db.Reader.QueryRowContext(ctx,
		`SELECT id FROM invitation WHERE token_hash = ? AND revoked_at IS NULL`, tokenHash).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return "", false, nil
	}
	if err != nil {
		return "", false, fmt.Errorf("find invitation: %w", err)
	}
	return id, true, nil
}

// ListInvitations returns every link with what it has used, oldest first.
func (db *DB) ListInvitations(ctx context.Context) ([]Invitation, error) {
	rows, err := db.Reader.QueryContext(ctx,
		`SELECT i.id, i.label, i.budget_usd, i.spent_usd, i.max_accounts, i.max_anon_projects,
		        (SELECT COUNT(*) FROM account a WHERE a.invitation_id = i.id),
		        (SELECT COUNT(*) FROM project p WHERE p.invitation_id = i.id),
		        COALESCE(i.revoked_at, ''), i.created_at
		 FROM invitation i ORDER BY i.created_at, i.id`)
	if err != nil {
		return nil, fmt.Errorf("list invitations: %w", err)
	}
	defer func() { _ = rows.Close() }()
	out := []Invitation{}
	for rows.Next() {
		var i Invitation
		if err := rows.Scan(&i.ID, &i.Label, &i.BudgetUSD, &i.SpentUSD, &i.MaxAccounts, &i.MaxAnonProjects,
			&i.Accounts, &i.Projects, &i.RevokedAt, &i.CreatedAt); err != nil {
			return nil, fmt.Errorf("list invitations: %w", err)
		}
		out = append(out, i)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list invitations: %w", err)
	}
	return out, nil
}

// RevokeInvitation marks an active link revoked (B3-22 row 2).
func (db *DB) RevokeInvitation(ctx context.Context, id string, now time.Time) error {
	res, err := db.Writer.ExecContext(ctx,
		`UPDATE invitation SET revoked_at = ? WHERE id = ? AND revoked_at IS NULL`, timeText(now), id)
	if err != nil {
		return fmt.Errorf("revoke invitation: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("revoke invitation: %w", err)
	}
	if n == 0 {
		return ErrInvitationNotFound
	}
	return nil
}

// CreateSession stores a session without an account by its cookie's hash.
func (db *DB) CreateSession(ctx context.Context, idHash, invitationID string, expiresAt, now time.Time) error {
	if _, err := db.Writer.ExecContext(ctx,
		`INSERT INTO session (id_hash, invitation_id, expires_at, created_at) VALUES (?, ?, ?, ?)`,
		idHash, invitationID, timeText(expiresAt), timeText(now)); err != nil {
		return fmt.Errorf("create session: %w", err)
	}
	return nil
}

// ValidSession finds the session of a cookie's hash that has not expired.
func (db *DB) ValidSession(ctx context.Context, idHash string, now time.Time) (Session, bool, error) {
	s := Session{IDHash: idHash}
	err := db.Reader.QueryRowContext(ctx,
		`SELECT COALESCE(account_id, ''), invitation_id, expires_at FROM session
		 WHERE id_hash = ? AND expires_at > ?`, idHash, timeText(now)).
		Scan(&s.AccountID, &s.InvitationID, &s.ExpiresAt)
	if errors.Is(err, sql.ErrNoRows) {
		return Session{}, false, nil
	}
	if err != nil {
		return Session{}, false, fmt.Errorf("find session: %w", err)
	}
	return s, true, nil
}

// ownedBy is the one definition of «yours» (B-87): the project belongs to
// the session's account, or, without an account, to the session itself.
// Its parameters are the session's AccountID and IDHash. A project from
// before migration 0005 has neither, and belongs to no one.
const ownedBy = `(account_id = ? OR (account_id IS NULL AND anon_session_hash = ?))`

// ProjectOwned reports whether the project belongs to the session.
func (db *DB) ProjectOwned(ctx context.Context, projectID string, s Session) (bool, error) {
	var one int
	err := db.Reader.QueryRowContext(ctx,
		`SELECT 1 FROM project WHERE id = ? AND `+ownedBy,
		projectID, s.AccountID, s.IDHash).Scan(&one)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("project owner: %w", err)
	}
	return true, nil
}

// TouchSession moves a session's expiry and, without an account, its
// project's, so both last from your last activity (B-88, B3-18 2-nov): at
// most one write an hour, measured on the expiry the session was read with.
// It reports whether it wrote. A session further out than its TTL is left
// alone.
func (db *DB) TouchSession(ctx context.Context, s Session, now time.Time) (bool, error) {
	ttl := AnonSessionTTL
	if s.AccountID != "" {
		ttl = SessionTTL
	}
	next := timeText(now.Add(ttl))
	if s.ExpiresAt >= timeText(now.Add(ttl-touchStep)) {
		return false, nil
	}
	tx, err := db.Writer.BeginTx(ctx, nil)
	if err != nil {
		return false, fmt.Errorf("touch session: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.ExecContext(ctx, `UPDATE session SET expires_at = ? WHERE id_hash = ?`, next, s.IDHash); err != nil {
		return false, fmt.Errorf("touch session: %w", err)
	}
	if s.AccountID == "" {
		if _, err := tx.ExecContext(ctx,
			`UPDATE project SET expires_at = ? WHERE account_id IS NULL AND anon_session_hash = ? AND expires_at IS NOT NULL`,
			next, s.IDHash); err != nil {
			return false, fmt.Errorf("touch project: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return false, fmt.Errorf("touch session: %w", err)
	}
	return true, nil
}

// createRefusal is the error that refuses a new project for the session, or
// nil: with an account, AccountProjectLimit; without one, the rules of B3-18 and
// B3-22 row 2. CreateOwnedProject asks it inside its transaction and CreateRefused
// outside any, so the two answer from one set of rules.
func createRefusal(ctx context.Context, q rowQuerier, s Session) error {
	if s.AccountID != "" {
		var mine int
		var revoked sql.NullString
		if err := q.QueryRowContext(ctx,
			`SELECT (SELECT COUNT(*) FROM project WHERE account_id = ?), i.revoked_at
			 FROM invitation i WHERE i.id = ?`, s.AccountID, s.InvitationID).Scan(&mine, &revoked); err != nil {
			return fmt.Errorf("create project: %w", err)
		}
		// [EL DISEÑO NO LO DICE]: an account of a link revoked later is treated
		// as a session without one below: what exists stays and no new project
		// opens, with the same code.
		if revoked.Valid {
			return ErrLinkFull
		}
		if mine >= AccountProjectLimit {
			return ErrAccountLimit
		}
		return nil
	}
	var mine int
	if err := q.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM project WHERE account_id IS NULL AND anon_session_hash = ?`, s.IDHash).Scan(&mine); err != nil {
		return fmt.Errorf("create project: %w", err)
	}
	if mine > 0 {
		return ErrAccountRequired
	}
	var max, used int
	var revoked sql.NullString
	if err := q.QueryRowContext(ctx,
		`SELECT i.max_anon_projects, i.revoked_at,
		        (SELECT COUNT(*) FROM project p WHERE p.invitation_id = i.id AND p.account_id IS NULL)
		 FROM invitation i WHERE i.id = ?`, s.InvitationID).Scan(&max, &revoked, &used); err != nil {
		return fmt.Errorf("create project: %w", err)
	}
	// [EL DISEÑO NO LO DICE]: max_anon_projects counts the projects that
	// exist, so one the cleanup removed frees its place.
	// [EL DISEÑO NO LO DICE]: what a session from a link revoked later
	// may do. Smallest choice: what exists stays reachable and no new
	// project opens, with the same code as a full link.
	if revoked.Valid || used >= max {
		return ErrLinkFull
	}
	return nil
}

// CreateRefused answers, reading only, what CreateOwnedProject would refuse
// now: the error, or nil (P1, can_create).
func (db *DB) CreateRefused(ctx context.Context, s Session) error {
	return createRefusal(ctx, db.Reader, s)
}

// CreateOwnedProject creates a project for the session, the account's or the
// session's own. The rules are checked and the project created in one
// transaction over the single writer, so two requests at once cannot both pass.
func (db *DB) CreateOwnedProject(ctx context.Context, s Session, name string, now time.Time) (string, error) {
	id, err := newID()
	if err != nil {
		return "", err
	}
	tx, err := db.Writer.BeginTx(ctx, nil)
	if err != nil {
		return "", fmt.Errorf("create project: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	if err := createRefusal(ctx, tx, s); err != nil {
		return "", err
	}
	if s.AccountID != "" {
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO project (id, name, invitation_id, account_id, created_at) VALUES (?, ?, ?, ?, ?)`,
			id, name, s.InvitationID, s.AccountID, timeText(now)); err != nil {
			return "", fmt.Errorf("create project: %w", err)
		}
	} else {
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO project (id, name, invitation_id, anon_session_hash, expires_at, created_at)
			 VALUES (?, ?, ?, ?, ?, ?)`,
			id, name, s.InvitationID, s.IDHash, timeText(now.Add(AnonProjectTTL)), timeText(now)); err != nil {
			return "", fmt.Errorf("create project: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return "", fmt.Errorf("create project: %w", err)
	}
	return id, nil
}

// OwnedProject is one project of «Tus proyectos» (P1).
type OwnedProject struct {
	Project
	// LastActivity is its last event's moment, or its creation with none yet.
	// [EL DISEÑO NO LO DICE]: the row says «último evento» and a new project
	// has none; the smallest choice is its creation.
	LastActivity string
	// NoAccount: it belongs to a session without an account, so it goes with
	// that session (B3-18, 2-nov).
	NoAccount bool
}

// ListOwnedProjects returns the session's projects, newest first (P1).
func (db *DB) ListOwnedProjects(ctx context.Context, s Session) ([]OwnedProject, error) {
	rows, err := db.Reader.QueryContext(ctx,
		`SELECT id, COALESCE(name, ''), created_at,
		        COALESCE((SELECT MAX(e.created_at) FROM event e WHERE e.project_id = project.id), created_at),
		        account_id IS NULL
		 FROM project WHERE `+ownedBy+`
		 ORDER BY created_at DESC, id`, s.AccountID, s.IDHash)
	if err != nil {
		return nil, fmt.Errorf("list projects: %w", err)
	}
	defer func() { _ = rows.Close() }()
	out := []OwnedProject{}
	for rows.Next() {
		var p OwnedProject
		if err := rows.Scan(&p.ID, &p.Name, &p.CreatedAt, &p.LastActivity, &p.NoAccount); err != nil {
			return nil, fmt.Errorf("list projects: %w", err)
		}
		out = append(out, p)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list projects: %w", err)
	}
	return out, nil
}

// SessionNames returns the session's username, empty without an account, and
// its link's label (E.6). Never the link's token nor its hash.
func (db *DB) SessionNames(ctx context.Context, s Session) (username, linkLabel string, err error) {
	if err := db.Reader.QueryRowContext(ctx,
		`SELECT COALESCE((SELECT a.username FROM account a WHERE a.id = ?), ''), i.label
		 FROM invitation i WHERE i.id = ?`, s.AccountID, s.InvitationID).Scan(&username, &linkLabel); err != nil {
		return "", "", fmt.Errorf("session names: %w", err)
	}
	return username, linkLabel, nil
}

// PurgeExpired is the hourly cleanup (B-88): it deletes the projects
// without an account past their expires_at, with all their rows in every
// table with a project_id (read from the schema, so a later table is
// covered), and the expired sessions. It is the one exception to «history
// only grows» (AGENTS rule 6), which the design asks for. A project from
// before migration 0005 has no anon_session_hash and stays (B-87).
// invitation.spent_usd is not touched.
func (db *DB) PurgeExpired(ctx context.Context, now time.Time) (projects, sessions int64, err error) {
	const expired = `SELECT id FROM project WHERE account_id IS NULL AND anon_session_hash IS NOT NULL
		AND expires_at IS NOT NULL AND expires_at <= ?`
	at := timeText(now)
	tx, err := db.Writer.BeginTx(ctx, nil)
	if err != nil {
		return 0, 0, fmt.Errorf("purge: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	// The rows go in any order; the foreign keys are checked at commit.
	if _, err := tx.ExecContext(ctx, `PRAGMA defer_foreign_keys = ON`); err != nil {
		return 0, 0, fmt.Errorf("purge: %w", err)
	}
	tables, err := tablesWithProjectID(ctx, tx)
	if err != nil {
		return 0, 0, err
	}
	for _, table := range tables {
		if _, err := tx.ExecContext(ctx, `DELETE FROM "`+table+`" WHERE project_id IN (`+expired+`)`, at); err != nil {
			return 0, 0, fmt.Errorf("purge %s: %w", table, err)
		}
	}
	res, err := tx.ExecContext(ctx, `DELETE FROM project WHERE id IN (`+expired+`)`, at)
	if err != nil {
		return 0, 0, fmt.Errorf("purge project: %w", err)
	}
	if projects, err = res.RowsAffected(); err != nil {
		return 0, 0, fmt.Errorf("purge project: %w", err)
	}
	res, err = tx.ExecContext(ctx, `DELETE FROM session WHERE expires_at <= ?`, at)
	if err != nil {
		return 0, 0, fmt.Errorf("purge session: %w", err)
	}
	if sessions, err = res.RowsAffected(); err != nil {
		return 0, 0, fmt.Errorf("purge session: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return 0, 0, fmt.Errorf("purge: %w", err)
	}
	return projects, sessions, nil
}

func tablesWithProjectID(ctx context.Context, tx *sql.Tx) ([]string, error) {
	rows, err := tx.QueryContext(ctx, `SELECT m.name FROM sqlite_master m
		JOIN pragma_table_info(m.name) c ON c.name = 'project_id' WHERE m.type = 'table'`)
	if err != nil {
		return nil, fmt.Errorf("purge tables: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var out []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, fmt.Errorf("purge tables: %w", err)
		}
		out = append(out, name)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("purge tables: %w", err)
	}
	return out, nil
}

// LabelJury is the label of the jury's link (§1.7, B-89).
const LabelJury = "jury"
