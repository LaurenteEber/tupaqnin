package server_test

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/LaurenteEber/tupaqnin/internal/cycle"
	"github.com/LaurenteEber/tupaqnin/internal/provider"
	"github.com/LaurenteEber/tupaqnin/internal/server"
	"github.com/LaurenteEber/tupaqnin/internal/store"
)

// access is a server with no visitor of its own: each request brings
// the cookie the test gives it, or none.
type access struct {
	h   http.Handler
	db  *store.DB
	dir string
	hub *server.Hub
	log *bytes.Buffer
	now time.Time
}

func newAccess(t *testing.T) *access {
	t.Helper()
	dir := t.TempDir()
	db, err := store.Open(context.Background(), dir)
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	a := &access{db: db, dir: dir, hub: server.NewHub(), log: &bytes.Buffer{},
		now: time.Date(2026, 11, 2, 12, 0, 0, 0, time.UTC)}
	fake := provider.NewFake()
	count := func(ctx context.Context, input []json.RawMessage) (int, error) {
		return fake.CountTokens(ctx, provider.PrincipalModel, input)
	}
	runner := cycle.NewRunner(db, fake, count, a.hub)
	t.Cleanup(runner.Close)
	a.h = server.NewWithAPI(slog.New(slog.NewJSONHandler(a.log, nil)), a.hub, db, runner,
		server.WithClock(func() time.Time { return a.now }))
	return a
}

func (a *access) do(method, path string, cookie *http.Cookie, body any) *httptest.ResponseRecorder {
	var raw bytes.Buffer
	if body != nil {
		_ = json.NewEncoder(&raw).Encode(body)
	}
	req := httptest.NewRequest(method, path, &raw)
	if cookie != nil {
		req.AddCookie(cookie)
	}
	rec := httptest.NewRecorder()
	a.h.ServeHTTP(rec, req)
	return rec
}

// visitor enters through the link, as a browser does, and returns the
// cookie the link gave.
func (a *access) visitor(t *testing.T, token string) *http.Cookie {
	t.Helper()
	rec := a.do(http.MethodGet, "/i/"+token, nil, nil)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("redeem: got %d (%s), want 303", rec.Code, rec.Body.String())
	}
	cookies := rec.Result().Cookies()
	if len(cookies) != 1 {
		t.Fatalf("redeem: got %d cookies, want 1", len(cookies))
	}
	return &http.Cookie{Name: cookies[0].Name, Value: cookies[0].Value}
}

func (a *access) project(t *testing.T, cookie *http.Cookie) string {
	t.Helper()
	rec := a.do(http.MethodPost, "/api/projects", cookie, nil)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create project: got %d (%s)", rec.Code, rec.Body.String())
	}
	return decode[server.CreateProjectResponse](t, rec).ProjectID
}

func (a *access) count(t *testing.T, query string, args ...any) int {
	t.Helper()
	var n int
	if err := a.db.Reader.QueryRowContext(context.Background(), query, args...).Scan(&n); err != nil {
		t.Fatalf("count %q: %v", query, err)
	}
	return n
}

func (a *access) text(t *testing.T, query string, args ...any) string {
	t.Helper()
	var s string
	if err := a.db.Reader.QueryRowContext(context.Background(), query, args...).Scan(&s); err != nil {
		t.Fatalf("text %q: %v", query, err)
	}
	return s
}

func wantCode(t *testing.T, what string, rec *httptest.ResponseRecorder, status int, code string) {
	t.Helper()
	if rec.Code != status {
		t.Fatalf("%s: got %d (%s), want %d", what, rec.Code, rec.Body.String(), status)
	}
	if got := decode[struct {
		Error string `json:"error"`
	}](t, rec).Error; got != code {
		t.Fatalf("%s: got code %q, want %q", what, got, code)
	}
}

// accountCookie is another account's session: its row is written here
// because registering arrives with E.5.
func accountCookie(t *testing.T, a *access, invitationID string) *http.Cookie {
	t.Helper()
	cookie, hash, err := store.NewSecret()
	if err != nil {
		t.Fatalf("NewSecret: %v", err)
	}
	ctx := context.Background()
	if _, err := a.db.Writer.ExecContext(ctx, `INSERT INTO account (id, invitation_id, username, password_hash, created_at)
		VALUES ('acc-other', ?, 'other', 'x', '2026-11-01T00:00:00.000Z')`, invitationID); err != nil {
		t.Fatalf("seed account: %v", err)
	}
	if _, err := a.db.Writer.ExecContext(ctx, `INSERT INTO session (id_hash, account_id, invitation_id, expires_at, created_at)
		VALUES (?, 'acc-other', ?, '2027-11-01T00:00:00.000Z', '2026-11-01T00:00:00.000Z')`, hash, invitationID); err != nil {
		t.Fatalf("seed session: %v", err)
	}
	return &http.Cookie{Name: sessionCookie, Value: cookie}
}

