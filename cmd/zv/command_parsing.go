package main

import (
	"strings"
	"unicode"
)

func splitCommandFields(line string) ([]string, bool) {
	var fields []string
	var field strings.Builder
	var quote rune
	inField := false

	for _, r := range line {
		if quote != 0 {
			if r == quote {
				quote = 0
				inField = true
				continue
			}
			field.WriteRune(r)
			inField = true
			continue
		}

		switch {
		case r == '"' || r == '\'':
			quote = r
			inField = true
		case unicode.IsSpace(r):
			if inField {
				fields = append(fields, field.String())
				field.Reset()
				inField = false
			}
		default:
			field.WriteRune(r)
			inField = true
		}
	}
	if quote != 0 {
		return nil, false
	}
	if inField {
		fields = append(fields, field.String())
	}
	return fields, true
}
