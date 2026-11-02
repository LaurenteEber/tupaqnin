package server_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"golang.org/x/crypto/bcrypt"

	"github.com/LaurenteEber/tupaqnin/internal/server"
	"github.com/LaurenteEber/tupaqnin/internal/store"
)

const (
	goodPassword  = "correct-horse-1"
	wrongPassword = "wrong-horse-22"
	accountAge    = "Max-Age=1209600"
)

type credentials struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// post sends one account form, with the headers a test gives it.
func (a *access) post(path string, cookie *http.Cookie, headers map[string]string, username, password string) *httptest.ResponseRecorder {
	body := fmt.Sprintf(`{"username":%q,"password":%q}`, username, password)
	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
	if cookie != nil {
		req.AddCookie(cookie)
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	rec := httptest.NewRecorder()
	a.h.ServeHTTP(rec, req)
	return rec
}

func (a *access) register(cookie *http.Cookie, username, password string) *httptest.ResponseRecorder {
	return a.post("/api/account/register", cookie, nil, username, password)
}

func (a *access) login(ip string, username, password string) *httptest.ResponseRecorder {
	return a.post("/api/account/login", nil, map[string]string{"Fly-Client-IP": ip}, username, password)
}

func newCookie(t *testing.T, rec *httptest.ResponseRecorder) *http.Cookie {
	t.Helper()
	cookies := rec.Result().Cookies()
	if len(cookies) != 1 {
		t.Fatalf("got %d cookies, want 1 (%s)", len(cookies), rec.Body.String())
	}
	return &http.Cookie{Name: cookies[0].Name, Value: cookies[0].Value}
}

// registered enters through the link and registers: it returns the account's
// cookie.
func (a *access) registered(t *testing.T, token, username string) *http.Cookie {
	t.Helper()
	rec := a.register(a.visitor(t, token), username, goodPassword)
	if rec.Code != http.StatusCreated {
		t.Fatalf("register %s: got %d (%s)", username, rec.Code, rec.Body.String())
	}
	return newCookie(t, rec)
}

func wantAccountCookie(t *testing.T, rec *httptest.ResponseRecorder) {
	t.Helper()
	raw := rec.Header().Get("Set-Cookie")
	for _, part := range []string{"__Host-tq_session=", accountAge, "Path=/", "HttpOnly", "Secure", "SameSite=Lax"} {
		if !strings.Contains(raw, part) {
			t.Fatalf("Set-Cookie %q lacks %q", raw, part)
		}
	}
}

func wantNoRows(t *testing.T, a *access, accounts int) {
	t.Helper()
	if got := a.count(t, `SELECT COUNT(*) FROM account`); got != accounts {
		t.Fatalf("accounts: got %d, want %d", got, accounts)
	}
}

func stamp(at time.Time) string { return at.UTC().Format("2006-01-02T15:04:05.000") + "Z" }

func wantRetryAfter(t *testing.T, what string, rec *httptest.ResponseRecorder, seconds int) {
	t.Helper()
	wantCode(t, what, rec, http.StatusTooManyRequests, "too_many_attempts")
	if got := rec.Header().Get("Retry-After"); got != strconv.Itoa(seconds) {
		t.Fatalf("%s: Retry-After %q, want %d", what, got, seconds)
	}
}

// B3-18: the anonymous project moves to the account and stops expiring;
// the anonymous session goes, in the same step, and the new one lasts 14 days.
func TestB3_18_RegisterKeepsAnonProject(t *testing.T) {
	t.Parallel()
	a := newAccess(t)
	linkID, token := newLink(t, a.db, "guests", 5)
	anon := a.visitor(t, token)
	projectID := a.project(t, anon)
	oldHash := store.HashSecret(anon.Value)

	rec := a.register(anon, "  Maria.P_1-  ", goodPassword)
	wantCode(t, "a username with spaces", rec, http.StatusBadRequest, "invalid_username")
	rec = a.register(anon, "Maria.P_1-", goodPassword)
	if rec.Code != http.StatusCreated {
		t.Fatalf("register: got %d (%s)", rec.Code, rec.Body.String())
	}
	if got := decode[map[string]string](t, rec)["account"]; got != "maria.p_1-" {
		t.Fatalf("account in the answer: %q", got)
	}
	wantAccountCookie(t, rec)
	cookie := newCookie(t, rec)
	if cookie.Value == anon.Value {
		t.Fatal("the session did not rotate")
	}

	accountID := a.text(t, `SELECT id FROM account WHERE username = 'maria.p_1-'`)
	if got := a.text(t, `SELECT invitation_id FROM account WHERE id = ?`, accountID); got != linkID {
		t.Fatalf("account's link: %s, want %s", got, linkID)
	}
	if got := a.text(t, `SELECT account_id FROM project WHERE id = ?`, projectID); got != accountID {
		t.Fatalf("project's account: %s, want %s", got, accountID)
	}
	if got := a.count(t, `SELECT COUNT(*) FROM project WHERE id = ? AND anon_session_hash IS NULL AND expires_at IS NULL`, projectID); got != 1 {
		t.Fatal("the project keeps its anonymous hash or its expiry")
	}
	if got := a.count(t, `SELECT COUNT(*) FROM session WHERE id_hash = ?`, oldHash); got != 0 {
		t.Fatal("the anonymous session is still there")
	}
	hash := store.HashSecret(cookie.Value)
	if got := a.text(t, `SELECT account_id FROM session WHERE id_hash = ?`, hash); got != accountID {
		t.Fatalf("session's account: %s, want %s", got, accountID)
	}
	if got := a.text(t, `SELECT invitation_id FROM session WHERE id_hash = ?`, hash); got != linkID {
		t.Fatalf("session's link: %s, want %s", got, linkID)
	}
	if got, want := a.text(t, `SELECT expires_at FROM session WHERE id_hash = ?`, hash), stamp(a.now.Add(store.SessionTTL)); got != want {
		t.Fatalf("session expiry: %s, want %s", got, want)
	}

	// Past the anonymous 24 h and the cleanup, the project is still yours.
	a.now = a.now.Add(store.AnonProjectTTL + 2*time.Hour)
	if _, _, err := a.db.PurgeExpired(context.Background(), a.now); err != nil {
		t.Fatalf("PurgeExpired: %v", err)
	}
	list := decode[[]server.ProjectView](t, a.do(http.MethodGet, "/api/projects", cookie, nil))
	if len(list) != 1 || list[0].ProjectID != projectID {
		t.Fatalf("list: %+v, want %s", list, projectID)
	}
	if rec := a.do(http.MethodGet, "/api/projects/"+projectID+"/sections", cookie, nil); rec.Code != http.StatusOK {
		t.Fatalf("project route: %d (%s)", rec.Code, rec.Body.String())
	}
}

// Solo se registra quien llegó con un link; a signed-in account cannot
// register again, and a taken name writes nothing.
func TestB3_18_RegisterRefusals(t *testing.T) {
	t.Parallel()
	a := newAccess(t)
	_, token := newLink(t, a.db, "guests", 5)

	wantCode(t, "no link", a.register(nil, "nobody", goodPassword), http.StatusUnauthorized, "no_session")
	wantNoRows(t, a, 0)

	for _, name := range []string{"ab", strings.Repeat("a", 33), "has space", "ñandú", "KelvinK", "a/b"} {
		anon := a.visitor(t, token)
		wantCode(t, "username "+name, a.register(anon, name, goodPassword), http.StatusBadRequest, "invalid_username")
	}
	wantNoRows(t, a, 0)

	first := a.registered(t, token, "taken")
	wantCode(t, "an account registers", a.register(first, "second", goodPassword), http.StatusConflict, "already_registered")
	wantNoRows(t, a, 1)

	anon := a.visitor(t, token)
	projectID := a.project(t, anon)
	wantCode(t, "taken name", a.register(anon, "TAKEN", goodPassword), http.StatusConflict, "username_taken")
	wantNoRows(t, a, 1)
	if rec := a.do(http.MethodGet, "/api/projects/"+projectID+"/sections", anon, nil); rec.Code != http.StatusOK {
		t.Fatalf("a refused register cost the visitor its project: %d", rec.Code)
	}
	if got := a.count(t, `SELECT COUNT(*) FROM project WHERE id = ? AND account_id IS NULL AND anon_session_hash IS NOT NULL`, projectID); got != 1 {
		t.Fatal("a refused register moved the project")
	}
}

func TestB3_18_ConcurrentRegistersWithOneName(t *testing.T) {
	t.Parallel()
	a := newAccess(t)
	_, token := newLink(t, a.db, "guests", 5)
	const n = 4
	cookies := make([]*http.Cookie, n)
	for i := range cookies {
		cookies[i] = a.visitor(t, token)
	}
	statuses := make([]int, n)
	var wg sync.WaitGroup
	for i := range cookies {
		wg.Add(1)
		go func() {
			defer wg.Done()
			statuses[i] = a.register(cookies[i], "same", goodPassword).Code
		}()
	}
	wg.Wait()
	created := 0
	for _, status := range statuses {
		switch status {
		case http.StatusCreated:
			created++
		case http.StatusConflict:
		default:
			t.Fatalf("status %d", status)
		}
	}
	if created != 1 {
		t.Fatalf("created %d accounts with one name: %v", created, statuses)
	}
	wantNoRows(t, a, 1)
}

// B3-22 row 4: registering, entering and leaving each leave the previous
// cookie worthless.
func TestB3_22_4_SessionRotates(t *testing.T) {
	t.Parallel()
	a := newAccess(t)
	linkID, token := newLink(t, a.db, "guests", 5)
	otherLinkID, otherToken := newLink(t, a.db, "jury", 5)

	anon := a.visitor(t, token)
	rec := a.register(anon, "rotator", goodPassword)
	if rec.Code != http.StatusCreated {
		t.Fatalf("register: %d (%s)", rec.Code, rec.Body.String())
	}
	registered := newCookie(t, rec)
	wantCode(t, "anonymous cookie after register", a.do(http.MethodGet, "/api/projects", anon, nil), http.StatusUnauthorized, "no_session")
	if rec := a.do(http.MethodGet, "/api/projects", registered, nil); rec.Code != http.StatusOK {
		t.Fatalf("registered cookie: %d", rec.Code)
	}

	// Entering with a cookie of another link: that session goes, and the new
	// one takes the account's link.
	elsewhere := a.visitor(t, otherToken)
	rec = a.post("/api/account/login", elsewhere, nil, "rotator", goodPassword)
	if rec.Code != http.StatusOK {
		t.Fatalf("login: %d (%s)", rec.Code, rec.Body.String())
	}
	if got := decode[map[string]string](t, rec)["account"]; got != "rotator" {
		t.Fatalf("account in the answer: %q", got)
	}
	wantAccountCookie(t, rec)
	entered := newCookie(t, rec)
	if entered.Value == elsewhere.Value || entered.Value == registered.Value {
		t.Fatal("login did not open a new session")
	}
	wantCode(t, "previous cookie after login", a.do(http.MethodGet, "/api/projects", elsewhere, nil), http.StatusUnauthorized, "no_session")
	hash := store.HashSecret(entered.Value)
	if got := a.text(t, `SELECT invitation_id FROM session WHERE id_hash = ?`, hash); got != linkID || got == otherLinkID {
		t.Fatalf("session's link: %s, want the account's %s", got, linkID)
	}
	if got, want := a.text(t, `SELECT expires_at FROM session WHERE id_hash = ?`, hash), stamp(a.now.Add(store.SessionTTL)); got != want {
		t.Fatalf("session expiry: %s, want %s", got, want)
	}

	// Entering with the previous account cookie: it stops working.
	rec = a.post("/api/account/login", entered, nil, "rotator", goodPassword)
	if rec.Code != http.StatusOK {
		t.Fatalf("second login: %d", rec.Code)
	}
	again := newCookie(t, rec)
	wantCode(t, "account cookie after login", a.do(http.MethodGet, "/api/projects", entered, nil), http.StatusUnauthorized, "no_session")

	// A failed login leaves the session as it was.
	if rec := a.post("/api/account/login", again, nil, "rotator", wrongPassword); rec.Code != http.StatusUnauthorized || rec.Header().Get("Set-Cookie") != "" {
		t.Fatalf("failed login: %d, cookie %q", rec.Code, rec.Header().Get("Set-Cookie"))
	}
	if rec := a.do(http.MethodGet, "/api/projects", again, nil); rec.Code != http.StatusOK {
		t.Fatalf("a failed login ended the session: %d", rec.Code)
	}

	// Leaving: the row goes, the cookie is cleared the way the browser honours.
	rec = a.do(http.MethodPost, "/api/account/logout", again, nil)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("logout: %d (%s)", rec.Code, rec.Body.String())
	}
	raw := rec.Header().Get("Set-Cookie")
	for _, part := range []string{"__Host-tq_session=;", "Max-Age=0", "Path=/", "HttpOnly", "Secure", "SameSite=Lax"} {
		if !strings.Contains(raw, part) {
			t.Fatalf("logout's Set-Cookie %q lacks %q", raw, part)
		}
	}
	if got := a.count(t, `SELECT COUNT(*) FROM session WHERE id_hash = ?`, store.HashSecret(again.Value)); got != 0 {
		t.Fatal("logout left the session in the database")
	}
	wantCode(t, "cookie after logout", a.do(http.MethodGet, "/api/projects", again, nil), http.StatusUnauthorized, "no_session")

	// Leaving with no session, or twice, still clears what the browser holds.
	for _, cookie := range []*http.Cookie{nil, again} {
		rec := a.do(http.MethodPost, "/api/account/logout", cookie, nil)
		if rec.Code != http.StatusNoContent || !strings.Contains(rec.Header().Get("Set-Cookie"), "Max-Age=0") {
			t.Fatalf("logout without a session: %d, %q", rec.Code, rec.Header().Get("Set-Cookie"))
		}
	}
	if got := a.count(t, `SELECT COUNT(*) FROM account`); got != 1 {
		t.Fatalf("logout touched the accounts: %d", got)
	}
}

