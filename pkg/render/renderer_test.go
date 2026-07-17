package render

import (
	"bytes"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/liamg/readline/pkg/ansi"
	"github.com/liamg/readline/pkg/config"
	"github.com/liamg/readline/pkg/editor"
	"github.com/liamg/readline/pkg/editor/completion"
)

// bufWriter is a simple io.Writer backed by a bytes.Buffer.
type bufWriter struct{ bytes.Buffer }

type positionedBufWriter struct {
	bytes.Buffer
	row int
	col int
	err error
}

func (w *positionedBufWriter) CursorPosition() (int, int, error) {
	return w.row, w.col, w.err
}

// errWriter always returns an error on Write.
type errWriter struct{ err error }

func (e *errWriter) Write(p []byte) (int, error) { return 0, e.err }

// ---- styleAt ---------------------------------------------------------------

func TestStyleAt_NoSpans(t *testing.T) {
	if got := styleAt(0, nil); got != (ansi.Style{}) {
		t.Fatalf("expected zero style, got %+v", got)
	}
}

func TestStyleAt_IndexBeforeSpan(t *testing.T) {
	spans := []ansi.Span{{Start: 5, End: 10, Style: ansi.Style{Attr: ansi.AttrBold}}}
	if got := styleAt(4, spans); got != (ansi.Style{}) {
		t.Fatalf("expected zero style, got %+v", got)
	}
}

func TestStyleAt_IndexInSpan(t *testing.T) {
	want := ansi.Style{Attr: ansi.AttrBold}
	spans := []ansi.Span{{Start: 2, End: 6, Style: want}}
	if got := styleAt(3, spans); got != want {
		t.Fatalf("got %+v, want %+v", got, want)
	}
}

func TestStyleAt_IndexAtSpanEnd_Exclusive(t *testing.T) {
	spans := []ansi.Span{{Start: 2, End: 6, Style: ansi.Style{Attr: ansi.AttrBold}}}
	if got := styleAt(6, spans); got != (ansi.Style{}) {
		t.Fatalf("end is exclusive, expected zero style, got %+v", got)
	}
}

func TestStyleAt_MultipleSpans(t *testing.T) {
	bold := ansi.Style{Attr: ansi.AttrBold}
	dim := ansi.Style{Attr: ansi.AttrDim}
	spans := []ansi.Span{
		{Start: 0, End: 3, Style: bold},
		{Start: 5, End: 8, Style: dim},
	}
	if got := styleAt(1, spans); got != bold {
		t.Fatalf("got %+v, want bold", got)
	}
	if got := styleAt(4, spans); got != (ansi.Style{}) {
		t.Fatalf("gap between spans, got %+v, want zero", got)
	}
	if got := styleAt(6, spans); got != dim {
		t.Fatalf("got %+v, want dim", got)
	}
}

// ---- buildColumns ----------------------------------------------------------

func TestBuildColumns_Empty(t *testing.T) {
	cols := buildColumns(nil)
	if len(cols) != 1 || cols[0] != 0 {
		t.Fatalf("empty cells: got %v, want [0]", cols)
	}
}

func TestBuildColumns_SingleWidth(t *testing.T) {
	cells := []Cell{{Rune: 'a', Width: 1}, {Rune: 'b', Width: 1}, {Rune: 'c', Width: 1}}
	cols := buildColumns(cells)
	want := []int{0, 1, 2, 3}
	if len(cols) != len(want) {
		t.Fatalf("len = %d, want %d", len(cols), len(want))
	}
	for i, v := range want {
		if cols[i] != v {
			t.Fatalf("cols[%d] = %d, want %d", i, cols[i], v)
		}
	}
}

func TestBuildColumns_DoubleWidth(t *testing.T) {
	cells := []Cell{{Rune: '中', Width: 2}, {Rune: 'a', Width: 1}}
	cols := buildColumns(cells)
	want := []int{0, 2, 3}
	for i, v := range want {
		if cols[i] != v {
			t.Fatalf("cols[%d] = %d, want %d", i, cols[i], v)
		}
	}
}

func TestFromSpans_GraphemeClusters(t *testing.T) {
	line := FromSpans([]rune("🐈🐈‍⬛"), nil)
	if got, want := len(line), 2; got != want {
		t.Fatalf("len(line) = %d, want %d", got, want)
	}
	if got, want := line[0].Text, "🐈"; got != want {
		t.Fatalf("line[0].Text = %q, want %q", got, want)
	}
	if got, want := line[0].Width, 2; got != want {
		t.Fatalf("line[0].Width = %d, want %d", got, want)
	}
	if got, want := line[1].Text, "🐈‍⬛"; got != want {
		t.Fatalf("line[1].Text = %q, want %q", got, want)
	}
	if got, want := line[1].Width, 2; got != want {
		t.Fatalf("line[1].Width = %d, want %d", got, want)
	}
}

// ---- colAt -----------------------------------------------------------------

func TestColAt_Empty(t *testing.T) {
	if got := colAt(nil, 0); got != 0 {
		t.Fatalf("colAt(nil,0) = %d, want 0", got)
	}
}

func TestColAt_InRange(t *testing.T) {
	cols := []int{0, 1, 3, 6}
	if got := colAt(cols, 2); got != 3 {
		t.Fatalf("colAt(cols,2) = %d, want 3", got)
	}
}

func TestColAt_OutOfRange(t *testing.T) {
	cols := []int{0, 1, 3, 6}
	if got := colAt(cols, 99); got != 6 {
		t.Fatalf("colAt(cols,99) = %d, want 6", got)
	}
}

// ---- Line.CountRune --------------------------------------------------------

func TestLine_CountRune_Empty(t *testing.T) {
	var l Line
	if l.CountRune('a') != 0 {
		t.Fatal("expected 0 on empty line")
	}
}

func TestLine_CountRune_NoMatch(t *testing.T) {
	l := Line{{Rune: 'b', Width: 1}, {Rune: 'c', Width: 1}}
	if l.CountRune('a') != 0 {
		t.Fatal("expected 0 when rune not present")
	}
}

func TestLine_CountRune_Match(t *testing.T) {
	l := Line{{Rune: 'a', Width: 1}, {Rune: 'b', Width: 1}, {Rune: 'a', Width: 1}}
	if got := l.CountRune('a'); got != 2 {
		t.Fatalf("CountRune = %d, want 2", got)
	}
}

// ---- Line.Width ------------------------------------------------------------

func TestLine_Width_Empty(t *testing.T) {
	var l Line
	if l.Width() != 0 {
		t.Fatal("expected 0 on empty line")
	}
}

func TestLine_Width_SingleWidth(t *testing.T) {
	l := Line{{Rune: 'a', Width: 1}, {Rune: 'b', Width: 1}}
	if l.Width() != 2 {
		t.Fatalf("Width = %d, want 2", l.Width())
	}
}

func TestLine_Width_DoubleWidth(t *testing.T) {
	l := Line{{Rune: '中', Width: 2}, {Rune: 'a', Width: 1}}
	if l.Width() != 3 {
		t.Fatalf("Width = %d, want 3", l.Width())
	}
}

// ---- FromSpans -------------------------------------------------------------

func TestFromSpans_NoSpans(t *testing.T) {
	runes := []rune("abc")
	line := FromSpans(runes, nil)
	if len(line) != 3 {
		t.Fatalf("len = %d, want 3", len(line))
	}
	for i, c := range line {
		if c.Rune != runes[i] {
			t.Errorf("cell[%d].Rune = %q, want %q", i, c.Rune, runes[i])
		}
		if c.Style != (ansi.Style{}) {
			t.Errorf("cell[%d].Style should be zero, got %+v", i, c.Style)
		}
		if c.Width != 1 {
			t.Errorf("cell[%d].Width = %d, want 1", i, c.Width)
		}
	}
}

func TestFromSpans_WithSpan(t *testing.T) {
	runes := []rune("hello")
	bold := ansi.Style{Attr: ansi.AttrBold}
	spans := []ansi.Span{{Start: 1, End: 3, Style: bold}}
	line := FromSpans(runes, spans)
	if line[0].Style != (ansi.Style{}) {
		t.Errorf("cell[0] should have zero style")
	}
	if line[1].Style != bold {
		t.Errorf("cell[1] should be bold, got %+v", line[1].Style)
	}
	if line[2].Style != bold {
		t.Errorf("cell[2] should be bold, got %+v", line[2].Style)
	}
	if line[3].Style != (ansi.Style{}) {
		t.Errorf("cell[3] should have zero style (span end exclusive)")
	}
}

func TestFromSpans_WideRune(t *testing.T) {
	runes := []rune("中")
	line := FromSpans(runes, nil)
	if len(line) != 1 {
		t.Fatalf("len = %d, want 1", len(line))
	}
	if line[0].Width != 2 {
		t.Fatalf("Width = %d, want 2 for wide char", line[0].Width)
	}
}

