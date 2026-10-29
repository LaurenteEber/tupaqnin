package provider

import (
	"context"
	"errors"
	"strings"
)

// Model ids and context windows, taken from Meta's model docs
// (https://dev.meta.ai/docs/models, read 2026-10-28). Changing the
// principal's model is changing PrincipalModel and deploying
// (backlog 0.5).
const (
	ModelSpark13  = "muse-spark-1.3"
	ModelSpark12  = "muse-spark-1.2"
	ModelSpark11  = "muse-spark-1.1"
	ModelWonder10 = "muse-wonder-1.0"
)

// The principal's model and effort (bloque-1-fichas §1: Spark 1.3,
// medium; high in the «Entendí» and for critical decisions, chosen by
// the caller per step, not here).
const (
	PrincipalModel  = ModelSpark13
	PrincipalEffort = "medium"
)

// Effort levels confirmed in Meta's reasoning docs
// (https://dev.meta.ai/docs/reasoning): minimal, low, medium, high,
// xhigh, max. They are passed through, never invented per call site.
const (
	EffortMinimal = "minimal"
	EffortLow     = "low"
	EffortMedium  = "medium"
	EffortHigh    = "high"
	EffortXHigh   = "xhigh"
	EffortMax     = "max"
)

// contextWindows in tokens, per Meta's model docs (all Muse Spark
// versions and Muse Wonder: 1,048,576).
var contextWindows = map[string]int{
	ModelSpark13:  1_048_576,
	ModelSpark12:  1_048_576,
	ModelSpark11:  1_048_576,
	ModelWonder10: 1_048_576,
}

// Standard-mode prices in USD per million tokens, from
// https://dev.meta.ai/docs/pricing-rate-limits (2026-10-28).
// Cached-input rates are not applied: Usage carries no cached count,
// so every input token is priced at the input rate.
type pricing struct{ inputPerMillion, outputPerMillion float64 }

var pricings = map[string]pricing{
	ModelSpark13:  {inputPerMillion: 1.25, outputPerMillion: 4.25},
	ModelSpark12:  {inputPerMillion: 1.25, outputPerMillion: 4.25},
	ModelSpark11:  {inputPerMillion: 1.25, outputPerMillion: 4.25},
	ModelWonder10: {inputPerMillion: 4.00, outputPerMillion: 20.00},
}

// ContextWindow returns the model's context window in tokens, or 0
// for a model the docs have not confirmed.
func ContextWindow(model string) int { return contextWindows[model] }

// CostUSD prices a call at Standard-mode rates. Unknown models cost 0.
func CostUSD(model string, inputTokens, outputTokens int) float64 {
	p, ok := pricings[model]
	if !ok {
		return 0
	}
	return float64(inputTokens)/1e6*p.inputPerMillion +
		float64(outputTokens)/1e6*p.outputPerMillion
}

// The contraster (Wonder, glossary §5) signs off with Wonder 1.0 at
// effort high; when Wonder is not available, Spark 1.3 at xhigh
// contrasts instead, and the signoff says so (flujos §3, backlog-r1
// 1.7).
const (
	ContrasterModel          = ModelWonder10
	ContrasterEffort         = EffortHigh
	ContrasterFallbackModel  = ModelSpark13
	ContrasterFallbackEffort = EffortXHigh
)

// Known reports whether the model is in this file (B-20 (c): a model
// that is not here is not available).
func Known(model string) bool {
	_, ok := contextWindows[model]
	return ok
}

// Unavailable reports whether a call error means the model is not
// available (B-20 (c)): the call timed out, or the provider answered
// 5xx or 429. A 4xx validation error is not: it is an error of the
// turn. The caller retries once before treating the model as not
// available.
// [EL DISEÑO NO LO DICE]: the status is read from the ProviderError
// code http_<status>, which Meta's own error code replaces when its
// body names one; such an error counts as an error of the turn.
func Unavailable(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}
	var timeout interface{ Timeout() bool }
	if errors.As(err, &timeout) && timeout.Timeout() {
		return true
	}
	var perr *ProviderError
	if errors.As(err, &perr) {
		return perr.Code == "http_429" || strings.HasPrefix(perr.Code, "http_5")
	}
	return false
}
