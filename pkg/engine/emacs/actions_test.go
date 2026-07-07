package emacs

import (
	"testing"

	"github.com/liamg/readline/pkg/editor"
	"github.com/liamg/readline/pkg/engine"
)

// ---- test helpers ----------------------------------------------------------

func newCtx(buf string, cursor int) *engine.ActionContext {
	ed := editor.New()
	ed.SetBuffer([]rune(buf))
	ed.MoveCursor(cursor - ed.Cursor())
	return &engine.ActionContext{Editor: ed}
}

func newCtxEnd(buf string) *engine.ActionContext {
	return newCtx(buf, len([]rune(buf)))
}

func runAction(t *testing.T, a *engine.Action, ctx *engine.ActionContext) engine.ActionResult {
	t.Helper()
	res, err := a.Func(ctx)
	if err != nil {
		t.Fatalf("%s: unexpected error: %v", a.Name, err)
	}
	return res
}

// ---- ForwardWord / BackwardWord --------------------------------------------

func TestForwardWord_SkipsToEndOfWord(t *testing.T) {
	ctx := newCtx("hello world", 0)
	runAction(t, ForwardWord, ctx)
	if ctx.Editor.Cursor() != 5 {
		t.Fatalf("ForwardWord: cursor = %d, want 5", ctx.Editor.Cursor())
	}
}

func TestForwardWord_SkipsLeadingNonWord(t *testing.T) {
	ctx := newCtx("hello world", 5)
	runAction(t, ForwardWord, ctx)
	if ctx.Editor.Cursor() != 11 {
		t.Fatalf("ForwardWord from space: cursor = %d, want 11", ctx.Editor.Cursor())
	}
}

func TestForwardWord_AtEndIsNoOp(t *testing.T) {
	ctx := newCtxEnd("hello")
	runAction(t, ForwardWord, ctx)
	if ctx.Editor.Cursor() != 5 {
		t.Fatalf("ForwardWord at end: cursor = %d, want 5", ctx.Editor.Cursor())
	}
}

func TestBackwardWord_SkipsToStartOfWord(t *testing.T) {
	ctx := newCtx("hello world", 11)
	runAction(t, BackwardWord, ctx)
	if ctx.Editor.Cursor() != 6 {
		t.Fatalf("BackwardWord: cursor = %d, want 6", ctx.Editor.Cursor())
	}
}

func TestBackwardWord_SkipsLeadingNonWord(t *testing.T) {
	ctx := newCtx("hello world", 6)
	runAction(t, BackwardWord, ctx)
	if ctx.Editor.Cursor() != 0 {
		t.Fatalf("BackwardWord from 'world' start: cursor = %d, want 0", ctx.Editor.Cursor())
	}
}

func TestBackwardWord_AtStartIsNoOp(t *testing.T) {
	ctx := newCtx("hello", 0)
	runAction(t, BackwardWord, ctx)
	if ctx.Editor.Cursor() != 0 {
		t.Fatalf("BackwardWord at start: cursor = %d, want 0", ctx.Editor.Cursor())
	}
}

// ---- KillLine --------------------------------------------------------------

func TestKillLine_DeletesToEndAndPushesKillRing(t *testing.T) {
	ctx := newCtx("hello world", 5)
	runAction(t, KillLine, ctx)
	if ctx.Editor.BufferString() != "hello" {
		t.Fatalf("KillLine: buffer = %q, want %q", ctx.Editor.BufferString(), "hello")
	}
	if ctx.Editor.Cursor() != 5 {
		t.Fatalf("KillLine: cursor = %d, want 5", ctx.Editor.Cursor())
	}
	if got := string(ctx.KillRing.Peek()); got != " world" {
		t.Fatalf("KillLine: kill ring = %q, want %q", got, " world")
	}
}

func TestKillLine_AtEndIsNoOp(t *testing.T) {
	ctx := newCtxEnd("hello")
	runAction(t, KillLine, ctx)
	if ctx.Editor.BufferString() != "hello" {
		t.Fatalf("KillLine at end: buffer changed unexpectedly")
	}
	if ctx.KillRing.Peek() != nil {
		t.Fatal("KillLine at end: should not push to kill ring")
	}
}

