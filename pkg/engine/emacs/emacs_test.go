package emacs

import (
	"testing"

	"github.com/liamg/readline/pkg/editor"
	"github.com/liamg/readline/pkg/editor/completion"
	"github.com/liamg/readline/pkg/engine"
	"github.com/liamg/readline/pkg/history"
	"github.com/liamg/readline/pkg/terminal"
)

type completionTestCompleter struct{}

func (completionTestCompleter) Complete(_ []rune, _ int) []completion.Group {
	return []completion.Group{{
		Candidates: []completion.Candidate{
			{Name: "foo", Content: "foo"},
			{Name: "bar", Content: "bar"},
			{Name: "baz", Content: "baz"},
		},
	}}
}

func key(r rune) terminal.KeyEvent {
	return terminal.KeyEvent{Key: terminal.KeyRune, Rune: r}
}

func namedKey(k terminal.Key) terminal.KeyEvent {
	return terminal.KeyEvent{Key: k}
}

func ctrlKey(r rune) terminal.KeyEvent {
	return terminal.KeyEvent{Key: terminal.KeyRune, Rune: r, Mod: terminal.ModCtrl}
}

func altKey(r rune) terminal.KeyEvent {
	return terminal.KeyEvent{Key: terminal.KeyRune, Rune: r, Mod: terminal.ModAlt}
}

func altNamedKey(k terminal.Key) terminal.KeyEvent {
	return terminal.KeyEvent{Key: k, Mod: terminal.ModAlt}
}

func typeString(eng *engine.Engine, s string) {
	for _, r := range s {
		eng.HandleKeyEvent(key(r))
	}
}

// TestEmacs_CtrlA_BeginningOfLine tests ctrl-a moves cursor to start.
func TestEmacs_CtrlA_BeginningOfLine(t *testing.T) {
	ed := editor.New()
	eng := NewEngine(ed, nil)
	typeString(eng, "hello")
	eng.HandleKeyEvent(ctrlKey('a'))
	if ed.Cursor() != 0 {
		t.Fatalf("cursor = %d, want 0", ed.Cursor())
	}
}

// TestEmacs_CtrlE_EndOfLine tests ctrl-e moves cursor to end.
func TestEmacs_CtrlE_EndOfLine(t *testing.T) {
	ed := editor.New()
	eng := NewEngine(ed, nil)
	typeString(eng, "hello")
	eng.HandleKeyEvent(ctrlKey('a'))
	eng.HandleKeyEvent(ctrlKey('e'))
	if ed.Cursor() != 5 {
		t.Fatalf("cursor = %d, want 5", ed.Cursor())
	}
}

// TestEmacs_CtrlF_ForwardChar tests ctrl-f moves cursor right.
func TestEmacs_CtrlF_ForwardChar(t *testing.T) {
	ed := editor.New()
	eng := NewEngine(ed, nil)
	typeString(eng, "ab")
	eng.HandleKeyEvent(ctrlKey('a'))
	eng.HandleKeyEvent(ctrlKey('f'))
	if ed.Cursor() != 1 {
		t.Fatalf("cursor = %d, want 1", ed.Cursor())
	}
}

// TestEmacs_CtrlB_BackwardChar tests ctrl-b moves cursor left.
func TestEmacs_CtrlB_BackwardChar(t *testing.T) {
	ed := editor.New()
	eng := NewEngine(ed, nil)
	typeString(eng, "ab")
	eng.HandleKeyEvent(ctrlKey('b'))
	if ed.Cursor() != 1 {
		t.Fatalf("cursor = %d, want 1", ed.Cursor())
	}
}

// TestEmacs_AltF_ForwardWord tests alt-f moves forward by word.
func TestEmacs_AltF_ForwardWord(t *testing.T) {
	ed := editor.New()
	eng := NewEngine(ed, nil)
	typeString(eng, "hello world")
	eng.HandleKeyEvent(ctrlKey('a'))
	eng.HandleKeyEvent(altKey('f'))
	if ed.Cursor() != 5 {
		t.Fatalf("cursor = %d, want 5", ed.Cursor())
	}
}

