package engine

import (
	"fmt"

	"github.com/liamg/readline/pkg/editor"
	"github.com/liamg/readline/pkg/history"
	"github.com/liamg/readline/pkg/keymap"
	"github.com/liamg/readline/pkg/terminal"
)

// Engine is a single, mode-agnostic key dispatch engine. Different editing
// modes (basic, emacs, vim) are expressed as different sets of keymaps passed
// to New — the engine logic itself is identical.
type Engine struct {
	keymaps map[string]*Keymap
	active  string
	initial string
	ctx     ActionContext
	pending ActionFunc      // non-nil while waiting for a continuation key
	buffer  keymap.Sequence // keys accumulated for multi-key sequence matching
	reset   func()          // function to call when a fresh key sequence is expected
}

var ErrUnrecognisedBinding = fmt.Errorf("unrecognised key binding")

// New creates an Engine. ed is shared state mutated by actions. keymaps is the
// full set of named keymaps; active is the name of the initially active one.
func New(ed *editor.Editor, hist history.History, keymaps map[string]*Keymap, active string, reset func()) *Engine {
	// TODO: validate keymaps by checking for duplicate bindings
	for name, keymap := range keymaps {
		_ = name
		_ = keymap
	}

	if hist == nil {
		hist = history.Empty{}
	}

	return &Engine{
		keymaps: keymaps,
		active:  active,
		initial: active,
		ctx:     ActionContext{Editor: ed, History: hist},
		reset:   reset,
	}
}

// HandleKeyEvent processes one key event and returns (true, nil) when the
// current line should be accepted. returns accepted, completeBinding, error
func (e *Engine) HandleKeyEvent(event terminal.KeyEvent) (bool, bool, error) {
	// If an action returned a continuation, feed the next key directly to it
	// without going through binding lookup.
	if e.pending != nil {
		e.ctx.Keys = append(e.ctx.Keys, event)
		fn := e.pending
		e.pending = nil
		return e.applyResult(fn(&e.ctx))
	}

	e.buffer = append(e.buffer, event)
	e.ctx.Keys = append(e.ctx.Keys, event)

	km, ok := e.keymaps[e.active]
	if !ok {
		return false, false, fmt.Errorf("unknown keymap %q", e.active)
	}

	var matched *Binding
	var partials int
	for i := range km.Bindings {
		b := &km.Bindings[i]
		match, complete := b.Sequence.Matches(e.buffer)
		if complete {
			matched = b
		} else if match {
			partials++
		}
	}

	// Prefer the longest match: if anything is still partial, keep buffering.
	if partials > 0 {
		return false, false, nil
	}

	buf := e.buffer
	e.buffer = nil

	if matched != nil {
		return e.applyResult(matched.Action.Func(&e.ctx))
	}

	// No match — try the fallback. Resetting is deferred to applyResult so the
	// fallback can accumulate state across keys via SkipReset (e.g. multi-digit
	// vi counts like "12l"); resetting here unconditionally would wipe the count
	// on every digit. An erroring or absent fallback still resets the chain.
	e.ctx.Keys = nil

	if km.Fallback != nil {
		result, ferr := km.Fallback(&e.ctx, event)
		if ferr != nil {
			e.reset()
			return false, false, ferr
		}
		return e.applyResult(result, nil)
	}

	e.reset()
	return false, false, fmt.Errorf("%w: %s", ErrUnrecognisedBinding, keymap.Sequence(buf))
}

// Cursor returns the cursor style declared by the currently active keymap.
func (e *Engine) Cursor() CursorStyle {
	if km, ok := e.keymaps[e.active]; ok {
		return km.Cursor
	}
	return CursorDefault
}

// ActiveKeymap returns the name of the currently active keymap.
func (e *Engine) ActiveKeymap() string {
	return e.active
}

// Reset returns the engine to its initial keymap and clears any pending key
// sequence state.
func (e *Engine) Reset() {
	e.active = e.initial
	e.pending = nil
	e.buffer = nil
	e.ctx.Keys = nil
	if e.reset != nil {
		e.reset()
	}
}

func (e *Engine) applyResult(result ActionResult, err error) (bool, bool, error) {
	if err != nil {
		return false, false, err
	}
	if result.Keymap != "" {
		if _, ok := e.keymaps[result.Keymap]; !ok {
			return false, false, fmt.Errorf("unknown keymap %q", result.Keymap)
		}
		e.active = result.Keymap
	}
	e.pending = result.Next
	if result.Next == nil {
		// Chain is complete; reset per-chain state for the next input.
		e.ctx.Keys = nil
		if !result.SkipReset {
			e.reset()
		}
		return result.Complete, true, nil
	}
	return result.Complete, false, nil
}
