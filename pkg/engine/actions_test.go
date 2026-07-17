package engine

import (
	"slices"
	"testing"

	"github.com/liamg/readline/pkg/editor"
	"github.com/liamg/readline/pkg/editor/completion"
	"github.com/liamg/readline/pkg/history"
	"github.com/liamg/readline/pkg/terminal"
)

// ---- test helpers ----------------------------------------------------------

// newCtx builds a fresh ActionContext with a new editor containing the given
// buffer text, with the cursor placed at the given position.
func newCtx(buf string, cursor int) *ActionContext {
	ed := editor.New()
	ed.SetBuffer([]rune(buf))
	// SetBuffer puts cursor at end; adjust to desired position.
	ed.MoveCursor(cursor - ed.Cursor())
	return &ActionContext{Editor: ed}
}

// newCtxEnd builds a fresh ActionContext with the cursor at the end of buf.
func newCtxEnd(buf string) *ActionContext {
	return newCtx(buf, len([]rune(buf)))
}

// run calls the action's Func on the context and fatals on error.
func run(t *testing.T, a *Action, ctx *ActionContext) ActionResult {
	t.Helper()
	res, err := a.Func(ctx)
	if err != nil {
		t.Fatalf("%s: unexpected error: %v", a.Name, err)
	}
	return res
}

type staticCompleter struct {
	groups []completion.Group
}

func (s staticCompleter) Complete(_ []rune, _ int) []completion.Group {
	return s.groups
}

type staticSuggester struct {
	suggestion []rune
}

func (s staticSuggester) Suggest(_ []rune) []rune {
	return s.suggestion
}

// ---- KillRing --------------------------------------------------------------

func TestKillRing_PushPeek(t *testing.T) {
	var k KillRing
	if k.Peek() != nil {
		t.Fatal("Peek on empty ring should be nil")
	}
	k.Push([]rune("hello"))
	if got := string(k.Peek()); got != "hello" {
		t.Fatalf("Peek = %q, want %q", got, "hello")
	}
}

func TestKillRing_Push_EmptyNoOp(t *testing.T) {
	var k KillRing
	k.Push(nil)
	k.Push([]rune{})
	if k.Peek() != nil {
		t.Fatal("empty pushes should not add entries")
	}
}

func TestKillRing_Push_StacksEntries(t *testing.T) {
	var k KillRing
	k.Push([]rune("first"))
	k.Push([]rune("second"))
	if got := string(k.Peek()); got != "second" {
		t.Fatalf("Peek = %q, want last pushed entry %q", got, "second")
	}
}

// ---- AcceptLine ------------------------------------------------------------

func TestAcceptLine_ReturnsComplete(t *testing.T) {
	ctx := newCtxEnd("")
	res := run(t, AcceptLine, ctx)
	if !res.Complete {
		t.Fatal("AcceptLine: Complete should be true")
	}
}

func TestComplete_TriggersEditorCompletions(t *testing.T) {
	want := []completion.Group{{
		Name: "files",
		Candidates: []completion.Candidate{
			{Name: "foo.txt", Content: "foo.txt"},
			{Name: "bar.txt", Content: "bar.txt"},
		},
	}}
	ed := editor.New(editor.WithCompleter(staticCompleter{groups: want}))
	ctx := &ActionContext{Editor: ed}

	run(t, Complete, ctx)

	got := ed.GetCompletions()
	if !slices.EqualFunc(got, want, func(a, b completion.Group) bool {
		return a.Name == b.Name && slices.Equal(a.Candidates, b.Candidates)
	}) {
		t.Fatalf("completions = %#v, want %#v", got, want)
	}
}

// ---- Back ------------------------------------------------------------------

func TestBack_MovesCursorLeft(t *testing.T) {
	ctx := newCtxEnd("abc")
	run(t, Back, ctx)
	if ctx.Editor.Cursor() != 2 {
		t.Fatalf("Back: cursor = %d, want 2", ctx.Editor.Cursor())
	}
}

func TestBack_AtStartIsNoOp(t *testing.T) {
	ctx := newCtx("abc", 0)
	run(t, Back, ctx)
	if ctx.Editor.Cursor() != 0 {
		t.Fatalf("Back at start: cursor = %d, want 0", ctx.Editor.Cursor())
	}
}

// ---- Forward ---------------------------------------------------------------

