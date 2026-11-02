package server_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/LaurenteEber/tupaqnin/internal/server"
	"github.com/LaurenteEber/tupaqnin/internal/store"
)

// sessionAnswer is what GET /api/session answers (E.6).
type sessionAnswer struct {
	Account      string `json:"account"`
	LinkLabel    string `json:"link_label"`
	CanCreate    bool   `json:"can_create"`
	CannotCreate string `json:"cannot_create"`
}

// projectLine is one row of GET /api/projects (P1).
type projectLine struct {
	ProjectID    string `json:"project_id"`
	Name         string `json:"name"`
	CreatedAt    string `json:"created_at"`
	LastActivity string `json:"last_activity"`
	Signed       int    `json:"signed"`
	Sections     int    `json:"sections"`
	NoAccount    bool   `json:"no_account"`
}

// wantCreateAgrees: what GET /api/session says is what POST /api/projects
// answers, with the same code, for a blank project and for the demo case.
func wantCreateAgrees(t *testing.T, a *access, cookie *http.Cookie, code string) {
	t.Helper()
	rec := a.do(http.MethodGet, "/api/session", cookie, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /api/session: %d (%s)", rec.Code, rec.Body.String())
	}
	got := decode[sessionAnswer](t, rec)
	if got.CanCreate != (code == "") || got.CannotCreate != code {
		t.Fatalf("session: %+v, want cannot_create %q", got, code)
	}
	if code == "" {
		if rec := a.do(http.MethodPost, "/api/projects", cookie, nil); rec.Code != http.StatusCreated {
			t.Fatalf("session said yes, POST: %d (%s)", rec.Code, rec.Body.String())
		}
		return
	}
	wantCode(t, "POST blank", a.do(http.MethodPost, "/api/projects", cookie, nil), http.StatusForbidden, code)
	wantCode(t, "POST demo", a.do(http.MethodPost, "/api/projects", cookie, map[string]string{"template": "case-5"}), http.StatusForbidden, code)
}

// E.1 and E.5: the rules of the link and of the account.
func TestP1_CanCreateFollowsTheLinkAndTheAccount(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name  string
		setup func(t *testing.T, a *access) *http.Cookie
		want  string
	}{
		{"first project without an account", func(t *testing.T, a *access) *http.Cookie {
			_, token := newLink(t, a.db, "guests", 5)
			return a.visitor(t, token)
		}, ""},
		{"second project without an account", func(t *testing.T, a *access) *http.Cookie {
			_, token := newLink(t, a.db, "guests", 5)
			cookie := a.visitor(t, token)
			a.project(t, cookie)
			return cookie
		}, "account_required"},
		{"the link's anonymous projects are full", func(t *testing.T, a *access) *http.Cookie {
			_, token := newLink(t, a.db, "guests", 1)
			a.project(t, a.visitor(t, token))
			return a.visitor(t, token)
		}, "link_full"},
		{"a revoked link", func(t *testing.T, a *access) *http.Cookie {
			id, token := newLink(t, a.db, "guests", 5)
			cookie := a.visitor(t, token)
			if err := a.db.RevokeInvitation(context.Background(), id, a.now); err != nil {
				t.Fatal(err)
			}
			return cookie
		}, "link_full"},
		{"an account with room", func(t *testing.T, a *access) *http.Cookie {
			_, token := newLink(t, a.db, "guests", 5)
			cookie := a.registered(t, token, "roomy")
			a.project(t, cookie)
			a.project(t, cookie)
			return cookie
		}, ""},
		{"an account with its 3 projects", func(t *testing.T, a *access) *http.Cookie {
			_, token := newLink(t, a.db, "guests", 5)
			cookie := a.registered(t, token, "three")
			for range 3 {
				a.project(t, cookie)
			}
			return cookie
		}, "account_limit"},
		{"an account on a revoked link", func(t *testing.T, a *access) *http.Cookie {
			id, token := newLink(t, a.db, "guests", 5)
			cookie := a.registered(t, token, "revoked")
			if err := a.db.RevokeInvitation(context.Background(), id, a.now); err != nil {
				t.Fatal(err)
			}
			return cookie
		}, "link_full"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			a := newAccess(t)
			wantCreateAgrees(t, a, c.setup(t, a), c.want)
		})
	}
}

