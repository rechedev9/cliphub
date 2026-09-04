package faceit

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"
)

func TestFollowStorePersistsAndUnfollows(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, time.August, 17, 12, 0, 0, 0, time.UTC)
	store, err := NewFollowStore(filepath.Join(t.TempDir(), "followed.json"), func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}

	first, err := store.Follow(Player{ID: "player-1", Nickname: "m0NESY", ELO: 4000, ProfileURL: "https://www.faceit.com/en/players/m0NESY"})
	if err != nil {
		t.Fatal(err)
	}
	if first.FollowedAt != now {
		t.Fatalf("followed_at = %v, want %v", first.FollowedAt, now)
	}
	second, err := store.Follow(Player{ID: "player-2", Nickname: "ZywOo", ELO: 3900, ProfileURL: "https://www.faceit.com/en/players/ZywOo"})
	if err != nil {
		t.Fatal(err)
	}
	if second.ID != "player-2" {
		t.Fatalf("second = %#v", second)
	}

	listed, err := store.List()
	if err != nil {
		t.Fatal(err)
	}
	if len(listed) != 2 || listed[0].ID != "player-2" || listed[1].ID != "player-1" {
		t.Fatalf("list = %#v, want newest first", listed)
	}

	updated, err := store.Follow(Player{ID: "player-1", Nickname: "m0NESY", ELO: 4010, ProfileURL: "https://www.faceit.com/en/players/m0NESY"})
	if err != nil {
		t.Fatal(err)
	}
	if updated.ELO != 4010 || updated.FollowedAt != now {
		t.Fatalf("update = %#v, want elo 4010 and original followed_at", updated)
	}
	listed, err = store.List()
	if err != nil {
		t.Fatal(err)
	}
	if len(listed) != 2 {
		t.Fatalf("list after update = %#v, want still 2", listed)
	}

	if err := store.Unfollow("player-2"); err != nil {
		t.Fatal(err)
	}
	if err := store.Unfollow("player-2"); err != nil {
		t.Fatal(err)
	}
	listed, err = store.List()
	if err != nil {
		t.Fatal(err)
	}
	if listed == nil || len(listed) != 1 || listed[0].ID != "player-1" || listed[0].ELO != 4010 {
		t.Fatalf("list after unfollow = %#v", listed)
	}
	if err := store.Unfollow("player-1"); err != nil {
		t.Fatal(err)
	}
	listed, err = store.List()
	if err != nil || listed == nil || len(listed) != 0 {
		t.Fatalf("list after last unfollow = %#v err=%v", listed, err)
	}
}

func TestFollowStoreTable(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, time.August, 17, 12, 0, 0, 0, time.UTC)

	t.Run("empty file lists nothing", func(t *testing.T) {
		t.Parallel()
		store, err := NewFollowStore(filepath.Join(t.TempDir(), "missing.json"), func() time.Time { return now })
		if err != nil {
			t.Fatal(err)
		}
		listed, err := store.List()
		if err != nil || len(listed) != 0 {
			t.Fatalf("list = %#v, %v", listed, err)
		}
	})

	t.Run("rejects invalid player", func(t *testing.T) {
		t.Parallel()
		store, err := NewFollowStore(filepath.Join(t.TempDir(), "followed.json"), func() time.Time { return now })
		if err != nil {
			t.Fatal(err)
		}
		tests := []Player{
			{ID: "", Nickname: "x"},
			{ID: "bad id", Nickname: "x"},
			{ID: "player-1", Nickname: ""},
		}
		for _, player := range tests {
			if _, err := store.Follow(player); err == nil {
				t.Fatalf("Follow(%#v) error = nil, want error", player)
			}
		}
	})

	t.Run("enforces follow cap", func(t *testing.T) {
		t.Parallel()
		store, err := NewFollowStore(filepath.Join(t.TempDir(), "followed.json"), func() time.Time { return now })
		if err != nil {
			t.Fatal(err)
		}
		for i := range MaxFollowedPlayers {
			if _, err := store.Follow(Player{ID: "player-" + strconv.Itoa(i), Nickname: "n"}); err != nil {
				t.Fatalf("seed follow %d: %v", i, err)
			}
		}
		_, err = store.Follow(Player{ID: "player-overflow", Nickname: "n"})
		if !errors.Is(err, ErrFollowLimit) {
			t.Fatalf("error = %v, want ErrFollowLimit", err)
		}
	})

	t.Run("rejects corrupt and unknown schema", func(t *testing.T) {
		t.Parallel()
		tests := []struct {
			name    string
			payload string
		}{
			{name: "not json", payload: "nope"},
			{name: "unknown schema", payload: `{"schema_version":"v0","players":[]}`},
		}
		for _, test := range tests {
			t.Run(test.name, func(t *testing.T) {
				t.Parallel()
				path := filepath.Join(t.TempDir(), "followed.json")
				if err := os.WriteFile(path, []byte(test.payload), 0o600); err != nil {
					t.Fatal(err)
				}
				store, err := NewFollowStore(path, func() time.Time { return now })
				if err != nil {
					t.Fatal(err)
				}
				if _, err := store.List(); err == nil {
					t.Fatal("List error = nil, want error")
				}
			})
		}
	})

	t.Run("requires path", func(t *testing.T) {
		t.Parallel()
		if _, err := NewFollowStore("", nil); err == nil {
			t.Fatal("NewFollowStore empty path error = nil")
		}
	})
}

