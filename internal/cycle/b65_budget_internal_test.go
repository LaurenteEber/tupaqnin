package cycle

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/LaurenteEber/tupaqnin/internal/provider"
)

// blockingProvider holds every call until release closes, counting how
// many got through the budget.
type blockingProvider struct {
	mu      sync.Mutex
	entered int
	release chan struct{}
	cost    float64
}

func (p *blockingProvider) Respond(ctx context.Context, req provider.Request, onDelta func(provider.Delta)) (provider.Response, error) {
	p.mu.Lock()
	p.entered++
	p.mu.Unlock()
	<-p.release
	return provider.Response{Usage: provider.Usage{CostUSD: p.cost}}, nil
}

func (p *blockingProvider) count() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.entered
}

// B-65 «reserva del máximo antes de cada llamada»: with 0.50 USD spent
// of a 0.60 round cap, three readers asking at once cannot all start:
// each call reserves before it starts, so the calls in flight never
// take the round past its cap.
func TestB65_ParallelReadersReserveBeforeEachCall(t *testing.T) {
	inner := &blockingProvider{release: make(chan struct{}), cost: readerCallReserveUSD}
	budget := &roundBudget{capUSD: 0.60, projectCap: 3, spent: 0.50}
	p := budgetProvider{inner: inner, budget: budget}
	var wg sync.WaitGroup
	errs := make(chan error, 3)
	for i := 0; i < 3; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := p.Respond(context.Background(), provider.Request{}, nil)
			errs <- err
		}()
	}
	deadline := time.Now().Add(2 * time.Second)
	for inner.count() < 3 && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
		if inner.count() == 2 && len(errs) == 1 {
			break
		}
	}
	close(inner.release)
	wg.Wait()
	close(errs)
	var capped int
	for err := range errs {
		if err == errRoundCap {
			capped++
		}
	}
	if inner.count() != 2 || capped != 1 {
		t.Fatalf("calls started %d, refused by the round cap %d; want 2 and 1", inner.count(), capped)
	}
	if budget.spent > budget.capUSD+1e-9 || budget.reserved != 0 {
		t.Fatalf("spent %.2f (cap %.2f), reserved %.2f after the calls; want within the cap and nothing reserved", budget.spent, budget.capUSD, budget.reserved)
	}
}