// E.3: the caps of money and disk.
func TestP1_CanCreateFollowsTheCaps(t *testing.T) {
	full := func() (bool, error) { return true, nil }
	unreadable := func() (bool, error) { return false, errors.New("statfs") }

	t.Run("link_budget", func(t *testing.T) {
		s := newSpendAPI(t, nil)
		link := budgetLink(t, s.db, "guests", 1)
		cookie := newSessionCookie(t, s.db, link)
		spendOn(t, s.db, s.project(t, newSessionCookie(t, s.db, link)), 1)
		wantCreateAgrees(t, s.access, cookie, "link_budget")
	})
	t.Run("day_cap", func(t *testing.T) {
		s := newSpendAPI(t, nil)
		s.runner.SpendCapDayUSD = 1
		spendOn(t, s.db, s.project(t, newSessionCookie(t, s.db, budgetLink(t, s.db, "guests", 10))), 1)
		wantCreateAgrees(t, s.access, newSessionCookie(t, s.db, budgetLink(t, s.db, "guests", 10)), "day_cap")
		wantCreateAgrees(t, s.access, newSessionCookie(t, s.db, budgetLink(t, s.db, store.LabelJury, 30)), "")
	})
	t.Run("disk_full", func(t *testing.T) {
		s := newSpendAPI(t, full)
		wantCreateAgrees(t, s.access, newSessionCookie(t, s.db, budgetLink(t, s.db, "guests", 10)), "disk_full")
		wantCreateAgrees(t, s.access, newSessionCookie(t, s.db, budgetLink(t, s.db, store.LabelJury, 30)), "")
	})
	t.Run("a disk that cannot be read", func(t *testing.T) {
		s := newSpendAPI(t, unreadable)
		wantCreateAgrees(t, s.access, newSessionCookie(t, s.db, budgetLink(t, s.db, "guests", 10)), "")
	})
	t.Run("the caps come before the rules of the account", func(t *testing.T) {
		s := newSpendAPI(t, full)
		cookie := newSessionCookie(t, s.db, budgetLink(t, s.db, store.LabelJury, 30))
		s.project(t, cookie)
		wantCreateAgrees(t, s.access, cookie, "account_required")
		other := newSessionCookie(t, s.db, budgetLink(t, s.db, "guests", 10))
		wantCreateAgrees(t, s.access, other, "disk_full")
	})
}

// The session holds exactly the row's fields, and never the link's token.
func TestP1_SessionNamesTheAccountAndTheLink(t *testing.T) {
	t.Parallel()
	a := newAccess(t)
	_, token := newLink(t, a.db, "guests", 5)
	anon := a.visitor(t, token)

	rec := a.do(http.MethodGet, "/api/session", anon, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /api/session: %d (%s)", rec.Code, rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), token) {
		t.Fatalf("the session carries the link's token: %s", rec.Body.String())
	}
	var raw map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &raw); err != nil {
		t.Fatal(err)
	}
	keys := make([]string, 0, len(raw))
	for key := range raw {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	if got := strings.Join(keys, ","); got != "account,can_create,cannot_create,link_label" {
		t.Fatalf("fields: %s", got)
	}
	if got := decode[sessionAnswer](t, rec); got != (sessionAnswer{LinkLabel: "guests", CanCreate: true}) {
		t.Fatalf("anonymous session: %+v", got)
	}

	cookie := newCookie(t, a.register(anon, "Mixed.Name", goodPassword))
	if got := decode[sessionAnswer](t, a.do(http.MethodGet, "/api/session", cookie, nil)); got != (sessionAnswer{Account: "mixed.name", LinkLabel: "guests", CanCreate: true}) {
		t.Fatalf("account's session: %+v", got)
	}
}

// Without a session the answer is 401 no_session, sets no cookie and says
// nothing; a logout, a bad cookie and an expired one answer the same.
func TestP1_NoSession(t *testing.T) {
	t.Parallel()
	a := newAccess(t)
	_, token := newLink(t, a.db, "guests", 5)
	wantNone := func(what string, cookie *http.Cookie) {
		t.Helper()
		rec := a.do(http.MethodGet, "/api/session", cookie, nil)
		wantCode(t, what, rec, http.StatusUnauthorized, "no_session")
		if got := rec.Header().Get("Set-Cookie"); got != "" {
			t.Fatalf("%s: Set-Cookie %q", what, got)
		}
		wantCode(t, what+": the list", a.do(http.MethodGet, "/api/projects", cookie, nil), http.StatusUnauthorized, "no_session")
	}
	wantNone("no cookie", nil)
	wantNone("a cookie of nothing", &http.Cookie{Name: sessionCookie, Value: "nothing"})

	anon := a.visitor(t, token)
	a.now = a.now.Add(store.AnonSessionTTL + time.Minute)
	wantNone("an expired session", anon)

	cookie := a.registered(t, token, "leaving")
	if rec := a.do(http.MethodGet, "/api/session", cookie, nil); rec.Code != http.StatusOK {
		t.Fatalf("before logout: %d", rec.Code)
	}
	if rec := a.do(http.MethodPost, "/api/account/logout", cookie, nil); rec.Code != http.StatusNoContent {
		t.Fatalf("logout: %d", rec.Code)
	}
	wantNone("after logout", cookie)

	rec := a.login("198.51.100.9", "leaving", goodPassword)
	if rec.Code != http.StatusOK {
		t.Fatalf("login: %d (%s)", rec.Code, rec.Body.String())
	}
	if got := decode[sessionAnswer](t, a.do(http.MethodGet, "/api/session", newCookie(t, rec), nil)); got.Account != "leaving" {
		t.Fatalf("after login: %+v", got)
	}
}

