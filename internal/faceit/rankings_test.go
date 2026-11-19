package faceit

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

const rankingsTestKey = "faceit-rankings-secret"

// rankingsServer serves one canned body for the CS2 leaderboard endpoint and
// counts how many requests reached it, so a rejected region can be shown to
// never leave the process.
func rankingsServer(t *testing.T, body string) (*Client, *int) {
	t.Helper()
	var mu sync.Mutex
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		requests++
		mu.Unlock()
		if got, want := r.Header.Get("Authorization"), "Bearer "+rankingsTestKey; got != want {
			t.Errorf("authorization = %q, want %q", got, want)
		}
		if !strings.HasPrefix(r.URL.Path, "/rankings/games/cs2/regions/") {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(server.Close)
	client, err := New(Options{APIKey: rankingsTestKey, BaseURL: server.URL, HTTPClient: server.Client()})
	if err != nil {
		t.Fatal(err)
	}
	return client, &requests
}

func TestRankingsDecodesRegionalLeaderboard(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		// region and country are the arguments; body is what the leaderboard
		// endpoint answers with.
		region  string
		country string
		body    string
		want    []RankedPlayer
		// wantErr says the call must fail; wantSentinel, when set, says with
		// which sentinel. Argument rejections have no sentinel to match on.
		wantErr      bool
		wantSentinel error
		wantRequest  bool
	}{
		{
			name:   "documented level field",
			region: "EU",
			body: `{"items":[
				{"player_id":"e5e8e2a6-d716-4493-b949-e16965f41654","nickname":"donk666","country":"kr","position":1,"faceit_elo":4636,"game_skill_level":10},
				{"player_id":"be994de3-89c8-4d40-9286-c14035208743","nickname":"-SYPH0","country":"kz","position":2,"faceit_elo":4409,"game_skill_level":10}
			],"start":0,"end":2}`,
			want: []RankedPlayer{
				{PlayerID: "e5e8e2a6-d716-4493-b949-e16965f41654", Nickname: "donk666", Country: "kr", Region: "EU", Position: 1, ELO: 4636, SkillLevel: 10},
				{PlayerID: "be994de3-89c8-4d40-9286-c14035208743", Nickname: "-SYPH0", Country: "kz", Region: "EU", Position: 2, ELO: 4409, SkillLevel: 10},
			},
			wantRequest: true,
		},
		{
			name:   "legacy skill_level field",
			region: "NA",
			body:   `{"items":[{"player_id":"player-1","nickname":"nafany","country":"us","position":1,"faceit_elo":3600,"skill_level":10}]}`,
			want: []RankedPlayer{
				{PlayerID: "player-1", Nickname: "nafany", Country: "us", Region: "NA", Position: 1, ELO: 3600, SkillLevel: 10},
			},
			wantRequest: true,
		},
		{
			name:        "lowercase region is canonicalised",
			region:      "  sea ",
			body:        `{"items":[{"player_id":"player-2","nickname":"kaze","position":1,"faceit_elo":3000,"game_skill_level":10}]}`,
			want:        []RankedPlayer{{PlayerID: "player-2", Nickname: "kaze", Region: "SEA", Position: 1, ELO: 3000, SkillLevel: 10}},
			wantRequest: true,
		},
		{
			name:         "invalid player id fails the page",
			region:       "EU",
			body:         `{"items":[{"player_id":"ok-1","nickname":"good","position":1,"faceit_elo":4000,"game_skill_level":10},{"player_id":"not a uuid","nickname":"bad","position":2,"faceit_elo":3900,"game_skill_level":10}]}`,
			wantErr:      true,
			wantSentinel: ErrInvalidResponse,
			wantRequest:  true,
		},
		{
			name:         "missing nickname fails the page",
			region:       "EU",
			body:         `{"items":[{"player_id":"ok-1","nickname":"","position":1,"faceit_elo":4000,"game_skill_level":10}]}`,
			wantErr:      true,
			wantSentinel: ErrInvalidResponse,
			wantRequest:  true,
		},
		{
			name:         "reflected credential fails the page",
			region:       "EU",
			body:         `{"items":[{"player_id":"ok-1","nickname":"` + rankingsTestKey + `","position":1,"faceit_elo":4000,"game_skill_level":10}]}`,
			wantErr:      true,
			wantSentinel: ErrInvalidResponse,
			wantRequest:  true,
		},
		{
			name:    "region outside the allowlist",
			region:  "GLOBAL",
			body:    `{"items":[]}`,
			wantErr: true,
		},
		{
			name:    "empty region",
			region:  "",
			body:    `{"items":[]}`,
			wantErr: true,
		},
		{
			name:    "invalid country filter",
			region:  "EU",
			country: "es/../",
			body:    `{"items":[]}`,
			wantErr: true,
		},
		{
			name:        "empty leaderboard",
			region:      "OCE",
			body:        `{"items":[]}`,
			want:        []RankedPlayer{},
			wantRequest: true,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			client, requests := rankingsServer(t, test.body)
			got, err := client.Rankings(context.Background(), test.region, test.country, 0, 10)
			if test.wantErr {
				if err == nil {
					t.Fatalf("Rankings(%q, %q) error = nil, want a rejection", test.region, test.country)
				}
				if test.wantSentinel != nil && !errors.Is(err, test.wantSentinel) {
					t.Fatalf("error = %v, want %v", err, test.wantSentinel)
				}
				if got != nil {
					t.Fatalf("players = %#v, want no partial rows", got)
				}
			} else {
				if err != nil {
					t.Fatal(err)
				}
				if fmt.Sprint(got) != fmt.Sprint(test.want) {
					t.Fatalf("players = %#v, want %#v", got, test.want)
				}
			}
			if hit := *requests > 0; hit != test.wantRequest {
				t.Fatalf("request reached upstream = %v, want %v", hit, test.wantRequest)
			}
		})
	}
}

