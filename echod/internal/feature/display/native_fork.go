//go:build !dot && !spot

package display

// fork: the native UI's place in the screen (feature/nativeui, docs/native-ui.md). Upstream files call
// in here from four lines marked "fork:" — the scene (display.go frame), the drawing (render.go draw),
// the pace and the touch (display.go) — and Rainbow steps aside for it (render_rainbow.go).
//
// The native UI takes the place of the clock and the turn screens only. Everything render.draw puts
// before them (a call, a ring, the PIN pad, the settings sheet, the camera, the calendar, the deck,
// the weather, the dashboard) is still drawn instead of it, and the header, the strips, the cards and
// the update dialog still go over it. A swipe down from the top edge is still the settings sheet.

import (
	"image"
	"image/draw"
	"log/slog"
	"runtime/debug"
	"sync/atomic"
	"time"

	"github.com/HuskerMinion/techo5/echod/internal/config"
	"github.com/HuskerMinion/techo5/echod/internal/feature/activity"
	"github.com/HuskerMinion/techo5/echod/internal/feature/nativeui"
	"github.com/HuskerMinion/techo5/echod/internal/feature/setup"
	"github.com/HuskerMinion/techo5/echod/internal/feature/voice"
	"github.com/HuskerMinion/techo5/echod/internal/hardware/touch"
	"github.com/HuskerMinion/techo5/echod/internal/lib/fonts"
	"github.com/HuskerMinion/techo5/echod/internal/lib/spectrum"
)

func init() {
	setup.OfferNativeUI() // the setup page offers the choice on a build that draws it
	activity.Ended.Listen(func(o activity.Outcome) {
		e := nativeui.Ending{How: string(o.How), Reason: string(o.Reason), Failure: o.Failure, TimedOut: o.TimedOut, At: o.At}
		lastEnding.Store(&e)
		if d := nativeDisplay.Load(); d != nil {
			d.wake() // the notice goes up as the turn ends, not at the next second
		}
	})
}

// nativeDisplay is the display drawing the native UI, once it has: what a turn's end wakes. Not Get,
// which would build a display in a process (a test) that has none.
var nativeDisplay atomic.Pointer[Display]

// lastEnding is how the last turn ended, from feature/activity: the dashboard's notice and status.
var lastEnding atomic.Pointer[nativeui.Ending]

// native is the native UI's state, kept between frames; nativeDraw draws it, a test's to replace.
var (
	native     = nativeui.New()
	nativeDraw = native.Draw
)

// nativeMaxFails is how many of the native UI's frames may panic before this process stops trying it
// and draws the standard screens until it is restarted: a broken page is never a screen nobody can use.
const nativeMaxFails = 3

var nativeFails atomic.Int32

// nativeShown is whether the last frame drew the native UI, read by the touchscreen's goroutine so a
// finger goes to what is on the screen.
var nativeShown atomic.Bool

// nativeBands is how many of the spectrum's bands the native UI is handed.
const nativeBands = 8

// nativeView is the scene's part for the native UI: what it is handed, and whether the frame drew it.
type nativeView struct {
	snap  nativeui.Snapshot
	drawn bool
}

// nativeWanted is whether the native UI is chosen and has not been given up on.
func nativeWanted() bool {
	return config.Get().UI.Native() && nativeFails.Load() < nativeMaxFails
}

// japanese is whether a font in reach draws Japanese, worked out again only when the fonts change.
var japanese struct {
	gen  atomic.Uint64
	has  atomic.Bool
	seen atomic.Bool
}

func japaneseDrawn() bool {
	g := fonts.Generation()
	if !japanese.seen.Load() || japanese.gen.Load() != g {
		japanese.has.Store(fonts.OwnDraws('あ', ""))
		japanese.gen.Store(g)
		japanese.seen.Store(true)
	}
	return japanese.has.Load()
}

