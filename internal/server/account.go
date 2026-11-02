package server

import (
	"encoding/json"
	"errors"
	"math"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/LaurenteEber/tupaqnin/internal/store"
)

// accountForm is what register and login read. The username and the password
// are never logged (AGENTS rule 9, «No hacer» of E.5).
type accountForm struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// accountAnswer is what register and login answer: the same field the session
// will carry (E.6).
type accountAnswer struct {
	Account string `json:"account"`
}

func readAccountForm(w http.ResponseWriter, r *http.Request) (accountForm, bool) {
	var form accountForm
	if err := json.NewDecoder(r.Body).Decode(&form); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_body")
		return accountForm{}, false
	}
	return form, true
}

// clientIP is the IP the limit counts: only Fly-Client-IP, otherwise the
// connection's address, never X-Forwarded-For (B3-22 row 1). A header that is
// not an IP counts as absent, so a key stays short.
func clientIP(r *http.Request) string {
	if ip := net.ParseIP(strings.TrimSpace(r.Header.Get("Fly-Client-IP"))); ip != nil {
		return ip.String()
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

// register is POST /api/account/register (B3-18, B3-22 rows 2, 4 and 5).
// Only a session from a link and without an account registers. The hash, which
// is slow, runs outside the transaction; the rules are checked before it, so a
// refused register costs no hash, and inside the transaction again.
func (s *Server) register(w http.ResponseWriter, r *http.Request) {
	sess, ok, err := s.sessionOf(r)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal")
		return
	}
	if !ok {
		writeError(w, http.StatusUnauthorized, "no_session")
		return
	}
	if sess.AccountID != "" {
		// [EL DISEÑO NO LO DICE]: the code when an account already signed in registers.
		writeError(w, http.StatusConflict, "already_registered")
		return
	}
	form, ok := readAccountForm(w, r)
	if !ok {
		return
	}
	username, ok := store.NormalizeUsername(form.Username)
	if !ok {
		// [EL DISEÑO NO LO DICE]: the code of a username outside B-91.
		writeError(w, http.StatusBadRequest, "invalid_username")
		return
	}
	if !store.ValidPassword(form.Password) {
		// [EL DISEÑO NO LO DICE]: the code of a password outside B-91.
		writeError(w, http.StatusBadRequest, "invalid_password")
		return
	}
	if err := s.db.RegisterBlocked(r.Context(), sess.InvitationID, username); err != nil {
		writeRegisterRefusal(w, err)
		return
	}
	hash, err := store.HashPassword(form.Password)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal")
		return
	}
	cookie, cookieHash, err := store.NewSecret()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal")
		return
	}
	if err := s.db.RegisterAccount(r.Context(), sess, username, hash, cookieHash, s.now()); err != nil {
		writeRegisterRefusal(w, err)
		return
	}
	setSessionCookie(w, cookie, true)
	writeJSON(w, http.StatusCreated, accountAnswer{Account: username})
}

func writeRegisterRefusal(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, store.ErrLinkAccountsFull):
		writeError(w, http.StatusForbidden, "link_accounts_full")
	case errors.Is(err, store.ErrUsernameTaken):
		writeError(w, http.StatusConflict, "username_taken")
	case errors.Is(err, store.ErrSessionGone):
		writeError(w, http.StatusUnauthorized, "no_session")
	default:
		writeError(w, http.StatusInternalServerError, "internal")
	}
}

// login is POST /api/account/login (B3-22 rows 1, 4 and 5). It asks for no link
// (B-91). An unknown user, a wrong password and a password that could not exist
// answer the same and pay for one comparison: bcrypt reads 72 bytes, so a longer
// password that starts the same must not match. A failure logs nothing of who.
func (s *Server) login(w http.ResponseWriter, r *http.Request) {
	form, ok := readAccountForm(w, r)
	if !ok {
		return
	}
	username, shaped := store.NormalizeUsername(form.Username)
	ip := clientIP(r)
	if wait := s.attempts.Reserve(ip, username, s.now()); wait > 0 {
		w.Header().Set("Retry-After", strconv.Itoa(retryAfterSeconds(wait)))
		// [EL DISEÑO NO LO DICE]: the code of this 429.
		writeError(w, http.StatusTooManyRequests, "too_many_attempts")
		return
	}

	var account store.Account
	found := false
	if shaped {
		var err error
		account, found, err = s.db.AccountByUsername(r.Context(), username)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "internal")
			return
		}
	}
	if !found || !store.ValidPassword(form.Password) {
		store.CompareWithFiller(form.Password)
		writeError(w, http.StatusUnauthorized, "invalid_credentials")
		return
	}
	if !store.PasswordMatches(account.PasswordHash, form.Password) {
		writeError(w, http.StatusUnauthorized, "invalid_credentials")
		return
	}

	cookie, cookieHash, err := store.NewSecret()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal")
		return
	}
	previous := ""
	if c, err := r.Cookie(sessionCookie); err == nil {
		previous = store.HashSecret(c.Value)
	}
	if err := s.db.EnterAccount(r.Context(), account, previous, cookieHash, s.now()); err != nil {
		writeError(w, http.StatusInternalServerError, "internal")
		return
	}
	s.attempts.Clear(ip, username)
	setSessionCookie(w, cookie, true)
	writeJSON(w, http.StatusOK, accountAnswer{Account: username})
}

// logout is POST /api/account/logout (B3-22 row 4): the session goes on the
// server and the cookie in the browser.
// [EL DISEÑO NO LO DICE]: with no valid session it answers the same 204, so
// whatever the browser holds is cleared; with an anonymous session it does the
// same, and the cleanup takes that session's project.
func (s *Server) logout(w http.ResponseWriter, r *http.Request) {
	if c, err := r.Cookie(sessionCookie); err == nil {
		if err := s.db.DeleteSession(r.Context(), store.HashSecret(c.Value)); err != nil {
			writeError(w, http.StatusInternalServerError, "internal")
			return
		}
	}
	clearSessionCookie(w)
	w.WriteHeader(http.StatusNoContent)
}

func retryAfterSeconds(wait time.Duration) int {
	return max(1, int(math.Ceil(wait.Seconds())))
}
