package httpapi

import (
	"context"
	"net/http"
	"testing"

	"github.com/rechedev9/cliphub/internal/library"
	"github.com/rechedev9/cliphub/internal/tacticalplan"
)

func TestLibraryLeaderboardFiltersSideDatesAndSource(t *testing.T) {
	env := newLibraryTestEnv(t)
	m := env.seedLibraryMatch("first", "Vitality", "MOUZ")
	const base = "/api/library/leaderboard?metric=rating&min_rounds=1"

	var all, ct, tt leaderboardResponse
	env.json(http.MethodGet, base, nil, http.StatusOK, &all)
	env.json(http.MethodGet, base+"&side=CT", nil, http.StatusOK, &ct)
	env.json(http.MethodGet, base+"&side=T", nil, http.StatusOK, &tt)
	if len(all.Players) == 0 || all.Players[0].Metrics[tacticalplan.MetricRating] == 0 || len(all.Players[0].Examples) == 0 {
		t.Fatalf("leaderboard = %+v, want rows with a rating and rounds to open", all)
	}
	rounds := func(r leaderboardResponse, steam string) int {
		for _, p := range r.Players {
			if p.SteamID64 == steam {
				return p.Rounds
			}
		}
		return 0
	}
	steam := all.Players[0].SteamID64
	if got := rounds(ct, steam) + rounds(tt, steam); got != rounds(all, steam) {
		t.Fatalf("CT %d + T %d rounds, want the %d of both sides", rounds(ct, steam), rounds(tt, steam), rounds(all, steam))
	}

	var none leaderboardResponse
	env.json(http.MethodGet, base+"&from=2999-01-01T00:00:00Z", nil, http.StatusOK, &none)
	if none.Matches != 0 || len(none.Players) != 0 {
		t.Fatalf("a period after every match = %+v, want an empty board", none)
	}
	env.json(http.MethodGet, base+"&source=hltv", nil, http.StatusOK, &none)
	if len(none.Players) != 0 {
		t.Fatalf("hltv board = %+v, want nobody before the demo is a tournament one", none)
	}
	hltv := library.SourceHLTV
	if _, err := env.repo.UpdateDemo(context.Background(), m.DemoID, library.DemoPatch{Source: &hltv}); err != nil {
		t.Fatal(err)
	}
	env.json(http.MethodGet, base+"&source=hltv", nil, http.StatusOK, &none)
	if len(none.Players) == 0 || none.MatchSource != "hltv" {
		t.Fatalf("hltv board = %+v, want the tournament demo", none)
	}

	var withRoles leaderboardResponse
	env.json(http.MethodGet, base+"&map="+m.Map, nil, http.StatusOK, &withRoles)
	if len(withRoles.Players) == 0 || withRoles.Players[0].Roles == nil {
		t.Fatalf("board on a map = %+v, want each row to carry its roles", withRoles)
	}
}
