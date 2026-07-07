package engine

import (
	"testing"

	"github.com/liamg/readline/pkg/keymap"
)

func TestBind_ReplacesExistingBinding(t *testing.T) {
	original := &Action{Name: "original", Func: nopFunc}
	replacement := &Action{Name: "replacement", Func: nopFunc}
	km := &Keymap{
		Bindings: []Binding{
			{Sequence: keymap.MustParseSequence("ctrl-a"), Action: original},
		},
	}

	Bind(km, keymap.MustParseSequence("ctrl-a"), replacement)

	if len(km.Bindings) != 1 {
		t.Fatalf("len = %d, want 1", len(km.Bindings))
	}
	if km.Bindings[0].Action != replacement {
		t.Fatalf("action = %q, want %q", km.Bindings[0].Action.Name, replacement.Name)
	}
}

func TestBind_AppendsNewBinding(t *testing.T) {
	existing := &Action{Name: "existing", Func: nopFunc}
	added := &Action{Name: "added", Func: nopFunc}
	km := &Keymap{
		Bindings: []Binding{
			{Sequence: keymap.MustParseSequence("ctrl-a"), Action: existing},
		},
	}

	Bind(km, keymap.MustParseSequence("ctrl-g"), added)

	if len(km.Bindings) != 2 {
		t.Fatalf("len = %d, want 2", len(km.Bindings))
	}
	if !km.Bindings[1].Sequence.Equal(keymap.MustParseSequence("ctrl-g")) {
		t.Fatalf("added sequence = %s, want ctrl-g", km.Bindings[1].Sequence)
	}
	if km.Bindings[1].Action != added {
		t.Fatalf("action = %q, want %q", km.Bindings[1].Action.Name, added.Name)
	}
}
