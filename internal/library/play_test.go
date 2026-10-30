package library

import (
	"math"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/rechedev9/cliphub/internal/tacticalplan"
)

func validPlay() Play {
	return Play{
		Title: "  Ejecución   A  ", Map: " DE_Mirage ", Side: "t", Kind: "execute", Site: "A", Origin: PlayOriginRound,
		Grenades:  []tacticalplan.PlayGrenade{{Kind: tacticalplan.EventSmoke, Seconds: 15, Pos: [3]float64{1, 2, 3}, Callout: " CT "}},
		Positions: []tacticalplan.PlayPosition{{SteamID64: "76561198000000001", Pos: [3]float64{4, 5, 6}}},
		Examples:  []tacticalplan.Evidence{{MatchID: uuid.New(), Round: 3, Tick: 640}},
	}
}

func TestCleanPlayNormalizesAndFillsLists(t *testing.T) {
	p, err := CleanPlay(Play{Title: "B", Map: "de_nuke", Side: "CT", Kind: "hold", Origin: PlayOriginImport})
	if err != nil {
		t.Fatal(err)
	}
	if p.Site != "none" || p.Grenades == nil || p.Positions == nil || p.Examples == nil {
		t.Fatalf("clean play = %+v, want site none and empty lists", p)
	}
	p, err = CleanPlay(validPlay())
	if err != nil {
		t.Fatal(err)
	}
	if p.Title != "Ejecución A" || p.Map != "de_mirage" || p.Side != "T" || p.Grenades[0].Callout != "CT" {
		t.Fatalf("clean play = %+v", p)
	}
}

func TestCleanPlayRejectsOutOfVocabularyFields(t *testing.T) {
	cases := map[string]func(p *Play){
		"empty title":      func(p *Play) { p.Title = " " },
		"long notes":       func(p *Play) { p.Notes = strings.Repeat("x", MaxPlayNotesRunes+1) },
		"bad map":          func(p *Play) { p.Map = "../x" },
		"bad side":         func(p *Play) { p.Side = "spec" },
		"CT kind on T":     func(p *Play) { p.Kind = "retake" },
		"bad site":         func(p *Play) { p.Site = "C" },
		"bad origin":       func(p *Play) { p.Origin = "" },
		"decoy":            func(p *Play) { p.Grenades[0].Kind = tacticalplan.EventDecoy },
		"NaN grenade":      func(p *Play) { p.Grenades[0].Pos[0] = math.NaN() },
		"share over one":   func(p *Play) { p.Grenades[0].Share = 2 },
		"bad steam id":     func(p *Play) { p.Positions[0].SteamID64 = "abc" },
		"round zero":       func(p *Play) { p.Examples[0].Round = 0 },
		"too many players": func(p *Play) { p.Positions = make([]tacticalplan.PlayPosition, MaxPlayPositions+1) },
	}
	for name, mutate := range cases {
		p := validPlay()
		mutate(&p)
		if _, err := CleanPlay(p); err == nil {
			t.Errorf("%s: CleanPlay = nil error", name)
		}
	}
}
