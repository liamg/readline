package vi

import (
	"fmt"
	"strconv"
	"unicode"

	"github.com/liamg/readline/pkg/editor"
	"github.com/liamg/readline/pkg/engine"
	"github.com/liamg/readline/pkg/engine/shared"
	"github.com/liamg/readline/pkg/keymap"
	"github.com/liamg/readline/pkg/terminal"
)

func (v *Vi) buildNormalKeymap() *engine.Keymap {
	km := &engine.Keymap{
		Cursor: engine.CursorBlock,
		Bindings: append([]engine.Binding{ // 1234
			{Sequence: keymap.MustParseSequence("i"), Action: v.actionSwitchToInsertMode()},
			{Sequence: keymap.MustParseSequence("a"), Action: v.actionSwitchToAppendMode()},
			{Sequence: keymap.MustParseSequence("v"), Action: v.actionSwitchToVisualMode()},
			{Sequence: keymap.MustParseSequence("x"), Action: v.actionDeleteCurrentChar()},
			{Sequence: keymap.MustParseSequence("r"), Action: engine.ReplaceRune},
			{Sequence: keymap.MustParseSequence("\""), Action: v.actionSetRegister()},
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
			{Sequence: keymap.MustParseSequence("g,e"), Action: v.moveToEndOfNextWordBackward()},
			{Sequence: keymap.MustParseSequence("g,E"), Action: v.moveToEndOfNextNonWhitespaceBackward()},
			{Sequence: keymap.MustParseSequence("I"), Action: v.actionInsertAtFirstNonBlank()},
			{Sequence: keymap.MustParseSequence("A"), Action: v.actionAppendAtEndOfLine()},
			{Sequence: keymap.MustParseSequence("S"), Action: v.actionChangeCurrentLine()},
			{Sequence: keymap.MustParseSequence("s"), Action: v.actionSubstituteCurrentChar()},
			{Sequence: keymap.MustParseSequence("p"), Action: v.actionPasteAfter()},
			{Sequence: keymap.MustParseSequence("P"), Action: v.actionPasteBefore()},
			{Sequence: keymap.MustParseSequence("f"), Action: v.findCharacterForward()},
			{Sequence: keymap.MustParseSequence("F"), Action: v.findCharacterBackward()},
			{Sequence: keymap.MustParseSequence("t"), Action: v.untilCharacterForward()},
			{Sequence: keymap.MustParseSequence("T"), Action: v.untilCharacterBackward()},
			{Sequence: keymap.MustParseSequence(";"), Action: v.repeatLastCharacterAction(false)},
			{Sequence: keymap.MustParseSequence("comma"), Action: v.repeatLastCharacterAction(true)},
			{Sequence: keymap.MustParseSequence("u"), Action: engine.TODO},
			{Sequence: keymap.MustParseSequence("ctrl-r"), Action: engine.TODO},
			{Sequence: keymap.MustParseSequence("~"), Action: v.actionToggleCase()},
			{Sequence: keymap.MustParseSequence("R"), Action: v.actionReplaceUntilEscape()},
			{Sequence: keymap.MustParseSequence("."), Action: v.actionRepeatLastChange()},
			{Sequence: keymap.MustParseSequence("j"), Action: engine.HistoryNext},
			{Sequence: keymap.MustParseSequence("k"), Action: engine.HistoryPrevious},
			{Sequence: keymap.MustParseSequence("G"), Action: engine.TODO},
			{Sequence: keymap.MustParseSequence("g,g"), Action: engine.TODO},
			{Sequence: keymap.MustParseSequence("/"), Action: v.actionHistorySearch()},
			{Sequence: keymap.MustParseSequence("?"), Action: engine.TODO},
			{Sequence: keymap.MustParseSequence("n"), Action: v.actionHistorySearchNext()},
			{Sequence: keymap.MustParseSequence("N"), Action: v.actionHistorySearchPrevious()},
			{Sequence: keymap.MustParseSequence("ctrl-a"), Action: v.actionIncrementNumber(1)},
			{Sequence: keymap.MustParseSequence("ctrl-x"), Action: v.actionIncrementNumber(-1)},
			{Sequence: keymap.MustParseSequence("C"), Action: v.actionChangeToEndOfLine()},
			{Sequence: keymap.MustParseSequence("D"), Action: v.actionDeleteToEndOfLine()},
			{Sequence: keymap.MustParseSequence("Y"), Action: engine.TODO},
			{Sequence: keymap.MustParseSequence("c"), Action: v.actionChange()},
			{Sequence: keymap.MustParseSequence("d"), Action: v.actionDelete()},
			{Sequence: keymap.MustParseSequence("y"), Action: v.actionYank()},
			{Sequence: keymap.MustParseSequence("g,~"), Action: engine.TODO},
			{Sequence: keymap.MustParseSequence("g,u"), Action: engine.TODO},
			{Sequence: keymap.MustParseSequence("g,U"), Action: engine.TODO},
			{Sequence: keymap.MustParseSequence("esc"), Action: engine.NoOp}, // hide unrecognised binding when user hits escape in normal mode by binding it to a noop
		}, shared.Bindings...),
		Fallback: func(ctx *engine.ActionContext, ev terminal.KeyEvent) (engine.ActionResult, error) {
			if v.handleCountOrFallback(ev) {
				return engine.ActionResult{
					SkipReset: true, // don't reset the count if we just added a digit to it
				}, nil
			}
			return engine.ActionResult{}, fmt.Errorf("%w: %s", engine.ErrUnrecognisedBinding, ev)
		},
	}
	engine.Bind(km, keymap.MustParseSequence("end"), v.actionAppendAtEndOfLine())
	engine.Bind(km, keymap.MustParseSequence("right"), v.actionMoveRightOrAppend())
	return km
}

