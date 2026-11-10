package httpapi

import (
	"net/http"
	"testing"

	"github.com/google/uuid"
)

func TestLibraryTeamMapCountsFollowTheScope(t *testing.T) {
	f := newScoutFixture(t)
	base := "/api/library/teams/" + f.vitality.String() + "/map-counts"
	count := func(query string) int {
		t.Helper()
		var resp teamMapCountsResponse
		f.env.json(http.MethodGet, base+query, nil, http.StatusOK, &resp)
		if len(resp.Maps) != 1 || resp.Maps[0].Map != f.home.Map {
			t.Fatalf("maps = %+v, want only %s", resp.Maps, f.home.Map)
		}
		return resp.Maps[0].Matches
	}
	if got := count(""); got != 2 {
		t.Fatalf("every match = %d, want 2", got)
	}
	if got := count("?opponent_id=" + f.mouz.String()); got != 1 {
		t.Fatalf("against MOUZ = %d, want 1", got)
	}
	if got := count("?opponent_id=" + uuid.NewString()); got != 0 {
		t.Fatalf("against a rival never played = %d, want 0 with the map still listed", got)
	}
	if got := count("?max_matches=1"); got != 1 {
		t.Fatalf("last 1 = %d, want 1", got)
	}
	if got := count("?map=de_nuke&side=CT"); got != 2 {
		t.Fatalf("map and side must not narrow the counts, got %d", got)
	}
	if rw := f.env.do(http.MethodGet, base+"?lineup=nope", nil, ""); rw.Code != http.StatusBadRequest {
		t.Fatalf("bad lineup = %d, want 400", rw.Code)
	}
}
