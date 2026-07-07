package engine

import (
	"errors"
	"testing"

	"github.com/liamg/readline/pkg/editor"
)

func nopFunc(_ *ActionContext) (ActionResult, error) { return ActionResult{}, nil }

func TestActionRegistry_Register(t *testing.T) {
	r := NewRegistry()
	a, err := r.Register(Action{Name: "foo", Func: nopFunc})
	if err != nil {
		t.Fatalf("Register error: %v", err)
	}
	if a.Name != "foo" {
		t.Fatalf("Name = %q, want %q", a.Name, "foo")
	}
}

func TestActionRegistry_DuplicateNameErrors(t *testing.T) {
	r := NewRegistry()
	r.Register(Action{Name: "foo", Func: nopFunc})
	_, err := r.Register(Action{Name: "foo", Func: nopFunc})
	if err == nil {
		t.Fatal("expected error for duplicate name")
	}
}

func TestActionRegistry_EmptyNameErrors(t *testing.T) {
	r := NewRegistry()
	_, err := r.Register(Action{Name: "", Func: nopFunc})
	if err == nil {
		t.Fatal("expected error for empty name")
	}
}

func TestActionRegistry_NilFuncErrors(t *testing.T) {
	r := NewRegistry()
	_, err := r.Register(Action{Name: "foo", Func: nil})
	if err == nil {
		t.Fatal("expected error for nil func")
	}
}

func TestActionRegistry_MustRegisterPanicsOnDuplicate(t *testing.T) {
	r := NewRegistry()
	r.MustRegister(Action{Name: "foo", Func: nopFunc})
	defer func() {
		if recover() == nil {
			t.Fatal("expected panic for duplicate MustRegister")
		}
	}()
	r.MustRegister(Action{Name: "foo", Func: nopFunc})
}

func TestActionRegistry_LookupFound(t *testing.T) {
	r := NewRegistry()
	r.Register(Action{Name: "bar", Func: nopFunc})
	a, ok := r.Lookup("bar")
	if !ok || a == nil {
		t.Fatal("expected to find registered action")
	}
}

func TestActionRegistry_LookupNotFound(t *testing.T) {
	r := NewRegistry()
	_, ok := r.Lookup("missing")
	if ok {
		t.Fatal("expected not found for unregistered action")
	}
}

func TestComposeMultiAction_RunsAll(t *testing.T) {
	counts := [3]int{}
	actions := []*Action{
		{Name: "a1", Func: func(_ *ActionContext) (ActionResult, error) { counts[0]++; return ActionResult{}, nil }},
		{Name: "a2", Func: func(_ *ActionContext) (ActionResult, error) { counts[1]++; return ActionResult{}, nil }},
		{Name: "a3", Func: func(_ *ActionContext) (ActionResult, error) { counts[2]++; return ActionResult{}, nil }},
	}
	composed := ComposeMultiAction("all", actions...)
	ctx := &ActionContext{Editor: editor.New()}
	composed.Func(ctx)
	for i, c := range counts {
		if c != 1 {
			t.Errorf("action[%d] called %d times, want 1", i, c)
		}
	}
}

func TestComposeMultiAction_StopsOnComplete(t *testing.T) {
	second := 0
	composed := ComposeMultiAction("stop",
		&Action{Name: "a", Func: func(_ *ActionContext) (ActionResult, error) {
			return ActionResult{Complete: true}, nil
		}},
		&Action{Name: "b", Func: func(_ *ActionContext) (ActionResult, error) {
			second++
			return ActionResult{}, nil
		}},
	)
	ctx := &ActionContext{Editor: editor.New()}
	res, _ := composed.Func(ctx)
	if !res.Complete {
		t.Fatal("expected Complete=true")
	}
	if second != 0 {
		t.Fatal("second action should not have run after Complete")
	}
}

func TestComposeMultiAction_StopsOnKeymap(t *testing.T) {
	second := 0
	composed := ComposeMultiAction("stop",
		&Action{Name: "a", Func: func(_ *ActionContext) (ActionResult, error) {
			return ActionResult{Keymap: "other"}, nil
		}},
		&Action{Name: "b", Func: func(_ *ActionContext) (ActionResult, error) {
			second++
			return ActionResult{}, nil
		}},
	)
	ctx := &ActionContext{Editor: editor.New()}
	res, _ := composed.Func(ctx)
	if res.Keymap != "other" {
		t.Fatalf("Keymap = %q, want %q", res.Keymap, "other")
	}
	if second != 0 {
		t.Fatal("second action should not run after Keymap switch")
	}
}

func TestComposeMultiAction_StopsOnNext(t *testing.T) {
	second := 0
	composed := ComposeMultiAction("stop",
		&Action{Name: "a", Func: func(_ *ActionContext) (ActionResult, error) {
			return ActionResult{Next: nopFunc}, nil
		}},
		&Action{Name: "b", Func: func(_ *ActionContext) (ActionResult, error) {
			second++
			return ActionResult{}, nil
		}},
	)
	ctx := &ActionContext{Editor: editor.New()}
	res, _ := composed.Func(ctx)
	if res.Next == nil {
		t.Fatal("expected Next to be set")
	}
	if second != 0 {
		t.Fatal("second action should not run after Next is set")
	}
}

func TestComposeMultiAction_PropagatesError(t *testing.T) {
	sentinel := errors.New("action failed")
	second := 0
	composed := ComposeMultiAction("err",
		&Action{Name: "a", Func: func(_ *ActionContext) (ActionResult, error) {
			return ActionResult{}, sentinel
		}},
		&Action{Name: "b", Func: func(_ *ActionContext) (ActionResult, error) {
			second++
			return ActionResult{}, nil
		}},
	)
	ctx := &ActionContext{Editor: editor.New()}
	_, err := composed.Func(ctx)
	if !errors.Is(err, sentinel) {
		t.Fatalf("err = %v, want sentinel", err)
	}
	if second != 0 {
		t.Fatal("second action should not run after error")
	}
}
