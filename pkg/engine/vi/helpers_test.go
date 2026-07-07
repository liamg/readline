package vi

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/liamg/readline/pkg/editor"
	"github.com/liamg/readline/pkg/engine"
	"github.com/liamg/readline/pkg/history"
	"github.com/liamg/readline/pkg/terminal"
)

func key(r rune) terminal.KeyEvent {
	return terminal.KeyEvent{Key: terminal.KeyRune, Rune: r}
}

func namedKey(k terminal.Key) terminal.KeyEvent {
	return terminal.KeyEvent{Key: k}
}

func ctrlKey(r rune) terminal.KeyEvent {
	return terminal.KeyEvent{Key: terminal.KeyRune, Rune: r, Mod: terminal.ModCtrl}
}

func typeString(eng *engine.Engine, s string) {
	for _, r := range s {
		eng.HandleKeyEvent(key(r))
	}
}

func mustHandleKey(t *testing.T, eng *engine.Engine, ev terminal.KeyEvent) (bool, bool) {
	t.Helper()
	done, complete, err := eng.HandleKeyEvent(ev)
	if err != nil {
		t.Fatalf("handle key %v: %v", ev, err)
	}
	return done, complete
}

func mustHandleKeys(t *testing.T, eng *engine.Engine, events ...terminal.KeyEvent) {
	t.Helper()
	for _, ev := range events {
		mustHandleKey(t, eng, ev)
	}
}

func mustHandleRunes(t *testing.T, eng *engine.Engine, s string) {
	t.Helper()
	for _, r := range s {
		mustHandleKey(t, eng, key(r))
	}
}

func moveCursorTo(ed *editor.Editor, pos int) {
	ed.MoveCursor(pos - ed.Cursor())
}

func normalEngine(t *testing.T, buffer string, historyEntries ...string) (*editor.Editor, *engine.Engine, *Vi) {
	t.Helper()
	ed := editor.New()
	if buffer != "" {
		ed.SetBuffer([]rune(buffer))
	}
	historyPath := filepath.Join(t.TempDir(), "history")
	h := history.NewDefaultImplementation(historyPath, 100)
	for i, entry := range historyEntries {
		if i > 0 {
			time.Sleep(time.Millisecond)
		}
		h.Append(entry, false)
	}
	v := New()
	eng := v.BuildEngine(ed, h)
	if _, _, err := eng.HandleKeyEvent(namedKey(terminal.KeyEscape)); err != nil {
		t.Fatalf("switch to normal mode: %v", err)
	}
	return ed, eng, v
}

func visualSelectionEngine(t *testing.T, buffer string, anchor, cursor int) (*editor.Editor, *engine.Engine, *Vi) {
	t.Helper()
	ed := editor.New()
	ed.SetBuffer([]rune(buffer))
	ed.SetClampCursorBeforeEnd(true)
	moveCursorTo(ed, anchor)
	ed.SetSelectionAnchor(editor.SelectionChar)
	moveCursorTo(ed, cursor)
	historyPath := filepath.Join(t.TempDir(), "history")
	h := history.NewDefaultImplementation(historyPath, 100)
	v := New()
	v.state.SetMode(ModeVisual)
	eng := v.BuildEngine(ed, h)
	return ed, eng, v
}

func visualEngine(t *testing.T) (*editor.Editor, *engine.Engine) {
	ed := editor.New()
	historyPath := filepath.Join(t.TempDir(), "history")
	h := history.NewDefaultImplementation(historyPath, 100)
	v := New()
	v.state.SetMode(ModeVisual)
	eng := v.BuildEngine(ed, h)
	return ed, eng
}