// Asking for the session counts as activity (B-88): it moves the expiry of the
// session and of its project, and with an account the browser's cookie too.
func TestP1_SessionCountsAsActivity(t *testing.T) {
	t.Parallel()
	a := newAccess(t)
	_, token := newLink(t, a.db, "guests", 5)
	anon := a.visitor(t, token)
	projectID := a.project(t, anon)

	a.now = a.now.Add(2 * time.Hour)
	rec := a.do(http.MethodGet, "/api/session", anon, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /api/session: %d", rec.Code)
	}
	if got := rec.Header().Get("Set-Cookie"); got != "" {
		t.Fatalf("anonymous Set-Cookie: %q", got)
	}
	want := stamp(a.now.Add(store.AnonSessionTTL))
	if got := a.text(t, `SELECT expires_at FROM session WHERE id_hash = ?`, store.HashSecret(anon.Value)); got != want {
		t.Fatalf("session expires %s, want %s", got, want)
	}
	if got := a.text(t, `SELECT expires_at FROM project WHERE id = ?`, projectID); got != want {
		t.Fatalf("project expires %s, want %s", got, want)
	}

	cookie := a.registered(t, token, "active")
	a.now = a.now.Add(2 * time.Hour)
	rec = a.do(http.MethodGet, "/api/session", cookie, nil)
	want = stamp(a.now.Add(store.SessionTTL))
	if got := a.text(t, `SELECT expires_at FROM session WHERE id_hash = ?`, store.HashSecret(cookie.Value)); got != want {
		t.Fatalf("account's session expires %s, want %s", got, want)
	}
	wantAccountCookie(t, rec)
}

// P1: name, last activity, signed of how many, and whether it has no account,
// all computed in Go; the fields that existed keep their shape; only yours.
func TestP1_ProjectListFields(t *testing.T) {
	t.Parallel()
	a := newAccess(t)
	_, token := newLink(t, a.db, "guests", 5)
	anon := a.visitor(t, token)
	rec := a.do(http.MethodPost, "/api/projects", anon, map[string]string{"name": "Microplásticos"})
	if rec.Code != http.StatusCreated {
		t.Fatalf("create: %d (%s)", rec.Code, rec.Body.String())
	}
	projectID := decode[server.CreateProjectResponse](t, rec).ProjectID
	createdAt := a.text(t, `SELECT created_at FROM project WHERE id = ?`, projectID)

	// With no event yet, the last activity is its creation.
	list := decode[[]projectLine](t, a.do(http.MethodGet, "/api/projects", anon, nil))
	want := projectLine{ProjectID: projectID, Name: "Microplásticos", CreatedAt: createdAt, LastActivity: createdAt, NoAccount: true}
	if len(list) != 1 || list[0] != want {
		t.Fatalf("new project: %+v, want %+v", list, want)
	}

	for _, title := range []string{"Uno", "Dos", "Tres"} {
		rec := a.do(http.MethodPost, "/api/projects/"+projectID+"/sections", anon, map[string]string{"title": title})
		if rec.Code != http.StatusCreated {
			t.Fatalf("section %s: %d (%s)", title, rec.Code, rec.Body.String())
		}
	}
	// One signed; one changed since its signoff, which is not signed any more.
	for label, state := range map[string]string{"sec-1": "signed", "sec-2": "changed_since_signoff"} {
		if _, err := a.db.Writer.ExecContext(context.Background(),
			`UPDATE object SET state = ? WHERE project_id = ? AND label = ?`, state, projectID, label); err != nil {
			t.Fatal(err)
		}
	}
	const lastEvent = "2031-01-02T03:04:05.000Z"
	if _, err := a.db.Writer.ExecContext(context.Background(),
		`UPDATE event SET created_at = ? WHERE project_id = ? AND seq = (SELECT MAX(seq) FROM event WHERE project_id = ?)`,
		lastEvent, projectID, projectID); err != nil {
		t.Fatal(err)
	}

	list = decode[[]projectLine](t, a.do(http.MethodGet, "/api/projects", anon, nil))
	want.LastActivity, want.Signed, want.Sections = lastEvent, 1, 3
	if len(list) != 1 || list[0] != want {
		t.Fatalf("with sections: %+v, want %+v", list, want)
	}

	// Another visitor of the same link sees nothing of it.
	other := a.visitor(t, token)
	if list := decode[[]projectLine](t, a.do(http.MethodGet, "/api/projects", other, nil)); len(list) != 0 {
		t.Fatalf("another visitor sees: %+v", list)
	}

	// Registering moves it to the account: it has an account now.
	cookie := newCookie(t, a.register(anon, "paula", goodPassword))
	list = decode[[]projectLine](t, a.do(http.MethodGet, "/api/projects", cookie, nil))
	want.NoAccount = false
	if len(list) != 1 || list[0] != want {
		t.Fatalf("with an account: %+v, want %+v", list, want)
	}

	var raw []map[string]any
	if err := json.Unmarshal(a.do(http.MethodGet, "/api/projects", cookie, nil).Body.Bytes(), &raw); err != nil {
		t.Fatal(err)
	}
	if _, ok := raw[0]["expires_in_days"]; ok {
		t.Fatalf("expires_in_days is gone (Eber, 2-nov): %v", raw[0])
	}
	if v, ok := raw[0]["no_account"]; !ok || v != false {
		t.Fatalf("no_account is sent with an account too: %v", raw[0])
	}
}
