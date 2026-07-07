package terminal

import (
	"errors"
	"io"
	"syscall"
	"testing"
)

type flakyWriter struct {
	chunks [][]byte
	calls  int
}

func (w *flakyWriter) Write(p []byte) (int, error) {
	w.calls++
	switch w.calls {
	case 1:
		n := min(3, len(p))
		w.chunks = append(w.chunks, append([]byte(nil), p[:n]...))
		return n, nil
	case 2:
		return 0, syscall.EAGAIN
	default:
		w.chunks = append(w.chunks, append([]byte(nil), p...))
		return len(p), nil
	}
}

func TestWriteAllRetriesEAGAINAndHandlesPartialWrites(t *testing.T) {
	writer := &flakyWriter{}
	n, err := writeAll(writer, []byte("abcdef"))
	if err != nil {
		t.Fatalf("writeAll error: %v", err)
	}
	if n != 6 {
		t.Fatalf("written = %d, want 6", n)
	}
	if writer.calls != 3 {
		t.Fatalf("calls = %d, want 3", writer.calls)
	}
	got := string(writer.chunks[0]) + string(writer.chunks[1])
	if got != "abcdef" {
		t.Fatalf("written chunks = %q, want abcdef", got)
	}
}

type errWriter struct{}

func (errWriter) Write([]byte) (int, error) {
	return 0, errors.New("boom")
}

func TestWriteAllReturnsPermanentError(t *testing.T) {
	if _, err := writeAll(errWriter{}, []byte("abc")); err == nil {
		t.Fatal("expected error")
	}
}

// --- parseEvent ---

