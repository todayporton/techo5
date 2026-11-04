package nativeui

import (
	"strings"
	"testing"
	"time"
)

func TestUIMessagesAreHeldToTheSchema(t *testing.T) {
	good := []string{
		`{"version":1,"kind":"lookup","request_id":"t1","lookup":{"title":"タスク","rows":[{"label":"メール確認"},{"label":"資料作成","value":"今日まで"}]}}`,
		`{"version":1,"kind":"cards","cards":[{"id":"tasks","title":"タスク","value":"3件","detail":"メール確認\n資料作成"}]}`,
		`{"version":1,"kind":"cards","cards":[]}`,
	}
	for _, s := range good {
		if _, err := ParseMessage([]byte(s)); err != nil {
			t.Errorf("%s: %v", s, err)
		}
	}
	bad := map[string]string{
		"version 2":      `{"version":2,"kind":"cards","cards":[]}`,
		"no version":     `{"kind":"cards","cards":[]}`,
		"unknown kind":   `{"version":1,"kind":"phase","phase":"speaking"}`,
		"unknown field":  `{"version":1,"kind":"cards","cards":[],"phase":"speaking"}`,
		"no lookup":      `{"version":1,"kind":"lookup"}`,
		"rowless label":  `{"version":1,"kind":"lookup","lookup":{"title":"x","rows":[{"value":"v"}]}}`,
		"long title":     `{"version":1,"kind":"lookup","lookup":{"title":"` + strings.Repeat("あ", 81) + `","rows":[]}}`,
		"too many rows":  `{"version":1,"kind":"lookup","lookup":{"title":"x","rows":[` + strings.TrimSuffix(strings.Repeat(`{"label":"a"},`, 31), ",") + `]}}`,
		"bad card id":    `{"version":1,"kind":"cards","cards":[{"id":"Tasks!","title":"t","value":"v"}]}`,
		"card no value":  `{"version":1,"kind":"cards","cards":[{"id":"t","title":"t","value":""}]}`,
		"too many cards": `{"version":1,"kind":"cards","cards":[` + strings.TrimSuffix(strings.Repeat(`{"id":"a","title":"t","value":"v"},`, 7), ",") + `]}`,
		"two messages":   `{"version":1,"kind":"cards","cards":[]}{"version":1}`,
		"not json":       `hello`,
	}
	for name, s := range bad {
		if _, err := ParseMessage([]byte(s)); err == nil {
			t.Errorf("%s was taken", name)
		}
	}
}

func TestLookupAndCardsOfAMessage(t *testing.T) {
	m, err := ParseMessage([]byte(`{"version":1,"kind":"lookup","lookup":{"title":"タスク","rows":[{"label":"a","value":"b"}]}}`))
	if err != nil {
		t.Fatal(err)
	}
	r := m.LookupOf(at)
	if r.Title != "タスク" || len(r.Rows) != 1 || r.Rows[0] != (Row{"a", "b"}) || !r.At.Equal(at) {
		t.Errorf("lookup %+v", r)
	}
	m, _ = ParseMessage([]byte(`{"version":1,"kind":"cards","cards":[{"id":"tasks","title":"タスク","value":"3件"},{"id":"news","title":"ニュース","value":"5件"}]}`))
	merged := MergeCards(DemoContent().Cards, m.CardsOf())
	if len(merged) != 4 || merged[2].Value != "3件" || merged[3].ID != "news" {
		t.Errorf("merged %+v", merged)
	}
	_ = time.Second
}