// B3-22 row 6, B-87: every route with {projectID}, with another
// visitor's cookie and with another account's, answers 404 and writes
// nothing; without a cookie, 401.
func TestB3_22_6_EveryProjectRouteChecksOwner(t *testing.T) {
	a := newAccess(t)
	linkID, token := newLink(t, a.db, "jury", 10)
	owner := a.visitor(t, token)
	other := a.visitor(t, token)
	otherAccount := accountCookie(t, a, linkID)
	projectID := a.project(t, owner)
	if rec := a.do(http.MethodPost, "/api/projects/"+projectID+"/sections", owner, map[string]any{"title": "Alcance"}); rec.Code != http.StatusCreated {
		t.Fatalf("seed section: %d (%s)", rec.Code, rec.Body.String())
	}
	expires := a.text(t, `SELECT expires_at FROM project WHERE id = ?`, projectID)
	events := a.count(t, `SELECT COUNT(*) FROM event`)
	calls := a.count(t, `SELECT COUNT(*) FROM model_call`)
	projects := a.count(t, `SELECT COUNT(*) FROM project`)

	walked := 0
	a.now = a.now.Add(3 * 24 * time.Hour) // the owner's expiry would move
	for _, row := range server.RouteTableForTest() {
		method, pattern, _ := strings.Cut(row, " ")
		if !strings.Contains(pattern, "{projectID}") {
			continue
		}
		walked++
		path := strings.NewReplacer("{projectID}", projectID, "{label}", "sec-1", "{n}", "1", "{turnID}", "turn-1").Replace(pattern)
		wantCode(t, "other visitor "+row, a.do(method, path, other, nil), http.StatusNotFound, "project_not_found")
		wantCode(t, "other account "+row, a.do(method, path, otherAccount, nil), http.StatusNotFound, "project_not_found")
		wantCode(t, "no cookie "+row, a.do(method, path, nil, nil), http.StatusUnauthorized, "no_session")
		// The same answer as a project that does not exist.
		nope := strings.Replace(path, projectID, "does-not-exist", 1)
		wantCode(t, "unknown project "+row, a.do(method, nope, other, nil), http.StatusNotFound, "project_not_found")
	}
	if walked < 50 {
		t.Fatalf("walked %d routes with {projectID}, want the whole table", walked)
	}
	if got := a.count(t, `SELECT COUNT(*) FROM event`); got != events {
		t.Fatalf("events: got %d, want %d", got, events)
	}
	if got := a.count(t, `SELECT COUNT(*) FROM model_call`); got != calls {
		t.Fatalf("model_call: got %d, want %d", got, calls)
	}
	if got := a.count(t, `SELECT COUNT(*) FROM project`); got != projects {
		t.Fatalf("projects: got %d, want %d", got, projects)
	}
	if got := a.text(t, `SELECT expires_at FROM project WHERE id = ?`, projectID); got != expires {
		t.Fatalf("another's request moved the expiry: %s → %s", expires, got)
	}
}

// B3-22 row 6: the stream of another's turn is 404; yours opens.
func TestB3_22_6_StreamOfAnotherVisitorIs404(t *testing.T) {
	a := newAccess(t)
	_, token := newLink(t, a.db, "jury", 10)
	owner := a.visitor(t, token)
	other := a.visitor(t, token)
	projectID := a.project(t, owner)
	a.hub.BeginTurn(projectID, "t1")
	a.hub.PublishError(projectID, "t1", "provider_error")
	path := "/api/projects/" + projectID + "/turns/t1/stream"

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	req := httptest.NewRequest(http.MethodGet, path, nil).WithContext(ctx)
	req.AddCookie(other)
	rec := httptest.NewRecorder()
	a.h.ServeHTTP(rec, req)
	wantCode(t, "other's stream", rec, http.StatusNotFound, "project_not_found")
	if strings.Contains(rec.Body.String(), "event:") {
		t.Fatalf("other's stream sent events: %s", rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodGet, path, nil).WithContext(ctx)
	req.AddCookie(owner)
	rec = httptest.NewRecorder()
	a.h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "event: error") {
		t.Fatalf("own stream: %d (%s)", rec.Code, rec.Body.String())
	}
}

