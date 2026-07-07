package keymap

import (
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/liamg/readline/pkg/terminal"
)

// Sequence is an ordered list of key events that form a key binding, e.g.
// ctrl-x followed by ctrl-u.
type Sequence []terminal.KeyEvent

func (s Sequence) Equal(other Sequence) bool {
	if len(s) != len(other) {
		return false
	}
	for i, ev := range s {
		if ev != other[i] {
			return false
		}
	}
	return true
}

func (s Sequence) Matches(events []terminal.KeyEvent) (match bool, complete bool) {
	if len(events) == 0 {
		return true, false // no input
	}
	for i, ev := range s {
		if i >= len(events) {
			return true, false // incomplete match
		}
		if ev != events[i] {
			return false, false // no match
		}
	}
	return true, true // complete match
}

// ParseSequence parses a human-readable key sequence string into a Sequence.
// Keys are comma-separated for multi-key chords, e.g. "ctrl-x,ctrl-u".
// Modifiers are hyphen-prefixed and may be stacked: ctrl, alt, shift, meta.
//
// Examples:
//
//	"ctrl-c"         single ctrl+c
//	"alt-left"       alt+left arrow
//	"shift-f3"       shift+F3
//	"ctrl-x,u"       ctrl-x then u (two-key chord)
//	"ctrl-x,ctrl-u"  ctrl-x then ctrl-u
func ParseSequence(s string) (Sequence, error) {
	parts := strings.Split(s, ",")
	seq := make(Sequence, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			return nil, fmt.Errorf("empty key in sequence %q - use \"comma\" instead of a literal \",\"", s)
		}
		ev, err := parseKeyEvent(part)
		if err != nil {
			return nil, err
		}
		seq = append(seq, ev)
	}
	if len(seq) == 0 {
		return nil, fmt.Errorf("empty key sequence %q", s)
	}
	return seq, nil
}

func MustParseSequence(s string) Sequence {
	seq, err := ParseSequence(s)
	if err != nil {
		panic(fmt.Sprintf("invalid key sequence %q: %v", s, err))
	}
	return seq
}

func parseKeyEvent(s string) (terminal.KeyEvent, error) {
	var mod terminal.Modifier
	lower := strings.ToLower(s)
	for {
		var consumed int
		switch {
		case strings.HasPrefix(lower, "ctrl-"):
			mod |= terminal.ModCtrl
			consumed = 5
		case strings.HasPrefix(lower, "alt-"):
			mod |= terminal.ModAlt
			consumed = 4
		case strings.HasPrefix(lower, "shift-"):
			mod |= terminal.ModShift
			consumed = 6
		case strings.HasPrefix(lower, "meta-"):
			mod |= terminal.ModMeta
			consumed = 5
		}
		if consumed == 0 {
			break
		}
		s = s[consumed:]
		lower = lower[consumed:]
	}

	if key, ok := namedKeys[lower]; ok {
		return terminal.KeyEvent{Key: key, Rune: 0, Mod: mod}, nil
	}
	switch lower {
	case "space":
		return terminal.KeyEvent{Key: terminal.KeyRune, Rune: ' ', Mod: mod}, nil
	case "comma":
		return terminal.KeyEvent{Key: terminal.KeyRune, Rune: ',', Mod: mod}, nil

	}
	r, size := utf8.DecodeRuneInString(s)
	if r != utf8.RuneError && size == len(s) {
		return terminal.KeyEvent{Key: terminal.KeyRune, Rune: r, Mod: mod}, nil
	}
	return terminal.KeyEvent{}, fmt.Errorf("unknown key %q", s)
}

var namedKeys = map[string]terminal.Key{
	"up":        terminal.KeyUp,
	"down":      terminal.KeyDown,
	"left":      terminal.KeyLeft,
	"right":     terminal.KeyRight,
	"home":      terminal.KeyHome,
	"end":       terminal.KeyEnd,
	"pageup":    terminal.KeyPageUp,
	"page-up":   terminal.KeyPageUp,
	"pagedown":  terminal.KeyPageDown,
	"page-down": terminal.KeyPageDown,
	"insert":    terminal.KeyInsert,
	"delete":    terminal.KeyDelete,
	"backspace": terminal.KeyBackspace,
	"enter":     terminal.KeyEnter,
	"tab":       terminal.KeyTab,
	"escape":    terminal.KeyEscape,
	"esc":       terminal.KeyEscape,
	"f1":        terminal.KeyF1,
	"f2":        terminal.KeyF2,
	"f3":        terminal.KeyF3,
	"f4":        terminal.KeyF4,
	"f5":        terminal.KeyF5,
	"f6":        terminal.KeyF6,
	"f7":        terminal.KeyF7,
	"f8":        terminal.KeyF8,
	"f9":        terminal.KeyF9,
	"f10":       terminal.KeyF10,
	"f11":       terminal.KeyF11,
	"f12":       terminal.KeyF12,
}

var keysToName = make(map[terminal.Key]string, len(namedKeys))

func init() {
	for k, v := range namedKeys {
		keysToName[v] = k
	}
}

var allMods = []terminal.Modifier{
	terminal.ModMeta,
	terminal.ModCtrl,
	terminal.ModAlt,
	terminal.ModShift,
}

var modNames = map[terminal.Modifier]string{
	terminal.ModMeta:  "meta",
	terminal.ModCtrl:  "ctrl",
	terminal.ModAlt:   "alt",
	terminal.ModShift: "shift",
}

var runeNames = map[rune]string{
	' ':  "space",
	'\t': "tab",
}

func keyEventToString(sb *strings.Builder, evt terminal.KeyEvent) {
	for _, mod := range allMods {
		if evt.Mod&mod != 0 {
			sb.WriteString(modNames[mod])
			sb.WriteRune('-')
		}
	}
	// Both KeyRune and KeyNone-with-rune (ctrl-letter) write the rune.
	if evt.Key == terminal.KeyRune || (evt.Key == terminal.KeyNone && evt.Rune != 0) {
		if name, ok := runeNames[evt.Rune]; ok {
			sb.WriteString(name)
			return
		}
		sb.WriteRune(evt.Rune)
		return
	}

	if name, ok := keysToName[evt.Key]; ok {
		sb.WriteString(name)
		return
	}

	sb.WriteString("<unknown>")
}

func (s Sequence) String() string {
	var sb strings.Builder
	for i, k := range s {
		if i > 0 {
			sb.WriteRune(',')
		}
		keyEventToString(&sb, k)
	}
	return sb.String()
}