// actionZero handles the "0" key: while a count is being entered it extends the
// count (so "10l" moves ten), otherwise it moves to the beginning of the line.
func (v *Vi) actionZero() *engine.Action {
	return &engine.Action{
		Name: "vi-zero",
		Func: func(c *engine.ActionContext) (engine.ActionResult, error) {
			if v.state.count > 0 {
				v.state.AddCountDigit('0')
				return engine.ActionResult{SkipReset: true}, nil
			}
			return engine.BeginningOfLine.Func(c)
		},
	}
}

// actionMoveLeft moves the cursor left by the pending count (h), one grapheme
// per step, clamped at the start of the line.
func (v *Vi) actionMoveLeft() *engine.Action {
	return &engine.Action{
		Name: "vi-move-left",
		Func: func(c *engine.ActionContext) (engine.ActionResult, error) {
			for range v.state.Count() {
				c.Editor.MoveCursor(-1)
			}
			return engine.ActionResult{}, nil
		},
	}
}

// actionMoveRight moves the cursor right by the pending count (l), one grapheme
// per step, clamped at the end of the line.
func (v *Vi) actionMoveRight() *engine.Action {
	return &engine.Action{
		Name: "vi-move-right",
		Func: func(c *engine.ActionContext) (engine.ActionResult, error) {
			for range v.state.Count() {
				c.Editor.MoveCursor(1)
			}
			return engine.ActionResult{}, nil
		},
	}
}

func (v *Vi) actionSwitchToInsertMode() *engine.Action {
	return &engine.Action{
		Name: "set-insert-mode",
		Func: func(c *engine.ActionContext) (engine.ActionResult, error) {
			v.state.SetMode(ModeInsert)
			c.Editor.SetClampCursorBeforeEnd(false)
			return engine.ActionResult{Keymap: ModeInsert}, nil
		},
	}
}

func (v *Vi) actionSwitchToAppendMode() *engine.Action {
	return &engine.Action{
		Name: "set-append-mode",
		Func: func(c *engine.ActionContext) (engine.ActionResult, error) {
			v.state.SetMode(ModeInsert)
			c.Editor.SetClampCursorBeforeEnd(false)
			c.Editor.MoveCursor(1)
			return engine.ActionResult{Keymap: ModeInsert}, nil
		},
	}
}

func (v *Vi) actionInsertAtFirstNonBlank() *engine.Action {
	return &engine.Action{
		Name: "vi-insert-at-first-non-blank",
		Func: func(c *engine.ActionContext) (engine.ActionResult, error) {
			cursor, _, err := motionLineFirstNonBlank(c.Editor.Cursor(), c.Editor.Buffer(), v.state.Count())
			if err != nil {
				return engine.ActionResult{}, err
			}
			c.Editor.MoveCursor(cursor - c.Editor.Cursor())
			return v.actionSwitchToInsertMode().Func(c)
		},
	}
}

func (v *Vi) actionAppendAtEndOfLine() *engine.Action {
	return &engine.Action{
		Name: "vi-append-at-end-of-line",
		Func: func(c *engine.ActionContext) (engine.ActionResult, error) {
			res, err := v.actionSwitchToInsertMode().Func(c)
			if err != nil {
				return engine.ActionResult{}, err
			}
			if _, err := engine.EndOfLine.Func(c); err != nil {
				return engine.ActionResult{}, err
			}
			return res, nil
		},
	}
}

func (v *Vi) actionMoveRightOrAppend() *engine.Action {
	return &engine.Action{
		Name: "vi-move-right-or-append",
		Func: func(c *engine.ActionContext) (engine.ActionResult, error) {
			if c.Editor.Cursor() >= len(c.Editor.Runes())-1 {
				return v.actionSwitchToAppendMode().Func(c)
			}
			return engine.Forward.Func(c)
		},
	}
}

func (v *Vi) actionSwitchToNormalMode() *engine.Action {
	return &engine.Action{
		Name: "set-normal",
		Func: func(c *engine.ActionContext) (engine.ActionResult, error) {
			v.state.SetMode(ModeNormal)
			c.Editor.ClearCompletions()
			c.Editor.MoveCursor(-1)
			c.Editor.SetClampCursorBeforeEnd(true)
			return engine.ActionResult{Keymap: ModeNormal}, nil
		},
	}
}

func (v *Vi) actionSwitchToVisualMode() *engine.Action {
	return &engine.Action{
		Name: "set-visual",
		Func: func(c *engine.ActionContext) (engine.ActionResult, error) {
			v.state.SetMode(ModeVisual)
			// Anchor a character-wise selection at the cursor so visual-mode
			// operators (d/c/y/etc.) always have a selection to act on. Without
			// this, Selection() is nil and Range() panics.
			c.Editor.SetSelectionAnchor(editor.SelectionChar)
			c.Editor.SetClampCursorBeforeEnd(true)
			return engine.ActionResult{Keymap: ModeVisual}, nil
		},
	}
}

