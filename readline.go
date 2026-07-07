package readline

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"sync"
	"syscall"

	"github.com/liamg/readline/pkg/config"
	"github.com/liamg/readline/pkg/editor"
	"github.com/liamg/readline/pkg/engine"
	"github.com/liamg/readline/pkg/engine/emacs"
	"github.com/liamg/readline/pkg/engine/vi"
	"github.com/liamg/readline/pkg/keymap"
	"github.com/liamg/readline/pkg/render"
	"github.com/liamg/readline/pkg/terminal"
)

// ErrInterrupted is returned from Readline() when the user presses Ctrl-C.
var ErrInterrupted = terminal.ErrInterrupted

// ErrClosed is returned when using a Readline after Close.
var ErrClosed = errors.New("readline is closed")

type Readline struct {
	cfg           config.Config
	editor        *editor.Editor
	renderer      render.Renderer
	driver        driver
	emacsEngine   Engine
	viEngine      Engine
	activeEngine  Engine
	mu            sync.RWMutex
	outputMu      sync.Mutex
	active        bool
	closed        bool
	suspended     bool
	suspendWrites [][]byte
}

type driver interface {
	Write([]byte) (int, error)
	MakeRaw() error
	Restore() error
	Size() (rows, cols int)
	Read() (terminal.Event, error)
	Close() error
}

type Engine interface {
	HandleKeyEvent(terminal.KeyEvent) (bool, bool, error)
	Cursor() engine.CursorStyle
	ActiveKeymap() string
	Reset()
}

func New(appName string, opts ...config.Option) (*Readline, error) {
	r := &Readline{
		cfg: config.Default(appName),
	}
	for _, opt := range opts {
		opt(&r.cfg)
	}

	emacsKeymaps := emacs.BuildKeymaps()
	viMode := vi.New()
	viKeymaps := viMode.BuildKeymaps()

	// Bind ctrl-g to "edit the current line in $EDITOR" across every keymap.
	// This runs before custom bindings so a user-supplied ctrl-g binding wins.
	if err := bindToAll(r.editInEditorAction(), "ctrl-g", emacsKeymaps, viKeymaps); err != nil {
		return nil, err
	}

	if err := applyCustomBindings(r.cfg.CustomBindings, emacsKeymaps, viKeymaps); err != nil {
		return nil, err
	}

	d, err := terminal.New()
	if err != nil {
		return nil, err
	}
	r.driver = d
	r.editor = editor.New(
		editor.WithSuggester(r.cfg.Suggester),
		editor.WithCompleter(r.cfg.Completer),
	)
	r.emacsEngine = emacs.NewEngineWithKeymaps(r.editor, r.cfg.History, emacsKeymaps)
	r.viEngine = viMode.BuildEngineWithKeymaps(r.editor, r.cfg.History, viKeymaps)
	r.renderer = render.New(r.cfg, r.editor, d)

	switch r.cfg.InputMode {
	case config.InputModeEmacs:
		r.activeEngine = r.emacsEngine
	case config.InputModeVi:
		r.activeEngine = r.viEngine
	default:
		return nil, fmt.Errorf("unsupported input mode '%d'", r.cfg.InputMode)
	}

	// provide the initial terminal size to the renderer
	rows, cols := d.Size()
	r.renderer.SetSize(cols, rows)

	return r, nil
}

func applyCustomBindings(bindings []config.Binding, keymapSets ...map[string]*engine.Keymap) error {
	for _, binding := range bindings {
		if binding.Action == nil {
			return fmt.Errorf("binding %q on keymap %q has nil action", binding.Sequence, binding.Keymap)
		}
		if binding.Action.Func == nil {
			return fmt.Errorf("binding %q on keymap %q has nil action function", binding.Sequence, binding.Keymap)
		}
		seq, err := keymap.ParseSequence(binding.Sequence)
		if err != nil {
			return fmt.Errorf("invalid binding %q on keymap %q: %w", binding.Sequence, binding.Keymap, err)
		}

		var km *engine.Keymap
		for _, keymaps := range keymapSets {
			if candidate, ok := keymaps[string(binding.Keymap)]; ok {
				km = candidate
				break
			}
		}
		if km == nil {
			return fmt.Errorf("unknown keymap %q", binding.Keymap)
		}
		engine.Bind(km, seq, binding.Action)
	}
	return nil
}

// bindToAll binds sequence to action on every keymap in each of the supplied
// keymap sets.
func bindToAll(action *engine.Action, sequence string, keymapSets ...map[string]*engine.Keymap) error {
	seq, err := keymap.ParseSequence(sequence)
	if err != nil {
		return fmt.Errorf("invalid binding sequence %q: %w", sequence, err)
	}
	for _, keymaps := range keymapSets {
		for _, km := range keymaps {
			engine.Bind(km, seq, action)
		}
	}
	return nil
}

