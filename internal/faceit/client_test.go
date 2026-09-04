package faceit

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestClientCapsConcurrentUpstreamRequests(t *testing.T) {
	t.Parallel()
	const (
		slots     = 3
		callers   = 12
		holdLimit = 5 * time.Second
	)
	arrived := make(chan struct{}, callers)
	release := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		arrived <- struct{}{}
		select {
		case <-release:
		case <-time.After(holdLimit):
		}
		_, _ = w.Write([]byte(`{"items":[]}`))
	}))
	defer server.Close()
	client, err := New(Options{APIKey: "faceit-test-key", BaseURL: server.URL, HTTPClient: server.Client(), DetailWorkers: slots})
	if err != nil {
		t.Fatal(err)
	}

	var wg sync.WaitGroup
	failures := make(chan error, callers)
	for range callers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			var page apiHistoryResponse
			if err := client.getJSON(context.Background(), "/players/player-1/history", nil, &page); err != nil {
				failures <- err
			}
		}()
	}

	for range slots {
		select {
		case <-arrived:
		case <-time.After(3 * time.Second):
			t.Fatalf("only %d of %d slots reached the server", len(arrived), slots)
		}
	}
	select {
	case <-arrived:
		t.Fatalf("more than %d requests were in flight at once", slots)
	case <-time.After(200 * time.Millisecond):
	}

	close(release)
	wg.Wait()
	close(failures)
	for err := range failures {
		t.Fatalf("getJSON: %v", err)
	}
}

func TestClientReleasesRequestSlotOnEveryOutcome(t *testing.T) {
	t.Parallel()
	var inFlight, peak atomic.Int64
	tests := []struct {
		name    string
		status  int
		body    string
		wantErr bool
	}{
		{name: "ok", status: http.StatusOK, body: `{"items":[]}`},
		{name: "not found", status: http.StatusNotFound, body: `{}`, wantErr: true},
		{name: "invalid body", status: http.StatusOK, body: `not json`, wantErr: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				current := inFlight.Add(1)
				defer inFlight.Add(-1)
				for {
					observed := peak.Load()
					if current <= observed || peak.CompareAndSwap(observed, current) {
						break
					}
				}
				w.WriteHeader(test.status)
				_, _ = w.Write([]byte(test.body))
			}))
			defer server.Close()
			client, err := New(Options{APIKey: "faceit-test-key", BaseURL: server.URL, HTTPClient: server.Client(), DetailWorkers: 1})
			if err != nil {
				t.Fatal(err)
			}
			// A leaked slot would block the second call; the single slot has to
			// come back after success, an API error, and a decode failure
			// alike. The deadline turns a leak into a failure, not a hang.
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			for range 2 {
				var page apiHistoryResponse
				err := client.getJSON(ctx, "/players/player-1/history", nil, &page)
				if (err != nil) != test.wantErr {
					t.Fatalf("getJSON error = %v, wantErr = %v", err, test.wantErr)
				}
			}
			if err := ctx.Err(); err != nil {
				t.Fatalf("request slot was not released: %v", err)
			}
			if got := peak.Load(); got > 1 {
				t.Fatalf("peak in-flight = %d, want 1", got)
			}
		})
	}
}

func TestRecentMatchesFetchesHistoryAndStatsConcurrently(t *testing.T) {
	t.Parallel()
	arrived := make(chan string, 4)
	release := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/history"):
			arrived <- "history"
		case strings.HasSuffix(r.URL.Path, "/stats"):
			arrived <- "stats"
		default:
			http.NotFound(w, r)
			return
		}
		// A serial client never puts both handlers on the barrier, so it times
		// out here and fails the assertion below instead of hanging the test.
		select {
		case <-release:
		case <-time.After(5 * time.Second):
		}
		_, _ = w.Write([]byte(`{"items":[]}`))
	}))
	defer server.Close()
	client, err := New(Options{APIKey: "faceit-test-key", BaseURL: server.URL, HTTPClient: server.Client()})
	if err != nil {
		t.Fatal(err)
	}

	done := make(chan error, 1)
	go func() {
		_, err := client.RecentMatches(context.Background(), "player-1", 10)
		done <- err
	}()

	seen := map[string]bool{}
	deadline := time.After(2 * time.Second)
	for len(seen) < 2 {
		select {
		case kind := <-arrived:
			seen[kind] = true
		case <-deadline:
			t.Fatalf("only %v reached the server together; history and stats are still serial", seen)
		}
	}
	close(release)
	if err := <-done; err != nil {
		t.Fatalf("RecentMatches: %v", err)
	}
}

