package main

import (
	"strings"
	"unicode"
)

// maxNameRunes caps a name in tool output.
const maxNameRunes = 64

// cleanName keeps a player-chosen string printable and short before it reaches the model.
func cleanName(s string) string {
	var b strings.Builder
	n := 0
	for _, r := range s {
		if n == maxNameRunes {
			b.WriteRune('…')
			break
		}
		if unicode.IsControl(r) || unicode.Is(unicode.Cf, r) {
			continue
		}
		b.WriteRune(r)
		n++
	}
	return strings.TrimSpace(b.String())
}

// quotedName puts an untrusted name in «» for approval text, without the quote
// characters that could close the quote early; empty falls back.
func quotedName(raw, fallback string) string {
	name := strings.Map(func(r rune) rune {
		if strings.ContainsRune("«»‹›\"'`“”„‘’‚", r) {
			return -1
		}
		return r
	}, raw)
	if name = cleanName(name); name != "" {
		return "«" + name + "»"
	}
	return fallback
}

func cleanNames(names []string) []string {
	out := make([]string, 0, len(names))
	for _, n := range names {
		out = append(out, cleanName(n))
	}
	return out
}
