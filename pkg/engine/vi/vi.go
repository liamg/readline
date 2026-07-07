package vi

import (
	"github.com/liamg/readline/pkg/editor"
	"github.com/liamg/readline/pkg/engine"
	"github.com/liamg/readline/pkg/history"
	"github.com/liamg/readline/pkg/terminal"
)

type Vi struct {
	state *State
}

func New() *Vi {
	return &Vi{
		state: NewState(),
	}
}

const (
	ModeInsert = "vi-insert"
	ModeNormal = "vi-normal"
	ModeVisual = "vi-visual"
)

// NewEngine returns an Engine configured for vi editing
func NewEngine(ed *editor.Editor, history history.History) *engine.Engine {
	return New().BuildEngine(ed, history)
}

// BuildKeymaps returns the default vi keymaps before customisation.
func (v *Vi) BuildKeymaps() map[string]*engine.Keymap {
	return map[string]*engine.Keymap{
		ModeInsert: v.buildInsertKeymap(),
		ModeNormal: v.buildNormalKeymap(),
		ModeVisual: v.buildVisualKeymap(),
		// TODO: overwrite mode + more?
	}
}

func (v *Vi) BuildEngine(ed *editor.Editor, history history.History) *engine.Engine {
	return v.BuildEngineWithKeymaps(ed, history, v.BuildKeymaps())
}

// BuildEngineWithKeymaps returns a vi engine using the supplied keymaps.
func (v *Vi) BuildEngineWithKeymaps(ed *editor.Editor, history history.History, keymaps map[string]*engine.Keymap) *engine.Engine {
	return engine.New(
		ed,
		history,
		keymaps,
		v.state.GetMode(),
		v.reset,
	)
}

func (v *Vi) reset() {
	v.state.Reset()
}

func (v *Vi) handleCountOrFallback(ev terminal.KeyEvent) (handled bool) {
	if ev.Key != terminal.KeyRune {
		return false
	}
	if ev.Rune >= '0' && ev.Rune <= '9' {
		v.state.AddCountDigit(ev.Rune)
		return true
	}
	return false
}
