package faceit

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"github.com/rechedev9/cliphub/internal/filecommit"
)

const (
	FollowSchemaVersion = "cliphub.faceit-followed/v1"
	MaxFollowedPlayers  = 20
)

// DismissedSeedSchemaVersion identifies the hidden-seed document. Dismissals
// live beside followed.json rather than inside it: a seeded player is not a
// follow, and FollowSchemaVersion describes what the user actually chose.
const DismissedSeedSchemaVersion = "cliphub.faceit-dismissed-seeds/v1"

const dismissedSeedFileName = "dismissed-seeds.json"

type FollowedPlayer struct {
	ID         string    `json:"id"`
	Nickname   string    `json:"nickname"`
	Avatar     string    `json:"avatar,omitempty"`
	ProfileURL string    `json:"profile_url"`
	SteamID64  string    `json:"steam_id64,omitempty"`
	Country    string    `json:"country,omitempty"`
	SkillLevel int       `json:"skill_level,omitempty"`
	ELO        int       `json:"elo,omitempty"`
	FollowedAt time.Time `json:"followed_at"`
}

type followFile struct {
	SchemaVersion string           `json:"schema_version"`
	Players       []FollowedPlayer `json:"players"`
}

type dismissedFile struct {
	SchemaVersion string   `json:"schema_version"`
	PlayerIDs     []string `json:"player_ids"`
}

// RosterPlayer is one row of the Players section. Seeded marks a row that came
// from the default FACEIT top-N roster instead of from the user's own follows:
// it is never written to followed.json and never consumes one of the
// MaxFollowedPlayers slots, so the default list cannot quietly spend the
// budget the user needs for their own players.
type RosterPlayer struct {
	FollowedPlayer
	Seeded   bool   `json:"seeded,omitempty"`
	Region   string `json:"region,omitempty"`
	Position int    `json:"position,omitempty"`
}

type FollowStore struct {
	path          string
	dismissedPath string
	now           func() time.Time
	mu            sync.Mutex
}

func NewFollowStore(path string, now func() time.Time) (*FollowStore, error) {
	if path == "" {
		return nil, errors.New("FACEIT follow store path is required")
	}
	if now == nil {
		now = time.Now
	}
	return &FollowStore{
		path:          path,
		dismissedPath: filepath.Join(filepath.Dir(path), dismissedSeedFileName),
		now:           now,
	}, nil
}

