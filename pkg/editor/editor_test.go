package editor

import (
	"strings"
	"testing"

	"github.com/liamg/readline/pkg/editor/completion"
)

// bufStr returns the editor's buffer as a string.
func bufStr(e *Editor) string { return e.BufferString() }

func TestEditor_InsertAdvancesCursor(t *testing.T) {
	e := New()
	e.Insert('a')
	e.Insert('b')
	e.Insert('c')
	if got := bufStr(e); got != "abc" {
		t.Fatalf("buffer = %q, want %q", got, "abc")
	}
	if e.Cursor() != 3 {
		t.Fatalf("cursor = %d, want 3", e.Cursor())
	}
}

func TestEditor_InsertAtMiddle(t *testing.T) {
	e := New()
	e.Insert('a')
	e.Insert('c')
	e.MoveCursor(-1)
	e.Insert('b')
	if got := bufStr(e); got != "abc" {
		t.Fatalf("buffer = %q, want %q", got, "abc")
	}
}

func TestEditor_DeletePrevious(t *testing.T) {
	e := New()
	e.Insert('a')
	e.Insert('b')
	e.DeletePrevious()
	if got := bufStr(e); got != "a" {
		t.Fatalf("buffer = %q, want %q", got, "a")
	}
	if e.Cursor() != 1 {
		t.Fatalf("cursor = %d, want 1", e.Cursor())
	}
}

func TestEditor_DeletePreviousAtStart(t *testing.T) {
	e := New()
	e.DeletePrevious() // should be a no-op
	if e.Cursor() != 0 {
		t.Fatalf("cursor = %d, want 0", e.Cursor())
	}
}

func TestEditor_DeleteNext(t *testing.T) {
	e := New()
	e.Insert('a')
	e.Insert('b')
	e.MoveCursor(-2)
	e.DeleteNext()
	if got := bufStr(e); got != "b" {
		t.Fatalf("buffer = %q, want %q", got, "b")
	}
	if e.Cursor() != 0 {
		t.Fatalf("cursor = %d, want 0", e.Cursor())
	}
}

func TestEditor_DeleteNextGraphemesDeletesWholeClusters(t *testing.T) {
	e := New()
	e.SetBuffer([]rune("🐈🐈‍⬛abc"))
	e.MoveCursor(-e.Cursor())

	killed := e.DeleteNextGraphemes(2)

	if got, want := string(killed), "🐈🐈‍⬛"; got != want {
		t.Fatalf("killed = %q, want %q", got, want)
	}
	if got, want := bufStr(e), "abc"; got != want {
		t.Fatalf("buffer = %q, want %q", got, want)
	}
}

func TestEditor_DeletePreviousGraphemesDeletesWholeClusters(t *testing.T) {
	e := New()
	e.SetBuffer([]rune("abc🐈🐈‍⬛"))

	killed := e.DeletePreviousGraphemes(2)

	if got, want := string(killed), "🐈🐈‍⬛"; got != want {
		t.Fatalf("killed = %q, want %q", got, want)
	}
	if got, want := bufStr(e), "abc"; got != want {
		t.Fatalf("buffer = %q, want %q", got, want)
	}
}

func TestEditor_MoveCursorSkipsGraphemeClusters(t *testing.T) {
	e := New()
	e.SetBuffer([]rune("🐈🐈‍⬛"))

	e.MoveCursor(-1)
	if got, want := e.Cursor(), len([]rune("🐈")); got != want {
		t.Fatalf("cursor after first left = %d, want %d", got, want)
	}

	e.MoveCursor(-1)
	if got := e.Cursor(); got != 0 {
		t.Fatalf("cursor after second left = %d, want 0", got)
	}

	e.MoveCursor(1)
	if got, want := e.Cursor(), len([]rune("🐈")); got != want {
		t.Fatalf("cursor after right = %d, want %d", got, want)
	}
}

