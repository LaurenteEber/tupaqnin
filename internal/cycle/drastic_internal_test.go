package cycle

import "testing"

func TestB99_RemovesHalf(t *testing.T) {
	if !removesHalfOf("0123456789", "0123") {
		t.Fatal("60 % removed: want true")
	}
	if removesHalfOf("0123456789", "012345") {
		t.Fatal("40 % removed: want false")
	}
}

func TestB99_EndsASentence(t *testing.T) {
	text := "La cifra es 3.14 en total. Sigue otra.\n\n# Titulo\n\nFin"
	for _, c := range []struct {
		at   string
		want bool
	}{{"La cifra es 3.", false}, {"La cifra", false}, {"La cifra es 3.14 en total.", true},
		{"La cifra es 3.14 en total. Sigue otra.", true}, {"# Titulo", true}, {"Fin", true}} {
		end := indexEnd(text, c.at)
		if got := endsSentenceAt(text, end); got != c.want {
			t.Errorf("after %q: %v, want %v", c.at, got, c.want)
		}
	}
}

func indexEnd(text, part string) int {
	for i := 0; i+len(part) <= len(text); i++ {
		if text[i:i+len(part)] == part {
			return i + len(part)
		}
	}
	return -1
}

// (c): a paragraph left without its close; a heading or a list needs none.
func TestB99_BreaksASentence(t *testing.T) {
	base := "Primera oración. Segunda oración."
	if !breaksASentence(base, "Primera oración. Segunda") {
		t.Fatal("an unfinished sentence: want true")
	}
	if breaksASentence(base, "Primera oración.") {
		t.Fatal("a closed paragraph: want false")
	}
	if breaksASentence(base, base+"\n\n# Otro\n\n- un punto") {
		t.Fatal("a heading and a list: want false")
	}
}

// The case-5 batch (C.6 diagnosis): from the middle of a sentence to its period.
func TestB99_RewritesASentenceEnd(t *testing.T) {
	base := "El promedio fue de 640, es decir, 60 veces más. Esta comparación mezcla tamaños."
	tail := "es decir, 60 veces más."
	start := indexEnd(base, tail) - len(tail)
	if !rewritesASentenceEnd(base, []edit{{operation: "replace", anchor: tail, start: start, end: start + len(tail), text: "cálculo previo no comparable."}}) {
		t.Fatal("the end of a sentence: want true")
	}
	whole := "Esta comparación mezcla tamaños."
	at := indexEnd(base, whole) - len(whole)
	if rewritesASentenceEnd(base, []edit{{operation: "replace", anchor: whole, start: at, end: at + len(whole), text: "Otra oración."}}) {
		t.Fatal("a whole sentence: want false")
	}
	mid := "fue de 640"
	m := indexEnd(base, mid) - len(mid)
	if rewritesASentenceEnd(base, []edit{{operation: "replace", anchor: mid, start: m, end: m + len(mid), text: "fue de 700"}}) {
		t.Fatal("a few words inside a sentence: want false")
	}
}

// (b): the paragraph that holds what supports a decision is gone, not just a word.
func TestB99_ParagraphGone(t *testing.T) {
	base := "Primer párrafo con 60 veces.\n\nSegundo párrafo."
	first := "Primer párrafo con 60 veces."
	all := []edit{{start: 0, end: len(first)}}
	if !paragraphGone(base, "\n\nSegundo párrafo.", "60 veces", all) {
		t.Fatal("the whole paragraph went: want true")
	}
	word := indexEnd(base, "60 veces") - len("60 veces")
	if paragraphGone(base, "Primer párrafo con 70 veces.\n\nSegundo párrafo.", "60 veces", []edit{{start: word, end: word + len("60 veces")}}) {
		t.Fatal("only the figure changed: want false")
	}
	if paragraphGone(base, "Primer párrafo con 70 veces.\n\nSegundo párrafo.", "60 veces", []edit{{start: 0, end: len(first), text: "Primer párrafo con 70 veces."}}) {
		t.Fatal("the paragraph was rewritten, not deleted: want false")
	}
	if paragraphGone(base, base, "60 veces", nil) {
		t.Fatal("the anchor stays: want false")
	}
}