// [EL DISEÑO NO LO DICE]: entering from a browser with an anonymous project.
// The session goes (B3-22 row 4) and the project is not moved: only registering
// moves it (B3-18), so the cleanup takes it when its 24 h pass.
func TestB3_22_4_LoginEndsTheAnonymousSessionAndLeavesItsProjectToTheCleanup(t *testing.T) {
	t.Parallel()
	a := newAccess(t)
	_, token := newLink(t, a.db, "guests", 5)
	owner := a.registered(t, token, "owner")
	for range store.AccountProjectLimit {
		a.project(t, owner)
	}
	anon := a.visitor(t, token)
	projectID := a.project(t, anon)

	rec := a.post("/api/account/login", anon, nil, "owner", goodPassword)
	if rec.Code != http.StatusOK {
		t.Fatalf("login: %d (%s)", rec.Code, rec.Body.String())
	}
	if got := decode[map[string]string](t, rec)["account"]; got != "owner" {
		t.Fatalf("account in the answer: %q", got)
	}
	cookie := newCookie(t, rec)
	if list := decode[[]server.ProjectView](t, a.do(http.MethodGet, "/api/projects", cookie, nil)); len(list) != store.AccountProjectLimit {
		t.Fatalf("projects of the full account: %+v", list)
	}
	if got := a.count(t, `SELECT COUNT(*) FROM project WHERE id = ? AND account_id IS NULL AND expires_at IS NOT NULL`, projectID); got != 1 {
		t.Fatal("the anonymous project changed")
	}
	if got := a.do(http.MethodGet, "/api/session", anon, nil); got.Code != http.StatusUnauthorized {
		t.Fatalf("the anonymous session after login: %d", got.Code)
	}
	a.now = a.now.Add(store.AnonProjectTTL + time.Hour)
	if _, _, err := a.db.PurgeExpired(context.Background(), a.now); err != nil {
		t.Fatalf("PurgeExpired: %v", err)
	}
	if got := a.count(t, `SELECT COUNT(*) FROM project WHERE id = ?`, projectID); got != 0 {
		t.Fatal("the cleanup did not take it")
	}
}