// TestEmacs_AltB_BackwardWord tests alt-b moves backward by word.
func TestEmacs_AltB_BackwardWord(t *testing.T) {
	ed := editor.New()
	eng := NewEngine(ed, nil)
	typeString(eng, "hello world")
	eng.HandleKeyEvent(altKey('b'))
	if ed.Cursor() != 6 {
		t.Fatalf("cursor = %d, want 6", ed.Cursor())
	}
}

// TestEmacs_CtrlD_DeleteNext tests ctrl-d deletes char under cursor.
func TestEmacs_CtrlD_DeleteNext(t *testing.T) {
	ed := editor.New()
	eng := NewEngine(ed, nil)
	typeString(eng, "ab")
	eng.HandleKeyEvent(ctrlKey('a'))
	eng.HandleKeyEvent(ctrlKey('d'))
	if got := ed.BufferString(); got != "b" {
		t.Fatalf("buffer = %q, want %q", got, "b")
	}
}

func TestEmacs_CtrlD_DeletesWholeGrapheme(t *testing.T) {
	ed := editor.New()
	eng := NewEngine(ed, nil)
	typeString(eng, "🐈🐈‍⬛x")
	eng.HandleKeyEvent(ctrlKey('a'))
	eng.HandleKeyEvent(ctrlKey('d'))
	if got := ed.BufferString(); got != "🐈‍⬛x" {
		t.Fatalf("buffer = %q, want %q", got, "🐈‍⬛x")
	}
}

// TestEmacs_CtrlK_KillLine tests ctrl-k kills from cursor to end.
func TestEmacs_CtrlK_KillLine(t *testing.T) {
	ed := editor.New()
	eng := NewEngine(ed, nil)
	typeString(eng, "hello world")
	eng.HandleKeyEvent(ctrlKey('a'))
	eng.HandleKeyEvent(ctrlKey('f')) // cursor at 1
	eng.HandleKeyEvent(ctrlKey('k'))
	if got := ed.BufferString(); got != "h" {
		t.Fatalf("buffer = %q, want %q", got, "h")
	}
}

// TestEmacs_CtrlU_BackwardKillLine tests ctrl-u kills from cursor to start.
func TestEmacs_CtrlU_BackwardKillLine(t *testing.T) {
	ed := editor.New()
	eng := NewEngine(ed, nil)
	typeString(eng, "hello")
	eng.HandleKeyEvent(ctrlKey('u'))
	if got := ed.BufferString(); got != "" {
		t.Fatalf("buffer = %q, want empty", got)
	}
}

// TestEmacs_CtrlW_BackwardKillWord tests ctrl-w kills previous word.
func TestEmacs_CtrlW_BackwardKillWord(t *testing.T) {
	ed := editor.New()
	eng := NewEngine(ed, nil)
	typeString(eng, "hello world")
	eng.HandleKeyEvent(ctrlKey('w'))
	if got := ed.BufferString(); got != "hello " {
		t.Fatalf("buffer = %q, want %q", got, "hello ")
	}
}

// TestEmacs_AltD_KillWord tests alt-d kills next word.
func TestEmacs_AltD_KillWord(t *testing.T) {
	ed := editor.New()
	eng := NewEngine(ed, nil)
	typeString(eng, "hello world")
	eng.HandleKeyEvent(ctrlKey('a'))
	eng.HandleKeyEvent(altKey('d'))
	if got := ed.BufferString(); got != " world" {
		t.Fatalf("buffer = %q, want %q", got, " world")
	}
}

// TestEmacs_AltBackspace_BackwardKillWord tests alt-backspace kills previous word.
func TestEmacs_AltBackspace_BackwardKillWord(t *testing.T) {
	ed := editor.New()
	eng := NewEngine(ed, nil)
	typeString(eng, "hello world")
	eng.HandleKeyEvent(altNamedKey(terminal.KeyBackspace))
	if got := ed.BufferString(); got != "hello " {
		t.Fatalf("buffer = %q, want %q", got, "hello ")
	}
}

