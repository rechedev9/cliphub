package main

import (
	"testing"

	"github.com/rechedev9/cliphub/internal/tuiclient"
)

func TestDescribeImportedVideosDropsWhatStudioWouldNotServe(t *testing.T) {
	const id = "0b1f6c2e-8a4d-4f3a-9c55-2f6d1e7a9b10"
	videos := []tuiclient.ImportedVideo{
		{CloudJobID: id, Name: "short-1.mp4", Title: "Ace‮ en Mirage", LocalJobID: "not-a-job"},
		{CloudJobID: "../etc", Name: "short-2.mp4"},
		{CloudJobID: id, Name: "../short.mp4"},
	}
	out := describeImportedVideos(videos, "http://127.0.0.1:3000/")
	if len(out) != 1 {
		t.Fatalf("videos = %+v, want only the valid one", out)
	}
	v := out[0]
	if v.URL != "http://127.0.0.1:3000/api/imported-videos/"+id+"/short-1.mp4" {
		t.Errorf("url = %q", v.URL)
	}
	if v.Title != "Ace en Mirage" || v.JobID != "" {
		t.Errorf("title = %q, job_id = %q", v.Title, v.JobID)
	}
	if got := describeImportedVideos(videos[:1], ""); got[0].URL != "" {
		t.Errorf("url without a web origin = %q", got[0].URL)
	}
}
