package httpapi

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/rechedev9/cliphub/internal/community"
	"github.com/rechedev9/cliphub/internal/library"
	"github.com/rechedev9/cliphub/internal/tacticalplan"
)

const proTestMatch = 2400001

// fakeProServer serves a catalog of one HLTV match with two maps, their analyses
// and a third map whose file does not decode; tests may change the match it serves.
func fakeProServer(t *testing.T) (*httptest.Server, *community.ProMatch) {
	t.Helper()
	rank := 1
	files := map[string][]byte{}
	page := community.ProMatchesPage{Days: 7, Page: 1, Pages: 1, Total: 1, UpdatedAt: time.Now().UTC()}
	match := community.ProMatch{ID: proTestMatch, Event: "IEM Chengdu", PlayedAt: time.Date(2026, 11, 8, 15, 0, 0, 0, time.UTC),
		Teams: []community.ProTeam{{ID: 9565, Name: "Vitality", Rank: 1}, {ID: 6667, Name: "FaZe"}}}
	for _, entry := range []string{"vitality-vs-faze-m1-mirage", "vitality-vs-faze-m2-mirage"} {
		doc, blob := tacticalFixture(t, uuid.Nil)
		sum := sha256.Sum256([]byte(entry))
		doc.Demo.SHA256 = hex.EncodeToString(sum[:])
		doc.Teams = []tacticalplan.Team{{Key: "a", Name: "Vitality", StartSide: tacticalplan.SideCT}, {Key: "b", Name: "FaZe Clan", StartSide: tacticalplan.SideT}}
		doc.Players = []tacticalplan.Player{{Slot: 0, SteamID64: "1", Name: "one", TeamKey: "a", StartSide: tacticalplan.SideCT},
			{Slot: 1, SteamID64: "2", Name: "two", TeamKey: "b", StartSide: tacticalplan.SideT}}
		h := community.ProAnalysisHeader{Format: community.ProAnalysisFormat, Entry: entry + ".dem", Document: doc, Positions: blob.Descriptor,
			Match: community.ProSidecar{Source: "hltv", MatchID: proTestMatch, Event: match.Event, PlayedAt: match.PlayedAt.Format(time.RFC3339),
				Teams: []community.ProSidecarTeam{{ID: 9565, Name: "Vitality", Rank: &rank}, {ID: 6667, Name: "FaZe"}}, Maps: []string{"Mirage", "Mirage", "Nuke"}}}
		var buf bytes.Buffer
		if err := community.WriteProAnalysis(&buf, h, blob.Data); err != nil {
			t.Fatal(err)
		}
		files[entry] = buf.Bytes()
		fileSum := sha256.Sum256(buf.Bytes())
		part := community.ProMapPart{Entry: entry, FirstRound: 1, Rounds: 2, AnalysisVersion: doc.AnalysisVersion, Bytes: int64(buf.Len()), SHA256: hex.EncodeToString(fileSum[:])}
		match.Maps = append(match.Maps, community.ProMapFile{Entry: entry, Map: "de_mirage", Rounds: 2, AnalysisVersion: doc.AnalysisVersion, Bytes: part.Bytes,
			SHA256: part.SHA256, Parts: []community.ProMapPart{part}})
	}
	files["vitality-vs-faze-m3-nuke"] = []byte("not zstd")
	match.Maps = append(match.Maps, community.ProMapFile{Entry: "vitality-vs-faze-m3-nuke", Map: "de_nuke", Rounds: 20, AnalysisVersion: 9, SHA256: "broken",
		Parts: []community.ProMapPart{{Entry: "vitality-vs-faze-m3-nuke", FirstRound: 1, Rounds: 20, SHA256: "broken"}}})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/pro-matches":
			page.Matches = []community.ProMatch{match}
			_ = json.NewEncoder(w).Encode(page)
			return
		case "/v1/pro-matches/2400001":
			_ = json.NewEncoder(w).Encode(match)
			return
		}
		if body, ok := files[strings.TrimPrefix(r.URL.Path, "/v1/pro-matches/2400001/")]; ok {
			_, _ = w.Write(body)
			return
		}
		http.NotFound(w, r)
	}))
	t.Cleanup(srv.Close)
	return srv, &match
}

func proTestEnv(t *testing.T, url string) libraryTestEnv {
	t.Helper()
	env := newLibraryTestEnv(t)
	client, err := community.NewClient(url)
	if err != nil {
		t.Fatal(err)
	}
	env.h.references.community.Client = client
	return env
}