// B-87: a path written another way does not reach another's project.
func TestB3_22_6_RewrittenPathDoesNotReachAnotherProject(t *testing.T) {
	a := newAccess(t)
	_, token := newLink(t, a.db, "jury", 10)
	owner := a.visitor(t, token)
	attacker := a.visitor(t, token)
	victim := a.project(t, owner)
	if rec := a.do(http.MethodPost, "/api/projects/"+victim+"/sections", owner, map[string]any{"title": "Secreto"}); rec.Code != http.StatusCreated {
		t.Fatalf("seed: %d", rec.Code)
	}
	mine := a.project(t, attacker)
	for _, path := range []string{
		"/api/projects/" + mine + "/../" + victim + "/sections",
		"/api/projects/" + mine + "/%2e%2e/" + victim + "/sections",
		"/api/projects/" + mine + "%2F..%2F" + victim + "/sections",
		"//api/projects/" + victim + "/sections",
		"/api//projects/" + victim + "/sections",
		"/./api/projects/" + victim + "/sections",
		"/api/projects/" + victim + "%2F/sections",
		"/api/projects/%2e/" + victim + "/sections",
	} {
		rec := a.do(http.MethodGet, path, attacker, nil)
		if rec.Code == http.StatusOK || strings.Contains(rec.Body.String(), "Secreto") {
			t.Errorf("%s: got %d (%s)", path, rec.Code, rec.Body.String())
		}
	}
}

// B3-22 row 6: the mux decodes each segment before it matches, so a prefix
// written with percent escapes reaches the same route and has to meet the
// same check, on every route of the table.
func TestB3_22_6_EncodedPrefixMeetsTheOwnerCheck(t *testing.T) {
	a := newAccess(t)
	_, token := newLink(t, a.db, "jury", 10)
	owner := a.visitor(t, token)
	other := a.visitor(t, token)
	projectID := a.project(t, owner)
	events := a.count(t, `SELECT COUNT(*) FROM event`)
	prefixes := []string{"/api/pro%6Aects/", "/%61pi/projects/", "/api/project%73/", "/%61%70%69/%70rojects/"}
	for _, row := range server.RouteTableForTest() {
		method, pattern, _ := strings.Cut(row, " ")
		if !strings.Contains(pattern, "{projectID}") {
			continue
		}
		rest := strings.TrimPrefix(pattern, "/api/projects/")
		rest = strings.NewReplacer("{projectID}", projectID, "{label}", "sec-1", "{n}", "1", "{turnID}", "turn-1").Replace(rest)
		for _, prefix := range prefixes {
			wantCode(t, "other visitor "+prefix+rest, a.do(method, prefix+rest, other, nil), http.StatusNotFound, "project_not_found")
			wantCode(t, "no cookie "+prefix+rest, a.do(method, prefix+rest, nil, nil), http.StatusUnauthorized, "no_session")
		}
	}
	if got := a.count(t, `SELECT COUNT(*) FROM event`); got != events {
		t.Fatalf("events: got %d, want %d", got, events)
	}
	// The owner still reaches the project through the same spelling.
	if rec := a.do(http.MethodGet, "/api/pro%6Aects/"+projectID+"/sections", owner, nil); rec.Code != http.StatusOK {
		t.Fatalf("owner: got %d (%s)", rec.Code, rec.Body.String())
	}
}

