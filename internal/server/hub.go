package server

import (
	"encoding/json"
	"sync"
	"time"
)

// Stream event types (backlog 0.6). A heartbeat is an SSE comment, not
// an event, and is never stored for replay.
const (
	EventDelta = "delta"
	EventStep  = "step"
	EventError = "error"
)

// StreamEvent is one stored turn event. Delta carries Text, step
// carries the raw step result (Step), and error carries only a Code
// and a catalog Message: never a raw provider error (§6).
type StreamEvent struct {
	Type    string
	Text    string
	Step    json.RawMessage
	Code    string
	Message string
}

// errorMessages is the message catalog for stream errors.
// [EL DISEÑO NO LO DICE]: which catalog serves server-side messages
// is not stated (the on-screen catalog is web/src/i18n, T-1); this is
// the smallest choice, in English like the demo (H-1).
var errorMessages = map[string]string{
	"provider_error": "The model call failed. Please try again.",
	"agent_pending":  "The agent could not finish this step.",
	"turn_failed":    "The turn failed. Please try again.",
}

func catalogMessage(code string) string {
	if msg, ok := errorMessages[code]; ok {
		return msg
	}
	return errorMessages["turn_failed"]
}

// Hub stores what each turn has emitted so a stream opened late
// replays it from the start (B-1), and wakes open streams when new
// events arrive. Events are kept for the life of the process.
type Hub struct {
	// HeartbeatInterval is how often an idle stream sends a
	// heartbeat (specification §6: every 15 s).
	HeartbeatInterval time.Duration

	mu    sync.Mutex
	turns map[turnKey]*turnStream
}

type turnKey struct{ projectID, turnID string }

type turnStream struct {
	events  []StreamEvent
	done    bool
	changed chan struct{} // closed and replaced on every publish
}

// NewHub returns a hub with the 15 s heartbeat of specification §6.
func NewHub() *Hub {
	return &Hub{HeartbeatInterval: 15 * time.Second, turns: make(map[turnKey]*turnStream)}
}

func (h *Hub) turn(projectID, turnID string) *turnStream {
	key := turnKey{projectID, turnID}
	t, ok := h.turns[key]
	if !ok {
		t = &turnStream{changed: make(chan struct{})}
		h.turns[key] = t
	}
	return t
}

func (h *Hub) publish(projectID, turnID string, ev StreamEvent, terminal bool) {
	h.mu.Lock()
	defer h.mu.Unlock()
	t := h.turn(projectID, turnID)
	if t.done {
		return
	}
	t.events = append(t.events, ev)
	if terminal {
		t.done = true
	}
	close(t.changed)
	t.changed = make(chan struct{})
}

// PublishDelta stores and forwards one text delta of the turn.
func (h *Hub) PublishDelta(projectID, turnID, text string) {
	h.publish(projectID, turnID, StreamEvent{Type: EventDelta, Text: text}, false)
}

// PublishStep stores the step result and ends the turn's stream.
func (h *Hub) PublishStep(projectID, turnID string, step json.RawMessage) {
	h.publish(projectID, turnID, StreamEvent{Type: EventStep, Step: step}, true)
}

// PublishError stores an error (code and catalog message only) and
// ends the turn's stream.
func (h *Hub) PublishError(projectID, turnID, code string) {
	h.publish(projectID, turnID, StreamEvent{
		Type: EventError, Code: code, Message: catalogMessage(code)}, true)
}

// snapshot returns the events after index, whether the turn is done,
// and a channel closed when anything new is published.
func (h *Hub) snapshot(projectID, turnID string, index int) ([]StreamEvent, bool, <-chan struct{}) {
	h.mu.Lock()
	defer h.mu.Unlock()
	t := h.turn(projectID, turnID)
	var out []StreamEvent
	if index < len(t.events) {
		out = append(out, t.events[index:]...)
	}
	return out, t.done, t.changed
}