func TestFromSpans_Empty(t *testing.T) {
	line := FromSpans(nil, nil)
	if len(line) != 0 {
		t.Fatalf("expected empty line from nil runes")
	}
}

// ---- SplitLines ------------------------------------------------------------

func TestSplitLines_Empty(t *testing.T) {
	lines := SplitLines(nil, 80)
	if len(lines) != 1 {
		t.Fatalf("got %d lines, want 1", len(lines))
	}
	if len(lines[0]) != 0 {
		t.Fatalf("expected empty line, got len=%d", len(lines[0]))
	}
}

func TestSplitLines_NoWrap(t *testing.T) {
	line := runesLine("hello")
	lines := SplitLines(line, 80)
	if len(lines) != 1 {
		t.Fatalf("got %d lines, want 1", len(lines))
	}
	if lineStr(lines[0]) != "hello" {
		t.Fatalf("got %q, want %q", lineStr(lines[0]), "hello")
	}
}

func TestSplitLines_WrapAtWidth(t *testing.T) {
	line := runesLine("abcde")
	lines := SplitLines(line, 3)
	if len(lines) != 2 {
		t.Fatalf("got %d lines, want 2", len(lines))
	}
	if lineStr(lines[0]) != "abc" {
		t.Fatalf("line[0] = %q, want %q", lineStr(lines[0]), "abc")
	}
	if lineStr(lines[1]) != "de" {
		t.Fatalf("line[1] = %q, want %q", lineStr(lines[1]), "de")
	}
}

func TestSplitLines_NewlineSplits(t *testing.T) {
	runes := []rune("ab\ncd")
	cells := make(Line, len(runes))
	for i, r := range runes {
		cells[i] = Cell{Rune: r, Width: ansi.RuneWidth(r)}
	}
	lines := SplitLines(cells, 80)
	if len(lines) != 2 {
		t.Fatalf("got %d lines, want 2", len(lines))
	}
	if lineStr(lines[0]) != "ab" {
		t.Fatalf("line[0] = %q, want %q", lineStr(lines[0]), "ab")
	}
	if lineStr(lines[1]) != "cd" {
		t.Fatalf("line[1] = %q, want %q", lineStr(lines[1]), "cd")
	}
}

func TestSplitLines_NewlineAtEnd(t *testing.T) {
	runes := []rune("ab\n")
	cells := make(Line, len(runes))
	for i, r := range runes {
		cells[i] = Cell{Rune: r, Width: ansi.RuneWidth(r)}
	}
	lines := SplitLines(cells, 80)
	if len(lines) != 2 {
		t.Fatalf("got %d lines, want 2 (trailing newline adds empty line), got %d", len(lines), len(lines))
	}
}

func TestSplitLines_CarriageReturnSkipped(t *testing.T) {
	runes := []rune("a\rb")
	cells := make(Line, len(runes))
	for i, r := range runes {
		cells[i] = Cell{Rune: r, Width: ansi.RuneWidth(r)}
	}
	lines := SplitLines(cells, 80)
	if len(lines) != 1 {
		t.Fatalf("got %d lines, want 1 (\r should be skipped)", len(lines))
	}
	if lineStr(lines[0]) != "ab" {
		t.Fatalf("got %q, want %q", lineStr(lines[0]), "ab")
	}
}

func TestSplitLines_WideCharWraps(t *testing.T) {
	// "中" is width 2; with terminal width 3, "a中" (1+2) fits, "b" wraps
	cells := Line{
		{Rune: 'a', Width: 1},
		{Rune: '中', Width: 2},
		{Rune: 'b', Width: 1},
	}
	lines := SplitLines(cells, 3)
	if len(lines) != 2 {
		t.Fatalf("got %d lines, want 2", len(lines))
	}
	if lineStr(lines[0]) != "a中" {
		t.Fatalf("line[0] = %q, want %q", lineStr(lines[0]), "a中")
	}
	if lineStr(lines[1]) != "b" {
		t.Fatalf("line[1] = %q, want %q", lineStr(lines[1]), "b")
	}
}

func TestSplitLines_WideCharExactlyAtBoundary(t *testing.T) {
	// width=2, one wide char exactly fills it → single line
	cells := Line{{Rune: '中', Width: 2}}
	lines := SplitLines(cells, 2)
	if len(lines) != 1 {
		t.Fatalf("got %d lines, want 1", len(lines))
	}
}

func TestSplitLines_MultipleWraps(t *testing.T) {
	line := runesLine("abcdefghi")
	lines := SplitLines(line, 3)
	if len(lines) != 3 {
		t.Fatalf("got %d lines, want 3", len(lines))
	}
}

// ---- writeStyle ------------------------------------------------------------

func TestWriteStyle_ZeroStyle(t *testing.T) {
	var b bytes.Buffer
	writeStyle(&b, ansi.Style{})
	got := b.String()
	// minimal: reset + no extras + 'm'
	if !strings.HasPrefix(got, "\x1b[0") || !strings.HasSuffix(got, "m") {
		t.Fatalf("unexpected output: %q", got)
	}
	// no extra attributes
	if got != "\x1b[0m" {
		t.Fatalf("zero style: got %q, want %q", got, "\x1b[0m")
	}
}

func TestWriteStyle_Bold(t *testing.T) {
	var b bytes.Buffer
	writeStyle(&b, ansi.Style{Attr: ansi.AttrBold})
	got := b.String()
	if !strings.Contains(got, ";1") {
		t.Fatalf("bold: missing ;1 in %q", got)
	}
}

func TestWriteStyle_AllAttrs(t *testing.T) {
	tests := []struct {
		attr ansi.Attr
		code string
	}{
		{ansi.AttrBold, ";1"},
		{ansi.AttrDim, ";2"},
		{ansi.AttrItalic, ";3"},
		{ansi.AttrUnderline, ";4"},
		{ansi.AttrReverse, ";7"},
		{ansi.AttrStrike, ";9"},
	}
	for _, tt := range tests {
		var b bytes.Buffer
		writeStyle(&b, ansi.Style{Attr: tt.attr})
		got := b.String()
		if !strings.Contains(got, tt.code) {
			t.Errorf("attr %d: want %q in %q", tt.attr, tt.code, got)
		}
	}
}

func TestWriteStyle_MultipleAttrs(t *testing.T) {
	var b bytes.Buffer
	writeStyle(&b, ansi.Style{Attr: ansi.AttrBold | ansi.AttrItalic})
	got := b.String()
	if !strings.Contains(got, ";1") || !strings.Contains(got, ";3") {
		t.Fatalf("bold+italic: got %q", got)
	}
}

func TestWriteStyle_FgColor16(t *testing.T) {
	var b bytes.Buffer
	writeStyle(&b, ansi.Style{Fg: ansi.Color{Mode: ansi.Color16, Index: 2}})
	if !strings.Contains(b.String(), "32") { // 30+2
		t.Fatalf("fg Color16 index 2: got %q", b.String())
	}
}

func TestWriteStyle_FgColor16_High(t *testing.T) {
	var b bytes.Buffer
	writeStyle(&b, ansi.Style{Fg: ansi.Color{Mode: ansi.Color16, Index: 9}})
	// 30+60+(9-8)=91
	if !strings.Contains(b.String(), "91") {
		t.Fatalf("fg Color16 index 9 (bright): got %q", b.String())
	}
}

func TestWriteStyle_BgColor16(t *testing.T) {
	var b bytes.Buffer
	writeStyle(&b, ansi.Style{Bg: ansi.Color{Mode: ansi.Color16, Index: 1}})
	if !strings.Contains(b.String(), "41") { // 40+1
		t.Fatalf("bg Color16 index 1: got %q", b.String())
	}
}

func TestWriteStyle_BgColor16_High(t *testing.T) {
	var b bytes.Buffer
	writeStyle(&b, ansi.Style{Bg: ansi.Color{Mode: ansi.Color16, Index: 10}})
	// 40+60+(10-8)=102
	if !strings.Contains(b.String(), "102") {
		t.Fatalf("bg Color16 index 10 (bright): got %q", b.String())
	}
}

func TestWriteStyle_FgColor256(t *testing.T) {
	var b bytes.Buffer
	writeStyle(&b, ansi.Style{Fg: ansi.Color{Mode: ansi.Color256, Index: 200}})
	if !strings.Contains(b.String(), "38;5;200") {
		t.Fatalf("fg Color256: got %q", b.String())
	}
}

func TestWriteStyle_BgColor256(t *testing.T) {
	var b bytes.Buffer
	writeStyle(&b, ansi.Style{Bg: ansi.Color{Mode: ansi.Color256, Index: 100}})
	if !strings.Contains(b.String(), "48;5;100") {
		t.Fatalf("bg Color256: got %q", b.String())
	}
}

