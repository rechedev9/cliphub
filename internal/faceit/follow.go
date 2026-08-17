package faceit

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/rechedev9/cliphub/internal/filecommit"
)

const (
	FollowSchemaVersion = "cliphub.faceit-followed/v1"
	MaxFollowedPlayers  = 20
)

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

type FollowStore struct {
	path string
	now  func() time.Time
	mu   sync.Mutex
}

func NewFollowStore(path string, now func() time.Time) (*FollowStore, error) {
	if path == "" {
		return nil, errors.New("FACEIT follow store path is required")
	}
	if now == nil {
		now = time.Now
	}
	return &FollowStore{path: path, now: now}, nil
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
