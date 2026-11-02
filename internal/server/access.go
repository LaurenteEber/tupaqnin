package server

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"net/url"
	"path"
	"strings"
	"time"

	"github.com/LaurenteEber/tupaqnin/internal/store"
)

// sessionCookie is the one cookie, with or without an account (B-88).
const sessionCookie = "__Host-tq_session"

// CleanupEvery is how often expired projects and sessions go (B-88).
const CleanupEvery = time.Hour

type sessionKey struct{}

// sessionOf resolves the request's cookie to a valid session. Without a
// database there are no sessions, so every project route stays closed.
func (s *Server) sessionOf(r *http.Request) (store.Session, bool, error) {
	if s.db == nil {
		return store.Session{}, false, nil
	}
	c, err := r.Cookie(sessionCookie)
	if err != nil {
		return store.Session{}, false, nil
	}
	return s.db.ValidSession(r.Context(), store.HashSecret(c.Value), s.now())
}

// projectInPath finds the {projectID} of /api/projects/{projectID} and of
// anything under it (P-113: a route added on the project's own path cannot
// skip the check). It decides on each segment decoded, the way the mux matches them:
// comparing the escaped path would let /api/pro%6Aects/… reach the route
// without meeting the check.
func projectInPath(r *http.Request) (string, bool) {
	segments := strings.Split(r.URL.EscapedPath(), "/")
	if len(segments) < 4 || segments[0] != "" {
		return "", false
	}
	decoded := make([]string, 4)
	for i, segment := range segments[:4] {
		value, err := url.PathUnescape(segment)
		if err != nil {
			value = segment
		}
		decoded[i] = value
	}
	if decoded[1] != "api" || decoded[2] != "projects" {
		return "", false
	}
	// /api/projects/ names no project: it is not a route, and stays as it was.
	if len(segments) == 4 && decoded[3] == "" {
		return "", false
	}
	return decoded[3], true
}

// ownerControl is the one owner check (B-87, B3-22 row 6), in front of
// the mux so no handler repeats it or can forget it: no session answers
// 401 no_session; a project that is not yours answers 404
// project_not_found, the same as one that does not exist. Every request
// with a session counts as activity (B-88). No option, variable or build
// tag turns it off.
func (s *Server) ownerControl(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		projectID, scoped := projectInPath(r)
		if !scoped && r.URL.Path != "/api/projects" {
			next.ServeHTTP(w, r)
			return
		}
		sess, ok, err := s.sessionOf(r)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "internal")
			return
		}
		if !ok {
			writeError(w, http.StatusUnauthorized, "no_session")
			return
		}
		s.touchSession(w, r, sess)
		if scoped {
			owned, err := s.db.ProjectOwned(r.Context(), projectID, sess)
			if err != nil {
				writeError(w, http.StatusInternalServerError, "internal")
				return
			}
			if !owned {
				writeError(w, http.StatusNotFound, "project_not_found")
				return
			}
		}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), sessionKey{}, sess)))
	})
}

// touchSession counts a request as activity (B-88, B3-18 2-nov). With an
// account the browser's cookie moves with it, so it lasts 14 days from the
// last activity; without one it stays a session cookie. A failure is logged
// and the request goes on.
func (s *Server) touchSession(w http.ResponseWriter, r *http.Request, sess store.Session) {
	moved, err := s.db.TouchSession(r.Context(), sess, s.now())
	if err != nil {
		s.logger.Error("session expiry not moved", "error", err.Error())
		return
	}
	if !moved || sess.AccountID == "" {
		return
	}
	if c, err := r.Cookie(sessionCookie); err == nil {
		setSessionCookie(w, c.Value, true)
	}
}

// setSessionCookie writes the one cookie (B3-22 row 4): with an account it
// lasts 14 days; without one it has neither Max-Age nor Expires, so it ends
// when the browser closes (B3-18, 2-nov).
func setSessionCookie(w http.ResponseWriter, value string, withAccount bool) {
	cookie := &http.Cookie{
		Name:     sessionCookie,
		Value:    value,
		Path:     "/",
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteLaxMode,
	}
	if withAccount {
		cookie.MaxAge = int(store.SessionTTL / time.Second)
	}
	http.SetCookie(w, cookie)
}

// redeemInvite is GET /i/{token} (B3-22 row 3): a valid link gives a
// session without an account, or keeps the valid one you have, and
// redirects to an address without the token. Anything else is a 404
// without a cookie.
func (s *Server) redeemInvite(w http.ResponseWriter, r *http.Request) {
	invitationID, ok, err := s.db.ActiveInvitationByTokenHash(r.Context(), store.HashSecret(r.PathValue("token")))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal")
		return
	}
	if !ok {
		http.NotFound(w, r)
		return
	}
	sess, hasSession, err := s.sessionOf(r)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal")
		return
	}
	if hasSession {
		s.touchSession(w, r, sess)
	} else {
		cookie, hash, err := store.NewSecret()
		if err != nil {
			writeError(w, http.StatusInternalServerError, "internal")
			return
		}
		now := s.now()
		if err := s.db.CreateSession(r.Context(), hash, invitationID, now.Add(store.AnonSessionTTL), now); err != nil {
			writeError(w, http.StatusInternalServerError, "internal")
			return
		}
		setSessionCookie(w, cookie, false)
	}
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

// createOwnedProject creates a project for the request's session, and
// answers with the code that refused it, if any (403 link_budget, day_cap,
// disk_full, account_required, link_full; B-89, B3-18, B3-22 row 2).
func (s *Server) createOwnedProject(r *http.Request, name string) (id string, status int, code string) {
	sess, _ := r.Context().Value(sessionKey{}).(store.Session)
	blocked, err := s.createBlocked(r.Context(), sess)
	if err != nil {
		return "", http.StatusInternalServerError, "internal"
	}
	if blocked != "" {
		return "", http.StatusForbidden, blocked
	}
	id, err = s.db.CreateOwnedProject(r.Context(), sess, name, s.now())
	switch {
	case errors.Is(err, store.ErrAccountRequired):
		return "", http.StatusForbidden, "account_required"
	case errors.Is(err, store.ErrLinkFull):
		return "", http.StatusForbidden, "link_full"
	case err != nil:
		return "", http.StatusInternalServerError, "internal"
	}
	return id, http.StatusCreated, ""
}

// RunCleanup removes expired anonymous projects and sessions (B-88), at
// once and then every `every`, until ctx ends. It logs counts only.
func RunCleanup(ctx context.Context, db *store.DB, logger *slog.Logger, every time.Duration, now func() time.Time) {
	purge := func() {
		projects, sessions, err := db.PurgeExpired(ctx, now())
		if err != nil {
			if ctx.Err() == nil {
				logger.Error("cleanup", "error", err.Error())
			}
			return
		}
		logger.Info("cleanup", "projects", projects, "sessions", sessions)
	}
	purge()
	ticker := time.NewTicker(every)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			purge()
		}
	}
}

// loggedPath is the path the request log writes: the route that carries
// a link's token is written as /i/… (B3-22 row 3).
func loggedPath(p string) string {
	if strings.HasPrefix(path.Clean(p), "/i/") {
		return "/i/…"
	}
	return p
}
