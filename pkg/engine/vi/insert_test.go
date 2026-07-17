package vi

import (
	"testing"

	"github.com/liamg/readline/pkg/editor"
	"github.com/liamg/readline/pkg/editor/completion"
	"github.com/liamg/readline/pkg/engine"
	"github.com/liamg/readline/pkg/terminal"
)

type insertModeCompleter struct{}

func (insertModeCompleter) Complete(_ []rune, _ int) []completion.Group {
	return []completion.Group{{
		Candidates: []completion.Candidate{
			{Name: "foo", Content: "foo"},
			{Name: "bar", Content: "bar"},
		},
	}}
}

func TestVi_InsertModeTyping(t *testing.T) {
	ed := editor.New()
	eng := NewEngine(ed, nil)
	typeString(eng, "hello")
	if got := ed.BufferString(); got != "hello" {
		t.Fatalf("buffer = %q, want %q", got, "hello")
	}
}

func TestVi_InsertModeBackspace(t *testing.T) {
	ed := editor.New()
	eng := NewEngine(ed, nil)
	eng.HandleKeyEvent(key('a'))
	eng.HandleKeyEvent(key('b'))
	eng.HandleKeyEvent(namedKey(terminal.KeyBackspace))
	if got := ed.BufferString(); got != "a" {
		t.Fatalf("buffer = %q, want %q", got, "a")
	}
}

func TestVi_InsertModeDeleteRemovesCharAfterCursor(t *testing.T) {
	ed := editor.New()
	eng := NewEngine(ed, nil)
	typeString(eng, "abc")
	for range 2 {
		eng.HandleKeyEvent(namedKey(terminal.KeyLeft))
	}
	eng.HandleKeyEvent(namedKey(terminal.KeyDelete))
	if got := ed.BufferString(); got != "ac" {
		t.Fatalf("buffer = %q, want %q", got, "ac")
	}
}

func TestVi_InsertModeEnterAccepts(t *testing.T) {
	ed := editor.New()
	eng := NewEngine(ed, nil)
	eng.HandleKeyEvent(key('a'))
	done, _, err := eng.HandleKeyEvent(namedKey(terminal.KeyEnter))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !done {
		t.Fatal("expected done=true on Enter")
	}
}

func TestVi_InsertModeShiftEnterInsertsNewline(t *testing.T) {
	ed := editor.New()
	eng := NewEngine(ed, nil)
	eng.HandleKeyEvent(key('a'))
	done, _, err := eng.HandleKeyEvent(terminal.KeyEvent{Key: terminal.KeyEnter, Mod: terminal.ModShift})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if done {
		t.Fatal("expected done=false on Shift+Enter")
	}
	if got := ed.BufferString(); got != "a\n" {
		t.Fatalf("buffer = %q, want %q", got, "a\n")
	}
	if got := eng.ActiveKeymap(); got != ModeInsert {
		t.Fatalf("active keymap = %q, want %q", got, ModeInsert)
	}
}

func TestVi_EscapeSwitchesToNormal(t *testing.T) {
	ed := editor.New()
	eng := NewEngine(ed, nil)
	if eng.Cursor() != engine.CursorBar {
		t.Fatalf("initial cursor = %v, want CursorBar", eng.Cursor())
	}
	eng.HandleKeyEvent(namedKey(terminal.KeyEscape))
	if eng.Cursor() != engine.CursorBlock {
		t.Fatalf("after escape cursor = %v, want CursorBlock", eng.Cursor())
	}
}

func TestVi_EscapeDismissesCompletions(t *testing.T) {
	ed := editor.New(editor.WithCompleter(insertModeCompleter{}))
	eng := NewEngine(ed, nil)
	ed.TriggerCompletions()

	eng.HandleKeyEvent(namedKey(terminal.KeyEscape))

	if ed.GetCompletions() != nil {
		t.Fatal("completions should be dismissed after escape")
	}
}

func TestVi_InsertModeArrowsNavigateVisibleCompletions(t *testing.T) {
	ed := editor.New(editor.WithCompleter(insertModeCompleter{}))
	eng := NewEngine(ed, nil)
	ed.TriggerCompletions()

	eng.HandleKeyEvent(namedKey(terminal.KeyDown))
	if got, ok := ed.SelectedCompletion(); !ok || got != 1 {
		t.Fatalf("after down selected completion = (%d, %v), want (1, true)", got, ok)
	}
	if got := eng.ActiveKeymap(); got != ModeInsert {
		t.Fatalf("after down active keymap = %q, want %q", got, ModeInsert)
	}

	eng.HandleKeyEvent(namedKey(terminal.KeyUp))
	if got, ok := ed.SelectedCompletion(); !ok || got != 0 {
		t.Fatalf("after up selected completion = (%d, %v), want (0, true)", got, ok)
	}
	if got := eng.ActiveKeymap(); got != ModeInsert {
		t.Fatalf("after up active keymap = %q, want %q", got, ModeInsert)
	}
}

