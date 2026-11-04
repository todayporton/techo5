//go:build !dot && !spot && !panel

package all

// fork: this fork's Show build follows no release channel (update/releases_cronos.go is empty, as on a
// panel), so the channel's select is not put up. Added here rather than in device_cronos_test.go so
// upstream's list merges untouched.
func init() { notOnThisDevice = append(notOnThisDevice, "update_channel") }
