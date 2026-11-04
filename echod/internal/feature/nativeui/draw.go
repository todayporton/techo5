package nativeui

import (
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"math"
	"time"

	"golang.org/x/image/font"
)

// The layout is drawn for a 960×480 canvas (the Echo Show 5's) and scaled: px is a length in that
// layout, frame.s(px) the same length on the canvas in hand.
// maxCards is how many cards the dashboard's column has room for; any after are left out.
const maxCards = 4

const (
	layoutW = 960
	layoutH = 480
	// headerRoom is the top band display's header is drawn over (render.go headerH, with air).
	headerRoom = 44
)

// frame is one Draw: the canvas, its scale, the snapshot and the theme, and the zones being laid.
type frame struct {
	u     *UI
	dst   *image.RGBA
	w, h  int
	scale float64
	s     Snapshot
	t     Theme
	zones []zone
}

func (f *frame) px(n int) int { return int(math.Round(float64(n) * f.scale)) }

// text is the screen font at n layout pixels.
func (f *frame) text(bold bool, n int) font.Face { return f.u.face(f.s.Font, bold, f.px(n)) }

// Draw draws the native UI for s over the whole of dst and keeps where its taps land.
func (u *UI) Draw(dst *image.RGBA, s Snapshot) {
	u.mu.Lock()
	defer u.mu.Unlock()
	if s.Theme == (Theme{}) {
		s.Theme = DefaultTheme
	}
	w, h := dst.Rect.Dx(), dst.Rect.Dy()
	f := &frame{u: u, dst: dst, w: w, h: h, s: s, t: s.Theme,
		scale: math.Min(float64(w)/layoutW, float64(h)/layoutH)}
	fill(dst, dst.Rect, f.t.Ground)
	switch s.Phase {
	case PhaseListening:
		u.detail = ""
		if u.result != nil && (s.Since.IsZero() || u.result.At.Before(s.Since)) {
			u.result = nil // the last turn's: this one has looked nothing up yet
		}
		u.shown = ScreenListening
		f.listening()
	case PhaseThinking:
		u.detail = ""
		u.shown = ScreenThinking
		f.thinking()
	case PhaseReplying, PhaseLingering:
		u.detail = ""
		if s.Phase == PhaseLingering && s.Reply != "" && s.Reply == u.dismissed {
			u.shown = ScreenDashboard
			f.dashboard() // put away by a tap: the words linger no more
			break
		}
		u.dismissed = ""
		u.shown = ScreenAnswer
		f.answer()
	default:
		if u.resultHeld(s.Now) {
			u.detail = ""
			u.shown = ScreenResult
			f.resultPage()
			break
		}
		u.result = nil
		u.shown = ScreenDashboard
		f.dashboard()
		if c := u.card(u.detail); c != nil {
			u.shown = ScreenDetail
			f.detailSheet(*c)
		} else {
			u.detail = ""
		}
		if f.notice() {
			u.shown = ScreenNotice
		}
	}
	u.lingering = ""
	if s.Phase == PhaseLingering {
		u.lingering = s.Reply
	}
	u.zones = f.zones
}