// B-91 (Eber, 2-nov): logging in holding a project without an account moves it
// to the account, as registering does, and the project keeps its link. A wrong
// password moves nothing and ends nothing.
func TestB91_LoginMovesTheAnonymousProjectToTheAccount(t *testing.T) {
	t.Parallel()
	a := newAccess(t)
	linkID, token := newLink(t, a.db, "guests", 5)
	a.registered(t, token, "owner")
	anon := a.visitor(t, token)
	projectID := a.project(t, anon)

	wantCode(t, "wrong password", a.post("/api/account/login", anon, nil, "owner", wrongPassword), http.StatusUnauthorized, "invalid_credentials")
	if got := a.count(t, `SELECT COUNT(*) FROM project WHERE id = ? AND account_id IS NULL AND anon_session_hash IS NOT NULL AND expires_at IS NOT NULL`, projectID); got != 1 {
		t.Fatal("a wrong password changed the project")
	}
	if got := a.do(http.MethodGet, "/api/session", anon, nil); got.Code != http.StatusOK {
		t.Fatalf("the anonymous session after a wrong password: %d", got.Code)
	}

	rec := a.post("/api/account/login", anon, nil, "owner", goodPassword)
	if rec.Code != http.StatusOK {
		t.Fatalf("login: %d (%s)", rec.Code, rec.Body.String())
	}
	wantAccountCookie(t, rec)
	cookie := newCookie(t, rec)
	accountID := a.text(t, `SELECT id FROM account WHERE username = 'owner'`)
	if got := a.count(t, `SELECT COUNT(*) FROM project WHERE id = ? AND account_id = ? AND anon_session_hash IS NULL AND expires_at IS NULL AND invitation_id = ?`, projectID, accountID, linkID); got != 1 {
		t.Fatal("the project did not move to the account in full")
	}
	if list := decode[[]server.ProjectView](t, a.do(http.MethodGet, "/api/projects", cookie, nil)); len(list) != 1 || list[0].ProjectID != projectID {
		t.Fatalf("the account's projects: %+v", list)
	}
	if got := a.do(http.MethodGet, "/api/session", anon, nil); got.Code != http.StatusUnauthorized {
		t.Fatalf("the anonymous session after login: %d", got.Code)
	}
	a.now = a.now.Add(store.AnonProjectTTL + time.Hour)
	if _, _, err := a.db.PurgeExpired(context.Background(), a.now); err != nil {
		t.Fatalf("PurgeExpired: %v", err)
	}
	if got := a.count(t, `SELECT COUNT(*) FROM project WHERE id = ?`, projectID); got != 1 {
		t.Fatal("the cleanup took a project of the account")
	}
}

