package cycle

import (
	"strconv"
	"strings"
	"unicode/utf16"
)

// Segment is a piece of one streamed field: Start marks the first piece
// of a new value, so the page can show the triage line, each text and
// the note apart (P-28).
type Segment struct {
	Field string
	Text  string
	Start bool
}

// fieldFilter turns the raw JSON a model streams into what the person
// reads (P-28): only the string values of the chosen keys, decoded, as
// they arrive, each with its key. The raw JSON never reaches the stream
// (except in a devroutes build, rawStream). It reads one JSON document
// per call; reset starts the next.
type fieldFilter struct {
	keys map[string]bool

	stack     []byte // 'o' object, 'a' array
	expectKey bool
	inString  bool
	isKey     bool
	emitting  bool
	escape    bool
	hex       []byte // pending \uXXXX digits
	pendingHi rune   // high surrogate waiting for its pair
	key       strings.Builder
	lastKey   string
	emitted   bool // something was emitted in this turn
	startNext bool // the next emitted piece opens a new value
	out       []Segment
	published bool // a plain-text publisher already got something
}

// Keys streamed for the principal's step and for the understanding.
var (
	stepStreamKeys          = []string{"triage_line", "text", "note"}
	understandingStreamKeys = []string{"understood"}
)

func newFieldFilter(keys []string) *fieldFilter {
	f := &fieldFilter{keys: make(map[string]bool, len(keys))}
	for _, k := range keys {
		f.keys[k] = true
	}
	return f
}

// reset prepares the filter for the next document of the same turn.
func (f *fieldFilter) reset() {
	*f = fieldFilter{keys: f.keys, emitted: f.emitted, published: f.published}
}

// feed takes the next raw chunk and returns the pieces to stream.
func (f *fieldFilter) feed(chunk string) []Segment {
	f.out = nil
	for _, r := range chunk {
		if f.inString {
			f.stringRune(r)
			continue
		}
		switch r {
		case '{':
			f.stack = append(f.stack, 'o')
			f.expectKey = true
		case '[':
			f.stack = append(f.stack, 'a')
		case '}', ']':
			if len(f.stack) > 0 {
				f.stack = f.stack[:len(f.stack)-1]
			}
		case ',':
			f.expectKey = len(f.stack) > 0 && f.stack[len(f.stack)-1] == 'o'
		case ':':
			f.expectKey = false
		case '"':
			f.inString = true
			f.isKey = f.expectKey && len(f.stack) > 0 && f.stack[len(f.stack)-1] == 'o'
			f.key.Reset()
			f.emitting = !f.isKey && f.keys[f.lastKey]
			f.startNext = f.emitting
		}
	}
	return f.out
}

func (f *fieldFilter) stringRune(r rune) {
	if f.hex != nil {
		f.hex = append(f.hex, byte(r))
		if len(f.hex) < 4 {
			return
		}
		n, err := strconv.ParseUint(string(f.hex), 16, 32)
		f.hex = nil
		if err != nil {
			return
		}
		u := rune(n)
		switch {
		case utf16.IsSurrogate(u) && f.pendingHi == 0:
			f.pendingHi = u
			return
		case f.pendingHi != 0:
			u = utf16.DecodeRune(f.pendingHi, u)
			f.pendingHi = 0
		}
		f.write(u)
		return
	}
	if f.escape {
		f.escape = false
		switch r {
		case 'n':
			f.write('\n')
		case 't':
			f.write('\t')
		case 'r':
			f.write('\r')
		case 'b', 'f':
		case 'u':
			f.hex = []byte{}
		default:
			f.write(r)
		}
		return
	}
	switch r {
	case '\\':
		f.escape = true
	case '"':
		f.inString = false
		if f.isKey {
			f.lastKey = f.key.String()
		}
		f.emitting = false
	default:
		f.write(r)
	}
}

func (f *fieldFilter) write(r rune) {
	if f.isKey {
		f.key.WriteRune(r)
		return
	}
	if !f.emitting {
		return
	}
	if f.startNext || len(f.out) == 0 {
		f.out = append(f.out, Segment{Field: f.lastKey, Start: f.startNext})
		f.startNext = false
	}
	f.out[len(f.out)-1].Text += string(r)
	f.emitted = true
}
