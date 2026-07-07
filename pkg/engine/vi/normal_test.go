package vi

import (
	"testing"

	"github.com/liamg/readline/pkg/editor"
	"github.com/liamg/readline/pkg/engine"
	"github.com/liamg/readline/pkg/terminal"
)

func TestVi_NormalMode_iSwitchesToInsert(t *testing.T) {
	ed := editor.New()
	eng := NewEngine(ed, nil)
	eng.HandleKeyEvent(namedKey(terminal.KeyEscape)) // → normal
	eng.HandleKeyEvent(key('i'))                     // → insert
	if eng.Cursor() != engine.CursorBar {
		t.Fatalf("cursor = %v, want CursorBar after i", eng.Cursor())
	}
	eng.HandleKeyEvent(key('z'))
	if got := ed.BufferString(); got != "z" {
		t.Fatalf("buffer = %q, want %q", got, "z")
	}
}

func TestVi_NormalMode_aSwitchesToInsertAndAdvancesCursor(t *testing.T) {
	ed := editor.New()
	eng := NewEngine(ed, nil)
	eng.HandleKeyEvent(key('a'))
	eng.HandleKeyEvent(key('b'))
	eng.HandleKeyEvent(namedKey(terminal.KeyEscape)) // → normal
	ed.MoveCursor(-2)                                // cursor now at 0
	cursorBefore := ed.Cursor()
	eng.HandleKeyEvent(key('a')) // append: advance cursor then → insert
	if ed.Cursor() != cursorBefore+1 {
		t.Fatalf("cursor after 'a' = %d, want %d", ed.Cursor(), cursorBefore+1)
	}
	if eng.Cursor() != engine.CursorBar {
		t.Fatalf("cursor style = %v, want CursorBar", eng.Cursor())
	}
}

func TestVi_NormalMode_hlMoveLeftAndRight(t *testing.T) {
	ed, eng, _ := normalEngine(t, "abc")
	moveCursorTo(ed, 1)
	mustHandleRunes(t, eng, "h")
	if got := ed.Cursor(); got != 0 {
		t.Fatalf("after h cursor = %d, want 0", got)
	}
	mustHandleRunes(t, eng, "l")
	if got := ed.Cursor(); got != 1 {
		t.Fatalf("after l cursor = %d, want 1", got)
	}
}

func TestVi_NormalMode_EndMovesToInsertionPointAfterLine(t *testing.T) {
	ed, eng, _ := normalEngine(t, "bat '~/Library/Application Support/paw/settings.json")
	moveCursorTo(ed, 5)

	mustHandleKey(t, eng, namedKey(terminal.KeyEnd))

	if got := ed.Cursor(); got != len(ed.Runes()) {
		t.Fatalf("cursor = %d, want %d", got, len(ed.Runes()))
	}
	if got := eng.ActiveKeymap(); got != ModeInsert {
		t.Fatalf("keymap = %q, want %q", got, ModeInsert)
	}
}

func TestVi_NormalMode_RightAtLastCharacterEntersAppendMode(t *testing.T) {
	ed, eng, _ := normalEngine(t, "abc")
	moveCursorTo(ed, len(ed.Runes())-1)

	mustHandleKey(t, eng, namedKey(terminal.KeyRight))

	if got := ed.Cursor(); got != len(ed.Runes()) {
		t.Fatalf("cursor = %d, want %d", got, len(ed.Runes()))
	}
	if got := eng.ActiveKeymap(); got != ModeInsert {
		t.Fatalf("keymap = %q, want %q", got, ModeInsert)
	}
}

func TestVi_NormalMode_lRemainsClampedToLastCharacter(t *testing.T) {
	ed, eng, _ := normalEngine(t, "abc")
	moveCursorTo(ed, len(ed.Runes())-1)

	mustHandleRunes(t, eng, "l")

	if got := ed.Cursor(); got != len(ed.Runes())-1 {
		t.Fatalf("cursor = %d, want %d", got, len(ed.Runes())-1)
	}
	if got := eng.ActiveKeymap(); got != ModeNormal {
		t.Fatalf("keymap = %q, want %q", got, ModeNormal)
	}
}

