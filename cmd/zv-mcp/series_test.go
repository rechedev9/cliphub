package main

import (
	"strings"
	"testing"

	"github.com/rechedev9/cliphub/internal/tuiclient"
)

func seriesFixture() []seriesMap {
	return []seriesMap{
		{Number: 1, JobID: "job-1", Moments: []tuiclient.Moment{
			{SegmentID: "seg-3", Map: "de_mirage", Round: 12, TickStart: 90000, Score: 9},
			{SegmentID: "seg-1", Map: "de_mirage", Round: 2, TickStart: 8000, Score: 4},
			{SegmentID: "seg-2", Map: "de_mirage", Round: 2, TickStart: 7000, Score: 6},
		}},
		{Number: 2, JobID: "job-2", Moments: []tuiclient.Moment{
			{SegmentID: "seg-1", Map: "de_inferno", Round: 1, TickStart: 500, Score: 9},
			{SegmentID: "seg-2", Map: "de_inferno", Round: 20, TickStart: 150000, Score: 2},
		}},
		{Number: 3, JobID: "job-3", Moments: []tuiclient.Moment{
			{SegmentID: "seg-1", Map: "de_nuke", Round: 5, TickStart: 30000, Score: 7},
		}},
	}
}

func playKeys(out []seriesMomentOut) string {
	keys := make([]string, 0, len(out))
	for _, m := range out {
		keys = append(keys, m.Map+"/"+m.SegmentID)
	}
	return strings.Join(keys, " ")
}

func TestConcatSeriesPlayOrderFollowsMapsThenRoundsThenTicks(t *testing.T) {
	out := concatSeries(seriesFixture(), orderPlay)
	want := "de_mirage/seg-2 de_mirage/seg-1 de_mirage/seg-3 de_inferno/seg-1 de_inferno/seg-2 de_nuke/seg-1"
	if got := playKeys(out); got != want {
		t.Errorf("play order:\n got %s\nwant %s", got, want)
	}
	if out[3].MapNumber != 2 || out[3].JobID != "job-2" {
		t.Errorf("a play lost its map: %+v", out[3])
	}
}

func TestConcatSeriesBestOrderBreaksTiesInPlayOrder(t *testing.T) {
	out := concatSeries(seriesFixture(), orderBest)
	want := "de_mirage/seg-3 de_inferno/seg-1 de_nuke/seg-1 de_mirage/seg-2 de_mirage/seg-1 de_inferno/seg-2"
	if got := playKeys(out); got != want {
		t.Errorf("best order:\n got %s\nwant %s", got, want)
	}
}

func TestSeriesPlayerMatchesBySteamIDNotName(t *testing.T) {
	m := tuiclient.LibraryMatch{Players: []tuiclient.LibraryMatchPlayer{
		{SteamID64: "76561198000000001", Name: "Rival"},
		{SteamID64: "76561198000000002", Name: "renamed"},
	}}
	p, ok := seriesPlayer(m, "76561198000000002")
	if !ok || p.Name != "renamed" {
		t.Errorf("got %+v, %v", p, ok)
	}
	if _, ok := seriesPlayer(m, "76561198000000003"); ok {
		t.Error("found a player who did not play this map")
	}
}

func TestCheckSeriesJobs(t *testing.T) {
	same := []tuiclient.Job{
		{TargetSteamID: "76561198000000002", DemoSHA256: "a"},
		{TargetSteamID: "76561198000000002", DemoSHA256: "b"},
	}
	if err := checkSeriesJobs(same); err != nil {
		t.Errorf("valid series refused: %v", err)
	}
	other := []tuiclient.Job{same[0], {TargetSteamID: "76561198000000009", DemoSHA256: "b"}}
	if err := checkSeriesJobs(other); err == nil || !strings.Contains(err.Error(), "map 2 follows another player") {
		t.Errorf("mixed players: got %v", err)
	}
	twice := []tuiclient.Job{same[0], same[0]}
	if err := checkSeriesJobs(twice); err == nil || !strings.Contains(err.Error(), "same demo") {
		t.Errorf("same demo twice: got %v", err)
	}
}

func TestCheckSeriesIDs(t *testing.T) {
	a, b := "0b9c1f9e-0f4d-4c7a-9a59-2f4f6c1d2e3a", "1c2d3e4f-5a6b-4c7d-8e9f-0a1b2c3d4e5f"
	if err := checkSeriesIDs("job_ids", []string{a, b}); err != nil {
		t.Errorf("valid ids refused: %v", err)
	}
	for _, ids := range [][]string{{a}, {a, a}, {a, "seg-1"}, {a, b, a, b, a, b}} {
		if checkSeriesIDs("job_ids", ids) == nil {
			t.Errorf("accepted %v", ids)
		}
	}
}