// dashboard is the idle page: the time and date on the left, the cards on the right, the assistant's
// status and the way back to the standard screens along the foot.
func (f *frame) dashboard() {
	c := f.u.content
	margin := f.px(40)
	top := f.px(headerRoom)
	footH := f.px(56)
	footTop := f.h - footH
	split := f.w * 56 / 100

	// The clock, centered in the left column's height.
	timeFace := f.text(true, 150)
	dateFace := f.text(false, 34)
	clock := f.s.Now.Format("15:04")
	block := ascent(timeFace) + f.px(20) + lineHeight(dateFace)
	y := top + max(0, (footTop-top-block)/2) + ascent(timeFace)
	drawText(f.dst, timeFace, f.t.Text, margin, y, clock)
	date := japaneseDate(f.s.Now)
	if f.s.NoJapanese {
		date = f.s.Now.Format("Mon Jan 2")
	}
	drawText(f.dst, dateFace, f.t.Dim, margin+f.px(6), y+f.px(20)+ascent(dateFace), date)

	// The cards, stacked in the right column.
	gap := f.px(14)
	colX, colW := split, f.w-split-margin
	n := min(len(c.Cards), maxCards)
	if n > 0 {
		cardH := (footTop - top - f.px(8) - gap*(n-1)) / n
		titleFace, valueFace := f.text(false, 22), f.text(true, 34)
		for i, card := range c.Cards[:n] {
			r := image.Rect(colX, top+f.px(4)+i*(cardH+gap), colX+colW, top+f.px(4)+i*(cardH+gap)+cardH)
			fillRound(f.dst, r, f.px(14), f.t.Rule)
			pad := f.px(18)
			title, value := f.words(card.Title, card.ID), f.words(card.Value)
			ty := r.Min.Y + pad + ascent(titleFace)
			drawText(f.dst, titleFace, f.t.Dim, r.Min.X+pad, ty, clip(titleFace, title, r.Dx()-2*pad))
			vy := min(r.Max.Y-pad, ty+f.px(10)+ascent(valueFace))
			drawText(f.dst, valueFace, f.t.Text, r.Min.X+pad, vy, clip(valueFace, value, r.Dx()-2*pad))
			f.zones = append(f.zones, zone{rect: r, card: card.ID})
		}
	}

	// The foot: the status on the left, the button home on the right.
	footFace := f.text(false, 24)
	base := footTop + (footH+ascent(footFace))/2 - f.px(4)
	dot := f.t.Dim
	if c.Offline {
		dot = f.t.Accent
	}
	fillCircle(f.dst, image.Pt(margin+f.px(8), base-ascent(footFace)/2+f.px(2)), f.px(7), dot)
	drawText(f.dst, footFace, f.t.Dim, margin+f.px(26), base, clip(footFace, f.words(c.Status, noJapaneseNote), split-margin*2))

	label := f.words("標準画面")
	if f.s.NoJapanese {
		label = "Standard"
	}
	bw := textWidth(footFace, label) + f.px(40)
	btn := image.Rect(f.w-margin-bw, footTop+f.px(6), f.w-margin, f.h-f.px(10))
	strokeRound(f.dst, btn, f.px(12), max(1, f.px(2)), f.t.Dim)
	drawText(f.dst, footFace, f.t.Text, btn.Min.X+f.px(20), base, label)
	f.zones = append(f.zones, zone{rect: btn.Inset(-f.px(8)), leave: true})
}

// detailSheet is a card's detail over the dashboard; a tap anywhere puts it away.
func (f *frame) detailSheet(c Card) {
	shade(f.dst, f.dst.Rect, f.t.Ground, 200)
	r := image.Rect(f.w*12/100, f.px(headerRoom)+f.px(10), f.w*88/100, f.h-f.px(30))
	fillRound(f.dst, r, f.px(18), f.t.Rule)
	pad := f.px(32)
	titleFace, bodyFace, hintFace := f.text(true, 40), f.text(false, 30), f.text(false, 22)
	y := r.Min.Y + pad + ascent(titleFace)
	drawText(f.dst, titleFace, f.t.Accent, r.Min.X+pad, y, clip(titleFace, f.words(c.Title), r.Dx()-2*pad))
	y += f.px(16)
	drawText(f.dst, bodyFace, f.t.Text, r.Min.X+pad, y+ascent(bodyFace), clip(bodyFace, f.words(c.Value), r.Dx()-2*pad))
	y += lineHeight(bodyFace) + f.px(12)
	hintH := lineHeight(hintFace) + pad
	lines := wrap(bodyFace, f.words(c.Detail), r.Dx()-2*pad, max(1, (r.Max.Y-hintH-y)/max(1, lineHeight(bodyFace))))
	for _, l := range lines {
		y += lineHeight(bodyFace)
		drawText(f.dst, bodyFace, f.t.Dim, r.Min.X+pad, y, l)
	}
	drawText(f.dst, hintFace, f.t.Dim, r.Min.X+pad, r.Max.Y-pad+f.px(8), f.words("タップで閉じる"))
	// The sheet covers the page: what is under it is not there to tap.
	f.zones = []zone{{rect: f.dst.Rect, closes: true}}
}