// B3-22 row 3: the token is exchanged for a cookie, the address has no
// token, and the log line does not have it either.
func TestB3_22_3_InviteRedeemed(t *testing.T) {
	a := newAccess(t)
	linkID, token := newLink(t, a.db, "jury", 10)
	rec := a.do(http.MethodGet, "/i/"+token, nil, nil)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("redeem: got %d, want 303", rec.Code)
	}
	if got := rec.Header().Get("Location"); got != "/" {
		t.Fatalf("Location: %q, want /", got)
	}
	if strings.Contains(rec.Body.String(), token) {
		t.Fatalf("the answer carries the token: %s", rec.Body.String())
	}
	raw := rec.Header().Get("Set-Cookie")
	for _, want := range []string{"__Host-tq_session=", "Path=/", "HttpOnly", "Secure", "SameSite=Lax", "Max-Age=1209600"} {
		if !strings.Contains(raw, want) {
			t.Fatalf("Set-Cookie %q lacks %q", raw, want)
		}
	}
	if strings.Contains(raw, "Domain=") {
		t.Fatalf("Set-Cookie %q has a Domain, which __Host- forbids", raw)
	}
	cookies := rec.Result().Cookies()
	if value, err := base64.RawURLEncoding.DecodeString(cookies[0].Value); err != nil || len(value) != 32 {
		t.Fatalf("cookie value: %d bytes, %v; want 32 in base64url", len(value), err)
	}
	if strings.Contains(raw, token) {
		t.Fatal("the cookie is the token")
	}
	if got := a.count(t, `SELECT COUNT(*) FROM session WHERE invitation_id = ? AND account_id IS NULL
		AND id_hash = ? AND expires_at = ?`, linkID, store.HashSecret(cookies[0].Value),
		a.now.Add(14*24*time.Hour).Format("2006-01-02T15:04:05.000")+"Z"); got != 1 {
		t.Fatalf("session rows: %d, want 1 lasting 14 days", got)
	}
	log := a.log.String()
	if strings.Contains(log, token) {
		t.Fatalf("the log has the token: %s", log)
	}
	if !strings.Contains(log, `"path":"/i/…"`) || !strings.Contains(log, `"status":303`) {
		t.Fatalf("the log does not write the route as /i/…: %s", log)
	}
	// The cookie opens the API.
	if rec := a.do(http.MethodGet, "/api/projects", &http.Cookie{Name: sessionCookie, Value: cookies[0].Value}, nil); rec.Code != http.StatusOK {
		t.Fatalf("list with the cookie: %d", rec.Code)
	}
}

// B3-22 row 3: an unknown or revoked link is a 404 without a cookie,
// also in the log the token does not appear.
func TestB3_22_3_RevokedOrUnknownLinkIs404WithoutCookie(t *testing.T) {
	a := newAccess(t)
	linkID, token := newLink(t, a.db, "jury", 10)
	if err := a.db.RevokeInvitation(context.Background(), linkID, a.now); err != nil {
		t.Fatalf("RevokeInvitation: %v", err)
	}
	for name, tok := range map[string]string{"revoked": token, "unknown": "no-such-token"} {
		rec := a.do(http.MethodGet, "/i/"+tok, nil, nil)
		if rec.Code != http.StatusNotFound {
			t.Fatalf("%s: got %d, want 404", name, rec.Code)
		}
		if got := rec.Header().Get("Set-Cookie"); got != "" {
			t.Fatalf("%s: Set-Cookie %q", name, got)
		}
		if strings.Contains(a.log.String(), tok) {
			t.Fatalf("%s: token in the log: %s", name, a.log.String())
		}
	}
	if got := a.count(t, `SELECT COUNT(*) FROM session`); got != 0 {
		t.Fatalf("sessions: %d, want 0", got)
	}
}

// E.1: with a valid session the link keeps it; an expired one is
// replaced.
func TestB3_22_3_ValidSessionIsKept(t *testing.T) {
	a := newAccess(t)
	_, token := newLink(t, a.db, "jury", 10)
	first := a.visitor(t, token)
	rec := a.do(http.MethodGet, "/i/"+token, first, nil)
	if rec.Code != http.StatusSeeOther || rec.Header().Get("Set-Cookie") != "" {
		t.Fatalf("with a valid session: %d, Set-Cookie %q", rec.Code, rec.Header().Get("Set-Cookie"))
	}
	if got := a.count(t, `SELECT COUNT(*) FROM session`); got != 1 {
		t.Fatalf("sessions: %d, want 1", got)
	}
	a.now = a.now.Add(14*24*time.Hour + time.Minute)
	wantCode(t, "expired session", a.do(http.MethodGet, "/api/projects", first, nil), http.StatusUnauthorized, "no_session")
	rec = a.do(http.MethodGet, "/i/"+token, first, nil)
	if rec.Code != http.StatusSeeOther || rec.Header().Get("Set-Cookie") == "" {
		t.Fatalf("with an expired session: %d, Set-Cookie %q", rec.Code, rec.Header().Get("Set-Cookie"))
	}
}

