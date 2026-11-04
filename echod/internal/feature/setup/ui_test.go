package setup

// fork: the native UI's section (ui.go).

import (
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strings"
	"testing"

	"github.com/HuskerMinion/techo5/echod/internal/config"
)

func TestTheNativeUISectionSaves(t *testing.T) {
	config.Use(filepath.Join(t.TempDir(), "state.json"))
	nativeOffered.Store(true)
	t.Cleanup(func() { nativeOffered.Store(false) })

	w := httptest.NewRecorder()
	uiSection(w, "tok")
	if !strings.Contains(w.Body.String(), "Native UI") {
		t.Fatal("no section")
	}
	if p := saveUI(form(url.Values{"mode": {config.UINative}})); p != "" || !config.Get().UI.Native() {
		t.Fatalf("native not saved (%s)", p)
	}
	if p := saveUI(form(url.Values{"mode": {"hologram"}})); p == "" || !config.Get().UI.Native() {
		t.Errorf("an unknown mode was taken (%q)", p)
	}
	if p := saveUI(form(url.Values{"mode": {""}})); p != "" || config.Get().UI.Native() {
		t.Errorf("standard not saved (%s)", p)
	}
}

func TestABuildWithoutTheNativeUIOffersNone(t *testing.T) {
	config.Use(filepath.Join(t.TempDir(), "state.json"))
	nativeOffered.Store(false)
	w := httptest.NewRecorder()
	uiSection(w, "tok")
	if w.Body.Len() != 0 {
		t.Error("a section was offered")
	}
	if saveUI(form(url.Values{"mode": {config.UINative}})) == "" {
		t.Error("saved on a build without it")
	}
}