func TestVi_NormalMode_xDeletesCurrentChar(t *testing.T) {
	ed := editor.New()
	eng := NewEngine(ed, nil)
	eng.HandleKeyEvent(key('a'))
	eng.HandleKeyEvent(key('b'))
	eng.HandleKeyEvent(key('c'))
	eng.HandleKeyEvent(namedKey(terminal.KeyEscape)) // → normal, cursor clamped to 2 (on 'c')
	ed.MoveCursor(-1)                                // cursor at 1, pointing at 'b'
	eng.HandleKeyEvent(key('x'))
	if got := ed.BufferString(); got != "ac" {
		t.Fatalf("after x buffer = %q, want %q", got, "ac")
	}
}

func TestVi_NormalMode_xDeletesWholeGrapheme(t *testing.T) {
	ed, eng, _ := normalEngine(t, "🐈🐈‍⬛x")
	moveCursorTo(ed, 0)
	mustHandleRunes(t, eng, "x")
	if got := ed.BufferString(); got != "🐈‍⬛x" {
		t.Fatalf("after x buffer = %q, want %q", got, "🐈‍⬛x")
	}
}

func TestVi_NormalMode_CountXDeletesMultipleGraphemes(t *testing.T) {
	ed, eng, _ := normalEngine(t, "🐈🐈‍⬛abc")
	moveCursorTo(ed, 0)
	mustHandleRunes(t, eng, "3x")
	if got := ed.BufferString(); got != "bc" {
		t.Fatalf("after 3x buffer = %q, want %q", got, "bc")
	}
}

func TestVi_NormalMode_xWritesDeletedCharToDefaultRegister(t *testing.T) {
	ed, eng, v := normalEngine(t, "abc")
	moveCursorTo(ed, 1)
	mustHandleRunes(t, eng, "x")
	if got := string(v.state.registers[defaultRegister]); got != "b" {
		t.Fatalf("default register = %q, want %q", got, "b")
	}
}

func TestVi_NormalMode_rReplacesChar(t *testing.T) {
	ed := editor.New()
	eng := NewEngine(ed, nil)
	eng.HandleKeyEvent(key('a'))
	eng.HandleKeyEvent(key('b'))
	eng.HandleKeyEvent(key('c'))
	eng.HandleKeyEvent(namedKey(terminal.KeyEscape)) // → normal, cursor clamped to 2 (on 'c')
	ed.MoveCursor(-1)                                // cursor at 1, pointing at 'b'
	eng.HandleKeyEvent(key('r'))
	eng.HandleKeyEvent(key('X'))
	if got := ed.BufferString(); got != "aXc" {
		t.Fatalf("after r+X buffer = %q, want %q", got, "aXc")
	}
}

func TestVi_NormalMode_rReplacesGrapheme(t *testing.T) {
	ed, eng, _ := normalEngine(t, "🐈🐈‍⬛x")
	moveCursorTo(ed, 0)
	mustHandleRunes(t, eng, "rZ")
	if got := ed.BufferString(); got != "Z🐈‍⬛x" {
		t.Fatalf("after r+Z buffer = %q, want %q", got, "Z🐈‍⬛x")
	}
}

func TestVi_NormalMode_CountedMotions(t *testing.T) {
	tests := []struct {
		name       string
		buffer     string
		cursor     int
		keys       string
		wantCursor int
	}{
		{name: "2w", buffer: "one two three", cursor: 0, keys: "2w", wantCursor: 8},
		{name: "2b", buffer: "one two three", cursor: 8, keys: "2b", wantCursor: 0},
		{name: "2e", buffer: "one two three", cursor: 0, keys: "2e", wantCursor: 6},
		{name: "2W", buffer: "one,two three four", cursor: 0, keys: "2W", wantCursor: 14},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ed, eng, _ := normalEngine(t, tt.buffer)
			moveCursorTo(ed, tt.cursor)
			mustHandleRunes(t, eng, tt.keys)
			if got := ed.Cursor(); got != tt.wantCursor {
				t.Fatalf("cursor = %d, want %d", got, tt.wantCursor)
			}
		})
	}
}