// B3-22 row 4: neither the token nor the cookie is in the database,
// read as stored on disk; their hashes are.
func TestB3_22_4_OnlyHashesStored(t *testing.T) {
	a := newAccess(t)
	linkID, token := newLink(t, a.db, "jury", 10)
	cookie := a.visitor(t, token)
	projectID := a.project(t, cookie)

	if _, err := a.db.Writer.ExecContext(context.Background(), `PRAGMA wal_checkpoint(FULL)`); err != nil {
		t.Fatalf("checkpoint: %v", err)
	}
	files, err := filepath.Glob(filepath.Join(a.dir, "*"))
	if err != nil || len(files) == 0 {
		t.Fatalf("database files: %v, %v", files, err)
	}
	for _, f := range files {
		raw, err := os.ReadFile(f)
		if err != nil {
			t.Fatalf("read %s: %v", f, err)
		}
		if bytes.Contains(raw, []byte(token)) || bytes.Contains(raw, []byte(cookie.Value)) {
			t.Fatalf("%s holds a secret in clear", f)
		}
	}
	if got := a.text(t, `SELECT token_hash FROM invitation WHERE id = ?`, linkID); got != store.HashSecret(token) {
		t.Fatalf("token_hash: %q", got)
	}
	if got := a.text(t, `SELECT id_hash FROM session`); got != store.HashSecret(cookie.Value) {
		t.Fatalf("id_hash: %q", got)
	}
	if got := a.text(t, `SELECT anon_session_hash FROM project WHERE id = ?`, projectID); got != store.HashSecret(cookie.Value) {
		t.Fatalf("anon_session_hash: %q", got)
	}
	if strings.Contains(a.log.String(), cookie.Value) {
		t.Fatalf("the log has the cookie: %s", a.log.String())
	}
}

// B3-18: without an account, one project per browser; what is created
// keeps its link, its session's hash and its expiry.
func TestB3_18_SecondAnonProjectIsAccountRequired(t *testing.T) {
	a := newAccess(t)
	linkID, token := newLink(t, a.db, "jury", 10)
	cookie := a.visitor(t, token)
	wantCode(t, "no cookie", a.do(http.MethodPost, "/api/projects", nil, nil), http.StatusUnauthorized, "no_session")
	first := a.project(t, cookie)
	wantCode(t, "second", a.do(http.MethodPost, "/api/projects", cookie, nil), http.StatusForbidden, "account_required")
	wantCode(t, "second from the demo case", a.do(http.MethodPost, "/api/projects", cookie, map[string]any{"template": "case-5"}), http.StatusForbidden, "account_required")
	if got := a.count(t, `SELECT COUNT(*) FROM project`); got != 1 {
		t.Fatalf("projects: %d, want 1", got)
	}
	if got := a.count(t, `SELECT COUNT(*) FROM project WHERE id = ? AND invitation_id = ? AND anon_session_hash = ?
		AND account_id IS NULL AND expires_at = ?`, first, linkID, store.HashSecret(cookie.Value),
		a.now.Add(7*24*time.Hour).Format("2006-01-02T15:04:05.000")+"Z"); got != 1 {
		t.Fatal("the project does not keep its link, its session's hash and its expiry")
	}
}

// B3-18: two requests at once do not make two projects.
func TestB3_18_ConcurrentCreatesMakeOneProject(t *testing.T) {
	a := newAccess(t)
	_, token := newLink(t, a.db, "jury", 10)
	cookie := a.visitor(t, token)
	var wg sync.WaitGroup
	codes := make([]int, 8)
	for i := range codes {
		wg.Add(1)
		go func() {
			defer wg.Done()
			req := httptest.NewRequest(http.MethodPost, "/api/projects", nil)
			req.AddCookie(cookie)
			rec := httptest.NewRecorder()
			a.h.ServeHTTP(rec, req)
			codes[i] = rec.Code
		}()
	}
	wg.Wait()
	created := 0
	for _, c := range codes {
		if c == http.StatusCreated {
			created++
		} else if c != http.StatusForbidden {
			t.Fatalf("codes: %v", codes)
		}
	}
	if got := a.count(t, `SELECT COUNT(*) FROM project`); got != 1 || created != 1 {
		t.Fatalf("projects %d, created %d: want 1", got, created)
	}
}

// B3-22 row 2: the 6th project of a link with --anon 5 is link_full.
func TestB3_22_2_LinkFull(t *testing.T) {
	a := newAccess(t)
	_, token := newLink(t, a.db, "guests", 5)
	for range 5 {
		a.project(t, a.visitor(t, token))
	}
	wantCode(t, "6th", a.do(http.MethodPost, "/api/projects", a.visitor(t, token), nil), http.StatusForbidden, "link_full")
	if got := a.count(t, `SELECT COUNT(*) FROM project`); got != 5 {
		t.Fatalf("projects: %d, want 5", got)
	}
}