func TestVi_CursorStyles(t *testing.T) {
	ed := editor.New()
	eng := NewEngine(ed, nil)
	if eng.Cursor() != engine.CursorBar {
		t.Fatalf("insert cursor = %v, want CursorBar", eng.Cursor())
	}
	if got := eng.ActiveKeymap(); got != ModeInsert {
		t.Fatalf("insert ActiveKeymap = %q, want %q", got, ModeInsert)
	}
	eng.HandleKeyEvent(namedKey(terminal.KeyEscape))
	if eng.Cursor() != engine.CursorBlock {
		t.Fatalf("normal cursor = %v, want CursorBlock", eng.Cursor())
	}
	if got := eng.ActiveKeymap(); got != ModeNormal {
		t.Fatalf("normal ActiveKeymap = %q, want %q", got, ModeNormal)
	}
	eng.HandleKeyEvent(key('i'))
	if eng.Cursor() != engine.CursorBar {
		t.Fatalf("back to insert cursor = %v, want CursorBar", eng.Cursor())
	}
	if got := eng.ActiveKeymap(); got != ModeInsert {
		t.Fatalf("back to insert ActiveKeymap = %q, want %q", got, ModeInsert)
	}
}

func TestVi_InsertMode_CtrlH_DeletesPreviousChar(t *testing.T) {
	ed := editor.New()
	eng := NewEngine(ed, nil)
	typeString(eng, "ab")
	eng.HandleKeyEvent(ctrlKey('h'))
	if got := ed.BufferString(); got != "a" {
		t.Fatalf("buffer = %q, want %q", got, "a")
	}
}

func TestVi_InsertMode_CtrlH_AtBeginningDoesNothing(t *testing.T) {
	ed := editor.New()
	eng := NewEngine(ed, nil)
	eng.HandleKeyEvent(ctrlKey('h'))
	if got := ed.BufferString(); got != "" {
		t.Fatalf("buffer = %q, want %q", got, "")
	}
}

func TestVi_InsertMode_CtrlU_DeletesToBeginningOfLine(t *testing.T) {
	ed := editor.New()
	eng := NewEngine(ed, nil)
	typeString(eng, "hello")
	eng.HandleKeyEvent(ctrlKey('u'))
	if got := ed.BufferString(); got != "" {
		t.Fatalf("buffer = %q, want %q", got, "")
	}
}

func TestVi_InsertMode_CtrlU_PreservesTextAfterCursor(t *testing.T) {
	ed := editor.New()
	eng := NewEngine(ed, nil)
	typeString(eng, "hello world")
	for range 5 {
		eng.HandleKeyEvent(namedKey(terminal.KeyLeft))
	}
	eng.HandleKeyEvent(ctrlKey('u'))
	if got := ed.BufferString(); got != "world" {
		t.Fatalf("buffer = %q, want %q", got, "world")
	}
}

func TestVi_InsertMode_CtrlU_StopsAtNewline(t *testing.T) {
	ed := editor.New()
	eng := NewEngine(ed, nil)
	typeString(eng, "foo")
	eng.HandleKeyEvent(key('\n'))
	typeString(eng, "bar")
	eng.HandleKeyEvent(ctrlKey('u'))
	if got := ed.BufferString(); got != "foo\n" {
		t.Fatalf("buffer = %q, want %q", got, "foo\n")
	}
}

func TestVi_InsertMode_CtrlU_AtBeginningDoesNothing(t *testing.T) {
	ed := editor.New()
	eng := NewEngine(ed, nil)
	typeString(eng, "hello")
	for i := 0; i < 5; i++ {
		eng.HandleKeyEvent(namedKey(terminal.KeyLeft))
	}
	eng.HandleKeyEvent(ctrlKey('u'))
	if got := ed.BufferString(); got != "hello" {
		t.Fatalf("buffer = %q, want %q", got, "hello")
	}
}

func TestVi_InsertMode_CtrlW_DeletesPreviousWord(t *testing.T) {
	ed := editor.New()
	eng := NewEngine(ed, nil)
	typeString(eng, "hello world")
	eng.HandleKeyEvent(ctrlKey('w'))
	if got := ed.BufferString(); got != "hello " {
		t.Fatalf("buffer = %q, want %q", got, "hello ")
	}
}

func TestVi_InsertMode_CtrlW_DeletesTrailingWhitespaceAndWord(t *testing.T) {
	ed := editor.New()
	eng := NewEngine(ed, nil)
	typeString(eng, "hello   ")
	eng.HandleKeyEvent(ctrlKey('w'))
	if got := ed.BufferString(); got != "" {
		t.Fatalf("buffer = %q, want %q", got, "")
	}
}

func TestVi_InsertMode_CtrlW_OnlyWhitespaceDeletesAll(t *testing.T) {
	ed := editor.New()
	eng := NewEngine(ed, nil)
	typeString(eng, "   ")
	eng.HandleKeyEvent(ctrlKey('w'))
	if got := ed.BufferString(); got != "" {
		t.Fatalf("buffer = %q, want %q", got, "")
	}
}

func TestVi_InsertMode_CtrlW_AtBeginningDoesNothing(t *testing.T) {
	ed := editor.New()
	eng := NewEngine(ed, nil)
	eng.HandleKeyEvent(ctrlKey('w'))
	if got := ed.BufferString(); got != "" {
		t.Fatalf("buffer = %q, want %q", got, "")
	}
}

func TestVi_InsertMode_CtrlW_PreservesTextAfterCursor(t *testing.T) {
	ed := editor.New()
	eng := NewEngine(ed, nil)
	typeString(eng, "foo bar")
	for range 3 {
		eng.HandleKeyEvent(namedKey(terminal.KeyLeft))
	}
	eng.HandleKeyEvent(ctrlKey('w'))
	if got := ed.BufferString(); got != "bar" {
		t.Fatalf("buffer = %q, want %q", got, "bar")
	}
}
