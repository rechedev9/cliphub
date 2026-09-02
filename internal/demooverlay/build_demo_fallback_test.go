package demooverlay

import "testing"

func TestBuildForSourceDoesNotInventFACEITData(t *testing.T) {
	t.Parallel()
	for _, source := range []string{SourceFACEIT, SourcePremier} {
		source := source
		t.Run(source, func(t *testing.T) {
			t.Parallel()
			roster := Roster{
				ScoreCT: 13,
				ScoreT:  7,
				Rounds:  20,
				Players: []RosterPlayer{{
					SteamID64: "1", Name: "donk", Team: "CT", Kills: 23,
					Deaths: 14, Assists: 4, ADR: 101.6, Rounds: 20,
				}},
			}
			card := BuildForSource(roster, source, nil).Intro.Left[0]
			if card.Last20 != nil || card.ELO != nil || card.SkillLevel != nil {
				t.Fatalf("invented FACEIT fields: %+v", card)
			}
		})
	}
}
