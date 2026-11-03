package faceit

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestDefaultSeedParsesEmbeddedRoster(t *testing.T) {
	t.Parallel()
	doc := DefaultSeed()
	if doc.SchemaVersion != SeedSchemaVersion {
		t.Fatalf("schema_version = %q, want %q", doc.SchemaVersion, SeedSchemaVersion)
	}
	if doc.GeneratedAt.IsZero() {
		t.Fatal("generated_at is zero, want the measurement date")
	}
	if len(doc.Regions) != len(rankingRegions) {
		t.Fatalf("regions = %v, want every allowlisted region covered", doc.Regions)
	}
	byZone := map[string][]SeedPlayer{}
	for i, player := range doc.Players {
		if !ValidPlayerID(player.PlayerID) || player.Nickname == "" || player.ELO <= 0 {
			t.Fatalf("player %d = %#v", i, player)
		}
		if _, ok := canonicalRankingRegion(player.Region); !ok {
			t.Fatalf("player %d region = %q, want an allowlisted region", i, player.Region)
		}
		countries, _ := zoneCountries(player.Zone)
		if !slices.Contains(countries, player.Country) {
			t.Fatalf("player %d country %q is not in zone %q", i, player.Country, player.Zone)
		}
		byZone[player.Zone] = append(byZone[player.Zone], player)
	}
	for _, zone := range Zones() {
		players := byZone[zone]
		if len(players) != SeedZoneLimit {
			t.Fatalf("zone %s players = %d, want the shipped %d", zone, len(players), SeedZoneLimit)
		}
		for i, player := range players {
			if player.Rank != i+1 {
				t.Fatalf("zone %s player %d rank = %d, want %d", zone, i, player.Rank, i+1)
			}
			if i > 0 && players[i-1].ELO < player.ELO {
				t.Fatalf("zone %s is not ordered by elo at %d: %d then %d", zone, i, players[i-1].ELO, player.ELO)
			}
		}
	}

	// The embedded document is parsed once, so a caller must not be able to
	// edit what the next caller sees.
	doc.Players[0].Nickname = "mutated"
	doc.Regions[0] = "mutated"
	fresh := DefaultSeed()
	if fresh.Players[0].Nickname == "mutated" || fresh.Regions[0] == "mutated" {
		t.Fatalf("DefaultSeed leaks its parsed document: %#v", fresh.Players[0])
	}
}

func TestSeedDocumentValidate(t *testing.T) {
	t.Parallel()
	valid := DefaultSeed()
	tests := []struct {
		name    string
		mutate  func(*SeedDocument)
		wantErr bool
	}{
		{name: "shipped document", mutate: func(*SeedDocument) {}},
		{name: "unknown schema", mutate: func(d *SeedDocument) { d.SchemaVersion = "cliphub.faceit-zones/v0" }, wantErr: true},
		{name: "missing generated_at", mutate: func(d *SeedDocument) { d.GeneratedAt = time.Time{} }, wantErr: true},
		{name: "no players", mutate: func(d *SeedDocument) { d.Players = nil }, wantErr: true},
		{name: "invalid player id", mutate: func(d *SeedDocument) { d.Players[3].PlayerID = "not a uuid" }, wantErr: true},
		{name: "missing nickname", mutate: func(d *SeedDocument) { d.Players[3].Nickname = "" }, wantErr: true},
		{name: "duplicate player", mutate: func(d *SeedDocument) { d.Players[3].PlayerID = d.Players[0].PlayerID }, wantErr: true},
		{name: "unknown zone", mutate: func(d *SeedDocument) { d.Players[3].Zone = "eu" }, wantErr: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			doc := valid.clone()
			test.mutate(&doc)
			err := doc.Validate()
			if test.wantErr != (err != nil) {
				t.Fatalf("Validate error = %v, wantErr %v", err, test.wantErr)
			}
		})
	}
}

