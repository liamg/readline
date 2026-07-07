package completion

import "testing"

// staticCompleter confirms the Completer interface is satisfiable.
type staticCompleter struct{ groups []Group }

func (s staticCompleter) Complete(buffer []rune, cursor int) []Group { return s.groups }

var _ Completer = staticCompleter{}

func TestCompleterReturnsGroupedCandidates(t *testing.T) {
	c := Candidate{Name: "n", Description: "d", Content: "c", Join: "/"}
	if c.Name != "n" || c.Description != "d" || c.Content != "c" || c.Join != "/" {
		t.Fatal("candidate fields not preserved")
	}

	var comp Completer = staticCompleter{groups: []Group{{Name: "g", Candidates: []Candidate{c}}}}
	groups := comp.Complete([]rune("x"), 1)
	if len(groups) != 1 || groups[0].Name != "g" || len(groups[0].Candidates) != 1 {
		t.Fatalf("Complete returned %+v, want one group with one candidate", groups)
	}
}
