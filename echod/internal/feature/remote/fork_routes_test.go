package remote

// fork: fork_routes.go.

import (
	"errors"
	"net/http"
	"testing"
)

func TestTheUIRoute(t *testing.T) {
	f, _, _ := testRemote(t)
	uiHandler.Store(nil)
	t.Cleanup(func() { uiHandler.Store(nil) })

	if w := call(f, "POST", "/api/v1/ui", `{"version":1}`, true); w.Code != http.StatusNotFound {
		t.Errorf("no native UI: %d", w.Code)
	}
	var got string
	HandleUI(func(b []byte) error {
		if string(b) == "bad" {
			return errors.New("not a UI message")
		}
		got = string(b)
		return nil
	})
	if w := call(f, "POST", "/api/v1/ui", `{"version":1}`, false); w.Code != http.StatusUnauthorized {
		t.Errorf("unsigned: %d", w.Code)
	}
	if got != "" {
		t.Fatal("an unsigned message reached the screen")
	}
	if w := call(f, "POST", "/api/v1/ui", `{"version":1}`, true); w.Code != http.StatusOK || got != `{"version":1}` {
		t.Errorf("signed: %d %s, handed %q", w.Code, w.Body, got)
	}
	if w := call(f, "POST", "/api/v1/ui", "bad", true); w.Code != http.StatusBadRequest {
		t.Errorf("a bad message: %d", w.Code)
	}
	if w := call(f, "GET", "/api/v1/ui", "", true); w.Code != http.StatusMethodNotAllowed {
		t.Errorf("GET: %d", w.Code)
	}
	if w := call(f, "GET", "/api/v1/nothing", "", true); w.Code != http.StatusNotFound {
		t.Errorf("other paths: %d", w.Code)
	}
}