func TestRankingsSendsPagingAndCountryQuery(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name       string
		country    string
		offset     int
		limit      int
		wantOffset string
		wantLimit  string
		wantPath   string
	}{
		{name: "defaults the limit", limit: 0, wantOffset: "0", wantLimit: "10", wantPath: "/rankings/games/cs2/regions/EU"},
		{name: "clamps the limit", limit: 1000, wantOffset: "0", wantLimit: "100", wantPath: "/rankings/games/cs2/regions/EU"},
		{name: "floors a negative offset", offset: -5, limit: 3, wantOffset: "0", wantLimit: "3", wantPath: "/rankings/games/cs2/regions/EU"},
		{name: "passes the country filter", country: "es", offset: 20, limit: 5, wantOffset: "20", wantLimit: "5", wantPath: "/rankings/games/cs2/regions/EU"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			var got *http.Request
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				got = r
				_, _ = w.Write([]byte(`{"items":[]}`))
			}))
			defer server.Close()
			client, err := New(Options{APIKey: rankingsTestKey, BaseURL: server.URL, HTTPClient: server.Client()})
			if err != nil {
				t.Fatal(err)
			}
			if _, err := client.Rankings(context.Background(), "EU", test.country, test.offset, test.limit); err != nil {
				t.Fatal(err)
			}
			if got == nil {
				t.Fatal("no request reached the server")
			}
			if got.URL.Path != test.wantPath {
				t.Fatalf("path = %q, want %q", got.URL.Path, test.wantPath)
			}
			assertQueryValue(t, got.URL.Query(), "offset", test.wantOffset)
			assertQueryValue(t, got.URL.Query(), "limit", test.wantLimit)
			if test.country != "" {
				assertQueryValue(t, got.URL.Query(), "country", test.country)
			} else if _, ok := got.URL.Query()["country"]; ok {
				t.Fatalf("query = %v, want no country filter", got.URL.Query())
			}
		})
	}
}

// rankedFixture is a synthetic FACEIT population used to prove the merge.
// A fixture without a country reports "es", which is in no zone.
type rankedFixture struct {
	id      string
	region  string
	country string
	elo     int
}

func (f rankedFixture) countryCode() string {
	if f.country == "" {
		return "es"
	}
	return f.country
}

