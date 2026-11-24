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

const (
	testReviewJobID = "8c7b6a59-4e3d-4c2b-9a18-0f9e8d7c6b5a"
	testPhraseID2   = "7e2d3c4b-5a6f-4d7e-9f80-a1b2c3d4e5f6"
)

type recordedCalls struct {
	mu    sync.Mutex
	calls []string
}

func (r *recordedCalls) record(req *http.Request) {
	body, _ := io.ReadAll(req.Body)
	r.mu.Lock()
	defer r.mu.Unlock()
	line := req.Method + " " + req.URL.Path
	if req.URL.RawQuery != "" {
		line += "?" + req.URL.RawQuery
	}
	r.calls = append(r.calls, strings.TrimSpace(line+" "+string(body)))
}

func (r *recordedCalls) list() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]string{}, r.calls...)
}

func (r *recordedCalls) reply(body string) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		r.record(req)
		_, _ = w.Write([]byte(body))
	}
}

func subtitleToolsSession(t *testing.T, mux *http.ServeMux) *mcp.ClientSession {
	t.Helper()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte("ok")) })
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	s := closedStudio(t)
	t.Setenv("ORCHESTRATOR_URL", srv.URL)
	ctx := context.Background()
	server := mcp.NewServer(&mcp.Implementation{Name: "test", Version: "0"}, nil)
	registerDemoTranscriptTools(server, s)
	registerStreamTranscriptTools(server, s)
	registerSubtitleReviewTools(server, s)
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