func TestProMatchImportFollowAndSeries(t *testing.T) {
	srv, catalog := fakeProServer(t)
	env := proTestEnv(t, srv.URL)
	var list proMatchesResponse
	env.json(http.MethodGet, "/api/library/pro-matches", nil, http.StatusOK, &list)
	if list.Status != proCatalogOK || len(list.Matches) != 1 || list.Matches[0].Maps[0].LibraryMatchID != nil {
		t.Fatalf("catalog = %+v", list)
	}

	var first proImportResponse
	env.json(http.MethodPost, "/api/library/pro-matches/2400001/vitality-vs-faze-m1-mirage/import", nil, http.StatusOK, &first)
	if !first.Created || first.StudioPath != "/partidas/"+first.MatchID.String()+"?tab=replay" {
		t.Fatalf("import = %+v", first)
	}
	var match library.Match
	env.json(http.MethodGet, "/api/library/matches/"+first.MatchID.String(), nil, http.StatusOK, &match)
	names := []string{}
	for _, side := range match.Teams {
		names = append(names, side.TeamName)
	}
	if match.Source != library.SourcePro || strings.Join(names, ",") != "Vitality,FaZe" {
		t.Fatalf("pro match = %s teams %v", match.Source, names)
	}
	var again proImportResponse
	env.json(http.MethodPost, "/api/library/pro-matches/2400001/vitality-vs-faze-m1-mirage/import", nil, http.StatusOK, &again)
	if again.Created || again.MatchID != first.MatchID {
		t.Fatalf("a second import must return the first match: %+v", again)
	}
	catalog.Maps[0].SHA256 = "prepared-again"
	env.json(http.MethodPost, "/api/library/pro-matches/2400001/vitality-vs-faze-m1-mirage/import", nil, http.StatusOK, &again)
	if rec, err := env.repo.GetProMap(t.Context(), proTestMatch, "vitality-vs-faze-m1-mirage"); err != nil || again.MatchID != first.MatchID || rec.SHA256 != "prepared-again" {
		t.Fatalf("a newer catalog file must replace the analysis in place: %+v %+v %v", again, rec, err)
	}
	if rec, _ := env.repo.GetProMap(t.Context(), proTestMatch, "vitality-vs-faze-m1-mirage"); len(rec.Teams[0].SteamIDs) != 1 || rec.Teams[0].SteamIDs[0] != "1" {
		t.Fatalf("the record must keep the players of each HLTV team: %+v", rec.Teams)
	}
	if rw := env.do(http.MethodPost, "/api/library/pro-matches/2400001/vitality-vs-faze-m3-nuke/import", nil, ""); rw.Code != http.StatusBadGateway || !strings.Contains(rw.Body.String(), codeProMapUnreadable) {
		t.Fatalf("an unreadable map = %d %s", rw.Code, rw.Body)
	}
	if _, err := env.h.importProMap(t.Context(), proMapRef{matchID: proTestMatch, file: catalog.Maps[2]}); !errors.Is(err, errProMapBroken) {
		t.Fatalf("the sync must skip a hash that failed before: %v", err)
	}
	if rw := env.do(http.MethodPost, "/api/library/demos/"+first.DemoID.String()+"/analyze", nil, ""); rw.Code != http.StatusConflict || !strings.Contains(rw.Body.String(), codeSharedMatch) {
		t.Fatalf("analyze a pro map = %d %s", rw.Code, rw.Body)
	}
	var info proMatchInfo
	env.json(http.MethodGet, "/api/library/matches/"+first.MatchID.String()+"/pro", nil, http.StatusOK, &info)
	if info.MatchID != proTestMatch || info.Event != "IEM Chengdu" || info.Teams[0].Rank != 1 {
		t.Fatalf("pro info = %+v", info)
	}

	var second proImportResponse
	env.json(http.MethodPost, "/api/library/pro-matches/2400001/vitality-vs-faze-m2-mirage/import", nil, http.StatusOK, &second)
	a, _ := env.repo.GetMatch(t.Context(), first.MatchID)
	b, _ := env.repo.GetMatch(t.Context(), second.MatchID)
	if a.SeriesID == nil || b.SeriesID == nil || *a.SeriesID != *b.SeriesID {
		t.Fatalf("two maps of one HLTV match must share a series: %v %v", a.SeriesID, b.SeriesID)
	}
	if s, err := env.repo.GetSeries(t.Context(), *a.SeriesID); err != nil || s.Format != library.FormatBO3 || s.Event != "IEM Chengdu" {
		t.Fatalf("series = %+v %v", s, err)
	}
	env.json(http.MethodGet, "/api/library/pro-matches", nil, http.StatusOK, &list)
	if got := list.Matches[0].Maps[1].LibraryMatchID; got == nil || *got != second.MatchID {
		t.Fatalf("the catalog must show the imported map: %+v", list.Matches[0].Maps)
	}

	// A map the user deleted stays out of the follow sync, but a click brings it back.
	if err := env.repo.DeleteDemo(t.Context(), second.DemoID); err != nil {
		t.Fatal(err)
	}
	env.json(http.MethodPost, "/api/library/pro-follows", map[string]any{"team_id": 6667, "name": "FaZe"}, http.StatusOK, nil)
	if n, err := env.h.syncProFollows(context.Background()); err != nil || n != 0 {
		t.Fatalf("sync imported %d: %v", n, err)
	}
	follows, _ := env.repo.ListProFollows(t.Context())
	if len(follows) != 1 || follows[0].SyncedAt == nil {
		t.Fatalf("follows = %+v", follows)
	}
	var back proImportResponse
	env.json(http.MethodPost, "/api/library/pro-matches/2400001/vitality-vs-faze-m2-mirage/import", nil, http.StatusOK, &back)
	if !back.Created {
		t.Fatalf("an explicit import must bring a deleted map back: %+v", back)
	}
	if rw := env.do(http.MethodDelete, "/api/library/pro-follows/6667", nil, ""); rw.Code != http.StatusNoContent {
		t.Fatalf("unfollow = %d", rw.Code)
	}
}

