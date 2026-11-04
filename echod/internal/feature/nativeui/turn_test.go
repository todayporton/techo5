package nativeui

import (
	"image"
	"strings"
	"testing"
	"time"
)

func TestNoticeSaysWhyATurnWentUnanswered(t *testing.T) {
	cases := []struct {
		e       Ending
		want    string
		problem bool
	}{
		{Ending{How: EndNoSpeech}, "聞き取れませんでした", false},
		{Ending{How: EndCanceled, Reason: "stopped"}, "キャンセルしました", false},
		{Ending{How: EndCanceled, Reason: "interrupted"}, "", false},
		{Ending{How: EndCanceled, Reason: "cut_off"}, "", false},
		{Ending{How: EndFailed, Failure: "stt-failed"}, "音声認識に失敗しました", true},
		{Ending{How: EndFailed, Failure: "intent-failed"}, "AIの応答に失敗しました", true},
		{Ending{How: EndFailed, Failure: "tts-failed"}, "読み上げに失敗しました", true},
		{Ending{How: EndFailed, TimedOut: true}, "応答がありませんでした", true},
		{Ending{How: EndFailed, Failure: "something-new"}, "エラーが発生しました", true},
		{Ending{How: EndAnswered}, "", false},
	}
	for _, c := range cases {
		title, _, problem := noticeText(c.e)
		if title != c.want || problem != c.problem {
			t.Errorf("%+v: %q %v, want %q %v", c.e, title, problem, c.want, c.problem)
		}
	}
}

func TestNoticeIsUpForAWhileAndOnlyWhenIdle(t *testing.T) {
	e := Ending{How: EndNoSpeech, At: at}
	if _, ok := noticeUntil(Snapshot{Now: at.Add(time.Second), Phase: PhaseIdle, Ended: e}); !ok {
		t.Error("no notice a second after")
	}
	if _, ok := noticeUntil(Snapshot{Now: at.Add(noticeFor), Phase: PhaseIdle, Ended: e}); ok {
		t.Error("notice still up after noticeFor")
	}
	if _, ok := noticeUntil(Snapshot{Now: at.Add(time.Second), Phase: PhaseListening, Ended: e}); ok {
		t.Error("notice over a new turn")
	}
	u := New()
	if got := u.Next(Snapshot{Now: at.Add(noticeFor - 300*time.Millisecond), Phase: PhaseIdle, Ended: e}); got != 300*time.Millisecond {
		t.Errorf("next frame %v, want when the notice goes", got)
	}
}

func TestWindowFollowsTheVoice(t *testing.T) {
	cases := []struct {
		n, room  int
		said     float64
		from, to int
	}{
		{3, 5, 0.5, 0, 3},
		{10, 4, 0, 0, 4},
		{10, 4, 0.5, 3, 7},
		{10, 4, 1, 6, 10},
		{10, 4, 2, 6, 10},
		{10, 0, 0.5, 0, 0},
	}
	for _, c := range cases {
		if from, to := window(c.n, c.room, c.said); from != c.from || to != c.to {
			t.Errorf("window(%d, %d, %v) = %d..%d, want %d..%d", c.n, c.room, c.said, from, to, c.from, c.to)
		}
	}
}

func TestStatus(t *testing.T) {
	if s, off := Status(false, Ending{}); !off || !strings.Contains(s, "未設定") {
		t.Errorf("unconfigured: %q %v", s, off)
	}
	if s, off := Status(true, Ending{How: EndAnswered}); off || s == "" {
		t.Errorf("answered: %q %v", s, off)
	}
	if s, off := Status(true, Ending{How: EndFailed, Failure: "stt-failed"}); !off || !strings.Contains(s, "音声認識") {
		t.Errorf("stt failed: %q %v", s, off)
	}
	if _, off := Status(true, Ending{How: EndNoSpeech}); off {
		t.Error("nothing said is not the server's problem")
	}
}

func TestATapPutsALingeringAnswerAway(t *testing.T) {
	u := New()
	img := image.NewRGBA(image.Rect(0, 0, 960, 480))
	linger := Snapshot{Now: at, Phase: PhaseLingering, Heard: "天気", Reply: "晴れです"}
	u.Draw(img, linger)
	if got := u.Touch(Input{Kind: Tap, X: 480, Y: 240}); got != Handled {
		t.Fatalf("tap on a lingering answer = %v", got)
	}
	u.Draw(img, linger)
	if _, ok := zoneOf(u, func(z zone) bool { return z.leave }); !ok {
		t.Error("the dashboard did not come back")
	}
	// The next turn's answer lingers again.
	u.Draw(img, Snapshot{Now: at, Phase: PhaseReplying, Reply: "曇りです"})
	u.Draw(img, Snapshot{Now: at, Phase: PhaseLingering, Reply: "曇りです"})
	if _, ok := zoneOf(u, func(z zone) bool { return z.dismiss }); !ok {
		t.Error("a new answer was put away by the old tap")
	}
}

func TestTurnScreensDraw(t *testing.T) {
	previewFonts(t)
	long := strings.Repeat("明日の東京は晴れのち曇りで、最高気温は18度の予報です。", 12)
	levels := []float64{0.9, 0.7, 0.5, 0.6, 0.3, 0.2, 0.4, 0.1}
	scenes := map[string]Snapshot{
		"notice-nospeech":  {Phase: PhaseIdle, Ended: Ending{How: EndNoSpeech, At: at.Add(-time.Second)}},
		"notice-stt":       {Phase: PhaseIdle, Ended: Ending{How: EndFailed, Failure: "stt-failed", At: at.Add(-time.Second)}},
		"notice-timeout":   {Phase: PhaseIdle, Ended: Ending{How: EndFailed, TimedOut: true, At: at.Add(-time.Second)}},
		"listening-levels": {Phase: PhaseListening, Levels: levels},
		"thinking-slow":    {Phase: PhaseThinking, Heard: "明日の天気", Since: at.Add(-20 * time.Second)},
		"replying-scroll":  {Phase: PhaseReplying, Heard: "明日の天気", Reply: long, Said: 0.6, Levels: levels},
		"lingering-long":   {Phase: PhaseLingering, Heard: "明日の天気", Reply: long},
	}
	for _, sz := range sizes {
		for name, s := range scenes {
			u := New()
			if strings.HasPrefix(name, "notice") {
				c := DemoContent()
				c.Status, c.Offline = Status(true, s.Ended)
				u.SetContent(c)
			}
			s.Now = at
			img := image.NewRGBA(image.Rect(0, 0, sz.w, sz.h))
			u.Draw(img, s)
			save(t, name+"-"+sz.name, img)
		}
	}
}
