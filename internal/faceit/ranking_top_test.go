package faceit

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
)

func mustRankingEntry(t *testing.T, kind, code string) RankingEntry {
	t.Helper()
	entry, err := ParseRankingEntry(kind, code)
	if err != nil {
		t.Fatalf("ParseRankingEntry(%q, %q): %v", kind, code, err)
	}
	return entry
}

func rankingIDs(players []RankingPlayer) string {
	out := make([]string, 0, len(players))
	for _, player := range players {
		out = append(out, fmt.Sprintf("%d:%s:%d", player.Rank, player.PlayerID, player.ELO))
	}
	return fmt.Sprint(out)
}

// A country is read on every region, since players queue anywhere: one read
// per region, merged by ELO, a player seen twice keeping their higher ELO.
func TestRankingTopOfACountryMergesEveryRegionByELO(t *testing.T) {
	t.Parallel()
	population := []rankedFixture{
		{id: "es-eu", region: "EU", country: "es", elo: 3000},
		{id: "es-na", region: "NA", country: "es", elo: 3500},
		{id: "double", region: "EU", country: "es", elo: 3200},
		{id: "double", region: "SA", country: "es", elo: 3400},
		{id: "pt-eu", region: "EU", country: "pt", elo: 4000},
	}
	client, requests := leaderboardServer(t, population, nil)
	players, err := client.RankingTop(context.Background(), mustRankingEntry(t, RankingCountry, "ES"))
	if err != nil {
		t.Fatal(err)
	}
	if got, want := rankingIDs(players), "[1:es-na:3500 2:double:3400 3:es-eu:3000]"; got != want {
		t.Fatalf("ranking = %s, want %s", got, want)
	}
	if players[1].Region != "SA" {
		t.Fatalf("duplicate kept region %q, want the higher-ELO SA row", players[1].Region)
	}
	if got := requests.Load(); got != int32(len(rankingRegions)) {
		t.Fatalf("requests = %d, want one per region", got)
	}
}

// The ranking stops at RankingSize even when the regions together list more.
func TestRankingTopCutsAtTheTop100(t *testing.T) {
	t.Parallel()
	var population []rankedFixture
	for i := range 130 {
		region := "EU"
		if i%2 == 1 {
			region = "NA"
		}
		population = append(population, rankedFixture{id: fmt.Sprintf("es-%03d", i), region: region, country: "es", elo: 5000 - i})
	}
	client, _ := leaderboardServer(t, population, nil)
	players, err := client.RankingTop(context.Background(), mustRankingEntry(t, RankingCountry, "es"))
	if err != nil {
		t.Fatal(err)
	}
	if len(players) != RankingSize {
		t.Fatalf("players = %d, want %d", len(players), RankingSize)
	}
	if last := players[len(players)-1]; last.Rank != 100 || last.PlayerID != "es-099" || last.ELO != 4901 {
		t.Fatalf("last row = %+v, want es-099 at rank 100", last)
	}
}

// A region is one FACEIT ladder: one request, and its own position is the rank.
func TestRankingTopOfARegionIsOneRequestWithTheRegionalPosition(t *testing.T) {
	t.Parallel()
	population := []rankedFixture{
		{id: "eu-1", region: "EU", country: "pl", elo: 4100},
		{id: "eu-2", region: "EU", country: "es", elo: 3900},
		{id: "na-1", region: "NA", country: "us", elo: 4500},
	}
	client, requests := leaderboardServer(t, population, nil)
	players, err := client.RankingTop(context.Background(), mustRankingEntry(t, RankingRegion, "eu"))
	if err != nil {
		t.Fatal(err)
	}
	if got, want := rankingIDs(players), "[1:eu-1:4100 2:eu-2:3900]"; got != want {
		t.Fatalf("ranking = %s, want %s", got, want)
	}
	if got := requests.Load(); got != 1 {
		t.Fatalf("requests = %d, want 1", got)
	}
}

func TestRankingTopOfAZoneBreaksTiesByPlayerID(t *testing.T) {
	t.Parallel()
	population := []rankedFixture{
		{id: "zz-tied", region: "EU", country: "ru", elo: 4000},
		{id: "aa-tied", region: "NA", country: "ua", elo: 4000},
		{id: "mm-tied", region: "SA", country: "kz", elo: 4000},
		{id: "low", region: "SEA", country: "by", elo: 3000},
	}
	client, _ := leaderboardServer(t, population, nil)
	for attempt := range 5 {
		players, err := client.RankingTop(context.Background(), mustRankingEntry(t, RankingZone, "CIS"))
		if err != nil {
			t.Fatal(err)
		}
		if got, want := rankingIDs(players), "[1:aa-tied:4000 2:mm-tied:4000 3:zz-tied:4000 4:low:3000]"; got != want {
			t.Fatalf("attempt %d: ranking = %s, want %s", attempt, got, want)
		}
	}
}

