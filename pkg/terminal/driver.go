package terminal

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
	"unicode/utf8"

	"golang.org/x/term"
)

type Driver struct {
	mu        sync.Mutex
	ioMu      sync.Mutex
	tty       *os.File
	pending   []byte
	sigwinch  chan os.Signal
	resize    chan ResizeEvent
	interrupt chan struct{}
	termState *term.State
}

var (
	bracketedPasteEnable  = []byte("\x1b[?2004h")
	bracketedPasteDisable = []byte("\x1b[?2004l")
	bracketedPasteStart   = []byte("\x1b[200~")
	bracketedPasteEnd     = []byte("\x1b[201~")
)

const (
	escapeReadTimeout         = 50 * time.Millisecond
	bracketedPasteReadTimeout = 2 * time.Second
)

func New() (*Driver, error) {
	tty, err := os.OpenFile("/dev/tty", os.O_RDWR, 0)
	if err != nil {
		return nil, fmt.Errorf("no controlling terminal: %w", err)
	}
	d := &Driver{
		tty:       tty,
		sigwinch:  make(chan os.Signal, 1),
		resize:    make(chan ResizeEvent, 1),
		interrupt: make(chan struct{}, 1),
	}
	go d.watchResize()
	return d, nil
}

func (d *Driver) InterruptRead() error {
	d.mu.Lock()
	closed := d.tty == nil
	d.mu.Unlock()
	if closed {
		return os.ErrClosed
	}
	select {
	case d.interrupt <- struct{}{}:
	default:
	}
	return nil
}

func (d *Driver) watchResize() {
	for range d.sigwinch {
		rows, cols := d.Size()
		select {
		case d.resize <- ResizeEvent{Rows: rows, Cols: cols}:
		default:
			select {
			case <-d.resize:
			default:
			}
			d.resize <- ResizeEvent{Rows: rows, Cols: cols}
		}
	}
}

func (d *Driver) Close() error {
	signal.Stop(d.sigwinch)
	close(d.sigwinch)
	return d.closeTTY(false)
}

func (d *Driver) MakeRaw() error {
	d.mu.Lock()
	tty := d.tty
	d.mu.Unlock()

	var state *term.State
	err := d.withFd(tty, func(fd int) error {
		var err error
		state, err = term.MakeRaw(fd)
		return err
	})
	if err != nil {
		return err
	}
	if err := d.setNonblock(tty, true); err != nil {
		_ = d.withFd(tty, func(fd int) error {
			return term.Restore(fd, state)
		})
		return err
	}
	d.mu.Lock()
	d.termState = state
	d.mu.Unlock()
	if _, err := d.Write(bracketedPasteEnable); err != nil {
		_ = d.setNonblock(tty, false)
		_ = d.withFd(tty, func(fd int) error {
			return term.Restore(fd, state)
		})
		return err
	}
	signal.Notify(d.sigwinch, syscall.SIGWINCH)
	return nil
}

func (d *Driver) Restore() error {
	signal.Stop(d.sigwinch)
	d.mu.Lock()
	tty := d.tty
	state := d.termState
	d.mu.Unlock()
	_, disableErr := d.Write(bracketedPasteDisable)
	_ = d.setNonblock(tty, false)
	err := d.withFd(tty, func(fd int) error {
		return term.Restore(fd, state)
	})
	return errors.Join(disableErr, err)
}

func (d *Driver) Size() (rows, cols int) {
	d.mu.Lock()
	tty := d.tty
	d.mu.Unlock()

	var c, r int
	err := d.withFd(tty, func(fd int) error {
		var err error
		c, r, err = term.GetSize(fd)
		return err
	})
	if err != nil {
		return 24, 80 // sensible fallback
	}
	return r, c
}

