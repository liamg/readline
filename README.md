# Readline

A modern, pure-Go line editing library for shells, REPLs, and interactive CLIs.
Designed to be easy to embed, easy to extend, and easy to test, without the historical baggage of GNU Readline.

> [!CAUTION]
> This library is in early development. The **Features** below reflect what is
> implemented today; planned work is listed under **Roadmap**.

## Features

### 🎛️ Input modes

- **Emacs mode** - familiar bindings for movement, deletion, kill/yank, and history navigation
- **Vi mode** - insert, normal, and visual keymaps
- **Custom modes** - define your own keymaps and bind any key sequence to any action

### ⌨️ Key handling

- Full escape-sequence parsing for arrow keys, function keys, and modifier combinations (ctrl, alt/meta)
- Multi-key sequences and operator-pending actions (e.g. vi `r` + char)
- Numeric counts for motions and operators (e.g. `5l`, `d2fx`, `10j`)
- Configurable fallback handler per keymap for unbound keys
- Custom bindings layered on top of any built-in mode
- Which-key hints showing possible key chord continuations

### ✏️ Line editing

- Character, word, and line movement
- Delete previous/next character, word, and to end of line
- Kill and yank: kill text (kill-to-end-of-line, kill-word) and yank it back
- Replace character under cursor
- Transpose characters

### 🗡️ Vi operator model

- Operator-pending mode: `d`, `c`, `y` + motion or text object
- Text objects: word, WORD, bracket pairs (`()`, `[]`, `{}`), and quotes
- Visual character selection
- Named registers for yank/delete

### 🔍 Completion

- Caller-supplied completer: `Complete(line []rune, cursor int) []completion.Group`
- Common-prefix insertion
- Candidate listing below the prompt
- Menu-driven selection with descriptions and groups
- Custom continuation suffixes (e.g. `/` after a directory) via `Candidate.Join`

### 📜 History

- In-memory history
- Persistent file-backed history (stored `0600`)
- Up/down navigation through previous lines
- Prefix search: only lines starting with what you have typed
- Autosuggestions from history (dim inline suggestion, accept with →)
- Leading-space filtering (commands starting with a space are not recorded)

### 🌈 Syntax highlighting

- Caller-supplied highlighter via `WithHighlighter(func([]rune) []ansi.Span)`
- Spans carry ANSI style (bold, italic, underline, foreground/background colour)

### 📐 Multiline editing

- Caller-supplied completeness check via `WithIsComplete(func([]rune) bool)`
- Continuation prompt via `WithContinuation` (can differ from the primary prompt)
- Lines joined and returned as a single string on accept

### 💬 Prompts

- Primary and continuation prompts, re-evaluated on every redraw so they can be dynamic
- Prompt callbacks receive the terminal width and height: `func(w, h int) string`
- Optional status line via `WithStatusLine`
- ANSI- and grapheme-aware width measurement so styled prompts and emoji never misalign the cursor

### 🖥️ Rendering

- Efficient incremental redraws, only changed spans are re-emitted
- Unicode-correct display width (wide CJK characters, combining marks, multi-rune emoji)
- Terminal resize handling, redraws cleanly when the window changes size
- Safe handling when content is taller than the terminal

### ⏸️ Suspend / redraw lifecycle

- `Suspend(fn)` atomically hands the terminal to a subprocess or picker and restores the line afterwards
- Transient log output: print a message above the prompt without corrupting the input line
- Hints below the prompt (e.g. usage info, type hints)
- Edit the current line in `$EDITOR` (or `$VISUAL`) with `ctrl-g`, in both emacs and vi modes

### 🔌 Terminal

- Raw mode with guaranteed restore on exit, panic, or signal
- Pure-Go, no cgo, no C library dependency

### 🏗️ Architecture

The library is layered so that each concern is independently testable:

```
terminal bytes → key event → keymap lookup → action → state mutation → render
```

- **Editor core** - buffer, cursor, selection, and mode state; no TTY dependency; fully unit-tested
- **Engine** - dispatches key events through keymaps and applies action results
- **Renderer** - converts editor state to terminal output; no side effects on the editor
- **Terminal driver** - raw mode, key decoding, resize events

### 🧩 Extensibility

- Register custom actions and bind them to any key sequence
- Compose existing actions into new ones with `ComposeMultiAction`
- Swap the buffer implementation: the `Buffer` interface supports any backing store
- All keymaps, actions, and bindings are plain Go values, no DSL or config format required

## Roadmap

Planned but not yet implemented:

- Undo/redo (`u`, `ctrl-r`)
- Macros
- Visual line (`V`) and block (`ctrl-v`) selection
- Go-to-line motions (`G`, `gg`)
- Incremental reverse history search (`ctrl-r` in emacs)
- Right-aligned prompt
- Transient prompt (collapses to a minimal form after accept)
- Async completions with cancellation
- Overwrite mode
- Bracketed paste
- Sentence and paragraph text objects

## Non-goals

- Full `.inputrc` / GNU Readline compatibility
- Windows support (no plans for v1)
- Bash/Zsh/Vim behavioural parity

## Usage

```go
package main

import (
	"fmt"

	"github.com/liamg/readline"
	"github.com/liamg/readline/pkg/config"
)

func main() {
	rl, err := readline.New(
		"myapp",
		config.WithPrompt(func(w, h int) string { return "» " }),
		config.WithInputMode(config.InputModeVi),
	)
	if err != nil {
		panic(err)
	}
	defer rl.Close()

	for {
		line, err := rl.Readline()
		if err != nil {
			break
		}
		fmt.Println(line)
	}
}
```

See [`_examples`](./_examples) for emacs, vi, live-output, and agent integrations.
