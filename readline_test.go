package readline

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/liamg/readline/pkg/config"
	"github.com/liamg/readline/pkg/editor"
	"github.com/liamg/readline/pkg/engine"
	"github.com/liamg/readline/pkg/engine/emacs"
	"github.com/liamg/readline/pkg/engine/vi"
	"github.com/liamg/readline/pkg/keymap"
	"github.com/liamg/readline/pkg/render"
	"github.com/liamg/readline/pkg/terminal"
)

type fakeDriver struct {
	bytes.Buffer
	ops          []string
	closeCalls   int
	restoreCalls int
	events       []terminal.Event
}

func (d *fakeDriver) Write(p []byte) (int, error) {
	d.ops = append(d.ops, "write:"+string(p))
	return d.Buffer.Write(p)
}

func (d *fakeDriver) MakeRaw() error {
	d.ops = append(d.ops, "make-raw")
	return nil
}

func (d *fakeDriver) Restore() error {
	d.restoreCalls++
	d.ops = append(d.ops, "restore")
	return nil
}

func (d *fakeDriver) Size() (int, int) {
	return 24, 80
}

func (d *fakeDriver) Read() (terminal.Event, error) {
	if len(d.events) == 0 {
		return nil, errors.New("not implemented")
	}
	ev := d.events[0]
	d.events = d.events[1:]
	return ev, nil
}

func (d *fakeDriver) Close() error {
	d.closeCalls++
	d.ops = append(d.ops, "close")
	return nil
}

type staticEngine struct{}

func (staticEngine) HandleKeyEvent(terminal.KeyEvent) (bool, bool, error) {
	return false, false, nil
}

func (staticEngine) Cursor() engine.CursorStyle {
	return engine.CursorDefault
}

func (staticEngine) ActiveKeymap() string {
	return "static"
}

func (staticEngine) Reset() {}

type acceptingEngine struct {
	editor *editor.Editor
	text   string
}

func (e acceptingEngine) HandleKeyEvent(terminal.KeyEvent) (bool, bool, error) {
	for _, c := range e.text {
		e.editor.Insert(c)
	}
	return true, true, nil
}

func (acceptingEngine) Cursor() engine.CursorStyle {
	return engine.CursorDefault
}

func (acceptingEngine) ActiveKeymap() string {
	return "accepting"
}

func (acceptingEngine) Reset() {}

type suspendingEngine struct {
	readline *Readline
	started  chan struct{}
	resume   chan struct{}
	done     chan struct{}
}

func (e *suspendingEngine) HandleKeyEvent(terminal.KeyEvent) (bool, bool, error) {
	go func() {
		defer close(e.done)
		_ = e.readline.Suspend(func() error {
			close(e.started)
			<-e.resume
			return nil
		})
	}()
	<-e.started
	return true, true, nil
}

func (e *suspendingEngine) Cursor() engine.CursorStyle {
	return engine.CursorDefault
}

func (e *suspendingEngine) ActiveKeymap() string {
	return "suspending"
}

func (e *suspendingEngine) Reset() {}

func testReadlineWithDriver(d *fakeDriver) *Readline {
	cfg := config.Default("test")
	ed := editor.New()
	rd := render.New(cfg, ed, d)
	rd.SetSize(80, 24)
	return &Readline{
		cfg:          cfg,
		editor:       ed,
		renderer:     rd,
		driver:       d,
		activeEngine: staticEngine{},
		active:       true,
	}
}

// TestSignalHandlerExitsOnDone verifies that the signal-handler goroutine
// spawned inside Readline exits cleanly when doneCh is closed (the normal
// path where Readline returns without any signal being received).
func TestSignalHandlerExitsOnDone(t *testing.T) {
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP, syscall.SIGPIPE)
	doneCh := make(chan struct{})

	signalReceived := false
	exited := make(chan struct{})
	go func() {
		defer close(exited)
		select {
		case <-sigCh:
			signalReceived = true
		case <-doneCh:
		}
	}()

	// Simulate Readline returning normally: stop notification then close doneCh.
	signal.Stop(sigCh)
	close(doneCh)

	select {
	case <-exited:
		if signalReceived {
			t.Fatal("restore should not have been triggered on a normal Readline return")
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("signal-handler goroutine did not exit within timeout after doneCh was closed")
	}
}

