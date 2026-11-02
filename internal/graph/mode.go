package graph

import (
	"context"
	"encoding/json"

	"github.com/LaurenteEber/tupaqnin/internal/store"
)

// The modes of mode_set (glossary: normal · «continúa hasta terminar»).
const (
	ModeNormal          = "normal"
	ModeRunToCompletion = "run_to_completion"
)

// Mode is what the history says of the mode (espec. §1.3): the last mode_set,
// with your message in literal. A project without one is normal.
type Mode struct {
	Mode    string
	Message string
	// Steps counts the principal's steps since the mode was activated
	// (B-83: its cap), from the history, so a new turn keeps counting.
	Steps int
	// Unread: the principal took no step since this mode was set, so a live
	// conversation has not heard of it yet (a startup carries it in layer 3).
	Unread bool
}

// On reports the mode «continúa hasta terminar».
func (m Mode) On() bool { return m.Mode == ModeRunToCompletion }

// CurrentMode reads the mode from the history.
func CurrentMode(ctx context.Context, q store.Querier, projectID string) (Mode, error) {
	events, err := q.EventsSince(ctx, projectID, 0)
	if err != nil {
		return Mode{}, err
	}
	return modeOf(events), nil
}

func modeOf(events []store.Event) Mode {
	mode := Mode{Mode: ModeNormal}
	for _, ev := range events {
		switch ev.Type {
		case store.EventModeSet:
			var data struct {
				Mode    string `json:"mode"`
				Message string `json:"message"`
			}
			_ = json.Unmarshal(ev.Data, &data)
			if data.Mode == "" {
				continue
			}
			mode = Mode{Mode: data.Mode, Message: data.Message, Unread: true}
		case store.EventStepEnded:
			var data struct {
				TurnKind string `json:"turn_kind"`
			}
			_ = json.Unmarshal(ev.Data, &data)
			if data.TurnKind == "step" {
				mode.Steps++
				mode.Unread = false
			}
		}
	}
	return mode
}

// modeFinishedSinceYou reports that the app ended the mode (B-85) after your last
// event: the return line is due whatever time passed (M-2, espec. §9). Turning it
// off yourself is your last event, so it never counts.
func modeFinishedSinceYou(events []store.Event) bool {
	last, ok := lastHumanEvent(events)
	if !ok {
		return false
	}
	for _, ev := range events {
		if ev.Seq > last.Seq && ev.Type == store.EventModeSet && ev.Actor == "app" {
			return true
		}
	}
	return false
}
