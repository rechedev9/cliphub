package demooverlay

import (
	"slices"
	"testing"
)

func TestBuildSplitsPOVTeamLeftAndOmitsEmptyFACEITColumns(t *testing.T) {
	doc := Build(Roster{
		TargetSteamID64: "1",
		Map:             "de_mirage",
		ScoreCT:         13,
		ScoreT:          8,
		Players: []RosterPlayer{
			{SteamID64: "1", Name: "donk666", Team: "CT", Kills: 23, Deaths: 14, Assists: 4, Headshots: 12, MVPs: 4, Rounds: 21, ADR: 101.6, HSPct: 52, Rating: 1.35, Rounds2K: 3, Rounds3K: 1},
			{SteamID64: "2", Name: "mate", Team: "CT", Kills: 16, Deaths: 15, Assists: 6, Rounds: 21, ADR: 80, Rating: 1.05},
			{SteamID64: "3", Name: "KingwayO", Team: "T", Kills: 18, Deaths: 16, Assists: 3, Rounds: 21, ADR: 90, Rating: 1.10},
		},
	}, nil)

	if doc.SchemaVersion != SchemaVersion {
		t.Fatalf("schema = %q", doc.SchemaVersion)
	}
	if doc.TargetName != "donk666" || doc.TargetKills != 23 || doc.TargetDeaths != 14 {
		t.Fatalf("target = %+v", doc)
	}
	if len(doc.Intro.Left) != 2 || doc.Intro.Left[0].Name != "donk666" {
		t.Fatalf("intro left = %+v", doc.Intro.Left)
	}
	if len(doc.Intro.Right) != 1 || doc.Intro.Right[0].Name != "KingwayO" {
		t.Fatalf("intro right = %+v", doc.Intro.Right)
	}
	if slices.Contains(doc.Intro.Columns, ColELO) || slices.Contains(doc.Intro.Columns, ColLevel) || slices.Contains(doc.Intro.Columns, ColSwing) {
		t.Fatalf("intro columns invented FACEIT fields: %v", doc.Intro.Columns)
	}
	if !slices.Contains(doc.Intro.Columns, ColName) || !slices.Contains(doc.Intro.Columns, ColKDA) {
		t.Fatalf("intro columns missing demo facts: %v", doc.Intro.Columns)
	}
	if slices.Contains(doc.Outro.Columns, ColELO) || slices.Contains(doc.Outro.Columns, ColSwing) {
		t.Fatalf("outro invented FACEIT columns: %v", doc.Outro.Columns)
	}
	if !slices.Contains(doc.Outro.Columns, ColRating) || !slices.Contains(doc.Outro.Columns, ColADR) {
		t.Fatalf("outro missing demo columns: %v", doc.Outro.Columns)
	}
	if len(doc.Outro.Teams) != 2 || doc.Outro.Teams[0].Score != 13 || doc.Outro.Teams[0].Side != "CT" {
		t.Fatalf("outro teams = %+v", doc.Outro.Teams)
	}
	if doc.Outro.Teams[0].AverageELO != nil {
		t.Fatalf("average ELO invented: %v", doc.Outro.Teams[0].AverageELO)
	}
}

