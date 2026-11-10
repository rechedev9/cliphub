package main

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/rechedev9/cliphub/internal/community"
)

func TestSeriesBackfillRewritesHeadersAndTheCatalogRereadsThem(t *testing.T) {
	a, h := testAPI(t)
	dir, sidecars := t.TempDir(), t.TempDir()
	a.analysesDir = dir
	recent := time.Now().Add(-24 * time.Hour).UTC().Format(time.RFC3339)
	teams := []community.ProSidecarTeam{{ID: 1, Name: "Alpha"}, {ID: 2, Name: "Beta"}}
	hd := proHeader(2400005, "alpha-vs-beta-m1-nuke.dem", "de_nuke", recent, teams...)
	hd.Match.Maps = []string{"Nuke", "Mirage", "Anubis"}
	name := writeProMap(t, dir, hd)
	ctx := context.Background()
	if _, err := syncProCatalog(ctx, a.store, dir); err != nil {
		t.Fatal(err)
	}
	side := hd.Match
	side.Format, side.Veto = "bo3", []community.ProSidecarVeto{{Team: "Alpha", Action: "pick", Map: "Nuke"}, {Team: "Beta", Action: "pick", Map: "Mirage"}, {Action: "decider", Map: "Anubis"}}
	body, _ := json.Marshal(side)
	if err := os.WriteFile(filepath.Join(sidecars, "2400005.json"), body, 0o600); err != nil {
		t.Fatal(err)
	}
	// The stamp holds the modification time; move it so the rewrite is seen even within one clock tick.
	past := time.Now().Add(-time.Hour)
	if err := os.Chtimes(name, past, past); err != nil {
		t.Fatal(err)
	}
	if n, err := backfillSeries(dir, sidecars); err != nil || n != 1 {
		t.Fatalf("backfill changed %d: %v", n, err)
	}
	if n, err := backfillSeries(dir, sidecars); err != nil || n != 0 {
		t.Fatalf("a second backfill must change nothing: %d %v", n, err)
	}
	if n, err := syncProCatalog(ctx, a.store, dir); err != nil || n != 1 {
		t.Fatalf("the catalog must read the rewritten analysis: %d %v", n, err)
	}
	var m community.ProMatch
	if rec := do(t, h, http.MethodGet, "/v1/pro-matches/2400005", nil); json.Unmarshal(rec.Body.Bytes(), &m) != nil || len(m.Series.Veto) != 3 || m.Series.Maps[1].PickedBy != 2 {
		t.Fatalf("series = %+v", m.Series)
	}
}