// B-91: the move asks for a live session, as registering does: a project whose
// session already ended stays for the cleanup.
func TestB91_LoginWithAnExpiredSessionMovesNothing(t *testing.T) {
	t.Parallel()
	a := newAccess(t)
	_, token := newLink(t, a.db, "guests", 5)
	a.registered(t, token, "owner")
	anon := a.visitor(t, token)
	projectID := a.project(t, anon)
	if _, err := a.db.Writer.ExecContext(context.Background(),
		`UPDATE session SET expires_at = ? WHERE id_hash = ?`, stamp(a.now.Add(-time.Second)), store.HashSecret(anon.Value)); err != nil {
		t.Fatal(err)
	}

	if rec := a.post("/api/account/login", anon, nil, "owner", goodPassword); rec.Code != http.StatusOK {
		t.Fatalf("login: %d (%s)", rec.Code, rec.Body.String())
	}
	if got := a.count(t, `SELECT COUNT(*) FROM project WHERE id = ? AND account_id IS NULL AND expires_at IS NOT NULL`, projectID); got != 1 {
		t.Fatal("the project of an expired session moved")
	}
}

// B-91: coming from another account's session moves nothing: its projects stay
// with their account, as before.
func TestB91_LoginFromAnotherAccountKeepsItsProjects(t *testing.T) {
	t.Parallel()
	a := newAccess(t)
	_, token := newLink(t, a.db, "guests", 5)
	a.registered(t, token, "owner")
	first := a.registered(t, token, "first")
	projectID := a.project(t, first)

	if rec := a.post("/api/account/login", first, nil, "owner", goodPassword); rec.Code != http.StatusOK {
		t.Fatalf("login: %d (%s)", rec.Code, rec.Body.String())
	}
	firstID := a.text(t, `SELECT id FROM account WHERE username = 'first'`)
	if got := a.count(t, `SELECT COUNT(*) FROM project WHERE id = ? AND account_id = ?`, projectID, firstID); got != 1 {
		t.Fatal("the other account's project moved")
	}
}

// B3-22 row 5, B-91: 10 to 64 characters and at most 72 bytes; bcrypt of cost 12.
func TestB3_22_5_PasswordRules(t *testing.T) {
	t.Parallel()
	a := newAccess(t)
	_, token := newLink(t, a.db, "guests", 5)
	refused := map[string]string{
		"9 characters":                strings.Repeat("a", 9),
		"65 characters":               strings.Repeat("a", 65),
		"64 characters over 72 bytes": strings.Repeat("é", 64),
		"37 characters of 74 bytes":   strings.Repeat("é", 37),
		"empty":                       "",
	}
	for what, password := range refused {
		anon := a.visitor(t, token)
		wantCode(t, what, a.register(anon, "someone", password), http.StatusBadRequest, "invalid_password")
		if rec := a.do(http.MethodGet, "/api/projects", anon, nil); rec.Code != http.StatusOK {
			t.Fatalf("%s: the session did not survive: %d", what, rec.Code)
		}
	}
	wantNoRows(t, a, 0)

	exact72 := strings.Repeat("é", 36) // 36 characters, exactly 72 bytes
	accepted := map[string]string{
		"ten":   strings.Repeat("a", 10),
		"sixty": strings.Repeat("a", 64),
		"bytes": exact72,
		"tilde": "contraseña segura ñ",
	}
	for username, password := range accepted {
		anon := a.visitor(t, token)
		rec := a.register(anon, username, password)
		if rec.Code != http.StatusCreated {
			t.Fatalf("%s: got %d (%s)", username, rec.Code, rec.Body.String())
		}
		stored := a.text(t, `SELECT password_hash FROM account WHERE username = ?`, username)
		cost, err := bcrypt.Cost([]byte(stored))
		if err != nil {
			t.Fatalf("%s: stored %q is not bcrypt: %v", username, stored, err)
		}
		if cost != 12 {
			t.Fatalf("%s: bcrypt cost %d, want 12", username, cost)
		}
		if strings.Contains(stored, password) {
			t.Fatalf("%s: the password is in the database", username)
		}
		if rec := a.login("198.51.100.5", username, password); rec.Code != http.StatusOK {
			t.Fatalf("%s: login: %d (%s)", username, rec.Code, rec.Body.String())
		}
	}

	// bcrypt only reads 72 bytes: a longer password that starts the same is
	// still another password.
	wantCode(t, "72 bytes and one more", a.login("198.51.100.6", "bytes", exact72+"x"), http.StatusUnauthorized, "invalid_credentials")
}

