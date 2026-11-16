package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func autoPOVStudio(t *testing.T, body string) (*mcp.ClientSession, *string) {
	t.Helper()
	var query string
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte("ok")) })
	mux.HandleFunc("GET /api/jobs/"+batchJobID+"/auto-pov", func(w http.ResponseWriter, r *http.Request) {
		query = r.URL.RawQuery
		_, _ = w.Write([]byte(body))
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	s := closedStudio(t)
	t.Setenv("ORCHESTRATOR_URL", srv.URL)
	server := mcp.NewServer(&mcp.Implementation{Name: "test", Version: "0"}, nil)
	registerAutoPOVTool(server, s)
	return connectWith(t, server, nil), &query
}

func TestAutoPOVSummaryReturnsThePickWithCuesAndPlayers(t *testing.T) {
	cs, query := autoPOVStudio(t, `{"target_seconds":120,"extra_seconds":7,"total_seconds":34,"segment_ids":["seg-002","seg-009"],"round_results":true,
		"plays":[{"segment_id":"seg-002","player":"b1t","steamid64":"2","round":3,"score":0.8,"interest":0.86,"events":{"kills":3},
			"cues":[{"kind":"clutch","detail":"won","count":3},{"kind":"momentum","detail":"comeback","count":4}]},
		{"segment_id":"seg-009","player":"ropz","steamid64":"1","round":11,"score":0.7,"interest":0.75,"events":{"kills":2},"cues":[{"kind":"bomb","detail":"retake","value":0.9}]}],
		"players":[{"steamid64":"1","name":"ropz","plays":1,"seconds":17},{"steamid64":"2","name":"b1t","plays":1,"seconds":17},{"steamid64":"3","name":"jL\u0007","plays":0,"seconds":0}]}`)
	res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{Name: "auto_pov_summary", Arguments: map[string]any{"job_id": batchJobID, "target_seconds": 120, "extra_seconds": 7}})
	if err != nil {
		t.Fatal(err)
	}
	if res.IsError {
		t.Fatalf("error result: %+v", res.Content[0])
	}
	if *query != "extra_seconds=7&target_seconds=120" {
		t.Errorf("auto-pov query = %q", *query)
	}
	out, _ := res.StructuredContent.(map[string]any)
	ids, _ := out["segment_ids"].([]any)
	plays, _ := out["plays"].([]any)
	players, _ := out["players"].([]any)
	if len(ids) != 2 || len(plays) != 2 || len(players) != 3 || out["round_results"] != true {
		t.Fatalf("out = %v", out)
	}
	first, _ := plays[0].(map[string]any)
	cues, _ := first["cues"].([]any)
	if first["player"] != "b1t" || first["interest"] != 0.86 || len(cues) != 2 {
		t.Fatalf("first play = %v", first)
	}
	if left, _ := players[2].(map[string]any); left["name"] != "jL" || left["plays"] != float64(0) {
		t.Fatalf("left-out player = %v, want the cleaned name with 0 plays", left)
	}
}

func TestAutoPOVSummaryRejectsABudgetOutOfRange(t *testing.T) {
	cs, query := autoPOVStudio(t, `{}`)
	for _, args := range []map[string]any{{"job_id": batchJobID, "target_seconds": 5}, {"job_id": batchJobID, "extra_seconds": 30}, {"job_id": "nope"}} {
		res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{Name: "auto_pov_summary", Arguments: args})
		if err != nil {
			t.Fatal(err)
		}
		if !res.IsError {
			t.Errorf("%v: want a tool error", args)
		}
	}
	if *query != "" {
		t.Errorf("a rejected call reached Studio: %q", *query)
	}
}
