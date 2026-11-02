package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"golang.org/x/crypto/bcrypt"
)

const (
	// PasswordCost is bcrypt's cost (B3-22 row 5, §1.7).
	PasswordCost = 12
	// AccountProjectLimit is how many projects an account holds (B3-18, 3
	// [inicial]).
	AccountProjectLimit = 3

	passwordMinChars = 10
	passwordMaxChars = 64
	// passwordMaxBytes is what bcrypt reads: with accents or emoji, 64
	// characters can pass it (B-91).
	passwordMaxBytes = 72

	// fillerPasswordHash is what an unknown user is compared against, so that
	// the answer takes the same time as a wrong password (B-91). Its password
	// was random and was discarded. A constant, so no request pays to build it.
	fillerPasswordHash = "$2a$12$iqZXSRbaID1nGav1s86aduIzfmCR1pV4tgURm4nTU2QrgmXv8YCu2"

	// attemptWindow, attemptsPerIP: 10 failures every 15 minutes (B3-22 row 1).
	attemptWindow = 15 * time.Minute
	attemptsPerIP = 10
	// waitFromFailure and maxWait: from the 3rd failure in a row, 1 s, 2 s,
	// 4 s… up to 30 s (B-91).
	waitFromFailure = 3
	maxWait         = 30 * time.Second
	// maxTrackedAttempts bounds each map, so a stranger cannot grow it without
	// limit. [EL DISEÑO NO LO DICE]: the bound and what leaves when it is full.
	maxTrackedAttempts = 10000
)

var (
	// ErrUsernameTaken is a name another account has (409 username_taken).
	ErrUsernameTaken = errors.New("username taken")
	// ErrLinkAccountsFull is an account past the link's max_accounts, or on a
	// revoked link (403 link_accounts_full).
	ErrLinkAccountsFull = errors.New("link accounts full")
	// ErrSessionGone is a register whose anonymous session no longer exists.
	ErrSessionGone = errors.New("session gone")
	// ErrAccountLimit is a project past AccountProjectLimit (403 account_limit).
	ErrAccountLimit = errors.New("account limit")

	usernameShape = regexp.MustCompile(`^[A-Za-z0-9_.-]{3,32}$`)
)

// Account is what entering reads of an account row.
type Account struct {
	ID           string
	InvitationID string
	PasswordHash string
}

// NormalizeUsername applies B-91: 3 to 32 characters of [a-z0-9_.-], kept in
// lowercase. It is false for anything else.
func NormalizeUsername(username string) (string, bool) {
	if !usernameShape.MatchString(username) {
		return "", false
	}
	return strings.ToLower(username), true
}

// ValidPassword applies B3-22 row 5 and B-91: 10 to 64 characters and at most
// 72 bytes.
func ValidPassword(password string) bool {
	chars := utf8.RuneCountInString(password)
	return chars >= passwordMinChars && chars <= passwordMaxChars && len(password) <= passwordMaxBytes
}

// HashPassword is bcrypt of cost 12.
func HashPassword(password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), PasswordCost)
	if err != nil {
		return "", fmt.Errorf("hash password: %w", err)
	}
	return string(hash), nil
}

// PasswordMatches compares a password against its stored hash.
func PasswordMatches(hash, password string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) == nil
}

// CompareWithFiller spends the time of a comparison and answers nothing.
func CompareWithFiller(password string) {
	_ = bcrypt.CompareHashAndPassword([]byte(fillerPasswordHash), []byte(password))
}

type rowQuerier interface {
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

// registerBlocked is the error that refuses this name on this link, or nil. A
// full link answers first, so it says nothing about the names.
// [EL DISEÑO NO LO DICE]: a revoked link registers no one, with the code of a
// full one.
func registerBlocked(ctx context.Context, q rowQuerier, invitationID, username string) error {
	var max, used int
	var revoked sql.NullString
	if err := q.QueryRowContext(ctx,
		`SELECT i.max_accounts, i.revoked_at,
		        (SELECT COUNT(*) FROM account a WHERE a.invitation_id = i.id)
		 FROM invitation i WHERE i.id = ?`, invitationID).Scan(&max, &revoked, &used); err != nil {
		return fmt.Errorf("register: %w", err)
	}
	if revoked.Valid || used >= max {
		return ErrLinkAccountsFull
	}
	var one int
	err := q.QueryRowContext(ctx, `SELECT 1 FROM account WHERE username = ?`, username).Scan(&one)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("register: %w", err)
	}
	return ErrUsernameTaken
}

