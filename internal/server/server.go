// Package server implements HTTP, streaming and access for tupaqnin
// (specification §6): the API, the turn streams, the embedded web page
// and the access by invitation link.
package server

import (
	"bytes"
	"embed"
	"encoding/json"
	"errors"
	"io"
	"io/fs"
	"log/slog"
	"net/http"
	"time"

	"github.com/LaurenteEber/tupaqnin/internal/graph"
	"github.com/LaurenteEber/tupaqnin/internal/store"
)

//go:embed static
var staticFiles embed.FS

// securityHeaders sets the security headers required by specification
// §6 (CSP with no inline scripts, no MIME sniffing, no framing) and HSTS
// on every response (B3-22 row 10).
func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("Content-Security-Policy", "default-src 'self'; script-src 'self'; object-src 'none'; base-uri 'self'; frame-ancestors 'none'")
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("Referrer-Policy", "strict-origin-when-cross-origin")
		h.Set("Strict-Transport-Security", "max-age=31536000")
		next.ServeHTTP(w, r)
	})
}

const (
	// defaultBodyBytes is the cap of every request's body (B3-22 row 10):
	// a section of 200 KB fits, with its JSON around it. [inicial]
	defaultBodyBytes = 256 << 10
	// strangerBodyBytes is the cap without a session: an account's form fits,
	// and many stalled connections cannot fill the memory. [inicial]
	strangerBodyBytes = 4 << 10
	// sourceBodyBytes is the upload's cap: a source of 5 MB and its form.
	sourceBodyBytes = maxSourceBytes + (1 << 20)
	// sourcesPattern is the upload's route, the one over the default cap.
	sourcesPattern = "/api/projects/{projectID}/sources"
)

// bodyCap is the one body cap (B3-22 row 10), so no handler can forget it:
// a body over the cap answers 413 before any handler reads it, whatever the
// handler would have done with a cut body. The route is the one the mux
// picks, so the decision is on decoded segments, as the mux's. The upload
// reads its own body as it goes: its handler answers source_too_large.
// The body is held in memory, so without a session the cap is far smaller.
// [EL DISEÑO NO LO DICE]: B3-22 fixes one cap; this second one exists because
// the owner control lets through every path outside /api/projects.
func bodyCap(mux *http.ServeMux) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Body == nil || r.Body == http.NoBody {
			mux.ServeHTTP(w, r)
			return
		}
		if _, pattern := mux.Handler(r); pattern == http.MethodPost+" "+sourcesPattern {
			r.Body = http.MaxBytesReader(w, r.Body, sourceBodyBytes)
			mux.ServeHTTP(w, r)
			return
		}
		limit := int64(strangerBodyBytes)
		if _, hasSession := r.Context().Value(sessionKey{}).(store.Session); hasSession {
			limit = defaultBodyBytes
		}
		body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, limit))
		if err != nil {
			var tooLarge *http.MaxBytesError
			if errors.As(err, &tooLarge) {
				// [EL DISEÑO NO LO DICE]: the code of this 413.
				writeError(w, http.StatusRequestEntityTooLarge, "body_too_large")
				return
			}
			writeError(w, http.StatusBadRequest, "invalid_body")
			return
		}
		r.Body = io.NopCloser(bytes.NewReader(body))
		mux.ServeHTTP(w, r)
	})
}

// statusRecorder records the status code written by a handler.
type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(status int) {
	r.status = status
	r.ResponseWriter.WriteHeader(status)
}

// Flush forwards to the underlying writer so streaming handlers can
// flush through this middleware (specification §6).
func (r *statusRecorder) Flush() {
	if f, ok := r.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

// Unwrap lets http.ResponseController reach the underlying writer
// (for example, to clear the write deadline on streaming routes).
func (r *statusRecorder) Unwrap() http.ResponseWriter { return r.ResponseWriter }

// requestLog logs one JSON line per request with method, path, status
// and duration only (T-10): never the body, query, headers or any
// document or message content.
func requestLog(logger *slog.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rec, r)
		logger.Info("request",
			"method", r.Method,
			"path", loggedPath(r.URL.Path),
			"status", rec.status,
			"duration_ms", time.Since(start).Milliseconds(),
		)
	})
}

// Server is the HTTP surface: its routes, and the hub that turns
// publish their stream events to (backlog 0.6).
type Server struct {
	hub        *Hub
	logger     *slog.Logger
	db         *store.DB
	starter    TurnStarter
	signoffs   SignoffStarter
	editors    *editors
	projectCap float64
	// diskFull says the disk is at 80 % (espec. §6); nil: not checked.
	diskFull func() (bool, error)
	// providerName is shown in the summary (U-2): meta or fake.
	providerName string
	// now and returnAfter decide the return line (M-2, B-48 a); tests
	// inject the clock instead of sleeping.
	now         func() time.Time
	returnAfter time.Duration
	// attempts is the limit per IP and the wait per account (B3-22 row 1).
	attempts *store.LoginAttempts
}