func (d *Driver) closeTTY(reopen bool) error {
	d.mu.Lock()
	tty := d.tty
	d.tty = nil
	d.pending = nil
	d.mu.Unlock()

	var closeErr error
	if tty != nil {
		_ = d.setNonblock(tty, false)
		closeErr = tty.Close()
	}
	if !reopen {
		return closeErr
	}
	opened, openErr := os.OpenFile("/dev/tty", os.O_RDWR, 0)
	if openErr != nil {
		if closeErr != nil {
			return fmt.Errorf("close tty: %w; reopen tty: %v", closeErr, openErr)
		}
		return openErr
	}
	d.mu.Lock()
	d.tty = opened
	d.mu.Unlock()
	return closeErr
}

func (d *Driver) setNonblock(tty *os.File, enabled bool) error {
	return d.withFd(tty, func(fd int) error {
		return syscall.SetNonblock(fd, enabled)
	})
}

// withFd runs fn with the tty file descriptor. It uses SyscallConn so the fd
// is not removed from Go's runtime poller — unlike os.File.Fd(), which puts
// the fd back into blocking mode and permanently breaks SetReadDeadline.
func (d *Driver) withFd(tty *os.File, fn func(fd int) error) error {
	if tty == nil {
		return os.ErrClosed
	}
	rawConn, err := tty.SyscallConn()
	if err != nil {
		return err
	}
	var fnErr error
	ctrlErr := rawConn.Control(func(fd uintptr) {
		fnErr = fn(int(fd))
	})
	if ctrlErr != nil {
		return ctrlErr
	}
	return fnErr
}

// Write implements io.Writer so the renderer can write to the TTY directly.
func (d *Driver) Write(p []byte) (int, error) {
	d.mu.Lock()
	tty := d.tty
	d.mu.Unlock()
	if tty == nil {
		return 0, os.ErrClosed
	}
	d.ioMu.Lock()
	defer d.ioMu.Unlock()
	return writeAll(tty, p)
}

func (d *Driver) CursorPosition() (row, col int, err error) {
	d.ioMu.Lock()
	defer d.ioMu.Unlock()

	d.mu.Lock()
	tty := d.tty
	d.mu.Unlock()
	if tty == nil {
		return 0, 0, os.ErrClosed
	}

	if _, err := writeAll(tty, []byte("\x1b[6n")); err != nil {
		return 0, 0, err
	}

	deadline := time.Now().Add(100 * time.Millisecond)
	var buf []byte
	for time.Now().Before(deadline) {
		tmp := make([]byte, 64)
		n, err := tty.Read(tmp)
		if n > 0 {
			buf = append(buf, tmp[:n]...)
			if row, col, start, consumed, ok := parseCursorPositionResponse(buf); ok {
				preserve := append([]byte{}, buf[:start]...)
				preserve = append(preserve, buf[consumed:]...)
				if len(preserve) > 0 {
					d.prependPending(preserve)
				}
				return row, col, nil
			}
		}
		if err == nil {
			continue
		}
		if errors.Is(err, syscall.EAGAIN) || errors.Is(err, syscall.EWOULDBLOCK) {
			time.Sleep(5 * time.Millisecond)
			continue
		}
		if errors.Is(err, syscall.EINTR) {
			continue
		}
		return 0, 0, err
	}
	if len(buf) > 0 {
		d.prependPending(buf)
	}
	return 0, 0, errReadTimeout
}

