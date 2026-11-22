package vocab

import (
	_ "embed"
	"strings"
)

//go:embed lexicon.txt
var lexiconText string

const everyMap = "all"

var lexicon = func() map[string][]Term {
	out := map[string][]Term{}
	section := everyMap
	for _, line := range strings.Split(lexiconText, "\n") {
		line = strings.TrimSpace(line)
		switch {
		case line == "":
		case strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]"):
			section = strings.ToLower(strings.Trim(line, "[]"))
		default:
			out[section] = append(out[section], Term{Text: line})
		}
	}
	return out
}()

func Lexicon(mapName string) []Term {
	out := append([]Term{}, lexicon[everyMap]...)
	if name := strings.ToLower(mapName); name != "" && name != everyMap {
		out = append(out, lexicon[name]...)
	}
	return out
}