// TestEmacs_CtrlT_TransposeChars tests ctrl-t swaps adjacent chars.
func TestEmacs_CtrlT_TransposeChars(t *testing.T) {
	ed := editor.New()
	eng := NewEngine(ed, nil)
	typeString(eng, "ab")
	eng.HandleKeyEvent(ctrlKey('a'))
	eng.HandleKeyEvent(ctrlKey('f'))
	eng.HandleKeyEvent(ctrlKey('t'))
	if got := ed.BufferString(); got != "ba" {
		t.Fatalf("buffer = %q, want %q", got, "ba")
	}
}

func TestEmacs_CtrlT_TransposeGraphemes(t *testing.T) {
	ed := editor.New()
	eng := NewEngine(ed, nil)
	typeString(eng, "🐈🐈‍⬛x")
	eng.HandleKeyEvent(ctrlKey('a'))
	eng.HandleKeyEvent(ctrlKey('t'))
	if got := ed.BufferString(); got != "🐈‍⬛🐈x" {
		t.Fatalf("buffer = %q, want %q", got, "🐈‍⬛🐈x")
	}
}

// TestEmacs_CtrlY_Yank tests ctrl-y yanks last killed text.
func TestEmacs_CtrlY_Yank(t *testing.T) {
	ed := editor.New()
	eng := NewEngine(ed, nil)
	typeString(eng, "hello world")
	eng.HandleKeyEvent(ctrlKey('k')) // kill from end (nothing, cursor is at end)
	eng.HandleKeyEvent(ctrlKey('u')) // kill "hello world"
	eng.HandleKeyEvent(ctrlKey('y')) // yank "hello world"
	if got := ed.BufferString(); got != "hello world" {
		t.Fatalf("buffer = %q, want %q", got, "hello world")
	}
}

// TestEmacs_CtrlP_HistoryPrevious tests ctrl-p loads previous history.
func TestEmacs_CtrlP_HistoryPrevious(t *testing.T) {
	ed := editor.New()
	h := newStubHistory([]string{"cmd1", "cmd2"})
	eng := NewEngine(ed, h)
	eng.HandleKeyEvent(ctrlKey('p'))
	if got := ed.BufferString(); got != "cmd2" {
		t.Fatalf("buffer = %q, want %q", got, "cmd2")
	}
}

// TestEmacs_CtrlN_HistoryNext tests ctrl-n returns toward current after ctrl-p.
func TestEmacs_CtrlN_HistoryNext(t *testing.T) {
	ed := editor.New()
	h := newStubHistory([]string{"cmd1", "cmd2"})
	eng := NewEngine(ed, h)
	eng.HandleKeyEvent(ctrlKey('p')) // "cmd2"
	eng.HandleKeyEvent(ctrlKey('p')) // "cmd1"
	eng.HandleKeyEvent(ctrlKey('n')) // back to "cmd2"
	if got := ed.BufferString(); got != "cmd2" {
		t.Fatalf("buffer = %q, want %q", got, "cmd2")
	}
}

func TestEmacs_ArrowsNavigateVisibleCompletions(t *testing.T) {
	ed := editor.New(editor.WithCompleter(completionTestCompleter{}))
	eng := NewEngine(ed, nil)
	ed.TriggerCompletions()

	eng.HandleKeyEvent(namedKey(terminal.KeyDown))
	if got, ok := ed.SelectedCompletion(); !ok || got != 1 {
		t.Fatalf("after down selected completion = (%d, %v), want (1, true)", got, ok)
	}

	eng.HandleKeyEvent(namedKey(terminal.KeyUp))
	if got, ok := ed.SelectedCompletion(); !ok || got != 0 {
		t.Fatalf("after up selected completion = (%d, %v), want (0, true)", got, ok)
	}
}