func (s *FollowStore) List() ([]FollowedPlayer, error) {
	if s == nil {
		return nil, errors.New("FACEIT follow store is not configured")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.listLocked()
}

func (s *FollowStore) Follow(player Player) (FollowedPlayer, error) {
	if s == nil {
		return FollowedPlayer{}, errors.New("FACEIT follow store is not configured")
	}
	if !ValidPlayerID(player.ID) || player.Nickname == "" {
		return FollowedPlayer{}, fmt.Errorf("FACEIT followed player is invalid")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	players, err := s.listLocked()
	if err != nil {
		return FollowedPlayer{}, err
	}
	entry := FollowedPlayer{
		ID:         player.ID,
		Nickname:   player.Nickname,
		Avatar:     player.Avatar,
		ProfileURL: player.ProfileURL,
		SteamID64:  player.SteamID64,
		Country:    player.Country,
		SkillLevel: player.SkillLevel,
		ELO:        player.ELO,
		FollowedAt: s.now().UTC(),
	}
	for i, existing := range players {
		if existing.ID != player.ID {
			continue
		}
		entry.FollowedAt = existing.FollowedAt
		players[i] = entry
		if err := s.saveLocked(players); err != nil {
			return FollowedPlayer{}, err
		}
		return entry, nil
	}
	if len(players) >= MaxFollowedPlayers {
		return FollowedPlayer{}, ErrFollowLimit
	}
	players = append([]FollowedPlayer{entry}, players...)
	if err := s.saveLocked(players); err != nil {
		return FollowedPlayer{}, err
	}
	return entry, nil
}

func (s *FollowStore) Unfollow(playerID string) error {
	if s == nil {
		return errors.New("FACEIT follow store is not configured")
	}
	if !ValidPlayerID(playerID) {
		return fmt.Errorf("FACEIT player id is invalid")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	players, err := s.listLocked()
	if err != nil {
		return err
	}
	kept := players[:0]
	for _, player := range players {
		if player.ID != playerID {
			kept = append(kept, player)
		}
	}
	if len(kept) == len(players) {
		return nil
	}
	return s.saveLocked(kept)
}

// Roster projects the Players section: the user's own follows first, newest
// first as List returns them, then the seeded default roster minus anyone the
// user already follows or has dismissed.
//
// Nothing here is persisted. A seeded row exists only for as long as the caller
// holds it, which is why a dismissal has to be durable on its own (see
// DismissSeed) and why the projection is the only place Seeded, Region, and
// Position exist.
func (s *FollowStore) Roster(seed SeedDocument) ([]RosterPlayer, error) {
	if s == nil {
		return nil, errors.New("FACEIT follow store is not configured")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	followed, err := s.listLocked()
	if err != nil {
		return nil, err
	}
	dismissed, err := s.dismissedLocked()
	if err != nil {
		return nil, err
	}
	out := make([]RosterPlayer, 0, len(followed)+len(seed.Players))
	shown := make(map[string]bool, len(followed)+len(seed.Players))
	for _, player := range followed {
		shown[player.ID] = true
		out = append(out, RosterPlayer{FollowedPlayer: player})
	}
	for _, player := range seed.Players {
		if shown[player.PlayerID] || dismissed[player.PlayerID] {
			continue
		}
		shown[player.PlayerID] = true
		out = append(out, RosterPlayer{
			FollowedPlayer: FollowedPlayer{
				ID:         player.PlayerID,
				Nickname:   player.Nickname,
				ProfileURL: canonicalProfileURL(player.Nickname),
				Country:    player.Country,
				SkillLevel: player.SkillLevel,
				ELO:        player.ELO,
				// The leaderboard response carries no avatar and no
				// SteamID64, and filling them would mean one profile lookup
				// per seeded row on every list. That belongs in a lazy
				// per-player enrichment, not here.
				FollowedAt: seed.GeneratedAt,
			},
			Seeded:   true,
			Region:   player.Region,
			Position: player.Position,
		})
	}
	return out, nil
}

// DismissSeed hides a seeded player for good. It has to be persisted
// separately because a seeded row was never in followed.json: Unfollow would
// find nothing to delete, return nil, and the row would be back on the next
// Roster call.
//
// Dismissing a player the user actually follows is a no-op on their follow
// row; only seeded rows are filtered.
func (s *FollowStore) DismissSeed(playerID string) error {
	if s == nil {
		return errors.New("FACEIT follow store is not configured")
	}
	if !ValidPlayerID(playerID) {
		return fmt.Errorf("FACEIT player id is invalid")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	dismissed, err := s.dismissedLocked()
	if err != nil {
		return err
	}
	if dismissed[playerID] {
		return nil
	}
	ids := make([]string, 0, len(dismissed)+1)
	for id := range dismissed {
		ids = append(ids, id)
	}
	ids = append(ids, playerID)
	sort.Strings(ids)
	return s.saveDismissedLocked(ids)
}

// RestoreSeeds clears every dismissal, bringing the default roster back.
func (s *FollowStore) RestoreSeeds() error {
	if s == nil {
		return errors.New("FACEIT follow store is not configured")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.saveDismissedLocked(nil)
}

// DismissedSeeds lists the hidden seeded player ids, sorted, so the state is
// inspectable rather than implied by what the roster stopped showing.
func (s *FollowStore) DismissedSeeds() ([]string, error) {
	if s == nil {
		return nil, errors.New("FACEIT follow store is not configured")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	dismissed, err := s.dismissedLocked()
	if err != nil {
		return nil, err
	}
	ids := make([]string, 0, len(dismissed))
	for id := range dismissed {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids, nil
}

func (s *FollowStore) listLocked() ([]FollowedPlayer, error) {
	data, err := os.ReadFile(s.path)
	if errors.Is(err, os.ErrNotExist) {
		return []FollowedPlayer{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read FACEIT follow list: %w", err)
	}
	var file followFile
	if err := json.Unmarshal(data, &file); err != nil {
		return nil, fmt.Errorf("decode FACEIT follow list: %w", err)
	}
	if file.SchemaVersion != FollowSchemaVersion {
		return nil, fmt.Errorf("FACEIT follow list schema %q is unsupported", file.SchemaVersion)
	}
	if file.Players == nil {
		return []FollowedPlayer{}, nil
	}
	out := append([]FollowedPlayer(nil), file.Players...)
	if out == nil {
		return []FollowedPlayer{}, nil
	}
	return out, nil
}

func (s *FollowStore) saveLocked(players []FollowedPlayer) error {
	if players == nil {
		players = []FollowedPlayer{}
	}
	data, err := json.MarshalIndent(followFile{SchemaVersion: FollowSchemaVersion, Players: players}, "", "  ")
	if err != nil {
		return fmt.Errorf("encode FACEIT follow list: %w", err)
	}
	attempt, cleanup, err := filecommit.Attempt(s.path)
	if err != nil {
		return fmt.Errorf("stage FACEIT follow list: %w", err)
	}
	defer cleanup()
	if err := os.WriteFile(attempt, data, 0o600); err != nil {
		return fmt.Errorf("write FACEIT follow list: %w", err)
	}
	if err := filecommit.Commit(attempt, s.path); err != nil {
		return fmt.Errorf("commit FACEIT follow list: %w", err)
	}
	return nil
}

// dismissedLocked reads the hidden-seed set. A missing file means nothing is
// hidden; an unreadable one is an error, exactly as it is for the follow list,
// because filecommit rules out a half-written document.
func (s *FollowStore) dismissedLocked() (map[string]bool, error) {
	data, err := os.ReadFile(s.dismissedPath)
	if errors.Is(err, os.ErrNotExist) {
		return map[string]bool{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read FACEIT dismissed seed list: %w", err)
	}
	var file dismissedFile
	if err := json.Unmarshal(data, &file); err != nil {
		return nil, fmt.Errorf("decode FACEIT dismissed seed list: %w", err)
	}
	if file.SchemaVersion != DismissedSeedSchemaVersion {
		return nil, fmt.Errorf("FACEIT dismissed seed list schema %q is unsupported", file.SchemaVersion)
	}
	out := make(map[string]bool, len(file.PlayerIDs))
	for _, id := range file.PlayerIDs {
		if ValidPlayerID(id) {
			out[id] = true
		}
	}
	return out, nil
}

func (s *FollowStore) saveDismissedLocked(ids []string) error {
	if ids == nil {
		ids = []string{}
	}
	data, err := json.MarshalIndent(dismissedFile{SchemaVersion: DismissedSeedSchemaVersion, PlayerIDs: ids}, "", "  ")
	if err != nil {
		return fmt.Errorf("encode FACEIT dismissed seed list: %w", err)
	}
	attempt, cleanup, err := filecommit.Attempt(s.dismissedPath)
	if err != nil {
		return fmt.Errorf("stage FACEIT dismissed seed list: %w", err)
	}
	defer cleanup()
	if err := os.WriteFile(attempt, data, 0o600); err != nil {
		return fmt.Errorf("write FACEIT dismissed seed list: %w", err)
	}
	if err := filecommit.Commit(attempt, s.dismissedPath); err != nil {
		return fmt.Errorf("commit FACEIT dismissed seed list: %w", err)
	}
	return nil
}
