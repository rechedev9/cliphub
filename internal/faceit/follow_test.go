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