// editInEditorAction returns an action that opens the current line in the user's
// configured editor ($VISUAL, then $EDITOR) and replaces the buffer with the
// edited result. It is a no-op error (surfaced as a hint) when no editor is set.
func (r *Readline) editInEditorAction() *engine.Action {
	return &engine.Action{
		Name: "edit-in-editor",
		Func: func(c *engine.ActionContext) (engine.ActionResult, error) {
			return engine.ActionResult{}, r.editCurrentLineInEditor(c.Editor)
		},
	}
}

// editCurrentLineInEditor suspends the terminal, opens the current buffer in the
// user's editor, and loads the edited contents back into ed.
func (r *Readline) editCurrentLineInEditor(ed *editor.Editor) error {
	editorCmd := os.Getenv("VISUAL")
	if editorCmd == "" {
		editorCmd = os.Getenv("EDITOR")
	}
	if strings.TrimSpace(editorCmd) == "" {
		return fmt.Errorf("no editor configured: set $EDITOR or $VISUAL")
	}
	return r.Suspend(func() error {
		edited, err := runEditor(editorCmd, string(ed.Runes()))
		if err != nil {
			return err
		}
		ed.SetBuffer([]rune(edited))
		return nil
	})
}

// runEditor writes content to a temporary file, opens it with editorCmd (which
// may include arguments, e.g. "code -w"), and returns the edited contents with a
// single trailing line ending stripped.
func runEditor(editorCmd, content string) (string, error) {
	fields := strings.Fields(editorCmd)
	if len(fields) == 0 {
		return "", fmt.Errorf("empty editor command")
	}

	f, err := os.CreateTemp("", "readline-*.txt")
	if err != nil {
		return "", err
	}
	name := f.Name()
	defer func() { _ = os.Remove(name) }()

	if _, err := f.WriteString(content); err != nil {
		_ = f.Close()
		return "", err
	}
	if err := f.Close(); err != nil {
		return "", err
	}

	args := append(fields[1:], name)
	cmd := exec.Command(fields[0], args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("editor %q exited with error: %w", fields[0], err)
	}

	out, err := os.ReadFile(name)
	if err != nil {
		return "", err
	}
	edited := strings.TrimSuffix(string(out), "\n")
	edited = strings.TrimSuffix(edited, "\r")
	return edited, nil
}

type AppContext struct {
	Editor *editor.Editor
	Driver *terminal.Driver
}

func (r *Readline) isActive() bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.active
}

func (r *Readline) isClosed() bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.closed
}

func (r *Readline) setActive(active bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.active = active
}

// ActiveKeymap returns the name of the currently active editing keymap.
func (r *Readline) ActiveKeymap() string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if r.activeEngine == nil {
		return ""
	}
	return r.activeEngine.ActiveKeymap()
}

// Close releases the terminal resources owned by Readline. It is safe to call
// more than once.
func (r *Readline) Close() error {
	r.outputMu.Lock()
	defer r.outputMu.Unlock()

	r.mu.Lock()
	if r.closed {
		r.mu.Unlock()
		return nil
	}
	r.closed = true
	wasActive := r.active
	r.active = false
	wasSuspended := r.suspended
	r.suspended = false
	r.suspendWrites = nil
	r.mu.Unlock()

	if r.driver == nil {
		return nil
	}

	var err error
	if r.renderer.HasRenderedContent() {
		err = errors.Join(err, r.renderer.Clear(r.driver))
	}
	if wasActive || wasSuspended {
		err = errors.Join(err, r.driver.Restore())
	}
	if _, cursorErr := r.driver.Write([]byte("\x1b[0 q")); cursorErr != nil {
		err = errors.Join(err, cursorErr)
	}
	return errors.Join(err, r.driver.Close())
}

func (r *Readline) Suspend(f func() error) error {
	if r.isClosed() {
		return ErrClosed
	}
	if !r.isActive() {
		return fmt.Errorf("cannot suspend, readline is not active")
	}

	r.outputMu.Lock()
	if r.isClosed() {
		r.outputMu.Unlock()
		return ErrClosed
	}

	// Clear the current render from the terminal before handing it over.
	if err := r.renderer.Clear(r.driver); err != nil {
		r.outputMu.Unlock()
		return err
	}

	// take the terminal out of raw mode (i.e. restore)
	if err := r.driver.Restore(); err != nil {
		r.outputMu.Unlock()
		return err
	}

	r.suspended = true
	r.outputMu.Unlock()

	// launch the app and let it do its thing
	fnErr := f()

	r.outputMu.Lock()
	defer r.outputMu.Unlock()
	r.suspended = false

	// make the terminal raw again (and save state)
	if err := r.driver.MakeRaw(); err != nil {
		return err
	}

	if err := r.flushSuspendWritesLocked(); err != nil {
		return err
	}
	if fnErr != nil {
		return fnErr
	}

	// Rerender the prompt if this suspend temporarily handed off the terminal
	// while the current Readline call remained active.
	if r.isActive() {
		return r.renderStateLocked()
	}
	return nil
}

