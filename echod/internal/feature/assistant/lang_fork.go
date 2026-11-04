package assistant

// fork: answering in the language the voice is set to (brain.language), for a Japanese assistant
// (docs/native-ui.md). Upstream's instructions are English, and a model told nothing of the language
// tends to answer in English whatever it was asked in.

import (
	"strings"

	"github.com/HuskerMinion/techo5/echod/internal/config"
)

// languageNames are the languages named to the model in words, for the codes it might be unsure of.
var languageNames = map[string]string{"ja": "Japanese (日本語)"}

// languageLine tells the model which language to answer in, when the voice is set to one other than
// English; empty otherwise, which leaves upstream's instructions exactly as they were.
func languageLine(b config.Brain) string {
	code := strings.ToLower(strings.TrimSpace(b.Language))
	if code == "" || strings.HasPrefix(code, "en") {
		return ""
	}
	name := languageNames[strings.SplitN(code, "-", 2)[0]]
	if name == "" {
		name = "the language with code " + code
	}
	line := " Always answer in " + name + ", whatever language these instructions are in, the way it is " +
		"naturally spoken: numbers, times and dates as they are said in it."
	if strings.HasPrefix(code, "ja") {
		// Asked for 3分のタイマー, a model sent seconds: 3. The tools take seconds; say so with the units it hears.
		line += " Tools take durations in seconds: 3分 is 180 seconds, 1時間半 is 5400 seconds, 30秒 is 30 seconds."
	}
	return line
}

// stuckWords is what is said when the model ran out of rounds and said nothing, in the voice's language.
func stuckWords(b config.Brain) string {
	if strings.HasPrefix(strings.ToLower(strings.TrimSpace(b.Language)), "ja") {
		return "すみません、うまく答えられませんでした。"
	}
	return "Sorry, I got stuck on that one."
}
