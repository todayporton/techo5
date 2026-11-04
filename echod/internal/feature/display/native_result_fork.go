//go:build !dot && !spot

package display

// fork: what the native UI shows besides the turn (native_fork.go): the dashboard's cards, from what
// the device already keeps without Home Assistant (its own weather, the calendar links), and a turn's
// lookups, from what the assistant's tools found (assistant.Looked).

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/url"
	"regexp"
	"strings"
	"sync/atomic"
	"time"

	"github.com/HuskerMinion/techo5/echod/internal/feature/assistant"
	"github.com/HuskerMinion/techo5/echod/internal/feature/home"
	"github.com/HuskerMinion/techo5/echod/internal/feature/nativeui"
	"github.com/HuskerMinion/techo5/echod/internal/feature/remote"
	"github.com/HuskerMinion/techo5/echod/internal/lib/hass"
	"github.com/HuskerMinion/techo5/echod/internal/lib/locale"
)

func init() {
	assistant.Looked.Listen(func(l assistant.Look) {
		if r, ok := lookupFor(l, time.Now()); ok {
			native.SetLookup(r)
		}
	})
	remote.HandleUI(nativeMessage)
}

// sentCardsFor is how long a backend's cards are shown after they arrived: one that has stopped
// sending is not left showing this morning's tasks tonight.
const sentCardsFor = time.Hour

type sentCards struct {
	cards []nativeui.Card
	at    time.Time
}

// lastSent is the backend's latest cards.
var lastSent atomic.Pointer[sentCards]

// nativeMessage is a UI message from the backend (remote API, POST /api/v1/ui).
func nativeMessage(body []byte) error {
	m, err := nativeui.ParseMessage(body)
	if err != nil {
		return err
	}
	now := time.Now()
	switch m.Kind {
	case nativeui.KindLookup:
		native.SetLookup(m.LookupOf(now))
	case nativeui.KindCards:
		lastSent.Store(&sentCards{cards: m.CardsOf(), at: now})
	}
	slog.Info("nativeui: a message from the backend", "kind", m.Kind) // what it says is not logged
	if d := nativeDisplay.Load(); d != nil {
		d.wake()
	}
	return nil
}

// The native UI is Japanese for now: its words, and the weather's.
const nativeLang = "ja"

var weekdaysJa = [...]string{"日", "月", "火", "水", "木", "金", "土"}

// dayJa is a day as the native UI names it: 今日, 明日, or 11/6（金）.
func dayJa(d, now time.Time) string {
	y1, m1, d1 := now.Date()
	today := time.Date(y1, m1, d1, 0, 0, 0, 0, now.Location())
	switch day := time.Date(d.Year(), d.Month(), d.Day(), 0, 0, 0, 0, now.Location()); {
	case day.Equal(today):
		return "今日"
	case day.Equal(today.AddDate(0, 0, 1)):
		return "明日"
	}
	return fmt.Sprintf("%d/%d（%s）", int(d.Month()), d.Day(), weekdaysJa[d.Weekday()])
}

// skyJa is a weather condition in Japanese, or the condition as it is when the catalog has no word.
func skyJa(cond string) string { return locale.Sky(cond, nativeLang) }

// nativeContent is the dashboard's cards as the device knows them now. Cheap: home keeps all of it.
func nativeContent(now time.Time) nativeui.Content {
	own := []nativeui.Card{
		weatherCard(home.Get().Weather(), home.Get().Forecast(), now),
		eventsCard(now),
		{ID: "tasks", Title: "タスク", Value: "未接続",
			Detail: "タスクはMac mini側のサーバーから届くと表示されます。"},
	}
	if s := lastSent.Load(); s != nil && now.Sub(s.at) < sentCardsFor {
		own = nativeui.MergeCards(own, s.cards)
	}
	return nativeui.Content{Cards: own}
}

func weatherCard(w home.Weather, days []hass.Day, now time.Time) nativeui.Card {
	c := nativeui.Card{ID: "weather", Title: "天気"}
	if w.Condition == "" && len(days) == 0 {
		c.Value = "未取得"
		c.Detail = "セットアップページで場所を設定すると、この端末が天気を取得します（Open-Meteo）。"
		return c
	}
	c.Value = strings.TrimSpace(skyJa(w.Condition) + " " + w.Temp)
	var lines []string
	for _, r := range forecastRows(days, now, 5) {
		lines = append(lines, r.Label+"  "+r.Value)
	}
	c.Detail = strings.Join(lines, "\n")
	return c
}

func eventsCard(now time.Time) nativeui.Card {
	c := nativeui.Card{ID: "events", Title: "今日の予定"}
	evs, _ := home.Get().EventsOn(now)
	var next *hass.Event
	var lines []string
	for i, e := range evs {
		lines = append(lines, eventLabel(e))
		if next == nil && (e.AllDay || e.End.After(now)) {
			next = &evs[i]
		}
	}
	switch {
	case len(evs) == 0:
		c.Value = "予定なし"
		c.Detail = "今日の予定はありません。カレンダーはセットアップページでiCalのURLを登録すると表示されます。"
	case next == nil:
		c.Value = "残りなし"
		c.Detail = strings.Join(lines, "\n")
	default:
		c.Value = eventLabel(*next)
		c.Detail = strings.Join(lines, "\n")
	}
	return c
}

