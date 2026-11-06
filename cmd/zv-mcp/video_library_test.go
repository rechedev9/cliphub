package main

import (
	"testing"

	"github.com/rechedev9/cliphub/internal/tuiclient"
)

func TestNeedsRecaptureMatchesTheUI(t *testing.T) {
	for reason, want := range map[string]bool{
		"recording_not_reusable: clip missing":                  true,
		"render: capture input fingerprint does not match plan": true,
		"ffmpeg exited with status 1":                           false,
		"":                                                      false,
	} {
		if got := needsRecapture(reason); got != want {
			t.Errorf("needsRecapture(%q) = %v, want %v", reason, got, want)
		}
	}
}

func TestSameMixUsesTheDefaults(t *testing.T) {
	seventy, half := 0.7, 0.5
	cases := []struct {
		a, b *tuiclient.ShortMusic
		want bool
	}{
		{nil, nil, true},
		{nil, &tuiclient.ShortMusic{Key: "x"}, false},
		{&tuiclient.ShortMusic{Key: "x", Volume: 1, GameVolume: &seventy}, &tuiclient.ShortMusic{Key: "x"}, true},
		{&tuiclient.ShortMusic{Key: "x"}, &tuiclient.ShortMusic{Key: "x", GameVolume: &half}, false},
		{&tuiclient.ShortMusic{Key: "x"}, &tuiclient.ShortMusic{Key: "y"}, false},
	}
	for i, tc := range cases {
		if got := sameMix(tc.a, tc.b); got != tc.want {
			t.Errorf("case %d: sameMix = %v, want %v", i, got, tc.want)
		}
	}
}

func TestDescribeVideoLibrarySplitsReadyRunningAndFailed(t *testing.T) {
	const a, b, c = "11111111-1111-4111-8111-111111111111", "22222222-2222-4222-8222-222222222222", "33333333-3333-4333-8333-333333333333"
	status := &tuiclient.JobStatus{Status: "recorded"}
	ready := &tuiclient.RenderVideos{RenderVariantState: tuiclient.RenderVariantState{Status: "ready"}, Videos: []string{"demo-compilation", "../bad"}}
	failedRender := &tuiclient.RenderVideos{RenderVariantState: tuiclient.RenderVariantState{Status: "failed", Error: "ffmpeg\x00 failed"}}
	rows := []tuiclient.BatchStatusRow{
		{JobID: a, Variant: "viral-60-clean", Job: status, Render: ready},
		{JobID: a, Variant: "gameplay-pov-60", Job: status},
		{JobID: b, Variant: "viral-60-clean", Job: &tuiclient.JobStatus{Status: "failed"}, Render: failedRender},
	}
	failed := []tuiclient.Job{{ID: b, Status: "failed"}, {ID: c, Status: "failed", FailureReason: "CS2 closed"}}
	out := describeVideoLibrary(rows, failed, "http://127.0.0.1:3000")
	if len(out.Videos) != 1 || out.Videos[0].Name != "demo-compilation" || out.Videos[0].URL == "" {
		t.Errorf("videos: %+v", out.Videos)
	}
	if len(out.OtherRenders) != 1 || out.OtherRenders[0].JobID != b || out.OtherRenders[0].Error != "ffmpeg failed" {
		t.Errorf("other renders: %+v", out.OtherRenders)
	}
	if len(out.FailedJobs) != 1 || out.FailedJobs[0].JobID != c || out.FailedJobs[0].Reason != "CS2 closed" {
		t.Errorf("failed captures: %+v", out.FailedJobs)
	}
}
