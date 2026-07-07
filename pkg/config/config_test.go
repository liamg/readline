package config_test

import (
	"testing"

	"github.com/liamg/readline/pkg/ansi"
	"github.com/liamg/readline/pkg/config"
	"github.com/liamg/readline/pkg/engine"
)

func TestDefaultConfig(t *testing.T) {
	cfg := config.Default("")
	if cfg.Prompt == nil {
		t.Fatal("Prompt should not be nil")
	}
	if cfg.Prompt(80, 30) != "> " {
		t.Fatalf("default prompt = %q, want %q", cfg.Prompt(80, 30), "> ")
	}
	if cfg.Continuation == nil {
		t.Fatal("Continuation should not be nil")
	}
	if cfg.InputMode != config.InputModeEmacs {
		t.Fatalf("default InputMode = %v, want InputModeEmacs", cfg.InputMode)
	}
}

func TestWithPrompt(t *testing.T) {
	cfg := config.Default("")
	config.WithPrompt(func(_, _ int) string { return "$ " })(&cfg)
	if cfg.Prompt(80, 30) != "$ " {
		t.Fatalf("prompt = %q, want %q", cfg.Prompt(80, 30), "$ ")
	}
}

func TestWithHighlighter(t *testing.T) {
	cfg := config.Default("")
	if cfg.Highlighter != nil {
		t.Fatal("default Highlighter should be nil")
	}
	want := []ansi.Span{{}}
	config.WithHighlighter(func([]rune) []ansi.Span { return want })(&cfg)
	if cfg.Highlighter == nil {
		t.Fatal("Highlighter should be set")
	}
	if got := cfg.Highlighter([]rune("x")); len(got) != len(want) {
		t.Fatalf("Highlighter returned %d spans, want %d", len(got), len(want))
	}
}

func TestWithIsComplete(t *testing.T) {
	cfg := config.Default("")
	config.WithIsComplete(func(r []rune) bool { return len(r) > 3 })(&cfg)
	if cfg.IsComplete == nil {
		t.Fatal("IsComplete should be set")
	}
	if cfg.IsComplete([]rune("ab")) {
		t.Fatal("IsComplete(\"ab\") = true, want false")
	}
	if !cfg.IsComplete([]rune("abcd")) {
		t.Fatal("IsComplete(\"abcd\") = false, want true")
	}
}

func TestWithContinuation(t *testing.T) {
	cfg := config.Default("")
	config.WithContinuation(func() string { return "... " })(&cfg)
	if cfg.Continuation == nil {
		t.Fatal("Continuation should be set")
	}
	if got := cfg.Continuation(); got != "... " {
		t.Fatalf("Continuation = %q, want %q", got, "... ")
	}
}

func TestWithInputMode(t *testing.T) {
	cfg := config.Default("")
	config.WithInputMode(config.InputModeVi)(&cfg)
	if cfg.InputMode != config.InputModeVi {
		t.Fatalf("InputMode = %v, want InputModeVi", cfg.InputMode)
	}
}

func TestWithInputMode_Emacs(t *testing.T) {
	cfg := config.Default("")
	config.WithInputMode(config.InputModeEmacs)(&cfg)
	if cfg.InputMode != config.InputModeEmacs {
		t.Fatalf("InputMode = %v, want InputModeEmacs", cfg.InputMode)
	}
}

func TestWithHidePromptOnSubmit(t *testing.T) {
	cfg := config.Default("")
	config.WithHidePromptOnSubmit(true)(&cfg)
	if !cfg.HidePromptOnSubmit {
		t.Fatal("HidePromptOnSubmit should be true")
	}
}

func TestWithHideAcceptedLineOnSubmit(t *testing.T) {
	cfg := config.Default("")
	config.WithHideAcceptedLineOnSubmit(true)(&cfg)
	if !cfg.HideAcceptedLineOnSubmit {
		t.Fatal("HideAcceptedLineOnSubmit should be true")
	}
}

func TestWithBinding(t *testing.T) {
	cfg := config.Default("")
	action := &engine.Action{Name: "test", Func: func(*engine.ActionContext) (engine.ActionResult, error) {
		return engine.ActionResult{}, nil
	}}

	config.WithBinding(config.KeymapEmacs, "ctrl-g", action)(&cfg)

	if len(cfg.CustomBindings) != 1 {
		t.Fatalf("len(CustomBindings) = %d, want 1", len(cfg.CustomBindings))
	}
	got := cfg.CustomBindings[0]
	if got.Keymap != config.KeymapEmacs {
		t.Fatalf("Keymap = %q, want %q", got.Keymap, config.KeymapEmacs)
	}
	if got.Sequence != "ctrl-g" {
		t.Fatalf("Sequence = %q, want ctrl-g", got.Sequence)
	}
	if got.Action != action {
		t.Fatal("Action was not stored")
	}
}

func TestWithBindings(t *testing.T) {
	cfg := config.Default("")
	action := &engine.Action{Name: "test", Func: func(*engine.ActionContext) (engine.ActionResult, error) {
		return engine.ActionResult{}, nil
	}}

	config.WithBindings(
		config.Binding{Keymap: config.KeymapEmacs, Sequence: "ctrl-g", Action: action},
		config.Binding{Keymap: config.KeymapViNormal, Sequence: "x", Action: action},
	)(&cfg)

	if len(cfg.CustomBindings) != 2 {
		t.Fatalf("len(CustomBindings) = %d, want 2", len(cfg.CustomBindings))
	}
}
