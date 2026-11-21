package main

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const testPhraseID = "6f1c2d3e-4a5b-4c6d-8e7f-90a1b2c3d4e5"

func TestStreamTranscriptToolsSpeakTheStreamRoutes(t *testing.T) {
	var mu sync.Mutex
	calls := []string{}
	record := func(r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		mu.Lock()
		calls = append(calls, r.Method+" "+strings.TrimPrefix(r.URL.Path, "/api/stream-jobs/"+testStreamID)+" "+string(body))
		mu.Unlock()
	}
	transcript := `{"job_id":"` + testStreamID + `","state":"ready","availability":"available","spans":[{"start_ms":60000,"end_ms":80000}],"pending":[],
		"lines":[{"id":"` + testPhraseID + `","start_ms":61000,"end_ms":62500,"text":"vaya tiro, tío","corrected":true,"original_text":"vaya tiro","discarded":false}],
		"transcriber":{"state":"installed"},"subtitles":{"shorts":true,"summary":false,"hidden":{"clip-1":["` + testPhraseID + `"]}}}`
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte("ok")) })
	mux.HandleFunc("/api/stream-jobs/"+testStreamID+"/transcript", func(w http.ResponseWriter, r *http.Request) {
		record(r)
		_, _ = w.Write([]byte(transcript))
	})
	mux.HandleFunc("PATCH /api/stream-jobs/"+testStreamID+"/transcript/lines/"+testPhraseID, func(w http.ResponseWriter, r *http.Request) {
		record(r)
		_, _ = w.Write([]byte(`{"id":"` + testPhraseID + `","start_ms":61000,"end_ms":62500,"text":"vaya tiro","corrected":false,"discarded":true}`))
	})
	mux.HandleFunc("PATCH /api/stream-jobs/"+testStreamID+"/subtitles", func(w http.ResponseWriter, r *http.Request) {
		record(r)
		_, _ = w.Write([]byte(`{"shorts":true,"summary":true,"hidden":{}}`))
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	s := closedStudio(t)
	t.Setenv("ORCHESTRATOR_URL", srv.URL)
	ctx := context.Background()
	server := mcp.NewServer(&mcp.Implementation{Name: "test", Version: "0"}, nil)
	registerStreamTranscriptTools(server, s)
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

	var out streamTranscriptOut
	callTool(t, cs, "stream_transcript", map[string]any{"id": testStreamID}, &out)
	if out.State != "ready" || out.Availability != "available" || len(out.Phrases) != 1 || out.Phrases[0].StartSeconds != 61 || out.Phrases[0].OriginalText != "vaya tiro" ||
		!out.Subtitles.Shorts || len(out.Subtitles.Hidden["clip-1"]) != 1 {
		t.Fatalf("stream_transcript = %+v", out)
	}
	callTool(t, cs, "transcribe_stream", map[string]any{"id": testStreamID, "spans": []map[string]float64{{"start_seconds": 62, "end_seconds": 75}}, "force": true}, &out)
	var phrase streamPhraseOut
	callTool(t, cs, "correct_stream_phrase", map[string]any{"id": testStreamID, "phrase_id": testPhraseID, "text": "vaya tiro"}, &phrase)
	callTool(t, cs, "discard_stream_phrase", map[string]any{"id": testStreamID, "phrase_id": testPhraseID}, &phrase)
	if !phrase.Discarded {
		t.Fatalf("discard_stream_phrase = %+v", phrase)
	}
	var subs streamSubtitlesOut
	callTool(t, cs, "set_stream_subtitles", map[string]any{"id": testStreamID, "summary": true}, &subs)
	callTool(t, cs, "set_stream_subtitles", map[string]any{"id": testStreamID, "video": "clip-1", "phrase_id": testPhraseID, "hidden": false}, &subs)
	want := []string{
		"GET /transcript ",
		`POST /transcript {"spans":[{"start_seconds":62,"end_seconds":75}],"force":true}`,
		`PATCH /transcript/lines/` + testPhraseID + ` {"text":"vaya tiro"}`,
		`PATCH /transcript/lines/` + testPhraseID + ` {"discarded":true}`,
		`PATCH /subtitles {"summary":true}`,
		`PATCH /subtitles {"video":"clip-1","line_id":"` + testPhraseID + `","hidden":false}`,
	}
	mu.Lock()
	defer mu.Unlock()
	if strings.Join(calls, "\n") != strings.Join(want, "\n") {
		t.Fatalf("requests:\n%s\nwant:\n%s", strings.Join(calls, "\n"), strings.Join(want, "\n"))
	}
}
