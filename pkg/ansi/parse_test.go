package ansi

import (
	"testing"
)

func TestParse_PlainText(t *testing.T) {
	runes, spans := Parse("hello")
	if string(runes) != "hello" {
		t.Fatalf("runes = %q, want %q", string(runes), "hello")
	}
	if len(spans) != 0 {
		t.Fatalf("expected no spans, got %d", len(spans))
	}
}

func TestParse_EmptyString(t *testing.T) {
	runes, spans := Parse("")
	if len(runes) != 0 || len(spans) != 0 {
		t.Fatalf("expected empty, got runes=%v spans=%v", runes, spans)
	}
}

func TestParse_BoldAttribute(t *testing.T) {
	runes, spans := Parse("\x1b[1mhi\x1b[0m")
	if string(runes) != "hi" {
		t.Fatalf("runes = %q, want %q", string(runes), "hi")
	}
	if len(spans) != 1 {
		t.Fatalf("expected 1 span, got %d", len(spans))
	}
	if spans[0].Style.Attr&AttrBold == 0 {
		t.Fatal("expected bold attribute")
	}
	if spans[0].Start != 0 || spans[0].End != 2 {
		t.Fatalf("span range = [%d,%d), want [0,2)", spans[0].Start, spans[0].End)
	}
}

func TestParse_MultipleAttributes(t *testing.T) {
	// Bold + italic applied together.
	runes, spans := Parse("\x1b[1;3mhi\x1b[0m")
	if string(runes) != "hi" {
		t.Fatalf("runes = %q", string(runes))
	}
	if len(spans) != 1 {
		t.Fatalf("expected 1 span, got %d", len(spans))
	}
	if spans[0].Style.Attr&AttrBold == 0 {
		t.Error("expected bold")
	}
	if spans[0].Style.Attr&AttrItalic == 0 {
		t.Error("expected italic")
	}
}

func TestParse_16ColourFg(t *testing.T) {
	runes, spans := Parse("\x1b[31mred\x1b[0m")
	if string(runes) != "red" {
		t.Fatalf("runes = %q", string(runes))
	}
	if len(spans) != 1 {
		t.Fatalf("expected 1 span, got %d", len(spans))
	}
	fg := spans[0].Style.Fg
	if fg.Mode != Color16 || fg.Index != 1 {
		t.Fatalf("Fg = %+v, want Color16 index 1", fg)
	}
}

func TestParse_16ColourBg(t *testing.T) {
	_, spans := Parse("\x1b[42mtext\x1b[0m")
	if len(spans) != 1 {
		t.Fatalf("expected 1 span, got %d", len(spans))
	}
	bg := spans[0].Style.Bg
	if bg.Mode != Color16 || bg.Index != 2 {
		t.Fatalf("Bg = %+v, want Color16 index 2", bg)
	}
}

func TestParse_256ColourFg(t *testing.T) {
	_, spans := Parse("\x1b[38;5;200mtext\x1b[0m")
	if len(spans) != 1 {
		t.Fatalf("expected 1 span, got %d", len(spans))
	}
	fg := spans[0].Style.Fg
	if fg.Mode != Color256 || fg.Index != 200 {
		t.Fatalf("Fg = %+v, want Color256 index 200", fg)
	}
}

func TestParse_RGBColourFg(t *testing.T) {
	_, spans := Parse("\x1b[38;2;10;20;30mtext\x1b[0m")
	if len(spans) != 1 {
		t.Fatalf("expected 1 span, got %d", len(spans))
	}
	fg := spans[0].Style.Fg
	if fg.Mode != ColorRGB || fg.R != 10 || fg.G != 20 || fg.B != 30 {
		t.Fatalf("Fg = %+v, want RGB(10,20,30)", fg)
	}
}

func TestParse_RGBColourBg(t *testing.T) {
	_, spans := Parse("\x1b[48;2;1;2;3mtext\x1b[0m")
	if len(spans) != 1 {
		t.Fatalf("expected 1 span")
	}
	bg := spans[0].Style.Bg
	if bg.Mode != ColorRGB || bg.R != 1 || bg.G != 2 || bg.B != 3 {
		t.Fatalf("Bg = %+v, want RGB(1,2,3)", bg)
	}
}

