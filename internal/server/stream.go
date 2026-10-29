package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// streamTurn serves GET /api/projects/{projectID}/turns/{turnID}/stream
// as SSE (B-1): everything the turn has emitted, replayed from the
// start, then live events as they come, with a heartbeat every
// HeartbeatInterval. The handler is the only writer on its connection.
// There is no write deadline on this route (specification §6); the
// server's IdleTimeout is set in main.
func (s *Server) streamTurn(w http.ResponseWriter, r *http.Request) {
	projectID := r.PathValue("projectID")
	turnID := r.PathValue("turnID")

	// P-9: a turn that was never begun does not exist. Answer 404
	// without creating a hub entry (the hub lives in memory, so this
	// also covers turns from before a restart).
	if !s.hub.HasTurn(projectID, turnID) {
		http.NotFound(w, r)
		return
	}

	_ = http.NewResponseController(w).SetWriteDeadline(time.Time{})

	h := w.Header()
	h.Set("Content-Type", "text/event-stream")
	h.Set("Cache-Control", "no-cache")
	h.Set("Connection", "keep-alive")
	h.Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)

	flusher, ok := w.(http.Flusher)
	if !ok {
		return
	}
	flusher.Flush()

	heartbeat := time.NewTicker(s.hub.HeartbeatInterval)
	defer heartbeat.Stop()

	index := 0
	for {
		events, done, changed := s.hub.snapshot(projectID, turnID, index)
		for _, ev := range events {
			if !writeStreamEvent(w, ev) {
				return
			}
			index++
		}
		if len(events) > 0 {
			flusher.Flush()
		}
		if done {
			if remaining, _, _ := s.hub.snapshot(projectID, turnID, index); len(remaining) == 0 {
				return
			}
		}
		select {
		case <-r.Context().Done():
			return
		case <-changed:
		case <-heartbeat.C:
			if _, err := fmt.Fprint(w, ": heartbeat\n\n"); err != nil {
				return
			}
			flusher.Flush()
		}
	}
}

// writeStreamEvent writes one SSE event. A step forwards its raw
// result; a delta its text; an error only code and catalog message.
func writeStreamEvent(w http.ResponseWriter, ev StreamEvent) bool {
	var data []byte
	var err error
	switch ev.Type {
	case EventStep:
		data = ev.Step
	case EventError:
		data, err = json.Marshal(struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		}{Code: ev.Code, Message: ev.Message})
	default: // delta
		data, err = json.Marshal(struct {
			Text  string `json:"text"`
			Field string `json:"field,omitempty"`
			Start bool   `json:"start,omitempty"`
		}{Text: ev.Text, Field: ev.Field, Start: ev.Start})
	}
	if err != nil {
		return false
	}
	_, err = fmt.Fprintf(w, "event: %s\ndata: %s\n\n", ev.Type, data)
	return err == nil
}
