package vi

import "testing"

// These tests drive visual mode through the real `v` keybinding (via
// normalEngine) rather than fabricating a selection with SetSelectionAnchor.
// This is the path a user actually exercises, and it previously panicked
// because entering visual mode never created a selection.

func TestVi_VisualMode_DeleteViaRealBindingNoPanic(t *testing.T) {
	ed, eng, _ := normalEngine(t, "hello world")
	moveCursorTo(ed, 0)
	mustHandleRunes(t, eng, "vd")
	if got := ed.BufferString(); got != "ello world" {
		t.Fatalf("after vd buffer = %q, want %q", got, "ello world")
	}
}

func TestVi_VisualMode_DeleteExtendedSelection(t *testing.T) {
	ed, eng, v := normalEngine(t, "hello")
	moveCursorTo(ed, 0)
	mustHandleRunes(t, eng, "vlld")
	if got := ed.BufferString(); got != "lo" {
		t.Fatalf("after vlld buffer = %q, want %q", got, "lo")
	}
	if got := string(v.state.registers[defaultRegister]); got != "hel" {
		t.Fatalf("deleted register = %q, want %q", got, "hel")
	}
}

func TestVi_VisualMode_YankViaRealBinding(t *testing.T) {
	ed, eng, v := normalEngine(t, "hello")
	moveCursorTo(ed, 0)
	mustHandleRunes(t, eng, "vly")
	if got := ed.BufferString(); got != "hello" {
		t.Fatalf("yank must not modify buffer, got %q", got)
	}
	if got := string(v.state.registers[defaultRegister]); got != "he" {
		t.Fatalf("yanked register = %q, want %q", got, "he")
	}
}

func TestVi_VisualMode_ChangeViaRealBinding(t *testing.T) {
	ed, eng, _ := normalEngine(t, "hello")
	moveCursorTo(ed, 0)
	mustHandleRunes(t, eng, "vlc")
	if got := ed.BufferString(); got != "llo" {
		t.Fatalf("after vlc buffer = %q, want %q", got, "llo")
	}
	// c leaves us in insert mode; typed text replaces the deleted selection.
	mustHandleRunes(t, eng, "XY")
	if got := ed.BufferString(); got != "XYllo" {
		t.Fatalf("after typing buffer = %q, want %q", got, "XYllo")
	}
}

func TestVi_VisualMode_SingleCharDeleteViaRealBinding(t *testing.T) {
	// v with no motion selects exactly the character under the cursor.
	ed, eng, v := normalEngine(t, "abc")
	moveCursorTo(ed, 1)
	mustHandleRunes(t, eng, "vd")
	if got := ed.BufferString(); got != "ac" {
		t.Fatalf("after vd at index 1 buffer = %q, want %q", got, "ac")
	}
	if got := string(v.state.registers[defaultRegister]); got != "b" {
		t.Fatalf("deleted register = %q, want %q", got, "b")
	}
}
