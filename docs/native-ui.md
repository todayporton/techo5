# Native UI (fork)

This fork of TECHO5 adds a native UI to the Echo Show: a dashboard while the device is idle, and its
own listening, thinking and answer screens while a turn runs. It is drawn by echod onto the
framebuffer like every other page. There is no browser, no dashcast and no Home Assistant involved.

The AI backend (STT, the agent, VOICEVOX TTS) lives in a separate repository. This fork contains only
what runs on the device.

## Turning it on and off

- **Setup page:** the *Native UI* section on the Photos tab offers *Standard screens* or *Native UI*.
- **On the screen:** the dashboard's *標準画面* (Standard) button turns the native UI off.
- **Settings:** a swipe down from the top edge always opens the settings sheet, native UI or not.
- **state.json:** `"ui": {"mode": "native"}`. An empty or unknown mode means the standard screens.

If the native UI panics while drawing three times, the process gives up on it. It then draws the
standard screens until the next restart, and logs `nativeui: drawing failed`.

## What it replaces, and what it does not

The native UI takes the place of the clock and the turn screens. These are all still drawn instead of
it, exactly as before:

- calls
- alarms and timers ringing
- the PIN pad
- the setup prompts and Wi-Fi
- the settings sheet
- the camera, alerts and the calendar
- the deck, the weather and the dashboard
- the light before an alarm

The header, the announcement and reminder strips and cards, and the update dialog still go over it.

These things on the standard clock page are not shown while the native UI is on:

- the now-playing page
- the photo slideshow
- the music strip
- the drawer

## Code

| Where | What |
|---|---|
| `echod/internal/feature/nativeui/` | The UI itself: state, drawing, hit testing. It imports nothing of display, screen, touch or voice. |
| `echod/internal/feature/display/native_fork.go` | Adapter: builds the snapshot from the voice state, draws with panic fallback, sets frame pacing, routes touch. |
| `echod/internal/config/ui.go` | The `ui.mode` setting. |
| `echod/internal/feature/setup/ui.go` | The setup page section. |
| `echod/internal/update/releases_cronos.go` | Updates turned off, so an upstream release does not replace this build. |

Every edit to an upstream file is a short hook marked `fork:`. To list them:

```sh
git grep -n "fork:" -- echod ':!*native_fork*' ':!*/nativeui/*' ':!*/ui.go' ':!*/ui_test.go'
```

## Tests

The tests need Linux, because display does not build on macOS. On a Mac, run them in a container:

```sh
docker run --rm -v "$PWD":/src -v techo5-gomod:/go/pkg/mod -w /src/echod golang:1.27 go test ./...
```

Previews are PNGs of every native screen at the Show 5, Show 8, Skylight and Spot sizes. To write
them, set:

- `NATIVE_PREVIEW=<dir>` for the `nativeui` package;
- `SHOW_PREVIEW=<dir>` for display (`native-dashboard*.png`);
- `PREVIEW_FONTS_DIR=<dir>` holding `zz-noto-sans-jp.ttf`, so the Japanese is drawn. This is the font
  `feature/cjkfont` fetches.

`lib/tflite`'s `TestFloatModelsUnchanged` fails on arm64 hosts, as it does without this fork's
changes. Upstream CI runs it on amd64.

## Keeping up with upstream

```sh
git fetch upstream
git merge upstream/main   # on the native branch
```

The fork's own files have names upstream does not use. Conflicts can only come from the `fork:` hooks.