// RegisterBlocked answers before the slow hash whether a register would be
// refused. RegisterAccount decides again inside its transaction.
func (db *DB) RegisterBlocked(ctx context.Context, invitationID, username string) error {
	return registerBlocked(ctx, db.Reader, invitationID, username)
}

// RegisterAccount is registering, in one transaction over the single writer,
// so two requests at once cannot both pass max_accounts or take one name: the
// anonymous session goes, the account is created, the session's project passes
// to it and stops expiring, and a new session keeps the link (B3-18, B3-22 row 4).
func (db *DB) RegisterAccount(ctx context.Context, s Session, username, passwordHash, newSessionHash string, now time.Time) error {
	id, err := newID()
	if err != nil {
		return err
	}
	tx, err := db.Writer.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("register: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	res, err := tx.ExecContext(ctx,
		`DELETE FROM session WHERE id_hash = ? AND account_id IS NULL AND expires_at > ?`, s.IDHash, timeText(now))
	if err != nil {
		return fmt.Errorf("register: %w", err)
	}
	gone, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("register: %w", err)
	}
	if gone != 1 {
		return ErrSessionGone
	}
	if err := registerBlocked(ctx, tx, s.InvitationID, username); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO account (id, invitation_id, username, password_hash, created_at) VALUES (?, ?, ?, ?, ?)`,
		id, s.InvitationID, username, passwordHash, timeText(now)); err != nil {
		return fmt.Errorf("register: %w", err)
	}
	if _, err := tx.ExecContext(ctx,
		`UPDATE project SET account_id = ?, anon_session_hash = NULL, expires_at = NULL
		 WHERE account_id IS NULL AND anon_session_hash = ?`, id, s.IDHash); err != nil {
		return fmt.Errorf("register: %w", err)
	}
	if err := insertAccountSession(ctx, tx, newSessionHash, id, s.InvitationID, now); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("register: %w", err)
	}
	return nil
}

func insertAccountSession(ctx context.Context, tx *sql.Tx, idHash, accountID, invitationID string, now time.Time) error {
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO session (id_hash, account_id, invitation_id, expires_at, created_at) VALUES (?, ?, ?, ?, ?)`,
		idHash, accountID, invitationID, timeText(now.Add(SessionTTL)), timeText(now)); err != nil {
		return fmt.Errorf("account session: %w", err)
	}
	return nil
}

// AccountByUsername finds an account; ok is false when there is none.
func (db *DB) AccountByUsername(ctx context.Context, username string) (Account, bool, error) {
	var a Account
	err := db.Reader.QueryRowContext(ctx,
		`SELECT id, invitation_id, password_hash FROM account WHERE username = ?`, username).
		Scan(&a.ID, &a.InvitationID, &a.PasswordHash)
	if errors.Is(err, sql.ErrNoRows) {
		return Account{}, false, nil
	}
	if err != nil {
		return Account{}, false, fmt.Errorf("find account: %w", err)
	}
	return a, true, nil
}

// EnterAccount opens a new session on the account's own link, and ends the one
// the browser came with, if any (B3-22 row 4). previousHash may name nothing.
// [EL DISEÑO NO LO DICE]: a project of that session, if it had no account, is
// not moved: only registering moves it (B3-18), and the cleanup takes it.
func (db *DB) EnterAccount(ctx context.Context, a Account, previousHash, newSessionHash string, now time.Time) error {
	tx, err := db.Writer.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("enter account: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.ExecContext(ctx, `DELETE FROM session WHERE id_hash = ?`, previousHash); err != nil {
		return fmt.Errorf("enter account: %w", err)
	}
	if err := insertAccountSession(ctx, tx, newSessionHash, a.ID, a.InvitationID, now); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("enter account: %w", err)
	}
	return nil
}