// Write writes output above the active prompt. When Readline is active, it
// clears the current prompt, writes p, moves to a fresh line if needed, and
// redraws the prompt with the in-progress input preserved. During Suspend,
// writes are buffered and replayed when the terminal is resumed.
func (r *Readline) Write(p []byte) (int, error) {
	r.outputMu.Lock()
	defer r.outputMu.Unlock()

	if r.isClosed() {
		return 0, ErrClosed
	}

	if len(p) == 0 {
		return 0, nil
	}

	if r.suspended {
		r.suspendWrites = append(r.suspendWrites, bytes.Clone(p))
		return len(p), nil
	}

	if !r.isActive() {
		if r.renderer.HasRenderedContent() {
			if err := r.renderer.Clear(r.driver); err != nil {
				return 0, err
			}
		}
		return r.driver.Write(p)
	}

	if err := r.renderer.Clear(r.driver); err != nil {
		return 0, err
	}
	if err := r.writeOutputLineLocked(p); err != nil {
		return 0, err
	}
	if err := r.renderStateLocked(); err != nil {
		return 0, err
	}
	return len(p), nil
}

// Redraw refreshes the active prompt/status line without writing new output.
// It is safe to call when Readline is inactive or suspended; in those cases it
// does nothing.
func (r *Readline) Redraw() error {
	r.outputMu.Lock()
	defer r.outputMu.Unlock()

	if r.isClosed() {
		return ErrClosed
	}
	if r.suspended || !r.isActive() {
		return nil
	}
	r.renderer.UpdatePrompt()
	r.renderer.UpdateStatusLine()
	if err := r.renderer.Clear(r.driver); err != nil {
		return err
	}
	return r.renderStateLocked()
}

func (r *Readline) flushSuspendWritesLocked() error {
	for _, p := range r.suspendWrites {
		if err := r.writeOutputLineLocked(p); err != nil {
			return err
		}
	}
	r.suspendWrites = nil
	return nil
}

func (r *Readline) writeOutputLineLocked(p []byte) error {
	if _, err := r.driver.Write(normalizeOutputLineEndings(p)); err != nil {
		return err
	}
	if !endsWithNewline(p) {
		if _, err := r.driver.Write([]byte("\r\n")); err != nil {
			return err
		}
	}
	return nil
}

