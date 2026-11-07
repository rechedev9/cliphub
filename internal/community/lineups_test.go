package community

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"
)

func TestLineupsCacheKeepsTheLastCopyWhenTheServerFails(t *testing.T) {
	var calls, down atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		if down.Load() == 1 {
			w.WriteHeader(http.StatusBadGateway)
			return
		}
		if r.URL.Query().Get("map") == "de_nuke" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		if r.Header.Get("If-None-Match") == `"v1"` {
			w.WriteHeader(http.StatusNotModified)
			return
		}
		w.Header().Set("ETag", `"v1"`)
		_, _ = w.Write([]byte(`{"version":1,"map":"de_mirage","lineups":[{"id":"smoke-t-1","title":"Humo"}],"combos":[]}`))
	}))
	defer srv.Close()
	client, err := NewClient(srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	ctx := context.Background()
	c := OpenLineupsCache(dir, client)
	idx, fresh, err := c.Map(ctx, "de_mirage")
	if err != nil || !fresh || len(idx.Lineups) != 1 {
		t.Fatalf("first fetch: %v fresh=%v %+v", err, fresh, idx)
	}
	if _, _, err := c.Map(ctx, "de_mirage"); err != nil || calls.Load() != 1 {
		t.Fatalf("a fresh copy must not ask again: calls=%d err=%v", calls.Load(), err)
	}
	if _, _, err := c.Map(ctx, "de_nuke"); err != ErrNoLineups {
		t.Fatalf("missing map: %v", err)
	}
	// A restarted Studio with a stale copy on disk and the server down still answers.
	old := time.Now().Add(-2 * lineupsFresh)
	if err := os.Chtimes(filepath.Join(dir, "pro-lineups", "de_mirage.json"), old, old); err != nil {
		t.Fatal(err)
	}
	down.Store(1)
	idx, fresh, err = OpenLineupsCache(dir, client).Map(ctx, "de_mirage")
	if err != nil || fresh || idx.Map != "de_mirage" {
		t.Fatalf("stale fallback: %v fresh=%v", err, fresh)
	}
	if _, _, err := c.Map(ctx, "../etc"); err == nil {
		t.Fatal("an invalid map name must be refused")
	}
}