func TestFollowStoreRoundTripJSON(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "followed.json")
	store, err := NewFollowStore(path, func() time.Time { return time.Date(2026, 8, 17, 0, 0, 0, 0, time.UTC) })
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.Follow(Player{ID: "player-1", Nickname: "m0NESY"}); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var file followFile
	if err := json.Unmarshal(raw, &file); err != nil {
		t.Fatal(err)
	}
	if file.SchemaVersion != FollowSchemaVersion || len(file.Players) != 1 {
		t.Fatalf("file = %#v", file)
	}
}

// seedFor builds a default-roster document for the projection tests.
func seedFor(generatedAt time.Time, players ...RankedPlayer) SeedDocument {
	return SeedDocument{
		SchemaVersion: SeedSchemaVersion,
		GeneratedAt:   generatedAt,
		Regions:       []string{"EU"},
		Players:       seedPlayers(players),
	}
}

func TestFollowStoreRosterPutsFollowsBeforeSeeds(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, time.September, 4, 12, 0, 0, 0, time.UTC)
	generatedAt := time.Date(2026, time.September, 1, 0, 0, 0, 0, time.UTC)
	dir := t.TempDir()
	store, err := NewFollowStore(filepath.Join(dir, "followed.json"), func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.Follow(Player{ID: "own-1", Nickname: "mine", ELO: 2000, Avatar: "https://assets.faceit-cdn.net/a.png", SteamID64: "76561198000000001"}); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Follow(Player{ID: "seeded-2", Nickname: "alsoSeeded", ELO: 4500}); err != nil {
		t.Fatal(err)
	}

	seed := seedFor(generatedAt,
		RankedPlayer{PlayerID: "seeded-1", Nickname: "donk666", Country: "kr", Region: "EU", Position: 1, ELO: 4636, SkillLevel: 10},
		RankedPlayer{PlayerID: "seeded-2", Nickname: "alsoSeeded", Country: "kz", Region: "EU", Position: 2, ELO: 4409, SkillLevel: 10},
		RankedPlayer{PlayerID: "seeded-3", Nickname: "nipl", Country: "ua", Region: "NA", Position: 4, ELO: 4213, SkillLevel: 10},
	)
	roster, err := store.Roster(seed)
	if err != nil {
		t.Fatal(err)
	}
	wantIDs := []string{"seeded-2", "own-1", "seeded-1", "seeded-3"}
	if len(roster) != len(wantIDs) {
		t.Fatalf("roster = %#v, want %v", roster, wantIDs)
	}
	for i, want := range wantIDs {
		if roster[i].ID != want {
			t.Fatalf("roster[%d].ID = %q, want %q (follows first, then seeds)", i, roster[i].ID, want)
		}
	}
	for _, row := range roster[:2] {
		if row.Seeded || row.Region != "" || row.Position != 0 {
			t.Fatalf("own follow surfaced as seeded: %#v", row)
		}
		if !row.FollowedAt.Equal(now) {
			t.Fatalf("own follow followed_at = %v, want %v", row.FollowedAt, now)
		}
	}

	seededRow := roster[2]
	if !seededRow.Seeded || seededRow.Region != "EU" || seededRow.Position != 1 {
		t.Fatalf("seeded row = %#v", seededRow)
	}
	if !seededRow.FollowedAt.Equal(generatedAt) {
		t.Fatalf("seeded followed_at = %v, want the seed generated_at %v so the list is stable across restarts", seededRow.FollowedAt, generatedAt)
	}
	if seededRow.Avatar != "" || seededRow.SteamID64 != "" {
		t.Fatalf("seeded row invented avatar/steamid: %#v", seededRow)
	}
	if seededRow.ELO != 4636 || seededRow.SkillLevel != 10 || seededRow.Country != "kr" {
		t.Fatalf("seeded row lost leaderboard data: %#v", seededRow)
	}
	if seededRow.ProfileURL != "https://www.faceit.com/en/players/donk666" {
		t.Fatalf("seeded profile_url = %q", seededRow.ProfileURL)
	}
	if roster[3].Region != "NA" || roster[3].Position != 4 {
		t.Fatalf("second seeded row = %#v, want its own region position", roster[3])
	}

	// Seeded rows are a projection: followed.json still holds only what the
	// user chose.
	raw, err := os.ReadFile(filepath.Join(dir, "followed.json"))
	if err != nil {
		t.Fatal(err)
	}
	var file followFile
	if err := json.Unmarshal(raw, &file); err != nil {
		t.Fatal(err)
	}
	if len(file.Players) != 2 {
		t.Fatalf("followed.json players = %#v, want only the user's follows", file.Players)
	}
	for _, player := range file.Players {
		if player.ID == "seeded-1" || player.ID == "seeded-3" {
			t.Fatalf("seeded player %q was persisted", player.ID)
		}
	}
}

