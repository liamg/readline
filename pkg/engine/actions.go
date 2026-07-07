package engine

import (
	"fmt"

	"github.com/liamg/readline/pkg/terminal"
)

// DeleteRange removes runes [start, end) from the editor, repositions the
// cursor to start, and returns the removed runes.
func DeleteRange(c *ActionContext, start, end int) []rune {
	runes := c.Editor.Runes()
	n := len(runes)
	if start < 0 {
		start = 0
	}
	if end > n {
		end = n
	}
	if start >= end {
		return nil
	}
	killed := append([]rune{}, runes[start:end]...)
	newBuf := append(append([]rune{}, runes[:start]...), runes[end:]...)
	c.Editor.SetBuffer(newBuf)
	// SetBuffer puts cursor at end; nudge it back to start.
	c.Editor.MoveCursor(start - len(newBuf))
	return killed
}

var (
	TODO = &Action{ // TODO: remove once all references are removed
		Name: "todo",
		Func: func(c *ActionContext) (ActionResult, error) {
			return ActionResult{}, fmt.Errorf("action for %q has not been implemented yet", c.LastKey())
		},
	}
	NoOp = &Action{
		Name: "noop",
		Func: func(c *ActionContext) (ActionResult, error) {
			return ActionResult{}, nil
		},
	}
	AcceptLine = &Action{
		Name: "accept-line",
		Func: func(_ *ActionContext) (ActionResult, error) {
			return ActionResult{Complete: true}, nil
		},
	}
	Back = &Action{
		Name: "back",
		Func: func(c *ActionContext) (ActionResult, error) {
			c.Editor.MoveCursor(-1)
			return ActionResult{}, nil
		},
	}
	Forward = &Action{
		Name: "forward",
		Func: func(c *ActionContext) (ActionResult, error) {
			c.Editor.MoveCursor(1)
			return ActionResult{}, nil
		},
	}
	AcceptAutosuggestion = &Action{
		Name: "accept-autosuggestion",
		Func: func(c *ActionContext) (ActionResult, error) {
			if suggestion := c.Editor.GetAutoSuggestion(); len(suggestion) > 0 {
				c.Editor.Insert(suggestion...)
			}
			return ActionResult{}, nil
		},
	}
	AcceptAutosuggestionOrForward = &Action{
		Name: "accept-autosuggestion-or-move-forward",
		Func: func(c *ActionContext) (ActionResult, error) {
			if suggestion := c.Editor.GetAutoSuggestion(); len(suggestion) > 0 {
				c.Editor.Insert(suggestion...)
			} else {
				c.Editor.MoveCursor(1)
			}
			return ActionResult{}, nil
		},
	}
	Complete = &Action{
		Name: "complete",
		Func: func(c *ActionContext) (ActionResult, error) {
			c.Editor.TriggerCompletions()
			return ActionResult{}, nil
		},
	}
	BeginningOfLine = &Action{
		Name: "beginning-of-line",
		Func: func(c *ActionContext) (ActionResult, error) {
			c.Editor.MoveCursor(-c.Editor.Cursor())
			return ActionResult{}, nil
		},
	}
	EndOfLine = &Action{
		Name: "end-of-line",
		Func: func(c *ActionContext) (ActionResult, error) {
			c.Editor.MoveCursor(len(c.Editor.Runes()) - c.Editor.Cursor())
			return ActionResult{}, nil
		},
	}
	DeletePrevious = &Action{
		Name: "delete-previous",
		Func: func(c *ActionContext) (ActionResult, error) {
			c.Editor.DeletePrevious()
			return ActionResult{}, nil
		},
	}
	DeleteNext = &Action{
		Name: "delete-next",
		Func: func(c *ActionContext) (ActionResult, error) {
			c.Editor.DeleteNext()
			return ActionResult{}, nil
		},
	}
	ReplaceRune = &Action{
		Name: "replace-rune",
		Func: func(c *ActionContext) (ActionResult, error) {
			return ActionResult{
				Next: func(ctx *ActionContext) (ActionResult, error) {
					ev := ctx.LastKey()
					if ev.Key == terminal.KeyRune && ev.Mod == 0 {
						c.Editor.ReplaceRune(ev.Rune)
					}
					return ActionResult{}, nil
				},
			}, nil
		},
	}
	HistoryPrevious = &Action{
		Name: "history-previous",
		Func: func(c *ActionContext) (ActionResult, error) {
			wasCurrent := c.History.IsCurrent()
			entry := c.History.Previous()
			if entry.Text == "" {
				return ActionResult{}, nil
			}
			if wasCurrent {
				c.Editor.Backup()
			}
			c.Editor.SetBuffer([]rune(entry.Text))
			return ActionResult{}, nil
		},
	}
	HistoryNext = &Action{
		Name: "history-next",
		Func: func(c *ActionContext) (ActionResult, error) {
			if c.History.IsCurrent() {
				return ActionResult{}, nil
			}
			entry := c.History.Next()
			if c.History.IsCurrent() {
				c.Editor.Restore()
			} else {
				c.Editor.SetBuffer([]rune(entry.Text))
			}
			return ActionResult{}, nil
		},
	}
)