func TestWriteStyle_FgColorRGB(t *testing.T) {
	var b bytes.Buffer
	writeStyle(&b, ansi.Style{Fg: ansi.Color{Mode: ansi.ColorRGB, R: 10, G: 20, B: 30}})
	if !strings.Contains(b.String(), "38;2;10;20;30") {
		t.Fatalf("fg RGB: got %q", b.String())
	}
}

func TestWriteStyle_BgColorRGB(t *testing.T) {
	var b bytes.Buffer
	writeStyle(&b, ansi.Style{Bg: ansi.Color{Mode: ansi.ColorRGB, R: 1, G: 2, B: 3}})
	if !strings.Contains(b.String(), "48;2;1;2;3") {
		t.Fatalf("bg RGB: got %q", b.String())
	}
}

// ---- writeSGRColor ---------------------------------------------------------

func TestWriteSGRColor_Default_WritesNothing(t *testing.T) {
	var b bytes.Buffer
	writeSGRColor(&b, ansi.Color{Mode: ansi.ColorDefault}, true)
	if b.Len() != 0 {
		t.Fatalf("ColorDefault should write nothing, got %q", b.String())
	}
}

func TestWriteSGRColor_Color16_Fg_Low(t *testing.T) {
	var b bytes.Buffer
	writeSGRColor(&b, ansi.Color{Mode: ansi.Color16, Index: 3}, true)
	if !strings.Contains(b.String(), "33") {
		t.Fatalf("fg Color16 index 3: got %q", b.String())
	}
}

func TestWriteSGRColor_Color16_Bg_Low(t *testing.T) {
	var b bytes.Buffer
	writeSGRColor(&b, ansi.Color{Mode: ansi.Color16, Index: 5}, false)
	if !strings.Contains(b.String(), "45") {
		t.Fatalf("bg Color16 index 5: got %q", b.String())
	}
}

func TestWriteSGRColor_Color16_Fg_High(t *testing.T) {
	var b bytes.Buffer
	writeSGRColor(&b, ansi.Color{Mode: ansi.Color16, Index: 8}, true)
	// 30+60+(8-8) = 90
	if !strings.Contains(b.String(), "90") {
		t.Fatalf("fg Color16 index 8 (bright black): got %q", b.String())
	}
}

func TestWriteSGRColor_Color16_Bg_High(t *testing.T) {
	var b bytes.Buffer
	writeSGRColor(&b, ansi.Color{Mode: ansi.Color16, Index: 15}, false)
	// 40+60+(15-8)=107
	if !strings.Contains(b.String(), "107") {
		t.Fatalf("bg Color16 index 15: got %q", b.String())
	}
}

func TestWriteSGRColor_Color256_Fg(t *testing.T) {
	var b bytes.Buffer
	writeSGRColor(&b, ansi.Color{Mode: ansi.Color256, Index: 42}, true)
	if !strings.Contains(b.String(), "38;5;42") {
		t.Fatalf("fg Color256: got %q", b.String())
	}
}

func TestWriteSGRColor_Color256_Bg(t *testing.T) {
	var b bytes.Buffer
	writeSGRColor(&b, ansi.Color{Mode: ansi.Color256, Index: 42}, false)
	if !strings.Contains(b.String(), "48;5;42") {
		t.Fatalf("bg Color256: got %q", b.String())
	}
}

func TestWriteSGRColor_RGB_Fg(t *testing.T) {
	var b bytes.Buffer
	writeSGRColor(&b, ansi.Color{Mode: ansi.ColorRGB, R: 255, G: 128, B: 0}, true)
	if !strings.Contains(b.String(), "38;2;255;128;0") {
		t.Fatalf("fg RGB: got %q", b.String())
	}
}

func TestWriteSGRColor_RGB_Bg(t *testing.T) {
	var b bytes.Buffer
	writeSGRColor(&b, ansi.Color{Mode: ansi.ColorRGB, R: 0, G: 64, B: 128}, false)
	if !strings.Contains(b.String(), "48;2;0;64;128") {
		t.Fatalf("bg RGB: got %q", b.String())
	}
}

// ---- Renderer.SetSize ------------------------------------------------------

func TestRenderer_SetSize_Valid(t *testing.T) {
	r := newTestRenderer(nil)
	r.SetSize(100, 50)
	if r.width != 100 || r.height != 50 {
		t.Fatalf("SetSize(100,50): width=%d height=%d", r.width, r.height)
	}
}

func TestRenderer_SetSize_ZeroWidth_DefaultsTo80(t *testing.T) {
	r := newTestRenderer(nil)
	r.SetSize(0, 24)
	if r.width != 80 {
		t.Fatalf("width = %d, want 80", r.width)
	}
}

func TestRenderer_SetSize_ZeroHeight_DefaultsTo30(t *testing.T) {
	r := newTestRenderer(nil)
	r.SetSize(80, 0)
	if r.height != 30 {
		t.Fatalf("height = %d, want 30", r.height)
	}
}

func TestRenderer_SetSize_NegativeValues_Defaults(t *testing.T) {
	r := newTestRenderer(nil)
	r.SetSize(-5, -10)
	if r.width != 80 || r.height != 30 {
		t.Fatalf("width=%d height=%d; want 80,30", r.width, r.height)
	}
}

// ---- Renderer.Reset --------------------------------------------------------

func TestRenderer_Reset_ClearsState(t *testing.T) {
	r := newTestRenderer(nil)
	// poison the state
	r.state.termCur = 42
	r.state.previousCursorY = 5
	r.state.promptCells = []Cell{{Rune: 'x', Width: 1}}
	r.Reset()
	if r.state.termCur != 0 {
		t.Fatalf("termCur = %d after Reset, want 0", r.state.termCur)
	}
	if r.state.previousCursorY != 0 {
		t.Fatalf("previousCursorY = %d after Reset, want 0", r.state.previousCursorY)
	}
	if r.state.promptCells != nil {
		t.Fatal("promptCells should be nil after Reset")
	}
}

// ---- Renderer.Clear --------------------------------------------------------

func TestRenderer_Clear_WritesEraseSequence(t *testing.T) {
	w := &bufWriter{}
	r := newTestRenderer(w)
	r.SetSize(80, 24)
	if err := r.Clear(w); err != nil {
		t.Fatalf("Clear returned error: %v", err)
	}
	got := w.String()
	// must contain carriage return and erase-to-end-of-screen
	if !strings.Contains(got, "\r\x1b[J") {
		t.Fatalf("Clear output missing erase sequence: %q", got)
	}
}

func TestRenderer_Clear_ResetsState(t *testing.T) {
	w := &bufWriter{}
	r := newTestRenderer(w)
	r.SetSize(80, 24)
	r.state.previousCursorY = 2
	_ = r.Clear(w)
	if r.state.previousCursorY != 0 {
		t.Fatalf("state not reset after Clear")
	}
}

func TestRenderer_Clear_MovesUpWhenMultipleRows(t *testing.T) {
	w := &bufWriter{}
	r := newTestRenderer(w)
	r.SetSize(80, 24)
	r.state.previousCursorY = 2
	_ = r.Clear(w)
	got := w.String()
	if !strings.Contains(got, "\x1b[2A") {
		t.Fatalf("expected move-up 2 escape in %q", got)
	}
}

func TestRenderer_Clear_MovesUpAfterRenderedMultilinePrompt(t *testing.T) {
	w := &bufWriter{}
	r := newTestRenderer(w)
	r.SetSize(80, 24)
	r.config.Prompt = func(_, _ int) string { return "line 1\nline 2> " }
	r.editor.Insert('a')
	if err := r.Render(); err != nil {
		t.Fatalf("Render returned error: %v", err)
	}
	if r.state.previousCursorY != 1 {
		t.Fatalf("previousCursorY = %d, want 1", r.state.previousCursorY)
	}

	w.Reset()
	if err := r.Clear(w); err != nil {
		t.Fatalf("Clear returned error: %v", err)
	}
	if got := w.String(); !strings.Contains(got, "\x1b[1A\r\x1b[J") {
		t.Fatalf("Clear output = %q, want move to first prompt row and erase", got)
	}
}

func TestRenderer_ClearForResize_AccountsForReflowOnShrink(t *testing.T) {
	w := &bufWriter{}
	r := newTestRenderer(w)
	r.SetSize(20, 24)
	r.config.Prompt = func(_, _ int) string { return "123456789012345\n> " }
	r.editor.Insert('a')
	if err := r.Render(); err != nil {
		t.Fatalf("Render returned error: %v", err)
	}
	if r.state.previousCursorY != 1 {
		t.Fatalf("previousCursorY = %d, want 1 before shrink", r.state.previousCursorY)
	}

	w.Reset()
	if err := r.ClearForResize(w, 5); err != nil {
		t.Fatalf("ClearForResize returned error: %v", err)
	}
	if got := w.String(); !strings.Contains(got, "\x1b[3A\r\x1b[J") {
		t.Fatalf("ClearForResize output = %q, want move up 3 rows after shrink reflow", got)
	}
}