func TestEditor_DeletePreviousRemovesWholeGrapheme(t *testing.T) {
	e := New()
	e.SetBuffer([]rune("🐈🐈‍⬛"))

	e.DeletePrevious()

	if got, want := bufStr(e), "🐈"; got != want {
		t.Fatalf("buffer = %q, want %q", got, want)
	}
	if got, want := e.Cursor(), len([]rune("🐈")); got != want {
		t.Fatalf("cursor = %d, want %d", got, want)
	}
}

func TestEditor_ClampCursorBeforeEndUsesGraphemeBoundary(t *testing.T) {
	e := New()
	e.SetBuffer([]rune("🐈🐈‍⬛"))
	e.SetClampCursorBeforeEnd(true)

	if got, want := e.Cursor(), len([]rune("🐈")); got != want {
		t.Fatalf("cursor = %d, want %d", got, want)
	}
}

func TestEditor_TriggerCompletionsSingleCandidateAppliesImmediately(t *testing.T) {
	e := New(WithCompleter(staticCompleter{groups: []completion.Group{{
		Candidates: []completion.Candidate{{Name: "foo.txt", Content: "foo.txt"}},
	}}}))
	e.Insert('f', 'o')

	e.TriggerCompletions()

	if got := bufStr(e); got != "foo.txt" {
		t.Fatalf("buffer = %q, want %q", got, "foo.txt")
	}
	if e.Cursor() != len("foo.txt") {
		t.Fatalf("cursor = %d, want %d", e.Cursor(), len("foo.txt"))
	}
	if e.GetCompletions() != nil {
		t.Fatal("completions should not be shown after unambiguous completion")
	}
}

func TestEditor_TriggerCompletionsCommonPrefixAppliesWithoutShowingList(t *testing.T) {
	e := New(WithCompleter(staticCompleter{groups: []completion.Group{{
		Candidates: []completion.Candidate{
			{Name: "foobar", Content: "foobar"},
			{Name: "foobaz", Content: "foobaz"},
		},
	}}}))
	e.Insert('f', 'o')

	e.TriggerCompletions()

	if got := bufStr(e); got != "fooba" {
		t.Fatalf("buffer = %q, want %q", got, "fooba")
	}
	if e.GetCompletions() != nil {
		t.Fatal("completions should not be shown when common prefix advances input")
	}
}

func TestEditor_TriggerCompletionsShowsListWhenAmbiguous(t *testing.T) {
	groups := []completion.Group{{
		Candidates: []completion.Candidate{
			{Name: "foo", Content: "foo"},
			{Name: "bar", Content: "bar"},
		},
	}}
	e := New(WithCompleter(staticCompleter{groups: groups}))

	e.TriggerCompletions()

	if got := bufStr(e); got != "" {
		t.Fatalf("buffer = %q, want empty", got)
	}
	if e.GetCompletions() == nil {
		t.Fatal("completions should be shown when no common prefix can be applied")
	}
}

func TestEditor_TriggerCompletionsShowsListWhenCommonPrefixAlreadyTyped(t *testing.T) {
	groups := []completion.Group{{
		Candidates: []completion.Candidate{
			{Name: "version", Content: "shiv.version"},
			{Name: "list_modules", Content: "shiv.list_modules()"},
		},
	}}
	e := New(WithCompleter(staticCompleter{groups: groups}))
	for _, r := range "shiv." {
		e.Insert(r)
	}

	e.TriggerCompletions()

	if got := bufStr(e); got != "shiv." {
		t.Fatalf("buffer = %q, want unchanged", got)
	}
	if e.GetCompletions() == nil {
		t.Fatal("completions should be shown when common prefix is already typed")
	}
}

func TestEditor_TriggerCompletionsDoesNotAppendUnrelatedPrefix(t *testing.T) {
	groups := []completion.Group{{
		Candidates: []completion.Candidate{
			{Name: ".config", Content: "~/.config/"},
			{Name: ".local", Content: "~/.local/"},
		},
	}}
	e := New(WithCompleter(staticCompleter{groups: groups}))
	for _, r := range "vim ~/.cla" {
		e.Insert(r)
	}

	e.TriggerCompletions()

	if got := bufStr(e); got != "vim ~/.cla" {
		t.Fatalf("buffer = %q, want unchanged", got)
	}
	if e.GetCompletions() == nil {
		t.Fatal("unrelated completions should be shown instead of modifying the buffer")
	}
}

