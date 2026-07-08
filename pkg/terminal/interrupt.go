package terminal

import "errors"

// ErrInterrupted is returned from Readline() when the user presses Ctrl-C.
// Consumers should treat it as "cancel current input" and decide whether to
// show a new prompt, propagate a signal, or exit.
var ErrInterrupted = errors.New("interrupted")

// ErrCancelled is returned when the owner cancels an active Readline call.
var ErrCancelled = errors.New("cancelled")
