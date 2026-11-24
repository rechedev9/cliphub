package overlaylang

import (
	"fmt"
	"strings"
)

type Language string

const (
	Spanish Language = ""
	English Language = "en"
)

const spanishCode = "es"

func Codes() []string {
	return []string{spanishCode, string(English)}
}

func Parse(code string) (Language, error) {
	switch strings.ToLower(strings.TrimSpace(code)) {
	case "", spanishCode:
		return Spanish, nil
	case string(English):
		return English, nil
	}
	return Spanish, fmt.Errorf("unknown overlay language %q: use %s", code, strings.Join(Codes(), " or "))
}

func (l Language) String() string {
	if l == Spanish {
		return spanishCode
	}
	return string(l)
}

func (l *Language) UnmarshalText(text []byte) error {
	parsed, err := Parse(string(text))
	if err != nil {
		return err
	}
	*l = parsed
	return nil
}
