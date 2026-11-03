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
		"3,14e4": "3.14×10^4", "1 500": "1500", "1,5": "1.5", "104": "104", "10": "10",
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