func TestVi_NormalMode_CharacterFindMotions(t *testing.T) {
	tests := []struct {
		name       string
		buffer     string
		cursor     int
		keys       string
		wantCursor int
	}{
		{name: "f", buffer: "ab cd ef", cursor: 0, keys: "fc", wantCursor: 3},
		{name: "F", buffer: "ab cd ef cd", cursor: 8, keys: "Fd", wantCursor: 4},
		{name: "t", buffer: "ab cd ef", cursor: 0, keys: "td", wantCursor: 3},
		{name: "T", buffer: "ab cd ef", cursor: 7, keys: "Tc", wantCursor: 4},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ed, eng, _ := normalEngine(t, tt.buffer)
			moveCursorTo(ed, tt.cursor)
			mustHandleRunes(t, eng, tt.keys)
			if got := ed.Cursor(); got != tt.wantCursor {
				t.Fatalf("cursor = %d, want %d", got, tt.wantCursor)
			}
		})
	}
}

func TestVi_NormalMode_RepeatAndReverseCharacterFind(t *testing.T) {
	ed, eng, _ := normalEngine(t, "ab cd cd")
	moveCursorTo(ed, 0)
	mustHandleRunes(t, eng, "fc")
	if got := ed.Cursor(); got != 3 {
		t.Fatalf("after fc cursor = %d, want 3", got)
	}
	mustHandleRunes(t, eng, ";")
	if got := ed.Cursor(); got != 6 {
		t.Fatalf("after ; cursor = %d, want 6", got)
	}
	mustHandleRunes(t, eng, ",")
	if got := ed.Cursor(); got != 3 {
		t.Fatalf("after , cursor = %d, want 3", got)
	}
}

func TestVi_NormalMode_EnterAccepts(t *testing.T) {
	ed := editor.New()
	eng := NewEngine(ed, nil)
	eng.HandleKeyEvent(namedKey(terminal.KeyEscape))
	done, _, err := eng.HandleKeyEvent(namedKey(terminal.KeyEnter))
	if err != nil || !done {
		t.Fatalf("enter in normal mode: done=%v err=%v", done, err)
	}
}

func TestVi_NormalMode_CaretMovesToFirstNonBlank(t *testing.T) {
	ed, eng, _ := normalEngine(t, "  hello")
	moveCursorTo(ed, 5)
	mustHandleRunes(t, eng, "^")
	if got := ed.Cursor(); got != 2 {
		t.Fatalf("cursor = %d, want 2", got)
	}
}

func TestVi_NormalMode_ISwitchesToInsertAtFirstNonBlank(t *testing.T) {
	ed, eng, _ := normalEngine(t, "  hello")
	moveCursorTo(ed, 5)
	mustHandleRunes(t, eng, "I")
	if got := eng.Cursor(); got != engine.CursorBar {
		t.Fatalf("cursor style = %v, want CursorBar", got)
	}
	mustHandleRunes(t, eng, "X")
	if got := ed.BufferString(); got != "  Xhello" {
		t.Fatalf("buffer = %q, want %q", got, "  Xhello")
	}
}

func TestVi_NormalMode_ASwitchesToInsertAtEndOfLine(t *testing.T) {
	ed, eng, _ := normalEngine(t, "hello")
	moveCursorTo(ed, 0)
	mustHandleRunes(t, eng, "A")
	if got := eng.Cursor(); got != engine.CursorBar {
		t.Fatalf("cursor style = %v, want CursorBar", got)
	}
	mustHandleRunes(t, eng, "X")
	if got := ed.BufferString(); got != "helloX" {
		t.Fatalf("buffer = %q, want %q", got, "helloX")
	}
}

func TestVi_NormalMode_sSubstitutesCharAndEntersInsert(t *testing.T) {
	ed, eng, _ := normalEngine(t, "abc")
	moveCursorTo(ed, 1)
	mustHandleRunes(t, eng, "s")
	if got := eng.Cursor(); got != engine.CursorBar {
		t.Fatalf("cursor style = %v, want CursorBar", got)
	}
	mustHandleRunes(t, eng, "X")
	if got := ed.BufferString(); got != "aXc" {
		t.Fatalf("buffer = %q, want %q", got, "aXc")
	}
}