func TestEmacs_CtrlPNNavigateVisibleCompletions(t *testing.T) {
	ed := editor.New(editor.WithCompleter(completionTestCompleter{}))
	eng := NewEngine(ed, nil)
	ed.TriggerCompletions()

	eng.HandleKeyEvent(ctrlKey('p'))
	if got, ok := ed.SelectedCompletion(); !ok || got != 2 {
		t.Fatalf("after ctrl-p selected completion = (%d, %v), want (2, true)", got, ok)
	}

	eng.HandleKeyEvent(ctrlKey('n'))
	if got, ok := ed.SelectedCompletion(); !ok || got != 0 {
		t.Fatalf("after ctrl-n selected completion = (%d, %v), want (0, true)", got, ok)
	}
}

// TestEmacs_Home_BeginningOfLine tests Home key moves to start.
func TestEmacs_Home_BeginningOfLine(t *testing.T) {
	ed := editor.New()
	eng := NewEngine(ed, nil)
	typeString(eng, "hello")
	eng.HandleKeyEvent(namedKey(terminal.KeyHome))
	if ed.Cursor() != 0 {
		t.Fatalf("cursor = %d, want 0", ed.Cursor())
	}
}

// TestEmacs_End_EndOfLine tests End key moves to end.
func TestEmacs_End_EndOfLine(t *testing.T) {
	ed := editor.New()
	eng := NewEngine(ed, nil)
	typeString(eng, "hello")
	eng.HandleKeyEvent(namedKey(terminal.KeyHome))
	eng.HandleKeyEvent(namedKey(terminal.KeyEnd))
	if ed.Cursor() != 5 {
		t.Fatalf("cursor = %d, want 5", ed.Cursor())
	}
}

// TestEmacs_CtrlLeft_BackwardWord tests ctrl-left jumps back a word.
func TestEmacs_CtrlLeft_BackwardWord(t *testing.T) {
	ed := editor.New()
	eng := NewEngine(ed, nil)
	typeString(eng, "hello world")
	eng.HandleKeyEvent(terminal.KeyEvent{Key: terminal.KeyLeft, Mod: terminal.ModCtrl})
	if ed.Cursor() != 6 {
		t.Fatalf("cursor = %d, want 6", ed.Cursor())
	}
}

// TestEmacs_CtrlRight_ForwardWord tests ctrl-right jumps forward a word.
func TestEmacs_CtrlRight_ForwardWord(t *testing.T) {
	ed := editor.New()
	eng := NewEngine(ed, nil)
	typeString(eng, "hello world")
	eng.HandleKeyEvent(namedKey(terminal.KeyHome))
	eng.HandleKeyEvent(terminal.KeyEvent{Key: terminal.KeyRight, Mod: terminal.ModCtrl})
	if ed.Cursor() != 5 {
		t.Fatalf("cursor = %d, want 5", ed.Cursor())
	}
}

// ---- isWordChar ------------------------------------------------------------

func TestIsWordChar(t *testing.T) {
	for _, r := range "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789_" {
		if !isWordChar(r) {
			t.Errorf("isWordChar(%q) = false, want true", r)
		}
	}
	for _, r := range " \t!@#$%^&*()-+=[]{}|;':\",./<>?" {
		if isWordChar(r) {
			t.Errorf("isWordChar(%q) = true, want false", r)
		}
	}
}

// stubHistory is a minimal history.History implementation for emacs engine tests.
type stubHistory struct {
	entries []history.Entry
	idx     int
	backup  string
}

func newStubHistory(lines []string) *stubHistory {
	entries := make([]history.Entry, len(lines))
	for i, l := range lines {
		entries[i] = history.Entry{Text: l}
	}
	return &stubHistory{entries: entries}
}

func (h *stubHistory) IsCurrent() bool          { return h.idx == 0 }
func (h *stubHistory) Reset()                   { h.idx = 0 }
func (h *stubHistory) SetFilter(_ string)       {}
func (h *stubHistory) SetPrefixFilter(_ string) {}
func (h *stubHistory) Append(_ string, _ bool)  {}

