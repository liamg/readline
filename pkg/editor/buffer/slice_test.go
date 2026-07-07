package buffer

import (
	"testing"
)

func newFilled(s string) *SliceBuffer {
	b := NewSlice()
	b.Insert(0, []rune(s)...)
	return b
}

func TestSliceBuffer_Len(t *testing.T) {
	b := NewSlice()
	if b.Len() != 0 {
		t.Fatalf("empty buffer Len = %d, want 0", b.Len())
	}
	b.Insert(0, 'a', 'b', 'c')
	if b.Len() != 3 {
		t.Fatalf("Len = %d, want 3", b.Len())
	}
}

func TestSliceBuffer_InsertSingle(t *testing.T) {
	b := NewSlice()
	b.Insert(0, 'a')
	b.Insert(1, 'b')
	b.Insert(2, 'c')
	got := string(b.Slice(0, b.Len()))
	if got != "abc" {
		t.Fatalf("got %q, want %q", got, "abc")
	}
}

func TestSliceBuffer_InsertAtStart(t *testing.T) {
	b := newFilled("bc")
	b.Insert(0, 'a')
	got := string(b.Slice(0, b.Len()))
	if got != "abc" {
		t.Fatalf("got %q, want %q", got, "abc")
	}
}

func TestSliceBuffer_InsertInMiddle(t *testing.T) {
	b := newFilled("ac")
	b.Insert(1, 'b')
	got := string(b.Slice(0, b.Len()))
	if got != "abc" {
		t.Fatalf("got %q, want %q", got, "abc")
	}
}

func TestSliceBuffer_InsertMultipleRunes(t *testing.T) {
	b := NewSlice()
	b.Insert(0, 'a', 'b', 'c')
	got := string(b.Slice(0, b.Len()))
	if got != "abc" {
		t.Fatalf("got %q, want %q", got, "abc")
	}
}

func TestSliceBuffer_RuneAt(t *testing.T) {
	b := newFilled("hello")
	tests := []struct {
		i    int
		want rune
	}{
		{0, 'h'},
		{1, 'e'},
		{4, 'o'},
	}
	for _, tt := range tests {
		got := b.RuneAt(tt.i)
		if got != tt.want {
			t.Errorf("RuneAt(%d) = %q, want %q", tt.i, got, tt.want)
		}
	}
}

func TestSliceBuffer_Slice(t *testing.T) {
	b := newFilled("hello")
	got := string(b.Slice(1, 4))
	if got != "ell" {
		t.Fatalf("Slice(1,4) = %q, want %q", got, "ell")
	}
}

func TestSliceBuffer_SliceFullRange(t *testing.T) {
	b := newFilled("hi")
	got := string(b.Slice(0, b.Len()))
	if got != "hi" {
		t.Fatalf("Slice(0,Len) = %q, want %q", got, "hi")
	}
}

func TestSliceBuffer_SliceIsACopy(t *testing.T) {
	b := newFilled("abc")
	s := b.Slice(0, 3)
	s[0] = 'z'
	if b.RuneAt(0) != 'a' {
		t.Fatal("Slice returned a reference into internal storage, not a copy")
	}
}

func TestSliceBuffer_DeleteFromMiddle(t *testing.T) {
	b := newFilled("abcd")
	b.Delete(1, 2) // remove "bc"
	got := string(b.Slice(0, b.Len()))
	if got != "ad" {
		t.Fatalf("got %q, want %q", got, "ad")
	}
}

func TestSliceBuffer_DeleteFromStart(t *testing.T) {
	b := newFilled("abc")
	b.Delete(0, 1)
	got := string(b.Slice(0, b.Len()))
	if got != "bc" {
		t.Fatalf("got %q, want %q", got, "bc")
	}
}

func TestSliceBuffer_DeleteFromEnd(t *testing.T) {
	b := newFilled("abc")
	b.Delete(2, 1)
	got := string(b.Slice(0, b.Len()))
	if got != "ab" {
		t.Fatalf("got %q, want %q", got, "ab")
	}
}

func TestSliceBuffer_DeleteAll(t *testing.T) {
	b := newFilled("abc")
	b.Delete(0, 3)
	if b.Len() != 0 {
		t.Fatalf("Len after delete all = %d, want 0", b.Len())
	}
}
