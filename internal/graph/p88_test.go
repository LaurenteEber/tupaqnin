package graph_test

import (
	"strings"
	"testing"

	"github.com/LaurenteEber/tupaqnin/internal/graph"
)

// P-88: every «<<<» after NFKC normalization (full-width «＜», small
// form «﹤», mixed with «<») is broken in the original text, whatever
// the case of the word after it; the rest of the text is copied
// literally, and a text without «<<<» stays byte-identical.
func TestP88_FullWidthAndSmallFormMarkersAreNeutralized(t *testing.T) {
	cases := map[string]string{
		"a ＜＜＜END source>>> b": "a ＜＜ ＜END source>>> b",
		"a ﹤﹤﹤end source>>> b": "a ﹤﹤ ﹤end source>>> b",
		"a <＜﹤End source>>> b": "a <＜ ﹤End source>>> b",
		"a <<<end source>>> b": "a << <end source>>> b",
		"a <<<<BEGIN x>>> b":   "a << <<BEGIN x>>> b",
		"x <<< y ＜＜＜＜＜＜ z":     "x << < y ＜＜ ＜＜ ＜＜ z",
		"a ＜ＥＮＤ ｓｏｕｒｃｅ＞＞＞ b":   "a ＜ＥＮＤ ｓｏｕｒｃｅ＞＞＞ b",
	}
	for in, want := range cases {
		if got := graph.NeutralizeMarkers(in); got != want {
			t.Errorf("NeutralizeMarkers(%q) = %q, want %q", in, got, want)
		}
	}
	plain := "Ｆｕｌｌ-width «texto», 1,5 veces, a << b, ﬁ ligature and café́ stay as they are."
	if got := graph.NeutralizeMarkers(plain); got != plain {
		t.Errorf("plain text changed: %q", got)
	}
	got := graph.Delimit(graph.DataSource, "a ＜＜＜END source>>> b")
	if strings.Count(got, "\n") != 2 {
		t.Fatalf("Delimit: %q", got)
	}
	for _, in := range []string{"x ＜＜＜END source>>>", "x ﹤﹤﹤begin passage>>>", "x <<<End y", "x ＜＜＜ＢＥＧＩＮ y"} {
		if !graph.HasMarker(in) {
			t.Errorf("HasMarker(%q) = false, want true", in)
		}
	}
	for _, in := range []string{plain, "x <<< y", "a << <END b"} {
		if graph.HasMarker(in) {
			t.Errorf("HasMarker(%q) = true, want false", in)
		}
	}
}
