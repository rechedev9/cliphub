package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/rechedev9/cliphub/internal/community"
	"github.com/rechedev9/cliphub/internal/tacticalplan"
)

func TestGlobalMetricsPlacesOwnValuesAndFlipsDeaths(t *testing.T) {
	values := []float64{}
	for i := range community.MinRankingInstalls + 1 {
		values = append(values, float64(i)/float64(community.MinRankingInstalls))
	}
	q := community.Quantiles(values)
	dist := community.RankingsResponse{Metrics: map[string]community.RankingDistribution{
		tacticalplan.MetricKPR: {Installs: len(values), Quantiles: q},
		tacticalplan.MetricDPR: {Installs: len(values), Quantiles: q},
		tacticalplan.MetricADR: {Installs: 3},
	}}
	own := tacticalplan.PlayerLine{Rounds: 100, Matches: 4, Values: map[string]float64{
		tacticalplan.MetricKPR: 0.75, tacticalplan.MetricDPR: 0.75, tacticalplan.MetricADR: 80,
	}}
	rows := map[string]globalMetric{}
	for _, row := range globalMetrics(dist, own) {
		rows[row.Metric] = row
	}
	if len(rows) != len(tacticalplan.ReferenceMetrics) {
		t.Fatalf("rows = %d", len(rows))
	}
	if p := rows[tacticalplan.MetricKPR].Percentile; p == nil || *p != 75 {
		t.Fatalf("kpr percentile = %v", p)
	}
	if p := rows[tacticalplan.MetricDPR].Percentile; p == nil || *p != 25 {
		t.Fatalf("dying more than 75%% of installs beats only 25%%, got %v", p)
	}
	adr := rows[tacticalplan.MetricADR]
	if adr.Value == nil || adr.Percentile != nil || adr.Median != nil || adr.Installs != 3 {
		t.Fatalf("adr under the install threshold = %+v", adr)
	}
	few := globalMetrics(dist, tacticalplan.PlayerLine{Rounds: community.MinRankingRounds - 1, Values: own.Values})
	if few[0].Value != nil {
		t.Fatal("a player under the round minimum was never submitted, so shows no value")
	}
}

func TestCommunityReferenceAndRankingsOptIn(t *testing.T) {
	ref := tacticalplan.DefaultProReference()
	ref.ID, ref.Description, ref.UpdatedAt = "server-x", "283 mapas profesionales de IEM Cologne Major 2026.", "2026-11-06T09:00:00Z"
	body, err := json.Marshal(ref)
	if err != nil {
		t.Fatal(err)
	}
	submits := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/pro-reference":
			_, _ = w.Write(body)
		case "/v1/rankings/submit":
			submits++
			_, _ = w.Write([]byte(`{"maps":0}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()
	client, err := community.NewClient(srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	cache := community.OpenProReferenceCache(dir, client)
	if err := cache.Refresh(context.Background()); err != nil {
		t.Fatal(err)
	}
	env := newLibraryTestEnv(t)
	WithCommunity(CommunityDeps{Client: client, Reference: cache, Settings: community.NewSettingsStore(dir)})(env.h)

	var base baselineResponse
	env.json(http.MethodGet, "/api/library/baseline?scope=pro&map=de_nuke", nil, http.StatusOK, &base)
	if base.Source != referenceSourceServer || base.UpdatedAt != ref.UpdatedAt || base.Description != ref.Description {
		t.Fatalf("pro baseline = %+v, want the server reference", base)
	}
	var settings communitySettingsResponse
	env.json(http.MethodGet, "/api/library/community/settings", nil, http.StatusOK, &settings)
	if settings.RankingsEnabled || !settings.ServerEnabled || settings.SteamIDSet {
		t.Fatalf("default settings = %+v, want rankings off", settings)
	}
	var global globalRankingsResponse
	env.json(http.MethodGet, "/api/library/community/rankings", nil, http.StatusOK, &global)
	if global.RankingsEnabled || len(global.Metrics) != 0 {
		t.Fatalf("rankings while off = %+v, want no server call", global)
	}
	env.json(http.MethodPut, "/api/library/community/settings", map[string]bool{"rankings_enabled": true}, http.StatusOK, &settings)
	if !settings.RankingsEnabled || settings.SubmitError != codeSteamIDRequired || submits != 0 {
		t.Fatalf("enable without a SteamID = %+v, %d submits", settings, submits)
	}
}
