// Package cycle implements the end-of-step cycle: the agent write
// gate and its validation, checks, section states and signing
// (block 1, specification §2–§4). The agent never writes directly;
// the app validates its fixed-shape step output and applies it.
package cycle
