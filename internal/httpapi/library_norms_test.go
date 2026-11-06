package httpapi

import (
	"net/http"
	"testing"

	"github.com/rechedev9/cliphub/internal/artifacts"
	"github.com/rechedev9/cliphub/internal/tacticalplan"
)

func TestLibraryNormsPoolEveryTeamAndCache(t *testing.T) {
	f := newScoutFixture(t)

	var resp normsResponse
	f.env.json(http.MethodGet, "/api/library/norms?map=de_mirage", nil, http.StatusOK, &resp)
	if resp.Map != "de_mirage" || resp.Matches != 3 || resp.Skipped != 0 || len(resp.Bombsites) != 1 || len(resp.Playbook) != 2 {
		t.Fatalf("norms = %+v", resp)
	}
	// Both teams of each 2-round match are read: 6 rounds per side over 3 matches.
	sides := map[tacticalplan.Side]int{}
	for _, p := range resp.Playbook {
		sides[p.Side] = p.RoundCount
	}
	if sides[tacticalplan.SideT] != 6 || sides[tacticalplan.SideCT] != 6 {
		t.Fatalf("playbook rounds per side = %v, want 6 each", sides)
	}
	if m := resp.Bombsites[0]; m.Matches != 3 || m.CTRounds != 6 || m.TRounds != 6 {
		t.Fatalf("bombsites = %+v", m)
	}
	matchupRounds := 0
	for _, row := range resp.Matchups {
		matchupRounds += row.WinRate.Total
	}
	if matchupRounds != 12 {
		t.Fatalf("matchup rounds = %d, want every round once per team", matchupRounds)
	}

	key := artifacts.LibraryTacticalIndexKey(f.home.DemoID)
	opens := f.env.storage.openCount(key)
	f.env.json(http.MethodGet, "/api/library/norms?map=de_mirage", nil, http.StatusOK, &resp)
	if f.env.storage.openCount(key) != opens {
		t.Fatal("an unchanged library must be served from the cache")
	}

	f.env.json(http.MethodGet, "/api/library/norms", nil, http.StatusOK, &resp)
	if resp.Matches != 3 || len(resp.Playbook) != 0 || len(resp.Matchups) == 0 {
		t.Fatalf("norms without a map = %d matches, %d playbook sides; the categories need one map", resp.Matches, len(resp.Playbook))
	}
	f.env.json(http.MethodGet, "/api/library/norms?map=de_mirage&from=2099-01-01T00:00:00Z", nil, http.StatusOK, &resp)
	if resp.Matches != 0 || len(resp.Bombsites) != 0 {
		t.Fatalf("norms after every match = %+v", resp)
	}
	for _, bad := range []string{"?map=Mirage!", "?from=ayer", "?to=2026"} {
		if rw := f.env.do(http.MethodGet, "/api/library/norms"+bad, nil, ""); rw.Code != http.StatusBadRequest {
			t.Fatalf("norms%s = %d, want 400", bad, rw.Code)
		}
	}
}
