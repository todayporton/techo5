package nativeui

import (
	"image"
	"testing"
	"time"
)

var weatherLookup = Lookup{Title: "天気", Rows: []Row{
	{Label: "現在  晴れ 18°"},
	{Label: "今日  晴れ", Value: "最高 18° / 最低 9°・降水 10%"},
	{Label: "明日  雨", Value: "最高 15° / 最低 11°・降水 80%"},
	{Label: "11/6（金）  曇り", Value: "最高 16° / 最低 10°・降水 30%"},
	{Label: "11/7（土）  晴れ", Value: "最高 19° / 最低 8°・降水 0%"},
	{Label: "11/8（日）  晴れ", Value: "最高 20° / 最低 9°・降水 0%"},
}}

func TestALookupStaysAfterItsTurnUntilATapOrTheNextTurn(t *testing.T) {
	u := New()
	img := image.NewRGBA(image.Rect(0, 0, 960, 480))
	turn := at
	u.Draw(img, Snapshot{Now: turn, Phase: PhaseListening, Since: turn})
	r := weatherLookup
	r.At = turn.Add(2 * time.Second)
	u.SetLookup(r)
	u.Draw(img, Snapshot{Now: turn.Add(3 * time.Second), Phase: PhaseReplying, Reply: "明日は雨です"})
	if u.Shown() != ScreenAnswer || u.result == nil {
		t.Fatalf("%s shown, lookup %v", u.Shown(), u.result != nil)
	}
	u.Draw(img, Snapshot{Now: turn.Add(30 * time.Second), Phase: PhaseIdle})
	if u.Shown() != ScreenResult {
		t.Errorf("after the turn: %s, want the lookup held", u.Shown())
	}
	u.Draw(img, Snapshot{Now: turn.Add(2*time.Second + resultHold), Phase: PhaseIdle})
	if u.Shown() != ScreenDashboard {
		t.Errorf("after the hold: %s, want the dashboard", u.Shown())
	}

	// A tap puts it away.
	r.At = turn.Add(time.Hour)
	u.SetLookup(r)
	u.Draw(img, Snapshot{Now: r.At, Phase: PhaseIdle})
	if got := u.Touch(Input{Kind: Tap, X: 480, Y: 240}); got != Handled {
		t.Fatalf("tap on a held lookup = %v", got)
	}
	u.Draw(img, Snapshot{Now: r.At, Phase: PhaseIdle})
	if u.Shown() != ScreenDashboard {
		t.Errorf("after the tap: %s", u.Shown())
	}

	// The next turn starts without the last one's.
	u.SetLookup(r)
	u.Draw(img, Snapshot{Now: r.At.Add(time.Minute), Phase: PhaseListening, Since: r.At.Add(time.Minute)})
	if u.result != nil {
		t.Error("the last turn's lookup came into the new one")
	}
}

func TestLookupScreensDraw(t *testing.T) {
	previewFonts(t)
	search := Lookup{Title: "検索: 東京タワー 高さ", Rows: []Row{
		{Label: "東京タワーの高さは333メートル", Value: "回答"},
		{Label: "東京タワー - Wikipedia", Value: "ja.wikipedia.org"},
		{Label: "東京タワー公式サイト", Value: "tokyotower.co.jp"},
		{Label: "東京タワーの歴史と構造", Value: "example.com"},
		{Label: "展望台の営業時間", Value: "example.org"},
	}}
	for _, sz := range sizes {
		for name, sc := range map[string]struct {
			r Lookup
			s Snapshot
		}{
			"answer-weather": {weatherLookup, Snapshot{Phase: PhaseReplying, Heard: "今週の天気は？", Reply: "明日は雨で、週末にかけて晴れる見込みです。", Said: 0.5}},
			"answer-search":  {search, Snapshot{Phase: PhaseLingering, Heard: "東京タワーの高さは？", Reply: "東京タワーの高さは333メートルです。"}},
			"result-held":    {weatherLookup, Snapshot{Phase: PhaseIdle}},
		} {
			u := New()
			sc.r.At = at
			u.SetLookup(sc.r)
			sc.s.Now = at.Add(10 * time.Second)
			img := image.NewRGBA(image.Rect(0, 0, sz.w, sz.h))
			u.Draw(img, sc.s)
			save(t, name+"-"+sz.name, img)
		}
	}
}
