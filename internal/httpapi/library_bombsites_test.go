package httpapi

import (
	"net/http"
	"testing"

	"github.com/google/uuid"

	"github.com/rechedev9/cliphub/internal/tacticalplan"
)

func TestLibraryTeamBombsitesReadsBothSidesOfEachSite(t *testing.T) {
	f := newScoutFixture(t)
	base := "/api/library/teams/" + f.vitality.String() + "/bombsites"

	var resp bombsitesResponse
	f.env.json(http.MethodGet, base, nil, http.StatusOK, &resp)
	if resp.TeamName != "Vitality" || resp.MatchCount != 2 || len(resp.Skipped) != 0 || len(resp.Maps) != 1 {
		t.Fatalf("bombsites = %s over %d matches, %d maps, skipped %v", resp.TeamName, resp.MatchCount, len(resp.Maps), resp.Skipped)
	}
	m := resp.Maps[0]
	// Home: Vitality defends A (held) then B (lost). Away: it attacks A (lost) then B (won).
	if m.Map != "de_mirage" || m.Matches != 2 || m.CTRounds != 2 || m.TRounds != 2 {
		t.Fatalf("map = %+v", m)
	}
	a, b := m.Sites[0], m.Sites[1]
	if a.CT.Hit.Count != 1 || a.CT.Held.Count != 1 || b.CT.Hit.Count != 1 || b.CT.Held.Count != 0 {
		t.Fatalf("CT sites = A %+v / B %+v", a.CT, b.CT)
	}
	if a.T.Attacks.Total != 2 || a.T.Won.Count != 0 || b.T.Won.Count != 1 || b.T.Plants.Count != 0 {
		t.Fatalf("T sites = A %+v / B %+v", a.T, b.T)
	}
	if len(a.CT.Examples) != 1 || a.CT.Examples[0].MatchID != f.home.ID {
		t.Fatalf("A CT examples = %+v, want the home match", a.CT.Examples)
	}

	f.env.json(http.MethodGet, base+"?side=ct&opponent_id="+f.mouz.String(), nil, http.StatusOK, &resp)
	if resp.MatchCount != 1 || resp.Maps[0].TRounds != 0 || resp.Filter.Side != tacticalplan.SideCT {
		t.Fatalf("CT against MOUZ = %d matches, %d T rounds", resp.MatchCount, resp.Maps[0].TRounds)
	}
	for _, bad := range []string{"?side=mid", "?map=Mirage!", "?from=ayer", "?match_id=nope"} {
		if rw := f.env.do(http.MethodGet, base+bad, nil, ""); rw.Code != http.StatusBadRequest {
			t.Fatalf("bombsites%s = %d, want 400", bad, rw.Code)
		}
	}
	if rw := f.env.do(http.MethodGet, "/api/library/teams/"+uuid.NewString()+"/bombsites", nil, ""); rw.Code != http.StatusNotFound {
		t.Fatalf("bombsites of an unknown team = %d, want 404", rw.Code)
	}
}
