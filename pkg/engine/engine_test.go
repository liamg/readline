package engine

import (
	"errors"
	"testing"

	"github.com/liamg/readline/pkg/editor"
	"github.com/liamg/readline/pkg/keymap"
	"github.com/liamg/readline/pkg/terminal"
)

// key builds a KeyEvent for a printable rune with no modifiers.
func key(r rune) terminal.KeyEvent {
	return terminal.KeyEvent{Key: terminal.KeyRune, Rune: r}
}

// namedKey builds a KeyEvent for a named key (no rune).
func namedKey(k terminal.Key) terminal.KeyEvent {
	return terminal.KeyEvent{Key: k}
}

// buildEngine creates a minimal Engine with the provided keymaps, starting in
// the "default" keymap.
func buildEngine(keymaps map[string]*Keymap) *Engine {
	ed := editor.New()
	return New(ed, nil, keymaps, "default", func() {})
}

func TestNewUsesEmptyHistoryWhenHistoryIsNil(t *testing.T) {
	ed := editor.New()
	ed.SetBuffer([]rune("unchanged"))
	e := New(ed, nil, map[string]*Keymap{"default": {}}, "default", func() {})

	if _, err := HistoryPrevious.Func(&e.ctx); err != nil {
		t.Fatalf("history previous: %v", err)
	}
	if got := ed.BufferString(); got != "unchanged" {
		t.Fatalf("buffer = %q, want unchanged", got)
	}
}

// recordingAction returns an Action that records calls into *called and returns
// the supplied ActionResult.
func recordingAction(name string, called *int, result ActionResult) *Action {
	return &Action{
		Name: name,
		Func: func(_ *ActionContext) (ActionResult, error) {
			*called++
			return result, nil
		},
	}
}

func TestEngine_SingleKeyDispatch(t *testing.T) {
	called := 0
	e := buildEngine(map[string]*Keymap{
		"default": {
			Bindings: []Binding{
				{Sequence: keymap.MustParseSequence("a"), Action: recordingAction("test", &called, ActionResult{})},
			},
		},
	})
	done, _, err := e.HandleKeyEvent(key('a'))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if done {
		t.Fatal("expected done=false")
	}
	if called != 1 {
		t.Fatalf("action called %d times, want 1", called)
	}
}

func TestEngine_AcceptLineReturnsTrue(t *testing.T) {
	e := buildEngine(map[string]*Keymap{
		"default": {
			Bindings: []Binding{
				{Sequence: keymap.MustParseSequence("enter"), Action: AcceptLine},
			},
		},
	})
	done, _, err := e.HandleKeyEvent(namedKey(terminal.KeyEnter))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !done {
		t.Fatal("expected done=true for AcceptLine")
	}
}

func TestEngine_UnrecognisedKeyReturnsSentinelError(t *testing.T) {
	e := buildEngine(map[string]*Keymap{
		"default": {Bindings: nil},
	})
	_, _, err := e.HandleKeyEvent(key('z'))
	if !errors.Is(err, ErrUnrecognisedBinding) {
		t.Fatalf("err = %v, want ErrUnrecognisedBinding", err)
	}
}

func TestEngine_FallbackCalledWhenNoMatch(t *testing.T) {
	called := 0
	e := buildEngine(map[string]*Keymap{
		"default": {
			Fallback: func(_ *ActionContext, _ terminal.KeyEvent) (ActionResult, error) {
				called++
				return ActionResult{}, nil
			},
		},
	})
	_, _, err := e.HandleKeyEvent(key('x'))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if called != 1 {
		t.Fatalf("fallback called %d times, want 1", called)
	}
}

func TestEngine_MultiKeySequenceBuffers(t *testing.T) {
	called := 0
	e := buildEngine(map[string]*Keymap{
		"default": {
			Bindings: []Binding{
				{Sequence: keymap.MustParseSequence("ctrl-x,u"), Action: recordingAction("test", &called, ActionResult{})},
			},
		},
	})
	// First key: partial match — action must not fire yet.
	done, _, err := e.HandleKeyEvent(terminal.KeyEvent{Key: terminal.KeyRune, Rune: 'x', Mod: terminal.ModCtrl})
	if err != nil || done || called != 0 {
		t.Fatalf("after first key: done=%v err=%v called=%d", done, err, called)
	}
	// Second key: completes sequence.
	done, _, err = e.HandleKeyEvent(key('u'))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if called != 1 {
		t.Fatalf("action called %d times, want 1", called)
	}
	_ = done
}