func (h *stubHistory) Previous() history.Entry {
	if h.idx >= len(h.entries) {
		return history.Entry{}
	}
	h.idx++
	return h.entries[len(h.entries)-h.idx]
}

func (h *stubHistory) Next() history.Entry {
	if h.idx <= 0 {
		return history.Entry{}
	}
	h.idx--
	if h.idx == 0 {
		return history.Entry{Text: h.backup}
	}
	return h.entries[len(h.entries)-h.idx]
}

func TestEmacs_TypingInsertsChars(t *testing.T) {
	ed := editor.New()
	eng := NewEngine(ed, nil)
	for _, r := range "hello" {
		eng.HandleKeyEvent(key(r))
	}
	if got := ed.BufferString(); got != "hello" {
		t.Fatalf("buffer = %q, want %q", got, "hello")
	}
	if ed.Cursor() != 5 {
		t.Fatalf("cursor = %d, want 5", ed.Cursor())
	}
}

func TestEmacs_BackspaceDeletesPrevious(t *testing.T) {
	ed := editor.New()
	eng := NewEngine(ed, nil)
	eng.HandleKeyEvent(key('a'))
	eng.HandleKeyEvent(key('b'))
	eng.HandleKeyEvent(namedKey(terminal.KeyBackspace))
	if got := ed.BufferString(); got != "a" {
		t.Fatalf("buffer = %q, want %q", got, "a")
	}
	if ed.Cursor() != 1 {
		t.Fatalf("cursor = %d, want 1", ed.Cursor())
	}
}

func TestEmacs_BackspaceDeletesWholeGrapheme(t *testing.T) {
	ed := editor.New()
	eng := NewEngine(ed, nil)
	typeString(eng, "🐈🐈‍⬛")
	eng.HandleKeyEvent(namedKey(terminal.KeyBackspace))
	if got := ed.BufferString(); got != "🐈" {
		t.Fatalf("buffer = %q, want %q", got, "🐈")
	}
	if ed.Cursor() != len([]rune("🐈")) {
		t.Fatalf("cursor = %d, want %d", ed.Cursor(), len([]rune("🐈")))
	}
}

func TestEmacs_BackspaceAtStartIsNoOp(t *testing.T) {
	ed := editor.New()
	eng := NewEngine(ed, nil)
	eng.HandleKeyEvent(namedKey(terminal.KeyBackspace))
	if got := ed.BufferString(); got != "" {
		t.Fatalf("buffer = %q, want empty", got)
	}
	if ed.Cursor() != 0 {
		t.Fatalf("cursor = %d, want 0", ed.Cursor())
	}
}

func TestEmacs_EnterAcceptsLine(t *testing.T) {
	ed := editor.New()
	eng := NewEngine(ed, nil)
	eng.HandleKeyEvent(key('x'))
	done, _, err := eng.HandleKeyEvent(namedKey(terminal.KeyEnter))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !done {
		t.Fatal("expected done=true on Enter")
	}
	if ed.BufferString() != "x" {
		t.Fatalf("buffer = %q, want %q", ed.BufferString(), "x")
	}
}

func TestEmacs_NonPrintableIgnored(t *testing.T) {
	ed := editor.New()
	eng := NewEngine(ed, nil)
	// Alt-modified rune should not insert (Mod != 0).
	eng.HandleKeyEvent(terminal.KeyEvent{Key: terminal.KeyRune, Rune: 'a', Mod: terminal.ModAlt})
	if got := ed.BufferString(); got != "" {
		t.Fatalf("alt-modified rune inserted into buffer: %q", got)
	}
}

func TestEmacs_SingleKeymap_CursorIsDefault(t *testing.T) {
	ed := editor.New()
	eng := NewEngine(ed, nil)
	// Emacs has no modal cursor switching; default keymap has no Cursor set.
	if got := eng.Cursor(); got != engine.CursorDefault {
		t.Fatalf("cursor = %v, want CursorDefault", got)
	}
}
