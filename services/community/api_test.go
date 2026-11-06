package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rechedev9/cliphub/internal/community"
	"github.com/rechedev9/cliphub/internal/tacticalplan"
)

func testAPI(t *testing.T) (*api, http.Handler) {
	t.Helper()
	s, err := openStore(filepath.Join(t.TempDir(), "community.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })
	a := newAPI(s)
	return a, a.routes()
}

func installID(i int) string {
	return fmt.Sprintf("00000000-0000-4000-8000-%012d", i)
}

func submission(id string, kpr float64) community.RankingSubmission {
	return community.RankingSubmission{InstallID: id, Maps: []community.RankingMap{
		{Map: "de_mirage", Rounds: 120, Matches: 5, Values: map[string]float64{tacticalplan.MetricKPR: kpr, tacticalplan.MetricDPR: 0.7}},
		{Map: community.AllMaps, Rounds: 300, Matches: 12, Values: map[string]float64{tacticalplan.MetricKPR: kpr}},
	}}
}

func do(t *testing.T, h http.Handler, method, path string, body any) *httptest.ResponseRecorder {
	t.Helper()
	var reader *bytes.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			t.Fatal(err)
		}
		reader = bytes.NewReader(b)
	} else {
		reader = bytes.NewReader(nil)
	}
	req := httptest.NewRequest(method, path, reader)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func TestValidateSubmissionRejectsBogusValues(t *testing.T) {
	good := submission(installID(1), 0.8)
	if err := validateSubmission(good); err != nil {
		t.Fatalf("valid submission rejected: %v", err)
	}
	cases := map[string]func(*community.RankingSubmission){
		"telemetry style id":  func(s *community.RankingSubmission) { s.InstallID = "CH-1234" },
		"impossible kpr":      func(s *community.RankingSubmission) { s.Maps[0].Values[tacticalplan.MetricKPR] = 9 },
		"unknown metric":      func(s *community.RankingSubmission) { s.Maps[0].Values["rating"] = 1 },
		"few rounds":          func(s *community.RankingSubmission) { s.Maps[0].Rounds = 5 },
		"rounds per match":    func(s *community.RankingSubmission) { s.Maps[0].Matches = 1; s.Maps[0].Rounds = 500 },
		"repeated map":        func(s *community.RankingSubmission) { s.Maps[1].Map = "de_mirage" },
		"path as map":         func(s *community.RankingSubmission) { s.Maps[0].Map = "C:/demos/x" },
		"more than one death": func(s *community.RankingSubmission) { s.Maps[0].Values[tacticalplan.MetricDPR] = 1.2 },
	}
	for name, mutate := range cases {
		s := submission(installID(1), 0.8)
		mutate(&s)
		if err := validateSubmission(s); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
}

func TestRankingsNeedEnoughInstallsAndResubmitReplaces(t *testing.T) {
	_, h := testAPI(t)
	for i := range community.MinRankingInstalls - 1 {
		if rec := do(t, h, http.MethodPost, "/v1/rankings/submit", submission(installID(i), 0.5+float64(i)/100)); rec.Code != http.StatusOK {
			t.Fatalf("submit %d = %d %s", i, rec.Code, rec.Body)
		}
	}
	var got community.RankingsResponse
	rec := do(t, h, http.MethodGet, "/v1/rankings?map=de_mirage", nil)
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if d := got.Metrics[tacticalplan.MetricKPR]; d.Installs != community.MinRankingInstalls-1 || d.Quantiles != nil {
		t.Fatalf("below the threshold = %+v", d)
	}
	// The same install twice still counts once.
	do(t, h, http.MethodPost, "/v1/rankings/submit", submission(installID(99), 0.9))
	do(t, h, http.MethodPost, "/v1/rankings/submit", submission(installID(99), 0.95))
	rec = do(t, h, http.MethodGet, "/v1/rankings?map=de_mirage&metric=kpr", nil)
	got = community.RankingsResponse{}
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	d := got.Metrics[tacticalplan.MetricKPR]
	if d.Installs != community.MinRankingInstalls || len(d.Quantiles) != community.QuantilePoints || d.Quantiles[100] != 0.95 || len(got.Metrics) != 1 {
		t.Fatalf("at the threshold = %+v (%d metrics)", d, len(got.Metrics))
	}
	if rec := do(t, h, http.MethodDelete, "/v1/rankings/installs/"+installID(99), nil); rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"deleted":2`) {
		t.Fatalf("delete = %d %s", rec.Code, rec.Body)
	}
	rec = do(t, h, http.MethodGet, "/v1/rankings?map=de_mirage&metric=kpr", nil)
	if !strings.Contains(rec.Body.String(), fmt.Sprintf(`"installs":%d}`, community.MinRankingInstalls-1)) {
		t.Fatalf("after delete = %s", rec.Body)
	}
}

func TestSubmitLimitsPerInstallAndPerIP(t *testing.T) {
	a, h := testAPI(t)
	for i := range maxSubmitsPerDay {
		if rec := do(t, h, http.MethodPost, "/v1/rankings/submit", submission(installID(1), 0.8)); rec.Code != http.StatusOK {
			t.Fatalf("submit %d = %d", i, rec.Code)
		}
	}
	if rec := do(t, h, http.MethodPost, "/v1/rankings/submit", submission(installID(1), 0.8)); rec.Code != http.StatusTooManyRequests {
		t.Fatalf("submit past the daily cap = %d", rec.Code)
	}
	for range writesPerHour {
		a.limiter.allow("198.51.100.1")
	}
	if a.limiter.allow("198.51.100.1") || !a.limiter.allow("198.51.100.2") {
		t.Fatal("the IP window must stop one client and not another")
	}
	big := strings.Repeat("x", maxSubmitBytes+1)
	if rec := do(t, h, http.MethodPost, "/v1/rankings/submit", map[string]string{"install_id": big}); rec.Code != http.StatusBadRequest {
		t.Fatalf("oversized body = %d", rec.Code)
	}
}

func TestClientIPTrustsOnlyTheLoopbackProxy(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "127.0.0.1:5000"
	req.Header.Set("X-Forwarded-For", "203.0.113.9")
	if got := clientIP(req); got != "203.0.113.9" {
		t.Fatalf("behind the proxy = %s", got)
	}
	req.RemoteAddr = "198.51.100.4:5000"
	if got := clientIP(req); got != "198.51.100.4" {
		t.Fatalf("a direct client must not pick its own IP, got %s", got)
	}
}

func TestProReferenceServesIndexMapAndETag(t *testing.T) {
	a, h := testAPI(t)
	if rec := do(t, h, http.MethodGet, "/v1/pro-reference", nil); rec.Code != http.StatusNotFound {
		t.Fatalf("before any index = %d", rec.Code)
	}
	ctx := context.Background()
	for i, mapName := range []string{"de_nuke", "de_nuke", "de_inferno"} {
		d := pendingDemo{dataset: "o/d", path: fmt.Sprintf("iem_cologne_major_2026/%d/x.dem", i), sha256: "s", size: 1, event: eventName("iem_cologne_major_2026/x")}
		if err := a.store.markIndexed(ctx, d, demoSummary(mapName)); err != nil {
			t.Fatal(err)
		}
	}
	index, err := rebuildReference(ctx, a.store)
	if err != nil {
		t.Fatal(err)
	}
	if index.Matches != 3 || len(index.Maps) != 2 || index.Maps[0].Map != "de_nuke" || !strings.Contains(index.Description, "IEM Cologne Major 2026") {
		t.Fatalf("index = %+v", index)
	}
	rec := do(t, h, http.MethodGet, "/v1/pro-reference?map=all", nil)
	if _, err := tacticalplan.DecodeProReference(rec.Body.Bytes()); err != nil {
		t.Fatalf("map=all does not decode as a reference: %v", err)
	}
	req := httptest.NewRequest(http.MethodGet, "/v1/pro-reference?map=de_nuke", nil)
	first := do(t, h, http.MethodGet, "/v1/pro-reference?map=de_nuke", nil)
	req.Header.Set("If-None-Match", first.Header().Get("ETag"))
	again := httptest.NewRecorder()
	h.ServeHTTP(again, req)
	if again.Code != http.StatusNotModified {
		t.Fatalf("same etag = %d", again.Code)
	}
	if rec := do(t, h, http.MethodGet, "/v1/pro-reference?map=../x", nil); rec.Code != http.StatusBadRequest {
		t.Fatalf("bad map = %d", rec.Code)
	}
}

func demoSummary(mapName string) tacticalplan.ProDemoSummary {
	d := tacticalplan.ProDemoSummary{Match: tacticalplan.ProReferenceMatch{Map: mapName, Teams: []string{"A", "B"}, Rounds: 20}}
	for i := range 10 {
		d.Players = append(d.Players, tacticalplan.ProDemoLine{
			SteamID64: fmt.Sprint(76561190000000000 + i), Name: fmt.Sprint("p", i), Team: "A", Rounds: 20, Kills: 10 + i, Deaths: 12 + i%3,
			TradedDeaths: 2, FlashAssists: i % 3, UtilityDamage: 40 + i, WPA: float64(i) / 20,
			Box: tacticalplan.BoxScore{Damage: 1500 + 40*i, KASTRounds: 14 + i%4, OpeningKills: 1 + i%3, OpeningDeaths: 2, UtilityRounds: 20},
			RoundSums: map[string]tacticalplan.RoundSums{
				tacticalplan.MetricKPR: {N: 20, Sum: float64(10 + i), SumSq: float64(12 + i)},
				tacticalplan.MetricADR: {N: 20, Sum: 1500, SumSq: 200000},
			},
		})
	}
	return d
}

func TestEventNameReadsTheDatasetFolder(t *testing.T) {
	for in, want := range map[string]string{
		"iem_cologne_major_2026/1-a/x.dem": "IEM Cologne Major 2026",
		"cac2026/2-b/y.dem":                "CAC 2026",
	} {
		if got := eventName(in); got != want {
			t.Errorf("eventName(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestRebuildJoinsRestartPartsAndSkipsFragments(t *testing.T) {
	a, _ := testAPI(t)
	ctx := context.Background()
	add := func(path string, rounds, first int) {
		d := demoSummary("de_ancient")
		d.Match.Rounds, d.FirstRound, d.LastRound = rounds, first, first+rounds-1
		if err := a.store.markIndexed(ctx, pendingDemo{dataset: "o/d", path: path, sha256: "s", size: 1, event: "IEM Cologne Major 2026"}, d); err != nil {
			t.Fatal(err)
		}
	}
	add("ev/1-monte-vs-legacy/monte-vs-legacy-ancient-p2.dem", 3, 1)
	add("ev/1-monte-vs-legacy/monte-vs-legacy-ancient-p3.dem", 21, 4)
	add("ev/2-b8-vs-gl/b8-vs-gl-m1-ancient-p2.dem", 1, 1)
	add("ev/3-a-vs-b/a-vs-b-ancient.dem", 20, 1)
	index, err := rebuildReference(ctx, a.store)
	if err != nil {
		t.Fatal(err)
	}
	if index.Matches != 2 || index.Rounds != 44 || !strings.HasPrefix(index.Description, "2 mapas") {
		t.Fatalf("index = %d maps, %d rounds, %q; want the parts as one map and no fragment", index.Matches, index.Rounds, index.Description)
	}
	if got := mapKey("x/a-vs-b-m1-anubis-p2.dem"); got != "x/a-vs-b-m1-anubis" || mapKey("x/a-overpass(1).dem") != "x/a-overpass" {
		t.Fatalf("mapKey = %q", got)
	}
}
