package graph

// An external source that entered the project from a round (B-61): a
// source of kind web, with its url, its text and, in its body, doi,
// title, authors, year and access.
const (
	SourceKindWeb = "web"

	// Access says how much of an external source's text the app holds
	// (B-61): its full text, only its abstract, or none («no
	// verificado»: it can be named, but a citation of it stays
	// unverified, 2.11).
	AccessFullText = "full_text"
	AccessAbstract = "abstract"
	AccessNone     = "none"
)
