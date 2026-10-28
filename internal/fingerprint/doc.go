// Package fingerprint implements the tq-md/1 fingerprint
// (specification §4): SHA-256 of a canonical JSON (JCS) encoding of
// the goldmark GFM tree, so a section fingerprint depends on its
// structure, not its formatting.
package fingerprint