func TestParse_ResetMidString(t *testing.T) {
	// Bold "ab", then reset, then plain "cd".
	runes, spans := Parse("\x1b[1mab\x1b[0mcd")
	if string(runes) != "abcd" {
		t.Fatalf("runes = %q", string(runes))
	}
	// Only the first two runes should be spanned.
	if len(spans) != 1 {
		t.Fatalf("expected 1 span, got %d", len(spans))
	}
	if spans[0].End != 2 {
		t.Fatalf("span end = %d, want 2", spans[0].End)
	}
}

func TestParse_AdjacentSpans(t *testing.T) {
	// Red "a", then blue "b".
	runes, spans := Parse("\x1b[31ma\x1b[34mb\x1b[0m")
	if string(runes) != "ab" {
		t.Fatalf("runes = %q", string(runes))
	}
	if len(spans) != 2 {
		t.Fatalf("expected 2 spans, got %d", len(spans))
	}
	if spans[0].Style.Fg.Index != 1 || spans[1].Style.Fg.Index != 4 {
		t.Fatalf("spans fg indices = %d,%d, want 1,4", spans[0].Style.Fg.Index, spans[1].Style.Fg.Index)
	}
}

func TestParse_NonSGREscapesIgnored(t *testing.T) {
	// ESC[A (cursor up) is not SGR — should be consumed silently.
	runes, spans := Parse("\x1b[Ahi")
	if string(runes) != "hi" {
		t.Fatalf("runes = %q, want %q", string(runes), "hi")
	}
	if len(spans) != 0 {
		t.Fatalf("expected no spans, got %d", len(spans))
	}
}

func TestParse_BareReset(t *testing.T) {
	// ESC[m with no params is a reset.
	runes, spans := Parse("\x1b[1m\x1b[mafter")
	if string(runes) != "after" {
		t.Fatalf("runes = %q", string(runes))
	}
	if len(spans) != 0 {
		t.Fatalf("expected no spans after bare reset, got %d", len(spans))
	}
}

func TestParse_HighColour16(t *testing.T) {
	// Bright red fg (90–97 range → index 8–15).
	_, spans := Parse("\x1b[91mtext\x1b[0m")
	if len(spans) != 1 {
		t.Fatalf("expected 1 span, got %d", len(spans))
	}
	fg := spans[0].Style.Fg
	if fg.Mode != Color16 || fg.Index != 9 {
		t.Fatalf("Fg = %+v, want Color16 index 9 (bright red)", fg)
	}
}

func TestParse_256ColourBg(t *testing.T) {
	_, spans := Parse("\x1b[48;5;100mtext\x1b[0m")
	if len(spans) != 1 {
		t.Fatalf("expected 1 span, got %d", len(spans))
	}
	bg := spans[0].Style.Bg
	if bg.Mode != Color256 || bg.Index != 100 {
		t.Fatalf("Bg = %+v, want Color256 index 100", bg)
	}
}

func TestParse_ClearBoldDim(t *testing.T) {
	// Set bold, then clear it with SGR 22.
	_, spans := Parse("\x1b[1mab\x1b[22mcd\x1b[0m")
	// Only "ab" should be bold; "cd" should not.
	if len(spans) != 1 {
		t.Fatalf("expected 1 bold span, got %d", len(spans))
	}
	if spans[0].End != 2 {
		t.Fatalf("bold span end = %d, want 2", spans[0].End)
	}
}

func TestParse_ClearItalic(t *testing.T) {
	_, spans := Parse("\x1b[3mab\x1b[23mcd\x1b[0m")
	if len(spans) != 1 {
		t.Fatalf("expected 1 italic span, got %d", len(spans))
	}
	if spans[0].Style.Attr&AttrItalic == 0 {
		t.Fatal("expected italic attr in span")
	}
	if spans[0].End != 2 {
		t.Fatalf("italic span end = %d, want 2", spans[0].End)
	}
}