// A zone is every one of its countries read on every region: a player outside
// the zone never leaks in however high their ELO, and a zone player listed on
// two ladders takes one place, with the higher ELO.
func TestRankingTopOfAZoneIsTheUnionOfItsCountries(t *testing.T) {
	t.Parallel()
	population := []rankedFixture{
		{id: "es-top", region: "EU", elo: 5000},
		{id: "ru-eu", region: "EU", country: "ru", elo: 4600},
		{id: "ua-na", region: "NA", country: "ua", elo: 4500},
		{id: "kz-sea", region: "SEA", country: "kz", elo: 4400},
		{id: "double", region: "EU", country: "ru", elo: 4200},
		{id: "double", region: "NA", country: "ru", elo: 4300},
		{id: "mx-eu", region: "EU", country: "mx", elo: 3900},
		{id: "br-sa", region: "SA", country: "br", elo: 3800},
		{id: "us-na", region: "NA", country: "us", elo: 4200},
	}
	client, requests := leaderboardServer(t, population, nil)
	for zone, want := range map[string]string{
		ZoneCIS:   "[1:ru-eu:4600 2:ua-na:4500 3:kz-sea:4400 4:double:4300]",
		ZoneLATAM: "[1:mx-eu:3900 2:br-sa:3800]",
	} {
		before := requests.Load()
		players, err := client.RankingTop(context.Background(), mustRankingEntry(t, RankingZone, zone))
		if err != nil {
			t.Fatalf("%s: %v", zone, err)
		}
		if got := rankingIDs(players); got != want {
			t.Fatalf("%s ranking = %s, want %s", zone, got, want)
		}
		countries, _ := zoneCountries(zone)
		if got, want := requests.Load()-before, int32(len(countries)*len(rankingRegions)); got != want {
			t.Fatalf("%s requests = %d, want %d", zone, got, want)
		}
	}
}

// A ranking missing one ladder would look complete, so one failed ladder fails it.
func TestRankingTopFailsOnAnyMissingLadder(t *testing.T) {
	t.Parallel()
	population := []rankedFixture{{id: "ru-eu", region: "EU", country: "ru", elo: 4600}}
	client, _ := leaderboardServer(t, population, map[string]bool{"OCE/md": true, "SA/es": true})
	if _, err := client.RankingTop(context.Background(), mustRankingEntry(t, RankingZone, ZoneCIS)); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("CIS error = %v, want the failed OCE/md ladder", err)
	}
	if _, err := client.RankingTop(context.Background(), mustRankingEntry(t, RankingCountry, "es")); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("ES error = %v, want the failed SA/es ladder", err)
	}
	if players, err := client.RankingTop(context.Background(), mustRankingEntry(t, RankingZone, ZoneLATAM)); err != nil || len(players) != 0 {
		t.Fatalf("LATAM = %#v, %v; want an empty ranking from healthy ladders", players, err)
	}
}

func TestParseRankingEntryRejectsUnknownCodes(t *testing.T) {
	t.Parallel()
	for _, test := range []struct{ kind, code string }{
		{kind: RankingCountry, code: "esp"},
		{kind: RankingCountry, code: "e1"},
		{kind: RankingCountry, code: ""},
		{kind: RankingRegion, code: "GLOBAL"},
		{kind: RankingZone, code: "eu"},
		{kind: "team", code: "es"},
	} {
		if entry, err := ParseRankingEntry(test.kind, test.code); err == nil {
			t.Fatalf("ParseRankingEntry(%q, %q) = %v, want a rejection", test.kind, test.code, entry)
		}
	}
}

// FACEIT answers a burst with 429: the client's retries absorb a short one,
// and a lasting one comes back as ErrRateLimited for the page to explain.
func TestRankingTopRetriesARateLimitAndReportsALastingOne(t *testing.T) {
	t.Parallel()
	var limited sync.Map
	var always atomic.Bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		key := r.URL.Path + "?" + r.URL.Query().Get("country")
		if _, seen := limited.LoadOrStore(key, true); !seen || always.Load() {
			w.Header().Set("Retry-After", "0")
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		items := `[]`
		if strings.HasSuffix(r.URL.Path, "/NA") {
			items = `[{"player_id":"es-1","nickname":"mopoz","country":"es","position":1,"faceit_elo":3900,"game_skill_level":10}]`
		}
		_, _ = w.Write([]byte(`{"items":` + items + `}`))
	}))
	t.Cleanup(server.Close)
	client, err := New(Options{APIKey: rankingsTestKey, BaseURL: server.URL, HTTPClient: server.Client()})
	if err != nil {
		t.Fatal(err)
	}
	players, err := client.RankingTop(context.Background(), mustRankingEntry(t, RankingCountry, "es"))
	if err != nil {
		t.Fatalf("one 429 per ladder: %v, want the retries to absorb it", err)
	}
	if got, want := rankingIDs(players), "[1:es-1:3900]"; got != want {
		t.Fatalf("ranking = %s, want %s", got, want)
	}
	always.Store(true)
	if _, err := client.RankingTop(context.Background(), mustRankingEntry(t, RankingRegion, "EU")); !errors.Is(err, ErrRateLimited) {
		t.Fatalf("lasting 429 error = %v, want ErrRateLimited", err)
	}
}
