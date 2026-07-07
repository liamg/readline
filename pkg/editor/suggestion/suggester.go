package suggestion

type Suggester interface {
	Suggest(buffer []rune) []rune // dim text after buffer, use right arrow to complete
}
