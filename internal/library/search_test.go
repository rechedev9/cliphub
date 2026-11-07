package library

import (
	"reflect"
	"testing"
)

func TestParseSearchNarrowsMapAndGrenade(t *testing.T) {
	for raw, want := range map[string]SearchQuery{
		"Humo ventana MIRAGE":      {Words: []string{"ventana"}, Map: "de_mirage", Grenade: "smoke"},
		"de_nuke  señuelo":         {Map: "de_nuke", Grenade: "decoy"},
		"Rotación & lento":         {Words: []string{"rotacion", "y", "lento"}},
		"mirage inferno":           {Words: []string{"inferno"}, Map: "de_mirage"},
		"   ":                      {},
		"Équipe Vitality":          {Words: []string{"equipe", "vitality"}},
		"molotov molotov bancos":   {Words: []string{"molotov", "bancos"}, Grenade: "molotov"},
		"cegadora a B por palacio": {Words: []string{"a", "b", "por", "palacio"}, Grenade: "flash"},
	} {
		got := ParseSearch(raw)
		if got = (SearchQuery{Words: got.Words, Map: got.Map, Grenade: got.Grenade}); !reflect.DeepEqual(got, want) {
			t.Errorf("ParseSearch(%q) = %+v, want %+v", raw, got, want)
		}
	}
}

func TestParseSearchKeepsEachWordAsTyped(t *testing.T) {
	q := ParseSearch("humo Peña  Ñu")
	if !reflect.DeepEqual(q.Words, []string{"pena", "nu"}) || !reflect.DeepEqual(q.Typed, []string{"peña", "ñu"}) {
		t.Fatalf("ParseSearch = %+v, want folded words and the typed ones beside them", q)
	}
}

func TestSearchHoldsEveryWordAcrossFields(t *testing.T) {
	q := ParseSearch("rotacion y b")
	if !q.Holds("Rotación", "a B & mid") {
		t.Error("accents and & should fold")
	}
	if q.Holds("Rotación", "a mid") {
		t.Error("a missing word should not hold")
	}
	if !ParseSearch("mirage").Holds("anything") || ParseSearch("").Empty() != true {
		t.Error("a query of only a map holds every record; an empty one is empty")
	}
}
