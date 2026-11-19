package demooverlay

import (
	"path"
	"slices"
	"strings"
	"sync"
)

// notCountryFlags are the two-letter flag-icons files that name no ISO 3166-1
// country: the European Union, the United Nations, the unknown flag and the
// Pacific Community.
var notCountryFlags = []string{"eu", "un", "xx", "pc"}

var flagCountries = sync.OnceValue(func() []string {
	entries, err := neonAssets.ReadDir("assets/flags")
	if err != nil {
		panic("demooverlay: bundled flags are unreadable: " + err.Error())
	}
	var out []string
	for _, entry := range entries {
		code, ok := strings.CutSuffix(entry.Name(), ".svg")
		if ok && isFlagCode(code) && !slices.Contains(notCountryFlags, code) {
			out = append(out, code)
		}
	}
	slices.Sort(out)
	return out
})

// FlagCountries lists, sorted, the lowercase ISO 3166-1 codes with a bundled
// flag: the countries the overlays and the Jugadores FACEIT page can show.
func FlagCountries() []string {
	return slices.Clone(flagCountries())
}

// FlagSVG returns the bundled flag of a country FlagCountries lists.
func FlagSVG(code string) ([]byte, bool) {
	if _, found := slices.BinarySearch(flagCountries(), code); !found {
		return nil, false
	}
	data, err := neonAssets.ReadFile(path.Join("assets/flags", code+".svg"))
	return data, err == nil
}

func isFlagCode(code string) bool {
	return len(code) == 2 && code[0] >= 'a' && code[0] <= 'z' && code[1] >= 'a' && code[1] <= 'z'
}
