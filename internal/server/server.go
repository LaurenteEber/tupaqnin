// Package server implements HTTP, streaming and access for tupaqnin
// (specification §6). In this base skeleton it only serves /healthz
// and the embedded minimal page; streaming and invitation access
// arrive in slice 0 and the delivery slice.
package server

import (
	"embed"
	"encoding/json"
	"io"
	"io/fs"
	"log/slog"
	"net/http"
	"time"

	"github.com/LaurenteEber/tupaqnin/internal/store"
)

//go:embed static
var staticFiles embed.FS

// securityHeaders sets the security headers required by specification
// §6 (CSP with no inline scripts, no MIME sniffing, no framing).
func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("Content-Security-Policy", "default-src 'self'; script-src 'self'; object-src 'none'; base-uri 'self'; frame-ancestors 'none'")
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("Referrer-Policy", "strict-origin-when-cross-origin")
		next.ServeHTTP(w, r)
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
			"path", r.URL.Path,
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
}

// Option configures the API server.
type Option func(*Server)

// WithProjectCap sets the project's spending cap shown in the summary
// (B3-20; TQ_CAP_PROJECT_USD).
func WithProjectCap(usd float64) Option { return func(s *Server) { s.projectCap = usd } }

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
	s := &Server{hub: hub, logger: logger, db: db, starter: starter, editors: newEditors(), projectCap: 3}
	for _, opt := range opts {
		opt(s)
	}
	mux := http.NewServeMux()

	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	})

	if db != nil && starter != nil {
		mux.HandleFunc("POST /api/projects", s.createProjectV2)
		mux.HandleFunc("GET /api/projects", s.listProjects)
		mux.HandleFunc("GET /api/projects/{projectID}/request", s.getRequest)
		mux.HandleFunc("PUT /api/projects/{projectID}/request", s.putRequest)
		mux.HandleFunc("POST /api/projects/{projectID}/sources", s.uploadSource)
		mux.HandleFunc("GET /api/projects/{projectID}/sources/{label}/download", s.downloadSource)
		mux.HandleFunc("POST /api/projects/{projectID}/request/submit", s.submitRequest)
		mux.HandleFunc("GET /api/projects/{projectID}/understanding", s.getUnderstanding)
		mux.HandleFunc("POST /api/projects/{projectID}/cases/{label}/confirm", s.confirmCase)
		mux.HandleFunc("POST /api/projects/{projectID}/cases/{label}/correct", s.correctCase)
		mux.HandleFunc("PUT /api/projects/{projectID}/sections/{label}/role", s.putSectionRole)
		mux.HandleFunc("POST /api/projects/{projectID}/sections", s.createSection)
		mux.HandleFunc("GET /api/projects/{projectID}/sections", s.listSections)
		mux.HandleFunc("PUT /api/projects/{projectID}/sections/{label}", s.putSection)
		mux.HandleFunc("POST /api/projects/{projectID}/messages", s.postMessage)
		mux.HandleFunc("GET /api/projects/{projectID}/todo", s.getTodo)
		mux.HandleFunc("GET /api/projects/{projectID}/sections/{label}", s.getSection)
		mux.HandleFunc("GET /api/projects/{projectID}/sections/{label}/history", s.getBlockHistory)
		mux.HandleFunc("GET /api/projects/{projectID}/summary", s.getSummary)
		mux.HandleFunc("POST /api/projects/{projectID}/editor", s.claimEditor)
		mux.HandleFunc("GET /api/projects/{projectID}/cards/{label}", s.getCard)
		mux.HandleFunc("POST /api/projects/{projectID}/cards/{label}/answer", s.answerCard)
		mux.HandleFunc("GET /api/projects/{projectID}/frame", s.getFrame)
		mux.HandleFunc("POST /api/projects/{projectID}/frame/decisions/{label}/seen", s.postFrameDecisionSeen)
		// P7, the signoff (backlog-r1 1.7, B-13, B-14).
		mux.HandleFunc("POST /api/projects/{projectID}/sections/{label}/signoff", s.requestSignoff)
		mux.HandleFunc("GET /api/projects/{projectID}/signoffs/{label}", s.getSignoff)
		mux.HandleFunc("POST /api/projects/{projectID}/signoffs/{label}/sign", s.signSignoff)
		mux.HandleFunc("POST /api/projects/{projectID}/signoffs/{label}/leave", s.leaveSignoff)
		if signoffs, ok := starter.(SignoffStarter); ok {
			s.signoffs = signoffs
			mux.HandleFunc("POST /api/projects/{projectID}/signoffs/{label}/explanation", s.postSignoffExplanation)
			mux.HandleFunc("POST /api/projects/{projectID}/signoffs/{label}/answers", s.postSignoffAnswers)
			mux.HandleFunc("POST /api/projects/{projectID}/signoffs/{label}/ask", s.postSignoffAsk)
		}
	}

	mux.HandleFunc("GET /api/projects/{projectID}/turns/{turnID}/stream", s.streamTurn)
	registerDevRoutes(mux, s)

	staticFS, err := fs.Sub(staticFiles, "static")
	if err != nil {
		panic(err)
	}
	mux.Handle("GET /", http.FileServer(http.FS(staticFS)))

	return requestLog(logger, securityHeaders(mux))
}