// ---- Renderer.UpdatePrompt -------------------------------------------------

func TestRenderer_UpdatePrompt_NilPrompt(t *testing.T) {
	r := newTestRenderer(nil)
	r.config.Prompt = nil
	r.UpdatePrompt()
	if r.state.promptCells != nil {
		t.Fatal("promptCells should remain nil when Prompt func is nil")
	}
}

func TestRenderer_UpdatePrompt_WithPrompt(t *testing.T) {
	r := newTestRenderer(nil)
	r.config.Prompt = func(_, _ int) string { return "> " }
	r.UpdatePrompt()
	if len(r.state.promptCells) == 0 {
		t.Fatal("expected non-empty promptCells after UpdatePrompt")
	}
}

// ---- Renderer.Render -------------------------------------------------------

// TestRenderer_Render_ProducesOutput verifies a basic render writes something.
func TestRenderer_Render_ProducesOutput(t *testing.T) {
	w := &bufWriter{}
	r := newTestRenderer(w)
	r.config.Prompt = func(_, _ int) string { return "> " }
	if err := r.Render(); err != nil {
		t.Fatalf("Render error: %v", err)
	}
	if w.Len() == 0 {
		t.Fatal("expected non-empty output from Render")
	}
}

// TestRenderer_Render_ContentInOutput checks that prompt and buffer text are present.
func TestRenderer_Render_ContentInOutput(t *testing.T) {
	w := &bufWriter{}
	r := newTestRenderer(w)
	r.config.Prompt = func(_, _ int) string { return ">> " }
	r.editor.Insert('h')
	r.editor.Insert('i')
	if err := r.Render(); err != nil {
		t.Fatalf("Render error: %v", err)
	}
	got := w.String()
	if !strings.Contains(got, ">> ") {
		t.Errorf("prompt missing from output: %q", got)
	}
	if !strings.Contains(got, "hi") {
		t.Errorf("buffer content missing from output: %q", got)
	}
}

// TestRenderer_Render_StateAfterRender checks that previousLines is populated
// and currentLines is cleared after a successful Render call.
func TestRenderer_Render_StateAfterRender(t *testing.T) {
	w := &bufWriter{}
	r := newTestRenderer(w)
	r.config.Prompt = func(_, _ int) string { return "> " }
	r.editor.Insert('a')
	if err := r.Render(); err != nil {
		t.Fatalf("Render error: %v", err)
	}
	if len(r.state.previousLines) == 0 {
		t.Fatal("previousLines should be non-empty after Render")
	}
	if r.state.currentLines != nil {
		t.Fatal("currentLines should be nil after Render (swapped to previous)")
	}
}

func TestRenderer_Render_HidesCursorDuringWrite(t *testing.T) {
	w := &bufWriter{}
	r := newTestRenderer(w)
	r.config.Prompt = func(_, _ int) string { return "> " }
	r.editor.Insert('x')

	if err := r.Render(); err != nil {
		t.Fatalf("Render error: %v", err)
	}

	got := w.String()
	if !strings.HasPrefix(got, "\x1b[?25l") {
		t.Fatalf("render should hide cursor before writes: %q", got)
	}
	if !strings.HasSuffix(got, "\x1b[?25h") {
		t.Fatalf("render should show cursor after writes: %q", got)
	}
}

// TestRenderer_Render_CursorColumnAtEnd verifies the final \x1b[NG escape
// positions the cursor after the full prompt+buffer content.
// Prompt "> " (width 2) + buffer "hi" (width 2) → column 5.
func TestRenderer_Render_CursorColumnAtEnd(t *testing.T) {
	w := &bufWriter{}
	r := newTestRenderer(w)
	r.config.Prompt = func(_, _ int) string { return "> " }
	r.editor.Insert('h')
	r.editor.Insert('i')
	if err := r.Render(); err != nil {
		t.Fatalf("Render error: %v", err)
	}
	col, ok := lastColumnEscape(w.String())
	if !ok {
		t.Fatalf("no column escape found in output: %q", w.String())
	}
	// prompt=2 + buffer=2 + 1-indexed = 5
	if col != 5 {
		t.Fatalf("cursor column = %d, want 5; output: %q", col, w.String())
	}
}

// TestRenderer_Render_CursorColumnMidBuffer verifies column placement when
// the cursor is not at the end of the buffer.
// Prompt "> " (width 2) + pre-cursor "a" (width 1) → column 4.
func TestRenderer_Render_CursorColumnMidBuffer(t *testing.T) {
	w := &bufWriter{}
	r := newTestRenderer(w)
	r.config.Prompt = func(_, _ int) string { return "> " }
	r.editor.Insert('a')
	r.editor.Insert('b')
	r.editor.Insert('c')
	r.editor.MoveCursor(-2) // cursor at 1, after 'a'
	if err := r.Render(); err != nil {
		t.Fatalf("Render error: %v", err)
	}
	col, ok := lastColumnEscape(w.String())
	if !ok {
		t.Fatalf("no column escape in output: %q", w.String())
	}
	// prompt=2 + "a"=1 + 1-indexed = 4
	if col != 4 {
		t.Fatalf("cursor column = %d, want 4; output: %q", col, w.String())
	}
}

// TestRenderer_Render_CursorColumnEmptyBuffer: prompt "> " (width 2), no buffer → column 3.
func TestRenderer_Render_CursorColumnEmptyBuffer(t *testing.T) {
	w := &bufWriter{}
	r := newTestRenderer(w)
	r.config.Prompt = func(_, _ int) string { return "> " }
	if err := r.Render(); err != nil {
		t.Fatalf("Render error: %v", err)
	}
	col, ok := lastColumnEscape(w.String())
	if !ok {
		t.Fatalf("no column escape in output: %q", w.String())
	}
	// prompt=2 + empty buffer + 1-indexed = 3
	if col != 3 {
		t.Fatalf("cursor column = %d, want 3; output: %q", col, w.String())
	}
}

func TestRenderer_Render_CursorColumnWithGraphemeClusters(t *testing.T) {
	w := &bufWriter{}
	r := newTestRenderer(w)
	r.config.Prompt = func(_, _ int) string { return "> " }
	for _, ru := range "🐈🐈‍⬛" {
		r.editor.Insert(ru)
	}
	if err := r.Render(); err != nil {
		t.Fatalf("Render error: %v", err)
	}
	col, ok := lastColumnEscape(w.String())
	if !ok {
		t.Fatalf("no column escape found in output: %q", w.String())
	}
	// prompt=2 + emoji buffer=4 + 1-indexed = 7
	if col != 7 {
		t.Fatalf("cursor column = %d, want 7; output: %q", col, w.String())
	}
}

// TestRenderer_Render_NoDiffPath verifies that when content hasn't changed,
// the second Render call takes the fast path and does not emit \x1b[J
// (erase-to-end-of-screen), since no content needs to be redrawn.
func TestRenderer_Render_NoDiffPath(t *testing.T) {
	w := &bufWriter{}
	r := newTestRenderer(w)
	r.config.Prompt = func(_, _ int) string { return "> " }
	r.editor.Insert('a')

	// first render — full draw
	if err := r.Render(); err != nil {
		t.Fatalf("first Render error: %v", err)
	}

	// second render — nothing changed, should use the no-diff fast path
	w.Reset()
	if err := r.Render(); err != nil {
		t.Fatalf("second Render error: %v", err)
	}
	got := w.String()
	if strings.Contains(got, "\x1b[J") {
		t.Fatalf("second identical render should not erase screen, got: %q", got)
	}
	// but it must still reposition the cursor
	if _, ok := lastColumnEscape(got); !ok {
		t.Fatalf("second render must emit cursor column escape: %q", got)
	}
}

// TestRenderer_Render_ContentChangeTriggersDiff verifies that when the buffer
// changes between renders, the output contains the updated content and the
// erase-to-end-of-screen escape \x1b[J.
func TestRenderer_Render_ContentChangeTriggersDiff(t *testing.T) {
	w := &bufWriter{}
	r := newTestRenderer(w)
	r.config.Prompt = func(_, _ int) string { return "> " }
	r.editor.Insert('a')
	r.editor.Insert('b')
	_ = r.Render()

	// change one character
	r.editor.MoveCursor(-1)
	r.editor.DeleteNext()
	r.editor.Insert('X')
	r.editor.MoveCursor(1)

	w.Reset()
	if err := r.Render(); err != nil {
		t.Fatalf("Render after change error: %v", err)
	}
	got := w.String()
	if !strings.Contains(got, "\x1b[J") {
		t.Fatalf("changed content should trigger screen erase: %q", got)
	}
	if !strings.Contains(got, "X") {
		t.Fatalf("new content 'X' not in output: %q", got)
	}
}

