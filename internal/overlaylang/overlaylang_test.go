package overlaylang

import (
	"encoding/json"
	"testing"
)

func TestParseKeepsSpanishTheZeroValue(t *testing.T) {
	t.Parallel()
	for code, want := range map[string]Language{"": Spanish, "es": Spanish, " ES ": Spanish, "en": English, "EN": English} {
		got, err := Parse(code)
		if err != nil || got != want {
			t.Errorf("Parse(%q) = %q, %v; want %q", code, got, err, want)
		}
	}
	if _, err := Parse("fr"); err == nil {
		t.Error("Parse accepted a language Studio does not write")
	}
	if Spanish.String() != "es" || English.String() != "en" {
		t.Errorf("codes = %s, %s; want es, en", Spanish, English)
	}
}

func TestSpanishStaysOffTheWire(t *testing.T) {
	t.Parallel()
	type doc struct {
		Language Language `json:"language,omitempty"`
	}
	for _, tc := range []struct {
		in   doc
		want string
	}{{doc{Spanish}, `{}`}, {doc{English}, `{"language":"en"}`}} {
		body, err := json.Marshal(tc.in)
		if err != nil || string(body) != tc.want {
			t.Errorf("Marshal(%+v) = %s, %v; want %s", tc.in, body, err, tc.want)
		}
	}
	var decoded doc
	if err := json.Unmarshal([]byte(`{"language":"es"}`), &decoded); err != nil || decoded.Language != Spanish {
		t.Errorf("decoded es = %q, %v; want Spanish", decoded.Language, err)
	}
	if err := json.Unmarshal([]byte(`{"language":"de"}`), &decoded); err == nil {
		t.Error("an unknown language decoded")
	}
}