func TestVi_NormalMode_sSubstitutesWholeGrapheme(t *testing.T) {
	ed, eng, _ := normalEngine(t, "🐈🐈‍⬛x")
	moveCursorTo(ed, 0)
	mustHandleRunes(t, eng, "s")
	mustHandleRunes(t, eng, "Y")
	if got := ed.BufferString(); got != "Y🐈‍⬛x" {
		t.Fatalf("buffer = %q, want %q", got, "Y🐈‍⬛x")
	}
}

func TestVi_NormalMode_SubstituteLineCommands(t *testing.T) {
	tests := []struct {
		name string
		keys string
	}{
		{name: "S", keys: "S"},
		{name: "cc", keys: "cc"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ed, eng, _ := normalEngine(t, "first\nsecond\nthird")
			moveCursorTo(ed, 7)
			mustHandleRunes(t, eng, tt.keys)
			if got := eng.Cursor(); got != engine.CursorBar {
				t.Fatalf("cursor style = %v, want CursorBar", got)
			}
			mustHandleRunes(t, eng, "X")
			if got := ed.BufferString(); got != "first\nX\nthird" {
				t.Fatalf("buffer = %q, want %q", got, "first\nX\nthird")
			}
		})
	}
}

func TestVi_NormalMode_CChangesToEndOfLineAndEntersInsert(t *testing.T) {
	ed, eng, _ := normalEngine(t, "hello world")
	moveCursorTo(ed, 6)
	mustHandleRunes(t, eng, "C")
	if got := eng.Cursor(); got != engine.CursorBar {
		t.Fatalf("cursor style = %v, want CursorBar", got)
	}
	mustHandleRunes(t, eng, "X")
	if got := ed.BufferString(); got != "hello X" {
		t.Fatalf("buffer = %q, want %q", got, "hello X")
	}
}

func TestVi_NormalMode_DDeletesToEndOfLine(t *testing.T) {
	ed, eng, _ := normalEngine(t, "hello world")
	moveCursorTo(ed, 6)
	mustHandleRunes(t, eng, "D")
	if got := ed.BufferString(); got != "hello " {
		t.Fatalf("buffer = %q, want %q", got, "hello ")
	}
}

func TestVi_NormalMode_DeleteWithMotion(t *testing.T) {
	ed, eng, _ := normalEngine(t, "hello world")
	moveCursorTo(ed, 6)
	mustHandleRunes(t, eng, "d$")
	if got := ed.BufferString(); got != "hello " {
		t.Fatalf("buffer = %q, want %q", got, "hello ")
	}
}

func TestVi_NormalMode_CountedDeleteWithMotion(t *testing.T) {
	ed, eng, _ := normalEngine(t, "one two three four")
	moveCursorTo(ed, 0)
	mustHandleRunes(t, eng, "d2w")
	if got := ed.BufferString(); got != "three four" {
		t.Fatalf("buffer = %q, want %q", got, "three four")
	}
}

func TestVi_NormalMode_DeleteWithTextObject(t *testing.T) {
	ed, eng, _ := normalEngine(t, `say "hello" now`)
	moveCursorTo(ed, 6)
	mustHandleRunes(t, eng, `di"`)
	if got := ed.BufferString(); got != `say "" now` {
		t.Fatalf("buffer = %q, want %q", got, `say "" now`)
	}
}

func TestVi_NormalMode_ddDeletesCurrentLine(t *testing.T) {
	ed, eng, _ := normalEngine(t, "first\nsecond\nthird")
	moveCursorTo(ed, 7)
	mustHandleRunes(t, eng, "dd")
	if got := ed.BufferString(); got != "first\nthird" {
		t.Fatalf("buffer = %q, want %q", got, "first\nthird")
	}
}

func TestVi_NormalMode_CountedddDeletesMultipleLines(t *testing.T) {
	ed, eng, _ := normalEngine(t, "first\nsecond\nthird\nfourth")
	moveCursorTo(ed, 0)
	mustHandleRunes(t, eng, "2dd")
	if got := ed.BufferString(); got != "third\nfourth" {
		t.Fatalf("buffer = %q, want %q", got, "third\nfourth")
	}
}

