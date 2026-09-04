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
			name:        "invalid player id fails the page",
			region:      "EU",
			body:        `{"items":[{"player_id":"ok-1","nickname":"good","position":1,"faceit_elo":4000,"game_skill_level":10},{"player_id":"not a uuid","nickname":"bad","position":2,"faceit_elo":3900,"game_skill_level":10}]}`,
			wantErr:      true,
			wantSentinel: ErrInvalidResponse,
			wantRequest:  true,
		},
		{
			name:        "missing nickname fails the page",
			region:      "EU",
			body:        `{"items":[{"player_id":"ok-1","nickname":"","position":1,"faceit_elo":4000,"game_skill_level":10}]}`,
			wantErr:      true,
			wantSentinel: ErrInvalidResponse,
			wantRequest:  true,
		},
		{
			name:        "reflected credential fails the page",
			region:      "EU",
			body:        `{"items":[{"player_id":"ok-1","nickname":"` + rankingsTestKey + `","position":1,"faceit_elo":4000,"game_skill_level":10}]}`,
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
type rankedFixture struct {
	id     string
	region string
	elo    int
}

// leaderboardServer serves each region's own top `limit` out of the population,
// exactly as FACEIT does: sorted by ELO with a position counted inside the
// region.
func leaderboardServer(t *testing.T, population []rankedFixture, broken map[string]bool) *Client {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		region := strings.TrimPrefix(r.URL.Path, "/rankings/games/cs2/regions/")
		if region == r.URL.Path || region == "" {
			http.NotFound(w, r)
			return
		}
		if broken[region] {
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
			if player.region == region {
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
				Country:        "es",
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
	return client
}

// globalTopOf is the answer the merge has to reproduce: the whole population
// ranked, truncated to limit.
func globalTopOf(population []rankedFixture, limit int) []string {
	ranked := append([]rankedFixture(nil), population...)
	sort.Slice(ranked, func(i, j int) bool {
		if ranked[i].elo != ranked[j].elo {
			return ranked[i].elo > ranked[j].elo
		}
		return ranked[i].id < ranked[j].id
	})
	if len(ranked) > limit {
		ranked = ranked[:limit]
	}
	out := make([]string, 0, len(ranked))
	for _, player := range ranked {
		out = append(out, player.id)
	}
	return out
}

// TestGlobalTopEqualsTheTrueGlobalRanking encodes the invariant the merge rests
// on: a player inside the global top N is necessarily inside their own
// region's top N, so N rows per region are enough to reproduce the global
// ranking exactly. The population is deliberately lopsided the way the real
// one is — the strong region's Nth outranks every other region's 1st.
func TestGlobalTopEqualsTheTrueGlobalRanking(t *testing.T) {
	t.Parallel()
	var population []rankedFixture
	for i := range 30 {
		population = append(population, rankedFixture{id: fmt.Sprintf("eu-%02d", i), region: "EU", elo: 4600 - i*10})
	}
	for i := range 30 {
		population = append(population, rankedFixture{id: fmt.Sprintf("na-%02d", i), region: "NA", elo: 4000 - i*10})
	}
	for i := range 12 {
		population = append(population, rankedFixture{id: fmt.Sprintf("sa-%02d", i), region: "SA", elo: 3800 - i*10})
	}
	population = append(population,
		rankedFixture{id: "oce-00", region: "OCE", elo: 3500},
		rankedFixture{id: "sea-00", region: "SEA", elo: 3400},
	)

	client := leaderboardServer(t, population, nil)
	for _, limit := range []int{1, 5, 10, 25} {
		players, err := client.GlobalTop(context.Background(), limit)
		if err != nil {
			t.Fatalf("limit %d: %v", limit, err)
		}
		got := make([]string, 0, len(players))
		for _, player := range players {
			got = append(got, player.PlayerID)
		}
		want := globalTopOf(population, limit)
		if fmt.Sprint(got) != fmt.Sprint(want) {
			t.Fatalf("limit %d: global top = %v, want %v", limit, got, want)
		}
	}

	players, err := client.GlobalTop(context.Background(), 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(players) != 10 {
		t.Fatalf("players = %d, want 10", len(players))
	}
	for _, player := range players {
		if player.Region != "EU" {
			t.Fatalf("player %#v, want the lopsided population to leave only EU in the top 10", player)
		}
	}
	if players[0].ELO < players[9].ELO {
		t.Fatalf("players are not ordered by elo: %d then %d", players[0].ELO, players[9].ELO)
	}
	// Region position is preserved, so a seeded row can say where it came from.
	if players[9].Position != 10 {
		t.Fatalf("tenth player position = %d, want its EU position 10", players[9].Position)
	}
}

func TestGlobalTopBreaksTiesByPlayerID(t *testing.T) {
	t.Parallel()
	population := []rankedFixture{
		{id: "zz-tied", region: "EU", elo: 4000},
		{id: "aa-tied", region: "NA", elo: 4000},
		{id: "mm-tied", region: "SA", elo: 4000},
		{id: "low", region: "SEA", elo: 3000},
	}
	client := leaderboardServer(t, population, nil)
	for attempt := range 5 {
		players, err := client.GlobalTop(context.Background(), 4)
		if err != nil {
			t.Fatal(err)
		}
		got := make([]string, 0, len(players))
		for _, player := range players {
			got = append(got, player.PlayerID)
		}
		want := "[aa-tied mm-tied zz-tied low]"
		if fmt.Sprint(got) != want {
			t.Fatalf("attempt %d: order = %v, want %s", attempt, got, want)
		}
	}
}

func TestGlobalTopDropsDuplicatePlayerAcrossRegions(t *testing.T) {
	t.Parallel()
	population := []rankedFixture{
		{id: "double", region: "EU", elo: 4200},
		{id: "double", region: "NA", elo: 4100},
		{id: "single", region: "SA", elo: 4000},
	}
	client := leaderboardServer(t, population, nil)
	players, err := client.GlobalTop(context.Background(), 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(players) != 2 {
		t.Fatalf("players = %#v, want the duplicate collapsed", players)
	}
	if players[0].PlayerID != "double" || players[0].ELO != 4200 {
		t.Fatalf("first player = %#v, want the higher-elo copy", players[0])
	}
}

func TestGlobalTopRegionOutages(t *testing.T) {
	t.Parallel()
	population := []rankedFixture{
		{id: "eu-0", region: "EU", elo: 4600},
		{id: "na-0", region: "NA", elo: 4000},
		{id: "sa-0", region: "SA", elo: 3900},
		{id: "oce-0", region: "OCE", elo: 3800},
		{id: "sea-0", region: "SEA", elo: 3700},
	}
	tests := []struct {
		name        string
		broken      []string
		wantIDs     string
		wantRegions string
		wantErr     bool
	}{
		{name: "all regions answer", wantIDs: "[eu-0 na-0 sa-0 oce-0 sea-0]", wantRegions: "[EU NA SA OCE SEA]"},
		{name: "one region down", broken: []string{"EU"}, wantIDs: "[na-0 sa-0 oce-0 sea-0]", wantRegions: "[NA SA OCE SEA]"},
		{name: "only one region up", broken: []string{"EU", "NA", "SA", "OCE"}, wantIDs: "[sea-0]", wantRegions: "[SEA]"},
		{name: "every region down", broken: []string{"EU", "NA", "SA", "OCE", "SEA"}, wantErr: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			broken := make(map[string]bool, len(test.broken))
			for _, region := range test.broken {
				broken[region] = true
			}
			client := leaderboardServer(t, population, broken)
			players, regions, err := client.globalTop(context.Background(), 10)
			if test.wantErr {
				if err == nil {
					t.Fatal("error = nil, want a failure when no region answers")
				}
				if !errors.Is(err, ErrUnavailable) {
					t.Fatalf("error = %v, want the per-region cause joined in", err)
				}
				if players != nil || regions != nil {
					t.Fatalf("players = %#v regions = %#v, want none", players, regions)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			ids := make([]string, 0, len(players))
			for _, player := range players {
				ids = append(ids, player.PlayerID)
			}
			if fmt.Sprint(ids) != test.wantIDs {
				t.Fatalf("players = %v, want %s", ids, test.wantIDs)
			}
			if fmt.Sprint(regions) != test.wantRegions {
				t.Fatalf("regions = %v, want %s", regions, test.wantRegions)
			}
		})
	}
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
			if _, err := client.GlobalTop(context.Background(), 10); !errors.Is(err, ErrNotConfigured) {
				t.Fatalf("GlobalTop error = %v, want ErrNotConfigured", err)
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
