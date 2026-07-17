package shared

import (
	"github.com/liamg/readline/pkg/engine"
	"github.com/liamg/readline/pkg/keymap"
)

var Bindings = []engine.Binding{
	{Sequence: keymap.MustParseSequence("enter"), Action: engine.AcceptCompletionOrLine},
	{Sequence: keymap.MustParseSequence("shift-enter"), Action: engine.InsertNewline},
	{Sequence: keymap.MustParseSequence("up"), Action: engine.CompletionPreviousOrHistoryPrevious},
	{Sequence: keymap.MustParseSequence("down"), Action: engine.CompletionNextOrHistoryNext},
	{Sequence: keymap.MustParseSequence("left"), Action: engine.Back},
	{Sequence: keymap.MustParseSequence("right"), Action: engine.AcceptAutosuggestionOrForward},
	{Sequence: keymap.MustParseSequence("tab"), Action: engine.Complete},
	{Sequence: keymap.MustParseSequence("home"), Action: engine.BeginningOfLine},
	{Sequence: keymap.MustParseSequence("end"), Action: engine.EndOfLine},
	{Sequence: keymap.MustParseSequence("delete"), Action: engine.DeleteNext},
}
