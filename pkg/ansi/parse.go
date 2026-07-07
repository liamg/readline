package ansi

import (
	"strconv"
	"strings"
)

// Parse extracts plain text and styling spans from an ANSI-escaped string.
// Only SGR (Select Graphic Rendition, ESC [ ... m) sequences are interpreted;
// all other escape sequences are silently consumed.
// Returned spans are non-overlapping and sorted by Start.
func Parse(s string) ([]rune, []Span) {
	var plain []rune
	var spans []Span

	runes := []rune(s)
	i := 0
	current := Style{}
	spanStart := 0

	flush := func() {
		end := len(plain)
		if end > spanStart && current != (Style{}) {
			spans = append(spans, Span{Start: spanStart, End: end, Style: current})
		}
		spanStart = end
	}

	for i < len(runes) {
		r := runes[i]

		// CSI sequence: ESC [ params final
		if r == '\x1b' && i+1 < len(runes) && runes[i+1] == '[' {
			i += 2
			paramStart := i
			for i < len(runes) {
				c := runes[i]
				if (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') {
					break
				}
				i++
			}
			if i < len(runes) && runes[i] == 'm' {
				flush()
				current = applySGR(current, string(runes[paramStart:i]))
			}
			if i < len(runes) {
				i++ // consume final byte
			}
			continue
		}

		// Other escape sequences: skip ESC + one char
		if r == '\x1b' && i+1 < len(runes) {
			i += 2
			continue
		}

		plain = append(plain, r)
		i++
	}

	flush()
	return plain, spans
}

// applySGR applies a semicolon-separated SGR parameter string to s and returns
// the updated Style. Handles 4-bit, 8-bit (38;5;n / 48;5;n), and 24-bit
// (38;2;r;g;b / 48;2;r;g;b) colour encodings.
func applySGR(s Style, params string) Style {
	if params == "" {
		return Style{} // bare ESC[m == reset
	}
	parts := strings.Split(params, ";")
	i := 0
	for i < len(parts) {
		n, err := strconv.Atoi(parts[i])
		if err != nil {
			i++
			continue
		}
		switch {
		case n == 0:
			s = Style{}
		case n == 1:
			s.Attr |= AttrBold
		case n == 2:
			s.Attr |= AttrDim
		case n == 3:
			s.Attr |= AttrItalic
		case n == 4:
			s.Attr |= AttrUnderline
		case n == 7:
			s.Attr |= AttrReverse
		case n == 9:
			s.Attr |= AttrStrike
		case n == 22:
			s.Attr &^= AttrBold | AttrDim
		case n == 23:
			s.Attr &^= AttrItalic
		case n == 24:
			s.Attr &^= AttrUnderline
		case n == 27:
			s.Attr &^= AttrReverse
		case n == 29:
			s.Attr &^= AttrStrike
		case n == 39:
			s.Fg = Color{}
		case n == 49:
			s.Bg = Color{}
		case n >= 30 && n <= 37:
			s.Fg = Color{Mode: Color16, Index: uint8(n - 30)}
		case n >= 40 && n <= 47:
			s.Bg = Color{Mode: Color16, Index: uint8(n - 40)}
		case n >= 90 && n <= 97:
			s.Fg = Color{Mode: Color16, Index: uint8(n - 90 + 8)}
		case n >= 100 && n <= 107:
			s.Bg = Color{Mode: Color16, Index: uint8(n - 100 + 8)}
		case n == 38 && i+1 < len(parts):
			switch parts[i+1] {
			case "5":
				if i+2 < len(parts) {
					idx, _ := strconv.Atoi(parts[i+2])
					s.Fg = Color{Mode: Color256, Index: uint8(idx)}
					i += 2
				}
			case "2":
				if i+4 < len(parts) {
					rv, _ := strconv.Atoi(parts[i+2])
					gv, _ := strconv.Atoi(parts[i+3])
					bv, _ := strconv.Atoi(parts[i+4])
					s.Fg = Color{Mode: ColorRGB, R: uint8(rv), G: uint8(gv), B: uint8(bv)}
					i += 4
				}
			}
		case n == 48 && i+1 < len(parts):
			switch parts[i+1] {
			case "5":
				if i+2 < len(parts) {
					idx, _ := strconv.Atoi(parts[i+2])
					s.Bg = Color{Mode: Color256, Index: uint8(idx)}
					i += 2
				}
			case "2":
				if i+4 < len(parts) {
					rv, _ := strconv.Atoi(parts[i+2])
					gv, _ := strconv.Atoi(parts[i+3])
					bv, _ := strconv.Atoi(parts[i+4])
					s.Bg = Color{Mode: ColorRGB, R: uint8(rv), G: uint8(gv), B: uint8(bv)}
					i += 4
				}
			}
		}
		i++
	}
	return s
}