func TestBuildMergesFACEITWithoutInventingMissingLast20(t *testing.T) {
	elo := 4370
	matches := 20
	win := 57.0
	kd := 1.46
	doc := Build(Roster{
		TargetSteamID64: "1",
		ScoreCT:         13,
		ScoreT:          8,
		Players: []RosterPlayer{
			{SteamID64: "1", Name: "donk666", Team: "CT", Kills: 23, Deaths: 14},
			{SteamID64: "9", Name: "enemy", Team: "T", Kills: 10, Deaths: 18},
		},
	}, map[string]Enrichment{
		"1": {
			Nickname:   "donk666",
			Country:    "ru",
			ELO:        elo,
			SkillLevel: 10,
			Last20:     &Last20{Matches: &matches, WinPct: &win, KD: &kd},
		},
	})
	if doc.TargetELO == nil || *doc.TargetELO != elo {
		t.Fatalf("target elo = %v", doc.TargetELO)
	}
	if !slices.Contains(doc.Intro.Columns, ColELO) || !slices.Contains(doc.Intro.Columns, ColMatches) {
		t.Fatalf("intro missing FACEIT columns: %v", doc.Intro.Columns)
	}
	if slices.Contains(doc.Intro.Columns, ColRating) || slices.Contains(doc.Intro.Columns, ColSwing) {
		t.Fatalf("intro invented last-20 rating/swing: %v", doc.Intro.Columns)
	}
	if doc.Intro.Left[0].Last20 == nil || doc.Intro.Left[0].Last20.Rating != nil {
		t.Fatalf("last20 rating invented: %+v", doc.Intro.Left[0].Last20)
	}
	if doc.Outro.Teams[0].AverageELO == nil || *doc.Outro.Teams[0].AverageELO != elo {
		t.Fatalf("CT average ELO = %v, want %d for a single enriched player", doc.Outro.Teams[0].AverageELO, elo)
	}
}

func TestBuildAverageELORequiresEveryTeammate(t *testing.T) {
	doc := Build(Roster{
		TargetSteamID64: "1",
		ScoreCT:         13,
		ScoreT:          8,
		Players: []RosterPlayer{
			{SteamID64: "1", Name: "a", Team: "CT", Kills: 10},
			{SteamID64: "2", Name: "b", Team: "CT", Kills: 8},
			{SteamID64: "3", Name: "c", Team: "T", Kills: 7},
		},
	}, map[string]Enrichment{
		"1": {ELO: 4000, SkillLevel: 10},
		"3": {ELO: 3000, SkillLevel: 9},
	})
	ct := doc.Outro.Teams[0]
	if ct.AverageELO != nil {
		t.Fatalf("CT average ELO = %d, want omitted because mate has no FACEIT ELO", *ct.AverageELO)
	}
}

func TestBuildZeroFACEITValuesAreOmitted(t *testing.T) {
	doc := Build(Roster{
		TargetSteamID64: "1",
		Players:         []RosterPlayer{{SteamID64: "1", Name: "a", Team: "CT"}},
	}, map[string]Enrichment{"1": {ELO: 0, SkillLevel: 0, Last20: &Last20{}}})
	if doc.Intro.Left[0].ELO != nil || doc.Intro.Left[0].SkillLevel != nil || doc.Intro.Left[0].Last20 != nil {
		t.Fatalf("zero FACEIT values leaked: %+v", doc.Intro.Left[0])
	}
	if slices.Contains(doc.Intro.Columns, ColELO) {
		t.Fatalf("elo column present: %v", doc.Intro.Columns)
	}
}

