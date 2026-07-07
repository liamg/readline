package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/liamg/readline/pkg/ansi"
	"github.com/liamg/readline/pkg/editor/completion"
	"github.com/liamg/readline/pkg/editor/suggestion"
	"github.com/liamg/readline/pkg/engine"
	"github.com/liamg/readline/pkg/history"
)

type Config struct {
	AppName                  string
	Prompt                   func(w, h int) string
	StatusLine               func(w, h int) string
	Continuation             func() string
	IsComplete               func([]rune) bool // multiline
	Highlighter              func([]rune) []ansi.Span
	History                  history.History
	InputMode                InputMode
	AutoWriteHistory         bool
	HidePromptOnSubmit       bool
	HideAcceptedLineOnSubmit bool
	Completer                completion.Completer
	Suggester                suggestion.Suggester
	CustomBindings           []Binding
}

type InputMode uint8

const (
	InputModeEmacs InputMode = iota
	InputModeVi
)

// KeymapName identifies a keymap that bindings can be added to or replaced in.
type KeymapName string

const (
	KeymapEmacs    KeymapName = "emacs"
	KeymapViInsert KeymapName = "vi-insert"
	KeymapViNormal KeymapName = "vi-normal"
	KeymapViVisual KeymapName = "vi-visual"
)

// Binding maps a key sequence in a keymap to an action. If the sequence already
// exists in that keymap, the custom action replaces the built-in action.
type Binding struct {
	Keymap   KeymapName
	Sequence string
	Action   *engine.Action
}

func Default(appName string) Config {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		homeDir = os.TempDir()
	}
	historyPath := filepath.Join(homeDir, fmt.Sprintf(".%s_history", appName))
	hist := history.NewDefaultImplementation(historyPath, 10000)
	return Config{
		AppName:          appName,
		History:          hist,
		Suggester:        hist,
		Prompt:           func(_, _ int) string { return "> " },
		Continuation:     func() string { return "> " },
		InputMode:        InputModeEmacs,
		AutoWriteHistory: true,
	}
}

type Option func(*Config)

func WithPrompt(f func(w, h int) string) Option {
	return func(cfg *Config) {
		cfg.Prompt = f
	}
}

func WithInputMode(mode InputMode) Option {
	return func(c *Config) {
		c.InputMode = mode
	}
}

func WithHistoryAutoWrite(enabled bool) Option {
	return func(c *Config) {
		c.AutoWriteHistory = enabled
	}
}

func WithHidePromptOnSubmit(enabled bool) Option {
	return func(c *Config) {
		c.HidePromptOnSubmit = enabled
	}
}

func WithHideAcceptedLineOnSubmit(enabled bool) Option {
	return func(c *Config) {
		c.HideAcceptedLineOnSubmit = enabled
	}
}

func WithCompleter(completer completion.Completer) Option {
	return func(c *Config) {
		c.Completer = completer
	}
}

func WithSuggester(suggester suggestion.Suggester) Option {
	return func(c *Config) {
		c.Suggester = suggester
	}
}

func WithStatusLine(f func(w, h int) string) Option {
	return func(c *Config) {
		c.StatusLine = f
	}
}

// WithHighlighter sets the syntax highlighter callback. It is called with the
// current line and returns styled spans overlaid by the renderer.
func WithHighlighter(f func([]rune) []ansi.Span) Option {
	return func(c *Config) {
		c.Highlighter = f
	}
}

// WithIsComplete sets the multiline completeness callback. When it returns false
// for the accepted line, a newline is inserted and input continues instead of
// returning from Readline.
func WithIsComplete(f func([]rune) bool) Option {
	return func(c *Config) {
		c.IsComplete = f
	}
}

// WithContinuation sets the prompt shown on continuation lines during multiline
// input.
func WithContinuation(f func() string) Option {
	return func(c *Config) {
		c.Continuation = f
	}
}

// WithBinding binds sequence to action in keymap, replacing an existing exact
// sequence binding when present.
func WithBinding(keymap KeymapName, sequence string, action *engine.Action) Option {
	return func(c *Config) {
		c.CustomBindings = append(c.CustomBindings, Binding{
			Keymap:   keymap,
			Sequence: sequence,
			Action:   action,
		})
	}
}

// WithBindings adds multiple custom bindings. Later bindings for the same
// keymap and sequence replace earlier ones.
func WithBindings(bindings ...Binding) Option {
	return func(c *Config) {
		c.CustomBindings = append(c.CustomBindings, bindings...)
	}
}
