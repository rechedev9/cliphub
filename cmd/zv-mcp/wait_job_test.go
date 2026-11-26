package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const waitJobID = "0b6c1d7e-52a4-4f0b-8f3e-9a1d2c3b4e5f"

func waitJobStudio(t *testing.T, status, otherRender *string) *mcp.ClientSession {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte("ok")) })
	mux.HandleFunc("GET /api/presets", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"default":"viral-60-clean","presets":[{"name":"viral-60-clean","label":"Killfeed"},{"name":"clean-pov-60","label":"POV limpio"}]}`))
	})
	mux.HandleFunc("GET /api/jobs/"+waitJobID+"/renders/clean-pov-60", func(w http.ResponseWriter, _ *http.Request) {
		if *otherRender == "" {
			w.WriteHeader(http.StatusNotFound)
		}
		_, _ = w.Write([]byte(*otherRender))
	})
	mux.HandleFunc("GET /api/jobs/"+waitJobID, func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte(*status)) })
	mux.HandleFunc("GET /api/jobs/"+waitJobID+"/renders/viral-60-clean", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"job_id":"` + waitJobID + `","variant":"viral-60-clean","status":"ready","created_at":"2026-11-26T10:00:00Z","updated_at":"2026-11-26T10:05:00Z"}`))
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	s := closedStudio(t)
	t.Setenv("ORCHESTRATOR_URL", srv.URL)
	server := mcp.NewServer(&mcp.Implementation{Name: "test", Version: "0"}, nil)
	registerTools(server, s)
	return connectWith(t, server, nil)
}

func waitJob(t *testing.T, cs *mcp.ClientSession, variant string) map[string]any {
	t.Helper()
	args := map[string]any{"id": waitJobID, "wait_seconds": 1}
	if variant != "" {
		args["variant"] = variant
	}
	res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{Name: "wait_for_job", Arguments: args})
	if err != nil {
		t.Fatal(err)
	}
	if res.IsError {
		t.Fatalf("error result: %+v", res.Content[0])
	}
	out, _ := res.StructuredContent.(map[string]any)
	return out
}

func TestWaitForJobNeverReportsTheEarlierRenderAsTheNewOne(t *testing.T) {
	status, otherRender := "", ""
	cs := waitJobStudio(t, &status, &otherRender)

	status = `{"status":"recorded","generate":{"variant":"viral-60-clean","active":true}}`
	queued := waitJob(t, cs, "viral-60-clean")
	if queued["running"] != true || queued["stage"] != "capture queued" || queued["renders"] != nil || queued["note"] != nil {
		t.Fatalf("a generate run that waits behind the earlier ready render = %v, want running in the queue", queued)
	}

	status = `{"status":"parsed","generate":{"variant":"viral-60-clean","active":true}}`
	if first := waitJob(t, cs, ""); first["running"] != true || first["stage"] != "capture queued" {
		t.Fatalf("a first generate run no worker took yet = %v, want running in the queue", first)
	}

	status = `{"status":"failed","failure_reason":"recorder failed","failure_code":"capture_flake","generate":{"variant":"viral-60-clean","active":true}}`
	if retried := waitJob(t, cs, ""); retried["running"] != true || retried["failure_code"] != nil || retried["failure_reason"] != nil {
		t.Fatalf("a generate run accepted after a failed capture = %v, want running without the earlier failure", retried)
	}

	status = `{"status":"recorded","generate":{"variant":"viral-60-clean","active":false,"failure_reason":"recorder failed: full demo aggregate coverage differs from its original run","failure_code":"capture_flake","failed_at":"2026-11-26T10:20:00Z"}}`
	failed := waitJob(t, cs, "viral-60-clean")
	reason, _ := failed["failure_reason"].(string)
	note, _ := failed["note"].(string)
	if failed["running"] != false || failed["failure_code"] != "capture_flake" || !strings.Contains(reason, "aggregate coverage") || strings.Contains(note, "finished") {
		t.Fatalf("a failed capture that kept the earlier recording = %v, want its failure", failed)
	}

	status = `{"status":"recorded","generate":{"variant":"viral-60-clean","active":false,"failure_reason":"recorder failed","failed_at":"2026-11-26T10:20:00Z"}}`
	if uncoded := waitJob(t, cs, ""); uncoded["failure_code"] != "capture_failed" {
		t.Fatalf("a failed capture without a code = %v, want capture_failed", uncoded)
	}
	otherRender = `{"job_id":"` + waitJobID + `","variant":"clean-pov-60","status":"ready","created_at":"2026-11-26T10:25:00Z","updated_at":"2026-11-26T10:30:00Z"}`
	if unrelated := waitJob(t, cs, ""); unrelated["failure_code"] != "capture_failed" {
		t.Fatalf("a failed capture with a later render of another variant = %v, want capture_failed", unrelated)
	}
	otherRender = ""

	status = `{"status":"recorded","generate":{"variant":"viral-60-clean","active":false,"failure_reason":"recorder failed","failed_at":"2026-11-26T10:01:00Z"}}`
	later := waitJob(t, cs, "viral-60-clean")
	if later["running"] != false || later["failure_code"] != nil || later["note"] != "The render finished; list_videos has the videos." {
		t.Fatalf("a render made after the failed capture = %v, want it finished", later)
	}

	status = `{"status":"recorded","generate":{"variant":"gameplay-pov-60","active":false,"failure_reason":"recorder failed","failed_at":"2026-11-26T10:20:00Z"}}`
	if other := waitJob(t, cs, "viral-60-clean"); other["failure_code"] != nil {
		t.Fatalf("the failed capture of another variant = %v, want this variant finished", other)
	}

	status = `{"status":"recorded"}`
	if idle := waitJob(t, cs, "viral-60-clean"); idle["running"] != false || idle["failure_code"] != nil {
		t.Fatalf("a finished render with no generate run = %v, want it finished", idle)
	}
}
