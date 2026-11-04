package setup

// fork: the native UI's section (docs/native-ui.md); upstream has no such section.

import (
	"fmt"
	"log/slog"
	"net/http"
	"sync/atomic"

	"github.com/HuskerMinion/techo5/echod/internal/config"
)

// nativeOffered is whether this build draws the native UI: display says so (native_fork.go), so a
// build without it (a Dot, a Spot) offers nothing that would do nothing.
var nativeOffered atomic.Bool

// OfferNativeUI is display saying this build can draw the native UI.
func OfferNativeUI() { nativeOffered.Store(true) }

// uiSection is the choice between the standard screens and the native UI.
func uiSection(w http.ResponseWriter, token string) {
	if !nativeOffered.Load() {
		return
	}
	native := config.Get().UI.Native()
	fmt.Fprint(w, `<fieldset><legend>Native UI</legend><form method="post" action="/setup/save">`)
	hidden(w, token, "ui", "photos")
	fmt.Fprintf(w, `<p><label><input type="radio" name="mode" value="" style="width:auto"%s> Standard screens</label></p>
	 <p><label><input type="radio" name="mode" value="%s" style="width:auto"%s> Native UI: the dashboard and the
	  assistant's own listening, thinking and answer screens in place of the clock and the turn screens</label></p>
	 <p class="note">Calls, alarms, the settings sheet and every other page are drawn as before. A swipe down
	  from the top edge still opens the settings, and the dashboard's Standard button turns this off.</p>
	 <p><button type="submit">Save</button></p></form></fieldset>`,
		checked(!native), config.UINative, checked(native))
}

// saveUI saves the choice. A mode the build does not know is refused rather than stored.
func saveUI(r *http.Request) string {
	if !nativeOffered.Load() {
		return "this device does not draw the native UI"
	}
	mode := r.PostFormValue("mode")
	if mode != config.UIStandard && mode != config.UINative {
		return "that is not a screen this device has"
	}
	if err := config.Set().UI().Mode(mode); err != nil {
		return "could not save: " + err.Error()
	}
	slog.Info("setup page: screens chosen", "native", mode == config.UINative)
	return ""
}
