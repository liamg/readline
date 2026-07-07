package ansi

// Span associates a Style with a half-open range of rune indices [Start, End)
// within a plain-text rune slice. Spans must not overlap.
type Span struct {
	Start, End int
	Style      Style
}