func (v *Vi) actionDeleteCurrentChar() *engine.Action {
	return &engine.Action{
		Name: "vi-delete-current-char",
		Func: func(c *engine.ActionContext) (engine.ActionResult, error) {
			killed := c.Editor.DeleteNextGraphemes(v.state.Count())
			if len(killed) == 0 {
				return engine.ActionResult{}, nil
			}
			if err := v.state.WriteActiveRegister(killed); err != nil {
				return engine.ActionResult{}, err
			}
			v.state.lastChangeAction = v.actionDeleteCurrentChar()
			return engine.ActionResult{}, nil
		},
	}
}

func (v *Vi) actionSubstituteCurrentChar() *engine.Action {
	return &engine.Action{
		Name: "vi-substitute-current-char",
		Func: func(c *engine.ActionContext) (engine.ActionResult, error) {
			killed := c.Editor.DeleteNextGraphemes(v.state.Count())
			if len(killed) > 0 {
				if err := v.state.WriteActiveRegister(killed); err != nil {
					return engine.ActionResult{}, err
				}
			}
			v.state.lastChangeAction = v.actionSubstituteCurrentChar()
			return v.actionSwitchToInsertMode().Func(c)
		},
	}
}

func (v *Vi) actionPasteAfter() *engine.Action {
	return &engine.Action{
		Name: "vi-paste-after",
		Func: func(c *engine.ActionContext) (engine.ActionResult, error) {
			content, err := v.state.ReadPasteRegister()
			if err != nil {
				return engine.ActionResult{}, err
			}
			if len(content) == 0 {
				return engine.ActionResult{}, nil
			}
			c.Editor.SetClampCursorBeforeEnd(false)
			c.Editor.MoveCursor(1)
			c.Editor.Insert(content...)
			c.Editor.SetClampCursorBeforeEnd(true)
			v.state.lastChangeAction = v.actionPasteAfter()
			return engine.ActionResult{}, nil
		},
	}
}

func (v *Vi) actionPasteBefore() *engine.Action {
	return &engine.Action{
		Name: "vi-paste-before",
		Func: func(c *engine.ActionContext) (engine.ActionResult, error) {
			content, err := v.state.ReadPasteRegister()
			if err != nil {
				return engine.ActionResult{}, err
			}
			if len(content) == 0 {
				return engine.ActionResult{}, nil
			}
			c.Editor.Insert(content...)
			v.state.lastChangeAction = v.actionPasteBefore()
			return engine.ActionResult{}, nil
		},
	}
}

func (v *Vi) actionToggleCase() *engine.Action {
	return &engine.Action{
		Name: "vi-toggle-case",
		Func: func(c *engine.ActionContext) (engine.ActionResult, error) {
			start, end := c.Editor.GraphemeBoundsAtCursor()
			if start >= end {
				return engine.ActionResult{}, nil
			}
			cluster := c.Editor.Buffer()[start:end]
			if len(cluster) != 1 {
				c.Editor.MoveCursor(end - c.Editor.Cursor())
				return engine.ActionResult{}, nil
			}
			ru := cluster[0]
			switch {
			case unicode.IsLower(ru):
				ru = unicode.ToUpper(ru)
			case unicode.IsUpper(ru):
				ru = unicode.ToLower(ru)
			}
			c.Editor.MoveCursor(start - c.Editor.Cursor())
			c.Editor.ReplaceRune(ru)
			c.Editor.MoveCursor(1)
			v.state.lastChangeAction = v.actionToggleCase()
			return engine.ActionResult{}, nil
		},
	}
}

func (v *Vi) actionToggleSelectionCase() *engine.Action {
	return &engine.Action{
		Name: "vi-toggle-selection-case",
		Func: func(c *engine.ActionContext) (engine.ActionResult, error) {
			selection := c.Editor.Selection()
			if selection == nil {
				return v.actionToggleCase().Func(c)
			}
			buffer := c.Editor.Buffer()
			start, end := selection.Range(c.Editor.Cursor(), buffer)
			for i := start; i < end && i < len(buffer); i++ {
				ru := buffer[i]
				switch {
				case unicode.IsLower(ru):
					ru = unicode.ToUpper(ru)
				case unicode.IsUpper(ru):
					ru = unicode.ToLower(ru)
				}
				buffer[i] = ru
			}
			c.Editor.SetBuffer(buffer)
			c.Editor.MoveCursor(start - c.Editor.Cursor())
			c.Editor.ClearSelection()
			v.state.SetMode(ModeNormal)
			c.Editor.SetClampCursorBeforeEnd(true)
			v.state.lastChangeAction = v.actionToggleSelectionCase()
			return engine.ActionResult{Keymap: ModeNormal}, nil
		},
	}
}

func (v *Vi) actionRepeatLastChange() *engine.Action {
	return &engine.Action{
		Name: "vi-repeat-last-change",
		Func: func(c *engine.ActionContext) (engine.ActionResult, error) {
			if v.state.lastChangeAction == nil {
				return engine.ActionResult{}, fmt.Errorf("no change to repeat")
			}
			return v.state.lastChangeAction.Func(c)
		},
	}
}