// B3-22 row 5, B-91: an unknown user, a wrong password, a username that could
// not exist and a password that could not exist answer the same, and all pay
// for one bcrypt comparison.
func TestB3_22_5_SameAnswerForUnknownAndWrong(t *testing.T) {
	t.Parallel()
	a := newAccess(t)
	_, token := newLink(t, a.db, "guests", 5)
	a.registered(t, token, "known")

	wrong := a.login("198.51.100.1", "known", wrongPassword)
	wantCode(t, "wrong password", wrong, http.StatusUnauthorized, "invalid_credentials")
	cases := map[string]credentials{
		"unknown user":   {"ghost", wrongPassword},
		"invalid name":   {"NOT A NAME!", wrongPassword},
		"long password":  {"known", strings.Repeat("a", 2000)},
		"short password": {"known", "x"},
		"empty":          {"", ""},
	}
	for what, c := range cases {
		start := time.Now()
		rec := a.login("198.51.100.2", c.Username, c.Password)
		took := time.Since(start)
		wantCode(t, what, rec, wrong.Code, "invalid_credentials")
		if rec.Body.String() != wrong.Body.String() {
			t.Fatalf("%s: body %q, want %q", what, rec.Body.String(), wrong.Body.String())
		}
		if rec.Header().Get("Set-Cookie") != "" {
			t.Fatalf("%s: a failed login set a cookie", what)
		}
		if took < 20*time.Millisecond {
			t.Fatalf("%s: answered in %v, without a bcrypt comparison", what, took)
		}
	}
	if got := a.count(t, `SELECT COUNT(*) FROM session WHERE account_id IS NOT NULL`); got != 1 {
		t.Fatalf("sessions with an account: %d, want the register's 1", got)
	}
}

// Rule 9, «No hacer» of E.5: no line names a user, a password or a cookie.
func TestB3_22_5_NothingSensitiveIsLogged(t *testing.T) {
	t.Parallel()
	a := newAccess(t)
	_, token := newLink(t, a.db, "guests", 5)
	anon := a.visitor(t, token)
	rec := a.register(anon, "logged-name", goodPassword)
	if rec.Code != http.StatusCreated {
		t.Fatalf("register: %d", rec.Code)
	}
	cookie := newCookie(t, rec)
	a.login("198.51.100.9", "logged-name", wrongPassword)
	a.login("198.51.100.9", "unknown-name", wrongPassword)
	a.login("198.51.100.9", "logged-name", goodPassword)
	a.do(http.MethodPost, "/api/account/logout", cookie, nil)
	stored := a.count(t, `SELECT COUNT(*) FROM account`)
	if stored != 1 {
		t.Fatalf("accounts: %d", stored)
	}
	log := a.log.String()
	if !strings.Contains(log, "/api/account/login") {
		t.Fatalf("the requests were not logged: %s", log)
	}
	for _, secret := range []string{"logged-name", "unknown-name", goodPassword, wrongPassword, cookie.Value, anon.Value, "198.51.100.9", "$2a$"} {
		if strings.Contains(log, secret) {
			t.Fatalf("the log carries %q: %s", secret, log)
		}
	}
}

// B3-22 row 1, B-91: 10 failures from one Fly-Client-IP give 429 on the 11th;
// X-Forwarded-For does not get around it, another IP enters.
func TestB3_22_1_IPLimit(t *testing.T) {
	t.Parallel()
	a := newAccess(t)
	_, token := newLink(t, a.db, "guests", 5)
	a.registered(t, token, "victim")
	const attacker = "203.0.113.7"

	// A success costs nothing.
	for range 2 {
		if rec := a.login(attacker, "victim", goodPassword); rec.Code != http.StatusOK {
			t.Fatalf("success: %d (%s)", rec.Code, rec.Body.String())
		}
	}
	// One name each, so that it is the IP's limit that answers.
	for i := range 10 {
		rec := a.login(attacker, fmt.Sprintf("ghost-%d", i), wrongPassword)
		wantCode(t, fmt.Sprintf("failure %d", i+1), rec, http.StatusUnauthorized, "invalid_credentials")
	}
	wantRetryAfter(t, "11th", a.login(attacker, "victim", goodPassword), 900)
	for _, forged := range []string{"198.51.100.200", attacker, "10.0.0.1, 198.51.100.201"} {
		rec := a.post("/api/account/login", nil, map[string]string{"Fly-Client-IP": attacker, "X-Forwarded-For": forged}, "victim", goodPassword)
		wantRetryAfter(t, "X-Forwarded-For "+forged, rec, 900)
	}

	// Another IP, the right password.
	if rec := a.login("203.0.113.8", "victim", goodPassword); rec.Code != http.StatusOK {
		t.Fatalf("another IP: %d (%s)", rec.Code, rec.Body.String())
	}

	// The window slides: at 15 minutes the failures have aged out.
	a.now = a.now.Add(14 * time.Minute)
	wantRetryAfter(t, "at 14 minutes", a.login(attacker, "victim", goodPassword), 60)
	a.now = a.now.Add(time.Minute)
	if rec := a.login(attacker, "victim", goodPassword); rec.Code != http.StatusOK {
		t.Fatalf("after the window: %d (%s)", rec.Code, rec.Body.String())
	}
}

