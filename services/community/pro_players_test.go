package main

import (
	"context"
	"encoding/json"
	"net/http"
	"path/filepath"
	"testing"
	"time"

	"github.com/rechedev9/cliphub/internal/community"
	"github.com/rechedev9/cliphub/internal/tacticalplan"
)

func ptr(n int) *int { return &n }

func TestResolveTeamsPairsClanNamesWithHLTVRanks(t *testing.T) {
	doc := []tacticalplan.Team{{Key: "ct-start", Name: "Team Vitality"}, {Key: "t-start", Name: "FALCONS eSports"}}
	got := resolveTeams(doc, []hltvTeam{{Name: "Falcons", Rank: ptr(4)}, {Name: "Vitality", Rank: ptr(1)}})
	if got["ct-start"].Rank != 1 || got["t-start"].Rank != 4 || got["t-start"].Name != "Falcons" {
		t.Fatalf("teams = %+v", got)
	}
	// One clan name nobody recognises takes the HLTV team that is left.
	odd := resolveTeams([]tacticalplan.Team{{Key: "ct-start", Name: "VIT"}, {Key: "t-start", Name: "Falcons"}}, []hltvTeam{{Name: "Falcons", Rank: ptr(4)}, {Name: "Vitality", Rank: ptr(1)}})
	if odd["ct-start"].Rank != 1 || odd["t-start"].Rank != 4 {
		t.Fatalf("leftover = %+v", odd)
	}
	none := resolveTeams(doc, []hltvTeam{{Name: "Spirit"}, {Name: "NAVI"}})
	if none["ct-start"].Rank != 0 || none["ct-start"].Name != "Team Vitality" {
		t.Fatalf("unranked = %+v, want the clan name and no rank", none)
	}
}

func storeProPlayers(t *testing.T, a *api, lines []tacticalplan.ProPlayerSide, playedAt time.Time) {
	t.Helper()
	m := tacticalplan.ProPlayersMatch{MatchID: "1", Map: "de_anubis", PlayedAt: playedAt, Rounds: 24, Sides: lines}
	body, err := json.Marshal(proPlayersPayload{BuiltAt: playedAt, Matches: []tacticalplan.ProPlayersMatch{m}})
	if err != nil {
		t.Fatal(err)
	}
	index, _ := json.Marshal(proPlayersIndex{BuiltAt: playedAt, Maps: []string{"de_anubis"}})
	payloads := map[string][]byte{proPlayersPrefix + "de_anubis": body, proPlayersIndexKey: index}
	if err := a.store.replacePrefixed(context.Background(), proPlayersPrefix, payloads); err != nil {
		t.Fatal(err)
	}
}

func proLine(steam, name string, side tacticalplan.Side, rank int, roles tacticalplan.RoleCounts) tacticalplan.ProPlayerSide {
	return tacticalplan.ProPlayerSide{
		SteamID64: steam, Name: name, Team: "Falcons", Side: side, OpponentRank: rank, Rounds: 12, Kills: 9, Deaths: 7,
		Box: tacticalplan.BoxScore{Damage: 900, KASTRounds: 9}, Roles: roles,
	}
}

func TestProPlayersEndpointFiltersAndServesTheStoredLines(t *testing.T) {
	a, h := testAPI(t)
	lurk := tacticalplan.RoleCounts{Sampled: 12, Lurk: 9, LurkB: 9}
	storeProPlayers(t, a, []tacticalplan.ProPlayerSide{
		proLine("1", "lurker", tacticalplan.SideT, 3, lurk),
		proLine("2", "entry", tacticalplan.SideT, 3, tacticalplan.RoleCounts{Sampled: 12}),
		proLine("3", "weak-opponent", tacticalplan.SideT, 40, lurk),
	}, time.Now().AddDate(0, 0, -5))

	var board community.ProPlayersBoard
	rec := do(t, h, http.MethodGet, "/v1/pro-players?map=de_anubis&role=b_lurk&days=30&top=15&min_rounds=10", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d: %s", rec.Code, rec.Body.String())
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &board); err != nil {
		t.Fatal(err)
	}
	if board.Side != "T" || board.Maps != 1 || len(board.Players) != 1 || board.Players[0].Name != "lurker" || board.Players[0].Team != "Falcons" {
		t.Fatalf("board = %+v, want the B lurker who faced a top 15 team", board)
	}
	if rec.Header().Get("ETag") == "" || rec.Header().Get("Cache-Control") == "" {
		t.Fatal("a board is cached like the other stored payloads")
	}

	rec = do(t, h, http.MethodGet, "/v1/pro-players?days=1", nil)
	if err := json.Unmarshal(rec.Body.Bytes(), &board); err != nil || len(board.Players) != 0 || board.Maps != 0 {
		t.Fatalf("one day = %+v (%v), want an empty board", board, err)
	}
	rec = do(t, h, http.MethodGet, "/v1/pro-players?metric=kpr&min_rounds=1", nil)
	if err := json.Unmarshal(rec.Body.Bytes(), &board); err != nil || len(board.Players) != 3 || board.Players[0].Roles != nil {
		t.Fatalf("all maps = %+v (%v), want every player and no roles without a map", board, err)
	}
}

func TestProPlayersEndpointRejectsBadQueries(t *testing.T) {
	a, h := testAPI(t)
	storeProPlayers(t, a, nil, time.Now())
	for _, q := range []string{"?role=b_lurk", "?map=de_anubis&role=boss", "?map=de_anubis&role=b_lurk&side=CT", "?side=X", "?metric=nope", "?days=0", "?top=51", "?map=Anubis!"} {
		if rec := do(t, h, http.MethodGet, "/v1/pro-players"+q, nil); rec.Code != http.StatusBadRequest {
			t.Errorf("%s = %d, want 400", q, rec.Code)
		}
	}
	var empty community.ProPlayersBoard
	rec := do(t, h, http.MethodGet, "/v1/pro-players?map=de_nuke", nil)
	if err := json.Unmarshal(rec.Body.Bytes(), &empty); rec.Code != http.StatusOK || err != nil || len(empty.Players) != 0 {
		t.Errorf("a map without payload = %d %s, want an empty board", rec.Code, rec.Body.String())
	}
}

func TestRebuildReferenceKeepsTheProPlayersPayloads(t *testing.T) {
	a, _ := testAPI(t)
	storeProPlayers(t, a, nil, time.Now())
	if err := a.store.replaceReference(context.Background(), map[string][]byte{"index": []byte(`{}`)}); err != nil {
		t.Fatal(err)
	}
	if _, _, err := a.store.referenceBody(context.Background(), proPlayersIndexKey); err != nil {
		t.Fatalf("a reference rebuild dropped the pro players: %v", err)
	}
}

func TestBuildProPlayersKeepsStoredPayloadsWhenNothingIsRead(t *testing.T) {
	a, _ := testAPI(t)
	storeProPlayers(t, a, nil, time.Now())
	if _, err := buildProPlayers(context.Background(), a.store, filepath.Join(t.TempDir(), "missing"), time.Now()); err == nil {
		t.Fatal("an empty analyses folder must not rebuild the pro players")
	}
	if _, _, err := a.store.referenceBody(context.Background(), proPlayersPrefix+"de_anubis"); err != nil {
		t.Fatalf("stored payload dropped: %v", err)
	}
}