func TestRecentMatchesMatchesSequentialMerge(t *testing.T) {
	t.Parallel()
	const historyBody = `{"items":[
		{"match_id":"match-a","started_at":1700000000,"finished_at":1700003600,"competition_name":"FACEIT Ladder","teams":{"faction1":{"players":[{"player_id":"player-1"}]},"faction2":{"players":[{"player_id":"player-2"}]}},"results":{"winner":"faction1","score":{"faction1":13,"faction2":7}}},
		{"match_id":"match-b","started_at":1699990000,"finished_at":1699993600,"competition_name":"FACEIT Ladder","teams":{"faction1":{"players":[{"player_id":"player-2"}]},"faction2":{"players":[{"player_id":"player-1"}]}},"results":{"winner":"faction1","score":{"faction1":13,"faction2":4}}}
	]}`
	const statsBody = `{"items":[
		{"stats":{"Match Id":"match-b","Map":"de_mirage","Result":"0","Rounds":"17","Kills":"14","Deaths":"18","Assists":"3","ADR":"61.4","K/D Ratio":"0.78","K/R Ratio":"0.82","Headshots":"7","Headshots %":"50"}},
		{"stats":{"Match Id":"match-a","Map":"de_dust2","Result":"1","Rounds":"20","Kills":"28","Deaths":"12","Assists":"5","ADR":"104.2","K/D Ratio":"2.33","K/R Ratio":"1.4","Headshots":"16","Headshots %":"57"}}
	]}`
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/history"):
			_, _ = w.Write([]byte(historyBody))
		case strings.HasSuffix(r.URL.Path, "/stats"):
			_, _ = w.Write([]byte(statsBody))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	client, err := New(Options{APIKey: "faceit-test-key", BaseURL: server.URL, HTTPClient: server.Client()})
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()

	history, err := client.fetchRecentHistory(ctx, "player-1", 10)
	if err != nil {
		t.Fatal(err)
	}
	stats, err := client.fetchRecentStats(ctx, "player-1", 10)
	if err != nil {
		t.Fatal(err)
	}
	want := buildRecentMatches("player-1", history, stats, 10)
	if len(want) != 2 || want[0].ID != "match-a" || want[0].Stats == nil || want[0].Stats.Kills != 28 {
		t.Fatalf("sequential reference is not the expected fixture merge: %+v", want)
	}

	got, err := client.RecentMatches(ctx, "player-1", 10)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("concurrent merge = %+v, want sequential merge %+v", got, want)
	}
}

func TestClientCredentialSurfaceStaysRedacted(t *testing.T) {
	t.Parallel()
	const apiKey = "faceit-secret-key-value"
	configured, err := New(Options{APIKey: apiKey})
	if err != nil {
		t.Fatal(err)
	}
	unconfigured, err := New(Options{})
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name   string
		client *Client
		want   string
	}{
		{name: "nil client", client: nil, want: `{"configured":false}`},
		{name: "without key", client: unconfigured, want: `{"configured":false}`},
		{name: "with key", client: configured, want: `{"configured":true}`},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			body, err := test.client.MarshalJSON()
			if err != nil {
				t.Fatal(err)
			}
			if string(body) != test.want {
				t.Fatalf("MarshalJSON = %s, want %s", body, test.want)
			}
			for _, rendered := range []string{
				string(body),
				test.client.String(),
				test.client.GoString(),
				fmt.Sprintf("%v", test.client),
				fmt.Sprintf("%#v", test.client),
			} {
				if strings.Contains(rendered, apiKey) {
					t.Fatalf("rendered client leaked the API key: %s", rendered)
				}
			}
		})
	}
	if field, ok := reflect.TypeOf(Client{}).FieldByName("apiKey"); !ok || field.IsExported() {
		t.Fatal("Client.apiKey must stay unexported")
	}
	// The exported surface must not marshal the key through the struct either.
	body, err := json.Marshal(struct{ Client *Client }{Client: configured})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(body), apiKey) {
		t.Fatalf("embedded client leaked the API key: %s", body)
	}
}