func sameCalls(t *testing.T, got, want []string) {
	t.Helper()
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Fatalf("requests:\n%s\nwant:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}

const demoReviewLine = `{"id":"` + testPhraseID + `","steamid64":"76561198000000002","name":"two","round":1,"tick":1000,"text":"flesh on Palas","corrected":false,"discarded":false}`

func TestDemoSubtitleReviewReadsTheVideosPhrasesAndAppliesCorrectionsAfterApproval(t *testing.T) {
	rec := &recordedCalls{}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/jobs/"+testReviewJobID+"/subtitle-review", rec.reply(`{"match_id":"`+testMatchID+`","state":"ready","lines":[`+demoReviewLine+`],
		"glossary":{"players":["one","two"],"callouts":["Palace"],"lexicon":["flash","molly"]}}`))
	mux.HandleFunc("GET /api/library/matches/"+testMatchID+"/voice/demo/transcript", rec.reply(`{"match_id":"`+testMatchID+`","state":"ready","lines":[`+demoReviewLine+`,
		{"id":"`+testPhraseID2+`","round":2,"tick":9000,"text":"drop the vesty"}],"transcriber":{"state":"installed"}}`))
	mux.HandleFunc("POST /api/library/matches/"+testMatchID+"/voice/demo/transcript/corrections", rec.reply(`{"lines":[
		{"id":"`+testPhraseID+`","round":1,"tick":1000,"text":"flash on Palace","corrected":true,"corrected_by":"assistant","original_text":"flesh on Palas"},
		{"id":"`+testPhraseID2+`","round":2,"tick":9000,"text":"drop the vest","corrected":true,"corrected_by":"assistant","original_text":"drop the vesty"}]}`))
	cs := subtitleToolsSession(t, mux)

	var review demoSubtitleReviewOut
	callTool(t, cs, "review_demo_subtitles", map[string]any{"job_id": testReviewJobID, "segment_ids": []string{"r1-k1", "r1-k2"}, "pre_roll_seconds": 8}, &review)
	if review.MatchID != testMatchID || len(review.Phrases) != 1 || review.Phrases[0].Text != "flesh on Palas" || len(review.Glossary.Callouts) != 1 || len(review.Glossary.Players) != 2 {
		t.Fatalf("review = %+v", review)
	}
	callTool(t, cs, "review_demo_subtitles", map[string]any{"job_id": testReviewJobID, "rounds": []int{1, 2}}, &review)

	fixes := []map[string]string{{"phrase_id": testPhraseID, "text": "flash on Palace"}, {"phrase_id": testPhraseID2, "text": "drop  the vest"}}
	var ask correctDemoPhrasesOut
	callTool(t, cs, "correct_demo_phrases", map[string]any{"match_id": testMatchID, "corrections": fixes}, &ask)
	if !ask.NeedsApproval || ask.ApprovalCode == "" || !strings.Contains(ask.Summary, "«flesh on Palas» → «flash on Palace»") || !strings.Contains(ask.Summary, "2 frases") {
		t.Fatalf("first call = %+v; want the approval with each change", ask)
	}
	toolFails(t, cs, "correct_demo_phrases", map[string]any{"match_id": testMatchID, "corrections": fixes[:1], "approve": ask.ApprovalCode}, "not valid")
	var done correctDemoPhrasesOut
	callTool(t, cs, "correct_demo_phrases", map[string]any{"match_id": testMatchID, "corrections": fixes, "approve": ask.ApprovalCode}, &done)
	if len(done.Phrases) != 2 || done.Phrases[0].CorrectedBy != "assistant" || done.Phrases[1].OriginalText != "drop the vesty" {
		t.Fatalf("approved call = %+v", done)
	}

	base := "/api/library/matches/" + testMatchID + "/voice/demo/transcript"
	sameCalls(t, rec.list(), []string{
		"GET /api/jobs/" + testReviewJobID + "/subtitle-review?pre_roll_seconds=8&segments=r1-k1%2Cr1-k2",
		"GET /api/jobs/" + testReviewJobID + "/subtitle-review?rounds=1%2C2",
		"GET " + base,
		"GET " + base,
		"GET " + base,
		"POST " + base + `/corrections {"lines":[{"line":"` + testPhraseID + `","text":"flash on Palace"},{"line":"` + testPhraseID2 + `","text":"drop the vest"}]}`,
	})
}

func TestSubtitleCorrectionsRefuseBadInputBeforeAnyRequest(t *testing.T) {
	rec := &recordedCalls{}
	mux := http.NewServeMux()
	mux.HandleFunc("/api/", rec.reply(`{}`))
	cs := subtitleToolsSession(t, mux)
	one := func(id, text string) map[string]string { return map[string]string{"phrase_id": id, "text": text} }
	many := make([]map[string]string, maxPhraseCorrections+1)
	for i := range many {
		many[i] = one(testPhraseID, "x")
	}
	for _, tool := range []struct{ name, idKey, id string }{{"correct_demo_phrases", "match_id", testMatchID}, {"correct_stream_phrases", "id", testStreamID}} {
		toolFails(t, cs, tool.name, map[string]any{tool.idKey: tool.id, "corrections": []any{one(testPhraseID, "  ")}}, "1 to 2000")
		toolFails(t, cs, tool.name, map[string]any{tool.idKey: tool.id, "corrections": []any{one(testPhraseID, "a"), one(testPhraseID, "b")}}, "twice")
		toolFails(t, cs, tool.name, map[string]any{tool.idKey: tool.id, "corrections": many}, "1 to 300")
		toolFails(t, cs, tool.name, map[string]any{tool.idKey: tool.id, "corrections": []any{}}, "1 to 300")
		toolFails(t, cs, tool.name, map[string]any{tool.idKey: tool.id, "corrections": []any{one("../x", "a")}}, "phrase_id")
	}
	toolFails(t, cs, "review_demo_subtitles", map[string]any{"job_id": testReviewJobID, "segment_ids": []string{"a"}, "rounds": []int{1}}, "not both")
	toolFails(t, cs, "review_demo_subtitles", map[string]any{"job_id": testReviewJobID, "deaths": true}, "segment_ids")
	toolFails(t, cs, "review_demo_subtitles", map[string]any{"job_id": testReviewJobID, "segment_ids": []string{"a"}, "pre_roll_seconds": 4}, "3, 5 or 8")
	toolFails(t, cs, "review_stream_subtitles", map[string]any{"id": testStreamID, "summary": true, "clip_ids": []string{"clip-1"}}, "play_ids")
	if calls := rec.list(); len(calls) != 0 {
		t.Fatalf("refused calls reached Studio: %q", calls)
	}
}

func TestStreamSubtitleReviewReadsTheVideosPhrasesAndAppliesCorrectionsAfterApproval(t *testing.T) {
	rec := &recordedCalls{}
	line := `{"id":"` + testPhraseID + `","start_ms":61000,"end_ms":62500,"text":"moly left","corrected":false,"discarded":false}`
	mux := http.NewServeMux()
	long := `{"id":"` + testPhraseID2 + `","start_ms":70000,"end_ms":90000,"text":"` + strings.Repeat("a", maxCommsTextRunes+1) + `"}`
	mux.HandleFunc("GET /api/stream-jobs/"+testStreamID+"/subtitle-review", rec.reply(`{"stream_id":"`+testStreamID+`","state":"ready","untranscribed":[{"start_seconds":290,"end_seconds":320}],"lines":[`+line+`],"glossary":{"players":[],"callouts":[],"lexicon":["molly"]}}`))
	mux.HandleFunc("GET /api/stream-jobs/"+testStreamID+"/transcript", rec.reply(`{"job_id":"`+testStreamID+`","state":"ready","lines":[`+line+`,`+long+`],"subtitles":{"hidden":{}}}`))
	mux.HandleFunc("POST /api/stream-jobs/"+testStreamID+"/transcript/corrections", rec.reply(`{"lines":[{"id":"`+testPhraseID+`","start_ms":61000,"end_ms":62500,"text":"molly left","corrected":true,"corrected_by":"assistant","original_text":"moly left"}]}`))
	cs := subtitleToolsSession(t, mux)

	var review streamSubtitleReviewOut
	callTool(t, cs, "review_stream_subtitles", map[string]any{"id": testStreamID, "clip_ids": []string{"clip-1"}, "play_ids": []string{"kill-3"}}, &review)
	if len(review.Phrases) != 1 || review.Phrases[0].StartSeconds != 61 || len(review.Glossary.Lexicon) != 1 || len(review.Untranscribed) != 1 || review.Untranscribed[0].StartSeconds != 290 {
		t.Fatalf("review = %+v", review)
	}
	callTool(t, cs, "review_stream_subtitles", map[string]any{"id": testStreamID, "summary": true}, &review)
	toolFails(t, cs, "correct_stream_phrases", map[string]any{"id": testStreamID, "corrections": []map[string]string{{"phrase_id": testPhraseID2, "text": "a"}}}, "longer than")
	fixes := []map[string]string{{"phrase_id": testPhraseID, "text": "molly left"}}
	var ask correctStreamPhrasesOut
	callTool(t, cs, "correct_stream_phrases", map[string]any{"id": testStreamID, "corrections": fixes}, &ask)
	if !ask.NeedsApproval || !strings.Contains(ask.Summary, "«moly left» → «molly left»") {
		t.Fatalf("first call = %+v", ask)
	}
	var done correctStreamPhrasesOut
	callTool(t, cs, "correct_stream_phrases", map[string]any{"id": testStreamID, "corrections": fixes, "approve": ask.ApprovalCode}, &done)
	if len(done.Phrases) != 1 || done.Phrases[0].CorrectedBy != "assistant" {
		t.Fatalf("approved call = %+v", done)
	}
	base := "/api/stream-jobs/" + testStreamID
	sameCalls(t, rec.list(), []string{
		"GET " + base + "/subtitle-review?clips=clip-1&plays=kill-3",
		"GET " + base + "/subtitle-review?summary=1",
		"GET " + base + "/transcript",
		"GET " + base + "/transcript",
		"GET " + base + "/transcript",
		"POST " + base + `/transcript/corrections {"lines":[{"line":"` + testPhraseID + `","text":"molly left"}]}`,
	})
}

func TestDemoTranscriptToolsSpeakTheTranscriptRoutes(t *testing.T) {
	rec := &recordedCalls{}
	base := "/api/library/matches/" + testMatchID + "/voice/demo/transcript"
	mux := http.NewServeMux()
	mux.HandleFunc("GET "+base, rec.reply(`{"match_id":"`+testMatchID+`","state":"ready","lines":[`+demoReviewLine+`],"transcriber":{"state":"installed"}}`))
	mux.HandleFunc("PATCH "+base+"/lines/"+testPhraseID, rec.reply(`{"id":"`+testPhraseID+`","round":1,"tick":1000,"text":"flash on Palace","corrected":true,"corrected_by":"user","discarded":false}`))
	cs := subtitleToolsSession(t, mux)

	var out demoTranscriptOut
	callTool(t, cs, "demo_transcript", map[string]any{"match_id": testMatchID}, &out)
	if len(out.Phrases) != 1 || out.Phrases[0].Text != "flesh on Palas" {
		t.Fatalf("demo_transcript = %+v", out)
	}
	var phrase demoPhraseOut
	callTool(t, cs, "correct_demo_phrase", map[string]any{"match_id": testMatchID, "phrase_id": testPhraseID, "text": "flash on Palace"}, &phrase)
	if phrase.CorrectedBy != "user" {
		t.Fatalf("correct_demo_phrase = %+v; want a user correction", phrase)
	}
	callTool(t, cs, "discard_demo_phrase", map[string]any{"match_id": testMatchID, "phrase_id": testPhraseID}, &phrase)
	sameCalls(t, rec.list(), []string{
		"GET " + base,
		"PATCH " + base + "/lines/" + testPhraseID + ` {"text":"flash on Palace"}`,
		"PATCH " + base + "/lines/" + testPhraseID + ` {"discarded":true}`,
	})
}

func TestSubtitleReviewSaysWhenItListsOnlyTheFirstPhrases(t *testing.T) {
	lines := make([]string, maxStreamPhrases+1)
	for i := range lines {
		lines[i] = `{"id":"` + testPhraseID + `","start_ms":61000,"end_ms":62500,"text":"moly left"}`
	}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/stream-jobs/"+testStreamID+"/subtitle-review", (&recordedCalls{}).reply(`{"stream_id":"`+testStreamID+`","state":"ready","lines":[`+strings.Join(lines, ",")+`]}`))
	cs := subtitleToolsSession(t, mux)
	var review streamSubtitleReviewOut
	callTool(t, cs, "review_stream_subtitles", map[string]any{"id": testStreamID}, &review)
	if !review.Truncated || len(review.Phrases) != maxStreamPhrases {
		t.Fatalf("review of %d phrases = %d listed, truncated %v", len(lines), len(review.Phrases), review.Truncated)
	}
}
