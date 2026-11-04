//go:build !dot && !spot

package display

// fork: native_result_fork.go.

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/HuskerMinion/techo5/echod/internal/feature/assistant"
	"github.com/HuskerMinion/techo5/echod/internal/feature/home"
	"github.com/HuskerMinion/techo5/echod/internal/lib/hass"
)

func TestDaysInJapanese(t *testing.T) {
	now := time.Date(2026, 11, 4, 9, 0, 0, 0, time.Local) // a Wednesday
	for d, want := range map[time.Time]string{
		now.Add(5 * time.Hour): "今日",
		now.AddDate(0, 0, 1):   "明日",
		now.AddDate(0, 0, 2):   "11/6（金）",
		now.AddDate(0, 0, -1):  "11/3（火）",
		now.AddDate(0, 1, 0):   "12/4（金）",
	} {
		if got := dayJa(d, now); got != want {
			t.Errorf("%v: %q, want %q", d, got, want)
		}
	}
}

// The words read back are the words assistant/todo.go writes.
func TestTodoResultReadBack(t *testing.T) {
	name, items, ok := todoResult("on the " + "shopping" + " list: " + strings.Join([]string{"milk", "eggs", "牛乳"}, "; "))
	if !ok || name != "shopping" || len(items) != 3 || items[2] != "牛乳" {
		t.Errorf("got %q %q %v", name, items, ok)
	}
	if name, items, ok := todoResult("nothing is left on the " + "chores" + " list"); !ok || name != "chores" || len(items) != 0 {
		t.Errorf("empty list: %q %q %v", name, items, ok)
	}
	if _, _, ok := todoResult("added to the shopping list"); ok {
		t.Error("add_todo's words read as a list")
	}
}

// The words read back are the words assistant/web.go writes.
func TestSearchRowsReadBack(t *testing.T) {
	var lines []string
	lines = append(lines, "answer: 東京タワーの高さは333メートル")
	lines = append(lines, "about Tokyo Tower: a tower (height 333 m)")
	for i, r := range []struct{ title, url, text string }{
		{"東京タワー - Wikipedia", "https://ja.wikipedia.org/wiki/東京タワー", "東京タワーは <script> ではなく"},
		{"Tokyo Tower official", "https://www.tokyotower.co.jp/", "Open 9:00"},
	} {
		lines = append(lines, fmt.Sprintf("%d. %s <%s> %s", i+1, r.title, r.url, r.text))
	}
	rows := searchRows(strings.Join(lines, "\n"))
	if len(rows) != 3 {
		t.Fatalf("rows %+v", rows)
	}
	if rows[0].Value != "回答" || rows[1].Label != "東京タワー - Wikipedia" || rows[1].Value != "ja.wikipedia.org" || rows[2].Value != "tokyotower.co.jp" {
		t.Errorf("rows %+v", rows)
	}
	if rows := searchRows("no results"); len(rows) != 0 {
		t.Errorf("no results gave %+v", rows)
	}
}

func TestLookupsForTools(t *testing.T) {
	now := time.Date(2026, 11, 4, 9, 0, 0, 0, time.Local)
	if r, ok := lookupFor(assistant.Look{Tool: "web_search", Args: `{"query":"東京タワー 高さ"}`,
		Result: "1. 東京タワー <https://example.com/t> 333m"}, now); !ok || r.Title != "検索: 東京タワー 高さ" || len(r.Rows) != 1 || !r.At.Equal(now) {
		t.Errorf("web_search: %+v %v", r, ok)
	}
	if r, ok := lookupFor(assistant.Look{Tool: "read_todo", Args: `{}`, Result: "on the 買い物 list: 牛乳; 卵"}, now); !ok || r.Title != "買い物" || len(r.Rows) != 2 {
		t.Errorf("read_todo: %+v %v", r, ok)
	}
	if r, ok := lookupFor(assistant.Look{Tool: "calendar", Args: `{"date":"2030-01-01","days":2}`}, now); !ok || len(r.Rows) != 1 || r.Rows[0].Label != "予定はありません" {
		t.Errorf("calendar with nothing on: %+v %v", r, ok)
	}
	for _, tool := range []string{"start_timer", "set_volume", "play_radio"} {
		if _, ok := lookupFor(assistant.Look{Tool: tool, Args: `{}`, Result: "done"}, now); ok {
			t.Errorf("%s put a lookup up", tool)
		}
	}
	if home.Get().Weather().Condition == "" && len(home.Get().Forecast()) == 0 {
		if _, ok := lookupFor(assistant.Look{Tool: "weather", Args: `{}`}, now); ok {
			t.Error("a weather lookup with no weather")
		}
	}
}

func TestWeatherCard(t *testing.T) {
	now := time.Date(2026, 11, 4, 9, 0, 0, 0, time.Local)
	c := weatherCard(home.Weather{Condition: "sunny", Temp: "18°"}, []hass.Day{
		{When: now, Condition: "sunny", High: 18, Low: 9, Rain: 10},
		{When: now.AddDate(0, 0, 1), Condition: "rainy", High: 15, Low: 11, Rain: -1},
	}, now)
	if c.Value != "晴れ 18°" {
		t.Errorf("value %q", c.Value)
	}
	if !strings.Contains(c.Detail, "今日  晴れ  最高 18° / 最低 9°・降水 10%") || !strings.Contains(c.Detail, "明日  雨") || strings.Contains(c.Detail, "降水 -1") {
		t.Errorf("detail %q", c.Detail)
	}
	if c := weatherCard(home.Weather{}, nil, now); c.Value != "未取得" {
		t.Errorf("no weather: %q", c.Value)
	}
}

func TestBackendMessagesReachTheNativeUI(t *testing.T) {
	useNative(t, true)
	lastSent.Store(nil)
	t.Cleanup(func() { lastSent.Store(nil) })
	if err := nativeMessage([]byte(`{"version":1,"kind":"cards","cards":[{"id":"tasks","title":"タスク","value":"3件"}]}`)); err != nil {
		t.Fatal(err)
	}
	c := nativeContent(time.Now())
	if len(c.Cards) != 3 || c.Cards[2].Value != "3件" {
		t.Errorf("cards %+v", c.Cards)
	}
	if c := nativeContent(time.Now().Add(sentCardsFor)); c.Cards[2].Value != "未接続" {
		t.Errorf("stale cards kept: %+v", c.Cards[2])
	}
	if err := nativeMessage([]byte(`{"version":1,"kind":"phase","phase":"speaking"}`)); err == nil {
		t.Error("a message that sets the phase was taken")
	}
}