// leaderboardServer serves each region's own top `limit` out of the population,
// exactly as FACEIT does: sorted by ELO with a position counted inside the
// region, narrowed to the country query when there is one. A broken key is a
// region ("EU") or a region/country ladder ("EU/ru").
func leaderboardServer(t *testing.T, population []rankedFixture, broken map[string]bool) (*Client, *atomic.Int32) {
	t.Helper()
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		region := strings.TrimPrefix(r.URL.Path, "/rankings/games/cs2/regions/")
		if region == r.URL.Path || region == "" {
			http.NotFound(w, r)
			return
		}
		country := r.URL.Query().Get("country")
		if broken[region] || broken[region+"/"+country] {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		limit, err := strconv.Atoi(r.URL.Query().Get("limit"))
		if err != nil || limit <= 0 {
			t.Errorf("limit = %q", r.URL.Query().Get("limit"))
			limit = 10
		}
		var rows []rankedFixture
		for _, player := range population {
			if player.region == region && (country == "" || player.countryCode() == country) {
				rows = append(rows, player)
			}
		}
		sort.Slice(rows, func(i, j int) bool {
			if rows[i].elo != rows[j].elo {
				return rows[i].elo > rows[j].elo
			}
			return rows[i].id < rows[j].id
		})
		if len(rows) > limit {
			rows = rows[:limit]
		}
		items := make([]apiRankedPlayer, 0, len(rows))
		for i, row := range rows {
			items = append(items, apiRankedPlayer{
				PlayerID:       row.id,
				Nickname:       row.id + "-nick",
				Country:        row.countryCode(),
				Position:       i + 1,
				FaceitELO:      row.elo,
				GameSkillLevel: 10,
			})
		}
		body, err := json.Marshal(apiRankingList{Items: items, End: len(items)})
		if err != nil {
			t.Errorf("marshal %s leaderboard: %v", region, err)
			http.Error(w, "marshal", http.StatusInternalServerError)
			return
		}
		_, _ = w.Write(body)
	}))
	t.Cleanup(server.Close)
	client, err := New(Options{
		APIKey:     rankingsTestKey,
		BaseURL:    server.URL,
		HTTPClient: server.Client(),
		Now:        func() time.Time { return time.Date(2026, time.September, 4, 0, 0, 0, 0, time.UTC) },
	})
	if err != nil {
		t.Fatal(err)
	}
	return client, &requests
}

func TestRankingsRequiresAPIKey(t *testing.T) {
	t.Parallel()
	var nilClient *Client
	unconfigured, err := New(Options{})
	if err != nil {
		t.Fatal(err)
	}
	for name, client := range map[string]*Client{"nil client": nilClient, "no api key": unconfigured} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			if _, err := client.Rankings(context.Background(), "EU", "", 0, 10); !errors.Is(err, ErrNotConfigured) {
				t.Fatalf("Rankings error = %v, want ErrNotConfigured", err)
			}
			if _, err := client.RankingTop(context.Background(), RankingEntry{kind: RankingZone, code: ZoneCIS}); !errors.Is(err, ErrNotConfigured) {
				t.Fatalf("RankingTop error = %v, want ErrNotConfigured", err)
			}
		})
	}
}

func TestClampRankingLimit(t *testing.T) {
	t.Parallel()
	tests := []struct {
		in   int
		want int
	}{
		{in: 0, want: defaultRankingLimit},
		{in: -3, want: defaultRankingLimit},
		{in: 1, want: 1},
		{in: 10, want: 10},
		{in: maxRankingLimit, want: maxRankingLimit},
		{in: maxRankingLimit + 1, want: maxRankingLimit},
	}
	for _, test := range tests {
		if got := clampRankingLimit(test.in); got != test.want {
			t.Fatalf("clampRankingLimit(%d) = %d, want %d", test.in, got, test.want)
		}
	}
}

func TestCanonicalRankingRegion(t *testing.T) {
	t.Parallel()
	tests := []struct {
		in   string
		want string
		ok   bool
	}{
		{in: "EU", want: "EU", ok: true},
		{in: "eu", want: "EU", ok: true},
		{in: " sea ", want: "SEA", ok: true},
		{in: "NA", want: "NA", ok: true},
		{in: "SA", want: "SA", ok: true},
		{in: "OCE", want: "OCE", ok: true},
		{in: "", ok: false},
		{in: "GLOBAL", ok: false},
		{in: "EU/../NA", ok: false},
	}
	for _, test := range tests {
		got, ok := canonicalRankingRegion(test.in)
		if got != test.want || ok != test.ok {
			t.Fatalf("canonicalRankingRegion(%q) = %q, %v, want %q, %v", test.in, got, ok, test.want, test.ok)
		}
	}
}
