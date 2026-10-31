//go:build devroutes

package server_test

import (
	"net/http"
	"testing"
)

// backlog-r2 2.7, «en el navegador»: in a development build, moving the
// server's clock forward shows the return line (M-2) without waiting
// three days; the history keeps its real dates.
func TestM2_DevClockShowsTheReturnLine(t *testing.T) {
	h, db, _ := newAPI(t)
	projectID := seedCase5Frame(t, h, db)
	editSection(t, h, db, projectID, "sec-1", synthesisText+"\n\nUna línea tuya.")
	if f := getFrame(t, h, projectID); f.ReturnLine != nil {
		t.Fatalf("return line before moving the clock: %+v", f.ReturnLine)
	}
	if rec := do(t, h, http.MethodPost, "/api/dev/clock", map[string]any{"days": 4}); rec.Code != http.StatusNoContent {
		t.Fatalf("dev clock: %d (%s)", rec.Code, rec.Body.String())
	}
	t.Cleanup(func() { do(t, h, http.MethodPost, "/api/dev/clock", map[string]any{"days": 0}) })
	if f := getFrame(t, h, projectID); f.ReturnLine == nil || f.ReturnLine.Days != 4 {
		t.Fatalf("return line after moving the clock 4 days: %+v", f.ReturnLine)
	}
}