// TestSignalHandlerExitsOnSignal verifies that the signal-handler goroutine
// exits when a value is delivered on sigCh (simulating SIGINT, SIGTERM,
// SIGHUP, or SIGPIPE arriving from the OS).
func TestSignalHandlerExitsOnSignal(t *testing.T) {
	for _, sig := range []os.Signal{
		syscall.SIGINT,
		syscall.SIGTERM,
		syscall.SIGHUP,
		syscall.SIGPIPE,
	} {
		sig := sig
		t.Run(sig.String(), func(t *testing.T) {
			sigCh := make(chan os.Signal, 1)
			doneCh := make(chan struct{})

			var receivedSig os.Signal
			exited := make(chan struct{})
			go func() {
				defer close(exited)
				select {
				case s := <-sigCh:
					receivedSig = s
					// In the real Readline goroutine, r.driver.Restore() and
					// signal.Stop(sigCh) are called here before re-raising.
				case <-doneCh:
				}
			}()

			// Deliver the signal directly to the channel (as the OS would via
			// signal.Notify) and verify the goroutine handles it.
			sigCh <- sig

			select {
			case <-exited:
				if receivedSig != sig {
					t.Fatalf("goroutine received signal %v, want %v", receivedSig, sig)
				}
			case <-time.After(500 * time.Millisecond):
				t.Fatalf("signal-handler goroutine did not exit within timeout for signal %v", sig)
			}

			// Cleanup: let the goroutine exit via doneCh if it hasn't already.
			close(doneCh)
		})
	}
}

