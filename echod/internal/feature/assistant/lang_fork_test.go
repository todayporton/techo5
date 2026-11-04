package assistant

// fork: lang_fork.go.

import (
	"strings"
	"testing"
	"time"

	"github.com/HuskerMinion/techo5/echod/internal/config"
)

func TestTheModelIsToldTheVoicesLanguage(t *testing.T) {
	if got := instructions(config.Brain{Language: "ja"}, time.Now()); !strings.Contains(got, "Japanese (日本語)") {
		t.Errorf("no Japanese in the instructions:\n%s", got)
	}
	for _, lang := range []string{"", "en", "en-US"} {
		if got := languageLine(config.Brain{Language: lang}); got != "" {
			t.Errorf("%q: %q, want upstream's instructions untouched", lang, got)
		}
	}
	if got := languageLine(config.Brain{Language: "de"}); !strings.Contains(got, "code de") {
		t.Errorf("de: %q", got)
	}
	// The owner's own prompt still comes last, after the language.
	got := instructions(config.Brain{Language: "ja", Prompt: "関西弁で話して"}, time.Now())
	if strings.Index(got, "関西弁") < strings.Index(got, "Japanese") {
		t.Error("the owner's prompt is not last")
	}
}

func TestStuckWordsInJapanese(t *testing.T) {
	if got := stuckWords(config.Brain{Language: "ja"}); !strings.Contains(got, "すみません") {
		t.Errorf("ja: %q", got)
	}
	if got := stuckWords(config.Brain{}); !strings.HasPrefix(got, "Sorry") {
		t.Errorf("en: %q", got)
	}
}

func TestJapaneseDurationsAreSpelledOutInSeconds(t *testing.T) {
	if got := languageLine(config.Brain{Language: "ja"}); !strings.Contains(got, "3分 is 180 seconds") {
		t.Errorf("ja: %q", got)
	}
	if got := languageLine(config.Brain{Language: "de"}); strings.Contains(got, "180") {
		t.Errorf("de got the Japanese units: %q", got)
	}
}