func (v *Vi) actionHistorySearch() *engine.Action {
	var query []rune
	var collect engine.ActionFunc
	collect = func(c *engine.ActionContext) (engine.ActionResult, error) {
		key := c.LastKey()
		switch {
		case key.Key == terminal.KeyEnter:
			if len(query) == 0 {
				return engine.ActionResult{}, nil
			}
			v.state.lastSearch = append([]rune(nil), query...)
			c.History.SetFilter(string(query))
			if entry := c.History.Previous(); entry.Text != "" {
				c.Editor.SetBuffer([]rune(entry.Text))
			}
			return engine.ActionResult{}, nil
		case key.Key == terminal.KeyBackspace:
			if len(query) > 0 {
				query = query[:len(query)-1]
			}
			return engine.ActionResult{Next: collect}, nil
		case key.Key == terminal.KeyEscape:
			return engine.ActionResult{}, nil
		case key.Key == terminal.KeyRune && key.Mod == 0:
			query = append(query, key.Rune)
			return engine.ActionResult{Next: collect}, nil
		default:
			return engine.ActionResult{Next: collect}, nil
		}
	}
	return &engine.Action{
		Name: "vi-history-search",
		Func: func(*engine.ActionContext) (engine.ActionResult, error) {
			return engine.ActionResult{Next: collect}, nil
		},
	}
}

func (v *Vi) actionHistorySearchNext() *engine.Action {
	return &engine.Action{
		Name: "vi-history-search-next",
		Func: func(c *engine.ActionContext) (engine.ActionResult, error) {
			if len(v.state.lastSearch) == 0 {
				return engine.ActionResult{}, fmt.Errorf("no history search to repeat")
			}
			c.History.SetFilter(string(v.state.lastSearch))
			if entry := c.History.Previous(); entry.Text != "" {
				c.Editor.SetBuffer([]rune(entry.Text))
			}
			return engine.ActionResult{}, nil
		},
	}
}

func (v *Vi) actionHistorySearchPrevious() *engine.Action {
	return &engine.Action{
		Name: "vi-history-search-previous",
		Func: func(c *engine.ActionContext) (engine.ActionResult, error) {
			if len(v.state.lastSearch) == 0 {
				return engine.ActionResult{}, fmt.Errorf("no history search to repeat")
			}
			c.History.SetFilter(string(v.state.lastSearch))
			if entry := c.History.Next(); entry.Text != "" {
				c.Editor.SetBuffer([]rune(entry.Text))
			}
			return engine.ActionResult{}, nil
		},
	}
}

func (v *Vi) actionReplaceUntilEscape() *engine.Action {
	var replace engine.ActionFunc
	replace = func(c *engine.ActionContext) (engine.ActionResult, error) {
		key := c.LastKey()
		if key.Key == terminal.KeyEscape {
			v.state.lastChangeAction = v.actionReplaceUntilEscape()
			return v.actionSwitchToNormalMode().Func(c)
		}
		if key.Key != terminal.KeyRune || key.Mod != 0 {
			return engine.ActionResult{Next: replace}, nil
		}
		if c.Editor.Cursor() < len(c.Editor.Buffer()) {
			c.Editor.ReplaceRune(key.Rune)
		} else {
			c.Editor.Insert(key.Rune)
		}
		return engine.ActionResult{Next: replace}, nil
	}
	return &engine.Action{
		Name: "vi-replace-until-escape",
		Func: func(*engine.ActionContext) (engine.ActionResult, error) {
			return engine.ActionResult{Next: replace}, nil
		},
	}
}

func (v *Vi) actionIncrementNumber(direction int) *engine.Action {
	return &engine.Action{
		Name: "vi-increment-number",
		Func: func(c *engine.ActionContext) (engine.ActionResult, error) {
			buffer := c.Editor.Buffer()
			start, end, ok := findNumberAtOrAfterCursor(buffer, c.Editor.Cursor())
			if !ok {
				return engine.ActionResult{}, nil
			}

			n, err := strconv.Atoi(string(buffer[start:end]))
			if err != nil {
				return engine.ActionResult{}, err
			}

			replacement := formatIncrementedNumber(buffer[start:end], n+(direction*v.state.Count()))
			next := append([]rune{}, buffer[:start]...)
			next = append(next, replacement...)
			next = append(next, buffer[end:]...)
			targetCursor := start + len(replacement) - 1

			c.Editor.SetBuffer(next)
			c.Editor.MoveCursor(targetCursor - c.Editor.Cursor())
			v.state.lastChangeAction = v.actionIncrementNumber(direction)
			return engine.ActionResult{}, nil
		},
	}
}

func findNumberAtOrAfterCursor(buffer []rune, cursor int) (start, end int, ok bool) {
	if cursor < 0 {
		cursor = 0
	}
	for i := cursor; i < len(buffer); i++ {
		if buffer[i] == '-' && i+1 < len(buffer) && unicode.IsDigit(buffer[i+1]) {
			start = i
			end = i + 1
			for end < len(buffer) && unicode.IsDigit(buffer[end]) {
				end++
			}
			return start, end, true
		}
		if !unicode.IsDigit(buffer[i]) {
			continue
		}

		start = i
		for start > 0 && unicode.IsDigit(buffer[start-1]) {
			start--
		}
		if start > 0 && buffer[start-1] == '-' {
			start--
		}
		end = i + 1
		for end < len(buffer) && unicode.IsDigit(buffer[end]) {
			end++
		}
		return start, end, true
	}
	return 0, 0, false
}

func formatIncrementedNumber(original []rune, value int) []rune {
	width := len(original)
	if width > 0 && original[0] == '-' {
		width--
	}

	sign := ""
	if value < 0 {
		sign = "-"
		value = -value
	}

	digits := strconv.Itoa(value)
	if len(digits) < width && hasLeadingZero(original) {
		digits = fmt.Sprintf("%0*s", width, digits)
	}
	return []rune(sign + digits)
}

