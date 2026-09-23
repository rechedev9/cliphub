package faceit

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestMatchIDFromDemoFileName(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		want string
		ok   bool
	}{
		{"1-2009a7d4-6c68-46e1-aebc-46629bd92649-1-1.dem", "1-2009a7d4-6c68-46e1-aebc-46629bd92649", true},
		{"1-2009A7D4-6C68-46E1-AEBC-46629BD92649-1-1.dem.zst", "1-2009a7d4-6c68-46e1-aebc-46629bd92649", true},
		{"1-2009a7d4-6c68-46e1-aebc-46629bd92649.dem", "1-2009a7d4-6c68-46e1-aebc-46629bd92649", true},
		{"spirit-vs-furia-m2-cache.dem", "", false},
		{"copy of 1-2009a7d4-6c68-46e1-aebc-46629bd92649-1-1.dem", "", false},
		{"", "", false},
	}
	for _, test := range tests {
		got, ok := MatchIDFromDemoFileName(test.name)
		if got != test.want || ok != test.ok {
			t.Errorf("MatchIDFromDemoFileName(%q) = %q, %v; want %q, %v", test.name, got, ok, test.want, test.ok)
		}
	}
}

// Values from match 1-2009a7d4-6c68-46e1-aebc-46629bd92649 (Mirage 13-7), the
// demo whose picker disagreed with the FACEIT room.
const scoreboardStatsBody = `{"rounds":[{"round_stats":{"Map":"de_mirage","Rounds":"20","Score":"13 / 7"},"teams":[
 {"team_id":"f2","team_stats":{"Team":"team_kiy0o","Final Score":"7","First Half Score":"5","Second Half Score":"2","Overtime score":"0","Team Win":"0"},
  "players":[{"player_id":"p-kiy0o","nickname":"kiy0o","player_stats":{"Kills":"17","Deaths":"16","Assists":"2","Headshots":"12","Headshots %":"71","ADR":"92","K/D Ratio":"1.06","K/R Ratio":"0.85","MVPs":"2","Double Kills":"2","Triple Kills":"0","Quadro Kills":"0","Penta Kills":"0"}}]},
 {"team_id":"f1","team_stats":{"Team":"team_Magnojezzz","Final Score":"13","First Half Score":"7","Second Half Score":"6","Overtime score":"0","Team Win":"1"},
  "players":[{"player_id":"p-magno","nickname":"Magnojezzz","player_stats":{"Kills":"14","Deaths":"14","Assists":"10","Headshots":"11","Headshots %":"79","ADR":"86.6","K/D Ratio":"1","K/R Ratio":"0.7","MVPs":"4","Double Kills":"2","Triple Kills":"1","Quadro Kills":"0","Penta Kills":"0"}}]}
]}]}`

const scoreboardDetailsBody = `{"teams":{
 "faction1":{"roster":[{"player_id":"p-magno","avatar":"https://distribution.faceit-cdn.net/images/magno.jpeg","game_player_id":"76561198372023214","game_skill_level":10}]},
 "faction2":{"roster":[{"player_id":"p-kiy0o","avatar":"","game_player_id":"76561199017580923","game_skill_level":9}]}}}`

func TestMatchScoreboardMirrorsTheRoom(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/matches/1-m/stats":
			_, _ = w.Write([]byte(scoreboardStatsBody))
		case "/matches/1-m":
			_, _ = w.Write([]byte(scoreboardDetailsBody))
		case "/players/p-magno":
			_, _ = w.Write([]byte(`{"player_id":"p-magno","games":{"cs2":{"skill_level":10,"faceit_elo":4271}}}`))
		case "/players/p-kiy0o":
			// A failed ELO lookup keeps the scoreboard; only the ELO is missing.
			w.WriteHeader(http.StatusInternalServerError)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	client, err := New(Options{APIKey: "faceit-test-key", BaseURL: server.URL, HTTPClient: server.Client()})
	if err != nil {
		t.Fatal(err)
	}
	board, err := client.MatchScoreboard(context.Background(), "1-m", "de_mirage")
	if err != nil {
		t.Fatal(err)
	}
	if board.Rounds != 20 || len(board.Teams) != 2 || board.RoomURL == "" {
		t.Fatalf("board = %+v", board)
	}
	winner, loser := board.Teams[0], board.Teams[1]
	if winner.Name != "team_Magnojezzz" || !winner.Won || winner.Score != 13 || winner.FirstHalf != 7 || winner.SecondHalf != 6 {
		t.Fatalf("winner not listed first with its halves: %+v", winner)
	}
	magno := winner.Players[0]
	if magno.HSPct != 78.6 || magno.KD != 1 || magno.KR != 0.7 || magno.Triple != 1 || magno.Double != 2 || magno.MVPs != 4 {
		t.Fatalf("Magnojezzz stats = %+v", magno)
	}
	if magno.SteamID64 != "76561198372023214" || magno.ELO == nil || *magno.ELO != 4271 || winner.AverageELO == nil || *winner.AverageELO != 4271 {
		t.Fatalf("Magnojezzz identity = %+v, team avg %v", magno, winner.AverageELO)
	}
	kiy0o := loser.Players[0]
	if kiy0o.ELO != nil || loser.AverageELO != nil || kiy0o.SkillLevel != 9 || kiy0o.HSPct != 70.6 || kiy0o.KR != 0.85 {
		t.Fatalf("kiy0o = %+v, team avg %v", kiy0o, loser.AverageELO)
	}
}

func TestMatchScoreboardUnknownMapOrMatch(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/matches/1-m/stats":
			_, _ = w.Write([]byte(scoreboardStatsBody))
		case "/matches/1-m":
			_, _ = w.Write([]byte(scoreboardDetailsBody))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	client, err := New(Options{APIKey: "faceit-test-key", BaseURL: server.URL, HTTPClient: server.Client()})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := client.MatchScoreboard(context.Background(), "1-m", "de_nuke"); !errors.Is(err, ErrMatchNotFound) {
		t.Fatalf("other map err = %v, want ErrMatchNotFound", err)
	}
	if _, err := client.MatchScoreboard(context.Background(), "1-gone", "de_mirage"); !errors.Is(err, ErrMatchNotFound) {
		t.Fatalf("unknown match err = %v, want ErrMatchNotFound", err)
	}
}
