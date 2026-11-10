package httpapi

import (
	"context"
	"net/http"
	"testing"

	"github.com/rechedev9/cliphub/internal/artifacts"
	"github.com/rechedev9/cliphub/internal/library"
	"github.com/rechedev9/cliphub/internal/tacticalplan"
)

func TestLibraryBaselineScopesAndCache(t *testing.T) {
	env := newLibraryTestEnv(t)
	first := env.seedLibraryMatch("first", "Vitality", "MOUZ")
	env.seedLibraryMatch("second", "Vitality", "MOUZ")

	var resp baselineResponse
	env.json(http.MethodGet, "/api/library/baseline?min_rounds=4", nil, http.StatusOK, &resp)
	// Player one plays both rounds of each match: one kill and one death in four rounds.
	if resp.Scope != "library" || resp.Matches != 2 || resp.Players != 1 || resp.MinRounds != 4 {
		t.Fatalf("baseline = %+v", resp)
	}
	if kpr := resp.Metrics[tacticalplan.MetricKPR]; kpr.Players != 1 || kpr.Mean != 0 {
		t.Fatalf("kpr = %+v, want the fixture's event-less rounds to count no kill", kpr)
	}
	if kast := resp.Metrics[tacticalplan.MetricKAST]; kast.Mean != 50 {
		t.Fatalf("kast = %+v, want the two kill-or-survive rounds of four", kast)
	}
	env.json(http.MethodGet, "/api/library/baseline", nil, http.StatusOK, &resp)
	if resp.Players != 0 || resp.MinRounds != tacticalplan.BaselineMinRounds {
		t.Fatalf("default baseline = %+v, want nobody past %d rounds", resp, tacticalplan.BaselineMinRounds)
	}

	key := artifacts.LibraryTacticalIndexKey(first.DemoID)
	opens := env.storage.openCount(key)
	env.json(http.MethodGet, "/api/library/baseline?min_rounds=1", nil, http.StatusOK, &resp)
	if env.storage.openCount(key) != opens {
		t.Fatal("an unchanged library must be served from the cache")
	}

	if resp.Source != "library" || resp.Description != "" {
		t.Fatalf("library baseline source = %q %q", resp.Source, resp.Description)
	}
	bundled := tacticalplan.DefaultProReference()
	env.json(http.MethodGet, "/api/library/baseline?scope=pro", nil, http.StatusOK, &resp)
	if resp.Scope != "pro" || resp.Source != "bundled" || resp.Description != bundled.Description || resp.Players != bundled.Baseline.Players || resp.Map != "" {
		t.Fatalf("pro baseline without HLTV demos = %+v, want the bundled reference", resp)
	}
	hltv := library.SourceHLTV
	if _, err := env.repo.UpdateDemo(context.Background(), first.DemoID, library.DemoPatch{Source: &hltv}); err != nil {
		t.Fatal(err)
	}
	// One HLTV player is under ProReferenceMinPlayers: the bundled map entry, or every map when it has none.
	env.json(http.MethodGet, "/api/library/baseline?scope=pro&map=de_mirage&min_rounds=1", nil, http.StatusOK, &resp)
	if resp.Source != "bundled" || resp.Map != "" {
		t.Fatalf("pro baseline on a map the reference lacks = %+v", resp)
	}
	env.json(http.MethodGet, "/api/library/baseline?scope=pro&map=de_nuke", nil, http.StatusOK, &resp)
	if resp.Source != "bundled" || resp.Map != "de_nuke" || resp.Players != bundled.Maps["de_nuke"].Baseline.Players {
		t.Fatalf("pro baseline on nuke = %+v", resp)
	}
	for _, bad := range []string{"?scope=faceit", "?map=Mirage!", "?min_rounds=0"} {
		if rw := env.do(http.MethodGet, "/api/library/baseline"+bad, nil, ""); rw.Code != http.StatusBadRequest {
			t.Fatalf("baseline%s = %d, want 400", bad, rw.Code)
		}
	}
}

func TestLibraryLeaderboard(t *testing.T) {
	env := newLibraryTestEnv(t)
	env.seedLibraryMatch("first", "Vitality", "MOUZ")

	var resp leaderboardResponse
	env.json(http.MethodGet, "/api/library/leaderboard?metric=dpr&min_rounds=1", nil, http.StatusOK, &resp)
	if resp.Metric != "dpr" || resp.HigherIsBetter || len(resp.Players) != 1 {
		t.Fatalf("leaderboard = %+v", resp)
	}
	if p := resp.Players[0]; p.Rank != 1 || p.SteamID64 != "76561198000000001" || p.Rounds != 2 || p.Matches != 1 {
		t.Fatalf("row = %+v", p)
	}
	env.json(http.MethodGet, "/api/library/leaderboard?scope=pro&metric=kpr&min_rounds=1&limit=5", nil, http.StatusOK, &resp)
	if resp.Source != "bundled" || len(resp.Players) != 5 || resp.Players[0].Team == "" || resp.Players[0].Value < resp.Players[4].Value {
		t.Fatalf("pro leaderboard without HLTV demos = %+v, want the bundled pros best first", resp)
	}
	for _, bad := range []string{"", "?metric=nope", "?metric=kpr&role=a_lurk", "?metric=kpr&map=de_anubis&role=a_lurk&side=CT", "?metric=kpr&source=x", "?metric=kpr&limit=0", "?metric=kpr&min_rounds=x"} {
		if rw := env.do(http.MethodGet, "/api/library/leaderboard"+bad, nil, ""); rw.Code != http.StatusBadRequest {
			t.Fatalf("leaderboard%s = %d, want 400", bad, rw.Code)
		}
	}
}
