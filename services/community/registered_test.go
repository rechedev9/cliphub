package main

import (
	"context"
	"encoding/json"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/rechedev9/cliphub/internal/community"
)

func TestRegisteredMatchesAnswersWhichIDsTheCatalogHolds(t *testing.T) {
	a, h := testAPI(t)
	dir := t.TempDir()
	a.analysesDir = dir
	old := time.Now().AddDate(-1, 0, 0).UTC().Format(time.RFC3339)
	teams := []community.ProSidecarTeam{{ID: 1, Name: "Alpha"}, {ID: 2, Name: "Beta"}}
	writeProMap(t, dir, proHeader(2400010, "alpha-vs-beta-m1-mirage.dem", "de_mirage", old, teams...))
	writeProMap(t, dir, proHeader(2400010, "alpha-vs-beta-m2-nuke.dem", "de_nuke", old, teams...))
	writeProMap(t, dir, proHeader(2400012, "alpha-vs-beta-m1-ancient.dem", "de_ancient", old, teams...))
	if _, err := syncProCatalog(context.Background(), a.store, dir); err != nil {
		t.Fatal(err)
	}
	rec := do(t, h, http.MethodGet, "/v1/pro-match-ids?ids=2400012,2400011,2400010,2400010", nil)
	var got registeredMatches
	if rec.Code != http.StatusOK || json.Unmarshal(rec.Body.Bytes(), &got) != nil {
		t.Fatalf("registered = %d %s", rec.Code, rec.Body)
	}
	if !slices.Equal(got.Registered, []int{2400010, 2400012}) {
		t.Fatalf("registered = %v, want the two catalog matches of any date, each once", got.Registered)
	}
	rec = do(t, h, http.MethodGet, "/v1/pro-match-ids?ids=2400011", nil)
	if rec.Code != http.StatusOK || strings.TrimSpace(rec.Body.String()) != `{"registered":[]}` {
		t.Fatalf("an unknown match = %d %s, want an empty list", rec.Code, rec.Body)
	}
}

func TestRegisteredMatchesRefusesBadIDsAndAMissingCatalog(t *testing.T) {
	a, h := testAPI(t)
	if rec := do(t, h, http.MethodGet, "/v1/pro-match-ids?ids=1", nil); rec.Code != http.StatusNotFound {
		t.Fatalf("no archive folder = %d, want 404 so the harvester falls back to its own state", rec.Code)
	}
	a.analysesDir = t.TempDir()
	many := make([]string, maxRegisteredQuery+1)
	for i := range many {
		many[i] = strconv.Itoa(i + 1)
	}
	for _, q := range []string{"", "?ids=", "?ids=abc", "?ids=1,-2", "?ids=0", "?ids=1,,2", "?ids=" + strings.Join(many, ",")} {
		if rec := do(t, h, http.MethodGet, "/v1/pro-match-ids"+q, nil); rec.Code != http.StatusBadRequest {
			t.Fatalf("%q = %d, want 400", q, rec.Code)
		}
	}
}