func TestVi_NormalMode_ChangeWithMotion(t *testing.T) {
	ed, eng, _ := normalEngine(t, "hello world")
	moveCursorTo(ed, 6)
	mustHandleRunes(t, eng, "c$")
	if got := eng.Cursor(); got != engine.CursorBar {
		t.Fatalf("cursor style = %v, want CursorBar", got)
	}
	mustHandleRunes(t, eng, "X")
	if got := ed.BufferString(); got != "hello X" {
		t.Fatalf("buffer = %q, want %q", got, "hello X")
	}
}

func TestVi_NormalMode_ChangeWithTextObject(t *testing.T) {
	ed, eng, _ := normalEngine(t, "alpha beta gamma")
	moveCursorTo(ed, 6)
	mustHandleRunes(t, eng, "ciw")
	if got := eng.Cursor(); got != engine.CursorBar {
		t.Fatalf("cursor style = %v, want CursorBar", got)
	}
	mustHandleRunes(t, eng, "X")
	if got := ed.BufferString(); got != "alpha X gamma" {
		t.Fatalf("buffer = %q, want %q", got, "alpha X gamma")
	}
}

func TestVi_NormalMode_YankWithMotionWritesToRegister(t *testing.T) {
	ed, eng, v := normalEngine(t, "hello world")
	moveCursorTo(ed, 6)
	mustHandleRunes(t, eng, `"ay$`)
	if got := string(v.state.registers['a']); got != "world" {
		t.Fatalf("register a = %q, want %q", got, "world")
	}
}

func TestVi_NormalMode_YankWithTextObjectWritesToDefaultRegister(t *testing.T) {
	ed, eng, v := normalEngine(t, "alpha beta gamma")
	moveCursorTo(ed, 6)
	mustHandleRunes(t, eng, "yiw")
	if got := string(v.state.registers[defaultRegister]); got != "beta" {
		t.Fatalf("default register = %q, want %q", got, "beta")
	}
}

func TestVi_NormalMode_yyYanksLineToRegister(t *testing.T) {
	ed, eng, v := normalEngine(t, "first\nsecond\nthird")
	moveCursorTo(ed, 7)
	mustHandleRunes(t, eng, `"ayy`)
	if got := string(v.state.registers['a']); got != "second\n" {
		t.Fatalf("register a = %q, want %q", got, "second\n")
	}
}

func TestVi_NormalMode_DeleteToNamedRegister(t *testing.T) {
	ed, eng, v := normalEngine(t, "hello world")
	moveCursorTo(ed, 6)
	mustHandleRunes(t, eng, `"adw`)
	if got := ed.BufferString(); got != "hello " {
		t.Fatalf("buffer = %q, want %q", got, "hello ")
	}
	if got := string(v.state.registers['a']); got != "world" {
		t.Fatalf("register a = %q, want %q", got, "world")
	}
}

func TestVi_NormalMode_pPastesAfterCursor(t *testing.T) {
	ed, eng, v := normalEngine(t, "ab")
	moveCursorTo(ed, 0)
	v.state.registers['"'] = []rune("XY")
	mustHandleRunes(t, eng, "p")
	if got := ed.BufferString(); got != "aXYb" {
		t.Fatalf("buffer = %q, want %q", got, "aXYb")
	}
}

func TestVi_NormalMode_PPastesBeforeCursor(t *testing.T) {
	ed, eng, v := normalEngine(t, "ab")
	moveCursorTo(ed, 0)
	v.state.registers['"'] = []rune("XY")
	mustHandleRunes(t, eng, "P")
	if got := ed.BufferString(); got != "XYab" {
		t.Fatalf("buffer = %q, want %q", got, "XYab")
	}
}

func TestVi_NormalMode_TildeTogglesCase(t *testing.T) {
	ed, eng, _ := normalEngine(t, "aBc")
	moveCursorTo(ed, 1)
	mustHandleRunes(t, eng, "~")
	if got := ed.BufferString(); got != "abc" {
		t.Fatalf("buffer = %q, want %q", got, "abc")
	}
}