func TestKillLine_AtStartDeletesEverything(t *testing.T) {
	ctx := newCtx("hello", 0)
	runAction(t, KillLine, ctx)
	if ctx.Editor.BufferString() != "" {
		t.Fatalf("KillLine from start: buffer = %q, want empty", ctx.Editor.BufferString())
	}
	if got := string(ctx.KillRing.Peek()); got != "hello" {
		t.Fatalf("KillLine from start: kill ring = %q, want %q", got, "hello")
	}
}

// ---- BackwardKillLine ------------------------------------------------------

func TestBackwardKillLine_DeletesToStartAndPushesKillRing(t *testing.T) {
	ctx := newCtx("hello world", 5)
	runAction(t, BackwardKillLine, ctx)
	if ctx.Editor.BufferString() != " world" {
		t.Fatalf("BackwardKillLine: buffer = %q, want %q", ctx.Editor.BufferString(), " world")
	}
	if ctx.Editor.Cursor() != 0 {
		t.Fatalf("BackwardKillLine: cursor = %d, want 0", ctx.Editor.Cursor())
	}
	if got := string(ctx.KillRing.Peek()); got != "hello" {
		t.Fatalf("BackwardKillLine: kill ring = %q, want %q", got, "hello")
	}
}

func TestBackwardKillLine_AtStartIsNoOp(t *testing.T) {
	ctx := newCtx("hello", 0)
	runAction(t, BackwardKillLine, ctx)
	if ctx.Editor.BufferString() != "hello" {
		t.Fatalf("BackwardKillLine at start: buffer changed unexpectedly")
	}
	if ctx.KillRing.Peek() != nil {
		t.Fatal("BackwardKillLine at start: should not push to kill ring")
	}
}

// ---- KillWord --------------------------------------------------------------

func TestKillWord_DeletesNextWord(t *testing.T) {
	ctx := newCtx("hello world", 0)
	runAction(t, KillWord, ctx)
	if ctx.Editor.BufferString() != " world" {
		t.Fatalf("KillWord: buffer = %q, want %q", ctx.Editor.BufferString(), " world")
	}
	if got := string(ctx.KillRing.Peek()); got != "hello" {
		t.Fatalf("KillWord: kill ring = %q, want %q", got, "hello")
	}
}

func TestKillWord_SkipsLeadingNonWordChars(t *testing.T) {
	ctx := newCtx("hello world", 5)
	runAction(t, KillWord, ctx)
	if ctx.Editor.BufferString() != "hello" {
		t.Fatalf("KillWord from space: buffer = %q, want %q", ctx.Editor.BufferString(), "hello")
	}
	if got := string(ctx.KillRing.Peek()); got != " world" {
		t.Fatalf("KillWord from space: kill ring = %q, want %q", got, " world")
	}
}

func TestKillWord_AtEndIsNoOp(t *testing.T) {
	ctx := newCtxEnd("hello")
	runAction(t, KillWord, ctx)
	if ctx.Editor.BufferString() != "hello" {
		t.Fatalf("KillWord at end: buffer changed unexpectedly")
	}
	if ctx.KillRing.Peek() != nil {
		t.Fatal("KillWord at end: should not push to kill ring")
	}
}

// ---- BackwardKillWord ------------------------------------------------------

func TestBackwardKillWord_DeletesPreviousWord(t *testing.T) {
	ctx := newCtxEnd("hello world")
	runAction(t, BackwardKillWord, ctx)
	if ctx.Editor.BufferString() != "hello " {
		t.Fatalf("BackwardKillWord: buffer = %q, want %q", ctx.Editor.BufferString(), "hello ")
	}
	if got := string(ctx.KillRing.Peek()); got != "world" {
		t.Fatalf("BackwardKillWord: kill ring = %q, want %q", got, "world")
	}
}

