//go:build !dot && !spot

package display

// fork: the native UI's place in the screen (native_fork.go).

import (
	"image"
	"path/filepath"
	"testing"
	"time"

	"github.com/HuskerMinion/techo5/echod/internal/config"
	"github.com/HuskerMinion/techo5/echod/internal/feature/nativeui"
	"github.com/HuskerMinion/techo5/echod/internal/feature/phone"
	"github.com/HuskerMinion/techo5/echod/internal/feature/voice"
	"github.com/HuskerMinion/techo5/echod/internal/hardware/touch"
)

var nativeAt = time.Date(2026, 11, 4, 14, 7, 0, 0, time.Local)

// useNative starts a test on a fresh config with the native UI chosen (or not) and nothing given up.
func useNative(t *testing.T, on bool) {
	t.Helper()
	config.Use(filepath.Join(t.TempDir(), "state.json"))
	mode := config.UIStandard
	if on {
		mode = config.UINative
	}
	if err := config.Set().UI().Mode(mode); err != nil {
		t.Fatal(err)
	}
	nativeFails.Store(0)
	nativeShown.Store(false)
	native.Reset()
	t.Cleanup(func() { nativeFails.Store(0); nativeShown.Store(false) })
}

// nativeScene is a scene as frame builds one, with the native UI's part added.
func nativeTestScene(view voice.State, at time.Time, edit func(*scene)) scene {
	s := scene{now: nativeAt, phase: view.Phase, heard: view.Heard, reply: view.Reply}
	if edit != nil {
		edit(&s)
	}
	(&Display{}).nativeScene(&s, view, at, nativeAt)
	return s
}

func TestStandardModeLeavesTheSceneAlone(t *testing.T) {
	useNative(t, false)
	s := nativeTestScene(voice.State{Phase: "idle"}, nativeAt, nil)
	if s.native != nil {
		t.Fatal("the native UI is in the scene with the standard screens chosen")
	}
}

func TestNativeModeDrawsTheNativeUI(t *testing.T) {
	previewFont(t) // PREVIEW_FONTS_DIR's fonts, the Japanese one among them, for the previews
	useNative(t, true)
	for _, sz := range []image.Point{{showWide, showHigh}, {show8Wide, show8High}, {skylightWide, skylightHigh}} {
		img := image.NewRGBA(image.Rect(0, 0, sz.X, sz.Y))
		s := nativeTestScene(voice.State{Phase: "idle"}, nativeAt, nil)
		newRenderer(img).draw(s)
		if s.native == nil || !s.native.drawn {
			t.Fatalf("%v: the native UI was not drawn", sz)
		}
		if next, ok := (&Display{}).nativeFrame(s); !ok || next <= 0 || next > time.Second {
			t.Errorf("%v: next frame %v, %v", sz, next, ok)
		}
		if !nativeShown.Load() {
			t.Errorf("%v: not noted as shown", sz)
		}
		writePreviewIfAsked(t, "native-dashboard", sz, img)
	}
}

// writePreviewIfAsked writes the frame to SHOW_PREVIEW with the others, when it is set.
func writePreviewIfAsked(t *testing.T, name string, sz image.Point, img *image.RGBA) {
	t.Helper()
	switch sz.X {
	case show8Wide:
		name += "-show8"
	case skylightWide:
		name += "-skylight"
	}
	writePreview(t, img, name+".png")
}

func TestLingeringIsTheVoicesIdleWithWordsLeft(t *testing.T) {
	useNative(t, true)
	view := voice.State{Phase: "idle", Heard: "天気は", Reply: "晴れです"}
	if s := nativeTestScene(view, nativeAt.Add(-time.Second), nil); s.native.snap.Phase != nativeui.PhaseLingering {
		t.Errorf("a second after: %q, want lingering", s.native.snap.Phase)
	}
	if s := nativeTestScene(view, nativeAt.Add(-time.Hour), nil); s.native.snap.Phase != nativeui.PhaseIdle {
		t.Errorf("an hour after: %q, want idle", s.native.snap.Phase)
	}
}

func TestTheCueStylesStepAside(t *testing.T) {
	useNative(t, true)
	s := nativeTestScene(voice.State{Phase: "listening"}, nativeAt, func(s *scene) { s.cue = "listening" })
	if s.cue != "" {
		t.Errorf("cue %q left over the native UI", s.cue)
	}
	if rainbowHere(s) {
		t.Error("Rainbow would draw over the native UI")
	}
}