// [EL DISEÑO NO LO DICE] choice: a session of a link revoked later keeps
// what it has and opens nothing new.
func TestB3_22_2_RevokedLinkOpensNoNewProject(t *testing.T) {
	a := newAccess(t)
	linkID, token := newLink(t, a.db, "guests", 5)
	withProject := a.visitor(t, token)
	projectID := a.project(t, withProject)
	without := a.visitor(t, token)
	if err := a.db.RevokeInvitation(context.Background(), linkID, a.now); err != nil {
		t.Fatalf("RevokeInvitation: %v", err)
	}
	wantCode(t, "new project", a.do(http.MethodPost, "/api/projects", without, nil), http.StatusForbidden, "link_full")
	if rec := a.do(http.MethodGet, "/api/projects/"+projectID+"/sections", withProject, nil); rec.Code != http.StatusOK {
		t.Fatalf("what exists: %d", rec.Code)
	}
}

// tablesWithProjectID is every table with a project_id, read from the
// schema so a later table is covered.
func tablesWithProjectID(t *testing.T, a *access) []string {
	t.Helper()
	rows, err := a.db.Reader.QueryContext(context.Background(), `SELECT m.name FROM sqlite_master m
		JOIN pragma_table_info(m.name) c ON c.name = 'project_id' WHERE m.type = 'table'`)
	if err != nil {
		t.Fatalf("tables: %v", err)
	}
	defer func() { _ = rows.Close() }()
	var out []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			t.Fatalf("tables: %v", err)
		}
		out = append(out, name)
	}
	return out
}

// seedEveryTable leaves the project with a row in every table.
func seedEveryTable(t *testing.T, a *access, cookie *http.Cookie, projectID string) {
	t.Helper()
	if rec := a.do(http.MethodPost, "/api/projects/"+projectID+"/sections", cookie, map[string]any{"title": "Alcance"}); rec.Code != http.StatusCreated {
		t.Fatalf("seed section: %d (%s)", rec.Code, rec.Body.String())
	}
	object := a.text(t, `SELECT id FROM object WHERE project_id = ?`, projectID)
	for _, q := range []string{
		`INSERT INTO edge (project_id, id, type, from_id, to_id, version, current) VALUES (?1, 'e1', 'used_in', ?2, ?2, 1, 1)`,
		`INSERT INTO delivery (project_id, created_at) VALUES (?1, '2026-11-02T00:00:00.000Z')`,
		`INSERT INTO model_call (project_id, cost, created_at) VALUES (?1, 0.5, '2026-11-02T00:00:00.000Z')`,
	} {
		args := []any{projectID}
		if strings.Contains(q, "?2") {
			args = append(args, object)
		}
		if _, err := a.db.Writer.ExecContext(context.Background(), q, args...); err != nil {
			t.Fatalf("seed %q: %v", q, err)
		}
	}
	for _, table := range tablesWithProjectID(t, a) {
		if got := a.count(t, `SELECT COUNT(*) FROM "`+table+`" WHERE project_id = ?`, projectID); got == 0 {
			t.Fatalf("seed: no row in %s", table)
		}
	}
}

