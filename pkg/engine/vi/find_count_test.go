package vi

import "testing"

// f/t/F/T motions must respect a count, both when supplied to an operator
// (d2fx) and when used standalone to move the cursor (2fx).

func TestVi_FindCharMotion_OperatorRespectsCount(t *testing.T) {
	tests := []struct {
		name   string
		buffer string
		cursor int
		keys   string
		want   string
	}{
		// buffer: f0 o1 o2 _3 b4 o5 o6 _7 z8 o9 o10
		// d2fo deletes from the cursor through the 2nd 'o' inclusive.
		{"d2fo", "foo boo zoo", 0, "d2fo", " boo zoo"},
		// dfo is the plain single-count case: deletes "fo" (through the 1st 'o').
		{"dfo", "foo boo zoo", 0, "dfo", "o boo zoo"},
		// d2to stops just before the 2nd 'o' (till).
		{"d2to", "foo boo zoo", 0, "d2to", "o boo zoo"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ed, eng, _ := normalEngine(t, tt.buffer)
			moveCursorTo(ed, tt.cursor)
			mustHandleRunes(t, eng, tt.keys)
			if got := ed.BufferString(); got != tt.want {
				t.Fatalf("%s: buffer = %q, want %q", tt.keys, got, tt.want)
			}
		})
	}
}

func TestVi_FindCharMotion_StandaloneRespectsCount(t *testing.T) {
	// 2fo moves the cursor to the 2nd 'o' (index 2); x then deletes it.
	ed, eng, _ := normalEngine(t, "foo boo zoo")
	moveCursorTo(ed, 0)
	mustHandleRunes(t, eng, "2fo")
	if got := ed.Cursor(); got != 2 {
		t.Fatalf("after 2fo cursor = %d, want 2", got)
	}
	mustHandleRunes(t, eng, "x")
	if got := ed.BufferString(); got != "fo boo zoo" {
		t.Fatalf("after 2fox buffer = %q, want %q", got, "fo boo zoo")
	}
}

func TestVi_FindCharMotion_BackwardRespectsCount(t *testing.T) {
	// From the end, F walking backward: 2Fo lands on the 2nd 'o' scanning left.
	// buffer: f0 o1 o2 _3 b4 o5 o6 _7 z8 o9 o10 ; start at index 10.
	ed, eng, _ := normalEngine(t, "foo boo zoo")
	moveCursorTo(ed, 10)
	mustHandleRunes(t, eng, "2Fo")
	if got := ed.Cursor(); got != 6 {
		t.Fatalf("after 2Fo cursor = %d, want 6", got)
	}
}

func TestVi_NormalMode_CountedLeftRight(t *testing.T) {
	ed, eng, _ := normalEngine(t, "hello world")
	moveCursorTo(ed, 0)
	mustHandleRunes(t, eng, "5l")
	if got := ed.Cursor(); got != 5 {
		t.Fatalf("after 5l cursor = %d, want 5", got)
	}
	mustHandleRunes(t, eng, "3h")
	if got := ed.Cursor(); got != 2 {
		t.Fatalf("after 3h cursor = %d, want 2", got)
	}
}

func TestVi_NormalMode_MultiDigitCount(t *testing.T) {
	ed, eng, _ := normalEngine(t, "abcdefghijklmnop")
	moveCursorTo(ed, 0)
	mustHandleRunes(t, eng, "12l")
	if got := ed.Cursor(); got != 12 {
		t.Fatalf("after 12l cursor = %d, want 12", got)
	}
}

func TestVi_NormalMode_CountWithZeroDigit(t *testing.T) {
	// "10l" must move ten cells; the 0 extends the count rather than jumping to
	// the beginning of the line.
	ed, eng, _ := normalEngine(t, "abcdefghijklmnop")
	moveCursorTo(ed, 0)
	mustHandleRunes(t, eng, "10l")
	if got := ed.Cursor(); got != 10 {
		t.Fatalf("after 10l cursor = %d, want 10", got)
	}
}

func TestVi_NormalMode_ZeroWithoutCountGoesToLineStart(t *testing.T) {
	// A bare 0 (no pending count) still moves to the beginning of the line.
	ed, eng, _ := normalEngine(t, "hello world")
	moveCursorTo(ed, 6)
	mustHandleRunes(t, eng, "0")
	if got := ed.Cursor(); got != 0 {
		t.Fatalf("after 0 cursor = %d, want 0", got)
	}
}

func TestVi_NormalMode_CountResetsAfterMotion(t *testing.T) {
	ed, eng, _ := normalEngine(t, "hello world")
	moveCursorTo(ed, 0)
	mustHandleRunes(t, eng, "5l")
	mustHandleRunes(t, eng, "l") // a bare l must move exactly one
	if got := ed.Cursor(); got != 6 {
		t.Fatalf("after 5l then l cursor = %d, want 6", got)
	}
}

func TestVi_VisualMode_CountedRightExtendsSelection(t *testing.T) {
	ed, eng, _ := normalEngine(t, "hello world")
	moveCursorTo(ed, 0)
	mustHandleRunes(t, eng, "v3l") // select 4 chars: anchor 0, cursor 3
	if got := string(ed.GetSelectedRunes()); got != "hell" {
		t.Fatalf("after v3l selection = %q, want %q", got, "hell")
	}
}

func TestVi_FindCharMotion_CountBeyondMatchesIsNoop(t *testing.T) {
	// Asking for more occurrences than exist should not move or delete.
	ed, eng, _ := normalEngine(t, "foo boo zoo")
	moveCursorTo(ed, 0)
	mustHandleRunes(t, eng, "d9fo")
	if got := ed.BufferString(); got != "foo boo zoo" {
		t.Fatalf("d9fo should be a no-op, buffer = %q", got)
	}
}
