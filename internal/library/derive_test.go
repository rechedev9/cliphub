package library

import (
	"testing"

	"github.com/google/uuid"

	"github.com/rechedev9/cliphub/internal/tacticalplan"
)

func TestDetectSource(t *testing.T) {
	cases := []struct {
		server, file string
		want         Source
	}{
		{"FACEIT.com register to play here", "whatever.dem", SourceFaceit},
		{"Valve Counter-Strike 2 eu_west Server", "", SourceValve},
		{"BLAST.tv Austin Major 2025", "", SourceHLTV},
		{"Perfect World Shanghai", "", SourceHLTV},
		{"Bestial ESL Server", "", SourceHLTV},
		{"", "1-9be6d5a6-abda-4110-908d-deae67dee533-1-1.dem", SourceFaceit},
		{"", "match730_003712345678901234567_1234567890_123.dem", SourceValve},
		{"", "vitality-vs-mouz-m2-mirage.dem", SourceHLTV},
		// "pgl" inside a word is not the organizer, and "vs" without a map
		// number is not an HLTV archive name.
		{"Mypglserver", "nuke-vs-pugs.dem", SourceUnknown},
		{"", "", SourceUnknown},
	}
	for _, tc := range cases {
		if got := DetectSource(tc.server, tc.file); got != tc.want {
			t.Errorf("DetectSource(%q, %q) = %q, want %q", tc.server, tc.file, got, tc.want)
		}
	}
}

func TestNormalizeName(t *testing.T) {
	for in, want := range map[string]string{
		"  Team   Vitality ": "vitality",
		"VITALITY":           "vitality",
		"team_apEX":          "team_apex",
		"Team":               "team",
		"Natus  Vincere":     "natus vincere",
	} {
		if got := NormalizeName(in); got != want {
			t.Errorf("NormalizeName(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestNormalizeTagsDedupesSortsAndCaps(t *testing.T) {
	long := "abcdefghijabcdefghijabcdefghijabcdefghijXYZ"
	got := NormalizeTags([]string{"b", " ", "Scrim", "scrim ", "A  b", long})
	want := []string{"A b", "abcdefghijabcdefghijabcdefghijabcdefghij", "b", "Scrim"}
	if len(got) != len(want) {
		t.Fatalf("NormalizeTags = %q, want %q", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("NormalizeTags = %q, want %q", got, want)
		}
	}
	many := make([]string, 0, 40)
	for i := range 40 {
		many = append(many, string(rune('a'+i%26))+string(rune('a'+i/26)))
	}
	if n := len(NormalizeTags(many)); n != maxTags {
		t.Fatalf("NormalizeTags kept %d tags, want %d", n, maxTags)
	}
}

func TestAnalysisFromDocumentSumsRoundsPerTeamAndPlayer(t *testing.T) {
	doc := tacticalplan.NewDocument()
	doc.Demo = tacticalplan.Demo{Map: "de_mirage", Tickrate: 64, DurationTicks: 9000}
	doc.Teams = []tacticalplan.Team{
		{Key: "t-start", Name: "T start", StartSide: tacticalplan.SideT},
		{Key: "ct-start", Name: "  Team  Vitality ", StartSide: tacticalplan.SideCT},
	}
	doc.Players = []tacticalplan.Player{
		{Slot: 1, SteamID64: "76561198000000002", Name: "ropz", TeamKey: "t-start"},
		{Slot: 0, SteamID64: "76561198000000001", Name: "ZywOo", TeamKey: "ct-start"},
		{Slot: 2, SteamID64: "0", Name: "BOT Kevin", TeamKey: "t-start"},
	}
	// Round 13 is in the second half, where the ct-start team plays T.
	doc.Rounds = []tacticalplan.Round{
		{Number: 1, Half: 1, Winner: tacticalplan.SideCT, Players: []tacticalplan.PlayerRound{
			{Slot: 0, Kills: 2, Damage: 180}, {Slot: 1, Deaths: 1}, {Slot: 2, Kills: 9},
		}},
		{Number: 2, Half: 1, Winner: tacticalplan.SideT, Players: []tacticalplan.PlayerRound{
			{Slot: 0, Deaths: 1}, {Slot: 1, Kills: 1, Assists: 1, Damage: 100},
		}},
		{Number: 13, Half: 2, Winner: tacticalplan.SideT, Players: []tacticalplan.PlayerRound{
			{Slot: 0, Kills: 1, Damage: 100},
		}},
		{Number: 14, Half: 2, Winner: ""},
	}
	id := uuid.New()
	got := AnalysisFromDocument(id, doc)
	if got.DemoID != id || got.Map != "de_mirage" || got.RoundCount != 4 || got.Tickrate != 64 || got.DurationTicks != 9000 {
		t.Fatalf("analysis header = %+v", got)
	}
	if len(got.Teams) != 2 {
		t.Fatalf("teams = %+v", got.Teams)
	}
	ct, tside := got.Teams[0], got.Teams[1]
	if ct.Key != "ct-start" || ct.ClanName != "Team Vitality" || ct.Score != 2 || ct.StartSide != "CT" {
		t.Fatalf("ct-start = %+v, want Team Vitality with rounds 1 and 13", ct)
	}
	if tside.Key != "t-start" || tside.ClanName != "" || tside.Score != 1 {
		t.Fatalf("t-start = %+v, want no clan name and round 2", tside)
	}
	if len(got.Players) != 2 {
		t.Fatalf("players = %+v, want the two humans", got.Players)
	}
	zywoo, ropz := got.Players[0], got.Players[1]
	if zywoo.Name != "ZywOo" || zywoo.Rounds != 3 || zywoo.Kills != 3 || zywoo.Deaths != 1 || zywoo.Damage != 280 {
		t.Fatalf("ZywOo = %+v", zywoo)
	}
	if ropz.Rounds != 2 || ropz.Kills != 1 || ropz.Assists != 1 || ropz.Deaths != 1 || ropz.Damage != 100 {
		t.Fatalf("ropz = %+v", ropz)
	}
}
