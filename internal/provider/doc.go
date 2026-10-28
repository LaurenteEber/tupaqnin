// Package provider is the client for Meta's Responses API plus a
// fake provider for tests (specification §7). It stores every raw
// output item and resends it byte-for-byte, with store: false and
// include: ["reasoning.encrypted_content"].
package provider