func TestBackwardKillWord_SkipsTrailingNonWordChars(t *testing.T) {
	ctx := newCtx("hello world", 6)
	runAction(t, BackwardKillWord, ctx)
	if ctx.Editor.BufferString() != "world" {
		t.Fatalf("BackwardKillWord from after space: buffer = %q, want %q", ctx.Editor.BufferString(), "world")
	}
	if got := string(ctx.KillRing.Peek()); got != "hello " {
		t.Fatalf("BackwardKillWord from after space: kill ring = %q, want %q", got, "hello ")
	}
}

func TestBackwardKillWord_AtStartIsNoOp(t *testing.T) {
	ctx := newCtx("hello", 0)
	runAction(t, BackwardKillWord, ctx)
	if ctx.Editor.BufferString() != "hello" {
		t.Fatalf("BackwardKillWord at start: buffer changed unexpectedly")
	}
	if ctx.KillRing.Peek() != nil {
		t.Fatal("BackwardKillWord at start: should not push to kill ring")
	}
}

// ---- TransposeChars --------------------------------------------------------

func TestTransposeChars_SwapsCharWithPrevious(t *testing.T) {
	ctx := newCtx("abcd", 2)
	runAction(t, TransposeChars, ctx)
	if ctx.Editor.BufferString() != "acbd" {
		t.Fatalf("TransposeChars: buffer = %q, want %q", ctx.Editor.BufferString(), "acbd")
	}
	if ctx.Editor.Cursor() != 3 {
		t.Fatalf("TransposeChars: cursor = %d, want 3", ctx.Editor.Cursor())
	}
}

func TestTransposeChars_AtEndSwapsLastTwo(t *testing.T) {
	ctx := newCtxEnd("abcd")
	runAction(t, TransposeChars, ctx)
	if ctx.Editor.BufferString() != "abdc" {
		t.Fatalf("TransposeChars at end: buffer = %q, want %q", ctx.Editor.BufferString(), "abdc")
	}
	if ctx.Editor.Cursor() != 4 {
		t.Fatalf("TransposeChars at end: cursor = %d, want 4", ctx.Editor.Cursor())
	}
}

func TestTransposeChars_SingleCharIsNoOp(t *testing.T) {
	ctx := newCtxEnd("a")
	runAction(t, TransposeChars, ctx)
	if ctx.Editor.BufferString() != "a" {
		t.Fatalf("TransposeChars on single char: buffer changed unexpectedly")
	}
}

func TestTransposeChars_EmptyIsNoOp(t *testing.T) {
	ctx := newCtxEnd("")
	runAction(t, TransposeChars, ctx)
	if ctx.Editor.BufferString() != "" {
		t.Fatalf("TransposeChars on empty: buffer changed unexpectedly")
	}
}

// ---- Yank ------------------------------------------------------------------

func TestYank_InsertsKillRingContent(t *testing.T) {
	ctx := newCtxEnd("hi ")
	ctx.KillRing.Push([]rune("world"))
	runAction(t, Yank, ctx)
	if ctx.Editor.BufferString() != "hi world" {
		t.Fatalf("Yank: buffer = %q, want %q", ctx.Editor.BufferString(), "hi world")
	}
}

func TestYank_EmptyKillRingIsNoOp(t *testing.T) {
	ctx := newCtxEnd("hi")
	runAction(t, Yank, ctx)
	if ctx.Editor.BufferString() != "hi" {
		t.Fatalf("Yank with empty ring: buffer changed unexpectedly")
	}
}

func TestYank_YanksThenKillRoundTrip(t *testing.T) {
	ctx := newCtx("hello world", 0)
	runAction(t, KillWord, ctx) // kills "hello", buffer = " world"

	ctx2 := newCtxEnd("foo ")
	ctx2.KillRing = ctx.KillRing
	runAction(t, Yank, ctx2)
	if ctx2.Editor.BufferString() != "foo hello" {
		t.Fatalf("Yank round-trip: buffer = %q, want %q", ctx2.Editor.BufferString(), "foo hello")
	}
}
