package ansi

import "testing"

func TestCellWidth_PlainAndWide(t *testing.T) {
	tests := []struct {
		in   string
		want int
	}{
		{"", 0},
		{"hello", 5},
		{"中文", 4},   // two wide CJK chars
		{"a中b", 4},  // mixed
		{"café", 4}, // combining-free accented
		{"é", 1},   // e + combining acute accent -> single cell
	}
	for _, tt := range tests {
		if got := CellWidth(tt.in); got != tt.want {
			t.Errorf("CellWidth(%q) = %d, want %d", tt.in, got, tt.want)
		}
	}
}

// CellWidth must measure multi-rune grapheme clusters as a single unit, unlike
// summing RuneWidth over each rune (which over-counts emoji sequences).
func TestCellWidth_GraphemeClusters(t *testing.T) {
	clusters := []string{
		"👨‍👩‍👧‍👦", // ZWJ family sequence (many runes)
		"🇬🇧",      // regional-indicator flag pair
		"👍🏽",      // emoji + skin-tone modifier
	}
	for _, c := range clusters {
		got := CellWidth(c)
		if got != 2 {
			t.Errorf("CellWidth(%q) = %d, want 2 (grapheme cluster)", c, got)
		}
		// Demonstrate the difference from naive per-rune summing.
		naive := 0
		for _, r := range c {
			naive += RuneWidth(r)
		}
		if naive <= 2 {
			t.Errorf("expected per-rune sum for %q to over-count (got %d)", c, naive)
		}
	}
}

func TestVisibleWidth_GraphemeClusterWithANSI(t *testing.T) {
	// A styled flag emoji: escapes stripped, cluster measured as 2 columns.
	s := "\x1b[31m🇬🇧\x1b[0m"
	if got := VisibleWidth(s); got != 2 {
		t.Errorf("VisibleWidth(%q) = %d, want 2", s, got)
	}
}

func TestRuneWidth_ASCII(t *testing.T) {
	for _, r := range "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789" {
		if w := RuneWidth(r); w != 1 {
			t.Errorf("RuneWidth(%q) = %d, want 1", r, w)
		}
	}
}

func TestRuneWidth_WideChars(t *testing.T) {
	wide := []rune{
		'\u4e2d', // 中 CJK
		'\u6587', // 文 CJK
		'\uff01', // ！ fullwidth
		'\u3042', // あ hiragana
	}
	for _, r := range wide {
		if w := RuneWidth(r); w != 2 {
			t.Errorf("RuneWidth(%q) = %d, want 2", r, w)
		}
	}
}

func TestRuneWidth_ControlChars(t *testing.T) {
	controls := []rune{
		0x00, // NUL
		0x08, // backspace
		0x1b, // escape
		0x7f, // DEL
		0x90, // control range 0x7f–0x9f
	}
	for _, r := range controls {
		if w := RuneWidth(r); w != 0 {
			t.Errorf("RuneWidth(0x%02x) = %d, want 0", r, w)
		}
	}
}

func TestVisibleWidth_PlainText(t *testing.T) {
	if w := VisibleWidth("hello"); w != 5 {
		t.Fatalf("VisibleWidth(%q) = %d, want 5", "hello", w)
	}
}

func TestVisibleWidth_StripsANSI(t *testing.T) {
	s := "\x1b[1mhi\x1b[0m"
	if w := VisibleWidth(s); w != 2 {
		t.Fatalf("VisibleWidth(%q) = %d, want 2", s, w)
	}
}

func TestVisibleWidth_WideChars(t *testing.T) {
	s := "\u4e2d\u6587" // two wide chars = 4 columns
	if w := VisibleWidth(s); w != 4 {
		t.Fatalf("VisibleWidth(%q) = %d, want 4", s, w)
	}
}

func TestVisibleWidth_Mixed(t *testing.T) {
	// ANSI bold + wide char + ASCII
	s := "\x1b[1m\u4e2d\x1b[0mA"
	if w := VisibleWidth(s); w != 3 {
		t.Fatalf("VisibleWidth(%q) = %d, want 3", s, w)
	}
}

func TestVisibleWidth_Empty(t *testing.T) {
	if w := VisibleWidth(""); w != 0 {
		t.Fatalf("VisibleWidth(\"\") = %d, want 0", w)
	}
}

func TestVisibleWidth_NonSGREscape(t *testing.T) {
	// Cursor movement escape should be stripped.
	s := "\x1b[2Ahi"
	if w := VisibleWidth(s); w != 2 {
		t.Fatalf("VisibleWidth(%q) = %d, want 2", s, w)
	}
}
