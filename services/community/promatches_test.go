package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/rechedev9/cliphub/internal/community"
	"github.com/rechedev9/cliphub/internal/tacticalplan"
)

func writeProMap(t *testing.T, dir string, h community.ProAnalysisHeader) string {
	t.Helper()
	h.Format = community.ProAnalysisFormat
	if len(h.Document.Rounds) == 0 {
		h.Document.Rounds = proRounds(1, 16)
	}
	name := filepath.Join(dir, analysisFileName(strconv.Itoa(h.Match.MatchID)+"/"+h.Entry))
	if err := writeAnalysis(name, h, []byte{1, 2, 3}); err != nil {
		t.Fatal(err)
	}
	return name
}

func proRounds(first, last int) []tacticalplan.Round {
	out := []tacticalplan.Round{}
	for n := first; n <= last; n++ {
		out = append(out, tacticalplan.Round{Number: n})
	}
	return out
}

func proHeader(match int, entry, mapName, playedAt string, teams ...community.ProSidecarTeam) community.ProAnalysisHeader {
	h := community.ProAnalysisHeader{Match: community.ProSidecar{Source: hltvDataset, MatchID: match, Event: "IEM Chengdu", PlayedAt: playedAt, Teams: teams}, Entry: entry, Document: tacticalplan.NewDocument()}
	h.Document.Demo.Map = mapName
	return h
}

func TestProMatchCatalogSyncsFiltersAndServesFiles(t *testing.T) {
	a, h := testAPI(t)
	dir := t.TempDir()
	a.analysesDir = dir
	rank := 3
	recent := time.Now().Add(-48 * time.Hour).UTC().Format(time.RFC3339)
	old := time.Now().AddDate(0, 0, -30).UTC().Format(time.RFC3339)
	vit, faze := community.ProSidecarTeam{ID: 9565, Name: "Vitality", Rank: &rank}, community.ProSidecarTeam{ID: 6667, Name: "FaZe"}
	writeProMap(t, dir, proHeader(2400001, "vitality-vs-faze-m1-mirage.dem", "de_mirage", recent, vit, faze))
	writeProMap(t, dir, proHeader(2400001, "vitality-vs-faze-m2-nuke.dem", "de_nuke", recent, vit, faze))
	gone := writeProMap(t, dir, proHeader(2300000, "a-vs-b-m1-inferno.dem", "de_inferno", old, community.ProSidecarTeam{ID: 1, Name: "A"}, community.ProSidecarTeam{ID: 2, Name: "B"}))
	ctx := context.Background()
	if n, err := syncProCatalog(ctx, a.store, dir); err != nil || n != 3 {
		t.Fatalf("first sync changed %d: %v", n, err)
	}
	if n, err := syncProCatalog(ctx, a.store, dir); err != nil || n != 0 {
		t.Fatalf("an unchanged folder must not be read again: %d %v", n, err)
	}

	var page community.ProMatchesPage
	rec := do(t, h, http.MethodGet, "/v1/pro-matches", nil)
	if rec.Code != http.StatusOK || json.Unmarshal(rec.Body.Bytes(), &page) != nil {
		t.Fatalf("catalog = %d %s", rec.Code, rec.Body)
	}
	if page.Total != 1 || len(page.Matches[0].Maps) != 2 || page.Matches[0].Teams[0].Rank != 3 || page.Matches[0].Maps[0].Entry != "vitality-vs-faze-m1-mirage" {
		t.Fatalf("the default week must hold the recent match only: %+v", page)
	}
	if len(page.Facets.Teams) != 2 || len(page.Facets.Maps) != 2 {
		t.Fatalf("facets = %+v", page.Facets)
	}
	for path, want := range map[string]int{"/v1/pro-matches?days=60": 2, "/v1/pro-matches?days=60&team=6667": 1, "/v1/pro-matches?days=60&team=fa": 1,
		"/v1/pro-matches?days=60&map=de_inferno": 1, "/v1/pro-matches?days=60&event=chengdu": 2, "/v1/pro-matches?days=60&event=major": 0} {
		page = community.ProMatchesPage{}
		rec = do(t, h, http.MethodGet, path, nil)
		if err := json.Unmarshal(rec.Body.Bytes(), &page); err != nil || page.Total != want {
			t.Fatalf("%s total = %d, want %d (%s)", path, page.Total, want, rec.Body)
		}
	}
	if rec := do(t, h, http.MethodGet, "/v1/pro-matches?days=999", nil); rec.Code != http.StatusBadRequest {
		t.Fatalf("a period past the limit must be refused, got %d", rec.Code)
	}

	rec = do(t, h, http.MethodGet, "/v1/pro-matches/2400001/vitality-vs-faze-m2-nuke", nil)
	etag := rec.Header().Get("ETag")
	if rec.Code != http.StatusOK || rec.Header().Get("Content-Type") != "application/zstd" || etag == "" {
		t.Fatalf("file = %d %v", rec.Code, rec.Header())
	}
	if got, _, err := community.ReadProAnalysis(rec.Body); err != nil || got.Document.Demo.Map != "de_nuke" {
		t.Fatalf("served file does not decode: %v", err)
	}
	req := httptest.NewRequest(http.MethodGet, "/v1/pro-matches/2400001/vitality-vs-faze-m2-nuke", nil)
	req.Header.Set("If-None-Match", etag)
	cached := httptest.NewRecorder()
	h.ServeHTTP(cached, req)
	if cached.Code != http.StatusNotModified {
		t.Fatalf("a matching ETag must answer 304, got %d", cached.Code)
	}
	for _, path := range []string{"/v1/pro-matches/2400001/nope", "/v1/pro-matches/x/vitality-vs-faze-m2-nuke", "/v1/pro-matches/2400001/..%2Fcommunity.db"} {
		if rec := do(t, h, http.MethodGet, path, nil); rec.Code != http.StatusNotFound {
			t.Fatalf("%s = %d, want 404", path, rec.Code)
		}
	}

	if err := os.Remove(gone); err != nil {
		t.Fatal(err)
	}
	if n, err := syncProCatalog(ctx, a.store, dir); err != nil || n != 1 {
		t.Fatalf("a removed file must leave the catalog: %d %v", n, err)
	}
	if _, err := syncProCatalog(ctx, a.store, filepath.Join(dir, "missing")); err == nil {
		t.Fatal("a missing folder must fail instead of emptying the catalog")
	}
}

