package faceit

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"sync/atomic"
	"testing"
	"time"
)

// rankingStoreFixture is a FACEIT whose one Spanish player's ELO and health
// the test steers, with a clock it moves by hand.
type rankingStoreFixture struct {
	client   *Client
	requests atomic.Int32
	elo      atomic.Int32
	status   atomic.Int32
	now      time.Time
}

func newRankingStoreFixture(t *testing.T) *rankingStoreFixture {
	t.Helper()
	f := &rankingStoreFixture{now: time.Date(2026, time.November, 18, 12, 0, 0, 0, time.UTC)}
	f.elo.Store(3000)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f.requests.Add(1)
		if status := int(f.status.Load()); status != 0 {
			w.WriteHeader(status)
			return
		}
		body := `{"items":[{"player_id":"es-1","nickname":"mopoz","country":"es","position":1,"faceit_elo":` +
			strconv.Itoa(int(f.elo.Load())) + `,"game_skill_level":10}]}`
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(server.Close)
	client, err := New(Options{APIKey: rankingsTestKey, BaseURL: server.URL, HTTPClient: server.Client(), RequestTimeout: time.Second})
	if err != nil {
		t.Fatal(err)
	}
	f.client = client
	return f
}

func (f *rankingStoreFixture) store(t *testing.T, dir string) *RankingStore {
	t.Helper()
	store, err := NewRankingStore(dir, func() time.Time { return f.now })
	if err != nil {
		t.Fatal(err)
	}
	return store
}

func TestRankingStoreServesACopyFor6HoursThenRereadsFACEIT(t *testing.T) {
	t.Parallel()
	f := newRankingStoreFixture(t)
	dir := t.TempDir()
	es := mustRankingEntry(t, RankingRegion, "EU")
	read := func(store *RankingStore) RankingList {
		t.Helper()
		list, err := store.Top(context.Background(), f.client, es)
		if err != nil {
			t.Fatal(err)
		}
		return list
	}

	first := read(f.store(t, dir))
	if first.Stale || !first.FetchedAt.Equal(f.now) || len(first.Players) != 1 || first.Players[0].ELO != 3000 || f.requests.Load() != 1 {
		t.Fatalf("first read = %+v after %d requests, want a fresh FACEIT read", first, f.requests.Load())
	}

	// Inside the window, even from a new store (a Studio restart), the copy on disk answers.
	f.elo.Store(3100)
	f.now = f.now.Add(RankingMaxAge - time.Minute)
	if cached := read(f.store(t, dir)); cached.Players[0].ELO != 3000 || !cached.FetchedAt.Equal(first.FetchedAt) || f.requests.Load() != 1 {
		t.Fatalf("read inside the window = %+v after %d requests, want the stored copy", cached, f.requests.Load())
	}

	f.now = first.FetchedAt.Add(RankingMaxAge)
	if fresh := read(f.store(t, dir)); fresh.Stale || fresh.Players[0].ELO != 3100 || !fresh.FetchedAt.Equal(f.now) || f.requests.Load() != 2 {
		t.Fatalf("read after 6 hours = %+v after %d requests, want a new FACEIT read", fresh, f.requests.Load())
	}
}

func TestRankingStoreServesTheLastCopyWhenFACEITFails(t *testing.T) {
	t.Parallel()
	f := newRankingStoreFixture(t)
	store := f.store(t, t.TempDir())
	eu := mustRankingEntry(t, RankingRegion, "EU")
	stored, err := store.Top(context.Background(), f.client, eu)
	if err != nil {
		t.Fatal(err)
	}

	f.status.Store(http.StatusInternalServerError)
	f.now = f.now.Add(RankingMaxAge + time.Hour)
	list, err := store.Top(context.Background(), f.client, eu)
	if err != nil {
		t.Fatalf("failed reread error = %v, want the stored copy", err)
	}
	if !list.Stale || !errors.Is(list.StaleErr, ErrUnavailable) || !list.FetchedAt.Equal(stored.FetchedAt) || list.Players[0].ELO != 3000 {
		t.Fatalf("failed reread = %+v, want the copy of %v marked stale", list, stored.FetchedAt)
	}

	// The failure dated nothing: once FACEIT answers again the next read is fresh.
	f.status.Store(0)
	if list, err := store.Top(context.Background(), f.client, eu); err != nil || list.Stale || !list.FetchedAt.Equal(f.now) {
		t.Fatalf("read after recovery = %+v, %v; want a fresh read", list, err)
	}
}

func TestRankingStoreFailsWithoutACopy(t *testing.T) {
	t.Parallel()
	f := newRankingStoreFixture(t)
	dir := t.TempDir()
	f.status.Store(http.StatusUnauthorized)
	if _, err := f.store(t, dir).Top(context.Background(), f.client, mustRankingEntry(t, RankingCountry, "es")); !errors.Is(err, ErrUnauthorized) {
		t.Fatalf("error = %v, want ErrUnauthorized with nothing stored", err)
	}

	// A damaged copy is no copy: it is read again, never served.
	if err := os.WriteFile(filepath.Join(dir, "country-es.json"), []byte("{\"schema_version\":"), 0o600); err != nil {
		t.Fatal(err)
	}
	f.status.Store(0)
	if list, err := f.store(t, dir).Top(context.Background(), f.client, mustRankingEntry(t, RankingCountry, "es")); err != nil || list.Stale || len(list.Players) != 1 {
		t.Fatalf("read over a damaged copy = %+v, %v; want a fresh read", list, err)
	}
}