func eventLabel(e hass.Event) string {
	if e.AllDay {
		return "終日 " + e.Summary
	}
	return e.Start.Local().Format("15:04") + " " + e.Summary
}

// forecastRows is up to n days of forecast as rows: the day and the sky, then the temperatures.
func forecastRows(days []hass.Day, now time.Time, n int) []nativeui.Row {
	var rows []nativeui.Row
	for i, d := range days {
		if i >= n {
			break
		}
		v := fmt.Sprintf("最高 %.0f° / 最低 %.0f°", d.High, d.Low)
		if d.Rain >= 0 {
			v += fmt.Sprintf("・降水 %d%%", d.Rain)
		}
		rows = append(rows, nativeui.Row{Label: dayJa(d.When, now) + "  " + skyJa(d.Condition), Value: v})
	}
	return rows
}

// lookupFor is what the native UI shows for a tool's result, when it shows anything for it.
func lookupFor(l assistant.Look, now time.Time) (nativeui.Lookup, bool) {
	var args map[string]any
	_ = json.Unmarshal([]byte(l.Args), &args)
	arg := func(k string) string { s, _ := args[k].(string); return strings.TrimSpace(s) }
	r := nativeui.Lookup{At: now}
	switch l.Tool {
	case "weather":
		w := home.Get().Weather()
		days := home.Get().Forecast()
		if w.Condition == "" && len(days) == 0 {
			return r, false
		}
		r.Title = "天気"
		if w.Condition != "" {
			r.Rows = append(r.Rows, nativeui.Row{Label: "現在  " + strings.TrimSpace(skyJa(w.Condition)+" "+w.Temp)})
		}
		r.Rows = append(r.Rows, forecastRows(days, now, 5)...)
	case "calendar":
		first := now
		if t, err := time.ParseInLocation("2006-01-02", arg("date"), time.Local); err == nil {
			first = t
		}
		n := 1
		if v, ok := args["days"].(float64); ok {
			n = int(max(1, min(14, v)))
		}
		r.Title = "予定"
		for i := range n {
			day := first.AddDate(0, 0, i)
			evs, _ := home.Get().EventsOn(day)
			for _, e := range evs {
				row := nativeui.Row{Label: eventLabel(e)}
				if n > 1 {
					row.Value = dayJa(day, now)
				}
				if e.Location != "" {
					row.Value = strings.TrimSpace(row.Value + " " + e.Location)
				}
				r.Rows = append(r.Rows, row)
			}
		}
		if len(r.Rows) == 0 {
			r.Rows = []nativeui.Row{{Label: "予定はありません"}}
		}
	case "read_todo":
		name, items, ok := todoResult(l.Result)
		if !ok {
			return r, false
		}
		r.Title = name
		for _, it := range items {
			r.Rows = append(r.Rows, nativeui.Row{Label: "□ " + it})
		}
		if len(items) == 0 {
			r.Rows = []nativeui.Row{{Label: "残っている項目はありません"}}
		}
	case "web_search":
		r.Title = "検索: " + arg("query")
		r.Rows = searchRows(l.Result)
		if len(r.Rows) == 0 {
			return r, false
		}
	default:
		return r, false
	}
	return r, true
}

// todoResult reads read_todo's words back (assistant/todo.go): the list's name and its items.
func todoResult(s string) (name string, items []string, ok bool) {
	if rest, found := strings.CutPrefix(s, "on the "); found {
		name, list, found := strings.Cut(rest, " list: ")
		if !found {
			return "", nil, false
		}
		for _, it := range strings.Split(list, "; ") {
			if it = strings.TrimSpace(it); it != "" {
				items = append(items, it)
			}
		}
		return name, items, true
	}
	if rest, found := strings.CutPrefix(s, "nothing is left on the "); found {
		return strings.TrimSuffix(rest, " list"), nil, true
	}
	return "", nil, false
}

// searchLine is one of web_search's results: "1. Title <https://example.com/page> what it says".
var searchLine = regexp.MustCompile(`^\d+\. (.*?) <(\S+)>`)

// searchRows reads web_search's words back (assistant/web.go): the quick answer, then each result's
// title over the site it is on.
func searchRows(s string) []nativeui.Row {
	var rows []nativeui.Row
	for _, line := range strings.Split(s, "\n") {
		if a, ok := strings.CutPrefix(line, "answer: "); ok {
			rows = append(rows, nativeui.Row{Label: a, Value: "回答"})
			continue
		}
		m := searchLine.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		title, site := strings.TrimSpace(m[1]), m[2]
		if u, err := url.Parse(site); err == nil && u.Host != "" {
			site = strings.TrimPrefix(u.Host, "www.")
		}
		rows = append(rows, nativeui.Row{Label: title, Value: site})
	}
	return rows
}