// B3-18, B-88: 7 days and 1 minute after your last request, the cleanup
// leaves the project with no row in any table and the link's spending
// as it was; a request at day 6 moves the expiry, at most once an hour.
func TestB3_18_AnonProjectExpires(t *testing.T) {
	a := newAccess(t)
	linkID, token := newLink(t, a.db, "jury", 10)
	gone := a.visitor(t, token)
	kept := a.visitor(t, token)
	goneID := a.project(t, gone)
	keptID := a.project(t, kept)
	seedEveryTable(t, a, gone, goneID)
	seedEveryTable(t, a, kept, keptID)
	ctx := context.Background()
	if _, err := a.db.Writer.ExecContext(ctx, `UPDATE invitation SET spent_usd = 1.25 WHERE id = ?`, linkID); err != nil {
		t.Fatalf("seed spent: %v", err)
	}
	// A project from before migration 0005 has no owner and stays (B-87).
	if _, err := a.db.Writer.ExecContext(ctx, `INSERT INTO project (id, created_at) VALUES ('legacy', '2026-10-01T00:00:00.000Z')`); err != nil {
		t.Fatalf("seed legacy: %v", err)
	}
	start := a.now

	a.now = start.Add(6 * 24 * time.Hour)
	if rec := a.do(http.MethodGet, "/api/projects/"+keptID+"/sections", kept, nil); rec.Code != http.StatusOK {
		t.Fatalf("day 6: %d", rec.Code)
	}
	moved := a.text(t, `SELECT expires_at FROM project WHERE id = ?`, keptID)
	if want := a.now.Add(7*24*time.Hour).Format("2006-01-02T15:04:05.000") + "Z"; moved != want {
		t.Fatalf("expiry after day 6: %s, want %s", moved, want)
	}
	// Within the hour there is no second write.
	a.now = a.now.Add(30 * time.Minute)
	a.do(http.MethodGet, "/api/projects/"+keptID+"/sections", kept, nil)
	if got := a.text(t, `SELECT expires_at FROM project WHERE id = ?`, keptID); got != moved {
		t.Fatalf("expiry moved twice within the hour: %s → %s", moved, got)
	}
	a.now = a.now.Add(31 * time.Minute)
	a.do(http.MethodGet, "/api/projects/"+keptID+"/sections", kept, nil)
	if got := a.text(t, `SELECT expires_at FROM project WHERE id = ?`, keptID); got <= moved {
		t.Fatalf("expiry did not move after an hour: %s", got)
	}

	// One minute before: nothing goes.
	if n, _, err := a.db.PurgeExpired(ctx, start.Add(7*24*time.Hour-time.Minute)); err != nil || n != 0 {
		t.Fatalf("purge before the time: %d, %v", n, err)
	}
	if got := a.count(t, `SELECT COUNT(*) FROM project WHERE id = ?`, goneID); got != 1 {
		t.Fatal("the project went before its time")
	}

	n, _, err := a.db.PurgeExpired(ctx, start.Add(7*24*time.Hour+time.Minute))
	if err != nil || n != 1 {
		t.Fatalf("purge: %d, %v; want 1", n, err)
	}
	for _, table := range tablesWithProjectID(t, a) {
		if got := a.count(t, `SELECT COUNT(*) FROM "`+table+`" WHERE project_id = ?`, goneID); got != 0 {
			t.Errorf("%s keeps %d rows of the expired project", table, got)
		}
		if got := a.count(t, `SELECT COUNT(*) FROM "`+table+`" WHERE project_id = ?`, keptID); got == 0 {
			t.Errorf("%s lost the rows of the project still alive", table)
		}
	}
	if got := a.count(t, `SELECT COUNT(*) FROM project WHERE id = ?`, goneID); got != 0 {
		t.Fatal("the project row stays")
	}
	if got := a.count(t, `SELECT COUNT(*) FROM project WHERE id IN (?, 'legacy')`, keptID); got != 2 {
		t.Fatalf("kept and legacy projects: %d, want 2", got)
	}
	var spent float64
	if err := a.db.Reader.QueryRowContext(ctx, `SELECT spent_usd FROM invitation WHERE id = ?`, linkID).Scan(&spent); err != nil || spent != 1.25 {
		t.Fatalf("spent_usd: %v, %v; want 1.25", spent, err)
	}
	a.now = start.Add(7*24*time.Hour + time.Minute)
	wantCode(t, "expired project", a.do(http.MethodGet, "/api/projects/"+goneID+"/sections", gone, nil), http.StatusNotFound, "project_not_found")

	// Sessions go when they expire (B-88).
	if _, s, err := a.db.PurgeExpired(ctx, start.Add(14*24*time.Hour+time.Minute)); err != nil || s != 2 {
		t.Fatalf("expired sessions: %d, %v; want 2", s, err)
	}
	if got := a.count(t, `SELECT COUNT(*) FROM session`); got != 0 {
		t.Fatalf("sessions: %d, want 0", got)
	}
}

// [EL DISEÑO NO LO DICE] choice: until the cleanup passes, a project past
// its expiry is still yours, and your request moves its expiry again.
func TestB3_18_RequestBeforeTheCleanupKeepsTheProject(t *testing.T) {
	a := newAccess(t)
	_, token := newLink(t, a.db, "jury", 10)
	cookie := a.visitor(t, token)
	projectID := a.project(t, cookie)
	a.now = a.now.Add(7*24*time.Hour + time.Minute)
	if rec := a.do(http.MethodGet, "/api/projects/"+projectID+"/sections", cookie, nil); rec.Code != http.StatusOK {
		t.Fatalf("request past the expiry: %d", rec.Code)
	}
	if n, _, err := a.db.PurgeExpired(context.Background(), a.now); err != nil || n != 0 {
		t.Fatalf("purge after the request: %d, %v; want 0", n, err)
	}
}

