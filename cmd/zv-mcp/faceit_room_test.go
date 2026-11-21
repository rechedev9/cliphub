package main

import (
	"testing"

	"github.com/rechedev9/cliphub/internal/tuiclient"
)

func TestDescribeFaceitRoomKeepsTheRoomNumbersAndDropsUnsafeLinks(t *testing.T) {
	elo := 2140
	board := tuiclient.FaceitScoreboard{RoomURL: "javascript:alert(1)", Rounds: 24, Teams: []tuiclient.FaceitScoreboardTeam{{
		Name: "team_x", Score: 13, Won: true, AverageELO: &elo,
		Players: []tuiclient.FaceitScoreboardPlayer{
			{Nickname: "rsn", SteamID64: "76561198000000001", SkillLevel: 10, ELO: &elo, Kills: 25, HSPct: 48.6, ADR: 91.27},
			{Nickname: "x", SteamID64: "not-a-steamid"},
		},
	}}}
	out := describeFaceitRoom(board)
	if out.RoomURL != "" {
		t.Errorf("room_url = %q, want a non-FACEIT link dropped", out.RoomURL)
	}
	p := out.Teams[0].Players
	if p[0].SteamID64 != "76561198000000001" || p[0].ELO != 2140 || p[0].HSPercent != 49 || p[0].ADR != 91.3 {
		t.Errorf("first player = %+v", p[0])
	}
	if p[1].SteamID64 != "" || out.Teams[0].AverageELO != 2140 {
		t.Errorf("second player = %+v, team elo %d", p[1], out.Teams[0].AverageELO)
	}
	board.RoomURL = "https://www.faceit.com/en/cs2/room/1-abc"
	if got := describeFaceitRoom(board).RoomURL; got != board.RoomURL {
		t.Errorf("room_url = %q", got)
	}
}
