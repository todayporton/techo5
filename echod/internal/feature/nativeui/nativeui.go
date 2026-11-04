// Package nativeui is the native UI fork's screen (docs/native-ui.md): a dashboard while the device is
// idle and its own listening, thinking and answer screens while a turn runs, drawn by the daemon like
// every other page.
//
// It knows nothing of the framebuffer, the touchscreen, the voice or Home Assistant. feature/display
// hands it a Snapshot and a canvas each frame and a finger's position when one lands, and does what
// the Result asks; everything above it — calls, rings, the settings sheet, the update dialog — stays
// display's and is drawn over it or instead of it. That keeps this package testable on any machine
// and out of the way of the upstream display code it sits in.
//
// The phase it is given is the device's own (voice.State, with display's lingering): nothing that
// arrives from a backend later is allowed to say a turn is in a phase it is not.
package nativeui

import (
	"image"
	"image/color"
	"sync"
	"time"
)

// Phases, as display names them. Lingering is an idle device still showing the turn's words.
const (
	PhaseIdle      = "idle"
	PhaseListening = "listening"
	PhaseThinking  = "thinking"
	PhaseReplying  = "replying"
	PhaseLingering = "lingering"
)

// Theme is the screen's colors in force: display's theme, so the native UI follows the one chosen.
type Theme struct {
	Ground, Accent, Text, Dim, Rule color.RGBA
}

// DefaultTheme is display's own default (walnut, amber, cream), for tests and a theme not handed in.
var DefaultTheme = Theme{
	Ground: color.RGBA{0x1c, 0x15, 0x11, 0xff},
	Accent: color.RGBA{0xe9, 0xa2, 0x3b, 0xff},
	Text:   color.RGBA{0xe8, 0xdc, 0xc8, 0xff},
	Dim:    color.RGBA{0x8a, 0x7d, 0x6c, 0xff},
	Rule:   color.RGBA{0x3a, 0x2e, 0x26, 0xff},
}

// Snapshot is one frame's worth of what the device is doing.
type Snapshot struct {
	Now   time.Time
	Phase string
	Heard string
	Reply string
	// Said is how much of the reply the voice has said, 0 to 1 (voice.Spoken).
	Said float64
	// Levels is the microphone's bands while listening and the speaker's while replying, 0 to 1 each;
	// nil when not known.
	Levels []float64
	// Font is the screen font (lib/fonts value); empty is Go.
	Font  string
	Theme Theme
	// NoJapanese is set when no font in reach draws Japanese (feature/cjkfont not fetched yet): the
	// screen says so in words it can draw instead of drawing boxes.
	NoJapanese bool
	// Since is when the voice entered the phase it is in (or went idle).
	Since time.Time
	// Ended is how the last turn ended; zero before the first.
	Ended Ending
}

// Ending is how a turn ended (feature/activity's Outcome, in this package's own words).
type Ending struct {
	How      string // EndAnswered, EndNoSpeech, EndCanceled, EndFailed
	Reason   string // a canceled turn's: stopped, interrupted, cut_off, answered_elsewhere
	Failure  string // a failed turn's code: stt-failed, intent-failed, tts-failed, ...
	TimedOut bool
	At       time.Time
}

// How a turn ends, as feature/activity names it.
const (
	EndAnswered = "answered"
	EndNoSpeech = "no_speech"
	EndCanceled = "canceled"
	EndFailed   = "error"
)

// Card is one of the dashboard's tiles: a title, a value read at a glance, and the detail a tap
// opens. ID is stable, for actions on it later.
type Card struct {
	ID     string
	Title  string
	Value  string
	Detail string
}

// Content is what the dashboard shows besides the clock. Until a backend sends it, it is DemoContent.
type Content struct {
	Cards []Card
	// Status is the one line about the assistant itself: connected, or what is wrong.
	Status string
	// Offline marks Status as a problem, drawn in the accent.
	Offline bool
}

// DemoContent is the placeholder the dashboard shows until there is real content: plainly made up.
func DemoContent() Content {
	return Content{
		Cards: []Card{
			{ID: "weather", Title: "天気", Value: "晴れ 18°", Detail: "サンプル表示です。天気の取得は今後のフェーズで接続します。"},
			{ID: "events", Title: "次の予定", Value: "14:00 打合せ", Detail: "サンプル表示です。予定の取得は今後のフェーズで接続します。"},
			{ID: "tasks", Title: "タスク", Value: "3件", Detail: "サンプル表示です。タスクの取得は今後のフェーズで接続します。"},
		},
		Status:  "AIサーバー未接続",
		Offline: true,
	}
}

// InputKind is the kind of a finger on the screen, as far as the native UI tells them apart.
type InputKind int

const (
	Tap InputKind = iota
	Other
)