func hasLeadingZero(value []rune) bool {
	if len(value) == 0 {
		return false
	}
	if value[0] == '-' {
		value = value[1:]
	}
	return len(value) > 1 && value[0] == '0'
}

func (v *Vi) actionSetRegister() *engine.Action {
	return &engine.Action{
		Name: "set-register",
		Func: func(c *engine.ActionContext) (engine.ActionResult, error) {
			c.Editor.SetHint("Selecting register...")
			return engine.ActionResult{
				Next: func(c *engine.ActionContext) (engine.ActionResult, error) {
					if c.LastKey().Key != terminal.KeyRune || c.LastKey().Mod != 0 {
						return engine.ActionResult{}, fmt.Errorf("expected register name, got %q", c.LastKey())
					}
					if err := v.state.SetActiveRegister(c.LastKey().Rune); err != nil {
						return engine.ActionResult{}, fmt.Errorf("invalid register name: %q", c.LastKey())
					}
					c.Editor.SetHint("Do what with register?")
					return engine.ActionResult{
						Next: v.actionRegisterOperation(),
					}, nil
				},
			}, nil
		},
	}
}

func (v *Vi) actionRegisterOperation() engine.ActionFunc {
	return func(c *engine.ActionContext) (engine.ActionResult, error) {
		if c.LastKey().Key != terminal.KeyRune || c.LastKey().Mod != 0 {
			return engine.ActionResult{}, fmt.Errorf("expected register operation, got '%q'", c.LastKey())
		}
		op := c.LastKey().Rune
		switch op {
		case 'p': // TODO: paste
			return engine.TODO.Func(c)
		case 'P': // TODO: paste
			return engine.TODO.Func(c)
		case 'y': // yank
			return v.actionYank().Func(c)
		case 'Y': // TODO: yank to end of line
			return engine.TODO.Func(c)
		case 'x': // TODO: delete char to register
			return engine.TODO.Func(c)
		case 'X': // TODO: delete char to register
			return engine.TODO.Func(c)
		case 's': // TODO: substitute char to register
			return engine.TODO.Func(c)
		case 'S': // TODO: substitute char to register
			return engine.TODO.Func(c)
		case 'd':
			return v.actionDelete().Func(c)
		case 'D': // TODO: delete to register
			return engine.TODO.Func(c)
		case 'c': // TODO: change to register
			return engine.TODO.Func(c)
		case 'C': // TODO: change to register
			return engine.TODO.Func(c)
		default:
			return engine.ActionResult{}, fmt.Errorf("unknown register operation '%q'", op)
		}
	}
}

func (v *Vi) actionYank() *engine.Action {
	return &engine.Action{
		Name: "vi-yank",
		Func: func(c *engine.ActionContext) (engine.ActionResult, error) {
			// in visual mode we are yanking the selection
			if v.state.GetMode() == ModeVisual {
				// in visual mode there is always a selection, even if it's just one character at the cursor
				// so we need to default to a one char selection if there is no "proper" selection
				content := c.Editor.GetSelectedRunes()
				if len(content) == 0 {
					content = []rune{c.Editor.GetRuneAt(c.Editor.Cursor())}
				}
				err := v.state.WriteActiveRegister(content)
				c.Editor.ClearSelection()
				v.state.SetMode(ModeNormal)
				c.Editor.SetClampCursorBeforeEnd(true)
				return engine.ActionResult{Keymap: ModeNormal}, err
			}

			// in normal mode we are operator pending
			return engine.ActionResult{
				Next: v.applyOpWithMotion(func(start, end, count int, motion rune) (engine.ActionResult, error) {
					buffer := c.Editor.Buffer()

					// yy is a special case
					if motion == 'y' {
						// yank entire line
						var err error
						start, end, err = motionLineEntire(c.Editor.Cursor(), buffer, count)
						if err != nil {
							return engine.ActionResult{}, err
						}
					}

					content := buffer[start:end]
					err := v.state.WriteActiveRegister(content)
					return engine.ActionResult{}, err
				}),
			}, nil
		},
	}
}

func (v *Vi) actionDelete() *engine.Action {
	return &engine.Action{
		Name: "vi-delete",
		Func: func(c *engine.ActionContext) (engine.ActionResult, error) {
			// in visual mode we are yanking the selection
			if v.state.GetMode() == ModeVisual {
				// in visual mode there is always a selection, even if it's just one character at the cursor
				// so we need to default to a one char selection if there is no "proper" selection
				content := c.Editor.GetSelectedRunes()
				if len(content) == 0 {
					content = []rune{c.Editor.GetRuneAt(c.Editor.Cursor())}
				}
				err := v.state.WriteActiveRegister(content)

				buffer := c.Editor.Buffer()
				start, end := c.Editor.Cursor(), c.Editor.Cursor()
				if selection := c.Editor.Selection(); selection != nil {
					start, end = selection.Range(c.Editor.Cursor(), buffer)
				}
				after := buffer[end:]
				before := buffer[:start]
				c.Editor.SetBuffer(append(before, after...))
				c.Editor.ClearSelection()
				c.Editor.MoveCursor(start - c.Editor.Cursor())

				v.state.SetMode(ModeNormal)
				c.Editor.SetClampCursorBeforeEnd(true)
				return engine.ActionResult{Keymap: ModeNormal}, err
			}

			// in normal mode we are operator pending
			return engine.ActionResult{
				Next: v.applyOpWithMotion(func(start, end, count int, motion rune) (engine.ActionResult, error) {
					buffer := c.Editor.Buffer()

					// dd is a special case
					if motion == 'd' {
						// delete entire line
						var err error
						start, end, err = motionLineEntire(c.Editor.Cursor(), buffer, count)
						if err != nil {
							return engine.ActionResult{}, err
						}
					}

					content := buffer[start:end]
					err := v.state.WriteActiveRegister(content)

					after := buffer[end:]
					before := buffer[:start]
					c.Editor.SetBuffer(append(before, after...))
					c.Editor.MoveCursor(start - c.Editor.Cursor())

					return engine.ActionResult{}, err
				}),
			}, nil
		},
	}
}

