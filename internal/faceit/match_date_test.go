package faceit

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestMatchStartedAtReadsTheStartOrNothing(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/matches/1-started":
			_, _ = w.Write([]byte(`{"match_id":"1-started","started_at":1792254000,"finished_at":1792257000}`))
		case "/matches/1-unstarted":
			_, _ = w.Write([]byte(`{"match_id":"1-unstarted"}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	client, err := New(Options{APIKey: "test", BaseURL: server.URL, HTTPClient: server.Client()})
	if err != nil {
		t.Fatal(err)
	}
	cases := map[string]time.Time{
		"1-started":   time.Date(2026, time.October, 17, 16, 20, 0, 0, time.UTC),
		"1-unstarted": {},
		"1-missing":   {},
	}
	for id, want := range cases {
		got, err := client.MatchStartedAt(context.Background(), id)
		if err != nil || !got.Equal(want) {
			t.Errorf("MatchStartedAt(%s) = %v, %v; want %v", id, got, err, want)
		}
	}
}
