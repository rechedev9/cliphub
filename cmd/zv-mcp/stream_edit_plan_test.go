package main

import (
	"net/url"
	"strings"
	"testing"

	"github.com/rechedev9/cliphub/internal/tuiclient"
)

func TestGetStreamEditPlanReportsMomentsAndCrops(t *testing.T) {
	f := &fakeStream{planClips: `[{"id":"mine","start_seconds":95,"end_seconds":105,"title":"Mío‮","edit":{"speed":1.5,"text_overlays":[{"text":"GG","position_y":0.2}]}}]`}
	cs := f.connect(t)
	var out streamPlanOut
	callTool(t, cs, "get_stream_edit_plan", map[string]any{"id": testStreamID}, &out)
	if out.UpdatedAt != "2026-11-06T10:00:00Z" || out.FaceCropReviewed || out.SourceSeconds != 3600 || out.Music.Key != "song-7" {
		t.Fatalf("got %+v", out)
	}
	if len(out.Clips) != 1 || out.Clips[0].Title != "Mío" || out.Clips[0].Edit == nil || out.Clips[0].Edit.Speed != 1.5 || out.Clips[0].Edit.TextOverlays[0].Text != "GG" {
		t.Errorf("clips %+v", out.Clips)
	}
}

func TestEditStreamPlanReplacesOnlyTheGivenParts(t *testing.T) {
	f := &fakeStream{planClips: `[{"id":"clip-001","start_seconds":95,"end_seconds":105}]`}
	cs := f.connect(t)
	var out streamPlanOut
	callTool(t, cs, "edit_stream_plan", map[string]any{
		"id": testStreamID, "expected_updated_at": "2026-11-06T10:00:00Z",
		"clips": []map[string]any{
			{"start_seconds": 10, "end_seconds": 20, "title": "Nuevo", "edit": map[string]any{"speed": 2, "source_volume": 0}},
			{"id": "clip-001", "start_seconds": 96, "end_seconds": 104},
		},
		"streamer_banner": map[string]any{"nick": "zack", "platform": "kick"},
	}, &out)
	if q, _ := url.ParseQuery(f.putQuery); q.Get("expected_updated_at") != "2026-11-06T10:00:00Z" {
		t.Errorf("put query %q", f.putQuery)
	}
	if got := string(f.savedPlan["clips"]); got != `[{"id":"clip-002","start_seconds":10,"end_seconds":20,"title":"Nuevo","edit":{"speed":2,"source_volume":0}},{"id":"clip-001","start_seconds":96,"end_seconds":104}]` {
		t.Errorf("saved clips %s", got)
	}
	if string(f.savedPlan["music"]) != `{"key":"song-7","volume":0.4}` || string(f.savedPlan["streamer_banner"]) != `{"nick":"zack","platform":"kick"}` {
		t.Errorf("plan fields %v", f.savedPlan)
	}
	if out.UpdatedAt != "2026-11-06T10:05:00.123Z" || len(out.Clips) != 2 || out.StreamerBanner.Nick != "zack" {
		t.Errorf("returned %+v", out)
	}

	callTool(t, cs, "edit_stream_plan", map[string]any{"id": testStreamID, "expected_updated_at": "2026-11-06T10:00:00Z", "clips": []any{}, "music": map[string]any{}}, &out)
	if string(f.savedPlan["clips"]) != `[]` || string(f.savedPlan["music"]) != `{}` {
		t.Errorf("clearing saved %s and %s", f.savedPlan["clips"], f.savedPlan["music"])
	}
}

func TestEditStreamPlanRefusesBadInput(t *testing.T) {
	cs := (&fakeStream{}).connect(t)
	at := "2026-11-06T10:00:00Z"
	toolFails(t, cs, "edit_stream_plan", map[string]any{"id": testStreamID, "music": map[string]any{"key": "song-7"}}, "expected_updated_at")
	toolFails(t, cs, "edit_stream_plan", map[string]any{"id": testStreamID, "expected_updated_at": at}, "nothing to change")
	toolFails(t, cs, "edit_stream_plan", map[string]any{"id": testStreamID, "expected_updated_at": at, "music": map[string]any{"key": "song-9"}}, "valid ids: song-7, song-8")
	twice := []map[string]any{{"id": "a", "start_seconds": 1, "end_seconds": 2}, {"id": "a", "start_seconds": 3, "end_seconds": 4}}
	toolFails(t, cs, "edit_stream_plan", map[string]any{"id": testStreamID, "expected_updated_at": at, "clips": twice}, "listed twice")
}

func TestRenderStreamSummaryRendersTheChosenPlays(t *testing.T) {
	f := &fakeStream{scanStarts: 1}
	cs := f.connect(t)
	args := map[string]any{"id": testStreamID, "play_ids": []string{"play-001", "play-003"}}
	var ask streamRenderStartOut
	callTool(t, cs, "render_stream_summary", args, &ask)
	if !ask.NeedsApproval || !strings.Contains(ask.Summary, "con 2 jugadas seguidas (de 3 encontradas), unos 0:20") {
		t.Fatalf("summary %+v", ask)
	}
	args["approve"] = ask.ApprovalCode
	callTool(t, cs, "render_stream_summary", args, &streamRenderStartOut{})
	if f.summaryBody != `{"play_ids":["play-001","play-003"]}` {
		t.Errorf("summary body %q", f.summaryBody)
	}
	toolFails(t, cs, "render_stream_summary", map[string]any{"id": testStreamID, "play_ids": []string{"play-009"}}, "not a play of this stream")

	all := map[string]any{"id": testStreamID}
	callTool(t, cs, "render_stream_summary", all, &ask)
	all["approve"] = ask.ApprovalCode
	callTool(t, cs, "render_stream_summary", all, &streamRenderStartOut{})
	if f.summaryBody != "" {
		t.Errorf("every play sent %q, want no body", f.summaryBody)
	}
}

func TestStreamFilesListsTheDeliveryPack(t *testing.T) {
	job := tuiclient.StreamJob{ID: testStreamID, RenderedOutputs: []tuiclient.StreamRenderedOutput{
		{Variant: "streamer-landscape-16x9", ClipID: "play-002", Stale: true},
	}}
	renders := map[string]tuiclient.StreamRenderState{"streamer-landscape-16x9": {Delivery: []tuiclient.StreamDeliveryEntry{
		{Name: "play-002.mp4", Kind: "video"}, {Name: "cover.jpg", Kind: "cover"}, {Name: "../x", Kind: "plan"}, {Name: "metadata.txt", Kind: "metadata"},
	}}}
	files := streamFiles(job, renders, "http://127.0.0.1:3000/", true)
	if len(files) != 2 || files[0].Name != "cover.jpg" || !files[0].Stale || files[1].Kind != "metadata" {
		t.Fatalf("files %+v", files)
	}
	if files[0].URL != "http://127.0.0.1:3000/api/streams/"+testStreamID+"/renders/streamer-landscape-16x9/delivery/cover.jpg" {
		t.Errorf("url %s", files[0].URL)
	}
}