func (v *Vi) actionDeleteToEndOfLine() *engine.Action {
	return &engine.Action{
		Name: "vi-delete-to-end-of-line",
		Func: func(c *engine.ActionContext) (engine.ActionResult, error) {
			buffer := c.Editor.Buffer()
			start := c.Editor.Cursor()
			_, end, err := motionLineEnd(start, buffer, v.state.Count())
			if err != nil {
				return engine.ActionResult{}, err
			}
			content := buffer[start:end]
			if err := v.state.WriteActiveRegister(content); err != nil {
				return engine.ActionResult{}, err
			}

			after := buffer[end:]
			before := buffer[:start]
			c.Editor.SetBuffer(append(before, after...))
			c.Editor.MoveCursor(start - c.Editor.Cursor())

			return engine.ActionResult{}, nil
		},
	}
}

func (v *Vi) actionChange() *engine.Action {
	return &engine.Action{
		Name: "vi-change",
		Func: func(c *engine.ActionContext) (engine.ActionResult, error) {
			// in visual mode we are yanking the selection
			if v.state.GetMode() == ModeVisual {
				// in visual mode there is always a selection, even if it's just one character at the cursor
				// so we need to default to a one char selection if there is no "proper" selection
				content := c.Editor.GetSelectedRunes()
				if len(content) == 0 {
					content = []rune{c.Editor.GetRuneAt(c.Editor.Cursor())}
				}
				err := v.state.WriteActiveRegister(content)

				buffer := c.Editor.Buffer()
				start, end := c.Editor.Cursor(), c.Editor.Cursor()
				if selection := c.Editor.Selection(); selection != nil {
					start, end = selection.Range(c.Editor.Cursor(), buffer)
				}
				after := buffer[end:]
				before := buffer[:start]
				c.Editor.SetBuffer(append(before, after...))
				c.Editor.ClearSelection()
				c.Editor.MoveCursor(start - c.Editor.Cursor())

				v.state.SetMode(ModeInsert)
				c.Editor.SetClampCursorBeforeEnd(false)
				return engine.ActionResult{Keymap: ModeInsert}, err
			}

			// in normal mode we are operator pending
			return engine.ActionResult{
				Next: v.applyOpWithMotion(func(start, end, count int, motion rune) (engine.ActionResult, error) {
					buffer := c.Editor.Buffer()

					// cc is a special case
					if motion == 'c' {
						return v.changeCurrentLine(c)
					}

					content := buffer[start:end]
					err := v.state.WriteActiveRegister(content)

					after := buffer[end:]
					before := buffer[:start]
					c.Editor.SetBuffer(append(before, after...))
					c.Editor.MoveCursor(start - c.Editor.Cursor())

					v.state.SetMode(ModeInsert)
					c.Editor.SetClampCursorBeforeEnd(false)
					return engine.ActionResult{Keymap: ModeInsert}, err
				}),
			}, nil
		},
	}
}

func (v *Vi) actionChangeToEndOfLine() *engine.Action {
	return &engine.Action{
		Name: "vi-change-to-end-of-line",
		Func: func(c *engine.ActionContext) (engine.ActionResult, error) {
			buffer := c.Editor.Buffer()
			start := c.Editor.Cursor()
			_, end, err := motionLineEnd(start, buffer, v.state.Count())
			if err != nil {
				return engine.ActionResult{}, err
			}
			content := buffer[start:end]
			if err := v.state.WriteActiveRegister(content); err != nil {
				return engine.ActionResult{}, err
			}

			after := buffer[end:]
			before := buffer[:start]
			c.Editor.SetBuffer(append(before, after...))
			c.Editor.MoveCursor(start - c.Editor.Cursor())

			return v.actionSwitchToInsertMode().Func(c)
		},
	}
}

func (v *Vi) actionChangeCurrentLine() *engine.Action {
	return &engine.Action{
		Name: "vi-change-current-line",
		Func: func(c *engine.ActionContext) (engine.ActionResult, error) {
			return v.changeCurrentLine(c)
		},
	}
}

func (v *Vi) changeCurrentLine(c *engine.ActionContext) (engine.ActionResult, error) {
	buffer := c.Editor.Buffer()
	start, _, _ := motionLineStart(c.Editor.Cursor(), buffer, 1)
	_, end, _ := motionLineEnd(c.Editor.Cursor(), buffer, 1)

	content := buffer[start:end]
	err := v.state.WriteActiveRegister(content)

	after := buffer[end:]
	before := buffer[:start]
	c.Editor.SetBuffer(append(before, after...))
	c.Editor.MoveCursor(start - c.Editor.Cursor())

	v.state.SetMode(ModeInsert)
	c.Editor.SetClampCursorBeforeEnd(false)
	return engine.ActionResult{Keymap: ModeInsert}, err
}

