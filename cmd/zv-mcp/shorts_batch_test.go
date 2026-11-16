package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/rechedev9/cliphub/internal/tuiclient"
)

const batchJobID = "6f1c2a9e-3b7d-4c8a-9e21-5d4b3a2f1e0c"

func batchStudio(t *testing.T, batch string) (*mcp.ClientSession, *[]tuiclient.GenerateRequest, *string) {
	t.Helper()
	var started []tuiclient.GenerateRequest
	var query string
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte("ok")) })
	mux.HandleFunc("GET /api/presets", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"default":"viral-60-clean","presets":[{"name":"viral-60-clean","label":"Killfeed"}]}`))
	})
	mux.HandleFunc("GET /api/jobs/"+batchJobID+"/shorts-batch", func(w http.ResponseWriter, r *http.Request) {
		query = r.URL.RawQuery
		_, _ = w.Write([]byte(batch))
	})
	mux.HandleFunc("POST /api/jobs/"+batchJobID+"/generate", func(w http.ResponseWriter, r *http.Request) {
		var body tuiclient.GenerateRequest
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("decode generate: %v", err)
		}
		started = append(started, body)
		w.WriteHeader(http.StatusAccepted)
		_, _ = w.Write([]byte(`{"id":"` + batchJobID + `","task":"record:demo","variant":"viral-60-clean"}`))
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	s := closedStudio(t)
	t.Setenv("ORCHESTRATOR_URL", srv.URL)
	server := mcp.NewServer(&mcp.Implementation{Name: "test", Version: "0"}, nil)
	registerShortsBatchTool(server, s)
	return connectWith(t, server, nil), &started, &query
}

func callBatch(t *testing.T, cs *mcp.ClientSession, args map[string]any) map[string]any {
	t.Helper()
	res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{Name: "generate_shorts_batch", Arguments: args})
	if err != nil {
		t.Fatal(err)
	}
	if res.IsError {
		t.Fatalf("error result: %+v", res.Content[0])
	}
	out, _ := res.StructuredContent.(map[string]any)
	return out
}

func TestGenerateShortsBatchStartsOneCaptureOfSeparateShortsAfterApproval(t *testing.T) {
	cs, started, query := batchStudio(t, `{"min_score":0.7,"limit":3,"segment_ids":["seg-002","seg-005"],"moments":[
		{"segment_id":"seg-002","round":3,"score":0.72,"events":{"kills":3}},
		{"segment_id":"seg-005","round":9,"score":0.81,"events":{"kills":4}}]}`)
	args := map[string]any{"job_id": batchJobID, "min_score": 0.7, "limit": 3}

	first := callBatch(t, cs, args)
	if first["needs_approval"] != true || len(*started) != 0 {
		t.Fatalf("first call = %v, started = %d; want an approval ask and no capture", first, len(*started))
	}
	if *query != "limit=3&min_score=0.7" {
		t.Errorf("batch query = %q", *query)
	}
	if summary, _ := first["summary"].(string); !strings.Contains(summary, "2 Shorts") || !strings.Contains(summary, "seg-005") {
		t.Errorf("summary = %q", summary)
	}

	args["approve"] = first["approval_code"]
	second := callBatch(t, cs, args)
	if second["variant"] != "viral-60-clean" || len(*started) != 1 {
		t.Fatalf("approved call = %v, started = %d", second, len(*started))
	}
	body := (*started)[0]
	if !body.Edit.SeparateShorts || !slices.Equal(body.SegmentIDs, []string{"seg-002", "seg-005"}) || body.Preset != "viral-60-clean" {
		t.Fatalf("generate body = %+v", body)
	}
}

func TestGenerateShortsBatchWithNoPlayOverTheBarStartsNothing(t *testing.T) {
	cs, started, _ := batchStudio(t, `{"min_score":0.9,"limit":5,"segment_ids":[],"moments":[]}`)
	out := callBatch(t, cs, map[string]any{"job_id": batchJobID, "min_score": 0.9})
	if out["needs_approval"] == true || len(*started) != 0 {
		t.Fatalf("out = %v, started = %d", out, len(*started))
	}
	if summary, _ := out["summary"].(string); !strings.Contains(summary, "Ninguna jugada") {
		t.Errorf("summary = %q", summary)
	}
}
