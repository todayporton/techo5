package activity

// fork: how a turn ended, for the native UI (feature/display/native_fork.go). Upstream sends it only to
// Home Assistant, as an event; the native UI wants it on the device, at once, to say why a turn that
// was not answered went away.

import (
	"time"

	"github.com/HuskerMinion/techo5/echod/internal/lib/hook"
)

// Outcome is a finished turn's ending: how, and for a canceled or failed one, why.
type Outcome struct {
	How      Ending
	Reason   Reason // a canceled turn's
	Failure  string // a failed turn's code: stt-failed, intent-failed, tts-failed, refused, ...
	TimedOut bool   // the device gave up waiting, rather than something reporting an error
	At       time.Time
}

// Ended fires once per turn, as it ends. It runs on the voice loop: listeners must not block.
var Ended hook.Hook[Outcome]

// outcomeLocked is the turn's ending as Ended reports it. Called under t.mu.
func (t *Turn) outcomeLocked() Outcome {
	return Outcome{How: t.how, Reason: t.reason, Failure: t.failure, TimedOut: t.timedOut, At: t.ended}
}
