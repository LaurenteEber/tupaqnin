package evidence

import (
	"context"
	"errors"
	"testing"
)

// Backlog-r3 3.2: with TQ_PROVIDER=fake the evidence answers from the
// embedded recordings and never reaches the network.
func TestB55_ReplayAnswersFromRecordingsAndNeverReachesTheNetwork(t *testing.T) {
	c := NewReplay()
	hits, err := c.SearchLiterature(context.Background(), Query{
		Text: "microplastics drinking water detection methods FTIR Raman minimum size", Where: ServiceOpenAlex, YearFrom: 2018})
	if err != nil || len(hits) == 0 || hits[0].Title == "" {
		t.Fatalf("recorded search: %d hits, %v", len(hits), err)
	}
	none, err := c.SearchLiterature(context.Background(), Query{Text: "not recorded at all", Where: ServiceOpenAlex})
	if err != nil || len(none) != 0 {
		t.Fatalf("unrecorded search: %v, %v", none, err)
	}
	if _, err := c.FetchText(context.Background(), hits[0].URL); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("unrecorded page: %v, want ErrUnavailable («no verificado», never «no existe»)", err)
	}
	if _, ok := c.http.Transport.(replayTransport); !ok {
		t.Fatal("the replay's transport is not the recordings")
	}
}