func (v *Vi) moveToFirstNonBlank() *engine.Action {
	return &engine.Action{
		Name: "vi-first-non-blank",
		Func: func(ac *engine.ActionContext) (engine.ActionResult, error) {
			cursor, _, err := motionLineFirstNonBlank(ac.Editor.Cursor(), ac.Editor.Buffer(), v.state.Count())
			if err != nil {
				return engine.ActionResult{}, err
			}
			ac.Editor.MoveCursor(cursor - ac.Editor.Cursor())
			return engine.ActionResult{}, nil
		},
	}
}

func (v *Vi) findCharacterForward() *engine.Action {
	return &engine.Action{
		Name: "vi-find-character-forward",
		Func: func(_ *engine.ActionContext) (engine.ActionResult, error) {
			return engine.ActionResult{
				Next: func(ac *engine.ActionContext) (engine.ActionResult, error) {
					if ac.LastKey().Key != terminal.KeyRune || ac.LastKey().Mod != 0 {
						return engine.ActionResult{}, fmt.Errorf("expecting character to find, received %q", ac.LastKey())
					}

					subject := ac.LastKey().Rune

					v.state.lastCharSearchRune = subject
					v.state.lastCharSearchAction = v.findCharacterForward()
					v.state.lastCharSearchReverseAction = v.findCharacterBackward()

					_, end, err := motionFindCharacterForward(subject)(ac.Editor.Cursor(), ac.Editor.Buffer(), v.state.Count())
					if err != nil {
						return engine.ActionResult{}, err
					}

					ac.Editor.MoveCursor(end - ac.Editor.Cursor())

					return engine.ActionResult{}, nil
				},
			}, nil
		},
	}
}

func (v *Vi) findCharacterBackward() *engine.Action {
	return &engine.Action{
		Name: "vi-find-character-backward",
		Func: func(_ *engine.ActionContext) (engine.ActionResult, error) {
			return engine.ActionResult{
				Next: func(ac *engine.ActionContext) (engine.ActionResult, error) {
					if ac.LastKey().Key != terminal.KeyRune || ac.LastKey().Mod != 0 {
						return engine.ActionResult{}, fmt.Errorf("expecting character to find, received %q", ac.LastKey())
					}

					subject := ac.LastKey().Rune

					v.state.lastCharSearchRune = subject
					v.state.lastCharSearchAction = v.findCharacterBackward()
					v.state.lastCharSearchReverseAction = v.findCharacterForward()

					start, _, err := motionFindCharacterBackward(subject)(ac.Editor.Cursor(), ac.Editor.Buffer(), v.state.Count())
					if err != nil {
						return engine.ActionResult{}, err
					}

					ac.Editor.MoveCursor(start - ac.Editor.Cursor())

					return engine.ActionResult{}, nil
				},
			}, nil
		},
	}
}

func (v *Vi) untilCharacterForward() *engine.Action {
	return &engine.Action{
		Name: "vi-until-character-forward",
		Func: func(_ *engine.ActionContext) (engine.ActionResult, error) {
			return engine.ActionResult{
				Next: func(ac *engine.ActionContext) (engine.ActionResult, error) {
					if ac.LastKey().Key != terminal.KeyRune || ac.LastKey().Mod != 0 {
						return engine.ActionResult{}, fmt.Errorf("expecting character to find, received %q", ac.LastKey())
					}

					subject := ac.LastKey().Rune

					v.state.lastCharSearchRune = subject
					v.state.lastCharSearchAction = v.untilCharacterForward()
					v.state.lastCharSearchReverseAction = v.untilCharacterBackward()

					_, end, err := motionUntilCharacterForward(subject)(ac.Editor.Cursor(), ac.Editor.Buffer(), v.state.Count())
					if err != nil {
						return engine.ActionResult{}, err
					}

					ac.Editor.MoveCursor(end - ac.Editor.Cursor())

					return engine.ActionResult{}, nil
				},
			}, nil
		},
	}
}

func (v *Vi) untilCharacterBackward() *engine.Action {
	return &engine.Action{
		Name: "vi-until-character-backward",
		Func: func(_ *engine.ActionContext) (engine.ActionResult, error) {
			return engine.ActionResult{
				Next: func(ac *engine.ActionContext) (engine.ActionResult, error) {
					if ac.LastKey().Key != terminal.KeyRune || ac.LastKey().Mod != 0 {
						return engine.ActionResult{}, fmt.Errorf("expecting character to find, received %q", ac.LastKey())
					}

					subject := ac.LastKey().Rune

					v.state.lastCharSearchRune = subject
					v.state.lastCharSearchAction = v.untilCharacterBackward()
					v.state.lastCharSearchReverseAction = v.untilCharacterForward()

					start, _, err := motionUntilCharacterBackward(subject)(ac.Editor.Cursor(), ac.Editor.Buffer(), v.state.Count())
					if err != nil {
						return engine.ActionResult{}, err
					}

					ac.Editor.MoveCursor(start - ac.Editor.Cursor())
					return engine.ActionResult{}, nil
				},
			}, nil
		},
	}
}

