package fingerprint

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"

	"golang.org/x/text/unicode/norm"
)

// Source returns the source fingerprint (specification §4, B-9):
// SHA-256, hex encoded, of the text in NFC with line endings
// normalized to "\n". For a PDF it is taken over the extracted text.
func Source(text string) string {
	s := strings.ReplaceAll(text, "\r\n", "\n")
	s = strings.ReplaceAll(s, "\r", "\n")
	sum := sha256.Sum256([]byte(norm.NFC.String(s)))
	return hex.EncodeToString(sum[:])
}
