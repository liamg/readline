package shared

import (
	"testing"

	"github.com/liamg/readline/pkg/engine"
	"github.com/liamg/readline/pkg/keymap"
)

func TestBindingsIncludeTabCompletion(t *testing.T) {
	tab := keymap.MustParseSequence("tab")
	for _, binding := range Bindings {
		if binding.Sequence.Equal(tab) {
			if binding.Action != engine.Complete {
				t.Fatalf("tab action = %q, want %q", binding.Action.Name, engine.Complete.Name)
			}
			return
		}
	}
	t.Fatal("tab completion binding not found")
}

func TestBindingsIncludeDeleteNext(t *testing.T) {
	deleteKey := keymap.MustParseSequence("delete")
	for _, binding := range Bindings {
		if binding.Sequence.Equal(deleteKey) {
			if binding.Action != engine.DeleteNext {
				t.Fatalf("delete action = %q, want %q", binding.Action.Name, engine.DeleteNext.Name)
			}
			return
		}
	}
	t.Fatal("delete binding not found")
}