// listening is the screen from the wake word until the device stops listening: a disc that breathes,
// ringed by the microphone's bands when they are known.
func (f *frame) listening() {
	cx, cy := f.w/2, f.px(headerRoom)+(f.h-f.px(headerRoom))*42/100
	breath := 0.5 + 0.5*math.Sin(float64(f.s.Now.UnixMilli()%1200)/1200*2*math.Pi)
	level := mean(f.s.Levels)
	r := f.px(58) + int(float64(f.px(8))*breath) + int(float64(f.px(22))*level)
	fillCircle(f.dst, image.Pt(cx, cy), r+f.px(16), blend(f.t.Ground, f.t.Accent, 0.18))
	f.bandRing(image.Pt(cx, cy), r+f.px(24))
	fillCircle(f.dst, image.Pt(cx, cy), r, f.t.Accent)
	f.centered(f.text(true, 44), f.t.Text, cy+f.px(150), f.words("聞いています", "Listening"))
	if f.s.Heard != "" {
		f.centered(f.text(false, 26), f.t.Dim, cy+f.px(192), f.words(f.s.Heard))
	}
}

// bandRing is the bands as dots around a circle of radius r, each pushed out by its level: mirrored
// left and right so the low bands sit at the top.
func (f *frame) bandRing(c image.Point, r int) {
	n := len(f.s.Levels)
	if n == 0 {
		return
	}
	const spokes = 32
	for i := range spokes {
		half := i
		if half >= spokes/2 {
			half = spokes - 1 - i
		}
		v := math.Max(0, math.Min(1, f.s.Levels[half*n/(spokes/2)]))
		a := 2*math.Pi*float64(i)/spokes - math.Pi/2
		d := float64(r) + v*float64(f.px(26))
		p := image.Pt(c.X+int(d*math.Cos(a)), c.Y+int(d*math.Sin(a)))
		fillCircle(f.dst, p, max(1, f.px(3)+int(v*float64(f.px(3)))), blend(f.t.Ground, f.t.Accent, 0.45+0.55*v))
	}
}

// thinking is the wait for the answer: three dots taking turns, what was heard, and a word when it is
// taking long.
func (f *frame) thinking() {
	cy := f.px(headerRoom) + f.px(110)
	step := int(f.s.Now.UnixMilli()/300) % 3
	for i := range 3 {
		col := blend(f.t.Ground, f.t.Accent, 0.35)
		if i == step {
			col = f.t.Accent
		}
		fillCircle(f.dst, image.Pt(f.w/2+(i-1)*f.px(44), cy), f.px(13), col)
	}
	title := "考えています"
	if !f.s.Since.IsZero() && f.s.Now.Sub(f.s.Since) >= slowThinking {
		title = "時間がかかっています…"
	}
	f.centered(f.text(true, 40), f.t.Text, cy+f.px(80), f.words(title, "Thinking"))
	if f.s.Heard != "" {
		face := f.text(false, 30)
		lines := wrap(face, "「"+f.words(f.s.Heard)+"」", f.w-f.px(160), 3)
		y := cy + f.px(140)
		for _, l := range lines {
			f.centered(face, f.t.Dim, y, l)
			y += lineHeight(face)
		}
	}
}

