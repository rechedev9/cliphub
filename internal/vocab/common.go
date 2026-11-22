package vocab

import (
	_ "embed"
	"strings"
)

//go:embed common_words.txt
var commonWords string

var common = func() map[string]bool {
	out := map[string]bool{}
	for _, w := range strings.Fields(commonWords) {
		out[normalize(w)] = true
	}
	return out
}()