// TestRenderer_Render_PreviousCursorYTracked verifies previousCursorY is 0
// after a single-line render (cursor stays on the first row).
func TestRenderer_Render_PreviousCursorYTracked(t *testing.T) {
	w := &bufWriter{}
	r := newTestRenderer(w)
	r.config.Prompt = func(_, _ int) string { return "> " }
	r.editor.Insert('a')
	_ = r.Render()
	if r.state.previousCursorY != 0 {
		t.Fatalf("previousCursorY = %d, want 0 for single-line render", r.state.previousCursorY)
	}
}

// TestRenderer_Render_PreviousCursorYMultiLine checks that when the prompt
// itself wraps to a second row, the cursor line index (previousCursorY) is > 0.
// Prompt "123456" on a width-5 terminal produces 2 rendered rows; the buffer
// starts on row 1, so previousCursorY should be 1 after the render.
func TestRenderer_Render_PreviousCursorYMultiLine(t *testing.T) {
	w := &bufWriter{}
	r := newTestRenderer(w)
	r.SetSize(5, 24)
	r.config.Prompt = func(_, _ int) string { return "123456" } // 6 chars → wraps at width 5
	r.editor.Insert('a')
	_ = r.Render()
	if r.state.previousCursorY != 1 {
		t.Fatalf("previousCursorY = %d, want 1 when prompt wraps to line 1", r.state.previousCursorY)
	}
}

// TestRenderer_Render_MoveUpOnSecondRender verifies that when previousCursorY>0
// the second render emits a cursor-up escape to return to the top of the output.
func TestRenderer_Render_MoveUpOnSecondRender(t *testing.T) {
	w := &bufWriter{}
	r := newTestRenderer(w)
	r.SetSize(5, 24)
	r.config.Prompt = func(_, _ int) string { return "123456" } // wraps to 2 rows
	r.editor.Insert('a')
	_ = r.Render()

	if r.state.previousCursorY == 0 {
		t.Skip("previousCursorY unexpectedly 0, skipping move-up check")
	}
	prevY := r.state.previousCursorY

	// second render (same content) — no-diff path still emits cursor-up
	w.Reset()
	_ = r.Render()

	upEsc := fmt.Sprintf("\x1b[%dA", prevY)
	if !strings.Contains(w.String(), upEsc) {
		t.Fatalf("expected move-up %q in output: %q", upEsc, w.String())
	}
}

// TestRenderer_Render_HighlighterApplied verifies SGR codes from the
// highlighter appear in the rendered output.
func TestRenderer_Render_HighlighterApplied(t *testing.T) {
	w := &bufWriter{}
	r := newTestRenderer(w)
	r.config.Prompt = func(_, _ int) string { return "" }
	r.config.Highlighter = func(runes []rune) []ansi.Span {
		if len(runes) == 0 {
			return nil
		}
		return []ansi.Span{{Start: 0, End: len(runes), Style: ansi.Style{Attr: ansi.AttrBold}}}
	}
	r.editor.Insert('x')
	if err := r.Render(); err != nil {
		t.Fatalf("Render error: %v", err)
	}
	// bold = SGR code ;1
	if !strings.Contains(w.String(), ";1") {
		t.Fatalf("bold SGR missing from output: %q", w.String())
	}
}

// TestRenderer_Render_HintAppearsInOutput checks that hint text is rendered
// below the main line.
func TestRenderer_Render_HintAppearsInOutput(t *testing.T) {
	w := &bufWriter{}
	r := newTestRenderer(w)
	r.config.Prompt = func(_, _ int) string { return "> " }
	r.editor.Insert('a')
	r.editor.SetHint("(type more)")
	if err := r.Render(); err != nil {
		t.Fatalf("Render error: %v", err)
	}
	if !strings.Contains(w.String(), "type more") {
		t.Fatalf("hint missing from output: %q", w.String())
	}
}

// TestRenderer_Render_HintNotRenderedWhenNoRemainingRows verifies that hints
// are suppressed when the terminal is completely full.
func TestRenderer_Render_HintNotRenderedWhenNoRemainingRows(t *testing.T) {
	w := &bufWriter{}
	r := newTestRenderer(w)
	r.SetSize(80, 1) // only 1 row total
	r.config.Prompt = func(_, _ int) string { return "" }
	r.editor.Insert('a')
	r.editor.SetHint("should not appear")
	if err := r.Render(); err != nil {
		t.Fatalf("Render error: %v", err)
	}
	if strings.Contains(w.String(), "should not appear") {
		t.Fatalf("hint should be suppressed when terminal is full: %q", w.String())
	}
}

// TestRenderer_Render_CompletionCandidatesInOutput checks that completion
// candidate names are rendered below the main line.
func TestRenderer_Render_CompletionCandidatesInOutput(t *testing.T) {
	w := &bufWriter{}
	r := newTestRenderer(w)
	r.config.Prompt = func(_, _ int) string { return "> " }
	r.editor.Insert('a')
	// inject completions directly via the editor (TriggerCompletions requires a completer)
	// we reach into the unexported field via the editor's GetCompletions — instead we call
	// TriggerCompletions with a completer set up via WithCompleter option.
	ed := editor.New(editor.WithCompleter(staticCompleter{
		candidates: []completionCandidate{
			{name: "alpha", desc: "first"},
			{name: "beta", desc: "second"},
		},
	}))
	r.editor = ed
	r.editor.Insert('a')
	r.editor.TriggerCompletions()
	if err := r.Render(); err != nil {
		t.Fatalf("Render error: %v", err)
	}
	got := w.String()
	if !strings.Contains(got, "alpha") {
		t.Errorf("completion 'alpha' missing from output: %q", got)
	}
	if !strings.Contains(got, "beta") {
		t.Errorf("completion 'beta' missing from output: %q", got)
	}
}

func TestRenderer_Render_SelectedCompletionUsesReverseVideo(t *testing.T) {
	w := &bufWriter{}
	r := newTestRenderer(w)
	r.config.Prompt = func(_, _ int) string { return "> " }
	r.editor = editor.New(editor.WithCompleter(staticCompleter{
		candidates: []completionCandidate{
			{name: "alpha", desc: "first"},
			{name: "beta", desc: "second"},
		},
	}))
	r.editor.Insert('a')
	r.editor.TriggerCompletions()
	r.editor.SelectNextCompletion()

	if err := r.Render(); err != nil {
		t.Fatalf("Render error: %v", err)
	}
	got := w.String()
	if !strings.Contains(got, ";7") {
		t.Fatalf("selected completion should use reverse video: %q", got)
	}
}

func TestRenderer_Render_CompletionsLimitedToAvailableTerminalHeight(t *testing.T) {
	w := &bufWriter{}
	r := newTestRenderer(w)
	r.SetSize(80, 4)
	r.config.Prompt = func(_, _ int) string { return "line 1\n> " }
	r.editor = editor.New(editor.WithCompleter(staticCompleter{
		candidates: []completionCandidate{
			{name: "alpha", desc: "first"},
			{name: "beta", desc: "second"},
			{name: "gamma", desc: "third"},
			{name: "delta", desc: "fourth"},
		},
	}))
	r.editor.Insert('a')
	r.editor.TriggerCompletions()

	if err := r.Render(); err != nil {
		t.Fatalf("Render error: %v", err)
	}

	got := w.String()
	if !strings.Contains(got, "alpha") {
		t.Fatalf("first completion should be rendered: %q", got)
	}
	if strings.Contains(got, "beta") || strings.Contains(got, "gamma") || strings.Contains(got, "delta") {
		t.Fatalf("completion output exceeded available rows: %q", got)
	}
	if rows := queuedPhysicalRows(r.state.previousLines, r.width); rows > r.height {
		t.Fatalf("rendered rows = %d, want <= terminal height %d", rows, r.height)
	}
}

func TestRenderer_Render_AnchorBottomDefaultsOff(t *testing.T) {
	w := &bufWriter{}
	r := newTestRenderer(w)
	r.SetSize(80, 6)
	r.config.Prompt = func(_, _ int) string { return "> " }
	r.editor.Insert('a')

	if err := r.Render(); err != nil {
		t.Fatalf("Render error: %v", err)
	}
	if rows := queuedPhysicalRows(r.state.previousLines, r.width); rows != 1 {
		t.Fatalf("rendered rows = %d, want no anchor padding", rows)
	}
}

func TestRenderer_Render_AnchorBottomPadsToTerminalHeight(t *testing.T) {
	w := &positionedBufWriter{row: 4, col: 1}
	r := newTestRenderer(w)
	r.SetSize(80, 6)
	r.config.AnchorBottom = true
	r.config.Prompt = func(_, _ int) string { return "> " }
	r.editor.Insert('a')

	if err := r.Render(); err != nil {
		t.Fatalf("Render error: %v", err)
	}
	if rows := queuedPhysicalRows(r.state.previousLines, r.width); rows != 3 {
		t.Fatalf("rendered rows = %d, want remaining rows from cursor to bottom", rows)
	}
	if got := r.state.previousCursorY; got != 2 {
		t.Fatalf("previousCursorY = %d, want bottom row of render area 2", got)
	}
	last := r.state.previousLines[len(r.state.previousLines)-1]
	if got := lineStr(last.Line); got != "> a" {
		t.Fatalf("bottom rendered line = %q, want prompt+buffer", got)
	}
}

