package main

import (
	"strings"
	"testing"

	"github.com/rechedev9/cliphub/internal/tuiclient"
)

func TestCleanTextKeepsLinesAndStripsControls(t *testing.T) {
	got := cleanText("Line one\r\nLine\x00 two‮\n#CS2 ", 100)
	if got != "Line one\nLine two\n#CS2" {
		t.Errorf("got %q", got)
	}
	runes := []rune(cleanText(strings.Repeat("x", 300), 100))
	if len(runes) != 101 || runes[100] != '…' {
		t.Errorf("not capped: %d runes", len(runes))
	}
}

func TestVideoURLCarriesNoToken(t *testing.T) {
	got := videoURL("http://127.0.0.1:3000/", "job 1", "viral-60-clean", "a_b")
	want := "http://127.0.0.1:3000/api/demos/job%201/renders/viral-60-clean/videos/a_b"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestDescribePublishCopy(t *testing.T) {
	var pa tuiclient.PublishAssistant
	pa.Metadata.Title = "s1mple 5K\x00 on Mirage"
	pa.Metadata.Description = "Player: s1mple\nMap: Mirage"
	pa.Metadata.Tags = []string{"CS2", "s1mple\n"}
	pa.Recommendations = []tuiclient.PublishRecommendation{{Title: pa.Metadata.Title}, {Title: "Ace on Mirage"}}
	out := describePublishCopy(pa)
	if out.Title != "s1mple 5K on Mirage" || out.Description != "Player: s1mple\nMap: Mirage" {
		t.Errorf("unexpected copy: %+v", out)
	}
	if len(out.Tags) != 2 || out.Tags[1] != "s1mple" {
		t.Errorf("tags: %v", out.Tags)
	}
	if len(out.AlternativeTitles) != 1 || out.AlternativeTitles[0] != "Ace on Mirage" {
		t.Errorf("alternatives: %v", out.AlternativeTitles)
	}
}

func TestRenderReady(t *testing.T) {
	for status, want := range map[string]bool{"ready": true, "review_required": true, "rendering": false, "queued": false, "failed": false} {
		if renderReady(status) != want {
			t.Errorf("renderReady(%q) != %v", status, want)
		}
	}
}

func TestDescribePublishCopyKeepsRecommendationsAndBestTimes(t *testing.T) {
	var pa tuiclient.PublishAssistant
	pa.Metadata.Title = "Ace on Mirage"
	pa.Recommendations = []tuiclient.PublishRecommendation{
		{Title: "Ace on Mirage", Score: 0.9, Rationale: "names the map"},
		{Title: "1v5\x00 clutch", Description: "Watch", Tags: []string{"clutch"}, Score: 0.7},
	}
	pa.Schedule.Days = []tuiclient.PublishScheduleDay{
		{Date: "2026-11-07", Weekday: "sábado", Slots: []tuiclient.PublishScheduleSlot{{LocalTime: "12:00", Score: 0.4}, {LocalTime: "19:00", Score: 0.95}}},
		{Date: "2026-11-08", Weekday: "domingo", Slots: []tuiclient.PublishScheduleSlot{{LocalTime: "18:00", Score: 0.8}}},
	}
	pa.Trends.Reason = "sin conexión"
	out := describePublishCopy(pa)
	if len(out.Recommendations) != 2 || out.Recommendations[1].Title != "1v5 clutch" || out.Recommendations[0].Rationale != "names the map" {
		t.Errorf("recommendations: %+v", out.Recommendations)
	}
	if len(out.AlternativeTitles) != 1 || out.AlternativeTitles[0] != "1v5 clutch" {
		t.Errorf("alternative titles: %v", out.AlternativeTitles)
	}
	if len(out.BestTimes) != 3 || out.BestTimes[0].LocalTime != "19:00" || out.BestTimes[1].Weekday != "domingo" {
		t.Errorf("best times: %+v", out.BestTimes)
	}
	if out.TrendingTerms != nil || out.TrendsNote != "sin conexión" {
		t.Errorf("trends: %v %q", out.TrendingTerms, out.TrendsNote)
	}
}