func TestProFollowSyncImportsNewMaps(t *testing.T) {
	srv, _ := fakeProServer(t)
	env := proTestEnv(t, srv.URL)
	env.json(http.MethodPost, "/api/library/pro-follows", map[string]any{"team_id": 9565, "name": "Vitality"}, http.StatusOK, nil)
	if n, err := env.h.syncProFollows(context.Background()); err != nil || n != 2 {
		t.Fatalf("sync imported %d: %v", n, err)
	}
	if n, err := env.h.syncProFollows(context.Background()); err != nil || n != 0 {
		t.Fatalf("a second sync must import nothing: %d %v", n, err)
	}
}

func TestProMatchesDegradeWithoutCatalog(t *testing.T) {
	var resp proMatchesResponse
	newLibraryTestEnv(t).json(http.MethodGet, "/api/library/pro-matches", nil, http.StatusOK, &resp)
	if resp.Status != proCatalogDisabled || resp.Matches == nil {
		t.Fatalf("no community server = %+v", resp)
	}
	old := httptest.NewServer(http.NotFoundHandler())
	defer old.Close()
	env := proTestEnv(t, old.URL)
	env.json(http.MethodGet, "/api/library/pro-matches", nil, http.StatusOK, &resp)
	if resp.Status != proCatalogMissing {
		t.Fatalf("a server without the catalog = %q", resp.Status)
	}
	if rw := env.do(http.MethodPost, "/api/library/pro-matches/2400001/a-vs-b-m1-nuke/import", nil, ""); rw.Code != http.StatusNotFound || !strings.Contains(rw.Body.String(), codeNoProMap) {
		t.Fatalf("import from a server without the catalog = %d %s", rw.Code, rw.Body)
	}
	if rw := env.do(http.MethodPost, "/api/library/pro-matches/x/..%2Fy/import", nil, ""); rw.Code != http.StatusBadRequest {
		t.Fatalf("a bad map name = %d", rw.Code)
	}
}

func TestProFollowSyncStopsWhenTheServerFails(t *testing.T) {
	down := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
	}))
	defer down.Close()
	env := proTestEnv(t, down.URL)
	for _, f := range []map[string]any{{"team_id": 1, "name": "A"}, {"team_id": 2, "name": "B"}} {
		env.json(http.MethodPost, "/api/library/pro-follows", f, http.StatusOK, nil)
	}
	if _, err := env.h.syncProFollows(context.Background()); !errors.Is(err, community.ErrProServerDown) {
		t.Fatalf("a failing server must end the pass: %v", err)
	}
	if follows, _ := env.repo.ListProFollows(t.Context()); follows[0].SyncedAt != nil || follows[1].SyncedAt != nil {
		t.Fatalf("no team is synced when the server fails: %+v", follows)
	}
}