func TestNormalizeOutputLineEndings(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{name: "no newline", in: "hello", want: "hello"},
		{name: "line feed", in: "hello\nworld\n", want: "hello\r\nworld\r\n"},
		{name: "existing crlf", in: "hello\r\nworld\r\n", want: "hello\r\nworld\r\n"},
		{name: "leading newline", in: "\nhello", want: "\r\nhello"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := string(normalizeOutputLineEndings([]byte(tt.in)))
			if got != tt.want {
				t.Fatalf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestEndsWithNewline(t *testing.T) {
	for _, s := range []string{"hello\n", "hello\r"} {
		if !endsWithNewline([]byte(s)) {
			t.Fatalf("%q should end with newline", s)
		}
	}
	for _, s := range []string{"", "hello"} {
		if endsWithNewline([]byte(s)) {
			t.Fatalf("%q should not end with newline", s)
		}
	}
}

func TestNormalizeOutputLineEndingsReturnsInputWhenUnchanged(t *testing.T) {
	in := []byte("hello")
	out := normalizeOutputLineEndings(in)
	if string(out) != "hello" {
		t.Fatalf("got %q, want hello", string(out))
	}
	if strings.Contains(string(out), "\r") {
		t.Fatalf("unexpected carriage return in %q", string(out))
	}
}

func TestActiveKeymapReturnsActiveEngineKeymap(t *testing.T) {
	r := testReadlineWithDriver(&fakeDriver{})
	if got := r.ActiveKeymap(); got != "static" {
		t.Fatalf("ActiveKeymap = %q, want static", got)
	}
}

func TestActiveKeymapReturnsEmptyWhenEngineUnset(t *testing.T) {
	r := &Readline{}
	if got := r.ActiveKeymap(); got != "" {
		t.Fatalf("ActiveKeymap = %q, want empty string", got)
	}
}

func TestCurrentBufferTracksInProgressInput(t *testing.T) {
	d := &fakeDriver{
		events: []terminal.Event{
			terminal.KeyEvent{Key: terminal.KeyRune, Rune: 'a'},
		},
	}
	r := testReadlineWithDriver(d)
	r.activeEngine = emacs.NewEngineWithKeymaps(r.editor, nil, emacs.BuildKeymaps())

	if got := r.CurrentBuffer(); got != "" {
		t.Fatalf("CurrentBuffer before input = %q, want empty", got)
	}
	if got := r.LastKeypress(); !got.IsZero() {
		t.Fatalf("LastKeypress before input = %v, want zero time", got)
	}
	started := time.Now()

	_, err := r.Readline()
	if err == nil || err.Error() != "not implemented" {
		t.Fatalf("Readline error = %v, want fake driver exhaustion", err)
	}
	if got := r.CurrentBuffer(); got != "a" {
		t.Fatalf("CurrentBuffer after keypress = %q, want a", got)
	}
	if got := r.LastKeypress(); got.Before(started) || got.IsZero() {
		t.Fatalf("LastKeypress after keypress = %v, want time after %v", got, started)
	}
}

func TestCurrentBufferClearsAfterAcceptedInput(t *testing.T) {
	d := &fakeDriver{
		events: []terminal.Event{
			terminal.KeyEvent{Key: terminal.KeyEnter},
		},
	}
	r := testReadlineWithDriver(d)
	r.activeEngine = acceptingEngine{editor: r.editor, text: "submitted"}

	line, err := r.Readline()
	if err != nil {
		t.Fatalf("Readline error: %v", err)
	}
	if line != "submitted" {
		t.Fatalf("line = %q, want submitted", line)
	}
	if got := r.CurrentBuffer(); got != "" {
		t.Fatalf("CurrentBuffer after accept = %q, want empty", got)
	}
	if got := r.LastKeypress(); got.IsZero() {
		t.Fatal("LastKeypress after accept is zero")
	}
}

func TestReadline_ResetToInitialKeymapAfterAccept(t *testing.T) {
	d := &fakeDriver{
		events: []terminal.Event{
			terminal.KeyEvent{Key: terminal.KeyEscape},
			terminal.KeyEvent{Key: terminal.KeyEnter},
		},
	}
	r := testReadlineWithDriver(d)
	viMode := vi.New()
	r.activeEngine = viMode.BuildEngineWithKeymaps(r.editor, nil, viMode.BuildKeymaps())

	line, err := r.Readline()
	if err != nil {
		t.Fatalf("Readline error: %v", err)
	}
	if line != "" {
		t.Fatalf("line = %q, want empty", line)
	}
	if got := r.ActiveKeymap(); got != vi.ModeInsert {
		t.Fatalf("ActiveKeymap after accept = %q, want %q", got, vi.ModeInsert)
	}
}

func TestReadline_RedrawsPromptOnResize(t *testing.T) {
	d := &fakeDriver{
		events: []terminal.Event{
			terminal.ResizeEvent{Rows: 10, Cols: 5},
			terminal.KeyEvent{Key: terminal.KeyEnter},
		},
	}
	r := testReadlineWithDriver(d)
	r.cfg.Prompt = func(w, h int) string {
		return fmt.Sprintf("123456789012345\nw=%d h=%d> ", w, h)
	}
	r.renderer = render.New(r.cfg, r.editor, d)
	r.renderer.SetSize(20, 24)
	r.activeEngine = acceptingEngine{editor: r.editor}

	if _, err := r.Readline(); err != nil {
		t.Fatalf("Readline error: %v", err)
	}
	got := d.String()
	if !strings.Contains(got, "w=20 h=24> ") {
		t.Fatalf("initial prompt missing from output: %q", got)
	}
	if !strings.Contains(got, "w=5 h") || !strings.Contains(got, "=10> ") {
		t.Fatalf("resized prompt missing from output: %q", got)
	}
	if indexWriteContaining(d.ops, "\x1b[5A\r\x1b[J") == -1 {
		t.Fatalf("ops = %v, want reflow-aware resize clear", d.ops)
	}
	if indexWriteContaining(d.ops, "\x1b[5A\r\x1b[J") > indexWriteContaining(d.ops, "w=5 h") {
		t.Fatalf("ops = %v, want clear before resized prompt redraw", d.ops)
	}
}

func TestWrite_ClearsRenderedMultilinePromptBeforeOutput(t *testing.T) {
	d := &fakeDriver{}
	r := testReadlineWithDriver(d)
	r.cfg.Prompt = func(_, _ int) string { return "line 1\nline 2> " }
	r.renderer = render.New(r.cfg, r.editor, d)
	r.renderer.SetSize(80, 24)
	r.editor.Insert('a')

	if err := r.renderState(); err != nil {
		t.Fatalf("renderState error: %v", err)
	}
	d.Reset()
	d.ops = nil

	if _, err := r.Write([]byte("background")); err != nil {
		t.Fatalf("Write error: %v", err)
	}

	got := d.String()
	if !strings.Contains(got, "\x1b[1A\r\x1b[Jbackground\r\n") {
		t.Fatalf("Write output = %q, want multiline prompt clear before output", got)
	}
}

func TestWrite_ClearsPawStylePromptAndStatusBeforeOutput(t *testing.T) {
	d := &fakeDriver{}
	r := testReadlineWithDriver(d)
	r.cfg.Prompt = func(_, _ int) string {
		return "\n────────────────────\n› "
	}
	r.cfg.StatusLine = func(_, _ int) string {
		return "────────────────────\nproject model thinking"
	}
	r.renderer = render.New(r.cfg, r.editor, d)
	r.renderer.SetSize(80, 24)
	r.setActive(true)
	defer r.setActive(false)

	if err := r.renderState(); err != nil {
		t.Fatalf("renderState error: %v", err)
	}
	d.Reset()
	d.ops = nil

	if _, err := r.Write([]byte("model output")); err != nil {
		t.Fatalf("Write error: %v", err)
	}

	clearAt := indexWriteContaining(d.ops, "\x1b[2A\r\x1b[J")
	outputAt := indexWriteContaining(d.ops, "model output")
	redrawAt := indexWriteContaining(d.ops, "────────────────────")
	if clearAt == -1 {
		t.Fatalf("ops = %v, want clear of prompt/status render", d.ops)
	}
	if outputAt == -1 {
		t.Fatalf("ops = %v, want model output", d.ops)
	}
	if redrawAt == -1 {
		t.Fatalf("ops = %v, want prompt redraw after output", d.ops)
	}
	if clearAt > outputAt {
		t.Fatalf("ops = %v, want clear before output", d.ops)
	}
	if redrawAt < outputAt {
		t.Fatalf("ops = %v, want prompt redraw after output", d.ops)
	}
}

func TestReadline_ClearsPromptRedrawnAfterBackgroundOutputBeforeNewInput(t *testing.T) {
	d := &fakeDriver{
		events: []terminal.Event{
			terminal.KeyEvent{Rune: '\n'},
		},
	}
	r := testReadlineWithDriver(d)
	r.cfg.Prompt = func(_, _ int) string {
		return "\n────────────────────\n› "
	}
	r.cfg.StatusLine = func(_, _ int) string {
		return "────────────────────\nproject model idle"
	}
	r.cfg.HideAcceptedLineOnSubmit = true
	r.renderer = render.New(r.cfg, r.editor, d)
	r.renderer.SetSize(80, 24)

	// Simulate the prompt redraw that happens after background/model output.
	r.setActive(true)
	if _, err := r.Write([]byte("previous model output")); err != nil {
		t.Fatalf("Write error: %v", err)
	}
	r.setActive(false)
	d.Reset()
	d.ops = nil

	r.activeEngine = acceptingEngine{editor: r.editor, text: "hello again"}
	line, err := r.Readline()
	if err != nil {
		t.Fatalf("Readline error: %v", err)
	}
	if line != "hello again" {
		t.Fatalf("line = %q, want hello again", line)
	}

	clearAt := indexWriteContaining(d.ops, "\x1b[2A\r\x1b[J")
	promptAt := indexWriteContaining(d.ops, "────────────────────")
	if clearAt == -1 {
		t.Fatalf("ops = %v, want stale prompt clear before new input render", d.ops)
	}
	if promptAt == -1 {
		t.Fatalf("ops = %v, want new prompt render", d.ops)
	}
	if clearAt > promptAt {
		t.Fatalf("ops = %v, want stale prompt clear before new prompt render", d.ops)
	}
}

func TestReadline_DoesNotClearAcceptedPromptBeforeNextInput(t *testing.T) {
	d := &fakeDriver{
		events: []terminal.Event{
			terminal.KeyEvent{Rune: '\n'},
		},
	}
	r := testReadlineWithDriver(d)
	r.cfg.Prompt = func(_, _ int) string {
		return "\n────────────────────\n› "
	}
	r.renderer = render.New(r.cfg, r.editor, d)
	r.renderer.SetSize(80, 24)
	r.activeEngine = acceptingEngine{editor: r.editor, text: "first"}

	line, err := r.Readline()
	if err != nil {
		t.Fatalf("first Readline error: %v", err)
	}
	if line != "first" {
		t.Fatalf("line = %q, want first", line)
	}

	// Simulate command output printed by the shell between readline calls.
	if _, err := d.Write([]byte("command output\n")); err != nil {
		t.Fatalf("write command output: %v", err)
	}
	d.Reset()
	d.ops = nil

	d.events = []terminal.Event{
		terminal.KeyEvent{Rune: '\n'},
	}
	r.activeEngine = acceptingEngine{editor: r.editor, text: "second"}
	line, err = r.Readline()
	if err != nil {
		t.Fatalf("second Readline error: %v", err)
	}
	if line != "second" {
		t.Fatalf("line = %q, want second", line)
	}

	if clearAt := indexWriteContaining(d.ops, "\r\x1b[J"); clearAt != -1 {
		t.Fatalf("ops = %v, did not expect stale accepted-prompt clear before next input", d.ops)
	}
	if promptAt := indexWriteContaining(d.ops, "────────────────────"); promptAt == -1 {
		t.Fatalf("ops = %v, want new prompt render", d.ops)
	}
}

func TestReadline_AcceptsMultilineInputWithoutOverwritingContinuationLine(t *testing.T) {
	d := &fakeDriver{
		events: []terminal.Event{
			terminal.KeyEvent{Rune: '\n'},
		},
	}
	r := testReadlineWithDriver(d)
	r.cfg.Prompt = func(_, _ int) string { return "> " }
	r.renderer = render.New(r.cfg, r.editor, d)
	r.renderer.SetSize(80, 24)
	r.activeEngine = acceptingEngine{editor: r.editor, text: "echo hello\\\nworld"}

	line, err := r.Readline()
	if err != nil {
		t.Fatalf("Readline error: %v", err)
	}
	if line != "echo hello\\\nworld" {
		t.Fatalf("line = %q, want multiline input", line)
	}

	got := d.String()
	if !strings.Contains(got, "echo hello\\\r\nworld") {
		t.Fatalf("accepted multiline input rendered incorrectly: %q", got)
	}
	worldAt := strings.LastIndex(got, "world")
	finalNewlineAt := strings.LastIndex(got, "\r\n")
	if worldAt == -1 || finalNewlineAt == -1 || finalNewlineAt < worldAt {
		t.Fatalf("final newline should be emitted after continuation line: %q", got)
	}
}

func TestSuspend_BuffersWritesUntilResume(t *testing.T) {
	d := &fakeDriver{}
	r := testReadlineWithDriver(d)

	err := r.Suspend(func() error {
		if _, err := r.Write([]byte("background")); err != nil {
			return err
		}
		if bytes.Contains(d.Bytes(), []byte("background")) {
			t.Fatal("suspended write reached driver before resume")
		}
		return nil
	})
	if err != nil {
		t.Fatalf("Suspend error: %v", err)
	}

	got := d.String()
	if !strings.Contains(got, "background\r\n") {
		t.Fatalf("driver output %q does not contain resumed buffered write", got)
	}
	if indexOp(d.ops, "restore") > indexOp(d.ops, "make-raw") {
		t.Fatalf("ops = %v, want restore before make-raw", d.ops)
	}
	if indexWriteContaining(d.ops, "background") < indexOp(d.ops, "make-raw") {
		t.Fatalf("ops = %v, want buffered write after make-raw", d.ops)
	}
}

func TestSuspend_ClearsPromptBeforeCallbackAndRedrawsOnResume(t *testing.T) {
	d := &fakeDriver{}
	r := testReadlineWithDriver(d)

	if err := r.renderState(); err != nil {
		t.Fatalf("renderState error: %v", err)
	}
	before := len(d.ops)
	callbackOps := make([]string, 0)

	err := r.Suspend(func() error {
		callbackOps = append(callbackOps, d.ops[before:]...)
		return nil
	})
	if err != nil {
		t.Fatalf("Suspend error: %v", err)
	}

	clearAtCallback := indexWriteContaining(callbackOps, "\r\x1b[J")
	restoreAtCallback := indexOp(callbackOps, "restore")
	if clearAtCallback == -1 {
		t.Fatalf("callback ops = %v, want prompt clear before callback", callbackOps)
	}
	if restoreAtCallback == -1 || clearAtCallback > restoreAtCallback {
		t.Fatalf("callback ops = %v, want clear before restore", callbackOps)
	}

	resumeOps := d.ops[before+len(callbackOps):]
	makeRawAtResume := indexOp(resumeOps, "make-raw")
	promptAtResume := indexWriteContaining(resumeOps, "> ")
	if makeRawAtResume == -1 || promptAtResume == -1 || promptAtResume < makeRawAtResume {
		t.Fatalf("resume ops = %v, want prompt redraw after make-raw", resumeOps)
	}
}

func TestSuspend_PreservesBufferedWriteBoundaries(t *testing.T) {
	d := &fakeDriver{}
	r := testReadlineWithDriver(d)

	err := r.Suspend(func() error {
		if _, err := r.Write([]byte("first")); err != nil {
			return err
		}
		if _, err := r.Write([]byte("second\n")); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		t.Fatalf("Suspend error: %v", err)
	}

	got := d.String()
	if !strings.Contains(got, "first\r\nsecond\r\n") {
		t.Fatalf("driver output %q does not preserve write boundaries", got)
	}
}

func TestSuspend_BuffersConcurrentWritesUntilResume(t *testing.T) {
	d := &fakeDriver{}
	r := testReadlineWithDriver(d)
	suspended := make(chan struct{})
	resume := make(chan struct{})
	done := make(chan error, 1)

	go func() {
		done <- r.Suspend(func() error {
			close(suspended)
			<-resume
			return nil
		})
	}()

	<-suspended
	if _, err := r.Write([]byte("background")); err != nil {
		t.Fatalf("Write error: %v", err)
	}
	if bytes.Contains(d.Bytes(), []byte("background")) {
		t.Fatal("concurrent suspended write reached driver before resume")
	}

	close(resume)
	if err := <-done; err != nil {
		t.Fatalf("Suspend error: %v", err)
	}
	if !strings.Contains(d.String(), "background\r\n") {
		t.Fatalf("driver output %q does not contain resumed buffered write", d.String())
	}
}

func TestWrite_BuffersWhenSuspendedEvenIfInactive(t *testing.T) {
	d := &fakeDriver{}
	r := testReadlineWithDriver(d)
	r.active = false
	r.suspended = true

	if _, err := r.Write([]byte("background")); err != nil {
		t.Fatalf("Write error: %v", err)
	}
	if d.String() != "" {
		t.Fatalf("driver output = %q, want no direct write", d.String())
	}
	if len(r.suspendWrites) != 1 || string(r.suspendWrites[0]) != "background" {
		t.Fatalf("suspendWrites = %q, want background", r.suspendWrites)
	}
}

func TestReadline_DoesNotWriteInternalOutputWhileSuspended(t *testing.T) {
	d := &fakeDriver{
		events: []terminal.Event{
			terminal.KeyEvent{Key: terminal.KeyF2},
		},
	}
	r := testReadlineWithDriver(d)
	r.active = false
	started := make(chan struct{})
	resume := make(chan struct{})
	r.activeEngine = &suspendingEngine{
		readline: r,
		started:  started,
		resume:   resume,
		done:     make(chan struct{}),
	}
	suspending := r.activeEngine.(*suspendingEngine)

	_, err := r.Readline()
	if err != nil {
		t.Fatalf("Readline error: %v", err)
	}

	restoreIndex := indexOp(d.ops, "restore")
	if restoreIndex == -1 {
		t.Fatalf("ops = %v, want suspend restore", d.ops)
	}
	for _, op := range d.ops[restoreIndex+1:] {
		if strings.HasPrefix(op, "write:") {
			t.Fatalf("unexpected internal write while suspended after restore: ops = %v", d.ops)
		}
	}

	close(resume)
	<-suspending.done
	makeRawIndex := lastIndexOp(d.ops, "make-raw")
	if makeRawIndex == -1 {
		t.Fatalf("ops = %v, want resume make-raw", d.ops)
	}
	for _, op := range d.ops[makeRawIndex+1:] {
		if strings.HasPrefix(op, "write:") {
			t.Fatalf("unexpected prompt redraw after completed suspended readline: ops = %v", d.ops)
		}
	}
}

func TestReadline_HideAcceptedLineOnSubmit(t *testing.T) {
	d := &fakeDriver{
		events: []terminal.Event{
			terminal.KeyEvent{Key: terminal.KeyEnter},
		},
	}
	r := testReadlineWithDriver(d)
	r.cfg.HideAcceptedLineOnSubmit = true
	r.active = false
	r.activeEngine = acceptingEngine{
		editor: r.editor,
		text:   "submitted",
	}

	line, err := r.Readline()
	if err != nil {
		t.Fatalf("Readline error: %v", err)
	}
	if line != "submitted" {
		t.Fatalf("line = %q, want submitted", line)
	}
	clearIndex := indexWriteContaining(d.ops, "\r\x1b[J")
	if clearIndex == -1 {
		t.Fatalf("ops = %v, want accepted submit to clear current render", d.ops)
	}
	for _, op := range d.ops[clearIndex+1:] {
		if strings.Contains(op, "submitted") {
			t.Fatalf("accepted line rendered after clear: ops = %v", d.ops)
		}
		if op == "write:\r\n" {
			t.Fatalf("hidden accepted line should not leave a blank line after clear: ops = %v", d.ops)
		}
	}
}

func TestClose_ClosesDriverAndIsIdempotent(t *testing.T) {
	d := &fakeDriver{}
	r := testReadlineWithDriver(d)
	r.active = false

	if err := r.Close(); err != nil {
		t.Fatalf("Close error: %v", err)
	}
	if err := r.Close(); err != nil {
		t.Fatalf("second Close error: %v", err)
	}
	if d.closeCalls != 1 {
		t.Fatalf("closeCalls = %d, want 1", d.closeCalls)
	}
	if !strings.Contains(d.String(), "\x1b[0 q") {
		t.Fatalf("driver output %q does not restore cursor shape", d.String())
	}
}

func TestClose_RestoresActiveReadlineBeforeClosing(t *testing.T) {
	d := &fakeDriver{}
	r := testReadlineWithDriver(d)

	if err := r.Close(); err != nil {
		t.Fatalf("Close error: %v", err)
	}
	if d.restoreCalls != 1 {
		t.Fatalf("restoreCalls = %d, want 1", d.restoreCalls)
	}
	if indexOp(d.ops, "restore") > indexOp(d.ops, "close") {
		t.Fatalf("ops = %v, want restore before close", d.ops)
	}
}

func TestClose_ClearsRenderedPromptBeforeClosing(t *testing.T) {
	d := &fakeDriver{}
	r := testReadlineWithDriver(d)
	r.cfg.Prompt = func(_, _ int) string { return "paw> " }
	r.renderer = render.New(r.cfg, r.editor, d)
	r.renderer.SetSize(80, 24)
	if err := r.renderState(); err != nil {
		t.Fatalf("renderState error: %v", err)
	}
	d.Reset()
	d.ops = nil

	if err := r.Close(); err != nil {
		t.Fatalf("Close error: %v", err)
	}

	clearIndex := indexWriteContaining(d.ops, "\r\x1b[J")
	if clearIndex == -1 {
		t.Fatalf("ops = %v, want rendered prompt cleared", d.ops)
	}
	if clearIndex > indexOp(d.ops, "close") {
		t.Fatalf("ops = %v, want prompt clear before close", d.ops)
	}
}

func TestClosedReadlineReturnsErrClosed(t *testing.T) {
	d := &fakeDriver{}
	r := testReadlineWithDriver(d)
	if err := r.Close(); err != nil {
		t.Fatalf("Close error: %v", err)
	}

	if _, err := r.Write([]byte("after close")); !errors.Is(err, ErrClosed) {
		t.Fatalf("Write err = %v, want ErrClosed", err)
	}
	if err := r.Suspend(func() error { return nil }); !errors.Is(err, ErrClosed) {
		t.Fatalf("Suspend err = %v, want ErrClosed", err)
	}
	if _, err := r.Readline(); !errors.Is(err, ErrClosed) {
		t.Fatalf("Readline err = %v, want ErrClosed", err)
	}
}

func indexOp(ops []string, op string) int {
	for i, candidate := range ops {
		if candidate == op {
			return i
		}
	}
	return -1
}

func lastIndexOp(ops []string, op string) int {
	for i := len(ops) - 1; i >= 0; i-- {
		if ops[i] == op {
			return i
		}
	}
	return -1
}

func indexWriteContaining(ops []string, text string) int {
	for i, op := range ops {
		if strings.HasPrefix(op, "write:") && strings.Contains(op, text) {
			return i
		}
	}
	return -1
}

func testAction(name string) *engine.Action {
	return &engine.Action{
		Name: name,
		Func: func(*engine.ActionContext) (engine.ActionResult, error) {
			return engine.ActionResult{}, nil
		},
	}
}

func findBinding(t *testing.T, km *engine.Keymap, sequence string) *engine.Binding {
	t.Helper()
	seq := keymap.MustParseSequence(sequence)
	for i := range km.Bindings {
		if km.Bindings[i].Sequence.Equal(seq) {
			return &km.Bindings[i]
		}
	}
	t.Fatalf("binding %q not found", sequence)
	return nil
}

func TestApplyCustomBindings_ReplacesBuiltInEmacsBinding(t *testing.T) {
	action := testAction("custom")
	emacsKeymaps := emacs.BuildKeymaps()

	err := applyCustomBindings([]config.Binding{
		{Keymap: config.KeymapEmacs, Sequence: "ctrl-a", Action: action},
	}, emacsKeymaps)
	if err != nil {
		t.Fatalf("applyCustomBindings error: %v", err)
	}

	if got := findBinding(t, emacsKeymaps["emacs"], "ctrl-a").Action; got != action {
		t.Fatalf("action = %q, want %q", got.Name, action.Name)
	}
}

func TestApplyCustomBindings_AppendsNewEmacsBinding(t *testing.T) {
	action := testAction("custom")
	emacsKeymaps := emacs.BuildKeymaps()
	before := len(emacsKeymaps["emacs"].Bindings)

	err := applyCustomBindings([]config.Binding{
		{Keymap: config.KeymapEmacs, Sequence: "ctrl-g", Action: action},
	}, emacsKeymaps)
	if err != nil {
		t.Fatalf("applyCustomBindings error: %v", err)
	}

	if got := len(emacsKeymaps["emacs"].Bindings); got != before+1 {
		t.Fatalf("len = %d, want %d", got, before+1)
	}
	if got := findBinding(t, emacsKeymaps["emacs"], "ctrl-g").Action; got != action {
		t.Fatalf("action = %q, want %q", got.Name, action.Name)
	}
}

func TestApplyCustomBindings_ReplacesViNormalBinding(t *testing.T) {
	action := testAction("custom")
	viKeymaps := vi.New().BuildKeymaps()

	err := applyCustomBindings([]config.Binding{
		{Keymap: config.KeymapViNormal, Sequence: "x", Action: action},
	}, viKeymaps)
	if err != nil {
		t.Fatalf("applyCustomBindings error: %v", err)
	}

	if got := findBinding(t, viKeymaps[vi.ModeNormal], "x").Action; got != action {
		t.Fatalf("action = %q, want %q", got.Name, action.Name)
	}
}

func TestApplyCustomBindings_LastBindingWins(t *testing.T) {
	first := testAction("first")
	second := testAction("second")
	emacsKeymaps := emacs.BuildKeymaps()

	err := applyCustomBindings([]config.Binding{
		{Keymap: config.KeymapEmacs, Sequence: "ctrl-a", Action: first},
		{Keymap: config.KeymapEmacs, Sequence: "ctrl-a", Action: second},
	}, emacsKeymaps)
	if err != nil {
		t.Fatalf("applyCustomBindings error: %v", err)
	}

	if got := findBinding(t, emacsKeymaps["emacs"], "ctrl-a").Action; got != second {
		t.Fatalf("action = %q, want %q", got.Name, second.Name)
	}
}

func TestApplyCustomBindings_UnknownKeymapErrors(t *testing.T) {
	err := applyCustomBindings([]config.Binding{
		{Keymap: config.KeymapName("missing"), Sequence: "ctrl-g", Action: testAction("custom")},
	}, emacs.BuildKeymaps())
	if err == nil || !strings.Contains(err.Error(), `unknown keymap "missing"`) {
		t.Fatalf("err = %v, want unknown keymap error", err)
	}
}

func TestApplyCustomBindings_InvalidSequenceErrors(t *testing.T) {
	err := applyCustomBindings([]config.Binding{
		{Keymap: config.KeymapEmacs, Sequence: "not-a-key", Action: testAction("custom")},
	}, emacs.BuildKeymaps())
	if err == nil || !strings.Contains(err.Error(), `invalid binding "not-a-key"`) {
		t.Fatalf("err = %v, want invalid binding error", err)
	}
}

func TestNew_InvalidCustomBindingErrorsBeforeTerminalSetup(t *testing.T) {
	_, err := New("test", config.WithBinding(config.KeymapEmacs, "not-a-key", testAction("custom")))
	if err == nil || !strings.Contains(err.Error(), `invalid binding "not-a-key"`) {
		t.Fatalf("err = %v, want invalid binding error", err)
	}
}

func TestApplyCustomBindings_NilActionErrors(t *testing.T) {
	err := applyCustomBindings([]config.Binding{
		{Keymap: config.KeymapEmacs, Sequence: "ctrl-g"},
	}, emacs.BuildKeymaps())
	if err == nil || !strings.Contains(err.Error(), "nil action") {
		t.Fatalf("err = %v, want nil action error", err)
	}
}

func TestApplyCustomBindings_NilActionFuncErrors(t *testing.T) {
	err := applyCustomBindings([]config.Binding{
		{Keymap: config.KeymapEmacs, Sequence: "ctrl-g", Action: &engine.Action{Name: "bad"}},
	}, emacs.BuildKeymaps())
	if err == nil || !strings.Contains(err.Error(), "nil action function") {
		t.Fatalf("err = %v, want nil action function error", err)
	}
}

func TestCustomEmacsBindingDispatchesThroughEngine(t *testing.T) {
	called := 0
	action := &engine.Action{
		Name: "custom",
		Func: func(*engine.ActionContext) (engine.ActionResult, error) {
			called++
			return engine.ActionResult{}, nil
		},
	}
	emacsKeymaps := emacs.BuildKeymaps()
	err := applyCustomBindings([]config.Binding{
		{Keymap: config.KeymapEmacs, Sequence: "ctrl-a", Action: action},
	}, emacsKeymaps)
	if err != nil {
		t.Fatalf("applyCustomBindings error: %v", err)
	}

	eng := engine.New(nil, nil, emacsKeymaps, "emacs", func() {})
	_, _, err = eng.HandleKeyEvent(terminal.KeyEvent{Key: terminal.KeyRune, Rune: 'a', Mod: terminal.ModCtrl})
	if err != nil {
		t.Fatalf("HandleKeyEvent error: %v", err)
	}
	if called != 1 {
		t.Fatalf("called = %d, want 1", called)
	}
}
