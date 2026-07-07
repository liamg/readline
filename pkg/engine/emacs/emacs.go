package emacs

import (
	"fmt"

	"github.com/liamg/readline/pkg/editor"
	"github.com/liamg/readline/pkg/engine"
	"github.com/liamg/readline/pkg/engine/shared"
	"github.com/liamg/readline/pkg/history"
	"github.com/liamg/readline/pkg/keymap"
	"github.com/liamg/readline/pkg/terminal"
)

// BuildKeymaps returns the default Emacs keymaps before customisation.
func BuildKeymaps() map[string]*engine.Keymap {
	return map[string]*engine.Keymap{"emacs": {
		Bindings: append([]engine.Binding{
			// Deletion
			{Sequence: keymap.Sequence{{Key: terminal.KeyBackspace}}, Action: engine.DeletePrevious},
			{Sequence: keymap.MustParseSequence("ctrl-d"), Action: engine.DeleteNext},
			{Sequence: keymap.MustParseSequence("ctrl-k"), Action: KillLine},
			{Sequence: keymap.MustParseSequence("ctrl-u"), Action: BackwardKillLine},
			{Sequence: keymap.MustParseSequence("ctrl-w"), Action: BackwardKillWord},
			{Sequence: keymap.MustParseSequence("alt-d"), Action: KillWord},
			{Sequence: keymap.MustParseSequence("alt-backspace"), Action: BackwardKillWord},
			// Movement
			{Sequence: keymap.MustParseSequence("ctrl-a"), Action: engine.BeginningOfLine},
			{Sequence: keymap.MustParseSequence("ctrl-e"), Action: engine.EndOfLine},
			{Sequence: keymap.MustParseSequence("ctrl-f"), Action: engine.Forward},
			{Sequence: keymap.MustParseSequence("ctrl-b"), Action: engine.Back},
			{Sequence: keymap.MustParseSequence("alt-f"), Action: ForwardWord},
			{Sequence: keymap.MustParseSequence("alt-b"), Action: BackwardWord},
			{Sequence: keymap.MustParseSequence("ctrl-left"), Action: BackwardWord},
			{Sequence: keymap.MustParseSequence("ctrl-right"), Action: ForwardWord},
			// History
			{Sequence: keymap.MustParseSequence("ctrl-p"), Action: engine.HistoryPrevious},
			{Sequence: keymap.MustParseSequence("ctrl-n"), Action: engine.HistoryNext},
			// Editing
			{Sequence: keymap.MustParseSequence("ctrl-t"), Action: TransposeChars},
			{Sequence: keymap.MustParseSequence("ctrl-y"), Action: Yank},
		}, shared.Bindings...),
		Fallback: func(ctx *engine.ActionContext, ev terminal.KeyEvent) (engine.ActionResult, error) {
			if ev.Key == terminal.KeyRune && ev.Mod == 0 {
				ctx.Editor.Insert(ev.Rune)
				return engine.ActionResult{}, nil
			}
			return engine.ActionResult{}, fmt.Errorf("%w: %s", engine.ErrUnrecognisedBinding, ev)
		},
	}}
}

// NewEngine returns an Engine configured for Emacs-style line editing.
func NewEngine(ed *editor.Editor, history history.History) *engine.Engine {
	return NewEngineWithKeymaps(ed, history, BuildKeymaps())
}

// NewEngineWithKeymaps returns an Emacs engine using the supplied keymaps.
func NewEngineWithKeymaps(ed *editor.Editor, history history.History, keymaps map[string]*engine.Keymap) *engine.Engine {
	return engine.New(ed, history, keymaps, "emacs", func() {})
}
