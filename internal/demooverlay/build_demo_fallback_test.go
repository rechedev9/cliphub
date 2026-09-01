package demooverlay

import (
	"strings"
	"testing"
)

func TestDemoDerivedLast20FromMatchStats(t *testing.T) {
	roster := Roster{
		ScoreCT: 13,
		ScoreT:  7,
		Rounds:  20,
		Players: []RosterPlayer{
			{SteamID64: "1", Name: "donk", Team: "CT", Kills: 23, Deaths: 14, Assists: 4, ADR: 101.6, Rounds: 20, HSPct: 52, Headshots: 12},
		},
	}
	doc := BuildForSource(roster, SourceFACEIT, nil)
	card := doc.Intro.Left[0]
	if card.Last20 == nil {
		t.Fatal("expected demo-derived last20")
	}
	if card.Last20.Matches == nil || *card.Last20.Matches != 1 {
		t.Fatalf("matches = %#v, want 1", card.Last20.Matches)
	}
	if card.Last20.WinPct == nil || *card.Last20.WinPct != 100 {
		t.Fatalf("win pct = %#v, want 100", card.Last20.WinPct)
	}
	if card.Last20.ADR == nil || *card.Last20.ADR != 101.6 {
		t.Fatalf("adr = %#v", card.Last20.ADR)
	}
	got := introFilter(doc, "/fonts/Montserrat-ExtraBold.ttf", false)
	for _, want := range []string{"This match", "Matches", "Win rate", "ADR-HS%", "1,64 / 1,15"} {
		if !strings.Contains(got, want) {
			t.Fatalf("intro missing %q:\n%s", want, got)
		}
	}
}

func TestDemoDerivedLast20NotInventedForPremier(t *testing.T) {
	roster := Roster{
		ScoreCT: 13,
		ScoreT:  7,
		Players: []RosterPlayer{
			{SteamID64: "1", Name: "donk", Team: "CT", Kills: 23, Deaths: 14, ADR: 101.6},
		},
	}
	doc := BuildForSource(roster, SourcePremier, nil)
	if doc.Intro.Left[0].Last20 != nil {
		t.Fatalf("premier leaked last20: %#v", doc.Intro.Left[0].Last20)
	}
}