// Input is a finger: its kind and where, in canvas pixels.
type Input struct {
	Kind InputKind
	X, Y int
}

// Result is what a finger asks of display.
type Result int

const (
	// Ignored leaves the finger to display, as if the native UI were not there.
	Ignored Result = iota
	// Handled is taken: display only draws again.
	Handled
	// Leave asks for the standard screens back: display turns the native UI off.
	Leave
)

// zone is a tappable place drawn in the last frame.
type zone struct {
	rect    image.Rectangle
	card    string // the card's ID, or empty
	leave   bool   // the Standard screen button
	closes  bool   // the detail sheet: a tap anywhere puts it away
	dismiss bool   // a lingering answer: a tap puts it away, back to the dashboard
}

// UI is the native UI's state between frames: what is open, what was drawn where, and the faces made.
// Draw runs on display's frame loop and Touch on the touchscreen's goroutine, so all of it is under mu.
type UI struct {
	mu sync.Mutex
	// lingering is the reply of the answer the last frame showed lingering, what a tap dismisses.
	lingering string
	// shown is the screen the last frame drew (Shown).
	shown   string
	content Content
	detail  string // the card whose detail is open, or empty
	// dismissed is the answer a tap put away while it lingered (its reply), so the dashboard comes back
	// at once rather than when display stops calling it lingering.
	dismissed string
	zones     []zone
	faces     faceCache
	// result is what the turn looked up (result.go), until a new turn or a tap puts it away.
	result *Lookup
}

// New is a native UI showing DemoContent.
func New() *UI { return &UI{content: DemoContent()} }

// SetContent replaces what the dashboard shows. A detail open on a card that is gone is closed.
func (u *UI) SetContent(c Content) {
	u.mu.Lock()
	defer u.mu.Unlock()
	u.content = c
	if u.detail != "" && u.card(u.detail) == nil {
		u.detail = ""
	}
}

// SetStatus replaces the line about the assistant, leaving the cards as they are.
func (u *UI) SetStatus(text string, offline bool) {
	u.mu.Lock()
	u.content.Status, u.content.Offline = text, offline
	u.mu.Unlock()
}

// card is the card with id in the content in hand, or nil. Called under mu.
func (u *UI) card(id string) *Card {
	for i := range u.content.Cards {
		if u.content.Cards[i].ID == id {
			return &u.content.Cards[i]
		}
	}
	return nil
}

// Touch is a finger on the native UI, against what the last frame drew.
func (u *UI) Touch(in Input) Result {
	if in.Kind != Tap {
		return Ignored
	}
	p := image.Pt(in.X, in.Y)
	u.mu.Lock()
	defer u.mu.Unlock()
	for _, z := range u.zones {
		if !p.In(z.rect) {
			continue
		}
		switch {
		case z.closes:
			u.detail = ""
			return Handled
		case z.dismiss:
			u.dismissed = u.lingering
			u.result = nil
			return Handled
		case z.leave:
			return Leave
		case z.card != "":
			u.detail = z.card
			return Handled
		}
	}
	return Ignored
}

// Screens, as Shown names them.
const (
	ScreenDashboard = "dashboard"
	ScreenDetail    = "detail"
	ScreenNotice    = "notice"
	ScreenListening = "listening"
	ScreenThinking  = "thinking"
	ScreenAnswer    = "answer"
	ScreenResult    = "result"
)

// Shown is the screen the last frame drew.
func (u *UI) Shown() string {
	u.mu.Lock()
	defer u.mu.Unlock()
	return u.shown
}

// Reset closes whatever is open, for a screen coming back to the native UI from another page.
func (u *UI) Reset() {
	u.mu.Lock()
	u.detail = ""
	u.mu.Unlock()
}

// Frame pacing: a turn's screens move, the dashboard changes once a second at most.
const (
	listenFrame = 100 * time.Millisecond
	replyFrame  = 150 * time.Millisecond
)

// Next is how long until the frame after one drawn for s is due.
func (u *UI) Next(s Snapshot) time.Duration {
	switch s.Phase {
	case PhaseListening, PhaseThinking:
		return listenFrame
	case PhaseReplying:
		return replyFrame
	}
	u.mu.Lock()
	held := u.resultHeld(s.Now)
	var until time.Time
	if held {
		until = u.result.At.Add(resultHold)
	}
	u.mu.Unlock()
	if held && s.Phase == PhaseIdle {
		return max(min(until.Sub(s.Now), time.Second), 20*time.Millisecond)
	}
	if until, ok := noticeUntil(s); ok {
		// The notice goes when it is due, not at the next whole second after.
		return max(min(until.Sub(s.Now), time.Second), 20*time.Millisecond)
	}
	// On the next whole second, so the clock turns over when the second does.
	return s.Now.Truncate(time.Second).Add(time.Second).Sub(s.Now)
}
