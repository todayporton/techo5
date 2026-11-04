package assistant

// fork: what the assistant's tools found, for the native UI's result panel (feature/display
// native_result_fork.go). Upstream gives a tool's result only to the model.

import "github.com/HuskerMinion/techo5/echod/internal/lib/hook"

// Look is one tool call of a turn: the tool, its arguments as the model sent them (JSON) and what it
// returned to the model.
type Look struct {
	Tool   string
	Args   string
	Result string
}

// Looked fires after each tool call, on the turn's goroutine: listeners must not block.
var Looked hook.Hook[Look]
