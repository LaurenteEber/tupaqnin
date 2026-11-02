package server_test

import (
	"context"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/LaurenteEber/tupaqnin/internal/store"
)

const sessionCookie = "__Host-tq_session"

// newLink creates a link in the store and returns its id and token.
func newLink(t *testing.T, db *store.DB, label string, maxAnon int) (string, string) {
	t.Helper()
	token, hash, err := store.NewSecret()
	if err != nil {
		t.Fatalf("NewSecret: %v", err)
	}
	id, err := db.CreateInvitation(context.Background(), store.InvitationInput{
		Label: label, BudgetUSD: 30, MaxAccounts: 10, MaxAnonProjects: maxAnon}, hash, time.Now())
	if err != nil {
		t.Fatalf("CreateInvitation: %v", err)
	}
	return id, token
}

// newSessionCookie creates a session of the link in the store. It lasts
// a year so a test's clock can come back days later.
func newSessionCookie(t *testing.T, db *store.DB, invitationID string) *http.Cookie {
	t.Helper()
	cookie, hash, err := store.NewSecret()
	if err != nil {
		t.Fatalf("NewSecret: %v", err)
	}
	now := time.Now()
	if err := db.CreateSession(context.Background(), hash, invitationID, now.Add(365*24*time.Hour), now); err != nil {
		t.Fatalf("CreateSession: %v", err)
	}
	return &http.Cookie{Name: sessionCookie, Value: cookie}
}

// visitors keeps one visitor per database, so two handlers over the same
// database in a test are the same person.
var visitors sync.Map

// withVisitor is h with the visitor of db: a request that brings no
// session cookie sends that visitor's. The owner check runs as ever.
func withVisitor(t *testing.T, db *store.DB, h http.Handler) http.Handler {
	t.Helper()
	cookie, ok := visitors.Load(db)
	if !ok {
		id, _ := newLink(t, db, "test", 1000)
		cookie = newSessionCookie(t, db, id)
		visitors.Store(db, cookie)
		t.Cleanup(func() { visitors.Delete(db) })
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, err := r.Cookie(sessionCookie); err != nil {
			r.AddCookie(cookie.(*http.Cookie))
		}
		h.ServeHTTP(w, r)
	})
}

// anotherVisitor is h with a visitor of its own, on the same database.
func anotherVisitor(t *testing.T, db *store.DB, h http.Handler) http.Handler {
	t.Helper()
	id, _ := newLink(t, db, "test", 1000)
	cookie := newSessionCookie(t, db, id)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.AddCookie(cookie)
		h.ServeHTTP(w, r)
	})
}
