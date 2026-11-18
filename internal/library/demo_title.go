package library

import (
	"errors"
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"
)

// MaxDemoTitleRunes keeps a demo title on one line of a Crear card.
const MaxDemoTitleRunes = 80

// CleanDemoTitle trims a demo title and refuses a line break or more than
// MaxDemoTitleRunes characters. An empty result clears the title.
func CleanDemoTitle(raw string) (string, error) {
	title := strings.TrimSpace(raw)
	if strings.ContainsFunc(title, func(r rune) bool { return unicode.IsControl(r) || r == '\u2028' || r == '\u2029' }) {
		return "", errors.New("title must be one line")
	}
	if utf8.RuneCountInString(title) > MaxDemoTitleRunes {
		return "", fmt.Errorf("title takes at most %d characters", MaxDemoTitleRunes)
	}
	return title, nil
}
