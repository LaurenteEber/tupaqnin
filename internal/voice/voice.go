// Package voice prepares the repo for voice input (B3-14): speaking
// your signing explanation and talking to the agent. It only declares
// the transcription interface; there is no implementation yet. Voice
// arrives after the text-only cycle works, without touching it: what
// reaches the cycle is always text, and audio is never stored.
package voice

import (
	"context"
	"io"
)

// Transcriber turns audio into text. It has no implementation yet;
// adding one (Meta transcription) and a microphone button to the
// shared text input component is all voice will take.
type Transcriber interface {
	Transcribe(ctx context.Context, audio io.Reader, lang string) (string, error)
}
