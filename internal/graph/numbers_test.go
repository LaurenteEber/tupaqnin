package graph_test

import (
	"reflect"
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
