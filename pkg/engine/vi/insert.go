package vi

import (
	"fmt"

	"github.com/liamg/readline/pkg/engine"
	"github.com/liamg/readline/pkg/engine/shared"
	"github.com/liamg/readline/pkg/keymap"
	"github.com/liamg/readline/pkg/terminal"
)

func (v *Vi) buildInsertKeymap() *engine.Keymap {
	return &engine.Keymap{
		Cursor: engine.CursorBar,
		Bindings: append([]engine.Binding{
			{Sequence: keymap.Sequence{{Key: terminal.KeyBackspace}}, Action: engine.DeletePrevious},
			{Sequence: keymap.Sequence{{Key: terminal.KeyRune, Rune: 'h', Mod: terminal.ModCtrl}}, Action: engine.DeletePrevious},
			{Sequence: keymap.Sequence{{Key: terminal.KeyRune, Rune: 'u', Mod: terminal.ModCtrl}}, Action: v.deleteToBeginningOfLine()},
			{Sequence: keymap.Sequence{{Key: terminal.KeyRune, Rune: 'w', Mod: terminal.ModCtrl}}, Action: v.deleteWord()},
			{Sequence: keymap.MustParseSequence("escape"), Action: v.actionSwitchToNormalMode()},
		}, shared.Bindings...),
		Fallback: func(ctx *engine.ActionContext, ev terminal.KeyEvent) (engine.ActionResult, error) {
			if ev.Key == terminal.KeyRune && ev.Mod == 0 {
				ctx.Editor.Insert(ev.Rune)
				return engine.ActionResult{}, nil
			}
			return engine.ActionResult{}, fmt.Errorf("%w: %s", engine.ErrUnrecognisedBinding, ev)
		},
	}
}

func (v *Vi) deleteWord() *engine.Action {
	return &engine.Action{
		Name: "vi-delete-to-beginning-of-previous-word",
		Func: func(ac *engine.ActionContext) (engine.ActionResult, error) {
			buffer := ac.Editor.Buffer()
			before := buffer[:ac.Editor.Cursor()]

			lastWhitespace := len(before)
			for i := len(before) - 1; i >= 0; i-- {
				if isWhitespaceChar(before[i]) {
					lastWhitespace = i
					continue
				}
				break
			}
			before = before[:lastWhitespace]
			start, end := findWord(before, len(before), true)
			if start != end {
				before = before[:start]
			}

			after := buffer[ac.Editor.Cursor():]
			ac.Editor.SetBuffer(append(before, after...))

			return engine.ActionResult{}, nil
		},
	}
}

func (v *Vi) deleteToBeginningOfLine() *engine.Action {
	return &engine.Action{
		Name: "vi-delete-to-beginning-of-line",
		Func: func(ac *engine.ActionContext) (engine.ActionResult, error) {
			buffer := ac.Editor.Buffer()
			before := buffer[:ac.Editor.Cursor()]
			after := buffer[ac.Editor.Cursor():]
			lastIndex := lastIndex(before, '\n')
			if lastIndex == -1 {
				ac.Editor.SetBuffer(after)
			} else {
				ac.Editor.SetBuffer(append(before[:lastIndex+1], after...))
			}
			return engine.ActionResult{}, nil
		},
	}
}

func lastIndex[S ~[]E, E comparable](s S, v E) int {
	for i := len(s) - 1; i >= 0; i-- {
		if s[i] == v {
			return i
		}
	}
	return -1
}

func findWord(input []rune, at int, backward bool) (start, end int) {
	// we're at the start, there's nothing before
	if at == 0 && backward {
		return 0, 0
	}

	inc := 1
	until := len(input) - 1

	if !backward && at > until {
		return 0, 0
	}

	if backward {
		inc = -1
		at = at - 1
		until = 0
	}

	var matcher func(rune) bool

	for i := at; ; i += inc {

		r := input[i]

		switch {
		case matcher != nil:
			if !matcher(r) {
				end = i - inc
				if end < start {
					start, end = end, start
				}
				return
			}
		case isPunctuationChar(r):
			matcher = isPunctuationChar
			start = i
		case isWordChar(r):
			matcher = isWordChar
			start = i
		default:
			return 0, 0
		}

		if i == until {
			break
		}
	}

	end = until
	if end < start {
		start, end = end, start
	}
	return start, end
}

func isWhitespaceChar(r rune) bool {
	return r == ' ' || r == '\t'
}

func isWordChar(r rune) bool {
	return (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') ||
		(r >= '0' && r <= '9') || r == '_'
}

func isPunctuationChar(r rune) bool {
	switch r {
	case '!', '@', '#', '$', '%', '^', '&', '*', '(', ')':
		return true
	default:
		return false
	}
}
