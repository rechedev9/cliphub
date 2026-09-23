package httpapi

import (
	"fmt"
	"testing"

	"github.com/rechedev9/cliphub/internal/faceit"
	"github.com/rechedev9/cliphub/internal/parser"
)

func scoreboardFixture(steamIDs ...string) faceit.Scoreboard {
	team := faceit.ScoreboardTeam{Name: "team_a"}
	for i, id := range steamIDs {
		team.Players = append(team.Players, faceit.ScoreboardPlayer{PlayerID: fmt.Sprintf("p%d", i), SteamID64: id, ADR: 86.6, Assists: 5})
	}
	return faceit.Scoreboard{Teams: []faceit.ScoreboardTeam{team}}
}

func TestMergeDemoIntoScoreboardUsesTheDemoADR(t *testing.T) {
	t.Parallel()
	ids := []string{"1", "2", "3", "4", "5", "6", "7", "8", "9", "10"}
	board := scoreboardFixture(ids...)
	roster := parser.RosterResult{}
	for _, id := range ids {
		roster.Players = append(roster.Players, parser.PlayerStat{SteamID64: id, Rounds: 20, ADR: 86.0})
	}
	// NAVEMONSTA in match 1-2009a7d4: the Data API says 5 assists, the room 4,
	// and the demo credits one of them for a flash.
	roster.Players[0].FlashAssists = 1
	got, ok := mergeDemoIntoScoreboard(board, roster)
	if !ok {
		t.Fatal("same roster rejected")
	}
	if got.Teams[0].Players[0].ADR != 86.0 {
		t.Fatalf("ADR = %v, want the demo's 86.0 (the room's DPR)", got.Teams[0].Players[0].ADR)
	}
	if got.Teams[0].Players[0].Assists != 4 || got.Teams[0].Players[1].Assists != 5 {
		t.Fatalf("assists = %d, %d; want flash assists removed (4) and untouched without them (5)",
			got.Teams[0].Players[0].Assists, got.Teams[0].Players[1].Assists)
	}
	if board.Teams[0].Players[0].ADR != 86.6 || board.Teams[0].Players[0].Assists != 5 {
		t.Fatal("merge mutated the cached scoreboard")
	}
}

func TestMergeDemoIntoScoreboardRejectsAnotherMatch(t *testing.T) {
	t.Parallel()
	board := scoreboardFixture("1", "2", "3", "4", "5", "6", "7", "8", "9", "10")
	roster := parser.RosterResult{}
	for _, id := range []string{"1", "2", "3", "4", "5", "6", "7", "11", "12", "13"} {
		roster.Players = append(roster.Players, parser.PlayerStat{SteamID64: id, Rounds: 20})
	}
	if _, ok := mergeDemoIntoScoreboard(board, roster); ok {
		t.Fatal("a roster sharing 7 of 10 players was accepted as the same match")
	}
	roster.Players = roster.Players[:8]
	roster.Players[7].SteamID64 = "8"
	if _, ok := mergeDemoIntoScoreboard(board, roster); !ok {
		t.Fatal("a roster missing two players (substitute or disconnect) was rejected")
	}
}