func TestFollowStoreRosterDoesNotSpendTheFollowCap(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, time.September, 4, 12, 0, 0, 0, time.UTC)
	store, err := NewFollowStore(filepath.Join(t.TempDir(), "followed.json"), func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}
	seed := seedFor(now.Add(-time.Hour),
		RankedPlayer{PlayerID: "seeded-1", Nickname: "donk666", Region: "EU", Position: 1, ELO: 4636},
		RankedPlayer{PlayerID: "seeded-2", Nickname: "-SYPH0", Region: "EU", Position: 2, ELO: 4409},
	)
	for i := range MaxFollowedPlayers {
		if _, err := store.Follow(Player{ID: "own-" + strconv.Itoa(i), Nickname: "n"}); err != nil {
			t.Fatalf("seed follow %d: %v", i, err)
		}
	}
	if _, err := store.Follow(Player{ID: "own-overflow", Nickname: "n"}); !errors.Is(err, ErrFollowLimit) {
		t.Fatalf("Follow error = %v, want ErrFollowLimit", err)
	}
	roster, err := store.Roster(seed)
	if err != nil {
		t.Fatal(err)
	}
	if len(roster) != MaxFollowedPlayers+2 {
		t.Fatalf("roster = %d rows, want %d follows plus 2 seeds", len(roster), MaxFollowedPlayers)
	}
	seeded := 0
	for _, row := range roster {
		if row.Seeded {
			seeded++
		}
	}
	if seeded != 2 {
		t.Fatalf("seeded rows = %d, want 2 even with the follow list full", seeded)
	}
}

func TestFollowStoreDismissSeedSurvivesRestart(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, time.September, 4, 12, 0, 0, 0, time.UTC)
	path := filepath.Join(t.TempDir(), "followed.json")
	store, err := NewFollowStore(path, func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}
	seed := seedFor(now.Add(-time.Hour),
		RankedPlayer{PlayerID: "seeded-1", Nickname: "donk666", Region: "EU", Position: 1, ELO: 4636},
		RankedPlayer{PlayerID: "seeded-2", Nickname: "-SYPH0", Region: "EU", Position: 2, ELO: 4409},
	)

	// Unfollow cannot hide a seeded row: it was never in followed.json, so it
	// removes nothing and reports success. That is why dismissal exists.
	if err := store.Unfollow("seeded-1"); err != nil {
		t.Fatal(err)
	}
	roster, err := store.Roster(seed)
	if err != nil {
		t.Fatal(err)
	}
	if len(roster) != 2 {
		t.Fatalf("roster after Unfollow = %#v, want the seeded rows untouched", roster)
	}

	if err := store.DismissSeed("seeded-1"); err != nil {
		t.Fatal(err)
	}
	if err := store.DismissSeed("seeded-1"); err != nil {
		t.Fatalf("second DismissSeed = %v, want it to be idempotent", err)
	}
	ids, err := store.DismissedSeeds()
	if err != nil || len(ids) != 1 || ids[0] != "seeded-1" {
		t.Fatalf("DismissedSeeds = %v, %v", ids, err)
	}

	reopened, err := NewFollowStore(path, func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}
	roster, err = reopened.Roster(seed)
	if err != nil {
		t.Fatal(err)
	}
	if len(roster) != 1 || roster[0].ID != "seeded-2" {
		t.Fatalf("roster after restart = %#v, want the dismissal to persist", roster)
	}

	if err := reopened.RestoreSeeds(); err != nil {
		t.Fatal(err)
	}
	ids, err = reopened.DismissedSeeds()
	if err != nil || len(ids) != 0 {
		t.Fatalf("DismissedSeeds after restore = %v, %v", ids, err)
	}
	roster, err = store.Roster(seed)
	if err != nil {
		t.Fatal(err)
	}
	if len(roster) != 2 {
		t.Fatalf("roster after restore = %#v, want both seeded rows back", roster)
	}
}

