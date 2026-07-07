package vi

import (
	"fmt"

	"github.com/liamg/readline/pkg/engine"
	"github.com/liamg/readline/pkg/engine/shared"
	"github.com/liamg/readline/pkg/keymap"
	"github.com/liamg/readline/pkg/terminal"
)

func (v *Vi) buildVisualKeymap() *engine.Keymap {
	return &engine.Keymap{
		Cursor: engine.CursorBlock,
		Bindings: append([]engine.Binding{
			{Sequence: keymap.MustParseSequence("escape"), Action: v.actionSwitchToNormalMode()},
			{Sequence: keymap.MustParseSequence("h"), Action: v.actionMoveLeft()},
			{Sequence: keymap.MustParseSequence("l"), Action: v.actionMoveRight()},
			{Sequence: keymap.MustParseSequence("0"), Action: v.actionZero()},
			{Sequence: keymap.MustParseSequence("$"), Action: engine.EndOfLine},
			{Sequence: keymap.MustParseSequence("^"), Action: v.moveToFirstNonBlank()},
			{Sequence: keymap.MustParseSequence("w"), Action: v.moveToStartOfNextWordForward()},
			{Sequence: keymap.MustParseSequence("b"), Action: v.moveToStartOfNextWordBackward()},
			{Sequence: keymap.MustParseSequence("e"), Action: v.moveToEndOfNextWordForward()},
			{Sequence: keymap.MustParseSequence("W"), Action: v.moveToStartOfNextNonWhitespaceForward()},
			{Sequence: keymap.MustParseSequence("B"), Action: v.moveToStartOfNextNonWhitespaceBackward()},
			{Sequence: keymap.MustParseSequence("E"), Action: v.moveToEndOfNextNonWhitespaceForward()},
			{Sequence: keymap.MustParseSequence("f"), Action: v.findCharacterForward()},
			{Sequence: keymap.MustParseSequence("F"), Action: v.findCharacterBackward()},
			{Sequence: keymap.MustParseSequence("t"), Action: v.untilCharacterForward()},
			{Sequence: keymap.MustParseSequence("T"), Action: v.untilCharacterBackward()},
			{Sequence: keymap.MustParseSequence("\""), Action: v.actionSetRegister()},
			{Sequence: keymap.MustParseSequence("y"), Action: v.actionYank()},
			{Sequence: keymap.MustParseSequence("d"), Action: v.actionDelete()},
			{Sequence: keymap.MustParseSequence("x"), Action: v.actionDelete()},
			{Sequence: keymap.MustParseSequence("c"), Action: v.actionChange()},
			{Sequence: keymap.MustParseSequence("~"), Action: v.actionToggleSelectionCase()},
		}, shared.Bindings...),
		Fallback: func(ctx *engine.ActionContext, ev terminal.KeyEvent) (engine.ActionResult, error) {
			if v.handleCountOrFallback(ev) {
				return engine.ActionResult{
					SkipReset: true,
				}, nil
			}
			return engine.ActionResult{}, fmt.Errorf("%w: %s", engine.ErrUnrecognisedBinding, ev)
		},
	}
}