func (v *Vi) repeatLastCharacterAction(reverse bool) *engine.Action {
	return &engine.Action{
		Name: "vi-repeat-char-search",
		Func: func(ac *engine.ActionContext) (engine.ActionResult, error) {
			action := v.state.lastCharSearchAction
			if reverse {
				action = v.state.lastCharSearchReverseAction
			}
			if action == nil {
				return engine.ActionResult{}, fmt.Errorf("no character search to repeat")
			}

			res, err := action.Func(ac)
			if err != nil {
				return res, err
			}

			if res.Next == nil {
				return engine.ActionResult{}, fmt.Errorf("internal error, cannot repeat character search")
			}

			ac.Keys = append(ac.Keys, terminal.KeyEvent{Key: terminal.KeyRune, Rune: v.state.lastCharSearchRune})
			return res.Next(ac)
		},
	}
}

func (v *Vi) moveToStartOfNextWordForward() *engine.Action {
	return &engine.Action{
		Name: "vi-find-word-start-forward",
		Func: func(ac *engine.ActionContext) (engine.ActionResult, error) {
			_, end, err := motionToStartOfWordForward(ac.Editor.Cursor(), ac.Editor.Buffer(), v.state.Count())
			if err != nil {
				return engine.ActionResult{}, err
			}

			ac.Editor.MoveCursor(end - ac.Editor.Cursor())

			return engine.ActionResult{}, nil
		},
	}
}

func (v *Vi) moveToEndOfNextWordForward() *engine.Action {
	return &engine.Action{
		Name: "vi-find-word-end-forward",
		Func: func(ac *engine.ActionContext) (engine.ActionResult, error) {
			_, end, err := motionToEndOfWordForward(ac.Editor.Cursor(), ac.Editor.Buffer(), v.state.Count())
			if err != nil {
				return engine.ActionResult{}, err
			}

			ac.Editor.MoveCursor(end - ac.Editor.Cursor())

			return engine.ActionResult{}, nil
		},
	}
}

func (v *Vi) moveToEndOfNextNonWhitespaceForward() *engine.Action {
	return &engine.Action{
		Name: "vi-find-word-end-forward",
		Func: func(ac *engine.ActionContext) (engine.ActionResult, error) {
			_, end, err := motionToEndOfNonWhitespaceForward(ac.Editor.Cursor(), ac.Editor.Buffer(), v.state.Count())
			if err != nil {
				return engine.ActionResult{}, err
			}

			ac.Editor.MoveCursor(end - ac.Editor.Cursor())

			return engine.ActionResult{}, nil
		},
	}
}

func (v *Vi) moveToStartOfNextNonWhitespaceForward() *engine.Action {
	return &engine.Action{
		Name: "vi-find-non-whitespace-start-forward",
		Func: func(ac *engine.ActionContext) (engine.ActionResult, error) {
			_, end, err := motionToStartOfNonWhitespaceForward(ac.Editor.Cursor(), ac.Editor.Buffer(), v.state.Count())
			if err != nil {
				return engine.ActionResult{}, err
			}

			ac.Editor.MoveCursor(end - ac.Editor.Cursor())

			return engine.ActionResult{}, nil
		},
	}
}

func (v *Vi) moveToStartOfNextWordBackward() *engine.Action {
	return &engine.Action{
		Name: "vi-find-word-start-backward",
		Func: func(ac *engine.ActionContext) (engine.ActionResult, error) {
			start, _, err := motionToStartOfWordBackward(ac.Editor.Cursor(), ac.Editor.Buffer(), v.state.Count())
			if err != nil {
				return engine.ActionResult{}, err
			}

			ac.Editor.MoveCursor(start - ac.Editor.Cursor())

			return engine.ActionResult{}, nil
		},
	}
}

func (v *Vi) moveToStartOfNextNonWhitespaceBackward() *engine.Action {
	return &engine.Action{
		Name: "vi-find-non-whitespace-start-backward",
		Func: func(ac *engine.ActionContext) (engine.ActionResult, error) {
			start, _, err := motionToStartOfNonWhitespaceBackward(ac.Editor.Cursor(), ac.Editor.Buffer(), v.state.Count())
			if err != nil {
				return engine.ActionResult{}, err
			}

			ac.Editor.MoveCursor(start - ac.Editor.Cursor())

			return engine.ActionResult{}, nil
		},
	}
}

func (v *Vi) moveToEndOfNextWordBackward() *engine.Action {
	return &engine.Action{
		Name: "vi-find-word-end-backward",
		Func: func(ac *engine.ActionContext) (engine.ActionResult, error) {
			start, _, err := motionToEndOfWordBackward(ac.Editor.Cursor(), ac.Editor.Buffer(), v.state.Count())
			if err != nil {
				return engine.ActionResult{}, err
			}

			ac.Editor.MoveCursor(start - ac.Editor.Cursor())

			return engine.ActionResult{}, nil
		},
	}
}

func (v *Vi) moveToEndOfNextNonWhitespaceBackward() *engine.Action {
	return &engine.Action{
		Name: "vi-find-non-whitespace-end-backward",
		Func: func(ac *engine.ActionContext) (engine.ActionResult, error) {
			start, _, err := motionToEndOfNonWhitespaceBackward(ac.Editor.Cursor(), ac.Editor.Buffer(), v.state.Count())
			if err != nil {
				return engine.ActionResult{}, err
			}

			ac.Editor.MoveCursor(start - ac.Editor.Cursor())

			return engine.ActionResult{}, nil
		},
	}
}