// Option configures the API server.
type Option func(*Server)

// WithProjectCap sets the project's spending cap shown in the summary
// (B3-20; TQ_CAP_PROJECT_USD).
func WithProjectCap(usd float64) Option { return func(s *Server) { s.projectCap = usd } }

// WithDiskFull sets how the server learns that the disk is at 80 %
// (espec. §6). main passes DiskFull(TQ_DATA_DIR); a test, what it simulates.
func WithDiskFull(full func() (bool, error)) Option { return func(s *Server) { s.diskFull = full } }

// WithProvider names the model provider shown in the summary (U-2;
// TQ_PROVIDER).
func WithProvider(name string) Option { return func(s *Server) { s.providerName = name } }

// WithClock sets the clock the return line reads (M-2); tests use it
// to come back days later without sleeping.
func WithClock(now func() time.Time) Option { return func(s *Server) { s.now = now } }

// WithReturnAfter sets M-2's threshold [inicial, parámetro]; the
// default is graph.ReturnAfter() (3 days).
func WithReturnAfter(d time.Duration) Option { return func(s *Server) { s.returnAfter = d } }

// Hub returns the hub turn runners publish stream events to.
func (s *Server) Hub() *Hub { return s.hub }

// New returns the HTTP handler for the application. A nil logger
// discards request logs.
func New(logger *slog.Logger) http.Handler {
	return NewWithHub(logger, NewHub())
}

// NewWithHub is New with a caller-provided hub, so the turn runner
// (and tests) publish to the same hub the streams read from.
func NewWithHub(logger *slog.Logger, hub *Hub) http.Handler {
	return buildHandler(logger, hub, nil, nil)
}

// NewWithAPI is NewWithHub plus the slice-0 API (backlog 0.10):
// projects, sections and messages over db, with turns started by
// starter.
func NewWithAPI(logger *slog.Logger, hub *Hub, db *store.DB, starter TurnStarter, opts ...Option) http.Handler {
	return buildHandler(logger, hub, db, starter, opts...)
}

func buildHandler(logger *slog.Logger, hub *Hub, db *store.DB, starter TurnStarter, opts ...Option) http.Handler {
	if logger == nil {
		logger = slog.New(slog.NewJSONHandler(io.Discard, nil))
	}
	if hub == nil {
		hub = NewHub()
	}
	s := &Server{hub: hub, logger: logger, db: db, starter: starter, editors: newEditors(), projectCap: 3,
		now: time.Now, returnAfter: graph.ReturnAfter(), attempts: store.NewLoginAttempts()}
	for _, opt := range opts {
		opt(s)
	}
	mux := http.NewServeMux()
	for _, rt := range s.routes(db != nil && starter != nil, starter) {
		mux.HandleFunc(rt.method+" "+rt.pattern, rt.handler)
	}
	registerDevRoutes(mux, s)

	staticFS, err := fs.Sub(staticFiles, "static")
	if err != nil {
		panic(err)
	}
	mux.Handle("GET /", http.FileServer(http.FS(staticFS)))

	// The log and the headers wrap every answer, refusals included; the
	// owner check runs first, so a project route never reads a stranger's
	// body; every other path reads at most strangerBodyBytes of it.
	crossOrigin := http.NewCrossOriginProtection()
	return requestLog(logger, securityHeaders(crossOrigin.Handler(s.ownerControl(bodyCap(mux)))))
}

// route is one row of the route table (B-87): a test walks the table,
// so a new route cannot be born outside the owner check.
type route struct {
	method  string
	pattern string
	handler http.HandlerFunc
}

// routes is the application's route table. The API needs a database and
// a turn starter; the signoff's turns need a starter that runs them.
func (s *Server) routes(withAPI bool, starter TurnStarter) []route {
	table := []route{
		{"GET", "/healthz", s.healthz},
		{"GET", "/api/projects/{projectID}/turns/{turnID}/stream", s.streamTurn},
	}
	if !withAPI {
		return table
	}
	table = append(table, route{"GET", "/i/{token}", s.redeemInvite})
	table = append(table, s.apiRoutes()...)
	if signoffs, ok := starter.(SignoffStarter); ok {
		s.signoffs = signoffs
		table = append(table, s.signoffTurnRoutes()...)
	}
	return table
}