func TestRenderer_Render_AnchorBottomRecalculatesWhenCompletionsShrink(t *testing.T) {
	w := &positionedBufWriter{row: 3, col: 1}
	r := newTestRenderer(w)
	r.SetSize(80, 8)
	r.config.AnchorBottom = true
	r.config.Prompt = func(_, _ int) string { return "> " }
	r.editor = editor.New(editor.WithCompleter(staticCompleter{
		candidates: []completionCandidate{
			{name: "alpha", desc: "first"},
			{name: "beta", desc: "second"},
			{name: "gamma", desc: "third"},
		},
	}))
	r.editor.Insert('a')
	r.editor.TriggerCompletions()

	if err := r.Render(); err != nil {
		t.Fatalf("first Render error: %v", err)
	}
	if rows := queuedPhysicalRows(r.state.previousLines, r.width); rows != 6 {
		t.Fatalf("first render rows = %d, want remaining rows from cursor to bottom", rows)
	}

	r.editor.ClearCompletions()
	w.Reset()
	w.row = 3 + r.state.previousCursorY
	if err := r.Render(); err != nil {
		t.Fatalf("second Render error: %v", err)
	}
	if rows := queuedPhysicalRows(r.state.previousLines, r.width); rows != 6 {
		t.Fatalf("second render rows = %d, want remaining rows from render start to bottom", rows)
	}
	if got := r.state.previousCursorY; got != 5 {
		t.Fatalf("previousCursorY = %d, want bottom row of render area 5", got)
	}
	last := r.state.previousLines[len(r.state.previousLines)-1]
	if got := lineStr(last.Line); got != "> a" {
		t.Fatalf("bottom rendered line = %q, want prompt+buffer", got)
	}
}

func TestRenderer_Render_AnchorBottomAcceptCompletionClearsOldCandidateLine(t *testing.T) {
	w := &positionedBufWriter{row: 3, col: 1}
	r := newTestRenderer(w)
	r.SetSize(120, 8)
	r.config.AnchorBottom = true
	r.config.Prompt = func(_, _ int) string { return "> " }
	r.config.StatusLine = func(_, _ int) string { return "status" }
	r.editor = editor.New(editor.WithCompleter(staticCompleter{
		candidates: []completionCandidate{
			{name: "/resume", desc: "Resume a previous chat session"},
			{name: "/model", desc: "Select the model to use for chat"},
		},
	}))
	r.editor.Insert('/')
	r.editor.TriggerCompletions()

	if err := r.Render(); err != nil {
		t.Fatalf("first Render error: %v", err)
	}
	if !r.editor.AcceptSelectedCompletion() {
		t.Fatal("AcceptSelectedCompletion = false, want true")
	}
	w.Reset()
	w.row = 3 + r.state.previousCursorY

	if err := r.Render(); err != nil {
		t.Fatalf("second Render error: %v", err)
	}
	got := w.String()
	if !strings.Contains(got, "\x1b[K> /resume") {
		t.Fatalf("accepting completion should clear prompt line before redraw: %q", got)
	}
	promptLine := r.state.previousLines[len(r.state.previousLines)-2]
	if got := lineStr(promptLine.Line); got != "> /resume" {
		t.Fatalf("bottom rendered line = %q, want accepted completion only", got)
	}
}

func TestRenderer_Render_AnchorBottomRecalculatesAfterResize(t *testing.T) {
	w := &positionedBufWriter{row: 3, col: 1}
	r := newTestRenderer(w)
	r.SetSize(80, 8)
	r.config.AnchorBottom = true
	r.config.Prompt = func(_, _ int) string { return "> " }
	r.editor.Insert('a')

	if err := r.Render(); err != nil {
		t.Fatalf("first Render error: %v", err)
	}
	if rows := queuedPhysicalRows(r.state.previousLines, r.width); rows != 6 {
		t.Fatalf("first render rows = %d, want remaining rows from cursor to bottom", rows)
	}

	r.SetSize(80, 4)
	w.row = 2
	if err := r.Render(); err != nil {
		t.Fatalf("second Render error: %v", err)
	}
	if rows := queuedPhysicalRows(r.state.previousLines, r.width); rows != 3 {
		t.Fatalf("second render rows = %d, want resized remaining rows", rows)
	}
}

func TestRenderer_RenderAccepted_ClearsAutosuggestion(t *testing.T) {
	w := &bufWriter{}
	r := newTestRenderer(w)
	r.config.Prompt = func(_, _ int) string { return "> " }
	r.editor = editor.New(editor.WithSuggester(staticSuggester{suggestion: []rune(" status")}))
	for _, c := range "git" {
		r.editor.Insert(c)
	}
	r.editor.TriggerAutoSuggestion()

	if err := r.Render(); err != nil {
		t.Fatalf("Render error: %v", err)
	}
	if !strings.Contains(w.String(), "git") || !strings.Contains(w.String(), " status") {
		t.Fatalf("initial render should include buffer and suggestion: %q", w.String())
	}

	w.Reset()
	if err := r.RenderAccepted(); err != nil {
		t.Fatalf("RenderAccepted error: %v", err)
	}
	got := w.String()
	if strings.Contains(got, " status") {
		t.Fatalf("accepted render should not render autosuggestion: %q", got)
	}
	if !strings.Contains(got, "\x1b[J") {
		t.Fatalf("accepted render should erase stale suggestion cells: %q", got)
	}
}

func TestRenderer_RenderAccepted_SuppressesTransientRows(t *testing.T) {
	w := &bufWriter{}
	r := newTestRenderer(w)
	r.config.Prompt = func(_, _ int) string { return "> " }
	r.config.StatusLine = func(_, _ int) string { return "status line" }
	r.editor = editor.New(editor.WithCompleter(staticCompleter{
		candidates: []completionCandidate{
			{name: "alpha", desc: "first"},
			{name: "beta", desc: "second"},
		},
	}))
	r.editor.Insert('a')
	r.editor.SetHint("hint text")
	r.editor.TriggerCompletions()

	if err := r.Render(); err != nil {
		t.Fatalf("Render error: %v", err)
	}
	initial := w.String()
	for _, want := range []string{"hint text", "alpha", "status line"} {
		if !strings.Contains(initial, want) {
			t.Fatalf("initial render should include %q: %q", want, initial)
		}
	}

	w.Reset()
	if err := r.RenderAccepted(); err != nil {
		t.Fatalf("RenderAccepted error: %v", err)
	}
	got := w.String()
	for _, suppressed := range []string{"hint text", "alpha", "status line"} {
		if strings.Contains(got, suppressed) {
			t.Fatalf("accepted render should suppress %q: %q", suppressed, got)
		}
	}
	if !strings.Contains(got, "\x1b[J") {
		t.Fatalf("accepted render should erase stale transient rows: %q", got)
	}
}

func TestRenderer_RenderAccepted_HidesPromptWhenConfigured(t *testing.T) {
	w := &bufWriter{}
	r := newTestRenderer(w)
	r.config.Prompt = func(_, _ int) string { return "prompt> " }
	r.config.HidePromptOnSubmit = true
	for _, c := range "submitted" {
		r.editor.Insert(c)
	}

	if err := r.RenderAccepted(); err != nil {
		t.Fatalf("RenderAccepted error: %v", err)
	}
	got := w.String()
	if strings.Contains(got, "prompt> ") {
		t.Fatalf("accepted render should hide prompt: %q", got)
	}
	if !strings.Contains(got, "submitted") {
		t.Fatalf("accepted render should keep submitted text: %q", got)
	}
}

func TestRenderer_RenderAccepted_ShowsPromptByDefault(t *testing.T) {
	w := &bufWriter{}
	r := newTestRenderer(w)
	r.config.Prompt = func(_, _ int) string { return "prompt> " }
	for _, c := range "submitted" {
		r.editor.Insert(c)
	}

	if err := r.RenderAccepted(); err != nil {
		t.Fatalf("RenderAccepted error: %v", err)
	}
	got := w.String()
	if !strings.Contains(got, "prompt> ") {
		t.Fatalf("accepted render should show prompt by default: %q", got)
	}
	if !strings.Contains(got, "submitted") {
		t.Fatalf("accepted render should keep submitted text: %q", got)
	}
}

