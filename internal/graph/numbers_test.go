package graph_test

import (
	"reflect"
	"strings"
	"testing"

	"github.com/LaurenteEber/tupaqnin/internal/graph"
)

func TestB20b_WhatCountsAsANumber(t *testing.T) {
	text := "1. Primer ítem con 60 veces\n2) Segundo, en 2023, ver sec-3 y new-12.\nUn 1,5 y 1.5 y 1\u00a0500."
	var got []string
	for _, n := range graph.Numbers(text) {
		got = append(got, n.Raw+"="+n.Value)
	}
	want := []string{"60=60", "2023=2023", "1,5=1.5", "1.5=1.5", "1\u00a0500=1500"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("numbers:\n got %q\nwant %q", got, want)
	}
}

func TestT4_CompareCollapsesSpaceAndFindsOverlappingMatches(t *testing.T) {
	c := graph.Compare("aa  aa\n\naa")
	if c.Text != "aa aa aa" {
		t.Fatalf("compared text: %q", c.Text)
	}
	if got := c.Find("aa  aa"); len(got) != 2 || c.Orig[got[1][0]:got[1][1]] != "aa\n\naa" {
		t.Fatalf("find: %v", got)
	}
}

func TestS3R4_ProvisionalIDsAreFoundAfterNormalizing(t *testing.T) {
	// Cyrillic е and a non-breaking hyphen still spell new-2.
	if got := graph.ProvisionalIDsIn("Ver n\u0435w\u20112 y renew-3x."); !reflect.DeepEqual(got, []string{"new-2"}) {
		t.Fatalf("provisional ids: %v", got)
	}
}

// C.14, B-104: a power is one figure, however it is written.
func TestB104_APowerIsTheSameFigureHoweverItIsWritten(t *testing.T) {
	want := map[string]string{
		"10⁴": "10^4", "10^4": "10^4", "1e4": "10^4", "1E4": "10^4", "10⁻³": "10^-3", "10^-3": "10^-3",
		"3,14e4": "3.14e4", "1 500": "1500", "1,5": "1.5", "104": "104", "10": "10",
	}
	for raw, value := range want {
		if got := graph.NormalizeFigure(raw); got != value {
			t.Fatalf("NormalizeFigure(%q) = %q, want %q", raw, got, value)
		}
	}
	values := func(text string) string {
		var out []string
		for _, n := range graph.Numbers(text) {
			out = append(out, n.Value)
		}
		return strings.Join(out, " ")
	}
	for _, text := range []string{"3,14 × 104", "3,14 × 10⁴", "3,14 × 10^4", "3,14×10^4", "3.14 × 10⁴"} {
		if got := values(text); got != "3.14 10^4" {
			t.Fatalf("Numbers(%q) = %q", text, got)
		}
	}
	// Not a lost superscript: no × before it, or not a power of ten.
	for text, value := range map[string]string{"son 104 muestras": "104", "2 × 100": "2 100", "2 × 1000": "2 1000", "2 × 150": "2 150"} {
		if got := values(text); got != value {
			t.Fatalf("Numbers(%q) = %q, want %q", text, got, value)
		}
	}
	if got := values("3,14 × 10⁵"); got == "3.14 10^4" {
		t.Fatal("10⁵ is another figure")
	}
}

// C.14: what is not a power stays what it was: an ordinary multiplication, a
// footnote mark, and digits inside a code, a hash or a colour.
func TestB104_WhatIsNotAPowerReadsAsBefore(t *testing.T) {
	for text, want := range map[string]string{
		"1920 × 1080":          "1920 1080",
		"4 × 1024":             "4 1024",
		"2 × 105 = 210":        "2 105 210",
		"Se incluyeron 120¹.":  "120",
		"ror.org/0003e4m70":    "0003",
		"ror.org/02e6z0y17":    "02",
		"1147691e-fd6d-41e9-a": "1147691 41",
		"8e525e7":              "8",
		"713e79d":              "713",
		"#1e3322":              "1",
		"2^10":                 "2 10",
		"el lote 1e4f":         "1",
		"10^4,5":               "10 4.5",
	} {
		var got []string
		for _, n := range graph.Numbers(text) {
			got = append(got, n.Value)
		}
		if strings.Join(got, " ") != want {
			t.Errorf("Numbers(%q) = %q, want %q", text, strings.Join(got, " "), want)
		}
	}
}

// C.14: a power at the start of a line is a figure, not a list's numbering.
func TestB104_APowerOpeningALineIsNotAListNumber(t *testing.T) {
	for _, text := range []string{"10⁴.\nSigue", "1e4. Sigue", "10^4) Sigue"} {
		if got := graph.Numbers(text); len(got) != 1 || got[0].Value != "10^4" {
			t.Errorf("Numbers(%q) = %+v, want 10^4", text, got)
		}
	}
}

// C.14: the figure is still there only as a whole number: digits added around it
// make another one.
func TestB104_AFigureStillInReadsWholeNumbers(t *testing.T) {
	for _, c := range []struct {
		text, anchor string
		want         bool
	}{
		{"son 3,14 × 10⁴ en total.", "3,14 × 104", true},
		{"son 3,14 × 10^4.", "3,14 × 104", true},
		{"son 3.14×10⁴, en total", "3,14 × 104", true},
		{"vale 3,14.", "3,14", true},
		{"son 13,14 × 10⁴ en total", "3,14 × 104", false},
		{"son 3,14 × 10⁴⁵ en total", "3,14 × 104", false},
		{"son 3,14 × 10^40 en total", "3,14 × 104", false},
		{"son 10^45", "10⁴", false},
		{"son 10^45", "1e4", false},
		{"son 13.14", "3,14", false},
		{"son 3.149", "3,14", false},
		{"son 15.02", "5,0", false},
		{"son 21.5", "1,5", false},
		{"son 10^4", "10", false},
	} {
		if got := graph.FigureStillIn(c.text, c.anchor); got != c.want {
			t.Errorf("FigureStillIn(%q, %q) = %v, want %v", c.text, c.anchor, got, c.want)
		}
	}
}