func TestEngine_KeymapSwitch(t *testing.T) {
	switchAction := &Action{
		Name: "switch",
		Func: func(_ *ActionContext) (ActionResult, error) {
			return ActionResult{Keymap: "other"}, nil
		},
	}
	otherCalled := 0
	e := buildEngine(map[string]*Keymap{
		"default": {
			Bindings: []Binding{
				{Sequence: keymap.MustParseSequence("a"), Action: switchAction},
			},
		},
		"other": {
			Bindings: []Binding{
				{Sequence: keymap.MustParseSequence("b"), Action: recordingAction("other", &otherCalled, ActionResult{})},
			},
		},
	})
	e.HandleKeyEvent(key('a')) // switch to "other"
	e.HandleKeyEvent(key('b')) // dispatch in "other"
	if otherCalled != 1 {
		t.Fatalf("action in 'other' keymap called %d times, want 1", otherCalled)
	}
}

func TestEngine_ActiveKeymapReflectsCurrentKeymap(t *testing.T) {
	switchAction := &Action{
		Name: "switch",
		Func: func(_ *ActionContext) (ActionResult, error) {
			return ActionResult{Keymap: "other"}, nil
		},
	}
	e := buildEngine(map[string]*Keymap{
		"default": {
			Bindings: []Binding{
				{Sequence: keymap.MustParseSequence("a"), Action: switchAction},
			},
		},
		"other": {},
	})
	if got := e.ActiveKeymap(); got != "default" {
		t.Fatalf("initial ActiveKeymap = %q, want default", got)
	}
	e.HandleKeyEvent(key('a'))
	if got := e.ActiveKeymap(); got != "other" {
		t.Fatalf("after switch ActiveKeymap = %q, want other", got)
	}
}

func TestEngine_ResetReturnsToInitialKeymap(t *testing.T) {
	resetCalled := 0
	switchAction := &Action{
		Name: "switch",
		Func: func(_ *ActionContext) (ActionResult, error) {
			return ActionResult{Keymap: "other"}, nil
		},
	}
	ed := editor.New()
	e := New(ed, nil, map[string]*Keymap{
		"default": {
			Bindings: []Binding{
				{Sequence: keymap.MustParseSequence("a"), Action: switchAction},
			},
		},
		"other": {},
	}, "default", func() { resetCalled++ })

	e.HandleKeyEvent(key('a'))
	if got := e.ActiveKeymap(); got != "other" {
		t.Fatalf("after switch ActiveKeymap = %q, want other", got)
	}
	e.Reset()
	if got := e.ActiveKeymap(); got != "default" {
		t.Fatalf("after reset ActiveKeymap = %q, want default", got)
	}
	if resetCalled == 0 {
		t.Fatal("reset callback was not called")
	}
}

func TestEngine_Continuation(t *testing.T) {
	continuationCalled := 0
	continuationAction := &Action{
		Name: "cont",
		Func: func(_ *ActionContext) (ActionResult, error) {
			return ActionResult{
				Next: func(_ *ActionContext) (ActionResult, error) {
					continuationCalled++
					return ActionResult{}, nil
				},
			}, nil
		},
	}
	e := buildEngine(map[string]*Keymap{
		"default": {
			Bindings: []Binding{
				{Sequence: keymap.MustParseSequence("r"), Action: continuationAction},
			},
		},
	})
	e.HandleKeyEvent(key('r')) // triggers action, sets pending
	e.HandleKeyEvent(key('x')) // feeds next key to pending
	if continuationCalled != 1 {
		t.Fatalf("continuation called %d times, want 1", continuationCalled)
	}
}

func TestEngine_CursorReflectsActiveKeymap(t *testing.T) {
	switchAction := &Action{
		Name: "switch",
		Func: func(_ *ActionContext) (ActionResult, error) {
			return ActionResult{Keymap: "insert"}, nil
		},
	}
	e := buildEngine(map[string]*Keymap{
		"default": {
			Cursor: CursorBlock,
			Bindings: []Binding{
				{Sequence: keymap.MustParseSequence("i"), Action: switchAction},
			},
		},
		"insert": {
			Cursor: CursorBar,
		},
	})
	if e.Cursor() != CursorBlock {
		t.Fatalf("initial cursor = %v, want CursorBlock", e.Cursor())
	}
	e.HandleKeyEvent(key('i'))
	if e.Cursor() != CursorBar {
		t.Fatalf("after switch cursor = %v, want CursorBar", e.Cursor())
	}
}

func TestEngine_UnknownKeymapSwitchReturnsError(t *testing.T) {
	badSwitch := &Action{
		Name: "bad",
		Func: func(_ *ActionContext) (ActionResult, error) {
			return ActionResult{Keymap: "nonexistent"}, nil
		},
	}
	e := buildEngine(map[string]*Keymap{
		"default": {
			Bindings: []Binding{
				{Sequence: keymap.MustParseSequence("a"), Action: badSwitch},
			},
		},
	})
	_, _, err := e.HandleKeyEvent(key('a'))
	if err == nil {
		t.Fatal("expected error switching to nonexistent keymap")
	}
}
