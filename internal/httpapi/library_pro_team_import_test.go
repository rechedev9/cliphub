package httpapi

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/rechedev9/cliphub/internal/community"
)

func brokenProClone(entry, sha string) community.ProMapFile {
	return community.ProMapFile{Entry: entry, Map: "de_nuke", Rounds: 20, AnalysisVersion: 9, SHA256: sha,
		Parts: []community.ProMapPart{{Entry: "vitality-vs-faze-m3-nuke", FirstRound: 1, Rounds: 20, SHA256: sha}}}
}

func TestProTeamImportBringsEveryPublicMapInBatches(t *testing.T) {
	srv, catalog := fakeProServer(t)
	for i, entry := range []string{"vitality-vs-faze-m4-nuke", "vitality-vs-faze-m5-nuke", "vitality-vs-faze-m6-nuke", "vitality-vs-faze-m7-nuke"} {
		catalog.Maps = append(catalog.Maps, brokenProClone(entry, "broken-"+string(rune('a'+i))))
	}
	env := proTestEnv(t, srv.URL)
	var first proTeamImportResponse
	env.json(http.MethodPost, "/api/library/pro-teams/9565/import", nil, http.StatusOK, &first)
	if first.Name != "Vitality" || first.Days != community.MaxProMatchDays || first.Total != 7 || first.Imported != 2 || first.Held != 2 || first.Remaining != 3 || first.TeamID == nil {
		t.Fatalf("first batch = %+v", first)
	}
	vitality, err := env.h.teamByName(t.Context(), "Vitality")
	if err != nil || *first.TeamID != vitality.ID {
		t.Fatalf("the answer must name the library team the maps went to: %v %v", first.TeamID, err)
	}
	var second proTeamImportResponse
	env.json(http.MethodPost, "/api/library/pro-teams/9565/import", nil, http.StatusOK, &second)
	if second.Imported != 0 || second.Held != 2 || second.Remaining != 0 {
		t.Fatalf("the second batch must try the rest and skip the maps that did not read: %+v", second)
	}
	var again proTeamImportResponse
	env.json(http.MethodPost, "/api/library/pro-teams/9565/import", nil, http.StatusOK, &again)
	if again.Imported != 0 || again.Remaining != 0 {
		t.Fatalf("a map that did not read is not fetched again: %+v", again)
	}
}

func TestProTeamImportKeepsADeletedMapOut(t *testing.T) {
	srv, _ := fakeProServer(t)
	env := proTestEnv(t, srv.URL)
	var first proImportResponse
	env.json(http.MethodPost, "/api/library/pro-matches/2400001/vitality-vs-faze-m1-mirage/import", nil, http.StatusOK, &first)
	if rw := env.do(http.MethodDelete, "/api/library/demos/"+first.DemoID.String(), nil, ""); rw.Code != http.StatusNoContent && rw.Code != http.StatusOK {
		t.Fatalf("delete = %d %s", rw.Code, rw.Body)
	}
	var resp proTeamImportResponse
	env.json(http.MethodPost, "/api/library/pro-teams/9565/import", nil, http.StatusOK, &resp)
	if resp.Imported != 1 || resp.Held != 1 || resp.Remaining != 0 {
		t.Fatalf("the map the user deleted must stay out: %+v", resp)
	}
}

func TestProTeamImportErrors(t *testing.T) {
	if rw := newLibraryTestEnv(t).do(http.MethodPost, "/api/library/pro-teams/9565/import", nil, ""); rw.Code != http.StatusServiceUnavailable || !strings.Contains(rw.Body.String(), codeCommunityOff) {
		t.Fatalf("no community server = %d %s", rw.Code, rw.Body)
	}
	old := httptest.NewServer(http.NotFoundHandler())
	defer old.Close()
	env := proTestEnv(t, old.URL)
	if rw := env.do(http.MethodPost, "/api/library/pro-teams/9565/import", nil, ""); rw.Code != http.StatusNotFound || !strings.Contains(rw.Body.String(), codeNoProMap) {
		t.Fatalf("a server without the catalog = %d %s", rw.Code, rw.Body)
	}
	if rw := env.do(http.MethodPost, "/api/library/pro-teams/x/import", nil, ""); rw.Code != http.StatusBadRequest {
		t.Fatalf("a bad team id = %d", rw.Code)
	}
	down := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusBadGateway) }))
	defer down.Close()
	if rw := proTestEnv(t, down.URL).do(http.MethodPost, "/api/library/pro-teams/9565/import", nil, ""); rw.Code != http.StatusBadGateway || !strings.Contains(rw.Body.String(), codeCommunityDown) {
		t.Fatalf("a failing server = %d %s", rw.Code, rw.Body)
	}
}
