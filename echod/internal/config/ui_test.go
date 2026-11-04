package config

import (
	"path/filepath"
	"testing"
)

func TestUIModeIsStandardUntilChosen(t *testing.T) {
	Use(filepath.Join(t.TempDir(), "state.json"))
	if Get().UI.Native() {
		t.Fatal("a new device is native")
	}
	if err := Set().UI().Mode(UINative); err != nil {
		t.Fatal(err)
	}
	if !Get().UI.Native() {
		t.Fatal("native was not kept")
	}
	if err := Set().UI().Mode("hologram"); err != nil {
		t.Fatal(err)
	}
	if got := Get().UI.Mode; got != UIStandard {
		t.Fatalf("unknown mode stored as %q, want standard", got)
	}
}

func TestUIModeSurvivesARestart(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.json")
	Use(path)
	if err := Set().UI().Mode(UINative); err != nil {
		t.Fatal(err)
	}
	Use(path)
	if !Get().UI.Native() {
		t.Fatal("native was lost on reload")
	}
}
