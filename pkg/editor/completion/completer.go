package completion

type Completer interface {
	Complete(buffer []rune, cursor int) []Group // grouped candidates for tabbed autocomplete
}