// TestRenderer_Render_LogLinesInOutput checks that log lines are prepended
// before the prompt.
func TestRenderer_Render_LogLinesInOutput(t *testing.T) {
	w := &bufWriter{}
	r := newTestRenderer(w)
	r.config.Prompt = func(_, _ int) string { return "> " }
	r.editor.Log("log line one")
	r.editor.Insert('a')
	if err := r.Render(); err != nil {
		t.Fatalf("Render error: %v", err)
	}
	if !strings.Contains(w.String(), "log line one") {
		t.Fatalf("log text missing from output: %q", w.String())
	}
}

// TestRenderer_Render_WriterError checks that driver write errors propagate.
func TestRenderer_Render_WriterError(t *testing.T) {
	ew := &errWriter{err: bytes.ErrTooLarge}
	r := newTestRenderer(ew)
	r.config.Prompt = func(_, _ int) string { return "> " }
	if err := r.Render(); err == nil {
		t.Fatal("expected error from writer, got nil")
	}
}

// TestRenderer_Render_LongLineWraps verifies a render with a narrow terminal
// and a long buffer does not return an error.
func TestRenderer_Render_LongLineWraps(t *testing.T) {
	w := &bufWriter{}
	r := newTestRenderer(w)
	r.SetSize(10, 24)
	r.config.Prompt = func(_, _ int) string { return "" }
	for _, c := range "abcdefghijklmnopqrstuvwxyz" {
		r.editor.Insert(c)
	}
	if err := r.Render(); err != nil {
		t.Fatalf("Render error: %v", err)
	}
}

const wrappedCommandRegression = `ic db --code-search --query "select count(1) from resources where org_id in ('87acbf2b-9480-444f-b17c-628d30ec8700', 'e9289523-453f-462a-bd2b-700edfcdcff4', 'd25dcda2-fec5-4b41-8793-894c9ffb61f8')" | jq -r '.[].count'`

func TestRenderer_Render_MovingLeftAcrossWrappedCommand(t *testing.T) {
	const width = 72
	const prompt = "> "

	w := &bufWriter{}
	r := newTestRenderer(w)
	r.SetSize(width, 24)
	r.config.Prompt = func(_, _ int) string { return prompt }
	for _, c := range wrappedCommandRegression {
		r.editor.Insert(c)
	}
	if err := r.Render(); err != nil {
		t.Fatalf("initial render: %v", err)
	}

	for cursor := len([]rune(wrappedCommandRegression)) - 1; cursor >= 0; cursor-- {
		r.editor.MoveCursor(-1)
		w.Reset()
		if err := r.Render(); err != nil {
			t.Fatalf("render at cursor %d: %v", cursor, err)
		}

		gotCol, ok := lastColumnEscape(w.String())
		if !ok {
			t.Fatalf("cursor %d: no column escape in %q", cursor, w.String())
		}
		flatCursor := len([]rune(prompt)) + cursor
		wantCol := flatCursor%width + 1
		if gotCol != wantCol {
			t.Fatalf("cursor %d: column = %d, want %d; output: %q", cursor, gotCol, wantCol, w.String())
		}
		if gotCol > width {
			t.Fatalf("cursor %d: emitted column %d exceeds terminal width %d", cursor, gotCol, width)
		}

		wantRow := flatCursor / width
		if gotRow := r.state.previousCursorY; gotRow != wantRow {
			t.Fatalf("cursor %d: row = %d, want %d", cursor, gotRow, wantRow)
		}
	}
}

func TestRenderer_Render_BackspaceAtEndOfWrappedCommandPreservesRows(t *testing.T) {
	const width = 72

	w := &bufWriter{}
	r := newTestRenderer(w)
	r.SetSize(width, 24)
	r.config.Prompt = func(_, _ int) string { return "> " }
	for _, c := range wrappedCommandRegression {
		r.editor.Insert(c)
	}
	if err := r.Render(); err != nil {
		t.Fatalf("initial render: %v", err)
	}

	for i := 0; i < 10; i++ {
		previousRow := r.state.previousCursorY
		r.editor.DeletePrevious()
		w.Reset()
		if err := r.Render(); err != nil {
			t.Fatalf("render after backspace %d: %v", i+1, err)
		}

		ups := countEscapeN(w.String(), "A")
		downs := countEscapeN(w.String(), "B")
		wantNetDown := r.state.previousCursorY - previousRow
		if gotNetDown := downs - ups; gotNetDown != wantNetDown {
			t.Fatalf(
				"backspace %d: net vertical movement = %d, want %d; output: %q",
				i+1, gotNetDown, wantNetDown, w.String(),
			)
		}
	}
}

// TestRenderer_Render_BufferTruncatedToTerminalHeight checks that when the
// buffer is taller than the terminal, postCursorLines are capped.
func TestRenderer_Render_BufferTruncatedToTerminalHeight(t *testing.T) {
	w := &bufWriter{}
	r := newTestRenderer(w)
	r.SetSize(3, 2) // only 2 rows
	r.config.Prompt = func(_, _ int) string { return "" }
	// cursor at start, so all content is "post-cursor"
	for _, c := range "abcdefghi" { // 3 rows at width 3
		r.editor.Insert(c)
	}
	r.editor.MoveCursor(-9) // cursor at 0
	if err := r.Render(); err != nil {
		t.Fatalf("Render error: %v", err)
	}
}

func TestRenderer_Render_MultilineBufferPlacesCursorOnLastLine(t *testing.T) {
	w := &bufWriter{}
	r := newTestRenderer(w)
	r.config.Prompt = func(_, _ int) string { return "> " }
	for _, c := range "echo hello\\\nworld" {
		r.editor.Insert(c)
	}

	if err := r.Render(); err != nil {
		t.Fatalf("Render error: %v", err)
	}

	if got, want := r.state.previousCursorY, 1; got != want {
		t.Fatalf("previousCursorY = %d, want %d", got, want)
	}
	if len(r.state.previousLines) != 2 {
		t.Fatalf("rendered %d lines, want 2", len(r.state.previousLines))
	}
	if got, want := lineStr(r.state.previousLines[0].Line), "> echo hello\\"; got != want {
		t.Fatalf("line 0 = %q, want %q", got, want)
	}
	if got, want := lineStr(r.state.previousLines[1].Line), "world"; got != want {
		t.Fatalf("line 1 = %q, want %q", got, want)
	}
	if got, want := r.state.previousLines[1].CursorCol, len("world")+1; got != want {
		t.Fatalf("second-line cursor = %d, want %d", got, want)
	}
	if r.state.previousLines[0].CursorCol != 0 {
		t.Fatalf("first line should not carry cursor, got %d", r.state.previousLines[0].CursorCol)
	}
}

func TestRenderer_Render_MultilineAutosuggestionStartsAtCursorLine(t *testing.T) {
	w := &bufWriter{}
	r := newTestRenderer(w)
	r.editor = editor.New(editor.WithSuggester(staticSuggester{suggestion: []rune("\nworld")}))
	r.config.Prompt = func(_, _ int) string { return "> " }
	for _, c := range "echo hello\\\n" {
		r.editor.Insert(c)
	}
	r.editor.TriggerAutoSuggestion()

	if err := r.Render(); err != nil {
		t.Fatalf("Render error: %v", err)
	}

	if len(r.state.previousLines) != 2 {
		t.Fatalf("rendered %d lines, want 2", len(r.state.previousLines))
	}
	if got, want := lineStr(r.state.previousLines[0].Line), "> echo hello\\"; got != want {
		t.Fatalf("line 0 = %q, want %q", got, want)
	}
	if got, want := lineStr(r.state.previousLines[1].Line), "world"; got != want {
		t.Fatalf("line 1 = %q, want %q", got, want)
	}
	if got, want := r.state.previousLines[1].CursorCol, 1; got != want {
		t.Fatalf("second-line cursor = %d, want %d", got, want)
	}
}

func TestRenderer_Render_MultilineAutosuggestionAfterContinuationText(t *testing.T) {
	w := &bufWriter{}
	r := newTestRenderer(w)
	r.editor = editor.New(editor.WithSuggester(staticSuggester{suggestion: []rune("rld")}))
	r.config.Prompt = func(_, _ int) string { return "> " }
	for _, c := range "echo hello\\\nwo" {
		r.editor.Insert(c)
	}
	r.editor.TriggerAutoSuggestion()

	if err := r.Render(); err != nil {
		t.Fatalf("Render error: %v", err)
	}

	if len(r.state.previousLines) != 2 {
		t.Fatalf("rendered %d lines, want 2", len(r.state.previousLines))
	}
	if got, want := lineStr(r.state.previousLines[1].Line), "world"; got != want {
		t.Fatalf("line 1 = %q, want %q", got, want)
	}
	if got, want := r.state.previousLines[1].CursorCol, len("wo")+1; got != want {
		t.Fatalf("second-line cursor = %d, want %d", got, want)
	}
}