// answer is the reply while it is said and for a while after: what was heard on one line at the top,
// the reply below, kept in step with the voice when it is longer than the screen, and how far the
// voice has got along the foot. A lingering answer is put away by a tap.
func (f *frame) answer() {
	margin := f.px(48)
	top := f.px(headerRoom) + f.px(8)
	footRoom := f.px(40)
	heardFace, replyFace := f.text(false, 24), f.text(false, 36)
	textW := f.w - 2*margin
	if res := f.u.result; res != nil {
		// What was looked up on the right, the words on the left, a little smaller to fit.
		split := f.w * 55 / 100
		f.resultPanel(image.Rect(split, top, f.w-margin, f.h-footRoom), *res)
		textW = split - margin - f.px(24)
		replyFace = f.text(false, 30)
	}
	y := top + ascent(heardFace)
	if f.s.Heard != "" {
		drawText(f.dst, heardFace, f.t.Dim, margin, y, clip(heardFace, "「"+f.words(f.s.Heard)+"」", textW))
	}
	y += f.px(18)
	lh := max(1, lineHeight(replyFace))
	room := max(1, (f.h-footRoom-y)/lh)
	lines := wrap(replyFace, f.words(f.s.Reply), textW, 0)
	said := f.s.Said
	if f.s.Phase == PhaseLingering {
		said = 0 // the answer from its start, to be read again
	}
	from, to := window(len(lines), room, said)
	shown := append([]string(nil), lines[from:to]...)
	if to < len(lines) && len(shown) > 0 {
		shown[len(shown)-1] = ellipsize(replyFace, shown[len(shown)-1], textW)
	}
	if from > 0 && len(shown) > 0 {
		shown[0] = "…" + shown[0]
	}
	for _, l := range shown {
		y += lh
		drawText(f.dst, replyFace, f.t.Text, margin, y, l)
	}
	if f.s.Phase == PhaseReplying {
		track := image.Rect(margin+f.px(70), f.h-f.px(22), f.w-margin, f.h-f.px(16))
		fillRound(f.dst, track, track.Dy()/2, f.t.Rule)
		done := track
		done.Max.X = track.Min.X + int(float64(track.Dx())*math.Max(0, math.Min(1, f.s.Said)))
		if done.Dx() > 0 {
			fillRound(f.dst, done, track.Dy()/2, f.t.Accent)
		}
		f.bars(image.Rect(margin, f.h-f.px(34), margin+f.px(54), f.h-f.px(12)))
		return
	}
	hint := f.text(false, 20)
	label := f.words("タップで閉じる", "Tap to close")
	drawText(f.dst, hint, f.t.Dim, f.w-margin-textWidth(hint, label), f.h-f.px(16), label)
	f.zones = append(f.zones, zone{rect: f.dst.Rect, dismiss: true})
}

// bars is the speaker's bands as a little equalizer in r, or nothing when they are not known.
func (f *frame) bars(r image.Rectangle) {
	n := len(f.s.Levels)
	if n == 0 {
		return
	}
	gap := max(1, f.px(2))
	w := max(1, (r.Dx()-gap*(n-1))/n)
	for i, v := range f.s.Levels {
		v = math.Max(0.08, math.Min(1, v))
		h := int(float64(r.Dy()) * v)
		x := r.Min.X + i*(w+gap)
		fill(f.dst, image.Rect(x, r.Max.Y-h, x+w, r.Max.Y), f.t.Accent)
	}
}

// centered draws s on one line centered across the canvas with its baseline at y.
func (f *frame) centered(face font.Face, col color.RGBA, y int, s string) {
	s = clip(face, s, f.w-f.px(80))
	drawText(f.dst, face, col, (f.w-textWidth(face, s))/2, y, s)
}

// noJapaneseNote is said once, along the foot, in place of the words no font in reach can draw.
const noJapaneseNote = "Japanese font not installed yet"

