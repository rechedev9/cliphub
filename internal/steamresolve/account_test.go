package steamresolve

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestParseSteamID(t *testing.T) {
	tests := []struct {
		name    string
		raw     string
		want    string
		wantErr bool
	}{
		{name: "steam id64", raw: "76561198000000001", want: "76561198000000001"},
		{name: "profiles url", raw: "https://steamcommunity.com/profiles/76561198000000001", want: "76561198000000001"},
		{name: "profiles url with slash", raw: "https://steamcommunity.com/profiles/76561198000000001/", want: "76561198000000001"},
		{name: "empty", raw: "  ", wantErr: true},
		{name: "vanity url", raw: "https://steamcommunity.com/id/someone", wantErr: true},
		{name: "too short", raw: "7656119", wantErr: true},
		{name: "not a steam id64 prefix", raw: "12345678901234567", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseSteamID(tt.raw)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("ParseSteamID(%q) error = nil", tt.raw)
				}
				return
			}
			if err != nil {
				t.Fatalf("ParseSteamID(%q) error = %v", tt.raw, err)
			}
			if got != tt.want {
				t.Errorf("ParseSteamID(%q) = %q, want %q", tt.raw, got, tt.want)
			}
		})
	}
}

func TestAccountStoreSaveMergesSecrets(t *testing.T) {
	store, err := NewAccountStore(filepath.Join(t.TempDir(), "account.json"), func() time.Time {
		return time.Date(2026, 8, 18, 12, 0, 0, 0, time.UTC)
	})
	if err != nil {
		t.Fatal(err)
	}
	first, err := store.Save(Account{
		SteamID:  "76561198000000001",
		AuthCode: "AAAAA-BBBBB-CCCCC",
		APIKey:   "0123456789ABCDEF",
	})
	if err != nil {
		t.Fatalf("first Save: %v", err)
	}
	if !first.HistoryConfigured() {
		t.Fatal("first save should be history-configured")
	}
	second, err := store.Save(Account{SteamID: "76561198000000002", KnownCode: goodCode})
	if err != nil {
		t.Fatalf("second Save: %v", err)
	}
	if second.SteamID != "76561198000000002" {
		t.Errorf("steam id = %q", second.SteamID)
	}
	if second.AuthCode != "AAAAA-BBBBB-CCCCC" {
		t.Errorf("auth code was overwritten")
	}
	if second.APIKey != "0123456789ABCDEF" {
		t.Errorf("api key was overwritten")
	}
	if second.KnownCode != goodCode {
		t.Errorf("known code = %q", second.KnownCode)
	}
	loaded, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if loaded.SteamID != second.SteamID || loaded.AuthCode != second.AuthCode || loaded.APIKey != second.APIKey || loaded.KnownCode != second.KnownCode {
		t.Errorf("Load() = %+v, want %+v", loaded, second)
	}
}

func TestAccountStoreRememberAndClear(t *testing.T) {
	path := filepath.Join(t.TempDir(), "account.json")
	store, err := NewAccountStore(path, func() time.Time {
		return time.Date(2026, 8, 18, 15, 0, 0, 0, time.UTC)
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.Save(Account{
		SteamID:  "76561198000000001",
		AuthCode: "AAAAA-BBBBB-CCCCC",
		APIKey:   "0123456789ABCDEF",
	}); err != nil {
		t.Fatal(err)
	}
	if err := store.RememberCode(goodCode); err != nil {
		t.Fatalf("RememberCode: %v", err)
	}
	loaded, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if loaded.KnownCode != goodCode {
		t.Errorf("known = %q", loaded.KnownCode)
	}
	if len(loaded.Matches) != 1 || loaded.Matches[0].ShareCode != goodCode {
		t.Errorf("matches = %+v", loaded.Matches)
	}
	if loaded.Matches[0].MatchID != "3230642215713767580" {
		t.Errorf("match id = %q", loaded.Matches[0].MatchID)
	}
	if err := store.Clear(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("cleared file still exists: %v", err)
	}
	empty, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if empty.HistoryConfigured() {
		t.Errorf("cleared account still configured: %+v", empty)
	}
}

func TestAccountStoreRejectsBadFields(t *testing.T) {
	store, err := NewAccountStore(filepath.Join(t.TempDir(), "account.json"), time.Now)
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name string
		acc  Account
	}{
		{name: "bad steam id", acc: Account{SteamID: "nope", AuthCode: "AAAAA-BBBBB-CCCCC", APIKey: "0123456789ABCDEF"}},
		{name: "short auth", acc: Account{SteamID: "76561198000000001", AuthCode: "ab", APIKey: "0123456789ABCDEF"}},
		{name: "short key", acc: Account{SteamID: "76561198000000001", AuthCode: "AAAAA-BBBBB-CCCCC", APIKey: "short"}},
		{name: "bad known code", acc: Account{SteamID: "76561198000000001", AuthCode: "AAAAA-BBBBB-CCCCC", APIKey: "0123456789ABCDEF", KnownCode: "CSGO-nope"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := store.Save(tt.acc); err == nil {
				t.Fatal("Save error = nil")
			}
		})
	}
}

func TestSessionFromEnv(t *testing.T) {
	tests := []struct {
		name                      string
		username, password, guard string
		want                      bool
	}{
		{name: "complete", username: "u", password: "p", guard: "g", want: true},
		{name: "blank password", username: "u", password: "   ", guard: "g", want: false},
		{name: "missing guard", username: "u", password: "p", want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv(EnvUsername, tt.username)
			t.Setenv(EnvPassword, tt.password)
			t.Setenv(EnvGuard, tt.guard)
			got := SessionFromEnv()
			if got.Complete() != tt.want {
				t.Errorf("Complete() = %v, want %v (session=%+v)", got.Complete(), tt.want, Session{Username: got.Username, Guard: got.Guard})
			}
		})
	}
}
