//go:build devroutes

package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync/atomic"
	"time"

	"github.com/LaurenteEber/tupaqnin/internal/provider"
)

// Development-only routes (B-4): compiled only with -tags=devroutes,
// never in the demo binary.

var devStreamCounter atomic.Int64

func registerDevRoutes(mux *http.ServeMux, s *Server) {
	mux.HandleFunc("GET /api/dev/stream-test", s.devStreamTest)
	// backlog-r2 2.7: the return line (M-2) reads this clock.
	base := s.now
	s.now = func() time.Time { return base().Add(time.Duration(devClockShift.Load())) }
	mux.HandleFunc("POST /api/dev/clock", s.devClock)
}

// devClockShift is how far the dev clock runs ahead of the real one.
var devClockShift atomic.Int64

// devClock moves the server's clock {days} ahead of the real one (0
// puts it back), so a development build shows the return line (M-2)
// without waiting; the history keeps its real dates.
// [EL DISEÑO NO LO DICE]: the route; «cambiar la fecha en una prueba de
// desarrollo» names the check, not the call.
func (s *Server) devClock(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Days int `json:"days"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Days < 0 {
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}
	devClockShift.Store(int64(time.Duration(req.Days) * 24 * time.Hour))
	w.WriteHeader(http.StatusNoContent)
}

// devStreamTest emits the recorded stream-90s fixture through the
// fake provider (backlog 0.6): GET it and watch 90 s of deltas.
func (s *Server) devStreamTest(w http.ResponseWriter, r *http.Request) {
	fixture, err := provider.Stream90Fixture()
	if err != nil {
		http.Error(w, "fixture unavailable", http.StatusInternalServerError)
		return
	}
	turnID := fmt.Sprintf("dev-%d", devStreamCounter.Add(1))
	s.hub.BeginTurn("dev", turnID)
	fake := provider.NewFake(fixture)
	go func() {
		resp, err := fake.Respond(r.Context(), provider.Request{
			ProjectID: "dev", TurnID: turnID, Role: provider.RolePrincipal,
			Model: provider.PrincipalModel,
		}, func(d provider.Delta) {
			s.hub.PublishDelta("dev", turnID, d.Text)
		})
		if err != nil {
			s.hub.PublishError("dev", turnID, "provider_error")
			return
		}
		step, _ := json.Marshal(map[string]any{
			"status": "completed", "fixture": fixture.Name, "text": resp.Text})
		s.hub.PublishStep("dev", turnID, step)
	}()

	// Serve this request from the turn just started.
	r2 := r.Clone(r.Context())
	r2.SetPathValue("projectID", "dev")
	r2.SetPathValue("turnID", turnID)
	s.streamTurn(w, r2)
}