func TestRenderer_RenderAccepted_MultilineBufferLeavesCursorAfterLastLine(t *testing.T) {
	w := &bufWriter{}
	r := newTestRenderer(w)
	r.config.Prompt = func(_, _ int) string { return "> " }
	for _, c := range "echo hello\\\nworld" {
		r.editor.Insert(c)
	}

	if err := r.RenderAccepted(); err != nil {
		t.Fatalf("RenderAccepted error: %v", err)
	}

	if got, want := r.state.previousCursorY, 1; got != want {
		t.Fatalf("previousCursorY = %d, want %d", got, want)
	}
	if len(r.state.previousLines) != 2 {
		t.Fatalf("rendered %d lines, want 2", len(r.state.previousLines))
	}
	if got, want := lineStr(r.state.previousLines[0].Line), "> echo hello\\"; got != want {
		t.Fatalf("line 0 = %q, want %q", got, want)
	}
	if got, want := lineStr(r.state.previousLines[1].Line), "world"; got != want {
		t.Fatalf("line 1 = %q, want %q", got, want)
	}
	if got, want := r.state.previousLines[1].CursorCol, len("world")+1; got != want {
		t.Fatalf("second-line cursor = %d, want %d", got, want)
	}
	if r.state.previousLines[0].CursorCol != 0 {
		t.Fatalf("first line should not carry cursor, got %d", r.state.previousLines[0].CursorCol)
	}
}

// TestRenderer_Render_PromptCachedAfterFirstCall verifies that the prompt
// function is called during the first Render and the result is cached in state.
func TestRenderer_Render_PromptCachedAfterFirstCall(t *testing.T) {
	calls := 0
	w := &bufWriter{}
	r := newTestRenderer(w)
	r.config.Prompt = func(_, _ int) string {
		calls++
		return "> "
	}
	_ = r.Render()
	if calls != 1 {
		t.Fatalf("prompt called %d times during first Render, want 1", calls)
	}
	// cached — should not be called again on second render
	_ = r.Render()
	if calls != 1 {
		t.Fatalf("prompt called again on second Render (want cached): calls=%d", calls)
	}
}

// TestRenderer_Render_SequentialBufferChanges does several renders with
// incremental buffer changes and checks cursor column stays consistent.
func TestRenderer_Render_SequentialBufferChanges(t *testing.T) {
	w := &bufWriter{}
	r := newTestRenderer(w)
	r.config.Prompt = func(_, _ int) string { return "" }

	for i, ch := range "abcd" {
		r.editor.Insert(ch)
		w.Reset()
		if err := r.Render(); err != nil {
			t.Fatalf("Render after insert %d error: %v", i, err)
		}
		col, ok := lastColumnEscape(w.String())
		if !ok {
			t.Fatalf("no column escape after insert %d: %q", i, w.String())
		}
		// no prompt, cursor at end: column = (i+1) + 1
		want := i + 2
		if col != want {
			t.Fatalf("after insert %d: column=%d, want %d; output: %q", i, col, want, w.String())
		}
	}
}

// ---- helpers ----------------------------------------------------------------

// newTestRenderer creates a Renderer with a fresh editor, a default config, and
// optionally a custom driver. If driver is nil, a no-op bufWriter is used.
func newTestRenderer(driver Driver) *Renderer {
	ed := editor.New()
	cfg := config.Config{}
	var d Driver
	if driver != nil {
		d = driver
	} else {
		d = &bufWriter{}
	}
	r := New(cfg, ed, d)
	r.SetSize(80, 24)
	return &r
}

// runesLine converts a string into a Line of single-width cells with no style.
func runesLine(s string) Line {
	cells := make(Line, 0, len(s))
	for _, r := range s {
		cells = append(cells, Cell{Rune: r, Width: 1})
	}
	return cells
}

// lineStr converts a Line back to a string (ignoring style/width).
func lineStr(l Line) string {
	var sb strings.Builder
	for _, c := range l {
		switch text := cellText(c); text {
		case "\n", "\r":
			continue
		default:
			sb.WriteString(text)
		}
	}
	return sb.String()
}

var columnEscapeRe = regexp.MustCompile(`\x1b\[(\d+)G`)

// lastColumnEscape extracts the column number from the last \x1b[NG escape in s.
func lastColumnEscape(s string) (int, bool) {
	matches := columnEscapeRe.FindAllStringSubmatch(s, -1)
	if len(matches) == 0 {
		return 0, false
	}
	last := matches[len(matches)-1]
	n, err := strconv.Atoi(last[1])
	if err != nil {
		return 0, false
	}
	return n, true
}

// completionCandidate is a simple name+description pair used in tests.
type completionCandidate struct {
	name, desc string
}

// staticCompleter implements completion.Completer and always returns a fixed
// set of candidates, regardless of the buffer contents.
type staticCompleter struct {
	candidates []completionCandidate
}

func (s staticCompleter) Complete(_ []rune, _ int) []completion.Group {
	out := make([]completion.Candidate, len(s.candidates))
	for i, c := range s.candidates {
		out[i] = completion.Candidate{Name: c.name, Description: c.desc}
	}
	return []completion.Group{
		{
			Candidates: out,
		},
	}
}

func (s staticCompleter) Suggest(_ []rune) []rune { return nil }

type staticSuggester struct {
	suggestion []rune
}

func (s staticSuggester) Suggest(_ []rune) []rune {
	return append([]rune(nil), s.suggestion...)
}

// countEscapeN counts occurrences of \x1b[<N><letter> in s.
func countEscapeN(s, letter string) int {
	re := regexp.MustCompile(`\x1b\[(\d+)` + letter)
	var total int
	for _, m := range re.FindAllStringSubmatch(s, -1) {
		n, _ := strconv.Atoi(m[1])
		total += n
	}
	return total
}

// TestRenderer_Render_HintChangedDoesNotMoveCursorAboveStart reproduces the
// bug where pressing two different unbound keys in vim normal mode moves the
// terminal cursor above the prompt line.
//
// The scenario:
//  1. First render: prompt + buffer, no hint → 1 line, cursorY=0, previousCursorY=0
//  2. Second render: hint set (simulating first unrecognised key) → 2 lines, cursorY=0
//  3. Third render: hint text changed (simulating second unrecognised key) →
//     firstDiffLine=1 (only the hint line changed) but cursorY=0.
//
// In the buggy code, step 3 never moves the terminal cursor DOWN to line 1
// before drawing the changed hint, but still moves UP by 1 at the end to
// reposition the cursor at cursorY=0. This leaves the cursor one row above
// the start of the output area, and the effect compounds with every
// subsequent unrecognised key.
//
// The test verifies that the net vertical cursor movement in the third
// render's output is zero (total ups == total downs), so the cursor stays
// within the output area.
func TestRenderer_Render_HintChangedDoesNotMoveCursorAboveStart(t *testing.T) {
	w := &bufWriter{}
	r := newTestRenderer(w)
	r.config.Prompt = func(_, _ int) string { return "> " }
	r.editor.Insert('a')

	// Render 1: no hint — establishes a single-line previous state.
	if err := r.Render(); err != nil {
		t.Fatalf("render 1 error: %v", err)
	}

	// Render 2: set a hint (first unrecognised key).
	r.editor.SetHint("unrecognised key binding: j")
	if err := r.Render(); err != nil {
		t.Fatalf("render 2 error: %v", err)
	}

	// Render 3: change the hint (second unrecognised key). The prompt+buffer
	// line (index 0) is identical to the previous render; only the hint line
	// (index 1) differs. So firstDiffLine=1 but cursorY=0.
	r.editor.SetHint("unrecognised key binding: k")
	w.Reset()
	if err := r.Render(); err != nil {
		t.Fatalf("render 3 error: %v", err)
	}

	out := w.String()
	ups := countEscapeN(out, "A")
	downs := countEscapeN(out, "B")

	// Net vertical movement must be zero: every cursor-up must be paired with
	// an equal cursor-down, otherwise the cursor drifts above the output area.
	if ups != downs {
		t.Fatalf(
			"render 3: net cursor movement is %d up, %d down (net %d up) — cursor drifted above output area\nraw output: %q",
			ups, downs, ups-downs, out,
		)
	}
}

func TestRenderer_Render_RefreshesStatusLineEveryRender(t *testing.T) {
	w := &bufWriter{}
	r := newTestRenderer(w)
	status := "INSERT"
	r.config.Prompt = func(_, _ int) string { return "> " }
	r.config.StatusLine = func(_, _ int) string { return status }

	if err := r.Render(); err != nil {
		t.Fatalf("render 1 error: %v", err)
	}
	status = "NORMAL"
	w.Reset()
	if err := r.Render(); err != nil {
		t.Fatalf("render 2 error: %v", err)
	}

	got := w.String()
	if !strings.Contains(got, "NORMAL") {
		t.Fatalf("status line was not refreshed: %q", got)
	}
	if strings.Contains(got, "INSERT") {
		t.Fatalf("stale status line was rendered: %q", got)
	}
}