// B-91: without Fly-Client-IP the limit reads the connection's address, and
// X-Forwarded-For still changes nothing. A header that is no IP counts as absent.
func TestB3_22_1_IPLimitReadsTheConnectionWithoutTheHeader(t *testing.T) {
	t.Parallel()
	a := newAccess(t)
	for i := range 10 {
		headers := map[string]string{"X-Forwarded-For": fmt.Sprintf("198.51.100.%d", i)}
		if i%2 == 1 {
			headers["Fly-Client-IP"] = "not an ip"
		}
		rec := a.post("/api/account/login", nil, headers, fmt.Sprintf("ghost-%d", i), wrongPassword)
		wantCode(t, fmt.Sprintf("failure %d", i+1), rec, http.StatusUnauthorized, "invalid_credentials")
	}
	rec := a.post("/api/account/login", nil, map[string]string{"X-Forwarded-For": "198.51.100.99"}, "ghost-11", wrongPassword)
	wantRetryAfter(t, "11th from the connection", rec, 900)
}

// B3-22 row 1, B-91: the wait grows from the 3rd failure in a row (1, 2, 4… up
// to 30 s) and is never a lock: past it, your password enters. It answers the
// same for a name no account has, so it does not tell who exists.
func TestB3_22_1_NoLockout(t *testing.T) {
	t.Parallel()
	a := newAccess(t)
	_, token := newLink(t, a.db, "guests", 5)
	a.registered(t, token, "victim")

	for i, username := range []string{"victim", "nobody-here"} {
		// One IP per name: the wait is the account's and the IP's.
		ip := fmt.Sprintf("203.0.113.%d", i+1)
		waits := []int{0, 0, 1, 2, 4, 8, 16, 30, 30}
		for j, wait := range waits {
			wantCode(t, fmt.Sprintf("%s failure %d", username, j+1), a.login(ip, username, wrongPassword), http.StatusUnauthorized, "invalid_credentials")
			if wait == 0 {
				continue
			}
			// Even the right password waits, and asking does not make the wait longer.
			wantRetryAfter(t, fmt.Sprintf("%s during wait %d", username, j+1), a.login(ip, username, goodPassword), wait)
			a.now = a.now.Add(time.Duration(wait) * time.Second)
		}
	}

	rec := a.login("203.0.113.1", "victim", goodPassword)
	if rec.Code != http.StatusOK {
		t.Fatalf("past the wait, your password: %d (%s)", rec.Code, rec.Body.String())
	}
	if got := a.count(t, `SELECT COUNT(*) FROM account WHERE username = 'victim'`); got != 1 {
		t.Fatal("the account went away")
	}
	// Entering ends the streak: from an IP that had to wait, two more failures
	// carry no wait.
	const again = "198.51.100.79"
	for i := range 3 {
		wantCode(t, fmt.Sprintf("first streak, failure %d", i+1), a.login(again, "victim", wrongPassword), http.StatusUnauthorized, "invalid_credentials")
	}
	wantRetryAfter(t, "first streak waits", a.login(again, "victim", goodPassword), 1)
	a.now = a.now.Add(time.Second)
	if rec := a.login(again, "victim", goodPassword); rec.Code != http.StatusOK {
		t.Fatalf("past the first streak: %d (%s)", rec.Code, rec.Body.String())
	}
	for i := range 2 {
		wantCode(t, fmt.Sprintf("new failure %d", i+1), a.login(again, "victim", wrongPassword), http.StatusUnauthorized, "invalid_credentials")
	}
	if rec := a.login(again, "victim", goodPassword); rec.Code != http.StatusOK {
		t.Fatalf("a streak of 2 waits: %d (%s)", rec.Code, rec.Body.String())
	}
}

// B-91 (Eber, 2-nov): the wait counts per account and IP, so other IPs cannot
// lock the owner out. Each attacking IP waits for itself, and its wait says
// nothing to the owner's IP or to another account from the same IP.
func TestB91_TheWaitCountsPerAccountAndIP(t *testing.T) {
	t.Parallel()
	a := newAccess(t)
	_, token := newLink(t, a.db, "guests", 5)
	a.registered(t, token, "owner")
	a.registered(t, token, "other")

	for ip := range 4 {
		attacker := fmt.Sprintf("203.0.113.%d", ip+1)
		for failure := range 3 {
			rec := a.login(attacker, "owner", wrongPassword)
			wantCode(t, fmt.Sprintf("%s failure %d", attacker, failure+1), rec, http.StatusUnauthorized, "invalid_credentials")
		}
		wantRetryAfter(t, attacker+" waits", a.login(attacker, "owner", goodPassword), 1)
	}

	if rec := a.login("198.51.100.1", "owner", goodPassword); rec.Code != http.StatusOK {
		t.Fatalf("the owner from another IP: %d (%s)", rec.Code, rec.Body.String())
	}
	// The owner's success ends the owner's streak only: the attacker still waits.
	wantRetryAfter(t, "the attacker after the owner entered", a.login("203.0.113.1", "owner", goodPassword), 1)
	if rec := a.login("203.0.113.1", "other", goodPassword); rec.Code != http.StatusOK {
		t.Fatalf("another account from the waiting IP: %d (%s)", rec.Code, rec.Body.String())
	}
}

// B-91: a name that is not the shape of any account has no wait, only the IP's
// limit.
func TestB91_AMalformedNameHasNoWait(t *testing.T) {
	t.Parallel()
	a := newAccess(t)
	for failure := range 4 {
		rec := a.login("203.0.113.9", "x", wrongPassword)
		wantCode(t, fmt.Sprintf("failure %d", failure+1), rec, http.StatusUnauthorized, "invalid_credentials")
	}
}

