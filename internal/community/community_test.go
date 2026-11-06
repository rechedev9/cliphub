package community

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/rechedev9/cliphub/internal/tacticalplan"
)

func TestQuantilesAndPercentileAgree(t *testing.T) {
	values := []float64{}
	for i := range 21 {
		values = append(values, float64(i)/20)
	}
	q := Quantiles(values)
	if len(q) != QuantilePoints || q[0] != 0 || q[50] != 0.5 || q[100] != 1 {
		t.Fatalf("quantiles = %v", q)
	}
	for v, want := range map[float64]float64{-1: 0, 0.5: 50, 0.525: 52.5, 2: 100} {
		if got := Percentile(q, v); got != want {
			t.Errorf("Percentile(%v) = %v, want %v", v, got, want)
		}
	}
	flat := Quantiles([]float64{1, 1, 1, 1})
	if got := Percentile(flat, 1); got != 50 {
		t.Fatalf("a value tied with everyone sits in the middle, got %v", got)
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

func TestSettingsStartOffWithARandomInstallID(t *testing.T) {
	s := NewSettingsStore(t.TempDir())
	first, err := s.Load()
	if err != nil || first.RankingsEnabled || !InstallIDPattern.MatchString(first.InstallID) {
		t.Fatalf("first load = %+v, %v", first, err)
	}
	updated, err := s.Update(func(v *Settings) { v.RankingsEnabled = true })
	if err != nil || !updated.RankingsEnabled || updated.InstallID != first.InstallID {
		t.Fatalf("update = %+v, %v", updated, err)
	}
}