func TestBuildForSourceHonorsPremierProfessionalAndFACEIT(t *testing.T) {
	matches := 20
	win := 57.0
	kd := 1.46
	roster := Roster{
		TargetSteamID64: "1",
		Map:             "de_mirage",
		ScoreCT:         13,
		ScoreT:          8,
		Players: []RosterPlayer{
			{SteamID64: "1", Name: "donk666", Team: "CT", Kills: 23, Deaths: 14, Assists: 4, Headshots: 12, ADR: 101.6, HSPct: 52, Rating: 1.35, Rounds: 21},
			{SteamID64: "2", Name: "KingwayO", Team: "T", Kills: 18, Deaths: 16, Assists: 3, ADR: 90, Rating: 1.10, Rounds: 21},
		},
	}
	leaked := map[string]Enrichment{
		"1": {
			Nickname:   "faceit-donk",
			Country:    "ru",
			ELO:        4370,
			SkillLevel: 10,
			AvatarURL:  "https://assets.faceit.com/avatars/donk.png",
			Last20:     &Last20{Matches: &matches, WinPct: &win, KD: &kd},
		},
	}

	tests := []struct {
		name         string
		source       string
		wantSource   string
		wantName     string
		wantELO      bool
		wantLast20   bool
		wantAvatar   bool
		wantIntroELO bool
	}{
		{
			name:         "faceit keeps enrichment",
			source:       SourceFACEIT,
			wantSource:   SourceFACEIT,
			wantName:     "faceit-donk",
			wantELO:      true,
			wantLast20:   true,
			wantAvatar:   true,
			wantIntroELO: true,
		},
		{
			name:       "premier ignores leftover FACEIT map",
			source:     SourcePremier,
			wantSource: SourcePremier,
			wantName:   "donk666",
		},
		{
			name:       "professional ignores leftover FACEIT map",
			source:     SourceProfessional,
			wantSource: SourceProfessional,
			wantName:   "donk666",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			doc := BuildForSource(roster, tc.source, leaked)
			if doc.SchemaVersion != SchemaVersion {
				t.Fatalf("schema = %q", doc.SchemaVersion)
			}
			if doc.Source != tc.wantSource {
				t.Fatalf("source = %q, want %q", doc.Source, tc.wantSource)
			}
			if len(doc.Intro.Left) == 0 || len(doc.Outro.Teams) == 0 {
				t.Fatalf("overlay skipped roster: intro=%+v outro=%+v", doc.Intro, doc.Outro)
			}
			card := doc.Intro.Left[0]
			if card.Name != tc.wantName {
				t.Fatalf("name = %q, want %q", card.Name, tc.wantName)
			}
			if card.Kills != 23 || card.Deaths != 14 || card.Assists != 4 {
				t.Fatalf("demo K/D/A leaked: %+v", card)
			}
			if !card.HasADR || card.ADR != 101.6 {
				t.Fatalf("demo ADR missing: %+v", card)
			}
			if !card.HasRating || card.Rating != 1.35 {
				t.Fatalf("demo rating missing: %+v", card)
			}
			if tc.wantELO {
				if card.ELO == nil || *card.ELO != 4370 || card.SkillLevel == nil || *card.SkillLevel != 10 {
					t.Fatalf("FACEIT elo/level missing: %+v", card)
				}
			} else if card.ELO != nil || card.SkillLevel != nil {
				t.Fatalf("FACEIT elo/level leaked onto %s: %+v", tc.source, card)
			}
			if tc.wantLast20 {
				if card.Last20 == nil || card.Last20.Matches == nil || *card.Last20.Matches != 20 {
					t.Fatalf("FACEIT last-20 missing: %+v", card.Last20)
				}
			} else if card.Last20 != nil {
				t.Fatalf("FACEIT last-20 leaked onto %s: %+v", tc.source, card.Last20)
			}
			if tc.wantAvatar {
				if card.AvatarURL != "https://assets.faceit.com/avatars/donk.png" {
					t.Fatalf("avatar url = %q", card.AvatarURL)
				}
			} else if card.AvatarURL != "" {
				t.Fatalf("FACEIT avatar leaked onto %s: %q", tc.source, card.AvatarURL)
			}
			if tc.wantIntroELO != slices.Contains(doc.Intro.Columns, ColELO) {
				t.Fatalf("intro columns = %v, FACEIT elo present = %v", doc.Intro.Columns, slices.Contains(doc.Intro.Columns, ColELO))
			}
			if !tc.wantELO && slices.Contains(doc.Outro.Columns, ColELO) {
				t.Fatalf("outro invented FACEIT elo: %v", doc.Outro.Columns)
			}
			if !slices.Contains(doc.Intro.Columns, ColName) || !slices.Contains(doc.Intro.Columns, ColKDA) {
				t.Fatalf("intro missing demo columns: %v", doc.Intro.Columns)
			}
			if !slices.Contains(doc.Outro.Columns, ColADR) || !slices.Contains(doc.Outro.Columns, ColRating) {
				t.Fatalf("outro missing demo columns: %v", doc.Outro.Columns)
			}
		})
	}
}
