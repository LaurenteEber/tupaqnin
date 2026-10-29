package fingerprint_test

import (
	"testing"

	"github.com/LaurenteEber/tupaqnin/internal/fingerprint"
)

func TestB9_SourceFingerprintIgnoresLineEndingsAndNormalization(t *testing.T) {
	a := fingerprint.Source("línea uno\nlínea dos\n")
	b := fingerprint.Source("línea uno\r\nlínea dos\r\n")
	if a == "" {
		t.Fatal("Source: empty fingerprint")
	}
	if a != b {
		t.Fatalf("Source: NFC/line-ending variants differ:\n%s\n%s", a, b)
	}
	if fingerprint.Source("otro texto") == a {
		t.Fatal("Source: different texts share a fingerprint")
	}
}
