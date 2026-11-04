package nativeui

import (
	"image"
	"strconv"
	"time"
)

// Lookup is what a turn looked up, to show beside the answer and for a while after it: a forecast, a
// day's events, a list, search results. Built by display from what the assistant's tools found, or
// later sent by a backend; the speech is the reply, this is what is on the screen with it.
type Lookup struct {
	Title string
	Rows  []Row
	// At is when it was found. A result found before the turn on the screen began is not that turn's.
	At time.Time
}

// Row is one line of a lookup: what it is, and a smaller line under it (empty for none).
type Row struct {
	Label string
	Value string
}

// resultHold is how long a result stays up after its turn, unless a tap puts it away first.
const resultHold = 60 * time.Second

// SetLookup puts up r for the turn under way.
func (u *UI) SetLookup(r Lookup) {
	u.mu.Lock()
	u.result = &r
	u.mu.Unlock()
}

// resultHeld is whether the result in hand is still to be shown on the idle screen at now.
func (u *UI) resultHeld(now time.Time) bool {
	return u.result != nil && now.Before(u.result.At.Add(resultHold))
}

// resultPage is a held result after its turn, on its own: a tap puts it away.
func (f *frame) resultPage() {
	margin := f.px(40)
	r := image.Rect(margin, f.px(headerRoom)+f.px(4), f.w-margin, f.h-f.px(44))
	f.resultPanel(r, *f.u.result)
	hint := f.text(false, 20)
	label := f.words("タップで閉じる", "Tap to close")
	drawText(f.dst, hint, f.t.Dim, f.w-margin-textWidth(hint, label), f.h-f.px(16), label)
	f.zones = append(f.zones, zone{rect: f.dst.Rect, dismiss: true})
}

// resultPanel draws res in r: its title, then as many rows as fit.
func (f *frame) resultPanel(r image.Rectangle, res Lookup) {
	fillRound(f.dst, r, f.px(16), f.surface())
	pad := f.px(22)
	titleFace, labelFace, valueFace := f.text(true, 26), f.text(false, 24), f.text(false, 19)
	inner := r.Dx() - 2*pad
	y := r.Min.Y + pad + ascent(titleFace)
	drawText(f.dst, titleFace, f.t.Accent, r.Min.X+pad, y, clip(titleFace, f.words(res.Title), inner))
	y += f.px(12)
	gap := f.px(10)
	for i, row := range res.Rows {
		h := lineHeight(labelFace)
		if row.Value != "" {
			h += lineHeight(valueFace)
		}
		if y+gap+h > r.Max.Y-pad {
			if i < len(res.Rows) {
				more := f.words("ほか" + strconv.Itoa(len(res.Rows)-i) + "件")
				drawText(f.dst, valueFace, f.t.Dim, r.Max.X-pad-textWidth(valueFace, more), r.Max.Y-pad+f.px(4), more)
			}
			break
		}
		if i > 0 {
			fill(f.dst, image.Rect(r.Min.X+pad, y+gap/2, r.Max.X-pad, y+gap/2+max(1, f.px(1))), blend(f.surface(), f.t.Dim, 0.35))
		}
		y += gap + ascent(labelFace)
		drawText(f.dst, labelFace, f.t.Text, r.Min.X+pad, y, clip(labelFace, f.words(row.Label), inner))
		if row.Value != "" {
			y += lineHeight(valueFace)
			drawText(f.dst, valueFace, f.t.Dim, r.Min.X+pad, y, clip(valueFace, f.words(row.Value), inner))
		}
		y += lineHeight(labelFace) - ascent(labelFace)
	}
}
