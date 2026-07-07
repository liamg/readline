package vi

import (
	"testing"

	"github.com/liamg/readline/pkg/engine"
	"github.com/liamg/readline/pkg/terminal"
)

func TestVi_VisualMode_EnterAccepts(t *testing.T) {
	_, eng := visualEngine(t)
	done, _, err := eng.HandleKeyEvent(namedKey(terminal.KeyEnter))
	if err != nil || !done {
		t.Fatalf("enter in visual mode: done=%v err=%v", done, err)
	}
}

func TestVi_VisualMode_EscapeSwitchesToNormal(t *testing.T) {
	_, eng := visualEngine(t)
	eng.HandleKeyEvent(namedKey(terminal.KeyEscape))
	if eng.Cursor() != engine.CursorBlock {
		t.Fatalf("after escape cursor = %v, want CursorBlock", eng.Cursor())
	}
}

func TestVi_VisualMode_FallbackIgnoresKeys(t *testing.T) {
	ed, eng := visualEngine(t)
	eng.HandleKeyEvent(key('z'))
	if got := ed.BufferString(); got != "" {
		t.Fatalf("visual fallback inserted %q into buffer, want empty", got)
	}
}

func TestVi_VisualMode_WordMotionsExtendSelection(t *testing.T) {
	tests := []struct {
		name       string
		buffer     string
		anchor     int
		cursor     int
		keys       string
		wantCursor int
	}{
		{name: "w", buffer: "one two", anchor: 0, cursor: 0, keys: "w", wantCursor: 4},
		{name: "b", buffer: "one two", anchor: 4, cursor: 4, keys: "b", wantCursor: 0},
		{name: "e", buffer: "one two", anchor: 0, cursor: 0, keys: "e", wantCursor: 2},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ed, eng, _ := visualSelectionEngine(t, tt.buffer, tt.anchor, tt.cursor)
			mustHandleRunes(t, eng, tt.keys)
			if got := ed.Cursor(); got != tt.wantCursor {
				t.Fatalf("cursor = %d, want %d", got, tt.wantCursor)
			}
			if ed.Selection() == nil {
				t.Fatal("expected selection to remain active")
			}
		})
	}
}

func TestVi_VisualMode_WORDMotionsExtendSelection(t *testing.T) {
	tests := []struct {
		name       string
		buffer     string
		anchor     int
		cursor     int
		keys       string
		wantCursor int
	}{
		{name: "W", buffer: "one,two three", anchor: 0, cursor: 0, keys: "W", wantCursor: 8},
		{name: "B", buffer: "one,two three", anchor: 8, cursor: 8, keys: "B", wantCursor: 0},
		{name: "E", buffer: "one,two three", anchor: 0, cursor: 0, keys: "E", wantCursor: 6},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ed, eng, _ := visualSelectionEngine(t, tt.buffer, tt.anchor, tt.cursor)
			mustHandleRunes(t, eng, tt.keys)
			if got := ed.Cursor(); got != tt.wantCursor {
				t.Fatalf("cursor = %d, want %d", got, tt.wantCursor)
			}
			if ed.Selection() == nil {
				t.Fatal("expected selection to remain active")
			}
		})
	}
}

func TestVi_VisualMode_LineMotionsExtendSelection(t *testing.T) {
	tests := []struct {
		name       string
		keys       string
		wantCursor int
	}{
		{name: "0", keys: "0", wantCursor: 0},
		{name: "^", keys: "^", wantCursor: 2},
		{name: "$", keys: "$", wantCursor: len([]rune("  hello world")) - 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ed, eng, _ := visualSelectionEngine(t, "  hello world", 8, 8)
			mustHandleRunes(t, eng, tt.keys)
			if got := ed.Cursor(); got != tt.wantCursor {
				t.Fatalf("cursor = %d, want %d", got, tt.wantCursor)
			}
		})
	}
}

func TestVi_VisualMode_CharacterFindExtendsSelection(t *testing.T) {
	tests := []struct {
		name       string
		buffer     string
		cursor     int
		keys       string
		wantCursor int
	}{
		{name: "f", buffer: "abc def ghi", cursor: 0, keys: "fd", wantCursor: 4},
		{name: "F", buffer: "abc def ghi", cursor: 8, keys: "Fd", wantCursor: 4},
		{name: "t", buffer: "abc def ghi", cursor: 0, keys: "td", wantCursor: 3},
		{name: "T", buffer: "abc def ghi", cursor: 8, keys: "Td", wantCursor: 5},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ed, eng, _ := visualSelectionEngine(t, tt.buffer, tt.cursor, tt.cursor)
			mustHandleRunes(t, eng, tt.keys)
			if got := ed.Cursor(); got != tt.wantCursor {
				t.Fatalf("cursor = %d, want %d", got, tt.wantCursor)
			}
		})
	}
}

func TestVi_VisualMode_yYanksSelectionAndReturnsToNormal(t *testing.T) {
	ed, eng, v := visualSelectionEngine(t, "hello", 1, 3)
	mustHandleRunes(t, eng, `"ay`)
	if got := string(v.state.registers['a']); got != "ell" {
		t.Fatalf("register a = %q, want %q", got, "ell")
	}
	if ed.Selection() != nil {
		t.Fatal("expected selection to be cleared after yank")
	}
}

func TestVi_VisualMode_DeleteSelectionCommands(t *testing.T) {
	tests := []struct {
		name string
		keys string
	}{
		{name: "d", keys: "d"},
		{name: "x", keys: "x"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ed, eng, _ := visualSelectionEngine(t, "hello", 1, 3)
			mustHandleRunes(t, eng, tt.keys)
			if got := ed.BufferString(); got != "ho" {
				t.Fatalf("buffer = %q, want %q", got, "ho")
			}
			if ed.Selection() != nil {
				t.Fatal("expected selection to be cleared")
			}
		})
	}
}

func TestVi_VisualMode_cChangesSelectionAndEntersInsert(t *testing.T) {
	ed, eng, _ := visualSelectionEngine(t, "hello", 1, 3)
	mustHandleRunes(t, eng, "c")
	if got := eng.Cursor(); got != engine.CursorBar {
		t.Fatalf("cursor style = %v, want CursorBar", got)
	}
	mustHandleRunes(t, eng, "X")
	if got := ed.BufferString(); got != "hXo" {
		t.Fatalf("buffer = %q, want %q", got, "hXo")
	}
	if ed.Selection() != nil {
		t.Fatal("expected selection to be cleared")
	}
}

func TestVi_VisualMode_TildeTogglesSelectionCase(t *testing.T) {
	ed, eng, _ := visualSelectionEngine(t, "hEllO", 1, 3)
	mustHandleRunes(t, eng, "~")
	if got := ed.BufferString(); got != "heLLO" {
		t.Fatalf("buffer = %q, want %q", got, "heLLO")
	}
	if ed.Selection() != nil {
		t.Fatal("expected selection to be cleared")
	}
}
