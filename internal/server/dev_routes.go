//go:build devroutes

package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync/atomic"

	"github.com/LaurenteEber/tupaqnin/internal/provider"
)

// Development-only routes (B-4): compiled only with -tags=devroutes,
// never in the demo binary.

var devStreamCounter atomic.Int64

func registerDevRoutes(mux *http.ServeMux, s *Server) {
	mux.HandleFunc("GET /api/dev/stream-test", s.devStreamTest)
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