// nativeScene gives the frame's scene the native UI's part when it is to be drawn. It is worked out
// from the voice's own state rather than s.phase, which the cue styles and screen commands rewrite.
func (d *Display) nativeScene(s *scene, view voice.State, at, now time.Time) {
	nativeDisplay.Store(d)
	if !nativeWanted() || s.sunrise > 0 {
		// The light before an alarm keeps the whole screen: the panel is the lamp in the room.
		return
	}
	phase := view.Phase
	if phase == nativeui.PhaseIdle && (view.Heard != "" || view.Reply != "") && now.Sub(at) < linger() {
		phase = nativeui.PhaseLingering
	}
	var ended nativeui.Ending
	if e := lastEnding.Load(); e != nil {
		ended = *e
	}
	// This fork answers without Home Assistant: a voice set up is the direct pipeline (brain.mode direct).
	native.SetStatus(nativeui.Status(config.Get().Brain.Direct(), ended))
	snap := nativeui.Snapshot{
		Now: now, Phase: phase, Heard: view.Heard, Reply: view.Reply, Since: at, Ended: ended,
		Said:       voice.Spoken(view.Reply, now),
		Font:       config.Get().Screen.Font,
		Theme:      nativeui.Theme{Ground: walnut, Accent: amber, Text: cream, Dim: dim, Rule: ember},
		NoJapanese: !japaneseDrawn(),
	}
	switch phase {
	case nativeui.PhaseListening:
		snap.Levels, _ = spectrum.Mic.Bands(nativeBands, now)
	case nativeui.PhaseReplying:
		snap.Levels, _ = spectrum.Speaker.Bands(nativeBands, now)
	}
	s.cue = "" // the native UI draws the turn itself: no cue light or captions over it
	s.native = &nativeView{snap: snap}
}

// nativePage draws the native UI over the canvas. A panic is caught, counted and logged, and the
// canvas cleared, so the frame goes on to draw the standard screens instead.
func (r *renderer) nativePage(s scene) (ok bool) {
	defer func() {
		if p := recover(); p != nil {
			n := nativeFails.Add(1)
			slog.Error("nativeui: drawing failed, the standard screens are drawn instead",
				"panic", p, "failures", n, "giving up", n >= nativeMaxFails, "stack", string(debug.Stack()))
			draw.Draw(r.dst, r.dst.Rect, image.NewUniform(walnut), image.Point{}, draw.Src)
			ok = false
		}
	}()
	nativeDraw(r.dst, s.native.snap)
	s.native.drawn = true
	return true
}

// nativeFrame notes whether the frame just presented was the native UI's and, when it was, how long
// until the next one is due.
func (d *Display) nativeFrame(s scene) (time.Duration, bool) {
	drawn := s.native != nil && s.native.drawn
	nativeShown.Store(drawn)
	if !drawn {
		return 0, false
	}
	switch {
	case s.updateActive:
		return 250 * time.Millisecond, true // the update dialog's progress, over it
	case s.showVolume:
		return activeFrame, true // the level following a finger
	}
	return max(native.Next(s.native.snap), 20*time.Millisecond), true
}

// nativeGesture is a finger on the native UI, asked once every page above it has passed on it. It
// takes the taps that land on its own buttons and the sideways swipes, and leaves the rest to the
// clock's handling: a swipe down from the top edge (the settings sheet), up from the bottom (the deck),
// right from the left edge (the dashboard), up or down elsewhere (the volume), and a tap on nothing.
func (d *Display) nativeGesture(g touch.Gesture) bool {
	if !nativeShown.Load() {
		return false
	}
	switch g.Kind {
	case touch.Tap:
		switch native.Touch(nativeui.Input{Kind: nativeui.Tap, X: g.X, Y: g.Y}) {
		case nativeui.Handled:
			d.wake()
			return true
		case nativeui.Leave:
			if err := config.Set().UI().Mode(config.UIStandard); err != nil {
				slog.Warn("nativeui: saving the standard screens failed", "err", err)
			}
			native.Reset()
			slog.Info("nativeui: standard screens chosen on the screen")
			d.wake()
			return true
		}
		return false
	case touch.SwipeRight:
		// From the left edge it is the dashboard, which is drawn over the native UI.
		if d.r != nil && g.X < d.r.drawerEdge() {
			return false
		}
		return true // not a clock style: none is drawn
	case touch.SwipeLeft:
		return true // not a clock style, and not the drawer, which only the clock draws
	}
	return false
}