func (s *Server) healthz(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

func (s *Server) apiRoutes() []route {
	return []route{
		{"POST", "/api/account/register", s.register},
		{"POST", "/api/account/login", s.login},
		{"POST", "/api/account/logout", s.logout},
		{"POST", "/api/projects", s.createProjectV2},
		{"GET", "/api/projects", s.listProjects},
		{"GET", "/api/projects/{projectID}/request", s.getRequest},
		{"PUT", "/api/projects/{projectID}/request", s.putRequest},
		{"POST", sourcesPattern, s.uploadSource},
		{"GET", "/api/projects/{projectID}/sources/{label}/download", s.downloadSource},
		{"GET", "/api/projects/{projectID}/sources/{label}", s.getSource},
		{"POST", "/api/projects/{projectID}/request/submit", s.submitRequest},
		{"GET", "/api/projects/{projectID}/understanding", s.getUnderstanding},
		{"POST", "/api/projects/{projectID}/cases/{label}/confirm", s.confirmCase},
		{"POST", "/api/projects/{projectID}/cases/{label}/correct", s.correctCase},
		{"PUT", "/api/projects/{projectID}/sections/{label}/role", s.putSectionRole},
		{"POST", "/api/projects/{projectID}/plan/confirm", s.confirmPlan},
		{"POST", "/api/projects/{projectID}/sections", s.createSection},
		{"GET", "/api/projects/{projectID}/sections", s.listSections},
		{"PUT", "/api/projects/{projectID}/sections/{label}", s.putSection},
		{"PUT", "/api/projects/{projectID}/sections/{label}/details", s.putSectionDetails},
		{"PUT", "/api/projects/{projectID}/sections/order", s.putSectionOrder},
		{"POST", "/api/projects/{projectID}/messages", s.postMessage},
		{"GET", "/api/projects/{projectID}/todo", s.getTodo},
		{"GET", "/api/projects/{projectID}/sections/{label}", s.getSection},
		{"GET", "/api/projects/{projectID}/sections/{label}/history", s.getBlockHistory},
		{"POST", "/api/projects/{projectID}/sections/{label}/claims/accept", s.acceptClaim},
		{"PUT", "/api/projects/{projectID}/request/evidence", s.putRequestEvidence},
		{"PUT", "/api/projects/{projectID}/request/audience", s.putRequestAudience},
		{"PUT", "/api/projects/{projectID}/criteria/{label}", s.putCriterion},
		{"GET", "/api/projects/{projectID}/export.md", s.getExport},
		{"GET", "/api/projects/{projectID}/sections/{label}/changes", s.getSectionChanges},
		{"POST", "/api/projects/{projectID}/changes/{label}/close", s.closeChange},
		{"GET", "/api/projects/{projectID}/decisions/{label}/history", s.getDecisionHistory},
		{"GET", "/api/projects/{projectID}/summary", s.getSummary},
		{"GET", "/api/projects/{projectID}/conversation", s.getConversation},
		{"GET", "/api/projects/{projectID}/rounds/{label}", s.getRound},
		{"POST", "/api/projects/{projectID}/rounds", s.postRound},
		{"POST", "/api/projects/{projectID}/rounds/{label}/repeat", s.repeatRound},
		{"POST", "/api/projects/{projectID}/rounds/{label}/leave-open", s.leaveRoundOpen},
		{"POST", "/api/projects/{projectID}/rounds/{label}/flaws/{n}/discard", s.discardFlaw},
		{"POST", "/api/projects/{projectID}/sections/{label}/audit", s.auditSection},
		{"PUT", "/api/projects/{projectID}/name", s.putProjectName},
		{"POST", "/api/projects/{projectID}/editor", s.claimEditor},
		{"GET", "/api/projects/{projectID}/cards/{label}", s.getCard},
		{"POST", "/api/projects/{projectID}/cards/{label}/answer", s.answerCard},
		{"PUT", "/api/projects/{projectID}/decisions/{label}/critical", s.putDecisionCritical},
		{"POST", "/api/projects/{projectID}/open-questions/{label}/answer", s.answerOpenQuestion},
		{"GET", "/api/projects/{projectID}/stop-report", s.getStopReport},
		{"POST", "/api/projects/{projectID}/stop-report/seen", s.postStopReportSeen},
		{"POST", "/api/projects/{projectID}/stop-report/continue", s.postStopReportContinue},
		{"POST", "/api/projects/{projectID}/audience-questions/{label}/answer", s.answerAudienceQuestion},
		{"GET", "/api/projects/{projectID}/frame", s.getFrame},
		{"POST", "/api/projects/{projectID}/frame/decisions/{label}/seen", s.postFrameDecisionSeen},
		{"POST", "/api/projects/{projectID}/sections/{label}/signoff", s.requestSignoff},
		{"GET", "/api/projects/{projectID}/signoffs/{label}", s.getSignoff},
		{"POST", "/api/projects/{projectID}/signoffs/{label}/sign", s.signSignoff},
		{"POST", "/api/projects/{projectID}/signoffs/{label}/leave", s.leaveSignoff},
	}
}

// signoffTurnRoutes are the routes of P7 that start a model's turn
// (backlog-r1 1.7, B-13, B-14).
func (s *Server) signoffTurnRoutes() []route {
	return []route{
		{"POST", "/api/projects/{projectID}/signoffs/{label}/explanation", s.postSignoffExplanation},
		{"POST", "/api/projects/{projectID}/signoffs/{label}/answers", s.postSignoffAnswers},
		{"POST", "/api/projects/{projectID}/signoffs/{label}/ask", s.postSignoffAsk},
	}
}