func TestForward_MovesCursorRight(t *testing.T) {
	ctx := newCtx("abc", 0)
	run(t, Forward, ctx)
	if ctx.Editor.Cursor() != 1 {
		t.Fatalf("Forward: cursor = %d, want 1", ctx.Editor.Cursor())
	}
}

func TestForward_AtEndIsNoOp(t *testing.T) {
	ctx := newCtxEnd("abc")
	run(t, Forward, ctx)
	if ctx.Editor.Cursor() != 3 {
		t.Fatalf("Forward at end: cursor = %d, want 3", ctx.Editor.Cursor())
	}
}

// ---- AcceptAutosuggestion --------------------------------------------------

func TestAcceptAutosuggestion_InsertsAtEnd(t *testing.T) {
	ed := editor.New(editor.WithSuggester(staticSuggester{suggestion: []rune("bar")}))
	ed.SetBuffer([]rune("foo"))
	ed.TriggerAutoSuggestion()
	ctx := &ActionContext{Editor: ed}

	run(t, AcceptAutosuggestion, ctx)

	if got := ctx.Editor.BufferString(); got != "foobar" {
		t.Fatalf("AcceptAutosuggestion: buffer = %q, want %q", got, "foobar")
	}
}

func TestAcceptAutosuggestion_IgnoresSuggestionBeforeEnd(t *testing.T) {
	ed := editor.New(editor.WithSuggester(staticSuggester{suggestion: []rune("bar")}))
	ed.SetBuffer([]rune("foo"))
	ed.TriggerAutoSuggestion()
	ed.MoveCursor(-1)
	ctx := &ActionContext{Editor: ed}

	run(t, AcceptAutosuggestion, ctx)

	if got := ctx.Editor.BufferString(); got != "foo" {
		t.Fatalf("AcceptAutosuggestion before end: buffer = %q, want unchanged %q", got, "foo")
	}
	if got := ctx.Editor.Cursor(); got != 2 {
		t.Fatalf("AcceptAutosuggestion before end: cursor = %d, want 2", got)
	}
}

func TestAcceptAutosuggestionOrForward_AcceptsAtEnd(t *testing.T) {
	ed := editor.New(editor.WithSuggester(staticSuggester{suggestion: []rune("bar")}))
	ed.SetBuffer([]rune("foo"))
	ed.TriggerAutoSuggestion()
	ctx := &ActionContext{Editor: ed}

	run(t, AcceptAutosuggestionOrForward, ctx)

	if got := ctx.Editor.BufferString(); got != "foobar" {
		t.Fatalf("AcceptAutosuggestionOrForward: buffer = %q, want %q", got, "foobar")
	}
	if got := ctx.Editor.Cursor(); got != 6 {
		t.Fatalf("AcceptAutosuggestionOrForward: cursor = %d, want 6", got)
	}
}

func TestAcceptAutosuggestionOrForward_MovesForwardBeforeEnd(t *testing.T) {
	ed := editor.New(editor.WithSuggester(staticSuggester{suggestion: []rune("bar")}))
	ed.SetBuffer([]rune("foo"))
	ed.TriggerAutoSuggestion()
	ed.MoveCursor(-2)
	ctx := &ActionContext{Editor: ed}

	run(t, AcceptAutosuggestionOrForward, ctx)

	if got := ctx.Editor.BufferString(); got != "foo" {
		t.Fatalf("AcceptAutosuggestionOrForward before end: buffer = %q, want unchanged %q", got, "foo")
	}
	if got := ctx.Editor.Cursor(); got != 2 {
		t.Fatalf("AcceptAutosuggestionOrForward before end: cursor = %d, want 2", got)
	}
}

// ---- BeginningOfLine -------------------------------------------------------

func TestBeginningOfLine_MovesCursorToZero(t *testing.T) {
	ctx := newCtxEnd("hello")
	run(t, BeginningOfLine, ctx)
	if ctx.Editor.Cursor() != 0 {
		t.Fatalf("BeginningOfLine: cursor = %d, want 0", ctx.Editor.Cursor())
	}
}

func TestBeginningOfLine_AlreadyAtStartIsNoOp(t *testing.T) {
	ctx := newCtx("hello", 0)
	run(t, BeginningOfLine, ctx)
	if ctx.Editor.Cursor() != 0 {
		t.Fatalf("BeginningOfLine at start: cursor = %d, want 0", ctx.Editor.Cursor())
	}
}

