package nativeui

import (
	"fmt"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/HuskerMinion/techo5/echod/internal/lib/fonts"
)

// The canvases the Show builds draw on (screen geometry): Show 5, Show 8, Skylight, and the Spot's,
// which the native UI is not offered on but must not break on either.
var sizes = []struct {
	name string
	w, h int
}{
	{"show5", 960, 480},
	{"show8", 1280, 800},
	{"skylight", 1920, 1080},
	{"spot", 480, 480},
}

var at = time.Date(2026, 11, 4, 14, 7, 30, 0, time.Local)

// previewFonts puts the fonts in PREVIEW_FONTS_DIR in reach, as display's preview tests do, so the
// Japanese is drawn rather than left out.
func previewFonts(t *testing.T) bool {
	t.Helper()
	dir := os.Getenv("PREVIEW_FONTS_DIR")
	if dir == "" {
		return false
	}
	old := fonts.Dir
	fonts.Dir = dir
	fonts.Rescan()
	t.Cleanup(func() { fonts.Dir = old; fonts.Rescan() })
	return true
}

func save(t *testing.T, name string, img image.Image) {
	t.Helper()
	dir := os.Getenv("NATIVE_PREVIEW")
	if dir == "" {
		return
	}
	f, err := os.Create(filepath.Join(dir, name+".png"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err := png.Encode(f, img); err != nil {
		t.Fatal(err)
	}
}

func TestKinsokuKeepsPunctuationWithItsWord(t *testing.T) {
	got := tokens("今日は晴れ。「タスク」を3件、確認します")
	for _, tok := range got {
		first := []rune(tok)[0]
		if strings.ContainsRune(noStart, first) {
			t.Errorf("piece %q begins with %q", tok, first)
		}
		last := []rune(tok)[len([]rune(tok))-1]
		if strings.ContainsRune(noEnd, last) {
			t.Errorf("piece %q ends with %q", tok, last)
		}
	}
	if strings.Join(got, "") != "今日は晴れ。「タスク」を3件、確認します" {
		t.Errorf("tokens lost text: %q", got)
	}
}

func TestLatinWordsStayWhole(t *testing.T) {
	got := tokens("open the Wi-Fi page")
	want := []string{"open ", "the ", "Wi-Fi ", "page"}
	if fmt.Sprint(got) != fmt.Sprint(want) {
		t.Errorf("tokens = %q, want %q", got, want)
	}
}

func TestWrapFitsTheWidth(t *testing.T) {
	previewFonts(t)
	u := New()
	face := u.face("", false, 30)
	for _, s := range []string{
		"明日の東京の天気は晴れのち曇り、最高気温は18度、最低気温は9度の予報です。傘は必要ありません。",
		"The quick brown fox jumps over the lazy dog, again and again, until the line has to break.",
		"Supercalifragilisticexpialidocious-and-a-word-with-no-room-to-break-anywhere-at-all",
	} {
		lines := wrap(face, s, 400, 0)
		if len(lines) < 2 {
			t.Errorf("%q: %d lines, want it broken", s, len(lines))
		}
		for _, l := range lines {
			if w := textWidth(face, l); w > 400 {
				t.Errorf("line %q is %d wide, over 400", l, w)
			}
		}
	}
	if lines := wrap(face, strings.Repeat("長い文章です。", 40), 400, 3); len(lines) != 3 || !strings.HasSuffix(lines[2], "…") {
		t.Errorf("a cut answer = %q, want 3 lines ending in an ellipsis", lines)
	}
}

// zoneOf is the first zone of the last frame that matches.
func zoneOf(u *UI, match func(zone) bool) (zone, bool) {
	for _, z := range u.zones {
		if match(z) {
			return z, true
		}
	}
	return zone{}, false
}

func center(r image.Rectangle) Input {
	return Input{Kind: Tap, X: (r.Min.X + r.Max.X) / 2, Y: (r.Min.Y + r.Max.Y) / 2}
}

func TestTapOpensACardAndAnotherClosesIt(t *testing.T) {
	u := New()
	img := image.NewRGBA(image.Rect(0, 0, 960, 480))
	idle := Snapshot{Now: at, Phase: PhaseIdle}
	u.Draw(img, idle)
	z, ok := zoneOf(u, func(z zone) bool { return z.card == "tasks" })
	if !ok {
		t.Fatal("no tasks card drawn")
	}
	if got := u.Touch(center(z.rect)); got != Handled {
		t.Fatalf("tap on a card = %v, want Handled", got)
	}
	u.Draw(img, idle)
	if u.detail != "tasks" {
		t.Fatalf("detail = %q, want tasks", u.detail)
	}
	if _, ok := zoneOf(u, func(z zone) bool { return z.leave }); ok {
		t.Error("the Standard button is tappable under the detail sheet")
	}
	if got := u.Touch(Input{Kind: Tap, X: 5, Y: 470}); got != Handled {
		t.Fatalf("tap on the sheet = %v, want Handled", got)
	}
	u.Draw(img, idle)
	if u.detail != "" {
		t.Errorf("detail still open: %q", u.detail)
	}
}

func TestStandardButtonAsksToLeave(t *testing.T) {
	for _, sz := range sizes {
		u := New()
		u.Draw(image.NewRGBA(image.Rect(0, 0, sz.w, sz.h)), Snapshot{Now: at, Phase: PhaseIdle})
		z, ok := zoneOf(u, func(z zone) bool { return z.leave })
		if !ok {
			t.Fatalf("%s: no Standard button", sz.name)
		}
		if got := u.Touch(center(z.rect)); got != Leave {
			t.Errorf("%s: tap on Standard = %v, want Leave", sz.name, got)
		}
	}
}

func TestATapOnNothingIsLeftToDisplay(t *testing.T) {
	u := New()
	img := image.NewRGBA(image.Rect(0, 0, 960, 480))
	u.Draw(img, Snapshot{Now: at, Phase: PhaseIdle})
	// The clock, in the left column, is not a button: display decides (a turn, by default).
	if got := u.Touch(Input{Kind: Tap, X: 200, Y: 240}); got != Ignored {
		t.Errorf("tap on the clock = %v, want Ignored", got)
	}
	if got := u.Touch(Input{Kind: Other, X: 700, Y: 200}); got != Ignored {
		t.Errorf("a swipe = %v, want Ignored", got)
	}
	// A turn's screens take no taps: ending a turn by a tap is display's.
	u.Draw(img, Snapshot{Now: at, Phase: PhaseReplying, Heard: "天気は", Reply: "晴れです"})
	if got := u.Touch(Input{Kind: Tap, X: 700, Y: 200}); got != Ignored {
		t.Errorf("tap during a reply = %v, want Ignored", got)
	}
}

func TestATurnClosesAnOpenDetail(t *testing.T) {
	u := New()
	img := image.NewRGBA(image.Rect(0, 0, 960, 480))
	u.Draw(img, Snapshot{Now: at, Phase: PhaseIdle})
	z, _ := zoneOf(u, func(z zone) bool { return z.card != "" })
	u.Touch(center(z.rect))
	u.Draw(img, Snapshot{Now: at, Phase: PhaseListening})
	u.Draw(img, Snapshot{Now: at, Phase: PhaseIdle})
	if u.detail != "" {
		t.Errorf("the detail outlived the turn: %q", u.detail)
	}
}

func TestContentWithoutTheOpenCardClosesIt(t *testing.T) {
	u := New()
	u.Draw(image.NewRGBA(image.Rect(0, 0, 960, 480)), Snapshot{Now: at, Phase: PhaseIdle})
	z, _ := zoneOf(u, func(z zone) bool { return z.card == "weather" })
	u.Touch(center(z.rect))
	u.SetContent(Content{Status: "接続済み"})
	if u.detail != "" {
		t.Errorf("detail %q kept with no such card", u.detail)
	}
}

func TestPacing(t *testing.T) {
	u := New()
	if got := u.Next(Snapshot{Now: at, Phase: PhaseListening}); got != listenFrame {
		t.Errorf("listening: %v", got)
	}
	if got := u.Next(Snapshot{Now: at, Phase: PhaseReplying}); got != replyFrame {
		t.Errorf("replying: %v", got)
	}
	if got := u.Next(Snapshot{Now: at.Add(250 * time.Millisecond), Phase: PhaseIdle}); got != 750*time.Millisecond {
		t.Errorf("idle: %v, want to the next second", got)
	}
}

// TestEveryScreenDraws draws each screen at each size, with long and empty words, and writes them to
// NATIVE_PREVIEW when it is set.
func TestEveryScreenDraws(t *testing.T) {
	previewFonts(t)
	long := strings.Repeat("明日の東京は晴れのち曇りで、最高気温は18度の予報です。", 12)
	scenes := []struct {
		name string
		s    Snapshot
		open string
		c    *Content
	}{
		{name: "dashboard", s: Snapshot{Phase: PhaseIdle}},
		{name: "dashboard-detail", s: Snapshot{Phase: PhaseIdle}, open: "weather"},
		{name: "dashboard-empty", s: Snapshot{Phase: PhaseIdle}, c: &Content{}},
		{name: "dashboard-nofont", s: Snapshot{Phase: PhaseIdle, NoJapanese: true}},
		{name: "listening", s: Snapshot{Phase: PhaseListening, Levels: []float64{0.2, 0.6, 0.4}}},
		{name: "listening-heard", s: Snapshot{Phase: PhaseListening, Heard: "明日の天気"}},
		{name: "thinking", s: Snapshot{Phase: PhaseThinking, Heard: "明日の東京の天気を教えて"}},
		{name: "replying", s: Snapshot{Phase: PhaseReplying, Heard: "明日の東京の天気を教えて", Reply: "明日の東京は晴れのち曇り、最高気温は18度、最低気温は9度の予報です。", Said: 0.4}},
		{name: "replying-long", s: Snapshot{Phase: PhaseReplying, Heard: long, Reply: long, Said: 0.9}},
		{name: "lingering", s: Snapshot{Phase: PhaseLingering, Heard: "タスクを確認して", Reply: "今日のタスクは3件あります。"}},
		{name: "unknown-phase", s: Snapshot{Phase: "something new"}},
	}
	for _, sz := range sizes {
		for _, sc := range scenes {
			u := New()
			if sc.c != nil {
				u.SetContent(*sc.c)
			}
			img := image.NewRGBA(image.Rect(0, 0, sz.w, sz.h))
			sc.s.Now = at
			if sc.open != "" {
				u.Draw(img, sc.s)
				z, ok := zoneOf(u, func(z zone) bool { return z.card == sc.open })
				if !ok {
					t.Fatalf("%s/%s: card %s not drawn", sz.name, sc.name, sc.open)
				}
				u.Touch(center(z.rect))
			}
			u.Draw(img, sc.s)
			for _, z := range u.zones {
				if !z.rect.In(img.Rect.Inset(-20)) {
					t.Errorf("%s/%s: zone %v is off the canvas", sz.name, sc.name, z.rect)
				}
			}
			save(t, sc.name+"-"+sz.name, img)
		}
	}
}

// Cards keep their words readable in a theme whose rule color is as light as its text.
func TestCardsReadInALightRuleTheme(t *testing.T) {
	th := Theme{Ground: color.RGBA{0, 0, 0, 255}, Accent: color.RGBA{255, 160, 0, 255}, Text: color.RGBA{255, 204, 153, 255},
		Dim: color.RGBA{204, 153, 204, 255}, Rule: color.RGBA{153, 153, 255, 255}}
	u := New()
	img := image.NewRGBA(image.Rect(0, 0, 960, 480))
	u.Draw(img, Snapshot{Now: at, Phase: PhaseIdle, Theme: th})
	z, _ := zoneOf(u, func(z zone) bool { return z.card != "" })
	c := img.RGBAAt(z.rect.Min.X+30, z.rect.Max.Y-6)
	if c == th.Rule {
		t.Errorf("a card is filled with the rule color %v", c)
	}
	if lum := int(c.R) + int(c.G) + int(c.B); lum > 3*80 {
		t.Errorf("a card on a black ground is as light as %v", c)
	}
}
