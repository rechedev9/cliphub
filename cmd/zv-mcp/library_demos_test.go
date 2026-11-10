package main

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const (
	testDemoID  = "0a1b2c3d-4e5f-4a6b-8c7d-9e0f1a2b3c4d"
	testDemoID2 = "1b2c3d4e-5f6a-4b7c-8d9e-0f1a2b3c4d5e"
)

// fakeLibrary is an orchestrator holding two ready demos; it records PATCH bodies and deleted ids.
type fakeLibrary struct {
	mu      sync.Mutex
	patches []string
	deleted []string
}

func (f *fakeLibrary) connect(t *testing.T) *mcp.ClientSession {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte("ok")) })
	demo := `{"id":"` + testDemoID + `","file_name":"final.dem","state":"ready","source":"faceit","tags":["ESL"],"imported_at":"2026-11-01T10:00:00Z"}`
	demo2 := `{"id":"` + testDemoID2 + `","file_name":"semi.dem","state":"ready","source":"faceit","tags":[],"imported_at":"2026-11-02T10:00:00Z"}`
	mux.HandleFunc("GET /api/library/demos/"+testDemoID, func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte(demo)) })
	mux.HandleFunc("GET /api/library/demos/"+testDemoID2, func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte(demo2)) })
	mux.HandleFunc("PATCH /api/library/demos/"+testDemoID, func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		f.mu.Lock()
		f.patches = append(f.patches, string(body))
		f.mu.Unlock()
		_, _ = w.Write([]byte(demo))
	})
	mux.HandleFunc("DELETE /api/library/demos/{id}", func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		f.deleted = append(f.deleted, r.PathValue("id"))
		f.mu.Unlock()
		w.WriteHeader(http.StatusNoContent)
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	s := closedStudio(t)
	t.Setenv("ORCHESTRATOR_URL", srv.URL)

	ctx := context.Background()
	server := mcp.NewServer(&mcp.Implementation{Name: "test", Version: "0"}, nil)
	registerLibraryDemoTools(server, s)
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

// An empty tag list must reach the server as [] (clear), and an absent one must not be sent at all (keep).
func TestUpdateDemoSendsOnlyTheFieldsGiven(t *testing.T) {
	f := &fakeLibrary{}
	cs := f.connect(t)
	var out demoOut
	callTool(t, cs, "update_demo", map[string]any{"id": testDemoID, "tags": []string{}}, &out)
	callTool(t, cs, "update_demo", map[string]any{"id": testDemoID, "source": "scrim"}, &out)
	if want := []string{`{"tags":[]}`, `{"source":"scrim"}`}; !slices.Equal(f.patches, want) {
		t.Fatalf("patches = %q, want %q", f.patches, want)
	}
	toolFails(t, cs, "update_demo", map[string]any{"id": testDemoID, "source": "shared"}, "source must be one of")
	toolFails(t, cs, "update_demo", map[string]any{"id": testDemoID}, "nothing to change")
	toolFails(t, cs, "update_demo", map[string]any{"id": testDemoID, "tags": []string{"ESL, scrim"}}, "comma")
	if len(f.patches) != 2 {
		t.Errorf("a refused call reached the server: %q", f.patches)
	}
}

func TestUpdateDemoAddsAndRemovesSingleTags(t *testing.T) {
	f := &fakeLibrary{}
	cs := f.connect(t)
	var out demoOut
	callTool(t, cs, "update_demo", map[string]any{"id": testDemoID, "add_tags": []string{"scrim", "ESL"}, "remove_tags": []string{"nada"}}, &out)
	callTool(t, cs, "update_demo", map[string]any{"id": testDemoID, "remove_tags": []string{"ESL"}}, &out)
	if want := []string{`{"tags":["ESL","scrim"]}`, `{"tags":[]}`}; !slices.Equal(f.patches, want) {
		t.Fatalf("patches = %q, want %q", f.patches, want)
	}
	toolFails(t, cs, "update_demo", map[string]any{"id": testDemoID, "tags": []string{"a"}, "add_tags": []string{"b"}}, "not both")
	toolFails(t, cs, "update_demo", map[string]any{"id": testDemoID, "add_tags": []string{"a,b"}}, "comma")
}

func TestDeleteDemoNeedsTheApprovalCode(t *testing.T) {
	f := &fakeLibrary{}
	cs := f.connect(t)
	var ask deleteOut
	callTool(t, cs, "delete_demo", map[string]any{"id": testDemoID}, &ask)
	if !ask.NeedsApproval || ask.ApprovalCode == "" || len(f.deleted) != 0 {
		t.Fatalf("first call = %+v, deleted %q", ask, f.deleted)
	}
	toolFails(t, cs, "delete_demo", map[string]any{"id": testDemoID, "approve": "0000"}, "not valid")
	var done deleteOut
	callTool(t, cs, "delete_demo", map[string]any{"id": testDemoID, "approve": ask.ApprovalCode}, &done)
	if !done.Deleted || !slices.Equal(f.deleted, []string{testDemoID}) {
		t.Fatalf("approved call = %+v, deleted %q", done, f.deleted)
	}
	toolFails(t, cs, "delete_demo", map[string]any{"id": "../jobs"}, "id")
}

// Several demos go under one approval, and the code for one set does not approve another.
func TestDeleteDemoRemovesSeveralWithOneApproval(t *testing.T) {
	f := &fakeLibrary{}
	cs := f.connect(t)
	both := []string{testDemoID, testDemoID2}
	var ask deleteOut
	callTool(t, cs, "delete_demo", map[string]any{"ids": both}, &ask)
	if !ask.NeedsApproval || !strings.Contains(ask.Summary, "2 partidas") || !strings.Contains(ask.Summary, "semi.dem") {
		t.Fatalf("first call = %+v", ask)
	}
	toolFails(t, cs, "delete_demo", map[string]any{"ids": []string{testDemoID2}, "approve": ask.ApprovalCode}, "not valid")
	var done deleteOut
	callTool(t, cs, "delete_demo", map[string]any{"ids": both, "approve": ask.ApprovalCode}, &done)
	if !done.Deleted || !slices.Equal(f.deleted, both) {
		t.Fatalf("approved call = %+v, deleted %q", done, f.deleted)
	}
	toolFails(t, cs, "delete_demo", map[string]any{"id": testDemoID, "ids": both}, "not both")
	toolFails(t, cs, "delete_demo", map[string]any{"ids": []string{testDemoID, testDemoID}}, "twice")
	toolFails(t, cs, "delete_demo", map[string]any{}, "required")
}
