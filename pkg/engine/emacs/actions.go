package emacs

import "github.com/liamg/readline/pkg/engine"

// isWordChar reports whether r is a word constituent for emacs word-motion purposes.
func isWordChar(r rune) bool {
	return (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') ||
		(r >= '0' && r <= '9') || r == '_'
}

var (
	ForwardWord = &engine.Action{
		Name: "forward-word",
		Func: func(c *engine.ActionContext) (engine.ActionResult, error) {
			runes := c.Editor.Runes()
			pos := c.Editor.Cursor()
			for pos < len(runes) && !isWordChar(runes[pos]) {
				pos++
			}
			for pos < len(runes) && isWordChar(runes[pos]) {
				pos++
			}
			c.Editor.MoveCursor(pos - c.Editor.Cursor())
			return engine.ActionResult{}, nil
		},
	}
	BackwardWord = &engine.Action{
		Name: "backward-word",
		Func: func(c *engine.ActionContext) (engine.ActionResult, error) {
			runes := c.Editor.Runes()
			pos := c.Editor.Cursor()
			for pos > 0 && !isWordChar(runes[pos-1]) {
				pos--
			}
			for pos > 0 && isWordChar(runes[pos-1]) {
				pos--
			}
			c.Editor.MoveCursor(pos - c.Editor.Cursor())
			return engine.ActionResult{}, nil
		},
	}
	KillLine = &engine.Action{
		Name: "kill-line",
		Func: func(c *engine.ActionContext) (engine.ActionResult, error) {
			killed := engine.DeleteRange(c, c.Editor.Cursor(), len(c.Editor.Runes()))
			c.KillRing.Push(killed)
			return engine.ActionResult{}, nil
		},
	}
	BackwardKillLine = &engine.Action{
		Name: "backward-kill-line",
		Func: func(c *engine.ActionContext) (engine.ActionResult, error) {
			killed := engine.DeleteRange(c, 0, c.Editor.Cursor())
			c.KillRing.Push(killed)
			return engine.ActionResult{}, nil
		},
	}
	KillWord = &engine.Action{
		Name: "kill-word",
		Func: func(c *engine.ActionContext) (engine.ActionResult, error) {
			runes := c.Editor.Runes()
			start := c.Editor.Cursor()
			end := start
			for end < len(runes) && !isWordChar(runes[end]) {
				end++
			}
			for end < len(runes) && isWordChar(runes[end]) {
				end++
			}
			killed := engine.DeleteRange(c, start, end)
			c.KillRing.Push(killed)
			return engine.ActionResult{}, nil
		},
	}
	BackwardKillWord = &engine.Action{
		Name: "backward-kill-word",
		Func: func(c *engine.ActionContext) (engine.ActionResult, error) {
			runes := c.Editor.Runes()
			end := c.Editor.Cursor()
			start := end
			for start > 0 && !isWordChar(runes[start-1]) {
				start--
			}
			for start > 0 && isWordChar(runes[start-1]) {
				start--
			}
			killed := engine.DeleteRange(c, start, end)
			c.KillRing.Push(killed)
			return engine.ActionResult{}, nil
		},
	}
	TransposeChars = &engine.Action{
		Name: "transpose-chars",
		Func: func(c *engine.ActionContext) (engine.ActionResult, error) {
			runes := c.Editor.Runes()
			if len(runes) < 2 {
				return engine.ActionResult{}, nil
			}
			cur := c.Editor.Cursor()
			var firstStart, firstEnd, secondStart, secondEnd int
			switch {
			case cur <= 0:
				firstStart, firstEnd = c.Editor.GraphemeBoundsAtPosition(0)
				secondStart, secondEnd = c.Editor.GraphemeBoundsAtPosition(firstEnd)
			case cur >= len(runes):
				secondStart, secondEnd = c.Editor.GraphemeBoundsBeforePosition(cur)
				firstStart, firstEnd = c.Editor.GraphemeBoundsBeforePosition(secondStart)
			default:
				firstStart, firstEnd = c.Editor.GraphemeBoundsBeforePosition(cur)
				secondStart, secondEnd = c.Editor.GraphemeBoundsAtPosition(cur)
			}
			if firstEnd <= firstStart || secondEnd <= secondStart {
				return engine.ActionResult{}, nil
			}
			before := append([]rune{}, runes[:firstStart]...)
			first := append([]rune{}, runes[firstStart:firstEnd]...)
			second := append([]rune{}, runes[secondStart:secondEnd]...)
			after := append([]rune{}, runes[secondEnd:]...)
			newRunes := append(before, second...)
			newRunes = append(newRunes, first...)
			newRunes = append(newRunes, after...)
			c.Editor.SetBuffer(newRunes)
			// Leave cursor after the later of the two transposed graphemes.
			c.Editor.MoveCursor(secondEnd - c.Editor.Cursor())
			return engine.ActionResult{}, nil
		},
	}
	Yank = &engine.Action{
		Name: "yank",
		Func: func(c *engine.ActionContext) (engine.ActionResult, error) {
			for _, r := range c.KillRing.Peek() {
				c.Editor.Insert(r)
			}
			return engine.ActionResult{}, nil
		},
	}
)