func (r *Readline) Readline() (line string, err error) {
	if r.isClosed() {
		return "", ErrClosed
	}
	r.setActive(true)
	defer r.setActive(false)

	// put terminal in raw mode (and restore after we grabbed our input)
	if err := r.driver.MakeRaw(); err != nil {
		return "", err
	}
	defer func() {
		r.outputMu.Lock()
		defer r.outputMu.Unlock()
		if !r.suspended {
			if restoreErr := r.driver.Restore(); err == nil {
				err = restoreErr
			}
			// Restore terminal default cursor shape on exit.
			_, _ = r.driver.Write([]byte("\x1b[0 q"))
		}
	}()

	// Restore the terminal cleanly if the process is killed externally.
	// We cover SIGINT (external kill -INT, not Ctrl-C which is handled as a
	// key event), SIGTERM, SIGHUP, and SIGPIPE.
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP, syscall.SIGPIPE)
	doneCh := make(chan struct{})
	defer func() {
		signal.Stop(sigCh)
		close(doneCh)
	}()
	go func() {
		select {
		case sig := <-sigCh:
			_ = r.driver.Restore()
			// Stop capturing before re-raising so the signal reaches its
			// default handler (process termination) rather than looping
			// back into sigCh.
			signal.Stop(sigCh)
			p, _ := os.FindProcess(os.Getpid())
			_ = p.Signal(sig)
		case <-doneCh:
			// Readline returned normally; nothing to do.
		}
	}()

	// A previous background Write may have redrawn the prompt after output.
	// Clear that tracked render before resetting renderer state; otherwise the
	// new input cycle forgets how to erase the old prompt chrome.
	r.outputMu.Lock()
	if !r.suspended && r.renderer.HasRenderedContent() {
		if err := r.renderer.Clear(r.driver); err != nil {
			r.outputMu.Unlock()
			return "", err
		}
	}
	r.outputMu.Unlock()

	// reset the buffer contents, as this is fresh input
	r.activeEngine.Reset()
	r.editor.Reset()
	r.renderer.Reset()
	r.cfg.History.Reset()

	// In the normal shell-style path, move past the accepted line. When the
	// accepted line is hidden, the submit path clears the render area instead.
	defer func() {
		r.outputMu.Lock()
		defer r.outputMu.Unlock()
		if !r.suspended && !r.cfg.HideAcceptedLineOnSubmit {
			_, _ = r.driver.Write([]byte("\r\n"))
		}
	}()

	// keep reading until the line is sent
	for {

		// needs to draw prompt, buffer, and cursor position
		if err := r.renderState(); err != nil {
			return "", err
		}
		evt, err := r.driver.Read()
		if err != nil {
			return "", err
		}

		switch event := evt.(type) {
		case terminal.KeyEvent:

			accepted, completeBinding, err := r.activeEngine.HandleKeyEvent(event)
			if err != nil {
				// non-critical error: show a hint so the user understands why their keybinding failed
				r.editor.SetHint("Warning: " + err.Error())
			} else if completeBinding {
				// remove the previous hint after a successful action if the complete sequence is done including continuations
				r.editor.ClearHint()
			}
			r.editor.TriggerAutoSuggestion()

			if accepted {
				line = r.editor.BufferString()
				if r.cfg.IsComplete == nil || r.cfg.IsComplete([]rune(line)) {
					if r.cfg.HideAcceptedLineOnSubmit {
						if err := r.clearState(); err != nil {
							return "", err
						}
					} else {
						if err := r.renderAcceptedState(); err != nil {
							return "", err
						}
					}
					r.renderer.Reset()
					if r.cfg.AutoWriteHistory {
						r.cfg.History.Append(line, true)
					}
					r.activeEngine.Reset()
					return line, nil
				}
				// TODO: we should pass isComplete to the engine somehow so it can make the decision on whether to add a \n
				// this is multiline! only write a new line to the buffer - this will get translated to a CRLF if needed when rendered
				// TODO: should we write a \r\n if we're on windows?
				r.editor.Insert('\n')
			} else {
				if err := r.renderState(); err != nil {
					return "", err
				}
			}

		case terminal.ResizeEvent:
			r.outputMu.Lock()
			if err := r.renderer.ClearForResize(r.driver, event.Cols); err != nil {
				r.outputMu.Unlock()
				return "", err
			}
			r.renderer.SetSize(event.Cols, event.Rows)
			if err := r.renderStateLocked(); err != nil {
				r.outputMu.Unlock()
				return "", err
			}
			r.outputMu.Unlock()
		}
	}
}

// renderState calls RenderState with the current prompt and editor buffer,
// then updates the terminal cursor shape to match the active keymap.
func (r *Readline) renderState() error {
	r.outputMu.Lock()
	defer r.outputMu.Unlock()
	return r.renderStateLocked()
}

func (r *Readline) clearState() error {
	r.outputMu.Lock()
	defer r.outputMu.Unlock()
	if r.suspended {
		return nil
	}
	return r.renderer.Clear(r.driver)
}

func (r *Readline) renderStateLocked() error {
	if r.suspended {
		return nil
	}
	if err := r.renderer.Render(); err != nil {
		return err
	}
	_, err := r.driver.Write(cursorShapeEscape(r.activeEngine.Cursor()))
	return err
}

func (r *Readline) renderAcceptedState() error {
	r.outputMu.Lock()
	defer r.outputMu.Unlock()
	return r.renderAcceptedStateLocked()
}

func (r *Readline) renderAcceptedStateLocked() error {
	if r.suspended {
		return nil
	}
	if err := r.renderer.RenderAccepted(); err != nil {
		return err
	}
	_, err := r.driver.Write(cursorShapeEscape(r.activeEngine.Cursor()))
	return err
}

func normalizeOutputLineEndings(p []byte) []byte {
	if !bytes.Contains(p, []byte{'\n'}) {
		return p
	}
	out := make([]byte, 0, len(p)+bytes.Count(p, []byte{'\n'}))
	for i, b := range p {
		if b == '\n' && (i == 0 || p[i-1] != '\r') {
			out = append(out, '\r')
		}
		out = append(out, b)
	}
	return out
}

func endsWithNewline(p []byte) bool {
	return len(p) > 0 && (p[len(p)-1] == '\n' || p[len(p)-1] == '\r')
}

// cursorShapeEscape returns the DECSCUSR escape sequence for the given style.
// https://vt100.net/docs/vt510-rm/DECSCUSR
func cursorShapeEscape(c engine.CursorStyle) []byte {
	switch c {
	case engine.CursorBlock:
		return []byte("\x1b[2 q") // steady block
	case engine.CursorBar:
		return []byte("\x1b[6 q") // steady bar
	case engine.CursorUnderline:
		return []byte("\x1b[4 q") // steady underline
	default:
		return nil
	}
}