func TestSeedStoreDocumentPrecedence(t *testing.T) {
	t.Parallel()
	override := `{
		"schema_version": "cliphub.faceit-zones/v1",
		"generated_at": "2026-09-04T10:00:00Z",
		"regions": ["EU"],
		"players": [{"player_id":"fresh-1","nickname":"fresher","country":"ru","region":"EU","position":1,"faceit_elo":5000,"game_skill_level":10,"zone":"cis","rank":1}]
	}`
	tests := []struct {
		name        string
		payload     string
		write       bool
		wantDefault bool
	}{
		{name: "no override uses the embedded default", wantDefault: true},
		{name: "valid override wins", payload: override, write: true},
		{name: "corrupt override falls back", payload: "{not json", write: true, wantDefault: true},
		{name: "legacy top-10 override falls back", payload: strings.ReplaceAll(override, "faceit-zones", "faceit-top10"), write: true, wantDefault: true},
		{name: "unknown schema falls back", payload: `{"schema_version":"v0","generated_at":"2026-09-04T10:00:00Z","players":[]}`, write: true, wantDefault: true},
		{name: "empty player list falls back", payload: `{"schema_version":"cliphub.faceit-zones/v1","generated_at":"2026-09-04T10:00:00Z","players":[]}`, write: true, wantDefault: true},
		{name: "invalid row falls back", payload: `{"schema_version":"cliphub.faceit-zones/v1","generated_at":"2026-09-04T10:00:00Z","players":[{"player_id":"bad id","nickname":"x"}]}`, write: true, wantDefault: true},
		{name: "oversized override falls back", payload: `{"schema_version":"cliphub.faceit-zones/v1","filler":"` + strings.Repeat("x", maxSeedFileBytes) + `"}`, write: true, wantDefault: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			path := filepath.Join(t.TempDir(), "zones.json")
			if test.write {
				if err := os.WriteFile(path, []byte(test.payload), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			store, err := NewSeedStore(path)
			if err != nil {
				t.Fatal(err)
			}
			doc := store.Document()
			if err := doc.Validate(); err != nil {
				t.Fatalf("Document returned an unusable roster: %v", err)
			}
			if test.wantDefault {
				if len(doc.Players) != len(DefaultSeed().Players) || doc.Players[0].PlayerID != DefaultSeed().Players[0].PlayerID {
					t.Fatalf("document = %#v, want the embedded default", doc.Players)
				}
				return
			}
			if len(doc.Players) != 1 || doc.Players[0].PlayerID != "fresh-1" || doc.Players[0].ELO != 5000 {
				t.Fatalf("document = %#v, want the override", doc.Players)
			}
			if !doc.GeneratedAt.Equal(time.Date(2026, time.September, 4, 10, 0, 0, 0, time.UTC)) {
				t.Fatalf("generated_at = %v, want the override's", doc.GeneratedAt)
			}
		})
	}
}

func TestSeedStoreDocumentFallsBackForNilStore(t *testing.T) {
	t.Parallel()
	var store *SeedStore
	if got := store.Document(); len(got.Players) != len(DefaultSeed().Players) {
		t.Fatalf("nil store document = %#v, want the embedded default", got)
	}
	if _, err := NewSeedStore(""); err == nil {
		t.Fatal("NewSeedStore(\"\") error = nil, want a rejection")
	}
	if _, err := (*SeedStore)(nil).Refresh(context.Background(), nil, 10); err == nil {
		t.Fatal("nil store Refresh error = nil, want a rejection")
	}
}

func TestSeedStoreRefreshCommitsZoneRosters(t *testing.T) {
	t.Parallel()
	population := []rankedFixture{
		{id: "es-0", region: "EU", elo: 5000},
		{id: "ru-0", region: "EU", country: "ru", elo: 4600},
		{id: "ua-0", region: "NA", country: "ua", elo: 4500},
		{id: "br-0", region: "SA", country: "br", elo: 4000},
		{id: "mx-0", region: "EU", country: "mx", elo: 3900},
	}
	client := leaderboardServer(t, population, nil)
	path := filepath.Join(t.TempDir(), "faceit", "zones.json")
	store, err := NewSeedStore(path)
	if err != nil {
		t.Fatal(err)
	}

	// Reading is explicit-refresh-only: it must not create the file, and it
	// must not need the network.
	if before := store.Document(); before.Players[0].PlayerID != DefaultSeed().Players[0].PlayerID {
		t.Fatalf("document before refresh = %#v, want the embedded default", before.Players[0])
	}
	if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("stat before refresh = %v, want the override to be absent", err)
	}

	doc, err := store.Refresh(context.Background(), client, 3)
	if err != nil {
		t.Fatal(err)
	}
	if doc.SchemaVersion != SeedSchemaVersion {
		t.Fatalf("schema_version = %q", doc.SchemaVersion)
	}
	want := time.Date(2026, time.September, 4, 0, 0, 0, 0, time.UTC)
	if !doc.GeneratedAt.Equal(want) {
		t.Fatalf("generated_at = %v, want the client clock %v", doc.GeneratedAt, want)
	}
	got := make([]string, 0, len(doc.Players))
	for _, player := range doc.Players {
		got = append(got, fmt.Sprintf("%s:%s#%d", player.Zone, player.PlayerID, player.Rank))
	}
	if fmt.Sprint(got) != "[cis:ru-0#1 cis:ua-0#2 latam:br-0#1 latam:mx-0#2]" {
		t.Fatalf("players = %v, want each zone ranked on its own and es-0 in none", got)
	}
	if got := strings.Join(doc.Regions, ","); got != "EU,NA,SA,OCE,SEA" {
		t.Fatalf("regions = %q, want every region read", got)
	}

	reopened, err := NewSeedStore(path)
	if err != nil {
		t.Fatal(err)
	}
	persisted := reopened.Document()
	if len(persisted.Players) != 4 || persisted.Players[2].PlayerID != "br-0" || persisted.Players[2].Zone != ZoneLATAM {
		t.Fatalf("persisted document = %#v", persisted.Players)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var shape struct {
		SchemaVersion string `json:"schema_version"`
		Players       []struct {
			PlayerID string `json:"player_id"`
			Zone     string `json:"zone"`
			Rank     int    `json:"rank"`
			Position int    `json:"position"`
		} `json:"players"`
	}
	if err := json.Unmarshal(raw, &shape); err != nil {
		t.Fatal(err)
	}
	if shape.SchemaVersion != SeedSchemaVersion || len(shape.Players) != 4 {
		t.Fatalf("committed file = %s", raw)
	}
	if row := shape.Players[3]; row.PlayerID != "mx-0" || row.Zone != "latam" || row.Rank != 2 || row.Position != 1 {
		t.Fatalf("committed row = %#v, want LATAM rank 2 and EU position 1", row)
	}
}

func TestSeedStoreRefreshRequiresClient(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "zones.json")
	store, err := NewSeedStore(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.Refresh(context.Background(), nil, 10); !errors.Is(err, ErrNotConfigured) {
		t.Fatalf("Refresh error = %v, want ErrNotConfigured", err)
	}
	unconfigured, err := New(Options{})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.Refresh(context.Background(), unconfigured, 10); !errors.Is(err, ErrNotConfigured) {
		t.Fatalf("Refresh error = %v, want ErrNotConfigured", err)
	}
	if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("stat = %v, want no file written by a failed refresh", err)
	}
}

