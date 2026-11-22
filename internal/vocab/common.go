package vocab

import (
	_ "embed"
	"strings"
)

//go:embed common_words.txt
var commonWords string

//go:embed common_extra.txt
var commonExtra string

var common = func() map[string]bool {
	out := map[string]bool{}
	for _, w := range strings.Fields(commonWords + "\n" + commonExtra) {
		out[normalize(w)] = true
	}
	return out
}()
