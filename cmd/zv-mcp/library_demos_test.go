package main

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"slices"
	"sync"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const testDemoID = "0a1b2c3d-4e5f-4a6b-8c7d-9e0f1a2b3c4d"

// fakeLibrary is an orchestrator holding one ready demo; it records PATCH bodies and deletes.
type fakeLibrary struct {
	mu      sync.Mutex
	patches []string
	deletes int
}

func (f *fakeLibrary) connect(t *testing.T) *mcp.ClientSession {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte("ok")) })
	demo := `{"id":"` + testDemoID + `","file_name":"final.dem","state":"ready","source":"faceit","tags":["ESL"],"imported_at":"2026-11-01T10:00:00Z"}`
	mux.HandleFunc("GET /api/library/demos/"+testDemoID, func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte(demo)) })
	mux.HandleFunc("PATCH /api/library/demos/"+testDemoID, func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		f.mu.Lock()
		f.patches = append(f.patches, string(body))
		f.mu.Unlock()
		_, _ = w.Write([]byte(demo))
	})
	mux.HandleFunc("DELETE /api/library/demos/"+testDemoID, func(w http.ResponseWriter, _ *http.Request) {
		f.mu.Lock()
		f.deletes++
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

func TestDeleteDemoNeedsTheApprovalCode(t *testing.T) {
	f := &fakeLibrary{}
	cs := f.connect(t)
	var ask deleteOut
	callTool(t, cs, "delete_demo", map[string]any{"id": testDemoID}, &ask)
	if !ask.NeedsApproval || ask.ApprovalCode == "" || f.deletes != 0 {
		t.Fatalf("first call = %+v, deletes %d", ask, f.deletes)
	}
	toolFails(t, cs, "delete_demo", map[string]any{"id": testDemoID, "approve": "0000"}, "not valid")
	var done deleteOut
	callTool(t, cs, "delete_demo", map[string]any{"id": testDemoID, "approve": ask.ApprovalCode}, &done)
	if !done.Deleted || f.deletes != 1 {
		t.Fatalf("approved call = %+v, deletes %d", done, f.deletes)
	}
	toolFails(t, cs, "delete_demo", map[string]any{"id": "../jobs"}, "id")
}
