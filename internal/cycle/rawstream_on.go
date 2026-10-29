//go:build devroutes

package cycle

// rawStream: in a development build (-tags=devroutes, B-4) the model's
// raw text is also streamed, as field "raw" (P-28).
const rawStream = true
