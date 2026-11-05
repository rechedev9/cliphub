package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"slices"
	"testing"
	"time"

	"github.com/rechedev9/cliphub/internal/tuiclient"
)

func TestMatchesBetweenKeepsWholeDaysOldestFirst(t *testing.T) {
	day := func(d, h int) time.Time { return time.Date(2026, 10, d, h, 0, 0, 0, time.UTC) }
	all := []tuiclient.LibraryMatch{
		{ID: "late", PlayedAt: day(31, 23)},
		{ID: "undated"},
		{ID: "first", PlayedAt: day(20, 1)},
		{ID: "before", PlayedAt: day(19, 23)},
		{ID: "after", PlayedAt: time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)},
	}
	from, to, err := parseDateRange("2026-10-20", "2026-10-31")
	if err != nil {
		t.Fatal(err)
	}
	if got := matchIDs(matchesBetween(all, from, to)); !slices.Equal(got, []string{"first", "late"}) {
		t.Fatalf("in range = %v, want [first late]", got)
	}
	if got := matchIDs(matchesBetween(all, time.Time{}, time.Time{})); len(got) != len(all) || got[0] != "undated" {
		t.Fatalf("no range = %v, want every match with the undated one first", got)
	}
}

func TestParseDateRangeRejectsBadInput(t *testing.T) {
	for _, tc := range [][2]string{{"31/10/2026", ""}, {"", "yesterday"}, {"2026-11-02", "2026-11-01"}} {
		if _, _, err := parseDateRange(tc[0], tc[1]); err == nil {
			t.Errorf("parseDateRange(%q, %q) accepted", tc[0], tc[1])
		}
	}
	if _, _, err := parseDateRange("2026-11-01", "2026-11-01"); err != nil {
		t.Errorf("a single day must be a valid range: %v", err)
	}
}

func TestSortFoundPlaysByScoreOrDate(t *testing.T) {
	older := tuiclient.LibraryMatch{ID: "a", PlayedAt: time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)}
	newer := tuiclient.LibraryMatch{ID: "b", PlayedAt: time.Date(2026, 10, 2, 0, 0, 0, 0, time.UTC)}
	plays := []foundPlay{
		{match: newer, m: tuiclient.Moment{SegmentID: "new-r3", Round: 3, Score: 0.9}},
		{match: older, m: tuiclient.Moment{SegmentID: "old-r7", Round: 7, Score: 0.5}},
		{match: older, m: tuiclient.Moment{SegmentID: "old-r2", Round: 2, Score: 0.5}},
	}
	sortFoundPlays(plays, orderBest)
	if got := segmentIDs(plays); !slices.Equal(got, []string{"new-r3", "old-r2", "old-r7"}) {
		t.Fatalf("best = %v", got)
	}
	sortFoundPlays(plays, orderDate)
	if got := segmentIDs(plays); !slices.Equal(got, []string{"old-r2", "old-r7", "new-r3"}) {
		t.Fatalf("date = %v", got)
	}
}

func TestMomentFilterQueryNormalizes(t *testing.T) {
	q, err := momentFilterIn{Weapon: "AWP", Kinds: []string{"Clutch"}, Side: "t"}.query("job")
	if err != nil {
		t.Fatal(err)
	}
	if q.JobID != "job" || q.Weapon != "awp" || q.Side != "T" || !slices.Equal(q.Kinds, []string{"clutch"}) {
		t.Fatalf("query = %+v", q)
	}
	if _, err := (momentFilterIn{Side: "both"}).query("job"); err == nil {
		t.Fatal("side both accepted")
	}
}

func matchIDs(ms []tuiclient.LibraryMatch) []string {
	out := make([]string, 0, len(ms))
	for _, m := range ms {
		out = append(out, m.ID)
	}
	return out
}

func segmentIDs(plays []foundPlay) []string {
	out := make([]string, 0, len(plays))
	for _, p := range plays {
		out = append(out, p.m.SegmentID)
	}
	return out
}

func TestFindSkipsAMatchWhoseDemoIsGone(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/library/demos/gone/jobs", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusConflict)
		_, _ = w.Write([]byte(`{"code":"job_demo_missing","error":"the library demo file is no longer stored"}`))
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	search := matchSearch{c: tuiclient.New(tuiclient.Config{BaseURL: srv.URL, HTTPClient: srv.Client()}), in: findMomentsIn{SteamID: "76561198000000000"}}

	found, plays, err := search.find(context.Background(), tuiclient.LibraryMatch{ID: "m1", DemoID: "gone", Map: "de_mirage"})
	if err != nil {
		t.Fatalf("a missing demo must not fail the search: %v", err)
	}
	if found.Skipped == "" || found.Pending || len(plays) != 0 {
		t.Fatalf("found = %+v, plays = %d; want a skipped match", found, len(plays))
	}
}