func TestSeedStoreRefreshKeepsTheLastDocumentOnFailure(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "zones.json")
	store, err := NewSeedStore(path)
	if err != nil {
		t.Fatal(err)
	}
	client := leaderboardServer(t, []rankedFixture{{id: "eu-0", region: "EU", country: "ru", elo: 4600}}, nil)
	if _, err := store.Refresh(context.Background(), client, 5); err != nil {
		t.Fatal(err)
	}
	// One unreadable region is enough: a zone roster is all or nothing.
	broken := leaderboardServer(t, nil, map[string]bool{"SEA": true})
	if _, err := store.Refresh(context.Background(), broken, 5); err == nil {
		t.Fatal("Refresh error = nil, want the SEA outage to fail it")
	}
	doc := store.Document()
	if len(doc.Players) != 1 || doc.Players[0].PlayerID != "eu-0" {
		t.Fatalf("document = %#v, want the last good roster", doc.Players)
	}
}

func TestSeedStoreRefreshIfStaleReusesARecentRoster(t *testing.T) {
	t.Parallel()
	store, err := NewSeedStore(filepath.Join(t.TempDir(), "zones.json"))
	if err != nil {
		t.Fatal(err)
	}
	client := leaderboardServer(t, []rankedFixture{{id: "eu-0", region: "EU", country: "ru", elo: 4600}}, nil)
	if _, rebuilt, err := store.RefreshIfStale(context.Background(), client, 5, time.Hour); err != nil || !rebuilt {
		t.Fatalf("stale embedded roster: %v", err)
	}
	refreshedAt := store.Document().GeneratedAt
	// Every region fails, so any request would surface as an error.
	broken := leaderboardServer(t, nil, map[string]bool{"EU": true, "NA": true, "SA": true, "OCE": true, "SEA": true})
	broken.now = func() time.Time { return refreshedAt.Add(59 * time.Minute) }
	if _, rebuilt, err := store.RefreshIfStale(context.Background(), broken, 5, time.Hour); err != nil || rebuilt {
		t.Fatalf("recent roster was refetched: %v", err)
	}
	broken.now = func() time.Time { return refreshedAt.Add(time.Hour) }
	if _, _, err := store.RefreshIfStale(context.Background(), broken, 5, time.Hour); err == nil {
		t.Fatal("roster at maxAge was reused, want a refresh")
	}
	broken.now = func() time.Time { return refreshedAt.Add(-time.Minute) }
	if _, _, err := store.RefreshIfStale(context.Background(), broken, 5, time.Hour); err == nil {
		t.Fatal("roster from the future was reused, want a refresh")
	}
}

