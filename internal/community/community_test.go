package community

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/rechedev9/cliphub/internal/tacticalplan"
)

func TestQuantiles(t *testing.T) {
	values := []float64{}
	for i := range 21 {
		values = append(values, float64(i)/20)
	}
	q := Quantiles(values)
	if len(q) != QuantilePoints || q[0] != 0 || q[50] != 0.5 || q[100] != 1 {
		t.Fatalf("quantiles = %v", q)
	}
	if Quantiles(nil) != nil {
		t.Fatal("no values, no quantiles")
	}
}

func TestNewClientRefusesPlainHTTPOffLoopback(t *testing.T) {
	for _, raw := range []string{"http://example.com", "ftp://x", "https://user@x.com", "not a url"} {
		if _, err := NewClient(raw); err == nil {
			t.Errorf("%q accepted", raw)
		}
	}
	if _, err := NewClient("https://cliphub.rechedev.cloud/"); err != nil {
		t.Fatal(err)
	}
}

func TestProReferenceCacheFallsBackAndRevalidates(t *testing.T) {
	ref := tacticalplan.DefaultProReference()
	ref.ID, ref.UpdatedAt = "server-test", "2026-11-06T00:00:00Z"
	body, err := json.Marshal(ref)
	if err != nil {
		t.Fatal(err)
	}
	calls, revalidated := 0, 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.URL.Query().Get("map") != AllMaps {
			t.Errorf("query = %s", r.URL.RawQuery)
		}
		if r.Header.Get("If-None-Match") == `"v1"` {
			revalidated++
			w.WriteHeader(http.StatusNotModified)
			return
		}
		w.Header().Set("ETag", `"v1"`)
		_, _ = w.Write(body)
	}))
	defer srv.Close()
	dir := t.TempDir()

	offline := OpenProReferenceCache(dir, nil)
	if got, server := offline.Current(); server || got.ID == ref.ID {
		t.Fatal("a cache that never fetched must use the bundled reference")
	}
	if err := offline.Refresh(context.Background()); err == nil {
		t.Fatal("a disabled server must report it")
	}

	client, err := NewClient(srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	c := OpenProReferenceCache(dir, client)
	if err := c.Refresh(context.Background()); err != nil {
		t.Fatal(err)
	}
	if got, server := c.Current(); !server || got.ID != "server-test" {
		t.Fatalf("after a fetch = %s server=%v", got.ID, server)
	}
	if err := c.Refresh(context.Background()); err != nil || revalidated != 1 {
		t.Fatalf("second refresh err=%v revalidated=%d", err, revalidated)
	}
	// A new Studio start offline reads the copy on disk.
	srv.Close()
	again := OpenProReferenceCache(dir, client)
	if err := again.Refresh(context.Background()); err == nil {
		t.Fatal("a closed server must fail the refresh")
	}
	if got, server := again.Current(); !server || got.ID != "server-test" || calls != 2 {
		t.Fatalf("offline restart = %s server=%v calls=%d", got.ID, server, calls)
	}
}

func TestRetireRankingsDeletesOnTheServerOnce(t *testing.T) {
	const id = "0f8fad5b-d9cb-469f-a165-70867728950e"
	deletes := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete || r.URL.Path != "/v1/rankings/installs/"+id {
			t.Errorf("request = %s %s", r.Method, r.URL.Path)
		}
		deletes++
		_, _ = w.Write([]byte(`{}`))
	}))
	defer srv.Close()
	client, err := NewClient(srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	path := filepath.Join(dir, rankingsFile)
	if err := os.WriteFile(path, []byte(`{"rankings_enabled":true,"install_id":"`+id+`"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	RetireRankings(context.Background(), dir, nil)
	if _, err := os.Stat(path); err != nil {
		t.Fatal("offline, the opt-in must stay for the next start")
	}
	RetireRankings(context.Background(), dir, client)
	RetireRankings(context.Background(), dir, client)
	if _, err := os.Stat(path); !os.IsNotExist(err) || deletes != 1 {
		t.Fatalf("after retiring: stat err=%v deletes=%d", err, deletes)
	}

	if err := os.WriteFile(path, []byte(`{"rankings_enabled":false,"install_id":"`+id+`"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	RetireRankings(context.Background(), dir, client)
	if _, err := os.Stat(path); !os.IsNotExist(err) || deletes != 1 {
		t.Fatalf("an install that never shared: stat err=%v deletes=%d", err, deletes)
	}
}
