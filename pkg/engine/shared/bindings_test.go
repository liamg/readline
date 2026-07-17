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

func TestBindingsIncludeEnterCompletionOrLine(t *testing.T) {
	enter := keymap.MustParseSequence("enter")
	for _, binding := range Bindings {
		if binding.Sequence.Equal(enter) {
			if binding.Action != engine.AcceptCompletionOrLine {
				t.Fatalf("enter action = %q, want %q", binding.Action.Name, engine.AcceptCompletionOrLine.Name)
			}
			return
		}
	}
	t.Fatal("enter binding not found")
}

func TestBindingsIncludeShiftEnterNewline(t *testing.T) {
	shiftEnter := keymap.MustParseSequence("shift-enter")
	for _, binding := range Bindings {
		if binding.Sequence.Equal(shiftEnter) {
			if binding.Action != engine.InsertNewline {
				t.Fatalf("shift-enter action = %q, want %q", binding.Action.Name, engine.InsertNewline.Name)
			}
			return
		}
	}
	t.Fatal("shift-enter newline binding not found")
}

func TestBindingsIncludeCompletionNavigation(t *testing.T) {
	tests := []struct {
		sequence string
		action   *engine.Action
	}{
		{"up", engine.CompletionPreviousOrHistoryPrevious},
		{"down", engine.CompletionNextOrHistoryNext},
	}
	for _, tt := range tests {
		seq := keymap.MustParseSequence(tt.sequence)
		found := false
		for _, binding := range Bindings {
			if binding.Sequence.Equal(seq) {
				found = true
				if binding.Action != tt.action {
					t.Fatalf("%s action = %q, want %q", tt.sequence, binding.Action.Name, tt.action.Name)
				}
			}
		}
		if !found {
			t.Fatalf("%s binding not found", tt.sequence)
		}
	}
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