func TestEditor_TriggerCompletionsReplacesCurrentTokenOnly(t *testing.T) {
	e := New(WithCompleter(staticCompleter{groups: []completion.Group{{
		Candidates: []completion.Candidate{{Name: "src/main.go", Content: "src/main.go"}},
	}}}))
	for _, r := range "cat src/m" {
		e.Insert(r)
	}

	e.TriggerCompletions()

	if got := bufStr(e); got != "cat src/main.go" {
		t.Fatalf("buffer = %q, want %q", got, "cat src/main.go")
	}
}

func TestEditor_TriggerCompletionsSecondTabAppliesExactContinuation(t *testing.T) {
	groups := []completion.Group{{
		Candidates: []completion.Candidate{
			{Name: "git", Content: "./.config/git/"},
			{Name: "github-copilot", Content: "./.config/github-copilot/"},
			{Name: "gitui", Content: "./.config/gitui/"},
		},
	}}
	e := New(WithCompleter(staticCompleter{groups: groups}))
	for _, r := range "./.config/git" {
		e.Insert(r)
	}

	e.TriggerCompletions()
	if got := bufStr(e); got != "./.config/git" {
		t.Fatalf("first tab buffer = %q, want unchanged", got)
	}
	if e.GetCompletions() == nil {
		t.Fatal("first tab should show ambiguous completions")
	}

	e.TriggerCompletions()
	if got := bufStr(e); got != "./.config/git/" {
		t.Fatalf("second tab buffer = %q, want %q", got, "./.config/git/")
	}
	if e.GetCompletions() != nil {
		t.Fatal("second tab should hide completions after applying continuation")
	}
}

func TestEditor_TriggerCompletionsSecondTabUsesCandidateJoin(t *testing.T) {
	groups := []completion.Group{{
		Candidates: []completion.Candidate{
			{Name: "git", Content: "git", Join: "/"},
			{Name: "gitui", Content: "gitui", Join: "/"},
		},
	}}
	e := New(WithCompleter(staticCompleter{groups: groups}))
	for _, r := range "git" {
		e.Insert(r)
	}

	e.TriggerCompletions()
	if e.GetCompletions() == nil {
		t.Fatal("first tab should show ambiguous completions")
	}

	e.TriggerCompletions()
	if got := bufStr(e); got != "git/" {
		t.Fatalf("second tab buffer = %q, want %q", got, "git/")
	}
}

func TestEditor_InsertRefreshesVisibleCompletions(t *testing.T) {
	e := New(WithCompleter(prefixCompleter{candidates: []completion.Candidate{
		{Name: "foo", Content: "foo"},
		{Name: "fizz", Content: "fizz"},
	}}))
	e.Insert('f')
	e.TriggerCompletions()

	e.Insert('o')

	groups := e.GetCompletions()
	if len(groups) != 1 || len(groups[0].Candidates) != 1 {
		t.Fatalf("completions = %#v, want one matching candidate", groups)
	}
	if got := groups[0].Candidates[0].Name; got != "foo" {
		t.Fatalf("candidate = %q, want %q", got, "foo")
	}
}

func TestEditor_InsertDismissesCompletionsWhenNoMatchesRemain(t *testing.T) {
	e := New(WithCompleter(prefixCompleter{candidates: []completion.Candidate{
		{Name: "foo", Content: "foo"},
		{Name: "fizz", Content: "fizz"},
	}}))
	e.Insert('f')
	e.TriggerCompletions()

	e.Insert('x')

	if e.GetCompletions() != nil {
		t.Fatal("completions should be dismissed when no candidates match")
	}
}

