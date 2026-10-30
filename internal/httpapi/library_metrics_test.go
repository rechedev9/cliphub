package httpapi

import (
	"net/http"
	"testing"

	"github.com/google/uuid"

	"github.com/rechedev9/cliphub/internal/library"
	"github.com/rechedev9/cliphub/internal/tacticalplan"
)

func TestLibraryMatchMetrics(t *testing.T) {
	env := newLibraryTestEnv(t)
	m := env.seedLibraryMatch("metrics", "Vitality", "MOUZ")

	var resp metricsResponse
	env.json(http.MethodGet, "/api/library/matches/"+m.ID.String()+"/metrics", nil, http.StatusOK, &resp)
	if resp.MatchID == nil || *resp.MatchID != m.ID || resp.Matches != 1 || resp.Rounds != 2 {
		t.Fatalf("metrics = match %v, %d matches, %d rounds", resp.MatchID, resp.Matches, resp.Rounds)
	}
	if resp.Model.Version != tacticalplan.WinModelVersion || len(resp.Timelines) != 2 {
		t.Fatalf("model %q with %d timelines, want %q and one per round", resp.Model.Version, len(resp.Timelines), tacticalplan.WinModelVersion)
	}
	if len(resp.Players) != 1 || resp.Players[0].SteamID64 != "76561198000000001" || resp.Players[0].Role != library.RoleUnknown {
		t.Fatalf("players = %+v, want the one player who played, with no role under 20 rounds", resp.Players)
	}
	if rw := env.do(http.MethodGet, "/api/library/matches/"+uuid.NewString()+"/metrics", nil, ""); rw.Code != http.StatusNotFound {
		t.Fatalf("metrics of an unknown match = %d, want 404", rw.Code)
	}
}

func TestLibraryPlayerMetrics(t *testing.T) {
	env := newLibraryTestEnv(t)
	env.seedLibraryMatch("first", "Vitality", "MOUZ")
	env.seedLibraryMatch("second", "Vitality", "MOUZ")

	var resp metricsResponse
	env.json(http.MethodGet, "/api/library/players/76561198000000001/metrics", nil, http.StatusOK, &resp)
	if resp.SteamID64 != "76561198000000001" || resp.Matches != 2 || len(resp.Players) != 1 || resp.Players[0].Matches != 2 {
		t.Fatalf("player metrics = %+v", resp)
	}
	if len(resp.Timelines) != 0 {
		t.Fatal("player metrics must not carry round timelines")
	}
	env.json(http.MethodGet, "/api/library/players/76561198000000001/metrics?limit=1&side=T", nil, http.StatusOK, &resp)
	if resp.Matches != 1 || len(resp.Players) != 0 {
		t.Fatalf("one T-side match = %d matches, %d players", resp.Matches, len(resp.Players))
	}
	for _, bad := range []string{"?limit=0", "?limit=51", "?side=mid", "?map=Mirage!", "?from=ayer"} {
		if rw := env.do(http.MethodGet, "/api/library/players/76561198000000001/metrics"+bad, nil, ""); rw.Code != http.StatusBadRequest {
			t.Fatalf("player metrics%s = %d, want 400", bad, rw.Code)
		}
	}
	if rw := env.do(http.MethodGet, "/api/library/players/76561198000000999/metrics", nil, ""); rw.Code != http.StatusNotFound {
		t.Fatalf("metrics of an unknown player = %d, want 404", rw.Code)
	}
}

func TestLibraryReportMetricsKeepTheScoutedTeam(t *testing.T) {
	f := newScoutFixture(t)
	var report library.Report
	f.env.json(http.MethodPost, "/api/library/reports", map[string]any{
		"title": "Vitality", "team_id": f.vitality, "match_ids": []uuid.UUID{f.home.ID, f.away.ID},
	}, http.StatusCreated, &report)

	var resp metricsResponse
	f.env.json(http.MethodGet, "/api/library/reports/"+report.ID.String()+"/metrics", nil, http.StatusOK, &resp)
	if resp.TeamID == nil || *resp.TeamID != f.vitality || resp.Matches != 2 {
		t.Fatalf("report metrics = team %v over %d matches", resp.TeamID, resp.Matches)
	}
	// Vitality is CT start at home (player one) and T start away (player two,
	// who plays no round in the fixture): MOUZ and Spirit players never show.
	if len(resp.Players) != 1 || resp.Players[0].Matches != 1 || resp.Rounds != 2 {
		t.Fatalf("players = %+v over %d rounds, want Vitality's home player only", resp.Players, resp.Rounds)
	}

	var empty library.Report
	f.env.json(http.MethodPost, "/api/library/reports", map[string]any{"title": "Vacío", "team_id": f.vitality}, http.StatusCreated, &empty)
	f.env.json(http.MethodGet, "/api/library/reports/"+empty.ID.String()+"/metrics", nil, http.StatusOK, &resp)
	if resp.Matches != 0 || len(resp.Players) != 0 {
		t.Fatalf("empty report metrics = %+v", resp)
	}
}
