package faceit

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
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
	if len(doc.Players) != 10 {
		t.Fatalf("players = %d, want the shipped top 10", len(doc.Players))
	}
	if len(doc.Regions) != len(rankingRegions) {
		t.Fatalf("regions = %v, want every allowlisted region covered", doc.Regions)
	}
	for i, player := range doc.Players {
		if !ValidPlayerID(player.PlayerID) || player.Nickname == "" || player.ELO <= 0 {
			t.Fatalf("player %d = %#v", i, player)
		}
		if _, ok := canonicalRankingRegion(player.Region); !ok {
			t.Fatalf("player %d region = %q, want an allowlisted region", i, player.Region)
		}
		if player.Rank != i+1 {
			t.Fatalf("player %d rank = %d, want %d", i, player.Rank, i+1)
		}
		if i > 0 && doc.Players[i-1].ELO < player.ELO {
			t.Fatalf("players are not ordered by elo at %d: %d then %d", i, doc.Players[i-1].ELO, player.ELO)
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
		{name: "unknown schema", mutate: func(d *SeedDocument) { d.SchemaVersion = "cliphub.faceit-top10/v0" }, wantErr: true},
		{name: "missing generated_at", mutate: func(d *SeedDocument) { d.GeneratedAt = time.Time{} }, wantErr: true},
		{name: "no players", mutate: func(d *SeedDocument) { d.Players = nil }, wantErr: true},
		{name: "invalid player id", mutate: func(d *SeedDocument) { d.Players[3].PlayerID = "not a uuid" }, wantErr: true},
		{name: "missing nickname", mutate: func(d *SeedDocument) { d.Players[3].Nickname = "" }, wantErr: true},
		{name: "duplicate player", mutate: func(d *SeedDocument) { d.Players[3].PlayerID = d.Players[0].PlayerID }, wantErr: true},
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
		"schema_version": "cliphub.faceit-top10/v1",
		"generated_at": "2026-09-04T10:00:00Z",
		"regions": ["EU"],
		"players": [{"player_id":"fresh-1","nickname":"fresher","country":"es","region":"EU","position":1,"faceit_elo":5000,"game_skill_level":10,"rank":1}]
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
		{name: "unknown schema falls back", payload: `{"schema_version":"v0","generated_at":"2026-09-04T10:00:00Z","players":[]}`, write: true, wantDefault: true},
		{name: "empty player list falls back", payload: `{"schema_version":"cliphub.faceit-top10/v1","generated_at":"2026-09-04T10:00:00Z","players":[]}`, write: true, wantDefault: true},
		{name: "invalid row falls back", payload: `{"schema_version":"cliphub.faceit-top10/v1","generated_at":"2026-09-04T10:00:00Z","players":[{"player_id":"bad id","nickname":"x"}]}`, write: true, wantDefault: true},
		{name: "oversized override falls back", payload: `{"schema_version":"cliphub.faceit-top10/v1","filler":"` + strings.Repeat("x", maxSeedFileBytes) + `"}`, write: true, wantDefault: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			path := filepath.Join(t.TempDir(), "top10.json")
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

func TestSeedStoreRefreshCommitsGlobalTop(t *testing.T) {
	t.Parallel()
	population := []rankedFixture{
		{id: "eu-0", region: "EU", elo: 4600},
		{id: "eu-1", region: "EU", elo: 4500},
		{id: "na-0", region: "NA", elo: 4000},
		{id: "sea-0", region: "SEA", elo: 3400},
	}
	client := leaderboardServer(t, population, nil)
	path := filepath.Join(t.TempDir(), "faceit", "top10.json")
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
	if got := len(doc.Players); got != 3 {
		t.Fatalf("players = %d, want the requested 3", got)
	}
	for i, player := range doc.Players {
		if player.Rank != i+1 {
			t.Fatalf("player %d rank = %d, want the merged global rank %d", i, player.Rank, i+1)
		}
	}
	if doc.Players[0].PlayerID != "eu-0" || doc.Players[2].PlayerID != "na-0" {
		t.Fatalf("players = %#v, want the global order", doc.Players)
	}
	// A region that answers with nothing still answered, so it stays in the
	// coverage record.
	if got := strings.Join(doc.Regions, ","); got != "EU,NA,SA,OCE,SEA" {
		t.Fatalf("regions = %q, want every region that answered", got)
	}

	reopened, err := NewSeedStore(path)
	if err != nil {
		t.Fatal(err)
	}
	persisted := reopened.Document()
	if len(persisted.Players) != 3 || persisted.Players[0].PlayerID != "eu-0" {
		t.Fatalf("persisted document = %#v", persisted.Players)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var shape struct {
		SchemaVersion string   `json:"schema_version"`
		GeneratedAt   string   `json:"generated_at"`
		Regions       []string `json:"regions"`
		Players       []struct {
			PlayerID string `json:"player_id"`
			Rank     int    `json:"rank"`
			Position int    `json:"position"`
			ELO      int    `json:"faceit_elo"`
		} `json:"players"`
	}
	if err := json.Unmarshal(raw, &shape); err != nil {
		t.Fatal(err)
	}
	if shape.SchemaVersion != SeedSchemaVersion || shape.GeneratedAt != "2026-09-04T00:00:00Z" || len(shape.Players) != 3 {
		t.Fatalf("committed file = %s", raw)
	}
	if shape.Players[2].PlayerID != "na-0" || shape.Players[2].Rank != 3 || shape.Players[2].Position != 1 {
		t.Fatalf("committed row = %#v, want global rank 3 and NA position 1", shape.Players[2])
	}
}

func TestSeedStoreRefreshRequiresClient(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "top10.json")
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
	path := filepath.Join(t.TempDir(), "top10.json")
	store, err := NewSeedStore(path)
	if err != nil {
		t.Fatal(err)
	}
	client := leaderboardServer(t, []rankedFixture{{id: "eu-0", region: "EU", elo: 4600}}, nil)
	if _, err := store.Refresh(context.Background(), client, 5); err != nil {
		t.Fatal(err)
	}
	broken := leaderboardServer(t, nil, map[string]bool{"EU": true, "NA": true, "SA": true, "OCE": true, "SEA": true})
	if _, err := store.Refresh(context.Background(), broken, 5); err == nil {
		t.Fatal("Refresh error = nil, want a total-outage failure")
	}
	doc := store.Document()
	if len(doc.Players) != 1 || doc.Players[0].PlayerID != "eu-0" {
		t.Fatalf("document = %#v, want the last good roster", doc.Players)
	}
}
