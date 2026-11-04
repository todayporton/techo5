package nativeui

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"time"
	"unicode/utf8"
)

// A UI message is what a backend sends the native UI (feature/remote's POST /api/v1/ui): a turn's
// result panel, or the dashboard's cards. Its schema is the backend's schema/ui-message.v1.schema.json;
// ParseMessage holds a message to it, limits included, so nothing oversized or unexpected is drawn.
type Message struct {
	Version   int        `json:"version"`
	Kind      string     `json:"kind"`
	RequestID string     `json:"request_id,omitempty"`
	Lookup    *msgLookup `json:"lookup,omitempty"`
	Cards     []msgCard  `json:"cards,omitempty"`
}

type msgLookup struct {
	Title string   `json:"title"`
	Rows  []msgRow `json:"rows"`
}

type msgRow struct {
	Label string `json:"label"`
	Value string `json:"value,omitempty"`
}

type msgCard struct {
	ID     string `json:"id"`
	Title  string `json:"title"`
	Value  string `json:"value"`
	Detail string `json:"detail,omitempty"`
}

// Message kinds.
const (
	KindLookup = "lookup"
	KindCards  = "cards"
)

var cardID = regexp.MustCompile(`^[a-z0-9_-]{1,32}$`)

// ParseMessage reads a UI message, refusing one that is not version 1 or breaks the schema's limits.
func ParseMessage(b []byte) (Message, error) {
	var m Message
	d := json.NewDecoder(bytes.NewReader(b))
	d.DisallowUnknownFields()
	if err := d.Decode(&m); err != nil {
		return m, fmt.Errorf("not a UI message: %w", err)
	}
	if d.More() {
		return m, errors.New("not a UI message: more after it")
	}
	if m.Version != 1 {
		return m, fmt.Errorf("UI message version %d, this device reads 1", m.Version)
	}
	long := func(what, s string, n int) error {
		if utf8.RuneCountInString(s) > n {
			return fmt.Errorf("%s is longer than %d characters", what, n)
		}
		return nil
	}
	if err := long("request_id", m.RequestID, 64); err != nil {
		return m, err
	}
	switch m.Kind {
	case KindLookup:
		if m.Lookup == nil {
			return m, errors.New("a lookup message with no lookup")
		}
		if err := errors.Join(long("the title", m.Lookup.Title, 80)); err != nil {
			return m, err
		}
		if len(m.Lookup.Rows) > 30 {
			return m, errors.New("more than 30 rows")
		}
		for _, r := range m.Lookup.Rows {
			if r.Label == "" {
				return m, errors.New("a row with no label")
			}
			if err := errors.Join(long("a row's label", r.Label, 120), long("a row's value", r.Value, 120)); err != nil {
				return m, err
			}
		}
	case KindCards:
		if len(m.Cards) > 6 {
			return m, errors.New("more than 6 cards")
		}
		for _, c := range m.Cards {
			if !cardID.MatchString(c.ID) {
				return m, fmt.Errorf("card id %q is not 1 to 32 of a-z, 0-9, _ and -", c.ID)
			}
			if c.Title == "" || c.Value == "" {
				return m, fmt.Errorf("card %s has no title or value", c.ID)
			}
			if err := errors.Join(long("a card's title", c.Title, 40), long("a card's value", c.Value, 60), long("a card's detail", c.Detail, 1000)); err != nil {
				return m, err
			}
		}
	default:
		return m, fmt.Errorf("UI message kind %q, this device reads lookup and cards", m.Kind)
	}
	return m, nil
}

// LookupOf is a lookup message's result panel, found at.
func (m Message) LookupOf(at time.Time) Lookup {
	r := Lookup{Title: m.Lookup.Title, At: at}
	for _, row := range m.Lookup.Rows {
		r.Rows = append(r.Rows, Row{Label: row.Label, Value: row.Value})
	}
	return r
}

// CardsOf is a cards message's cards.
func (m Message) CardsOf() []Card {
	var out []Card
	for _, c := range m.Cards {
		out = append(out, Card{ID: c.ID, Title: c.Title, Value: c.Value, Detail: c.Detail})
	}
	return out
}

// MergeCards is the device's own cards with a backend's laid over them: one with the same ID takes
// that card's place, a new one goes after.
func MergeCards(own, sent []Card) []Card {
	out := append([]Card(nil), own...)
	for _, c := range sent {
		replaced := false
		for i := range out {
			if out[i].ID == c.ID {
				out[i], replaced = c, true
				break
			}
		}
		if !replaced {
			out = append(out, c)
		}
	}
	return out
}
