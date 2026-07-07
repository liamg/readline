package engine

import "github.com/liamg/readline/pkg/keymap"

// Bind replaces an existing exact sequence binding or appends a new one.
func Bind(km *Keymap, seq keymap.Sequence, action *Action) {
	for i := range km.Bindings {
		if km.Bindings[i].Sequence.Equal(seq) {
			km.Bindings[i].Action = action
			return
		}
	}
	km.Bindings = append(km.Bindings, Binding{Sequence: seq, Action: action})
}
