//go:build !dot && !spot && !panel

package update

// releases is where the Echo Show 5's releases are published.
//
// fork: empty, as on a panel (releases_panel.go). This fork's builds are not published upstream, and
// an upstream release offered as "newer" would replace the native UI. Point it at the fork's own
// releases, signed with its own key (trust.go), to turn updates back on.
const releases = ""
