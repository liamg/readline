package ansi

// ColorMode describes how a Color value is encoded.
type ColorMode uint8

const (
	ColorDefault ColorMode = iota // use the terminal's default colour
	Color16                       // one of the 16 standard ANSI colours (Index 0–15)
	Color256                      // 256-colour palette (Index 0–255)
	ColorRGB                      // 24-bit RGB
)

// Color describes a terminal foreground or background colour.
type Color struct {
	Mode    ColorMode
	Index   uint8 // Color16 or Color256
	R, G, B uint8 // ColorRGB
}

// Attr is a bitmask of text attributes.
type Attr uint8

const (
	AttrBold Attr = 1 << iota
	AttrDim
	AttrItalic
	AttrUnderline
	AttrReverse
	AttrStrike
)

// Style combines foreground colour, background colour, and text attributes.
type Style struct {
	Fg   Color
	Bg   Color
	Attr Attr
}