func TestParseEvent_PrintableRune(t *testing.T) {
	ev, n, err := parseEvent([]byte("a"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	ke := ev.(KeyEvent)
	if ke.Key != KeyRune || ke.Rune != 'a' || ke.Mod != 0 {
		t.Fatalf("got %+v, want KeyRune 'a'", ke)
	}
	if n != 1 {
		t.Fatalf("consumed %d bytes, want 1", n)
	}
}

func TestParseEvent_UTF8Rune(t *testing.T) {
	b := []byte("é") // 2-byte UTF-8
	ev, n, err := parseEvent(b)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	ke := ev.(KeyEvent)
	if ke.Rune != 'é' {
		t.Fatalf("got rune %q, want 'é'", ke.Rune)
	}
	if n != len(b) {
		t.Fatalf("consumed %d bytes, want %d", n, len(b))
	}
}

func TestParseEvent_CtrlD_EOF(t *testing.T) {
	_, _, err := parseEvent([]byte{0x04})
	if err != io.EOF {
		t.Fatalf("expected io.EOF, got %v", err)
	}
}

func TestParseEvent_DEL_Backspace(t *testing.T) {
	ev, n, err := parseEvent([]byte{0x7f})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	ke := ev.(KeyEvent)
	if ke.Key != KeyBackspace {
		t.Fatalf("got %+v, want KeyBackspace", ke)
	}
	if n != 1 {
		t.Fatalf("consumed %d bytes, want 1", n)
	}
}

func TestParseEvent_Enter(t *testing.T) {
	for _, b := range []byte{0x0a, 0x0d} {
		ev, _, err := parseEvent([]byte{b})
		if err != nil {
			t.Fatalf("byte 0x%02x: unexpected error: %v", b, err)
		}
		ke := ev.(KeyEvent)
		if ke.Key != KeyEnter {
			t.Fatalf("byte 0x%02x: got %+v, want KeyEnter", b, ke)
		}
	}
}

func TestParseEvent_Escape_Bare(t *testing.T) {
	ev, n, err := parseEvent([]byte{0x1b})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	ke := ev.(KeyEvent)
	if ke.Key != KeyEscape {
		t.Fatalf("got %+v, want KeyEscape", ke)
	}
	if n != 1 {
		t.Fatalf("consumed %d bytes, want 1", n)
	}
}

func TestParseEvent_CtrlC(t *testing.T) {
	_, _, err := parseEvent([]byte{0x03})
	if err == nil {
		t.Fatalf("expected error 'interrupted', got: %v", err)
	}
	if err.Error() != "interrupted" {
		t.Fatalf("expected error 'interrupted', got: %v", err)
	}
}

func TestParseEvent_AltKey(t *testing.T) {
	// ESC + 'b' → Alt-b
	ev, n, err := parseEvent([]byte{0x1b, 'b'})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	ke := ev.(KeyEvent)
	if ke.Rune != 'b' || ke.Mod&ModAlt == 0 {
		t.Fatalf("got %+v, want alt-b", ke)
	}
	if n != 2 {
		t.Fatalf("consumed %d bytes, want 2", n)
	}
}

func TestParseEvent_EmptyInput(t *testing.T) {
	_, _, err := parseEvent([]byte{})
	if err != io.EOF {
		t.Fatalf("expected io.EOF on empty input, got %v", err)
	}
}

// --- parseCsi ---

func TestParseCsi_ArrowKeys(t *testing.T) {
	tests := []struct {
		seq  []byte
		want Key
	}{
		{[]byte("A"), KeyUp},
		{[]byte("B"), KeyDown},
		{[]byte("C"), KeyRight},
		{[]byte("D"), KeyLeft},
	}
	for _, tt := range tests {
		ev, _, err := parseCsi(tt.seq)
		if err != nil {
			t.Fatalf("parseCsi(%q): unexpected error: %v", tt.seq, err)
		}
		ke := ev.(KeyEvent)
		if ke.Key != tt.want {
			t.Errorf("parseCsi(%q) key = %v, want %v", tt.seq, ke.Key, tt.want)
		}
	}
}

func TestParseCsi_FunctionKeys(t *testing.T) {
	tests := []struct {
		seq  []byte
		want Key
	}{
		{[]byte("11~"), KeyF1},
		{[]byte("12~"), KeyF2},
		{[]byte("15~"), KeyF5},
		{[]byte("24~"), KeyF12},
	}
	for _, tt := range tests {
		ev, _, err := parseCsi(tt.seq)
		if err != nil {
			t.Fatalf("parseCsi(%q): %v", tt.seq, err)
		}
		ke := ev.(KeyEvent)
		if ke.Key != tt.want {
			t.Errorf("parseCsi(%q) = %v, want %v", tt.seq, ke.Key, tt.want)
		}
	}
}

func TestParseCsi_PageKeys(t *testing.T) {
	tests := []struct {
		seq  []byte
		want Key
	}{
		{[]byte("5~"), KeyPageUp},
		{[]byte("6~"), KeyPageDown},
		{[]byte("2~"), KeyInsert},
		{[]byte("3~"), KeyDelete},
	}
	for _, tt := range tests {
		ev, _, err := parseCsi(tt.seq)
		if err != nil {
			t.Fatalf("parseCsi(%q): %v", tt.seq, err)
		}
		ke := ev.(KeyEvent)
		if ke.Key != tt.want {
			t.Errorf("parseCsi(%q) = %v, want %v", tt.seq, ke.Key, tt.want)
		}
	}
}

func TestParseCsi_WithModifier(t *testing.T) {
	// ESC[1;5A = Ctrl+Up
	ev, _, err := parseCsi([]byte("1;5A"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	ke := ev.(KeyEvent)
	if ke.Key != KeyUp {
		t.Fatalf("key = %v, want KeyUp", ke.Key)
	}
	if ke.Mod&ModCtrl == 0 {
		t.Fatalf("modifier = %v, want ModCtrl set", ke.Mod)
	}
}

func TestParseCsi_ShiftTab(t *testing.T) {
	ev, _, err := parseCsi([]byte("Z"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	ke := ev.(KeyEvent)
	if ke.Key != KeyTab || ke.Mod&ModShift == 0 {
		t.Fatalf("got %+v, want shift-tab", ke)
	}
}

// --- parseSs3 ---

func TestParseSs3_ArrowAndFunction(t *testing.T) {
	tests := []struct {
		b    byte
		want Key
	}{
		{'A', KeyUp},
		{'B', KeyDown},
		{'C', KeyRight},
		{'D', KeyLeft},
		{'H', KeyHome},
		{'F', KeyEnd},
		{'P', KeyF1},
		{'Q', KeyF2},
		{'R', KeyF3},
		{'S', KeyF4},
	}
	for _, tt := range tests {
		ev, n, err := parseSs3([]byte{tt.b})
		if err != nil {
			t.Fatalf("parseSs3(%q): %v", tt.b, err)
		}
		ke := ev.(KeyEvent)
		if ke.Key != tt.want {
			t.Errorf("parseSs3(%q) = %v, want %v", tt.b, ke.Key, tt.want)
		}
		if n != 1 {
			t.Errorf("parseSs3(%q) consumed %d, want 1", tt.b, n)
		}
	}
}

func TestParseSs3_Unknown(t *testing.T) {
	ev, _, _ := parseSs3([]byte{'X'})
	ke := ev.(KeyEvent)
	if ke.Key != KeyEscape {
		t.Fatalf("unknown SS3 byte should produce KeyEscape, got %v", ke.Key)
	}
}

// --- isIncompleteEscape ---

func TestIsIncompleteEscape_Empty(t *testing.T) {
	if isIncompleteEscape([]byte{}) {
		t.Fatal("empty slice should not be incomplete escape")
	}
}

func TestIsIncompleteEscape_BareEsc(t *testing.T) {
	if !isIncompleteEscape([]byte{0x1b}) {
		t.Fatal("bare ESC should be incomplete")
	}
}

func TestIsIncompleteEscape_CsiNoTerminator(t *testing.T) {
	// ESC [ 1 — no terminating byte yet
	if !isIncompleteEscape([]byte{0x1b, '[', '1'}) {
		t.Fatal("CSI without terminator should be incomplete")
	}
}

func TestIsIncompleteEscape_CsiComplete(t *testing.T) {
	// ESC [ A — 'A' is the terminator
	if isIncompleteEscape([]byte{0x1b, '[', 'A'}) {
		t.Fatal("complete CSI should not be incomplete")
	}
}

func TestIsIncompleteEscape_Ss3Incomplete(t *testing.T) {
	if !isIncompleteEscape([]byte{0x1b, 'O'}) {
		t.Fatal("SS3 with no final byte should be incomplete")
	}
}

func TestIsIncompleteEscape_Ss3Complete(t *testing.T) {
	if isIncompleteEscape([]byte{0x1b, 'O', 'P'}) {
		t.Fatal("complete SS3 should not be incomplete")
	}
}

func TestIsIncompleteEscape_PlainByte(t *testing.T) {
	if isIncompleteEscape([]byte{'a'}) {
		t.Fatal("plain byte should not be an escape")
	}
}

// --- parseControlKey ---

func TestParseControlKey_CommonKeys(t *testing.T) {
	tests := []struct {
		b        byte
		wantKey  Key
		wantMod  Modifier
		wantRune rune
	}{
		{0x08, KeyBackspace, 0, 0},
		{0x09, KeyTab, 0, 0},
		{0x0d, KeyEnter, 0, 0},
	}
	for _, tt := range tests {
		ev, n := parseControlKey(tt.b)
		if n != 1 {
			t.Errorf("0x%02x: consumed %d bytes, want 1", tt.b, n)
		}
		if tt.wantKey != 0 && ev.Key != tt.wantKey {
			t.Errorf("0x%02x: key = %v, want %v", tt.b, ev.Key, tt.wantKey)
		}
	}
}

func TestParseControlKey_CtrlLetters(t *testing.T) {
	tests := []struct {
		b    byte
		rune rune
	}{
		{0x01, 'a'},
		{0x03, 'c'},
		{0x15, 'u'},
		{0x18, 'x'},
	}
	for _, tt := range tests {
		ev, _ := parseControlKey(tt.b)
		if ev.Rune != tt.rune || ev.Mod != ModCtrl {
			t.Errorf("0x%02x: got %+v, want ctrl-%c", tt.b, ev, tt.rune)
		}
	}
}
