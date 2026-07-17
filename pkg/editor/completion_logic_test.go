package editor

import (
	"testing"

	"github.com/liamg/readline/pkg/editor/completion"
)

type fakeCompleter struct {
	groups []completion.Group
}

func (f fakeCompleter) Complete(buffer []rune, cursor int) []completion.Group {
	return f.groups
}

func oneGroup(candidates ...completion.Candidate) []completion.Group {
	return []completion.Group{{Candidates: candidates}}
}

type fakeSuggester struct {
	suffix []rune
}

func (f fakeSuggester) Suggest(buffer []rune) []rune { return f.suffix }

func TestEditor_CompletionSingleCandidateExtends(t *testing.T) {
	e := New(WithCompleter(fakeCompleter{groups: oneGroup(completion.Candidate{Name: "foobar"})}))
	e.SetBuffer([]rune("fo"))
	e.TriggerCompletions()
	if got := e.BufferString(); got != "foobar" {
		t.Fatalf("buffer = %q, want %q", got, "foobar")
	}
}

func TestEditor_CompletionCommonPrefixExtends(t *testing.T) {
	e := New(WithCompleter(fakeCompleter{groups: oneGroup(
		completion.Candidate{Name: "foobar"},
		completion.Candidate{Name: "foobaz"},
	)}))
	e.SetBuffer([]rune("fo"))
	e.TriggerCompletions()
	// Ambiguous: extend only to the common prefix.
	if got := e.BufferString(); got != "fooba" {
		t.Fatalf("buffer = %q, want %q", got, "fooba")
	}
}

func TestEditor_CompletionRespectsWordBoundary(t *testing.T) {
	e := New(WithCompleter(fakeCompleter{groups: oneGroup(completion.Candidate{Name: "foobar"})}))
	e.SetBuffer([]rune("ls fo"))
	e.TriggerCompletions()
	if got := e.BufferString(); got != "ls foobar" {
		t.Fatalf("buffer = %q, want %q", got, "ls foobar")
	}
}

func TestEditor_CompletionUsesContentOverName(t *testing.T) {
	e := New(WithCompleter(fakeCompleter{groups: oneGroup(
		completion.Candidate{Name: "display", Content: "actual-value"},
	)}))
	e.SetBuffer([]rune("act"))
	e.TriggerCompletions()
	if got := e.BufferString(); got != "actual-value" {
		t.Fatalf("buffer = %q, want %q", got, "actual-value")
	}
}

func TestEditor_CompletionJoinAppendsOnContinuation(t *testing.T) {
	groups := oneGroup(completion.Candidate{Name: "src", Join: "/"})
	e := New(WithCompleter(fakeCompleter{groups: groups}))
	e.SetBuffer([]rune("src"))
	// Simulate the completion menu already being visible for the exact match,
	// so the next trigger is a continuation that appends the Join suffix.
	e.completions = groups
	e.TriggerCompletions()
	if got := e.BufferString(); got != "src/" {
		t.Fatalf("buffer = %q, want %q", got, "src/")
	}
}

func TestEditor_CompletionNoCompleterIsNoop(t *testing.T) {
	e := New()
	e.SetBuffer([]rune("fo"))
	e.TriggerCompletions()
	if got := e.BufferString(); got != "fo" {
		t.Fatalf("buffer = %q, want unchanged %q", got, "fo")
	}
	if e.GetCompletions() != nil {
		t.Fatal("expected no completions with no completer")
	}
}

func TestIsCompletionContinuation(t *testing.T) {
	cases := []struct {
		suffix string
		want   bool
	}{
		{"/", true},
		{".", true},
		{"()", true},
		{"  ", true}, // up to two boundary chars
		{"", false},
		{"/x", false},  // contains a non-boundary rune
		{"abc", false}, // too long and non-boundary
		{"///", false}, // longer than two
	}
	for _, tt := range cases {
		if got := isCompletionContinuation([]rune(tt.suffix)); got != tt.want {
			t.Errorf("isCompletionContinuation(%q) = %v, want %v", tt.suffix, got, tt.want)
		}
	}
}

func TestCommonCompletionPrefix(t *testing.T) {
	tests := []struct {
		names []string
		want  string
	}{
		{[]string{"foobar", "foobaz"}, "fooba"},
		{[]string{"abc", "xyz"}, ""},
		{[]string{"same", "same"}, "same"},
		{[]string{"only"}, "only"},
	}
	for _, tt := range tests {
		candidates := make([]completion.Candidate, len(tt.names))
		for i, n := range tt.names {
			candidates[i] = completion.Candidate{Name: n}
		}
		if got := commonCompletionPrefix(candidates); got != tt.want {
			t.Errorf("commonCompletionPrefix(%v) = %q, want %q", tt.names, got, tt.want)
		}
	}
}

func TestEditor_CompletionReplacementStart(t *testing.T) {
	e := New()
	e.SetBuffer([]rune("ls fo"))
	// The typed fragment "fo" begins after the space boundary at index 3.
	start, typedLen := e.completionReplacementStart([]rune("foobar"))
	if start != 3 || typedLen != 2 {
		t.Fatalf("completionReplacementStart = (%d,%d), want (3,2)", start, typedLen)
	}
}

func TestEditor_TriggerAutoSuggestion(t *testing.T) {
	// No suggester: must not panic and leaves no suggestion.
	e := New()
	e.SetBuffer([]rune("x"))
	e.TriggerAutoSuggestion()
	if e.GetAutoSuggestion() != nil {
		t.Fatal("expected nil autosuggestion with no suggester")
	}

	// With a suggester: stores the returned suffix.
	e2 := New(WithSuggester(fakeSuggester{suffix: []rune("bar")}))
	e2.SetBuffer([]rune("foo"))
	e2.TriggerAutoSuggestion()
	if got := string(e2.GetAutoSuggestion()); got != "bar" {
		t.Fatalf("autosuggestion = %q, want %q", got, "bar")
	}

	// Moving away from the end hides the stored suggestion.
	e2.MoveCursor(-1)
	if e2.GetAutoSuggestion() != nil {
		t.Fatal("autosuggestion should not be offered before the end of the buffer")
	}

	// Triggering away from the end clears stale suggestions.
	e2.TriggerAutoSuggestion()
	if e2.GetAutoSuggestion() != nil {
		t.Fatal("autosuggestion should clear when triggered before the end of the buffer")
	}

	// Empty buffer clears any existing suggestion without calling the suggester.
	e2.SetBuffer([]rune(""))
	e2.TriggerAutoSuggestion()
	if e2.GetAutoSuggestion() != nil {
		t.Fatal("empty buffer should clear autosuggestion")
	}
}
