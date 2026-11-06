package main

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/rechedev9/cliphub/internal/tuiclient"
)

const testStreamID = "3d2c1b0a-9f8e-4d7c-8b6a-5f4e3d2c1b0a"

// fakeStream is an orchestrator holding one ready VOD with three plays and a
// plan whose facecam crop was never checked.
type fakeStream struct {
	mu         sync.Mutex
	scanStarts int
	savedPlan  map[string]json.RawMessage
	renderBody string
}

func (f *fakeStream) connect(t *testing.T) *mcp.ClientSession {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte("ok")) })
	mux.HandleFunc("GET /api/stream-jobs/"+testStreamID, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"id":"` + testStreamID + `","status":"ready","title":"ESL‮ final","source_kind":"vod","probe":{"duration_seconds":3600}}`))
	})
	mux.HandleFunc("GET /api/stream-jobs/"+testStreamID+"/plays", func(w http.ResponseWriter, _ *http.Request) {
		f.mu.Lock()
		started := f.scanStarts > 0
		f.mu.Unlock()
		if !started {
			_, _ = w.Write([]byte(`{"status":"idle","progress":0,"plays":[]}`))
			return
		}
		_, _ = w.Write([]byte(`{"status":"done","progress":1,"plays":[
			{"id":"play-001","label":"Doble","start_seconds":100,"end_seconds":112,"kill_seconds":[104,106],"score":40},
			{"id":"play-002","label":"Ace","start_seconds":900,"end_seconds":930,"kill_seconds":[905,910,915,920,925],"score":95},
			{"id":"play-003","label":"Baja","start_seconds":50,"end_seconds":58,"kill_seconds":[54],"score":40}]}`))
	})
	mux.HandleFunc("POST /api/stream-jobs/"+testStreamID+"/plays", func(w http.ResponseWriter, _ *http.Request) {
		f.mu.Lock()
		f.scanStarts++
		f.mu.Unlock()
		w.WriteHeader(http.StatusAccepted)
		_, _ = w.Write([]byte(`{"status":"running","progress":0,"plays":[]}`))
	})
	mux.HandleFunc("GET /api/stream-variants", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"default":"streamer-vertical-stack-40-60","variants":[
			{"name":"streamer-vertical-stack-40-60","label":"Facecam 40 / Gameplay 60","default":true,"output_width":1080,"face_output_height":768,"game_output_height":1152},
			{"name":"streamer-landscape-16x9","label":"YouTube Landscape 16:9","full_frame":true,"output_width":1920,"game_output_height":1080}]}`))
	})
	mux.HandleFunc("GET /api/stream-jobs/"+testStreamID+"/edit-plan", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"schema_version":"1.1","variant":"streamer-vertical-stack-40-60","clips":[],
			"music":{"key":"song-7","volume":0.4},"updated_at":"2026-11-06T10:00:00Z"}`))
	})
	mux.HandleFunc("PUT /api/stream-jobs/"+testStreamID+"/edit-plan", func(w http.ResponseWriter, r *http.Request) {
		var plan map[string]json.RawMessage
		if err := json.NewDecoder(r.Body).Decode(&plan); err != nil {
			http.Error(w, `{"error":"bad plan"}`, http.StatusBadRequest)
			return
		}
		f.mu.Lock()
		f.savedPlan = plan
		f.mu.Unlock()
		plan["updated_at"] = json.RawMessage(`"2026-11-06T10:05:00.123Z"`)
		_ = json.NewEncoder(w).Encode(plan)
	})
	mux.HandleFunc("POST /api/stream-jobs/"+testStreamID+"/renders/{variant}", func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		f.mu.Lock()
		f.renderBody = r.PathValue("variant") + " " + string(body)
		f.mu.Unlock()
		w.WriteHeader(http.StatusAccepted)
		_, _ = w.Write([]byte(`{"id":"` + testStreamID + `","variant":"` + r.PathValue("variant") + `","status":"queued"}`))
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	s := closedStudio(t)
	t.Setenv("ORCHESTRATOR_URL", srv.URL)

	ctx := context.Background()
	server := mcp.NewServer(&mcp.Implementation{Name: "test", Version: "0"}, nil)
	registerStreamTools(server, s)
	serverT, clientT := mcp.NewInMemoryTransports()
	ss, err := server.Connect(ctx, serverT, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = ss.Close() })
	cs, err := mcp.NewClient(&mcp.Implementation{Name: "client", Version: "0"}, nil).Connect(ctx, clientT, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = cs.Close() })
	return cs
}

func TestFindStreamPlaysStartsTheScanAndRanksPlays(t *testing.T) {
	f := &fakeStream{}
	cs := f.connect(t)
	var out streamPlaysOut
	callTool(t, cs, "find_stream_plays", map[string]any{"id": testStreamID, "wait_seconds": 5}, &out)
	if f.scanStarts != 1 || out.Status != "done" || out.Total != 3 {
		t.Fatalf("starts %d, got %+v", f.scanStarts, out)
	}
	got := []string{out.Plays[0].PlayID, out.Plays[1].PlayID, out.Plays[2].PlayID}
	if strings.Join(got, ",") != "play-002,play-003,play-001" || out.Plays[0].Kills != 5 {
		t.Errorf("ranked %v, first %+v", got, out.Plays[0])
	}
	callTool(t, cs, "find_stream_plays", map[string]any{"id": testStreamID}, &out)
	if f.scanStarts != 1 {
		t.Errorf("a done scan was started again: %d starts", f.scanStarts)
	}
}

func TestRenderStreamSavesApprovedPlaysThenRendersThatPlan(t *testing.T) {
	f := &fakeStream{scanStarts: 1}
	cs := f.connect(t)
	args := map[string]any{"id": testStreamID, "variant": "streamer-landscape-16x9", "play_ids": []string{"play-002", "play-003"}}
	var ask streamRenderStartOut
	callTool(t, cs, "render_stream", args, &ask)
	if !ask.NeedsApproval || f.savedPlan != nil || f.renderBody != "" {
		t.Fatalf("started without approval: %+v", ask)
	}
	if !strings.Contains(ask.Summary, "2 vídeos de «ESL final»") || !strings.Contains(ask.Summary, "Baja (0:50 a 0:58); Ace (15:00 a 15:30)") {
		t.Errorf("summary %q", ask.Summary)
	}
	args["approve"] = ask.ApprovalCode
	var started streamRenderStartOut
	callTool(t, cs, "render_stream", args, &started)
	if started.Variant != "streamer-landscape-16x9" {
		t.Fatalf("got %+v", started)
	}
	var clips []planClip
	if err := json.Unmarshal(f.savedPlan["clips"], &clips); err != nil || len(clips) != 2 || clips[0].ID != "play-003" || clips[1].Title != "Ace" {
		t.Errorf("saved clips %s", f.savedPlan["clips"])
	}
	if string(f.savedPlan["music"]) != `{"key":"song-7","volume":0.4}` || string(f.savedPlan["variant"]) != `"streamer-landscape-16x9"` {
		t.Errorf("plan fields lost: %v", f.savedPlan)
	}
	if want := `streamer-landscape-16x9 {"expected_edit_plan_updated_at":"2026-11-06T10:05:00.123Z"}`; f.renderBody != want {
		t.Errorf("render %q, want %q", f.renderBody, want)
	}
}

func TestRenderStreamRefusesUncheckedFacecamAndEmptyPlans(t *testing.T) {
	cs := (&fakeStream{scanStarts: 1}).connect(t)
	toolFails(t, cs, "render_stream", map[string]any{"id": testStreamID, "play_ids": []string{"play-001"}}, "facecam")
	toolFails(t, cs, "render_stream", map[string]any{"id": testStreamID, "variant": "streamer-landscape-16x9"}, "no moments saved")
	toolFails(t, cs, "render_stream", map[string]any{"id": testStreamID, "variant": "streamer-landscape-16x9", "play_ids": []string{"play-009"}}, "not a play of this stream")
	toolFails(t, cs, "render_stream", map[string]any{"id": testStreamID, "variant": "square"}, "unknown variant")
	toolFails(t, cs, "render_stream", map[string]any{"id": testStreamID, "play_ids": []string{"../x"}}, "must come from find_stream_plays")
}

func TestCheckStreamImport(t *testing.T) {
	video := filepath.Join(t.TempDir(), "vod.mp4")
	if err := os.WriteFile(video, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		in   importStreamIn
		want string
	}{
		{importStreamIn{URL: "https://www.twitch.tv/videos/2920321742"}, ""},
		{importStreamIn{URL: "https://www.twitch.tv/videos/2920321742", Kind: "vod"}, ""},
		{importStreamIn{Path: video}, ""},
		{importStreamIn{}, "either url"},
		{importStreamIn{URL: "https://www.twitch.tv/videos/1", Path: video}, "either url"},
		{importStreamIn{URL: "http://www.twitch.tv/videos/1"}, "https"},
		{importStreamIn{URL: "https://evil.example/vod.mp4"}, "not supported"},
		{importStreamIn{URL: "https://www.twitch.tv/videos/1", Kind: "movie"}, "kind must be"},
		{importStreamIn{Path: video, Kind: "vod"}, "applies to links"},
		{importStreamIn{Path: strings.TrimSuffix(video, ".mp4") + ".mkv"}, ".mp4"},
	}
	for _, tc := range cases {
		err := checkStreamImport(tc.in)
		if tc.want == "" && err != nil || tc.want != "" && (err == nil || !strings.Contains(err.Error(), tc.want)) {
			t.Errorf("%+v: got %v, want %q", tc.in, err, tc.want)
		}
	}
}

func TestStreamVideosBuildsUIURLsAndSkipsForeignPaths(t *testing.T) {
	job := tuiclient.StreamJob{ID: testStreamID, RenderedOutputs: []tuiclient.StreamRenderedOutput{
		{Variant: "streamer-landscape-16x9", ClipID: "play-002", AspectRatio: "16:9", VideoURL: "/api/streams/" + testStreamID + "/renders/streamer-landscape-16x9/revisions/r1/videos/play-002"},
		{Variant: "streamer-landscape-16x9", ClipID: "evil", VideoURL: "https://evil.example/x.mp4"},
	}}
	out := streamVideos(job, tuiclient.StreamSummary{Status: "done", DurationSeconds: 50, RenderedAt: "2026-11-06T10:00:00Z"}, "http://127.0.0.1:3000/", true)
	if len(out.Videos) != 2 {
		t.Fatalf("got %+v", out.Videos)
	}
	if out.Videos[0].URL != "http://127.0.0.1:3000/api/streams/"+testStreamID+"/renders/streamer-landscape-16x9/revisions/r1/videos/play-002" {
		t.Errorf("clip url %s", out.Videos[0].URL)
	}
	if out.Videos[1].Variant != summaryVariant || out.Videos[1].URL != "http://127.0.0.1:3000/api/streams/"+testStreamID+"/summary/video?v=2026-11-06T10%3A00%3A00Z" {
		t.Errorf("summary %+v", out.Videos[1])
	}
	if out := streamVideos(job, tuiclient.StreamSummary{Status: "idle"}, "", false); len(out.Videos) != 1 || out.Videos[0].URL != "" {
		t.Errorf("without origin: %+v", out.Videos)
	}
}

func TestClockSpellsHours(t *testing.T) {
	for in, want := range map[float64]string{0: "0:00", 59.6: "1:00", 905: "15:05", 3725: "1:02:05"} {
		if got := clock(in); got != want {
			t.Errorf("clock(%v) = %q, want %q", in, got, want)
		}
	}
}

func TestDescribeIntegrityRanksPlayersAndMetrics(t *testing.T) {
	doc := tuiclient.AnticheatDocument{Status: "ready", Report: &tuiclient.AnticheatReport{
		Limitations: []string{"No es una prueba."},
		Players: []tuiclient.AnticheatPlayer{
			{SteamID64: "1", Name: "calm", Score: 10, Verdict: "clean"},
			{SteamID64: "2", Name: "sus\u0000", Score: 80, Verdict: "anomalous", Metrics: []tuiclient.AnticheatMetric{
				{Label: "a", Suspicion: 10}, {Label: "b", Suspicion: 90}, {Label: "c", Suspicion: 50}, {Label: "d", Suspicion: 70}}},
		}}}
	out := describeIntegrity(doc)
	if out.Players[0].Name != "sus" || len(out.Players[0].TopMetrics) != maxIntegrityMetrics || out.Players[0].TopMetrics[0].Label != "b" || out.Players[0].TopMetrics[2].Label != "c" {
		t.Errorf("got %+v", out.Players)
	}
	if len(out.Limitations) != 1 {
		t.Errorf("limitations %v", out.Limitations)
	}
}
