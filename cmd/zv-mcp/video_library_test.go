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
	// A render failure never fails the job, so b failed its capture after the render and is listed too.
	if len(out.FailedJobs) != 2 || out.FailedJobs[0].JobID != b || out.FailedJobs[1].Reason != "CS2 closed" {
		t.Errorf("failed captures: %+v", out.FailedJobs)
	}
}

func TestDescribeStreamLibraryKeepsOnlyStudioVideos(t *testing.T) {
	const id = "44444444-4444-4444-8444-444444444444"
	streams := []tuiclient.StreamJob{
		{ID: id, RenderedOutputs: []tuiclient.StreamRenderedOutput{
			{Variant: "short-9x16", VideoURL: "/api/streams/" + id + "/videos/a.mp4"},
			{Variant: "short-9x16", VideoURL: "https://evil.example/a.mp4"},
		}},
		{ID: "../bad", RenderedOutputs: []tuiclient.StreamRenderedOutput{{Variant: "short-9x16", VideoURL: "/api/streams/x/videos/a.mp4"}}},
	}
	out := describeStreamLibrary(streams, "http://127.0.0.1:3000", true)
	if len(out) != 1 || out[0].StreamID != id || out[0].URL != "http://127.0.0.1:3000/api/streams/"+id+"/videos/a.mp4" {
		t.Errorf("stream videos: %+v", out)
	}
}
