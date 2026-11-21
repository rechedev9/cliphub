package main

import (
	"strings"
	"testing"

	"github.com/rechedev9/cliphub/internal/tuiclient"
)

func TestDescribeCaptureCheckSaysWhatCS2RecordsAndWarnsOnLowSpace(t *testing.T) {
	kept := describeCaptureCheck("job", fullDemoCapture{})
	if kept.NeedsCapture || !strings.Contains(kept.Summary, "solo se vuelve a montar") || kept.PlanHash != "" {
		t.Errorf("kept captures = %+v", kept)
	}
	capture := fullDemoCapture{Needed: true, MissingRounds: []int{3, 7}}
	capture.Space = &struct {
		FreeBytes   int64 `json:"free_bytes"`
		NeededBytes int64 `json:"needed_bytes"`
		Short       bool  `json:"short"`
	}{FreeBytes: 1 << 30, NeededBytes: 5 << 30, Short: true}
	out := describeCaptureCheck("job", capture)
	if !out.NeedsCapture || !strings.Contains(out.Summary, "2 rondas") || out.LowSpace == "" {
		t.Errorf("missing rounds = %+v", out)
	}
}

func TestDescribeFullDemoRoundsTimesEachRoundByTheDemoTickrate(t *testing.T) {
	plan := tuiclient.Plan{Demo: tuiclient.PlanDemo{Tickrate: 128}, Segments: []tuiclient.Segment{
		{Round: 1, TickStart: 1000, TickEnd: 1000 + 128*90, Kills: make([]tuiclient.Kill, 2)},
		{Round: 3, TickStart: 50000, TickEnd: 50000 + 128*45},
		{Round: 4, TickStart: 70000, TickEnd: 69000},
	}}
	got := describeFullDemoRounds("job", plan)
	if len(got.Rounds) != 3 || got.Rounds[0] != (fullDemoRoundOut{Round: 1, Seconds: 90, Kills: 2}) || got.Rounds[1].Round != 3 || got.Rounds[1].Seconds != 45 {
		t.Fatalf("rounds = %+v, want round 1 at 90 s with 2 kills and round 3 at 45 s", got.Rounds)
	}
	if got.Rounds[2].Seconds != 0 || got.TotalSeconds != 135 {
		t.Fatalf("a reversed window must count 0 s and the total 135 s; got %+v total %d", got.Rounds[2], got.TotalSeconds)
	}
	if def := describeFullDemoRounds("job", tuiclient.Plan{Segments: []tuiclient.Segment{{TickEnd: 64 * 10}}}); def.TotalSeconds != 10 {
		t.Fatalf("a plan without tickrate must time rounds at 64 ticks/s, got %d s", def.TotalSeconds)
	}
}
