package keymap

import (
	"testing"

	"github.com/liamg/readline/pkg/terminal"
)

func TestParseSequence(t *testing.T) {
	tests := []struct {
		input string
		want  Sequence
		err   bool
	}{
		// Single printable rune
		{
			input: "a",
			want:  Sequence{{Key: terminal.KeyRune, Rune: 'a'}},
		},
		// Unicode rune
		{
			input: "é",
			want:  Sequence{{Key: terminal.KeyRune, Rune: 'é'}},
		},
		// space alias
		{
			input: "space",
			want:  Sequence{{Key: terminal.KeyRune, Rune: ' '}},
		},
		// ctrl+letter: Key=KeyRune to match driver output
		{
			input: "ctrl-c",
			want:  Sequence{{Key: terminal.KeyRune, Rune: 'c', Mod: terminal.ModCtrl}},
		},
		{
			input: "ctrl-x",
			want:  Sequence{{Key: terminal.KeyRune, Rune: 'x', Mod: terminal.ModCtrl}},
		},
		// alt+letter: Key=KeyRune to match driver output
		{
			input: "alt-a",
			want:  Sequence{{Key: terminal.KeyRune, Rune: 'a', Mod: terminal.ModAlt}},
		},
		// Named keys
		{
			input: "up",
			want:  Sequence{{Key: terminal.KeyUp}},
		},
		{
			input: "backspace",
			want:  Sequence{{Key: terminal.KeyBackspace}},
		},
		{
			input: "enter",
			want:  Sequence{{Key: terminal.KeyEnter}},
		},
		{
			input: "tab",
			want:  Sequence{{Key: terminal.KeyTab}},
		},
		{
			input: "escape",
			want:  Sequence{{Key: terminal.KeyEscape}},
		},
		{
			input: "esc",
			want:  Sequence{{Key: terminal.KeyEscape}},
		},
		{
			input: "f1",
			want:  Sequence{{Key: terminal.KeyF1}},
		},
		{
			input: "f12",
			want:  Sequence{{Key: terminal.KeyF12}},
		},
		{
			input: "pageup",
			want:  Sequence{{Key: terminal.KeyPageUp}},
		},
		{
			input: "page-down",
			want:  Sequence{{Key: terminal.KeyPageDown}},
		},
		// Modifier + named key
		{
			input: "alt-left",
			want:  Sequence{{Key: terminal.KeyLeft, Mod: terminal.ModAlt}},
		},
		{
			input: "shift-f3",
			want:  Sequence{{Key: terminal.KeyF3, Mod: terminal.ModShift}},
		},
		{
			input: "ctrl-delete",
			want:  Sequence{{Key: terminal.KeyDelete, Mod: terminal.ModCtrl}},
		},
		// Stacked modifiers
		{
			input: "ctrl-shift-left",
			want:  Sequence{{Key: terminal.KeyLeft, Mod: terminal.ModCtrl | terminal.ModShift}},
		},
		// Case insensitivity
		{
			input: "CTRL-C",
			want:  Sequence{{Key: terminal.KeyRune, Rune: 'C', Mod: terminal.ModCtrl}},
		},
		{
			input: "Alt-Left",
			want:  Sequence{{Key: terminal.KeyLeft, Mod: terminal.ModAlt}},
		},
		// Multi-key chord (sequence)
		{
			input: "ctrl-x,u",
			want: Sequence{
				{Key: terminal.KeyRune, Rune: 'x', Mod: terminal.ModCtrl},
				{Key: terminal.KeyRune, Rune: 'u'},
			},
		},
		{
			input: "ctrl-x,ctrl-u",
			want: Sequence{
				{Key: terminal.KeyRune, Rune: 'x', Mod: terminal.ModCtrl},
				{Key: terminal.KeyRune, Rune: 'u', Mod: terminal.ModCtrl},
			},
		},
		// Spaces around commas are trimmed
		{
			input: "ctrl-x, u",
			want: Sequence{
				{Key: terminal.KeyRune, Rune: 'x', Mod: terminal.ModCtrl},
				{Key: terminal.KeyRune, Rune: 'u'},
			},
		},
		// Errors
		{input: "ctrl-", err: true},
		{input: "unknownkey", err: true},
		{input: "ctrl-x,,u", err: true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, err := ParseSequence(tt.input)
			if tt.err {
				if err == nil {
					t.Errorf("ParseSequence(%q) = %v, want error", tt.input, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("ParseSequence(%q) error: %v", tt.input, err)
			}
			if len(got) != len(tt.want) {
				t.Fatalf("ParseSequence(%q) len=%d, want %d: got %v", tt.input, len(got), len(tt.want), got)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("ParseSequence(%q)[%d] = %+v, want %+v", tt.input, i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestSequence_Matches_Empty(t *testing.T) {
	seq := MustParseSequence("a")
	match, complete := seq.Matches(nil)
	if !match || complete {
		t.Fatalf("Matches(nil) = (%v,%v), want (true,false)", match, complete)
	}
}

func TestSequence_Matches_Partial(t *testing.T) {
	seq := MustParseSequence("ctrl-x,u")
	events := []terminal.KeyEvent{{Key: terminal.KeyRune, Rune: 'x', Mod: terminal.ModCtrl}}
	match, complete := seq.Matches(events)
	if !match || complete {
		t.Fatalf("Matches(partial) = (%v,%v), want (true,false)", match, complete)
	}
}

func TestSequence_Matches_Complete(t *testing.T) {
	seq := MustParseSequence("ctrl-x,u")
	events := []terminal.KeyEvent{
		{Key: terminal.KeyRune, Rune: 'x', Mod: terminal.ModCtrl},
		{Key: terminal.KeyRune, Rune: 'u'},
	}
	match, complete := seq.Matches(events)
	if !match || !complete {
		t.Fatalf("Matches(complete) = (%v,%v), want (true,true)", match, complete)
	}
}

func TestSequence_Matches_NoMatch(t *testing.T) {
	seq := MustParseSequence("a")
	events := []terminal.KeyEvent{{Key: terminal.KeyRune, Rune: 'z'}}
	match, complete := seq.Matches(events)
	if match || complete {
		t.Fatalf("Matches(wrong key) = (%v,%v), want (false,false)", match, complete)
	}
}

func TestSequence_Equal(t *testing.T) {
	seq := MustParseSequence("ctrl-x,u")
	if !seq.Equal(MustParseSequence("ctrl-x,u")) {
		t.Fatal("matching sequences should be equal")
	}
	if seq.Equal(MustParseSequence("ctrl-x,ctrl-u")) {
		t.Fatal("different key events should not be equal")
	}
	if seq.Equal(MustParseSequence("ctrl-x")) {
		t.Fatal("different length sequences should not be equal")
	}
}

func TestMustParseSequence_Valid(t *testing.T) {
	seq := MustParseSequence("ctrl-c")
	if len(seq) != 1 {
		t.Fatalf("len = %d, want 1", len(seq))
	}
}

func TestMustParseSequence_Panics(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("expected panic for invalid sequence")
		}
	}()
	MustParseSequence("not-a-real-key-xyz")
}

func TestSequence_String_SingleKey(t *testing.T) {
	seq := MustParseSequence("a")
	if got := seq.String(); got != "a" {
		t.Fatalf("String() = %q, want %q", got, "a")
	}
}

func TestSequence_String_CtrlKey(t *testing.T) {
	seq := MustParseSequence("ctrl-c")
	if got := seq.String(); got != "ctrl-c" {
		t.Fatalf("String() = %q, want %q", got, "ctrl-c")
	}
}

func TestSequence_String_MultiKey(t *testing.T) {
	seq := MustParseSequence("ctrl-x,u")
	if got := seq.String(); got != "ctrl-x,u" {
		t.Fatalf("String() = %q, want %q", got, "ctrl-x,u")
	}
}

func TestSequence_String_NamedKey(t *testing.T) {
	seq := MustParseSequence("enter")
	got := seq.String()
	if got == "" {
		t.Fatalf("String() for named key is empty")
	}
}

func TestSequence_String_Space(t *testing.T) {
	seq := MustParseSequence("space")
	if got := seq.String(); got != "space" {
		t.Fatalf("String() = %q, want %q", got, "space")
	}
}

func TestSequence_String_AltModifier(t *testing.T) {
	seq := MustParseSequence("alt-a")
	if got := seq.String(); got != "alt-a" {
		t.Fatalf("String() = %q, want %q", got, "alt-a")
	}
}