func (d *Driver) prependPending(b []byte) {
	if len(b) == 0 {
		return
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	d.pending = append(append([]byte{}, b...), d.pending...)
}

func parseCursorPositionResponse(b []byte) (row, col, start, consumed int, ok bool) {
	start = bytes.Index(b, []byte("\x1b["))
	if start == -1 {
		return 0, 0, 0, 0, false
	}
	endRel := bytes.IndexByte(b[start:], 'R')
	if endRel == -1 {
		return 0, 0, 0, 0, false
	}
	end := start + endRel
	body := string(b[start+2 : end])
	parts := strings.Split(body, ";")
	if len(parts) != 2 {
		return 0, 0, 0, 0, false
	}
	row, err := strconv.Atoi(parts[0])
	if err != nil {
		return 0, 0, 0, 0, false
	}
	col, err = strconv.Atoi(parts[1])
	if err != nil {
		return 0, 0, 0, 0, false
	}
	return row, col, start, end + 1, true
}

func writeAll(w io.Writer, p []byte) (int, error) {
	var written int
	for written < len(p) {
		n, err := w.Write(p[written:])
		if n > 0 {
			written += n
		}
		if err == nil {
			continue
		}
		if errors.Is(err, syscall.EAGAIN) || errors.Is(err, syscall.EWOULDBLOCK) {
			time.Sleep(5 * time.Millisecond)
			continue
		}
		return written, err
	}
	return written, nil
}

// Event is a sealed interface for things the driver can emit.
type Event interface {
	event()
}

// KeyEvent represents a single keypress, including modifier state.
type KeyEvent struct {
	Key  Key
	Rune rune
	Mod  Modifier
}

func (KeyEvent) event() {}

// PasteEvent represents bytes received between terminal bracketed-paste
// delimiters. Its text should be inserted literally, without key binding
// interpretation.
type PasteEvent struct {
	Text string
}

func (PasteEvent) event() {}

// String returns a human-readable representation of the key event, e.g.
// "ctrl+a", "shift+up", "f1", or "a".
func (e KeyEvent) String() string {
	var parts []string
	if e.Mod&ModCtrl != 0 {
		parts = append(parts, "ctrl")
	}
	if e.Mod&ModAlt != 0 {
		parts = append(parts, "alt")
	}
	if e.Mod&ModMeta != 0 {
		parts = append(parts, "meta")
	}
	if e.Mod&ModShift != 0 {
		parts = append(parts, "shift")
	}

	var name string
	if e.Key == KeyRune {
		name = string(e.Rune)
	} else {
		name = e.Key.String()
	}
	parts = append(parts, name)

	result := ""
	for i, p := range parts {
		if i > 0 {
			result += "+"
		}
		result += p
	}
	return result
}

// String returns the name of the key as a lowercase string, e.g. "up", "f1", "enter".
func (k Key) String() string {
	switch k {
	case KeyNone:
		return "none"
	case KeyRune:
		return "rune"
	case KeyUp:
		return "up"
	case KeyDown:
		return "down"
	case KeyLeft:
		return "left"
	case KeyRight:
		return "right"
	case KeyHome:
		return "home"
	case KeyEnd:
		return "end"
	case KeyPageUp:
		return "pageup"
	case KeyPageDown:
		return "pagedown"
	case KeyInsert:
		return "insert"
	case KeyDelete:
		return "delete"
	case KeyBackspace:
		return "backspace"
	case KeyEnter:
		return "enter"
	case KeyTab:
		return "tab"
	case KeyEscape:
		return "escape"
	case KeyF1:
		return "f1"
	case KeyF2:
		return "f2"
	case KeyF3:
		return "f3"
	case KeyF4:
		return "f4"
	case KeyF5:
		return "f5"
	case KeyF6:
		return "f6"
	case KeyF7:
		return "f7"
	case KeyF8:
		return "f8"
	case KeyF9:
		return "f9"
	case KeyF10:
		return "f10"
	case KeyF11:
		return "f11"
	case KeyF12:
		return "f12"
	default:
		return fmt.Sprintf("key(%d)", k)
	}
}

// ResizeEvent is emitted when the terminal window changes size.
type ResizeEvent struct {
	Rows int
	Cols int
}

func (ResizeEvent) event() {}

// Key represents a named key.
type Key uint32

const (
	KeyNone Key = iota
	KeyRune     // printable character; inspect the Rune field
	KeyUp
	KeyDown
	KeyLeft
	KeyRight
	KeyHome
	KeyEnd
	KeyPageUp
	KeyPageDown
	KeyInsert
	KeyDelete
	KeyBackspace
	KeyEnter
	KeyTab
	KeyEscape
	KeyF1
	KeyF2
	KeyF3
	KeyF4
	KeyF5
	KeyF6
	KeyF7
	KeyF8
	KeyF9
	KeyF10
	KeyF11
	KeyF12
)

// Modifier represents keys held alongside a keypress.
type Modifier uint8

const (
	ModShift Modifier = 1 << iota
	ModAlt
	ModCtrl
	ModMeta
)

// Read blocks until a key event is available, then returns it.
// It retries transparently on EINTR (e.g. from SIGWINCH).
// It returns io.EOF when the user presses Ctrl-D or the TTY is closed.
func (d *Driver) Read() (Event, error) {
	for {
		// Deliver any pending resize event before blocking on input.
		select {
		case <-d.interrupt:
			return nil, ErrCancelled
		case ev := <-d.resize:
			return ev, nil
		default:
		}

		// Refill the pending buffer if empty.
		d.mu.Lock()
		pendingLen := len(d.pending)
		d.mu.Unlock()
		if pendingLen == 0 {
			b, ev, err := d.readBytes(0, true)
			if ev != nil {
				return ev, nil
			}
			if err != nil {
				return nil, err
			}
			d.mu.Lock()
			d.pending = b
			d.mu.Unlock()
		}

		// Read more bytes until the escape sequence at the front is complete
		// or the read-ahead times out.
	readEscape:
		for {
			d.mu.Lock()
			incomplete := isIncompleteEscape(d.pending)
			d.mu.Unlock()
			if !incomplete {
				break
			}
			b, ev, err := d.readBytes(d.readAheadTimeout(), false)
			if ev != nil {
				return ev, nil
			}
			if errors.Is(err, errReadTimeout) {
				break readEscape
			}
			if err != nil {
				return nil, err
			}
			d.mu.Lock()
			d.pending = append(d.pending, b...)
			d.mu.Unlock()
		}

		// Parse one event from the front of the pending buffer.
		d.mu.Lock()
		event, n, err := parseEvent(d.pending)
		if n <= len(d.pending) {
			d.pending = d.pending[n:]
		} else {
			d.pending = nil
		}
		d.mu.Unlock()
		if err != nil {
			return nil, err
		}
		return event, nil
	}
}

var errReadTimeout = errors.New("terminal read timeout")

func (d *Driver) readAheadTimeout() time.Duration {
	d.mu.Lock()
	defer d.mu.Unlock()
	if bytes.HasPrefix(d.pending, bracketedPasteStart) {
		return bracketedPasteReadTimeout
	}
	return escapeReadTimeout
}

func (d *Driver) readBytes(timeout time.Duration, drainPending bool) ([]byte, Event, error) {
	deadline := time.Time{}
	if timeout > 0 {
		deadline = time.Now().Add(timeout)
	}
	for {
		if drainPending {
			d.mu.Lock()
			if len(d.pending) > 0 {
				b := append([]byte{}, d.pending...)
				d.pending = nil
				d.mu.Unlock()
				return b, nil, nil
			}
			d.mu.Unlock()
		}

		select {
		case <-d.interrupt:
			return nil, nil, ErrCancelled
		case ev := <-d.resize:
			return nil, ev, nil
		default:
		}

		d.mu.Lock()
		tty := d.tty
		d.mu.Unlock()
		if tty == nil {
			return nil, nil, os.ErrClosed
		}

		buf := make([]byte, 64)
		d.ioMu.Lock()
		n, err := tty.Read(buf)
		d.ioMu.Unlock()
		if n > 0 {
			b := make([]byte, n)
			copy(b, buf[:n])
			return b, nil, nil
		}
		if err == nil {
			continue
		}
		if errors.Is(err, syscall.EAGAIN) || errors.Is(err, syscall.EWOULDBLOCK) {
			if !deadline.IsZero() && time.Now().After(deadline) {
				return nil, nil, errReadTimeout
			}
			time.Sleep(5 * time.Millisecond)
			continue
		}
		if errors.Is(err, syscall.EINTR) {
			continue
		}
		return nil, nil, err
	}
}

// isIncompleteEscape reports whether b looks like the start of an escape
// sequence that hasn't yet received its terminating byte.
func isIncompleteEscape(b []byte) bool {
	if len(b) == 0 || b[0] != 0x1b {
		return false
	}
	if bytes.HasPrefix(bracketedPasteStart, b) {
		return true
	}
	if bytes.HasPrefix(b, bracketedPasteStart) {
		return !bytes.Contains(b[len(bracketedPasteStart):], bracketedPasteEnd)
	}
	if len(b) == 1 {
		return true // bare ESC — may be start of sequence
	}
	switch b[1] {
	case '[': // CSI — terminated by a byte in 0x40–0x7E
		for _, c := range b[2:] {
			if c >= 0x40 && c <= 0x7e {
				return false
			}
		}
		return true // no terminator yet
	case 'O': // SS3 — needs exactly one more byte
		return len(b) < 3
	}
	return false // ESC + unrecognised introducer — treat as complete
}

func parseEvent(b []byte) (Event, int, error) {
	if len(b) == 0 {
		return nil, 0, io.EOF
	}

	// Ctrl-D → EOF
	switch b[0] {
	case 0x03:
		return nil, 1, ErrInterrupted
	case 0x04:
		return nil, 1, io.EOF
	}

	// Escape sequences
	if b[0] == 0x1b {
		if bytes.HasPrefix(b, bracketedPasteStart) {
			payload := b[len(bracketedPasteStart):]
			end := bytes.Index(payload, bracketedPasteEnd)
			if end == -1 {
				return nil, 0, io.EOF
			}
			return PasteEvent{Text: string(payload[:end])}, len(bracketedPasteStart) + end + len(bracketedPasteEnd), nil
		}
		if len(b) == 1 {
			return KeyEvent{Key: KeyEscape}, 1, nil
		}
		switch b[1] {
		case '[':
			ev, n, err := parseCsi(b[2:])
			return ev, n + 2, err
		case 'O':
			ev, n, err := parseSs3(b[2:])
			return ev, n + 2, err
		default:
			// Alt+key: ESC followed by another key
			ev, n, err := parseEvent(b[1:])
			if err != nil {
				return nil, 1, err
			}
			if ke, ok := ev.(KeyEvent); ok {
				ke.Mod |= ModAlt
				return ke, n + 1, nil
			}
			return ev, n + 1, nil
		}
	}

	// Ctrl-A through Ctrl-Z (excluding Ctrl-D handled above)
	if b[0] > 0x00 && b[0] < 0x20 {
		ev, n := parseControlKey(b[0])
		return ev, n, nil
	}

	// DEL
	if b[0] == 0x7f {
		return KeyEvent{Key: KeyBackspace}, 1, nil
	}

	// Printable UTF-8 rune
	r, size := utf8.DecodeRune(b)
	return KeyEvent{Key: KeyRune, Rune: r}, size, nil
}

// parseCsi handles sequences of the form ESC [ ... (CSI).
func parseCsi(b []byte) (Event, int, error) {
	if len(b) == 0 {
		return KeyEvent{Key: KeyEscape}, 0, nil
	}

	var mod Modifier
	if len(b) >= 4 && b[0] == '1' && b[1] == ';' {
		mod = csiModifier(b[2])
		b = b[3:]
	}

	// Find the terminating byte (0x40–0x7E) to know how many bytes to consume.
	end := 0
	for end < len(b) && (b[end] < 0x40 || b[end] > 0x7e) {
		end++
	}
	if end >= len(b) {
		return KeyEvent{Key: KeyEscape}, end, nil
	}
	seq := string(b[:end+1])
	consumed := end + 1
	if ev, ok := parseExtendedCsiKey(seq); ok {
		return ev, consumed, nil
	}

	switch seq {
	case "A":
		return KeyEvent{Key: KeyUp, Mod: mod}, consumed, nil
	case "B":
		return KeyEvent{Key: KeyDown, Mod: mod}, consumed, nil
	case "C":
		return KeyEvent{Key: KeyRight, Mod: mod}, consumed, nil
	case "D":
		return KeyEvent{Key: KeyLeft, Mod: mod}, consumed, nil
	case "H", "1~":
		return KeyEvent{Key: KeyHome, Mod: mod}, consumed, nil
	case "F", "4~":
		return KeyEvent{Key: KeyEnd, Mod: mod}, consumed, nil
	case "Z":
		return KeyEvent{Key: KeyTab, Mod: ModShift | mod}, consumed, nil
	case "2~":
		return KeyEvent{Key: KeyInsert, Mod: mod}, consumed, nil
	case "3~":
		return KeyEvent{Key: KeyDelete, Mod: mod}, consumed, nil
	case "5~":
		return KeyEvent{Key: KeyPageUp, Mod: mod}, consumed, nil
	case "6~":
		return KeyEvent{Key: KeyPageDown, Mod: mod}, consumed, nil
	case "11~", "P":
		return KeyEvent{Key: KeyF1, Mod: mod}, consumed, nil
	case "12~", "Q":
		return KeyEvent{Key: KeyF2, Mod: mod}, consumed, nil
	case "13~", "R":
		return KeyEvent{Key: KeyF3, Mod: mod}, consumed, nil
	case "14~", "S":
		return KeyEvent{Key: KeyF4, Mod: mod}, consumed, nil
	case "15~":
		return KeyEvent{Key: KeyF5, Mod: mod}, consumed, nil
	case "17~":
		return KeyEvent{Key: KeyF6, Mod: mod}, consumed, nil
	case "18~":
		return KeyEvent{Key: KeyF7, Mod: mod}, consumed, nil
	case "19~":
		return KeyEvent{Key: KeyF8, Mod: mod}, consumed, nil
	case "20~":
		return KeyEvent{Key: KeyF9, Mod: mod}, consumed, nil
	case "21~":
		return KeyEvent{Key: KeyF10, Mod: mod}, consumed, nil
	case "23~":
		return KeyEvent{Key: KeyF11, Mod: mod}, consumed, nil
	case "24~":
		return KeyEvent{Key: KeyF12, Mod: mod}, consumed, nil
	}

	return KeyEvent{Key: KeyEscape}, consumed, nil
}

func parseExtendedCsiKey(seq string) (KeyEvent, bool) {
	if strings.HasSuffix(seq, "u") {
		parts := strings.Split(strings.TrimSuffix(seq, "u"), ";")
		if len(parts) < 1 || len(parts) > 2 {
			return KeyEvent{}, false
		}
		code, err := strconv.Atoi(parts[0])
		if err != nil {
			return KeyEvent{}, false
		}
		var mod Modifier
		if len(parts) == 2 {
			modNumber, err := strconv.Atoi(parts[1])
			if err != nil {
				return KeyEvent{}, false
			}
			mod = csiModifierNumber(modNumber)
		}
		switch code {
		case 13:
			return KeyEvent{Key: KeyEnter, Mod: mod}, true
		}
		return KeyEvent{}, false
	}

	if strings.HasSuffix(seq, "~") {
		parts := strings.Split(strings.TrimSuffix(seq, "~"), ";")
		if len(parts) != 3 || parts[0] != "27" {
			return KeyEvent{}, false
		}
		modNumber, err := strconv.Atoi(parts[1])
		if err != nil {
			return KeyEvent{}, false
		}
		code, err := strconv.Atoi(parts[2])
		if err != nil {
			return KeyEvent{}, false
		}
		switch code {
		case 13:
			return KeyEvent{Key: KeyEnter, Mod: csiModifierNumber(modNumber)}, true
		}
	}

	return KeyEvent{}, false
}

// parseSs3 handles sequences of the form ESC O ... (SS3).
func parseSs3(b []byte) (Event, int, error) {
	if len(b) == 0 {
		return KeyEvent{Key: KeyEscape}, 0, nil
	}
	switch b[0] {
	case 'A':
		return KeyEvent{Key: KeyUp}, 1, nil
	case 'B':
		return KeyEvent{Key: KeyDown}, 1, nil
	case 'C':
		return KeyEvent{Key: KeyRight}, 1, nil
	case 'D':
		return KeyEvent{Key: KeyLeft}, 1, nil
	case 'H':
		return KeyEvent{Key: KeyHome}, 1, nil
	case 'F':
		return KeyEvent{Key: KeyEnd}, 1, nil
	case 'P':
		return KeyEvent{Key: KeyF1}, 1, nil
	case 'Q':
		return KeyEvent{Key: KeyF2}, 1, nil
	case 'R':
		return KeyEvent{Key: KeyF3}, 1, nil
	case 'S':
		return KeyEvent{Key: KeyF4}, 1, nil
	}
	return KeyEvent{Key: KeyEscape}, 1, nil
}

// parseControlKey maps bytes 0x01–0x1F (excluding 0x04/Ctrl-D) to KeyEvents.
func parseControlKey(b byte) (KeyEvent, int) {
	switch b {
	case 0x01:
		return KeyEvent{Key: KeyRune, Rune: 'a', Mod: ModCtrl}, 1
	case 0x02:
		return KeyEvent{Key: KeyRune, Rune: 'b', Mod: ModCtrl}, 1
	case 0x03:
		return KeyEvent{Key: KeyRune, Rune: 'c', Mod: ModCtrl}, 1
	case 0x05:
		return KeyEvent{Key: KeyRune, Rune: 'e', Mod: ModCtrl}, 1
	case 0x06:
		return KeyEvent{Key: KeyRune, Rune: 'f', Mod: ModCtrl}, 1
	case 0x07:
		return KeyEvent{Key: KeyRune, Rune: 'g', Mod: ModCtrl}, 1
	case 0x08:
		return KeyEvent{Key: KeyBackspace}, 1
	case 0x09:
		return KeyEvent{Key: KeyTab}, 1
	case 0x0a, 0x0d:
		return KeyEvent{Key: KeyEnter}, 1
	case 0x0b:
		return KeyEvent{Key: KeyRune, Rune: 'k', Mod: ModCtrl}, 1
	case 0x0c:
		return KeyEvent{Key: KeyRune, Rune: 'l', Mod: ModCtrl}, 1
	case 0x0e:
		return KeyEvent{Key: KeyRune, Rune: 'n', Mod: ModCtrl}, 1
	case 0x0f:
		return KeyEvent{Key: KeyRune, Rune: 'o', Mod: ModCtrl}, 1
	case 0x10:
		return KeyEvent{Key: KeyRune, Rune: 'p', Mod: ModCtrl}, 1
	case 0x11:
		return KeyEvent{Key: KeyRune, Rune: 'q', Mod: ModCtrl}, 1
	case 0x12:
		return KeyEvent{Key: KeyRune, Rune: 'r', Mod: ModCtrl}, 1
	case 0x13:
		return KeyEvent{Key: KeyRune, Rune: 's', Mod: ModCtrl}, 1
	case 0x14:
		return KeyEvent{Key: KeyRune, Rune: 't', Mod: ModCtrl}, 1
	case 0x15:
		return KeyEvent{Key: KeyRune, Rune: 'u', Mod: ModCtrl}, 1
	case 0x16:
		return KeyEvent{Key: KeyRune, Rune: 'v', Mod: ModCtrl}, 1
	case 0x17:
		return KeyEvent{Key: KeyRune, Rune: 'w', Mod: ModCtrl}, 1
	case 0x18:
		return KeyEvent{Key: KeyRune, Rune: 'x', Mod: ModCtrl}, 1
	case 0x19:
		return KeyEvent{Key: KeyRune, Rune: 'y', Mod: ModCtrl}, 1
	case 0x1a:
		return KeyEvent{Key: KeyRune, Rune: 'z', Mod: ModCtrl}, 1
	case 0x1b:
		return KeyEvent{Key: KeyEscape}, 1
	default:
		return KeyEvent{Key: KeyRune, Rune: rune(b), Mod: ModCtrl}, 1
	}
}

// csiModifier maps the CSI modifier byte to a Modifier bitmask.
// The encoding is: value = modifier_number - 1, where the modifier number is
// a bitmask of shift(1), alt(2), ctrl(4).
func csiModifier(b byte) Modifier {
	return csiModifierNumber(int(b - '0'))
}

func csiModifierNumber(n int) Modifier {
	v := n - 1
	var mod Modifier
	if v&1 != 0 {
		mod |= ModShift
	}
	if v&2 != 0 {
		mod |= ModAlt
	}
	if v&4 != 0 {
		mod |= ModCtrl
	}
	return mod
}
