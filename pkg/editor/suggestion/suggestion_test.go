package suggestion

import "testing"

// staticSuggester confirms the Suggester interface is satisfiable.
type staticSuggester struct{ suffix []rune }

func (s staticSuggester) Suggest(buffer []rune) []rune { return s.suffix }

var _ Suggester = staticSuggester{}

func TestSuggesterReturnsSuffix(t *testing.T) {
	var s Suggester = staticSuggester{suffix: []rune("bar")}
	if got := string(s.Suggest([]rune("foo"))); got != "bar" {
		t.Fatalf("Suggest = %q, want %q", got, "bar")
	}
}
