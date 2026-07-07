package engine

import (
	"fmt"

	"github.com/liamg/readline/pkg/editor"
	"github.com/liamg/readline/pkg/history"
	"github.com/liamg/readline/pkg/terminal"
)

// KillRing is a stack of deleted text regions. Kill actions push onto it;
// Yank pastes the most recently pushed entry.
type KillRing struct {
	entries [][]rune
}

// Push appends runes to the kill ring. No-ops on empty input.
func (k *KillRing) Push(runes []rune) {
	if len(runes) == 0 {
		return
	}
	k.entries = append(k.entries, runes)
}

// Peek returns the most recently pushed entry, or nil if the ring is empty.
func (k *KillRing) Peek() []rune {
	if len(k.entries) == 0 {
		return nil
	}
	return k.entries[len(k.entries)-1]
}

// ActionContext is passed to every action. All engine modes share this type;
// fields like Count are simply ignored by modes that don't use them.
type ActionContext struct {
	Editor *editor.Editor
	// Keys holds the key events consumed to trigger this action, including any
	// keys accumulated by preceding continuations in the same chain.
	Keys []terminal.KeyEvent

	History  history.History
	KillRing KillRing
}

func (c *ActionContext) LastKey() terminal.KeyEvent {
	if len(c.Keys) == 0 {
		return terminal.KeyEvent{}
	}
	return c.Keys[len(c.Keys)-1]
}

// ActionResult is returned by every ActionFunc.
type ActionResult struct {
	// Complete signals that the current line should be accepted (accept-line).
	Complete bool
	// Next, if non-nil, tells the engine to call this function with the next
	// key event rather than resuming normal dispatch. Use this for motions that
	// need an additional character, e.g. "tX" or "ci\"".
	Next ActionFunc
	// Keymap, if non-empty, switches the engine to the named keymap after the
	// action returns. Use this for modal editing (e.g. switching between vim
	// insert and normal mode).
	Keymap string

	// SkipReset - we may want to skip resetting the engine state even without setting an explicit Next - e.g. when typing a count before a vim command
	SkipReset bool
}

type ActionFunc func(*ActionContext) (ActionResult, error)

type Action struct {
	Name string
	Func ActionFunc
}

func ComposeMultiAction(name string, actions ...*Action) *Action {
	return &Action{
		Name: name,
		Func: func(ctx *ActionContext) (ActionResult, error) {
			var err error
			var res ActionResult
			for _, a := range actions {
				res, err = a.Func(ctx)
				if err != nil {
					return ActionResult{}, err
				}
				if res.Next != nil || res.Keymap != "" || res.Complete {
					return res, nil
				}
			}
			return res, nil
		},
	}
}

// ActionRegistry maps action names to actions. Registration happens at
// initialisation time and is not concurrent, so a plain map is sufficient.
type ActionRegistry struct {
	actions map[string]*Action
}

func NewRegistry() *ActionRegistry {
	return &ActionRegistry{actions: make(map[string]*Action)}
}

func (r *ActionRegistry) Register(a Action) (*Action, error) {
	if a.Name == "" {
		return nil, fmt.Errorf("action cannot have empty name")
	}
	if a.Func == nil {
		return nil, fmt.Errorf("action cannot have empty function")
	}
	if _, ok := r.actions[a.Name]; ok {
		return nil, fmt.Errorf("action %q is already registered", a.Name)
	}
	r.actions[a.Name] = &a
	return &a, nil
}

func (r *ActionRegistry) MustRegister(a Action) *Action {
	registered, err := r.Register(a)
	if err != nil {
		panic(err)
	}
	return registered
}

func (r *ActionRegistry) Lookup(name string) (*Action, bool) {
	a, ok := r.actions[name]
	return a, ok
}