// B-88: the hourly cleanup runs at once and again on its tick, and logs
// counts only.
func TestB3_18_CleanupRunsOnItsClock(t *testing.T) {
	a := newAccess(t)
	_, token := newLink(t, a.db, "jury", 10)
	cookie := a.visitor(t, token)
	projectID := a.project(t, cookie)
	var mu sync.Mutex
	now := a.now
	clock := func() time.Time { mu.Lock(); defer mu.Unlock(); return now }
	var buf syncBuffer
	logger := slog.New(slog.NewJSONHandler(&buf, nil))
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		defer close(done)
		server.RunCleanup(ctx, a.db, logger, 5*time.Millisecond, clock)
	}()
	time.Sleep(20 * time.Millisecond)
	if got := a.count(t, `SELECT COUNT(*) FROM project WHERE id = ?`, projectID); got != 1 {
		t.Fatal("a live project was removed")
	}
	mu.Lock()
	now = now.Add(7*24*time.Hour + time.Minute)
	mu.Unlock()
	deadline := time.Now().Add(5 * time.Second)
	for a.count(t, `SELECT COUNT(*) FROM project WHERE id = ?`, projectID) != 0 {
		if time.Now().After(deadline) {
			t.Fatal("the cleanup never removed the expired project")
		}
		time.Sleep(5 * time.Millisecond)
	}
	cancel()
	<-done
	if !strings.Contains(buf.String(), `"projects":1`) || strings.Contains(buf.String(), projectID) {
		t.Fatalf("cleanup log: %s", buf.String())
	}
	if server.CleanupEvery != time.Hour {
		t.Fatalf("CleanupEvery: %v, want an hour (B-88)", server.CleanupEvery)
	}
}

type syncBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *syncBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *syncBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

// B3-18: GET /api/projects lists only yours; without a session, 401.
func TestB3_18_ListOnlyYourProjects(t *testing.T) {
	a := newAccess(t)
	_, token := newLink(t, a.db, "jury", 10)
	one := a.visitor(t, token)
	two := a.visitor(t, token)
	mine := a.project(t, one)
	theirs := a.project(t, two)
	// A project from before migration 0005 is not seen from the web (B-87).
	if _, err := a.db.Writer.ExecContext(context.Background(), `INSERT INTO project (id, created_at) VALUES ('legacy', '2026-10-01T00:00:00.000Z')`); err != nil {
		t.Fatalf("seed legacy: %v", err)
	}
	wantCode(t, "no cookie", a.do(http.MethodGet, "/api/projects", nil, nil), http.StatusUnauthorized, "no_session")
	wantCode(t, "invented cookie", a.do(http.MethodGet, "/api/projects", &http.Cookie{Name: sessionCookie, Value: "made-up"}, nil), http.StatusUnauthorized, "no_session")
	wantCode(t, "legacy", a.do(http.MethodGet, "/api/projects/legacy/sections", one, nil), http.StatusNotFound, "project_not_found")

	list := decode[[]store.Project](t, a.do(http.MethodGet, "/api/projects", one, nil))
	if len(list) != 1 || list[0].ID != mine {
		t.Fatalf("list: %+v, want only %s", list, mine)
	}
	if list := decode[[]store.Project](t, a.do(http.MethodGet, "/api/projects", two, nil)); len(list) != 1 || list[0].ID != theirs {
		t.Fatalf("other's list: %+v, want only %s", list, theirs)
	}
}

// B-87: what has no {projectID} stays open, and without a database a
// project route stays closed: nothing turns the check off.
func TestB87_PreviousOpenRoutesStayOpenAndNoDatabaseStaysClosed(t *testing.T) {
	a := newAccess(t)
	for _, path := range []string{"/healthz", "/"} {
		if rec := a.do(http.MethodGet, path, nil, nil); rec.Code != http.StatusOK {
			t.Fatalf("GET %s: %d", path, rec.Code)
		}
	}
	hub := server.NewHub()
	hub.BeginTurn("p1", "t1")
	hub.PublishError("p1", "t1", "provider_error")
	req := httptest.NewRequest(http.MethodGet, "/api/projects/p1/turns/t1/stream", nil)
	rec := httptest.NewRecorder()
	server.NewWithHub(nil, hub).ServeHTTP(rec, req)
	wantCode(t, "stream without a database", rec, http.StatusUnauthorized, "no_session")
	if b, _ := io.ReadAll(rec.Body); strings.Contains(string(b), "event:") {
		t.Fatalf("stream without a database sent events: %s", b)
	}
}
