package remote

// fork: the native UI's route on the remote API (docs/native-ui.md). The Mac mini that answers for the
// device sends it what to show — a turn's result panel, the dashboard's cards — signed like every other
// request here, with the key made on the setup page.
//
//	POST /api/v1/ui   body: a UI message (backend schema/ui-message.v1.schema.json)

import (
	"net/http"
	"sync/atomic"
)

// uiHandler takes a UI message's body; display registers it (native_fork.go). Nil on a build without
// the native UI, where the route answers that there is nothing to show it on.
var uiHandler atomic.Pointer[func(body []byte) error]

// HandleUI registers what takes the native UI's messages.
func HandleUI(fn func(body []byte) error) { uiHandler.Store(&fn) }

// forkRoute answers the fork's routes, and says whether path was one of them.
func forkRoute(w http.ResponseWriter, r *http.Request, path string, body []byte) bool {
	if path != "ui" {
		return false
	}
	if r.Method != http.MethodPost {
		http.Error(w, "POST a UI message", http.StatusMethodNotAllowed)
		return true
	}
	fn := uiHandler.Load()
	if fn == nil {
		http.Error(w, "this device draws no native UI", http.StatusNotFound)
		return true
	}
	if err := (*fn)(body); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return true
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
	return true
}