// ---- EndOfLine -------------------------------------------------------------

func TestEndOfLine_MovesCursorToEnd(t *testing.T) {
	ctx := newCtx("hello", 2)
	run(t, EndOfLine, ctx)
	if ctx.Editor.Cursor() != 5 {
		t.Fatalf("EndOfLine: cursor = %d, want 5", ctx.Editor.Cursor())
	}
}

func TestEndOfLine_AlreadyAtEndIsNoOp(t *testing.T) {
	ctx := newCtxEnd("hello")
	run(t, EndOfLine, ctx)
	if ctx.Editor.Cursor() != 5 {
		t.Fatalf("EndOfLine already at end: cursor = %d, want 5", ctx.Editor.Cursor())
	}
}

// ---- DeletePrevious --------------------------------------------------------

func TestDeletePrevious_RemovesCharBeforeCursor(t *testing.T) {
	ctx := newCtxEnd("abc")
	run(t, DeletePrevious, ctx)
	if ctx.Editor.BufferString() != "ab" {
		t.Fatalf("DeletePrevious: buffer = %q, want %q", ctx.Editor.BufferString(), "ab")
	}
	if ctx.Editor.Cursor() != 2 {
		t.Fatalf("DeletePrevious: cursor = %d, want 2", ctx.Editor.Cursor())
	}
}

func TestDeletePrevious_AtStartIsNoOp(t *testing.T) {
	ctx := newCtx("abc", 0)
	run(t, DeletePrevious, ctx)
	if ctx.Editor.BufferString() != "abc" {
		t.Fatalf("DeletePrevious at start: buffer modified unexpectedly")
	}
}

// ---- DeleteNext ------------------------------------------------------------

func TestDeleteNext_RemovesCharAtCursor(t *testing.T) {
	ctx := newCtx("abc", 0)
	run(t, DeleteNext, ctx)
	if ctx.Editor.BufferString() != "bc" {
		t.Fatalf("DeleteNext: buffer = %q, want %q", ctx.Editor.BufferString(), "bc")
	}
	if ctx.Editor.Cursor() != 0 {
		t.Fatalf("DeleteNext: cursor = %d, want 0", ctx.Editor.Cursor())
	}
}

func TestDeleteNext_AtEndIsNoOp(t *testing.T) {
	ctx := newCtxEnd("abc")
	run(t, DeleteNext, ctx)
	if ctx.Editor.BufferString() != "abc" {
		t.Fatalf("DeleteNext at end: buffer modified unexpectedly")
	}
}

// ---- ReplaceRune -----------------------------------------------------------

func TestReplaceRune_WaitsForNextKey(t *testing.T) {
	ctx := newCtx("abc", 1) // cursor at 1, pointing at 'b'
	res := run(t, ReplaceRune, ctx)
	if res.Next == nil {
		t.Fatal("ReplaceRune: expected Next continuation")
	}
	// Feed the replacement character.
	ctx.Keys = append(ctx.Keys, terminal.KeyEvent{Key: terminal.KeyRune, Rune: 'X'})
	_, err := res.Next(ctx)
	if err != nil {
		t.Fatalf("ReplaceRune continuation error: %v", err)
	}
	if ctx.Editor.BufferString() != "aXc" {
		t.Fatalf("ReplaceRune: buffer = %q, want %q", ctx.Editor.BufferString(), "aXc")
	}
}

func TestReplaceRune_IgnoresNonRuneKey(t *testing.T) {
	ctx := newCtx("abc", 1)
	res := run(t, ReplaceRune, ctx)
	// Feed a named key (e.g. Enter) — should not modify buffer.
	ctx.Keys = append(ctx.Keys, terminal.KeyEvent{Key: terminal.KeyEnter})
	res.Next(ctx)
	if ctx.Editor.BufferString() != "abc" {
		t.Fatalf("ReplaceRune with non-rune key: buffer = %q, want unchanged", ctx.Editor.BufferString())
	}
}

// ---- HistoryPrevious / HistoryNext -----------------------------------------