// Many attempts at once cannot slip past the limit before the first of them
// has failed: each one takes its place before the comparison.
func TestB3_22_1_ConcurrentAttemptsMeetTheLimit(t *testing.T) {
	t.Parallel()
	a := newAccess(t)
	const n = 20
	statuses := make([]int, n)
	var wg sync.WaitGroup
	for i := range n {
		wg.Add(1)
		go func() {
			defer wg.Done()
			statuses[i] = a.login("203.0.113.50", fmt.Sprintf("ghost-%d", i), wrongPassword).Code
		}()
	}
	wg.Wait()
	counted := map[int]int{}
	for _, status := range statuses {
		counted[status]++
	}
	if counted[http.StatusUnauthorized] != 10 || counted[http.StatusTooManyRequests] != 10 {
		t.Fatalf("got %v, want 10 failures and 10 refused", counted)
	}
}

// B3-18, §1.7: up to 3 projects with an account, checked in one transaction;
// a project with an account spends from the account's link.
func TestB3_18_FourthProjectIsAccountLimit(t *testing.T) {
	t.Parallel()
	a := newAccess(t)
	linkID, token := newLink(t, a.db, "guests", 5)
	anon := a.visitor(t, token)
	moved := a.project(t, anon)
	rec := a.register(anon, "three", goodPassword)
	if rec.Code != http.StatusCreated {
		t.Fatalf("register: %d (%s)", rec.Code, rec.Body.String())
	}
	cookie := newCookie(t, rec)
	second := a.project(t, cookie)
	third := a.project(t, cookie)
	wantCode(t, "4th project", a.do(http.MethodPost, "/api/projects", cookie, nil), http.StatusForbidden, "account_limit")
	wantCode(t, "4th project from the demo", a.do(http.MethodPost, "/api/projects", cookie, map[string]string{"template": "case-5"}), http.StatusForbidden, "account_limit")

	list := decode[[]server.ProjectView](t, a.do(http.MethodGet, "/api/projects", cookie, nil))
	if len(list) != 3 {
		t.Fatalf("list: %+v", list)
	}
	for _, id := range []string{moved, second, third} {
		if got := a.text(t, `SELECT invitation_id FROM project WHERE id = ?`, id); got != linkID {
			t.Fatalf("project %s's link: %s, want %s", id, got, linkID)
		}
		if got := a.count(t, `SELECT COUNT(*) FROM project WHERE id = ? AND expires_at IS NULL`, id); got != 1 {
			t.Fatalf("project %s expires", id)
		}
	}

	// Another account has its own 3.
	other := a.registered(t, token, "other")
	a.project(t, other)
}

func TestB3_18_ConcurrentCreatesStopAtTheAccountLimit(t *testing.T) {
	t.Parallel()
	a := newAccess(t)
	_, token := newLink(t, a.db, "guests", 5)
	cookie := a.registered(t, token, "racer")
	const n = 8
	statuses := make([]int, n)
	var wg sync.WaitGroup
	for i := range n {
		wg.Add(1)
		go func() {
			defer wg.Done()
			statuses[i] = a.do(http.MethodPost, "/api/projects", cookie, nil).Code
		}()
	}
	wg.Wait()
	counted := map[int]int{}
	for _, status := range statuses {
		counted[status]++
	}
	if counted[http.StatusCreated] != 3 || counted[http.StatusForbidden] != n-3 {
		t.Fatalf("got %v, want 3 created and %d refused", counted, n-3)
	}
	if got := a.count(t, `SELECT COUNT(*) FROM project`); got != 3 {
		t.Fatalf("projects: %d, want 3", got)
	}
}

// [EL DISEÑO NO LO DICE]: an account of a link revoked later keeps what it has
// and opens nothing new, the same as a session without an account (E.1).
func TestB3_22_2_RevokedLinkOpensNoNewProjectForAnAccount(t *testing.T) {
	t.Parallel()
	a := newAccess(t)
	linkID, token := newLink(t, a.db, "guests", 5)
	cookie := a.registered(t, token, "revoked")
	projectID := a.project(t, cookie)
	if err := a.db.RevokeInvitation(context.Background(), linkID, a.now); err != nil {
		t.Fatalf("RevokeInvitation: %v", err)
	}
	wantCode(t, "new project", a.do(http.MethodPost, "/api/projects", cookie, nil), http.StatusForbidden, "link_full")
	if rec := a.do(http.MethodGet, "/api/projects/"+projectID+"/sections", cookie, nil); rec.Code != http.StatusOK {
		t.Fatalf("what exists stays: %d", rec.Code)
	}
	if rec := a.login("198.51.100.3", "revoked", goodPassword); rec.Code != http.StatusOK {
		t.Fatalf("entering asks for no link: %d (%s)", rec.Code, rec.Body.String())
	}
}

func accountsLink(t *testing.T, a *access, maxAccounts int) (string, string) {
	t.Helper()
	token, hash, err := store.NewSecret()
	if err != nil {
		t.Fatalf("NewSecret: %v", err)
	}
	id, err := a.db.CreateInvitation(context.Background(), store.InvitationInput{
		Label: "guests", BudgetUSD: 10, MaxAccounts: maxAccounts, MaxAnonProjects: 10}, hash, a.now)
	if err != nil {
		t.Fatalf("CreateInvitation: %v", err)
	}
	return id, token
}

// B3-22 row 2: the 4th account of a link with --accounts 3 is refused, and the
// visitor keeps its session and project. Another link is not affected.
func TestB3_22_2_FourthAccountIsLinkAccountsFull(t *testing.T) {
	t.Parallel()
	a := newAccess(t)
	_, token := accountsLink(t, a, 3)
	for i := range 3 {
		a.registered(t, token, fmt.Sprintf("user-%d", i))
	}
	anon := a.visitor(t, token)
	projectID := a.project(t, anon)
	wantCode(t, "4th account", a.register(anon, "user-4", goodPassword), http.StatusForbidden, "link_accounts_full")
	wantNoRows(t, a, 3)
	if rec := a.do(http.MethodGet, "/api/projects/"+projectID+"/sections", anon, nil); rec.Code != http.StatusOK {
		t.Fatalf("the visitor lost its project: %d", rec.Code)
	}
	// Full comes before taken: a full link answers nothing about names.
	wantCode(t, "a taken name on a full link", a.register(anon, "user-0", goodPassword), http.StatusForbidden, "link_accounts_full")

	_, otherToken := accountsLink(t, a, 1)
	a.registered(t, otherToken, "elsewhere")
}

