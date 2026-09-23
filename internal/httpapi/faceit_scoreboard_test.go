package httpapi

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/rechedev9/cliphub/internal/artifacts"
	"github.com/rechedev9/cliphub/internal/faceit"
	"github.com/rechedev9/cliphub/internal/job"
	"github.com/rechedev9/cliphub/internal/parser"
	"github.com/rechedev9/cliphub/internal/rules"
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

const scoreboardHandlerStats = `{"rounds":[{"round_stats":{"Map":"de_mirage","Rounds":"20"},"teams":[
 {"team_stats":{"Team":"team_a","Final Score":"13","Team Win":"1"},"players":[{"player_id":"pa","nickname":"a","player_stats":{"Kills":"14","Deaths":"14","Assists":"5","ADR":"86.6"}}]},
 {"team_stats":{"Team":"team_b","Final Score":"7","Team Win":"0"},"players":[{"player_id":"pb","nickname":"b","player_stats":{"Kills":"6","Deaths":"17","Assists":"2","ADR":"43.2"}}]}]}]}`

const scoreboardHandlerDetails = `{"teams":{"faction1":{"roster":[{"player_id":"pa","avatar":"https://distribution.faceit-cdn.net/images/a.jpeg","game_player_id":"76561198000000001","game_skill_level":10}]},
 "faction2":{"roster":[{"player_id":"pb","game_player_id":"76561198000000002","game_skill_level":9}]}}}`

func TestGetFaceitScoreboard(t *testing.T) {
	t.Parallel()
	const faceitFile = "1-2009a7d4-6c68-46e1-aebc-46629bd92649-1-1.dem"
	demoRoster := `{"match":{"map":"de_mirage"},"players":[
		{"steamid64":"76561198000000001","rounds":20,"adr":86.0,"assists":5,"flash_assists":1},
		{"steamid64":"76561198000000002","rounds":20,"adr":43.0,"assists":2}]}`
	otherRoster := `{"match":{"map":"de_mirage"},"players":[{"steamid64":"76561198000000009","rounds":20}]}`
	tests := []struct {
		name         string
		file         string
		roster       string
		upstream     int
		want         int
		code         string
		wantRequests int
	}{
		{name: "not a FACEIT file name", file: "spirit-vs-furia-m2-cache.dem", roster: demoRoster, upstream: http.StatusOK, want: http.StatusNotFound, code: notFaceitDemo, wantRequests: 0},
		{name: "FACEIT rejects the key", file: faceitFile, roster: demoRoster, upstream: http.StatusUnauthorized, want: http.StatusBadGateway, code: "faceit_unauthorized"},
		{name: "unknown match", file: faceitFile, roster: demoRoster, upstream: http.StatusNotFound, want: http.StatusNotFound, code: notFaceitDemo},
		{name: "another match's roster", file: faceitFile, roster: otherRoster, upstream: http.StatusOK, want: http.StatusNotFound, code: faceitMatchMismatch},
		// Two match calls plus one current-ELO lookup per player: 12 for a 5v5.
		{name: "scoreboard", file: faceitFile, roster: demoRoster, upstream: http.StatusOK, want: http.StatusOK, wantRequests: 4},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			var requests atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				requests.Add(1)
				if test.upstream != http.StatusOK {
					w.WriteHeader(test.upstream)
					return
				}
				switch {
				case strings.HasSuffix(r.URL.Path, "/stats"):
					_, _ = w.Write([]byte(scoreboardHandlerStats))
				case strings.HasPrefix(r.URL.Path, "/matches/"):
					_, _ = w.Write([]byte(scoreboardHandlerDetails))
				default:
					_, _ = w.Write([]byte(`{"games":{"cs2":{"skill_level":10,"faceit_elo":3000}}}`))
				}
			}))
			t.Cleanup(server.Close)
			client, err := faceit.New(faceit.Options{APIKey: "faceit-test-key", BaseURL: server.URL, HTTPClient: server.Client()})
			if err != nil {
				t.Fatal(err)
			}
			follows, err := faceit.NewFollowStore(filepath.Join(t.TempDir(), "followed.json"), time.Now)
			if err != nil {
				t.Fatal(err)
			}
			repo := newFakeRepo()
			store := newFakeStorage()
			j := job.Job{ID: uuid.New(), Status: job.StatusScanned, Rules: rules.Default(), DemoFileName: test.file}
			repo.jobs[j.ID] = j
			_ = store.Put(artifacts.RosterKey(j.ID), bytes.NewReader([]byte(test.roster)))
			h := NewHandlers(repo, store, &fakeQueue{}, WithFaceit(client, follows))

			rw := httptest.NewRecorder()
			Routes(h).ServeHTTP(rw, httptest.NewRequest(http.MethodGet, "/api/jobs/"+j.ID.String()+"/faceit-scoreboard", nil))
			if rw.Code != test.want {
				t.Fatalf("status = %d, want %d; body=%s", rw.Code, test.want, rw.Body.String())
			}
			if test.code != "" && !strings.Contains(rw.Body.String(), test.code) {
				t.Fatalf("body = %s, want code %s", rw.Body.String(), test.code)
			}
			if test.name == "not a FACEIT file name" || test.want == http.StatusOK {
				if got := int(requests.Load()); got != test.wantRequests {
					t.Fatalf("FACEIT requests = %d, want %d", got, test.wantRequests)
				}
			}
			if test.want != http.StatusOK {
				return
			}
			var body struct {
				Scoreboard faceit.Scoreboard `json:"scoreboard"`
			}
			if err := json.Unmarshal(rw.Body.Bytes(), &body); err != nil {
				t.Fatal(err)
			}
			a := body.Scoreboard.Teams[0].Players[0]
			if a.ADR != 86.0 || a.Assists != 4 || a.ELO == nil || *a.ELO != 3000 {
				t.Fatalf("player a = %+v, want demo ADR 86.0, 4 assists and ELO 3000", a)
			}
			// A second load within the cache TTL reaches FACEIT no more.
			Routes(h).ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/api/jobs/"+j.ID.String()+"/faceit-scoreboard", nil))
			if got := int(requests.Load()); got != test.wantRequests {
				t.Fatalf("FACEIT requests after a cached load = %d, want %d", got, test.wantRequests)
			}
			// The avatar proxy can now serve the roster's FACEIT avatar URL.
			if url, ok := h.faceitCache.lookupAvatar("pa"); !ok || url == "" {
				t.Fatal("scoreboard avatar was not registered with the avatar proxy")
			}
		})
	}
}
