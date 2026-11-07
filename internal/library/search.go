package library

import (
	"strings"
	"unicode"

	"golang.org/x/text/unicode/norm"
)

// MaxSearchRunes bounds a search query.
const MaxSearchRunes = 200

// searchMaps are the map names a query word narrows to, without the de_ prefix.
var searchMaps = map[string]bool{
	"mirage": true, "inferno": true, "nuke": true, "ancient": true, "anubis": true, "dust2": true,
	"train": true, "overpass": true, "vertigo": true, "cache": true,
}

// searchGrenades are the grenade words a query narrows lineups to, in English and Spanish.
var searchGrenades = map[string]string{
	"smoke": "smoke", "smokes": "smoke", "humo": "smoke", "humos": "smoke",
	"flash": "flash", "flashes": "flash", "flashbang": "flash", "cegadora": "flash",
	"he": "he", "explosiva": "he",
	"molotov": "molotov", "molo": "molotov", "molly": "molotov", "incendiaria": "molotov",
	"decoy": "decoy", "senuelo": "decoy",
}

// SearchQuery is a parsed search: the words every result holds and the map
// and grenade the query narrows to.
type SearchQuery struct {
	Words []string
	// Typed is each of Words as typed, lowercased with its accents, for lookups that do not fold.
	Typed   []string
	Map     string
	Grenade string
}

// FoldSearch lowercases text, drops its accents and reads "&" as "y", so
// "Señuelo & Vitality" and "senuelo y vitality" compare equal.
func FoldSearch(text string) string {
	var b strings.Builder
	for _, r := range norm.NFD.String(strings.ToLower(text)) {
		switch {
		case unicode.Is(unicode.Mn, r):
		case r == '&':
			b.WriteString(" y ")
		default:
			b.WriteRune(r)
		}
	}
	return strings.Join(strings.Fields(b.String()), " ")
}

// ParseSearch splits a query into words, taking out a map name and a grenade.
func ParseSearch(raw string) SearchQuery {
	var q SearchQuery
	for _, field := range strings.Fields(strings.ToLower(raw)) {
		for _, word := range strings.Fields(FoldSearch(field)) {
			name := strings.TrimPrefix(word, "de_")
			switch {
			case q.Map == "" && searchMaps[name]:
				q.Map = "de_" + name
			case q.Grenade == "" && searchGrenades[word] != "":
				q.Grenade = searchGrenades[word]
			default:
				q.Words, q.Typed = append(q.Words, word), append(q.Typed, field)
			}
		}
	}
	return q
}

// Empty reports whether the query has nothing to look for.
func (q SearchQuery) Empty() bool {
	return len(q.Words) == 0 && q.Map == "" && q.Grenade == ""
}

// Narrowed reports whether the query names a map or a grenade, which records
// with neither, such as calls and teams, never match.
func (q SearchQuery) Narrowed() bool {
	return q.Map != "" || q.Grenade != ""
}

// Holds reports whether every word of the query is in one of the fields.
func (q SearchQuery) Holds(fields ...string) bool {
	text := FoldSearch(strings.Join(fields, " "))
	for _, word := range q.Words {
		if !strings.Contains(text, word) {
			return false
		}
	}
	return true
}