func TestHistoryPrevious_LoadsPreviousEntry(t *testing.T) {
	h := newMockHistory([]string{"cmd1", "cmd2"})
	ctx := &ActionContext{Editor: editor.New(), History: h}
	run(t, HistoryPrevious, ctx)
	if ctx.Editor.BufferString() != "cmd2" {
		t.Fatalf("HistoryPrevious: buffer = %q, want %q", ctx.Editor.BufferString(), "cmd2")
	}
}

func TestHistoryPrevious_BacksUpCurrentLine(t *testing.T) {
	h := newMockHistory([]string{"old"})
	ed := editor.New()
	ed.SetBuffer([]rune("current"))
	ctx := &ActionContext{Editor: ed, History: h}
	run(t, HistoryPrevious, ctx)
	run(t, HistoryNext, ctx)
	if ctx.Editor.BufferString() != "current" {
		t.Fatalf("after HistoryPrevious+Next: buffer = %q, want %q", ctx.Editor.BufferString(), "current")
	}
}

func TestHistoryNext_AtCurrentIsNoOp(t *testing.T) {
	h := newMockHistory([]string{"cmd1"})
	ctx := &ActionContext{Editor: editor.New(), History: h}
	// IsCurrent() is true initially — HistoryNext should be a no-op.
	run(t, HistoryNext, ctx)
	if ctx.Editor.BufferString() != "" {
		t.Fatalf("HistoryNext at current: buffer = %q, want empty", ctx.Editor.BufferString())
	}
}

func TestHistoryNext_LoadsNextEntry(t *testing.T) {
	h := newMockHistory([]string{"cmd1", "cmd2"})
	ctx := &ActionContext{Editor: editor.New(), History: h}
	run(t, HistoryPrevious, ctx) // loads "cmd2"
	run(t, HistoryPrevious, ctx) // loads "cmd1"
	run(t, HistoryNext, ctx)     // back to "cmd2"
	if ctx.Editor.BufferString() != "cmd2" {
		t.Fatalf("HistoryNext: buffer = %q, want %q", ctx.Editor.BufferString(), "cmd2")
	}
}

// ---- DeleteRange helper ----------------------------------------------------

func TestDeleteRange_MiddleOfBuffer(t *testing.T) {
	ctx := newCtxEnd("abcde")
	killed := DeleteRange(ctx, 1, 3)
	if ctx.Editor.BufferString() != "ade" {
		t.Fatalf("DeleteRange: buffer = %q, want %q", ctx.Editor.BufferString(), "ade")
	}
	if ctx.Editor.Cursor() != 1 {
		t.Fatalf("DeleteRange: cursor = %d, want 1", ctx.Editor.Cursor())
	}
	if !slices.Equal(killed, []rune("bc")) {
		t.Fatalf("DeleteRange: killed = %q, want %q", string(killed), "bc")
	}
}

func TestDeleteRange_EmptyRangeNoOp(t *testing.T) {
	ctx := newCtxEnd("abc")
	killed := DeleteRange(ctx, 2, 2)
	if ctx.Editor.BufferString() != "abc" {
		t.Fatalf("deleteRange empty: buffer changed unexpectedly")
	}
	if killed != nil {
		t.Fatalf("deleteRange empty: expected nil killed, got %q", string(killed))
	}
}

// ---- mock history ----------------------------------------------------------

// mockHistory is a simple in-memory history used in action tests.
type mockHistory struct {
	entries []history.Entry
	idx     int // 0 = "current" (after most recent)
}

func newMockHistory(lines []string) *mockHistory {
	entries := make([]history.Entry, len(lines))
	for i, l := range lines {
		entries[i] = history.Entry{Text: l}
	}
	return &mockHistory{entries: entries}
}

func (m *mockHistory) IsCurrent() bool          { return m.idx == 0 }
func (m *mockHistory) Reset()                   { m.idx = 0 }
func (m *mockHistory) SetFilter(_ string)       {}
func (m *mockHistory) SetPrefixFilter(_ string) {}
func (m *mockHistory) Append(_ string, _ bool)  {}

func (m *mockHistory) Previous() history.Entry {
	if m.idx >= len(m.entries) {
		return history.Entry{}
	}
	m.idx++
	return m.entries[len(m.entries)-m.idx]
}

func (m *mockHistory) Next() history.Entry {
	if m.idx <= 0 {
		return history.Entry{}
	}
	m.idx--
	if m.idx == 0 {
		return history.Entry{}
	}
	return m.entries[len(m.entries)-m.idx]
}