func TestParse_ClearUnderline(t *testing.T) {
	_, spans := Parse("\x1b[4mab\x1b[24mcd\x1b[0m")
	if len(spans) != 1 {
		t.Fatalf("expected 1 underline span, got %d", len(spans))
	}
	if spans[0].End != 2 {
		t.Fatalf("underline span end = %d, want 2", spans[0].End)
	}
}

func TestParse_ClearReverse(t *testing.T) {
	_, spans := Parse("\x1b[7mab\x1b[27mcd\x1b[0m")
	if len(spans) != 1 {
		t.Fatalf("expected 1 reverse span, got %d", len(spans))
	}
	if spans[0].End != 2 {
		t.Fatalf("reverse span end = %d, want 2", spans[0].End)
	}
}

func TestParse_ClearStrike(t *testing.T) {
	_, spans := Parse("\x1b[9mab\x1b[29mcd\x1b[0m")
	if len(spans) != 1 {
		t.Fatalf("expected 1 strike span, got %d", len(spans))
	}
	if spans[0].End != 2 {
		t.Fatalf("strike span end = %d, want 2", spans[0].End)
	}
}

func TestParse_ResetFgColor(t *testing.T) {
	// Set red fg, then reset to default with SGR 39.
	_, spans := Parse("\x1b[31mab\x1b[39mcd\x1b[0m")
	if len(spans) != 1 {
		t.Fatalf("expected 1 coloured span, got %d", len(spans))
	}
	if spans[0].End != 2 {
		t.Fatalf("fg span end = %d, want 2", spans[0].End)
	}
}

func TestParse_ResetBgColor(t *testing.T) {
	// Set green bg, then reset to default with SGR 49.
	_, spans := Parse("\x1b[42mab\x1b[49mcd\x1b[0m")
	if len(spans) != 1 {
		t.Fatalf("expected 1 bg span, got %d", len(spans))
	}
	if spans[0].End != 2 {
		t.Fatalf("bg span end = %d, want 2", spans[0].End)
	}
}

func TestParse_HighBgColour(t *testing.T) {
	// Bright green bg (100–107 range → index 8–15).
	_, spans := Parse("\x1b[102mtext\x1b[0m")
	if len(spans) != 1 {
		t.Fatalf("expected 1 span, got %d", len(spans))
	}
	bg := spans[0].Style.Bg
	if bg.Mode != Color16 || bg.Index != 10 {
		t.Fatalf("Bg = %+v, want Color16 index 10 (bright green)", bg)
	}
}

func TestParse_DimAttribute(t *testing.T) {
	_, spans := Parse("\x1b[2mtext\x1b[0m")
	if len(spans) != 1 {
		t.Fatalf("expected 1 span, got %d", len(spans))
	}
	if spans[0].Style.Attr&AttrDim == 0 {
		t.Fatal("expected dim attribute")
	}
}

func TestParse_UnderlineAttribute(t *testing.T) {
	_, spans := Parse("\x1b[4mtext\x1b[0m")
	if len(spans) != 1 {
		t.Fatalf("expected 1 span, got %d", len(spans))
	}
	if spans[0].Style.Attr&AttrUnderline == 0 {
		t.Fatal("expected underline attribute")
	}
}

func TestParse_ReverseAttribute(t *testing.T) {
	_, spans := Parse("\x1b[7mtext\x1b[0m")
	if len(spans) != 1 {
		t.Fatalf("expected 1 span, got %d", len(spans))
	}
	if spans[0].Style.Attr&AttrReverse == 0 {
		t.Fatal("expected reverse attribute")
	}
}

func TestParse_StrikeAttribute(t *testing.T) {
	_, spans := Parse("\x1b[9mtext\x1b[0m")
	if len(spans) != 1 {
		t.Fatalf("expected 1 span, got %d", len(spans))
	}
	if spans[0].Style.Attr&AttrStrike == 0 {
		t.Fatal("expected strike attribute")
	}
}
