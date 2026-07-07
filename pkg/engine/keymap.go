package engine

import (
	"github.com/liamg/readline/pkg/keymap"
	"github.com/liamg/readline/pkg/terminal"
)

// CursorStyle controls the terminal cursor shape for a keymap.
type CursorStyle uint8

const (
	CursorDefault   CursorStyle = iota // leave cursor as-is (terminal default)
	CursorBlock                        // steady block (▋) — typical for normal mode
	CursorBar                          // steady bar/caret (|) — typical for insert mode
	CursorUnderline                    // steady underline (_)
)

// Keymap is a named set of key bindings with an optional fallback for keys
// that don't match any binding.
type Keymap struct {
	Cursor   CursorStyle
	Bindings []Binding
	// Fallback is called when no binding matches the current key event.
	// If nil, unrecognised keys produce ErrUnrecognisedBinding.
	// Insert-mode keymaps typically set this to a rune-insertion handler.
	Fallback func(*ActionContext, terminal.KeyEvent) (ActionResult, error)
}

// Binding associates a key sequence with an action.
type Binding struct {
	Sequence   keymap.Sequence
	Action     *Action
	GroupTitle *string // optional display grouping hint
}
