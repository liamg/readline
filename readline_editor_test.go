package readline

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/liamg/readline/pkg/engine"
	"github.com/liamg/readline/pkg/keymap"
)

func writeFakeEditor(t *testing.T, body string) string {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("shell-script fake editor is unix-only")
	}
	dir := t.TempDir()
	script := filepath.Join(dir, "fakeeditor.sh")
	if err := os.WriteFile(script, []byte("#!/bin/sh\n"+body), 0o755); err != nil {
		t.Fatal(err)
	}
	return script
}

func TestRunEditor_RoundTrip(t *testing.T) {
	// Editor that rewrites the file with a marker prefixing the original text.
	script := writeFakeEditor(t, `printf 'EDITED:%s' "$(cat "$1")" > "$1"`+"\n")
	got, err := runEditor(script, "hello")
	if err != nil {
		t.Fatalf("runEditor: %v", err)
	}
	if got != "EDITED:hello" {
		t.Fatalf("runEditor = %q, want %q", got, "EDITED:hello")
	}
}

func TestRunEditor_StripsSingleTrailingNewline(t *testing.T) {
	// Most editors append a trailing newline; we strip exactly one, preserving
	// embedded newlines (multi-line input).
	script := writeFakeEditor(t, `printf 'line1\nline2\n' > "$1"`+"\n")
	got, err := runEditor(script, "")
	if err != nil {
		t.Fatalf("runEditor: %v", err)
	}
	if got != "line1\nline2" {
		t.Fatalf("runEditor = %q, want %q", got, "line1\nline2")
	}
}

func TestRunEditor_WithArguments(t *testing.T) {
	// editorCmd may carry arguments; the temp file is appended as the last arg.
	script := writeFakeEditor(t, `printf 'ok' > "$2"`+"\n") // $1 is --flag, $2 is the file
	got, err := runEditor(script+" --flag", "ignored")
	if err != nil {
		t.Fatalf("runEditor: %v", err)
	}
	if got != "ok" {
		t.Fatalf("runEditor = %q, want %q", got, "ok")
	}
}

func TestRunEditor_EmptyCommand(t *testing.T) {
	if _, err := runEditor("", "x"); err == nil {
		t.Fatal("expected error for empty editor command")
	}
}

func TestRunEditor_NonexistentEditor(t *testing.T) {
	if _, err := runEditor("definitely-not-a-real-editor-binary-xyz", "x"); err == nil {
		t.Fatal("expected error when editor binary cannot be run")
	}
}

func TestBindToAll_BindsSequenceAcrossKeymaps(t *testing.T) {
	noop := &engine.Action{
		Name: "noop",
		Func: func(*engine.ActionContext) (engine.ActionResult, error) {
			return engine.ActionResult{}, nil
		},
	}
	kmA, kmB := &engine.Keymap{}, &engine.Keymap{}
	setOne := map[string]*engine.Keymap{"a": kmA}
	setTwo := map[string]*engine.Keymap{"b": kmB}

	if err := bindToAll(noop, "ctrl-g", setOne, setTwo); err != nil {
		t.Fatalf("bindToAll: %v", err)
	}

	seq := keymap.MustParseSequence("ctrl-g")
	for name, km := range map[string]*engine.Keymap{"a": kmA, "b": kmB} {
		found := false
		for _, b := range km.Bindings {
			if b.Sequence.Equal(seq) {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("keymap %q missing ctrl-g binding", name)
		}
	}
}

func TestBindToAll_InvalidSequence(t *testing.T) {
	noop := &engine.Action{Name: "noop", Func: func(*engine.ActionContext) (engine.ActionResult, error) {
		return engine.ActionResult{}, nil
	}}
	if err := bindToAll(noop, "", map[string]*engine.Keymap{}); err == nil {
		t.Fatal("expected error for invalid sequence")
	}
}