// DeleteSession ends a session (B3-22 row 4). A hash that names none is no error.
func (db *DB) DeleteSession(ctx context.Context, idHash string) error {
	if _, err := db.Writer.ExecContext(ctx, `DELETE FROM session WHERE id_hash = ?`, idHash); err != nil {
		return fmt.Errorf("delete session: %w", err)
	}
	return nil
}

// LoginAttempts is the limit per IP and the wait per name (B3-22 row 1, B-91).
// It lives in memory, on one machine, and is never written to the database.
// It holds no password, and logs nothing.
type LoginAttempts struct {
	mu    sync.Mutex
	ips   map[string][]time.Time
	names map[string]*attemptStreak
}

type attemptStreak struct {
	fails int
	last  time.Time
	until time.Time
}

// NewLoginAttempts starts with no attempts.
func NewLoginAttempts() *LoginAttempts {
	return &LoginAttempts{ips: map[string][]time.Time{}, names: map[string]*attemptStreak{}}
}

// Reserve asks whether an attempt may run. It answers the time left when it may
// not, and that attempt counts for nothing. Otherwise it answers 0 and counts
// the attempt as a failure right away, before the comparison, so that many at
// once cannot slip past the limit; Clear gives it back. An empty name has no
// wait: it is not the shape of any account. A name no account has waits the
// same as one that exists, so the wait does not tell who exists.
func (l *LoginAttempts) Reserve(ip, name string, now time.Time) time.Duration {
	l.mu.Lock()
	defer l.mu.Unlock()

	recent := l.ips[ip]
	for len(recent) > 0 && now.Sub(recent[0]) >= attemptWindow {
		recent = recent[1:]
	}
	if len(recent) >= attemptsPerIP {
		return recent[0].Add(attemptWindow).Sub(now)
	}
	var streak *attemptStreak
	if name != "" {
		// [EL DISEÑO NO LO DICE]: a streak is forgotten attemptWindow after its
		// last failure.
		streak = l.names[name]
		if streak != nil && now.Sub(streak.last) >= attemptWindow {
			streak = nil
			delete(l.names, name)
		}
		if streak != nil && now.Before(streak.until) {
			return streak.until.Sub(now)
		}
	}

	if _, known := l.ips[ip]; !known && len(l.ips) >= maxTrackedAttempts {
		evictOne(l.ips, func(times []time.Time) time.Time { return times[len(times)-1] }, now)
	}
	l.ips[ip] = append(recent, now)
	if name == "" {
		return 0
	}
	if streak == nil {
		if len(l.names) >= maxTrackedAttempts {
			evictOne(l.names, func(s *attemptStreak) time.Time { return s.last }, now)
		}
		streak = &attemptStreak{}
		l.names[name] = streak
	}
	streak.fails++
	streak.last = now
	if streak.fails >= waitFromFailure {
		streak.until = now.Add(waitAfter(streak.fails))
	}
	return 0
}

// Clear gives back a reserved attempt that succeeded, and ends the name's streak.
func (l *LoginAttempts) Clear(ip, name string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if times := l.ips[ip]; len(times) > 1 {
		l.ips[ip] = times[:len(times)-1]
	} else {
		delete(l.ips, ip)
	}
	delete(l.names, name)
}

func waitAfter(fails int) time.Duration {
	steps := fails - waitFromFailure
	if steps >= 5 { // 32 s is past maxWait
		return maxWait
	}
	return time.Second << steps
}

// evictOne makes room: every stale entry goes or, if there is none, the oldest.
func evictOne[V any](m map[string]V, last func(V) time.Time, now time.Time) {
	var oldest string
	var oldestAt time.Time
	removed := false
	for key, value := range m {
		at := last(value)
		if now.Sub(at) >= attemptWindow {
			delete(m, key)
			removed = true
			continue
		}
		if oldest == "" || at.Before(oldestAt) {
			oldest, oldestAt = key, at
		}
	}
	if !removed && oldest != "" {
		delete(m, oldest)
	}
}
