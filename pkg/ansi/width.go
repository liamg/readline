package ansi

import "github.com/rivo/uniseg"

// RuneWidth returns the number of terminal columns needed to display a single
// rune r in isolation. Returns 2 for East Asian wide characters, 0 for
// control/combining characters, and 1 for everything else.
//
// RuneWidth measures one code point at a time and therefore cannot account for
// grapheme clusters that span multiple runes (ZWJ emoji sequences, flags, emoji
// with skin-tone or variation-selector modifiers). Use CellWidth to measure a
// string correctly; reach for RuneWidth only when you genuinely have a lone
// rune to place in a single cell.
func RuneWidth(r rune) int {
	switch {
	case r < 0x20 || (r >= 0x7f && r < 0xa0):
		return 0 // control characters
	case isWide(r):
		return 2
	default:
		return 1
	}
}

// CellWidth returns the number of terminal columns needed to display s,
// measuring by grapheme cluster so that multi-rune emoji, flags, and combining
// sequences count as a single (typically double-width) unit. It does not strip
// ANSI escape sequences — use VisibleWidth for text that may contain them.
func CellWidth(s string) int {
	w := 0
	g := uniseg.NewGraphemes(s)
	for g.Next() {
		w += g.Width()
	}
	return w
}

// VisibleWidth returns the number of terminal columns needed to display s,
// stripping any ANSI escape sequences before measuring. The visible remainder
// is measured with CellWidth, so grapheme clusters are counted correctly.
func VisibleWidth(s string) int {
	runes := []rune(s)
	visible := make([]rune, 0, len(runes))
	i := 0
	for i < len(runes) {
		r := runes[i]
		// CSI: ESC [ ... letter
		if r == '\x1b' && i+1 < len(runes) && runes[i+1] == '[' {
			i += 2
			for i < len(runes) {
				c := runes[i]
				i++
				if (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') {
					break
				}
			}
			continue
		}
		// Other escape sequences: ESC + one char
		if r == '\x1b' && i+1 < len(runes) {
			i += 2
			continue
		}
		visible = append(visible, r)
		i++
	}
	return CellWidth(string(visible))
}

// isWide reports whether r is an East Asian wide or fullwidth character.
func isWide(r rune) bool {
	if r < 0x1100 {
		return false
	}
	switch {
	case r <= 0x115F,
		r == 0x2329, r == 0x232A,
		r >= 0x2E80 && r <= 0x303E,
		r >= 0x3040 && r <= 0x33FF,
		r >= 0x3400 && r <= 0x4DBF,
		r >= 0x4E00 && r <= 0xA4CF,
		r >= 0xA960 && r <= 0xA97F,
		r >= 0xAC00 && r <= 0xD7FF,
		r >= 0xF900 && r <= 0xFAFF,
		r >= 0xFE10 && r <= 0xFE19,
		r >= 0xFE30 && r <= 0xFE6F,
		r >= 0xFF01 && r <= 0xFF60,
		r >= 0xFFE0 && r <= 0xFFE6,
		r >= 0x1B000 && r <= 0x1B0FF,
		r >= 0x1F004 && r <= 0x1F9FF,
		r >= 0x20000 && r <= 0x2FFFD,
		r >= 0x30000 && r <= 0x3FFFD:
		return true
	}
	return false
}
