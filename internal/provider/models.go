package provider

import (
	"context"
	"errors"
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

// The checker judges each step with Spark 1.3 at effort low, in one
// call with a clean context (backlog-r2 2.3, fichas §7).
const (
	CheckerModel  = ModelSpark13
	CheckerEffort = EffortLow
)

// The judge prepares the defense at the stop with Spark 1.3 at effort
// high, in one call with a clean context (backlog-r2 2.6, fichas §7).
const (
	JudgeDefenseModel  = ModelSpark13
	JudgeDefenseEffort = EffortHigh
)

// The judge verifies a section with Spark 1.3 at effort high, in a clean
// context (backlog-r3 3.6, fichas §1 and §7).
const (
	JudgeVerifyModel  = ModelSpark13
	JudgeVerifyEffort = EffortHigh
)

// What one model call reserves at least, by role, before it starts (B-89,
// [inicial]): each is the maximum of one call unless the dearest call of
// that role in the project cost more.
const (
	PrincipalReserveUSD  = 0.15
	JudgeReserveUSD      = 0.05
	DigestReserveUSD     = 0.05
	CheckerReserveUSD    = 0.05
	ContrasterReserveUSD = 0.05
	// ReaderReserveUSD is one reader call (3.0: a whole reader cost 0.025
	// to 0.042 USD, over 4 to 6 calls).
	ReaderReserveUSD = 0.05
	// SignoffReserveUSD is set aside of the project's cap for the signoff
	// (B3-20, B-89: ≈ 2 contrasts [inicial]).
	SignoffReserveUSD = 0.40
)

// ReserveUSD is what one call of this role and turn kind reserves at
// least (B-89). The digest is a call of the principal with a reserve of
// its own.
// [EL DISEÑO NO LO DICE]: «the dearest call of that role» for the digest:
// model_call keeps no turn kind, so it is the principal's dearest.
func ReserveUSD(role AgentRole, kind TurnKind) float64 {
	switch {
	case kind == TurnDigest:
		return DigestReserveUSD
	case role == RoleReader:
		return ReaderReserveUSD
	case role == RoleJudge:
		return JudgeReserveUSD
	case role == RoleChecker:
		return CheckerReserveUSD
	case role == RoleContraster:
		return ContrasterReserveUSD
	}
	return PrincipalReserveUSD
}

// The stop reasons of the caps (B-89), and which of the demo's caps
// refused the call (data.cap).
const (
	CapProjectStop = "project_cap"
	CapDemoStop    = "demo_cap"
	CapLink        = "link"
	CapDay         = "day"
	CapTotal       = "total"
)

// CapError is a model call the spend gate refused before it started
// (B-89): Reason is project_cap or demo_cap, and Cap says which of the
// demo's caps (link, day, total; empty for project_cap).
type CapError struct {
	Reason string
	Cap    string
}

func (e *CapError) Error() string {
	if e.Cap == "" {
		return "provider: spend cap reached: " + e.Reason
	}
	return "provider: spend cap reached: " + e.Reason + " " + e.Cap
}

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
// The status is the error's own HTTPStatus field, so it does not depend
// on Meta's body (Eber, 29-oct).
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
		return perr.HTTPStatus == 429 || perr.HTTPStatus >= 500
	}
	return false
}