// Every page render.draw puts before the clock is still drawn instead of the native UI.
func TestPagesAboveTheClockStayAbove(t *testing.T) {
	useNative(t, true)
	above := map[string]func(*scene){
		"a call":             func(s *scene) { s.call = phone.State{Phase: phone.Ringing} },
		"the settings sheet": func(s *scene) { s.showSheet = true },
		"the weather":        func(s *scene) { s.showWeather = true },
		"the dashboard":      func(s *scene) { s.showDash = true },
		"the Wi-Fi page":     func(s *scene) { s.showWifi = true },
		"the light at dawn":  func(s *scene) { s.sunrise = 0.5 },
	}
	for name, edit := range above {
		s := nativeTestScene(voice.State{Phase: "idle"}, nativeAt, edit)
		newRenderer(image.NewRGBA(image.Rect(0, 0, showWide, showHigh))).draw(s)
		if s.native != nil && s.native.drawn {
			t.Errorf("the native UI was drawn over %s", name)
		}
		if _, ok := (&Display{}).nativeFrame(s); ok {
			t.Errorf("the native UI paced the frame of %s", name)
		}
	}
}

func TestAPanickingNativeUIFallsBackAndIsGivenUp(t *testing.T) {
	useNative(t, true)
	old := nativeDraw
	nativeDraw = func(*image.RGBA, nativeui.Snapshot) { panic("broken page") }
	t.Cleanup(func() { nativeDraw = old })
	for i := range nativeMaxFails {
		s := nativeTestScene(voice.State{Phase: "idle"}, nativeAt, nil)
		if s.native == nil {
			t.Fatalf("given up after %d failures, want %d", i, nativeMaxFails)
		}
		newRenderer(image.NewRGBA(image.Rect(0, 0, showWide, showHigh))).draw(s) // the standard clock instead
		if s.native.drawn {
			t.Fatal("a frame that panicked counted as drawn")
		}
	}
	if s := nativeTestScene(voice.State{Phase: "idle"}, nativeAt, nil); s.native != nil {
		t.Error("still tried after giving up")
	}
}

func TestFingersOnTheNativeUI(t *testing.T) {
	useNative(t, true)
	img := image.NewRGBA(image.Rect(0, 0, showWide, showHigh))
	d := &Display{r: newRenderer(img)}
	s := nativeTestScene(voice.State{Phase: "idle"}, nativeAt, nil)
	d.r.draw(s)
	d.nativeFrame(s)

	// What the clock does with a finger is the clock's: the settings sheet from the top edge, the
	// volume, the deck and the dashboard from their edges, and a tap on nothing.
	for _, g := range []touch.Gesture{
		{Kind: touch.SwipeDown, X: 480, Y: 10},
		{Kind: touch.SwipeDown, X: 480, Y: 300},
		{Kind: touch.SwipeUp, X: 480, Y: 470},
		{Kind: touch.SwipeRight, X: 5, Y: 240},
		{Kind: touch.Hold, X: 200, Y: 240},
		{Kind: touch.Tap, X: 200, Y: 240},
	} {
		if d.nativeGesture(g) {
			t.Errorf("%v was taken from the clock's handling", g)
		}
	}
	// Sideways swipes change no clock style nobody can see, and open no drawer only the clock draws.
	for _, g := range []touch.Gesture{{Kind: touch.SwipeLeft, X: 950, Y: 240}, {Kind: touch.SwipeLeft, X: 480, Y: 240}, {Kind: touch.SwipeRight, X: 480, Y: 240}} {
		if !d.nativeGesture(g) {
			t.Errorf("%v reached the clock", g)
		}
	}
	// The Standard button turns the native UI off.
	if !d.nativeGesture(touch.Gesture{Kind: touch.Tap, X: showWide - 80, Y: showHigh - 28}) {
		t.Fatal("the Standard button did not take its tap")
	}
	if config.Get().UI.Native() {
		t.Error("still native after the Standard button")
	}
	// A frame that did not draw it leaves fingers alone.
	nativeShown.Store(false)
	if d.nativeGesture(touch.Gesture{Kind: touch.SwipeLeft, X: 480, Y: 240}) {
		t.Error("a finger went to a native UI not on the screen")
	}
}
