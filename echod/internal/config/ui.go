package config

// fork: this file is the native UI fork's (docs/native-ui.md); upstream has no UI section.

// UI is which screen the device draws while nothing above it has the screen: the standard clock and
// turn screens, or the native UI (feature/nativeui) in their place. Calls, rings, the settings sheet
// and every other page stay where they are either way.
type UI struct {
	// Mode is UIStandard or UINative. Anything else is read as standard, so a file from a build with
	// a mode this one does not know still leaves the device a screen it can use.
	Mode string `json:"mode,omitempty"`
}

const (
	UIStandard = ""
	UINative   = "native"
)

// Native is whether the native UI is chosen.
func (u UI) Native() bool { return u.Mode == UINative }

// UIModes is every mode, in the order the setup page offers them.
func UIModes() []string { return []string{UIStandard, UINative} }

type UIWriter struct{ st *Store }

// Mode sets the mode. One the build does not know is stored as standard.
func (w UIWriter) Mode(v string) error {
	if v != UINative {
		v = UIStandard
	}
	return w.st.Update(func(c *Config) { c.UI.Mode = v })
}
