package nativeui

import (
	"image"
	"image/color"
	"log/slog"
	"strings"
	"unicode"
	"unicode/utf8"

	"golang.org/x/image/font"
	"golang.org/x/image/math/fixed"

	"github.com/HuskerMinion/techo5/echod/internal/lib/fonts"
)

// faceKey is one face: the font, the own-fonts generation it was made in, weight and pixel size.
type faceKey struct {
	font string
	gen  uint64
	bold bool
	px   int
}

// faceCache keeps the faces made, dropped whole when the font or the own fonts change.
type faceCache struct {
	font  string
	gen   uint64
	faces map[faceKey]font.Face
}

// face is the screen font at px pixels. Called under mu.
func (u *UI) face(name string, bold bool, px int) font.Face {
	px = max(px, 6)
	gen := fonts.Generation()
	c := &u.faces
	if c.faces == nil || c.font != name || c.gen != gen {
		c.font, c.gen, c.faces = name, gen, map[faceKey]font.Face{}
	}
	k := faceKey{name, gen, bold, px}
	if f, ok := c.faces[k]; ok {
		return f
	}
	f, err := fonts.Face(name, bold, float64(px))
	if err != nil && name != fonts.Go {
		slog.Warn("nativeui: font face failed, drawing in Go", "font", name, "err", err)
		f, err = fonts.Face(fonts.Go, bold, float64(px))
	}
	if err != nil {
		slog.Error("nativeui: no font face", "err", err)
		return nil
	}
	c.faces[k] = f
	return f
}

// lineHeight is the face's line spacing in pixels.
func lineHeight(f font.Face) int { return f.Metrics().Height.Ceil() }

// ascent is how far above the baseline the face's tallest letters reach.
func ascent(f font.Face) int { return f.Metrics().Ascent.Ceil() }

// textWidth is how wide s is drawn in f.
func textWidth(f font.Face, s string) int { return font.MeasureString(f, s).Ceil() }

// drawText draws s in f with its baseline at y, starting at x.
func drawText(dst *image.RGBA, f font.Face, col color.Color, x, y int, s string) {
	if f == nil || s == "" {
		return
	}
	d := font.Drawer{Dst: dst, Src: image.NewUniform(col), Face: f, Dot: fixed.P(x, y)}
	d.DrawString(s)
}

// noStart are the characters a Japanese line does not begin with (kinsoku): closing brackets and
// punctuation stay with what they close.
const noStart = "、。，．,.!?！？）)」』】〕〉》・ー〜…‥ぁぃぅぇぉっゃゅょゎァィゥェォッャュョヮヵヶ"

// noEnd are the characters a line does not end with: opening brackets go with what they open.
const noEnd = "（(「『【〔〈《"

// wide is whether r is set one by one, a line able to break on either side of it: Japanese, Chinese
// and the full-width forms.
func wide(r rune) bool {
	return unicode.In(r, unicode.Han, unicode.Hiragana, unicode.Katakana) ||
		(r >= 0x3000 && r <= 0x303f) || (r >= 0xff00 && r <= 0xffef)
}

// tokens splits s into the pieces a line may break between: a Latin word with the spaces after it, or
// one wide character. Kinsoku then joins them: a piece that may not begin a line goes onto the one
// before it, and one that may not end a line takes the one after it. "\n" is a piece of its own.
func tokens(s string) []string {
	var raw []string
	var cur strings.Builder
	flush := func() {
		if cur.Len() > 0 {
			raw = append(raw, cur.String())
			cur.Reset()
		}
	}
	for _, r := range s {
		switch {
		case r == '\n':
			flush()
			raw = append(raw, "\n")
		case wide(r):
			flush()
			raw = append(raw, string(r))
		case unicode.IsSpace(r):
			cur.WriteRune(r)
			flush()
		default:
			cur.WriteRune(r)
		}
	}
	flush()

	var out []string
	joinNext := false
	for _, t := range raw {
		first, _ := utf8.DecodeRuneInString(t)
		last, _ := utf8.DecodeLastRuneInString(t)
		switch {
		case t == "\n":
			out = append(out, t)
		case joinNext && len(out) > 0 && out[len(out)-1] != "\n":
			out[len(out)-1] += t
		case strings.ContainsRune(noStart, first) && len(out) > 0 && out[len(out)-1] != "\n":
			out[len(out)-1] += t
		default:
			out = append(out, t)
		}
		joinNext = t != "\n" && strings.ContainsRune(noEnd, last)
	}
	return out
}

// wrap breaks s into lines no wider than width in f, at most maxLines of them (0 for any number); a
// last line that leaves words out ends in an ellipsis.
func wrap(f font.Face, s string, width, maxLines int) []string {
	if f == nil || width <= 0 {
		return nil
	}
	var lines []string
	var line string
	cut := false
	push := func() bool {
		lines = append(lines, strings.TrimRight(line, " "))
		line = ""
		if maxLines > 0 && len(lines) >= maxLines {
			return false
		}
		return true
	}
	toks := tokens(s)
	for i := 0; i < len(toks); i++ {
		t := toks[i]
		if t == "\n" {
			if !push() {
				cut = i+1 < len(toks)
				break
			}
			continue
		}
		if textWidth(f, line+t) <= width {
			line += t
			continue
		}
		if line != "" {
			if !push() {
				cut = true
				break
			}
			i-- // the token again, on the new line
			continue
		}
		// One piece wider than the line: it is broken by character.
		for _, r := range t {
			if line != "" && textWidth(f, line+string(r)) > width {
				if !push() {
					cut = true
					break
				}
			}
			line += string(r)
		}
		if cut {
			break
		}
	}
	if !cut && strings.TrimSpace(line) != "" {
		lines = append(lines, strings.TrimRight(line, " "))
	}
	if cut && len(lines) > 0 {
		lines[len(lines)-1] = ellipsize(f, lines[len(lines)-1], width)
	}
	return lines
}

// ellipsize ends s with an ellipsis, taking characters off its end until both fit in width.
func ellipsize(f font.Face, s string, width int) string {
	const dots = "…"
	for s != "" && textWidth(f, s+dots) > width {
		_, n := utf8.DecodeLastRuneInString(s)
		s = s[:len(s)-n]
	}
	return strings.TrimRight(s, " ") + dots
}

// clip is s on one line no wider than width, ellipsized if it is not.
func clip(f font.Face, s string, width int) string {
	s = strings.ReplaceAll(s, "\n", " ")
	if f == nil || textWidth(f, s) <= width {
		return s
	}
	return ellipsize(f, s, width)
}