func TestFollowStoreDismissSeedTable(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, time.September, 4, 12, 0, 0, 0, time.UTC)
	seed := seedFor(now, RankedPlayer{PlayerID: "seeded-1", Nickname: "donk666", Region: "EU", Position: 1, ELO: 4636})

	t.Run("rejects an invalid id", func(t *testing.T) {
		t.Parallel()
		store, err := NewFollowStore(filepath.Join(t.TempDir(), "followed.json"), func() time.Time { return now })
		if err != nil {
			t.Fatal(err)
		}
		for _, id := range []string{"", "not an id", "../escape"} {
			if err := store.DismissSeed(id); err == nil {
				t.Fatalf("DismissSeed(%q) error = nil, want a rejection", id)
			}
		}
	})

	t.Run("rejects a corrupt or unknown dismissal file", func(t *testing.T) {
		t.Parallel()
		tests := []struct {
			name    string
			payload string
		}{
			{name: "not json", payload: "nope"},
			{name: "unknown schema", payload: `{"schema_version":"v0","player_ids":[]}`},
		}
		for _, test := range tests {
			t.Run(test.name, func(t *testing.T) {
				t.Parallel()
				dir := t.TempDir()
				if err := os.WriteFile(filepath.Join(dir, dismissedSeedFileName), []byte(test.payload), 0o600); err != nil {
					t.Fatal(err)
				}
				store, err := NewFollowStore(filepath.Join(dir, "followed.json"), func() time.Time { return now })
				if err != nil {
					t.Fatal(err)
				}
				if _, err := store.Roster(seed); err == nil {
					t.Fatal("Roster error = nil, want the unreadable dismissal file surfaced")
				}
				if err := store.DismissSeed("seeded-1"); err == nil {
					t.Fatal("DismissSeed error = nil, want the unreadable dismissal file surfaced")
				}
				// RestoreSeeds replaces the file without reading it, so it is
				// the way out of a broken dismissal document.
				if err := store.RestoreSeeds(); err != nil {
					t.Fatal(err)
				}
				if _, err := store.Roster(seed); err != nil {
					t.Fatal(err)
				}
			})
		}
	})

	t.Run("dismissing a followed player leaves the follow", func(t *testing.T) {
		t.Parallel()
		store, err := NewFollowStore(filepath.Join(t.TempDir(), "followed.json"), func() time.Time { return now })
		if err != nil {
			t.Fatal(err)
		}
		if _, err := store.Follow(Player{ID: "seeded-1", Nickname: "donk666"}); err != nil {
			t.Fatal(err)
		}
		if err := store.DismissSeed("seeded-1"); err != nil {
			t.Fatal(err)
		}
		roster, err := store.Roster(seed)
		if err != nil {
			t.Fatal(err)
		}
		if len(roster) != 1 || roster[0].Seeded {
			t.Fatalf("roster = %#v, want the real follow kept", roster)
		}
	})

	t.Run("nil store", func(t *testing.T) {
		t.Parallel()
		var store *FollowStore
		if _, err := store.Roster(seed); err == nil {
			t.Fatal("Roster error = nil")
		}
		if err := store.DismissSeed("seeded-1"); err == nil {
			t.Fatal("DismissSeed error = nil")
		}
		if err := store.RestoreSeeds(); err == nil {
			t.Fatal("RestoreSeeds error = nil")
		}
		if _, err := store.DismissedSeeds(); err == nil {
			t.Fatal("DismissedSeeds error = nil")
		}
	})
}