func TestEditor_DeletePreviousToEmptyDismissesCompletions(t *testing.T) {
	e := New(WithCompleter(staticCompleter{groups: []completion.Group{{
		Candidates: []completion.Candidate{
			{Name: "xa", Content: "xa"},
			{Name: "xb", Content: "xb"},
		},
	}}}))
	e.Insert('x')
	e.TriggerCompletions()

	e.DeletePrevious()

	if got := e.BufferString(); got != "" {
		t.Fatalf("buffer = %q, want empty", got)
	}
	if e.GetCompletions() != nil {
		t.Fatal("completions should be dismissed after deleting input")
	}
}

func TestEditor_MoveCursorClampsAtZero(t *testing.T) {
	e := New()
	e.Insert('a')
	e.MoveCursor(-100)
	if e.Cursor() != 0 {
		t.Fatalf("cursor = %d, want 0", e.Cursor())
	}
}

func TestEditor_MoveCursorClampsAtEnd(t *testing.T) {
	e := New()
	e.Insert('a')
	e.Insert('b')
	e.MoveCursor(100)
	if e.Cursor() != 2 {
		t.Fatalf("cursor = %d, want 2", e.Cursor())
	}
}

func TestEditor_ReplaceRune(t *testing.T) {
	e := New()
	e.SetBuffer([]rune("🐈🐈‍⬛c"))
	e.MoveCursor(-e.Cursor())
	e.ReplaceRune('X')
	if got := bufStr(e); got != "X🐈‍⬛c" {
		t.Fatalf("buffer = %q, want %q", got, "X🐈‍⬛c")
	}
}

func TestEditor_Runes(t *testing.T) {
	e := New()
	e.Insert('h')
	e.Insert('i')
	r := e.Runes()
	if len(r) != 2 || r[0] != 'h' || r[1] != 'i' {
		t.Fatalf("Runes = %v, want [h i]", r)
	}
}

func TestEditor_Reset(t *testing.T) {
	e := New()
	e.Insert('a')
	e.Insert('b')
	e.SetSelectionAnchor(SelectionChar)
	e.SetClampCursorBeforeEnd(true)
	e.Reset()
	if bufStr(e) != "" {
		t.Fatalf("buffer not empty after Reset")
	}
	if e.Cursor() != 0 {
		t.Fatalf("cursor = %d after Reset, want 0", e.Cursor())
	}
	if e.Selection() != nil {
		t.Fatalf("selection not nil after Reset")
	}

	// A previous vi normal-mode input must not leave the next insert-mode
	// input unable to move its cursor past the final character.
	e.Insert('a', 'b')
	e.MoveCursor(-1)
	e.MoveCursor(1)
	if got, want := e.Cursor(), len(e.Runes()); got != want {
		t.Fatalf("cursor = %d after Reset and moving to end, want %d", got, want)
	}
}

func TestEditor_SelectionNilByDefault(t *testing.T) {
	e := New()
	if e.Selection() != nil {
		t.Fatal("expected nil selection on new editor")
	}
}

func TestEditor_SetSelectionAnchor(t *testing.T) {
	e := New()
	e.Insert('a')
	e.Insert('b')
	e.Insert('c')
	e.MoveCursor(-1) // cursor at 2
	e.SetSelectionAnchor(SelectionChar)
	sel := e.Selection()
	if sel == nil {
		t.Fatal("expected non-nil selection")
	}
	if sel.Anchor != 2 {
		t.Fatalf("Anchor = %d, want 2", sel.Anchor)
	}
	if sel.Type != SelectionChar {
		t.Fatalf("Type = %d, want SelectionChar", sel.Type)
	}
}

type staticCompleter struct {
	groups []completion.Group
}

func (s staticCompleter) Complete(_ []rune, _ int) []completion.Group {
	return s.groups
}

type prefixCompleter struct {
	candidates []completion.Candidate
}

func (p prefixCompleter) Complete(buffer []rune, _ int) []completion.Group {
	var candidates []completion.Candidate
	for _, candidate := range p.candidates {
		if strings.HasPrefix(candidate.Content, string(buffer)) {
			candidates = append(candidates, candidate)
		}
	}
	if len(candidates) == 0 {
		return nil
	}
	return []completion.Group{{Candidates: candidates}}
}