func TestVi_NormalMode_RReplacesUntilEscape(t *testing.T) {
	ed, eng, _ := normalEngine(t, "abcd")
	moveCursorTo(ed, 1)
	mustHandleRunes(t, eng, "RXY")
	mustHandleKey(t, eng, namedKey(terminal.KeyEscape))
	if got := ed.BufferString(); got != "aXYd" {
		t.Fatalf("buffer = %q, want %q", got, "aXYd")
	}
}

func TestVi_NormalMode_DotRepeatsLastChange(t *testing.T) {
	ed, eng, _ := normalEngine(t, "abcd")
	moveCursorTo(ed, 1)
	mustHandleRunes(t, eng, "x.")
	if got := ed.BufferString(); got != "ad" {
		t.Fatalf("buffer = %q, want %q", got, "ad")
	}
}

func TestVi_NormalMode_CtrlAIncrementsNumber(t *testing.T) {
	ed, eng, _ := normalEngine(t, "job 41 done")
	moveCursorTo(ed, 0)
	mustHandleKey(t, eng, ctrlKey('a'))
	if got := ed.BufferString(); got != "job 42 done" {
		t.Fatalf("buffer = %q, want %q", got, "job 42 done")
	}
	if got := ed.Cursor(); got != 5 {
		t.Fatalf("cursor = %d, want %d", got, 5)
	}
}

func TestVi_NormalMode_CtrlXDecrementsNumber(t *testing.T) {
	ed, eng, _ := normalEngine(t, "job 42 done")
	moveCursorTo(ed, 4)
	mustHandleKey(t, eng, ctrlKey('x'))
	if got := ed.BufferString(); got != "job 41 done" {
		t.Fatalf("buffer = %q, want %q", got, "job 41 done")
	}
}

func TestVi_NormalMode_CtrlAUsesCount(t *testing.T) {
	ed, eng, _ := normalEngine(t, "job 40 done")
	moveCursorTo(ed, 0)
	mustHandleRunes(t, eng, "5")
	mustHandleKey(t, eng, ctrlKey('a'))
	if got := ed.BufferString(); got != "job 45 done" {
		t.Fatalf("buffer = %q, want %q", got, "job 45 done")
	}
}

func TestVi_NormalMode_CtrlAFindsNumberContainingCursor(t *testing.T) {
	ed, eng, _ := normalEngine(t, "version 129")
	moveCursorTo(ed, 9)
	mustHandleKey(t, eng, ctrlKey('a'))
	if got := ed.BufferString(); got != "version 130" {
		t.Fatalf("buffer = %q, want %q", got, "version 130")
	}
}

func TestVi_NormalMode_CtrlAHandlesNegativeNumbers(t *testing.T) {
	ed, eng, _ := normalEngine(t, "offset -2")
	moveCursorTo(ed, 0)
	mustHandleKey(t, eng, ctrlKey('a'))
	if got := ed.BufferString(); got != "offset -1" {
		t.Fatalf("buffer = %q, want %q", got, "offset -1")
	}
}

func TestVi_NormalMode_CtrlAPreservesLeadingZeroWidth(t *testing.T) {
	ed, eng, _ := normalEngine(t, "id 007")
	moveCursorTo(ed, 0)
	mustHandleKey(t, eng, ctrlKey('a'))
	if got := ed.BufferString(); got != "id 008" {
		t.Fatalf("buffer = %q, want %q", got, "id 008")
	}
}

func TestVi_NormalMode_HistorySearchAndRepeat(t *testing.T) {
	ed, eng, _ := normalEngine(t, "", "git status", "go test", "git push", "go build")
	mustHandleRunes(t, eng, "/go")
	mustHandleKey(t, eng, namedKey(terminal.KeyEnter))
	if got := ed.BufferString(); got != "go build" {
		t.Fatalf("after / buffer = %q, want %q", got, "go build")
	}
	mustHandleRunes(t, eng, "n")
	if got := ed.BufferString(); got != "go test" {
		t.Fatalf("after n buffer = %q, want %q", got, "go test")
	}
	mustHandleRunes(t, eng, "N")
	if got := ed.BufferString(); got != "go build" {
		t.Fatalf("after N buffer = %q, want %q", got, "go build")
	}
}