func TestB3_22_2_ConcurrentRegistersStopAtMaxAccounts(t *testing.T) {
	t.Parallel()
	a := newAccess(t)
	_, token := accountsLink(t, a, 1)
	const n = 4
	cookies := make([]*http.Cookie, n)
	for i := range cookies {
		cookies[i] = a.visitor(t, token)
	}
	statuses := make([]int, n)
	var wg sync.WaitGroup
	for i := range n {
		wg.Add(1)
		go func() {
			defer wg.Done()
			statuses[i] = a.register(cookies[i], fmt.Sprintf("user-%d", i), goodPassword).Code
		}()
	}
	wg.Wait()
	counted := map[int]int{}
	for _, status := range statuses {
		counted[status]++
	}
	if counted[http.StatusCreated] != 1 || counted[http.StatusForbidden] != n-1 {
		t.Fatalf("got %v, want 1 created and %d refused", counted, n-1)
	}
	wantNoRows(t, a, 1)
}

// [EL DISEÑO NO LO DICE]: a revoked link registers no one, with the same code
// as a full one.
func TestB3_22_2_RevokedLinkRegistersNoOne(t *testing.T) {
	t.Parallel()
	a := newAccess(t)
	linkID, token := newLink(t, a.db, "guests", 5)
	anon := a.visitor(t, token)
	if err := a.db.RevokeInvitation(context.Background(), linkID, a.now); err != nil {
		t.Fatalf("RevokeInvitation: %v", err)
	}
	wantCode(t, "revoked", a.register(anon, "late", goodPassword), http.StatusForbidden, "link_accounts_full")
	wantNoRows(t, a, 0)
}

func TestB3_18_ConcurrentRegistersWithOneCookieMakeOneAccount(t *testing.T) {
	t.Parallel()
	a := newAccess(t)
	_, token := newLink(t, a.db, "guests", 5)
	anon := a.visitor(t, token)
	const n = 3
	statuses := make([]int, n)
	var wg sync.WaitGroup
	for i := range n {
		wg.Add(1)
		go func() {
			defer wg.Done()
			statuses[i] = a.register(anon, fmt.Sprintf("user-%d", i), goodPassword).Code
		}()
	}
	wg.Wait()
	counted := map[int]int{}
	for _, status := range statuses {
		counted[status]++
	}
	if counted[http.StatusCreated] != 1 || counted[http.StatusUnauthorized] != n-1 {
		t.Fatalf("got %v, want 1 created and %d without a session", counted, n-1)
	}
	wantNoRows(t, a, 1)
}

// B3-18: entering asks for no link, and shows your projects from any browser.
func TestB3_18_LoginWithoutLinkShowsYourProjects(t *testing.T) {
	t.Parallel()
	a := newAccess(t)
	_, token := newLink(t, a.db, "guests", 5)
	anon := a.visitor(t, token)
	projectID := a.project(t, anon)
	rec := a.register(anon, "traveller", goodPassword)
	if rec.Code != http.StatusCreated {
		t.Fatalf("register: %d", rec.Code)
	}

	rec = a.post("/api/account/login", nil, nil, "TRAVELLER", goodPassword)
	if rec.Code != http.StatusOK {
		t.Fatalf("login: %d (%s)", rec.Code, rec.Body.String())
	}
	cookie := newCookie(t, rec)
	list := decode[[]server.ProjectView](t, a.do(http.MethodGet, "/api/projects", cookie, nil))
	if len(list) != 1 || list[0].ProjectID != projectID {
		t.Fatalf("list: %+v, want %s", list, projectID)
	}
	if rec := a.do(http.MethodGet, "/api/projects/"+projectID+"/sections", cookie, nil); rec.Code != http.StatusOK {
		t.Fatalf("project route: %d (%s)", rec.Code, rec.Body.String())
	}

	// Another account sees none of it.
	other := a.registered(t, token, "stranger")
	if list := decode[[]server.ProjectView](t, a.do(http.MethodGet, "/api/projects", other, nil)); len(list) != 0 {
		t.Fatalf("another account's list: %+v", list)
	}
	wantCode(t, "another account's project", a.do(http.MethodGet, "/api/projects/"+projectID+"/sections", other, nil), http.StatusNotFound, "project_not_found")
}

// The account's routes are in the one table, and take a stranger's small body:
// a form fits, and more is refused before it is read.
func TestB3_22_10_AccountFormsFitTheStrangerCap(t *testing.T) {
	t.Parallel()
	a := newAccess(t)
	table := strings.Join(server.RouteTableForTest(), "\n")
	for _, row := range []string{"POST /api/account/register", "POST /api/account/login", "POST /api/account/logout"} {
		if !strings.Contains(table, row) {
			t.Fatalf("%s is not in the route table", row)
		}
	}
	rec := a.post("/api/account/login", nil, nil, "someone", strings.Repeat("a", 5<<10))
	wantCode(t, "over the cap", rec, http.StatusRequestEntityTooLarge, "body_too_large")
	req := httptest.NewRequest(http.MethodPost, "/api/account/login", strings.NewReader("not json"))
	bad := httptest.NewRecorder()
	a.h.ServeHTTP(bad, req)
	wantCode(t, "not json", bad, http.StatusBadRequest, "invalid_body")
}