func TestEditor_SetSelectionAnchorTypes(t *testing.T) {
	for _, typ := range []SelectionType{SelectionChar, SelectionLine, SelectionBlock} {
		e := New()
		e.SetSelectionAnchor(typ)
		if e.Selection().Type != typ {
			t.Errorf("Type = %d, want %d", e.Selection().Type, typ)
		}
	}
}

func TestEditor_ClearSelection(t *testing.T) {
	e := New()
	e.SetSelectionAnchor(SelectionChar)
	e.ClearSelection()
	if e.Selection() != nil {
		t.Fatal("expected nil after ClearSelection")
	}
}

func TestSelection_RangeCursorAfterAnchor(t *testing.T) {
	s := &Selection{Anchor: 2}
	start, end := s.Range(5, []rune("0123456789"))
	if start != 2 || end != 6 {
		t.Fatalf("Range(5) = (%d,%d), want (2,6)", start, end)
	}
}

func TestSelection_RangeCursorAtAnchor(t *testing.T) {
	s := &Selection{Anchor: 3}
	start, end := s.Range(3, []rune("0123456789"))
	if start != 3 || end != 4 {
		t.Fatalf("Range(3) = (%d,%d), want (3,4)", start, end)
	}
}

func TestSelection_RangeCursorBeforeAnchor(t *testing.T) {
	s := &Selection{Anchor: 5}
	start, end := s.Range(2, []rune("0123456789"))
	if start != 2 || end != 6 {
		t.Fatalf("Range(2) = (%d,%d), want (2,6)", start, end)
	}
}

func TestSelection_RangeCharClampsEndToBufferLength(t *testing.T) {
	// Cursor at the final index must not produce an end past len(buffer).
	s := &Selection{Anchor: 0}
	start, end := s.Range(4, []rune("hello"))
	if start != 0 || end != 5 {
		t.Fatalf("Range(4) = (%d,%d), want (0,5)", start, end)
	}
}

func TestSelection_RangeLineWise(t *testing.T) {
	buffer := []rune("one\ntwo\nthree")
	tests := []struct {
		name         string
		anchor, curs int
		start, end   int
	}{
		// anchor and cursor both inside the middle line -> whole middle line incl \n
		{"single middle line", 5, 6, 4, 8},
		// span from first line into middle line -> first two whole lines
		{"two lines forward", 1, 5, 0, 8},
		// reversed span still covers the same lines
		{"two lines reversed", 5, 1, 0, 8},
		// last line has no trailing newline -> end is len(buffer)
		{"last line to EOF", 9, 12, 8, 13},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &Selection{Anchor: tt.anchor, Type: SelectionLine}
			start, end := s.Range(tt.curs, buffer)
			if start != tt.start || end != tt.end {
				t.Fatalf("Range = (%d,%d), want (%d,%d)", start, end, tt.start, tt.end)
			}
		})
	}
}

func TestSelection_RangeLineWiseEmptyBuffer(t *testing.T) {
	s := &Selection{Anchor: 0, Type: SelectionLine}
	start, end := s.Range(0, nil)
	if start != 0 || end != 0 {
		t.Fatalf("Range on empty buffer = (%d,%d), want (0,0)", start, end)
	}
}

func TestSelection_GetSelectedRunesLineWise(t *testing.T) {
	e := New()
	e.SetBuffer([]rune("one\ntwo\nthree"))
	moveCursorTo(e, 5) // inside "two"
	e.SetSelectionAnchor(SelectionLine)
	if got := string(e.GetSelectedRunes()); got != "two\n" {
		t.Fatalf("GetSelectedRunes = %q, want %q", got, "two\n")
	}
}

// moveCursorTo is a small helper mirroring the vi test helper.
func moveCursorTo(e *Editor, pos int) {
	e.MoveCursor(pos - e.Cursor())
}