// A Studio start inside the ladder reuse window used to show the ELOs of the
// last ladder read; RefreshProfiles rereads each roster player instead.
func TestSeedStoreRefreshProfilesUpdatesELOsAndStopsOnAnAPIFailure(t *testing.T) {
	t.Parallel()
	generatedAt := time.Date(2026, 11, 3, 9, 0, 0, 0, time.UTC)
	refreshedAt := generatedAt.Add(time.Hour)
	var mu sync.Mutex
	var requested []string
	denied := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := strings.TrimPrefix(r.URL.Path, "/players/")
		mu.Lock()
		requested = append(requested, id)
		deny := denied
		mu.Unlock()
		switch {
		case deny:
			w.WriteHeader(http.StatusUnauthorized)
		case id == "cis-1":
			_, _ = w.Write([]byte(`{"player_id":"cis-1","nickname":"nipl-renamed","games":{"cs2":{"skill_level":10,"faceit_elo":4999}}}`))
		case id == "latam-1":
			// No CS2 stats: the ladder's ELO and level must survive.
			_, _ = w.Write([]byte(`{"player_id":"latam-1","nickname":"latam-nick"}`))
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(server.Close)
	client, err := New(Options{APIKey: "faceit-test-key", BaseURL: server.URL, HTTPClient: server.Client(),
		Now: func() time.Time { return refreshedAt }})
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "zones.json")
	seed := func(id, zone string, elo int) SeedPlayer {
		return SeedPlayer{RankedPlayer: RankedPlayer{PlayerID: id, Nickname: id + "-nick", Region: "EU", ELO: elo, SkillLevel: 10}, Zone: zone}
	}
	doc := SeedDocument{SchemaVersion: SeedSchemaVersion, GeneratedAt: generatedAt, Regions: []string{"EU"}, Players: []SeedPlayer{
		seed("cis-1", ZoneCIS, 4000), seed("cis-followed", ZoneCIS, 3900), seed("cis-gone", ZoneCIS, 3800), seed("latam-1", ZoneLATAM, 3700),
	}}
	raw, err := json.Marshal(doc)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, raw, 0o600); err != nil {
		t.Fatal(err)
	}
	store, err := NewSeedStore(path)
	if err != nil {
		t.Fatal(err)
	}

	if err := store.RefreshProfiles(context.Background(), client, map[string]bool{"cis-followed": true}); err != nil {
		t.Fatalf("RefreshProfiles: %v", err)
	}
	mu.Lock()
	calls := strings.Join(requested, ",")
	mu.Unlock()
	if calls != "cis-1,cis-gone,latam-1" {
		t.Fatalf("requested %s, want every roster player but the followed one", calls)
	}
	got := store.Document()
	if !got.UpdatedAt().Equal(refreshedAt) || !got.GeneratedAt.Equal(generatedAt) {
		t.Fatalf("updated_at=%v generated_at=%v, want the profile read time and the ladder time kept", got.UpdatedAt(), got.GeneratedAt)
	}
	if p := got.Players[0]; p.Nickname != "nipl-renamed" || p.ELO != 4999 {
		t.Fatalf("cis-1 = %#v, want the live profile", p)
	}
	if got.Players[1].ELO != 3900 || got.Players[2].ELO != 3800 || got.Players[3].ELO != 3700 || got.Players[3].SkillLevel != 10 {
		t.Fatalf("players = %#v, want skipped, missing and stat-less rows unchanged", got.Players)
	}

	// An API failure (offline, rate limited, revoked key) stops at the first
	// call and keeps the stored roster.
	mu.Lock()
	requested = nil
	denied = true
	mu.Unlock()
	if err := store.RefreshProfiles(context.Background(), client, nil); !errors.Is(err, ErrUnauthorized) {
		t.Fatalf("RefreshProfiles error = %v, want ErrUnauthorized", err)
	}
	mu.Lock()
	defer mu.Unlock()
	if len(requested) != 1 {
		t.Fatalf("requested %v after a failure, want one call", requested)
	}
	if after := store.Document(); after.Players[0].ELO != 4999 || !after.UpdatedAt().Equal(refreshedAt) {
		t.Fatalf("after failure = %#v, want the last roster", after)
	}
}