func TestProMatchCatalogServesSeriesAndUpgradesOldCatalog(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "community.db")
	old, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := old.Exec(strings.Replace(schema, "series           TEXT NOT NULL DEFAULT '',", "", 1)); err != nil {
		t.Fatal(err)
	}
	if _, err := old.Exec(`INSERT INTO pro_match_maps (match_id, entry, file, size, mod_time, sha256, map, rounds, analysis_version, event, played_at, teams, synced_at)
		VALUES (2400003, 'alpha-vs-beta-m1-nuke.dem', 'f', 1, 'stamp', 's', 'de_nuke', 16, 9, 'E', '2026-01-01T00:00:00Z', '[]', '2026-01-01T00:00:00Z')`); err != nil {
		t.Fatal(err)
	}
	_ = old.Close()
	s, err := openStore(dbPath)
	if err != nil {
		t.Fatalf("an old catalog must open: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })
	var stamp string
	if err := s.db.QueryRow(`SELECT mod_time FROM pro_match_maps`).Scan(&stamp); err != nil || stamp != "" {
		t.Fatalf("the upgrade must clear the stamps so every analysis is read again: %q %v", stamp, err)
	}
	a := newAPI(s)
	h := a.routes()
	dir := t.TempDir()
	a.analysesDir = dir
	recent := time.Now().Add(-24 * time.Hour).UTC().Format(time.RFC3339)
	teams := []community.ProSidecarTeam{{ID: 1, Name: "Alpha"}, {ID: 2, Name: "Beta"}}
	withVeto := proHeader(2400003, "alpha-vs-beta-m1-nuke.dem", "de_nuke", recent, teams...)
	withVeto.Match.Maps, withVeto.Match.Format = []string{"Nuke", "Mirage", "Anubis"}, "bo3"
	withVeto.Match.Veto = []community.ProSidecarVeto{{Team: "Alpha", Action: "pick", Map: "Nuke"}, {Team: "Beta", Action: "pick", Map: "Mirage"}, {Action: "decider", Map: "Anubis"}}
	oldSidecar := proHeader(2400004, "alpha-vs-beta-m1-dust2.dem", "de_dust2", recent, teams...)
	oldSidecar.Match.Maps = []string{"Dust2"}
	writeProMap(t, dir, withVeto)
	writeProMap(t, dir, oldSidecar)
	if _, err := syncProCatalog(context.Background(), a.store, dir); err != nil {
		t.Fatal(err)
	}
	var m community.ProMatch
	if rec := do(t, h, http.MethodGet, "/v1/pro-matches/2400003", nil); json.Unmarshal(rec.Body.Bytes(), &m) != nil {
		t.Fatalf("match = %d %s", rec.Code, rec.Body)
	}
	if m.Series.Format != "bo3" || len(m.Series.Veto) != 3 || m.Series.Veto[1].TeamID != 2 || m.Series.Maps[0].PickedBy != 1 || !m.Series.Maps[2].Decider {
		t.Fatalf("series = %+v", m.Series)
	}
	m = community.ProMatch{}
	if rec := do(t, h, http.MethodGet, "/v1/pro-matches/2400004", nil); json.Unmarshal(rec.Body.Bytes(), &m) != nil {
		t.Fatalf("match = %d %s", rec.Code, rec.Body)
	}
	if m.Series.Format != "bo1" || len(m.Series.Veto) != 0 || !m.Series.Maps[0].Decider {
		t.Fatalf("a sidecar without a veto still gives the format and decider: %+v", m.Series)
	}
}

func TestProMatchesWithoutArchiveIsNotFound(t *testing.T) {
	_, h := testAPI(t)
	if rec := do(t, h, http.MethodGet, "/v1/pro-matches", nil); rec.Code != http.StatusNotFound {
		t.Fatalf("no archive folder = %d, want 404 so Studio shows its empty state", rec.Code)
	}
}

func TestProMatchCatalogJoinsPartsAndDropsFragments(t *testing.T) {
	a, h := testAPI(t)
	dir := t.TempDir()
	a.analysesDir = dir
	recent := time.Now().Add(-24 * time.Hour).UTC().Format(time.RFC3339)
	teams := []community.ProSidecarTeam{{ID: 1, Name: "Alpha"}, {ID: 2, Name: "Beta"}}
	first := proHeader(2400002, "alpha-vs-beta-m1-anubis.dem", "de_anubis", recent, teams...)
	first.Document.Rounds = proRounds(1, 9)
	second := proHeader(2400002, "alpha-vs-beta-m1-anubis-p2.dem", "de_anubis", recent, teams...)
	second.Document.Rounds = proRounds(9, 21)
	fragment := proHeader(2400002, "alpha-vs-beta-m2-nuke.dem", "de_nuke", recent, teams...)
	fragment.Document.Rounds = proRounds(1, 8)
	for _, hd := range []community.ProAnalysisHeader{first, second, fragment} {
		writeProMap(t, dir, hd)
	}
	if _, err := syncProCatalog(context.Background(), a.store, dir); err != nil {
		t.Fatal(err)
	}
	var m community.ProMatch
	rec := do(t, h, http.MethodGet, "/v1/pro-matches/2400002", nil)
	if rec.Code != http.StatusOK || json.Unmarshal(rec.Body.Bytes(), &m) != nil {
		t.Fatalf("match = %d %s", rec.Code, rec.Body)
	}
	if len(m.Maps) != 1 {
		t.Fatalf("the parts must join into one map and the fragment drop: %+v", m.Maps)
	}
	got := m.Maps[0]
	if got.Entry != "alpha-vs-beta-m1-anubis" || got.Rounds != 21 || len(got.Parts) != 2 || got.Parts[1].Entry != "alpha-vs-beta-m1-anubis-p2" || got.Parts[1].FirstRound != 9 {
		t.Fatalf("joined map = %+v", got)
	}
	if got.SHA256 == got.Parts[0].SHA256 || len(got.SHA256) != 64 {
		t.Fatalf("a joined map's hash must cover every part: %q", got.SHA256)
	}
	if rec := do(t, h, http.MethodGet, "/v1/pro-matches/2400999", nil); rec.Code != http.StatusNotFound {
		t.Fatalf("an unknown match = %d, want 404", rec.Code)
	}
	a.downloads.limit = 1
	if rec := do(t, h, http.MethodGet, "/v1/pro-matches/2400002/alpha-vs-beta-m1-anubis-p2", nil); rec.Code != http.StatusOK {
		t.Fatalf("a part file = %d", rec.Code)
	}
	if rec := do(t, h, http.MethodGet, "/v1/pro-matches/2400002/alpha-vs-beta-m1-anubis", nil); rec.Code != http.StatusTooManyRequests {
		t.Fatalf("a download past the hourly cap = %d, want 429", rec.Code)
	}
}