// words is s as it can be drawn: as it is when a font has it, or else in it — a line of boxes being
// worse than nothing — empty, or the alternative given, when s needs Japanese and no font has it.
func (f *frame) words(s string, instead ...string) string {
	if !f.s.NoJapanese {
		return s
	}
	for _, r := range s {
		if wide(r) {
			if len(instead) > 0 {
				return instead[0]
			}
			return "…"
		}
	}
	return s
}

// japaneseDate is the date as a Japanese clock writes it: 11月4日（水）.
func japaneseDate(t time.Time) string {
	days := [...]string{"日", "月", "火", "水", "木", "金", "土"}
	return fmt.Sprintf("%d月%d日（%s）", int(t.Month()), t.Day(), days[t.Weekday()])
}

func mean(v []float64) float64 {
	if len(v) == 0 {
		return 0
	}
	sum := 0.0
	for _, x := range v {
		sum += math.Max(0, math.Min(1, x))
	}
	return sum / float64(len(v))
}

// --- shapes ---

func fill(dst *image.RGBA, r image.Rectangle, c color.RGBA) {
	draw.Draw(dst, r, image.NewUniform(c), image.Point{}, draw.Src)
}

// shade lays c over r at alpha a, out of 255.
func shade(dst *image.RGBA, r image.Rectangle, c color.RGBA, a uint8) {
	draw.DrawMask(dst, r, image.NewUniform(c), image.Point{}, image.NewUniform(color.Alpha{a}), image.Point{}, draw.Over)
}

// blend is a part of the way from a to b, 0 to 1.
func blend(a, b color.RGBA, t float64) color.RGBA {
	mix := func(x, y uint8) uint8 { return uint8(math.Round(float64(x) + (float64(y)-float64(x))*t)) }
	return color.RGBA{mix(a.R, b.R), mix(a.G, b.G), mix(a.B, b.B), 0xff}
}

// roundMask is a rounded rectangle's coverage, edges softened over a pixel; ring, when above zero, is
// the width of an outline instead of a filled shape.
type roundMask struct {
	r      image.Rectangle
	radius float64
	ring   float64
}

func (m roundMask) ColorModel() color.Model { return color.AlphaModel }
func (m roundMask) Bounds() image.Rectangle { return m.r }
func (m roundMask) At(x, y int) color.Color {
	// Distance from the point to the rectangle shrunk by the radius, less the radius: negative inside.
	px, py := float64(x)+0.5, float64(y)+0.5
	minX, minY := float64(m.r.Min.X)+m.radius, float64(m.r.Min.Y)+m.radius
	maxX, maxY := float64(m.r.Max.X)-m.radius, float64(m.r.Max.Y)-m.radius
	dx := math.Max(0, math.Max(minX-px, px-maxX))
	dy := math.Max(0, math.Max(minY-py, py-maxY))
	d := math.Hypot(dx, dy) - m.radius
	cover := math.Max(0, math.Min(1, 0.5-d))
	if m.ring > 0 {
		cover = math.Min(cover, math.Max(0, math.Min(1, d+m.ring+0.5)))
	}
	return color.Alpha{uint8(cover * 255)}
}

func fillRound(dst *image.RGBA, r image.Rectangle, radius int, c color.RGBA) {
	radius = min(radius, r.Dx()/2, r.Dy()/2)
	draw.DrawMask(dst, r, image.NewUniform(c), image.Point{}, roundMask{r: r, radius: float64(radius)}, r.Min, draw.Over)
}

func strokeRound(dst *image.RGBA, r image.Rectangle, radius, width int, c color.RGBA) {
	radius = min(radius, r.Dx()/2, r.Dy()/2)
	draw.DrawMask(dst, r, image.NewUniform(c), image.Point{}, roundMask{r: r, radius: float64(radius), ring: float64(width)}, r.Min, draw.Over)
}

func fillCircle(dst *image.RGBA, center image.Point, radius int, c color.RGBA) {
	r := image.Rect(center.X-radius, center.Y-radius, center.X+radius, center.Y+radius)
	fillRound(dst, r, radius, c)
}
