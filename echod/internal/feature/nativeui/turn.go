package nativeui

import (
	"image"
	"math"
	"strings"
	"time"
)

// noticeFor is how long the dashboard says why a turn went away unanswered.
const noticeFor = 4 * time.Second

// slowThinking is how long thinking goes on before the screen says it is taking a while.
const slowThinking = 12 * time.Second

// noticeText is what the dashboard says about a turn that ended without an answer, or nothing for an
// ending that needs no word: an answer, a turn another one took over, a reply another sound cut off.
func noticeText(e Ending) (title, sub string, problem bool) {
	switch e.How {
	case EndNoSpeech:
		return "聞き取れませんでした", "もう一度話しかけてください", false
	case EndCanceled:
		switch e.Reason {
		case "stopped":
			return "キャンセルしました", "", false
		case "answered_elsewhere":
			return "別の端末が応答しました", "", false
		}
	case EndFailed:
		if e.TimedOut {
			return "応答がありませんでした", "時間内に返事が届きませんでした", true
		}
		switch {
		case e.Failure == "stt-failed":
			return "音声認識に失敗しました", "音声認識サーバーに接続できません", true
		case e.Failure == "intent-failed":
			return "AIの応答に失敗しました", "AIサーバーに接続できません", true
		case e.Failure == "tts-failed":
			return "読み上げに失敗しました", "音声合成サーバーに接続できません", true
		case strings.HasPrefix(e.Failure, "realtime"):
			return "AIの応答に失敗しました", "", true
		}
		return "エラーが発生しました", "", true
	}
	return "", "", false
}

// noticeUntil is when the notice for s's last ending goes, and whether there is one now.
func noticeUntil(s Snapshot) (time.Time, bool) {
	if s.Phase != PhaseIdle || s.Ended.At.IsZero() {
		return time.Time{}, false
	}
	if title, _, _ := noticeText(s.Ended); title == "" {
		return time.Time{}, false
	}
	until := s.Ended.At.Add(noticeFor)
	return until, s.Now.Before(until) && !s.Now.Before(s.Ended.At)
}

// notice is the word about an unanswered turn, a card along the foot of the dashboard for a few
// seconds. It covers the status line, so the Standard button stays where a finger expects it.
func (f *frame) notice() bool {
	if _, ok := noticeUntil(f.s); !ok {
		return false
	}
	title, sub, problem := noticeText(f.s.Ended)
	titleFace, subFace := f.text(true, 26), f.text(false, 20)
	title, sub = f.words(title, "Not understood"), f.words(sub, "")
	// As wide as the status line's column, so none of the line it covers shows past it.
	w := f.w*56/100 - f.px(40)
	h := f.px(80)
	if sub == "" {
		h = f.px(52)
	}
	r := image.Rect(f.px(30), f.h-h-f.px(8), f.px(30)+w, f.h-f.px(8))
	fillRound(f.dst, r, f.px(14), f.surface())
	mark := f.t.Dim
	if problem {
		mark = f.t.Accent
	}
	fillRound(f.dst, image.Rect(r.Min.X, r.Min.Y, r.Min.X+f.px(6), r.Max.Y), f.px(3), mark)
	y := r.Min.Y + f.px(14) + ascent(titleFace)
	drawText(f.dst, titleFace, f.t.Text, r.Min.X+f.px(26), y, clip(titleFace, title, w-f.px(40)))
	if sub != "" {
		drawText(f.dst, subFace, f.t.Dim, r.Min.X+f.px(26), y+f.px(8)+ascent(subFace), clip(subFace, sub, w-f.px(40)))
	}
	return true
}

// window is which of n lines to show, at most room of them, so the line the voice is on stays in sight:
// from the top until the voice is past the middle of what fits, then moving with it, ending on the last.
func window(n, room int, said float64) (from, to int) {
	if room <= 0 {
		return 0, 0
	}
	if n <= room {
		return 0, n
	}
	at := int(math.Floor(math.Max(0, math.Min(1, said)) * float64(n)))
	from = min(max(0, at-room/2), n-room)
	return from, from + room
}

// Status is the dashboard's line about the assistant, from what the device knows without asking a
// server: whether a voice pipeline is set up, and how the last turn went. Offline is a problem to show.
func Status(configured bool, last Ending) (text string, offline bool) {
	if !configured {
		return "音声AIが未設定です", true
	}
	if last.How == EndFailed {
		_, sub, _ := noticeText(last)
		if sub == "" {
			sub = "AIの応答に失敗しました"
		}
		return sub, true
	}
	return "音声AI 準備完了", false
}
