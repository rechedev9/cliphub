package faceit

import (
	"bytes"
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"sync"

	"github.com/rechedev9/cliphub/internal/filecommit"
)

// SeedSchemaVersion identifies the default-roster document.
const SeedSchemaVersion = "cliphub.faceit-top10/v1"

// maxSeedFileBytes bounds the on-disk override. The document holds a handful of
// leaderboard rows; anything larger is not one.
const maxSeedFileBytes = 256 * 1024

// defaultSeedJSON is the roster the Players section shows before anyone
// follows anybody: the FACEIT global top 10 by ELO, measured against the live
// Data API. It lives as data rather than as literals so refreshing it is a file
// swap and its provenance (generated_at, regions) travels with the numbers.
//
//go:embed top10_default.json
var defaultSeedJSON []byte

// loadDefaultSeed parses the embedded roster once. A malformed embedded file is
// a build-time mistake, not a runtime condition, so it panics rather than
// silently shipping an empty Players section.
var loadDefaultSeed = sync.OnceValue(func() SeedDocument {
	doc, err := DecodeSeed(bytes.NewReader(defaultSeedJSON))
	if err != nil {
		panic("faceit: embedded default top-10 roster is invalid: " + err.Error())
	}
	return doc
})

// DefaultSeed returns the roster ClipHub ships with. The returned document owns
// its slices, so a caller can adjust a copy without changing what the next
// caller sees.
func DefaultSeed() SeedDocument {
	return loadDefaultSeed().clone()
}

func (d SeedDocument) clone() SeedDocument {
	d.Regions = append([]string(nil), d.Regions...)
	d.Players = append([]SeedPlayer(nil), d.Players...)
	return d
}

// Validate rejects a document that cannot seed the Players section. A row with
// an unusable id or no nickname would render as a dead entry that can never be
// followed, so one bad row fails the document.
func (d SeedDocument) Validate() error {
	if d.SchemaVersion != SeedSchemaVersion {
		return fmt.Errorf("FACEIT seed roster schema %q is unsupported", d.SchemaVersion)
	}
	if d.GeneratedAt.IsZero() {
		return errors.New("FACEIT seed roster is missing generated_at")
	}
	if len(d.Players) == 0 {
		return errors.New("FACEIT seed roster has no players")
	}
	seen := make(map[string]bool, len(d.Players))
	for i, player := range d.Players {
		if !ValidPlayerID(player.PlayerID) {
			return fmt.Errorf("FACEIT seed roster player %d has an invalid id", i)
		}
		if player.Nickname == "" {
			return fmt.Errorf("FACEIT seed roster player %d has no nickname", i)
		}
		if seen[player.PlayerID] {
			return fmt.Errorf("FACEIT seed roster player %d is a duplicate", i)
		}
		seen[player.PlayerID] = true
	}
	return nil
}

// DecodeSeed reads a seed document and validates it, so a truncated file fails
// at load instead of half-populating the Players section.
func DecodeSeed(r io.Reader) (SeedDocument, error) {
	data, err := io.ReadAll(io.LimitReader(r, maxSeedFileBytes+1))
	if err != nil {
		return SeedDocument{}, fmt.Errorf("read FACEIT seed roster: %w", err)
	}
	if int64(len(data)) > maxSeedFileBytes {
		return SeedDocument{}, fmt.Errorf("FACEIT seed roster exceeds %d bytes", maxSeedFileBytes)
	}
	var doc SeedDocument
	if err := json.Unmarshal(data, &doc); err != nil {
		return SeedDocument{}, fmt.Errorf("decode FACEIT seed roster: %w", err)
	}
	if err := doc.Validate(); err != nil {
		return SeedDocument{}, err
	}
	return doc, nil
}

// SeedStore serves the default roster from a refreshable file, falling back to
// the embedded document. Refresh is the only thing that talks to FACEIT: a read
// never reaches the network, so opening the Players section costs nothing and
// cannot fail because the Data API is down.
type SeedStore struct {
	path string
	mu   sync.Mutex
}

func NewSeedStore(path string) (*SeedStore, error) {
	if path == "" {
		return nil, errors.New("FACEIT seed roster path is required")
	}
	return &SeedStore{path: path}, nil
}

// Document returns the on-disk roster when it is present and parseable, and the
// embedded default otherwise. A corrupt, truncated, or unreadable override
// degrades to the default instead of erroring, because a bad cache file must
// never leave the Players section empty.
func (s *SeedStore) Document() SeedDocument {
	if s == nil {
		return DefaultSeed()
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if doc, err := s.loadLocked(); err == nil {
		return doc
	}
	return DefaultSeed()
}

// Refresh replaces the on-disk roster with a live FACEIT global top `limit`.
// It is explicit only: nothing calls it on startup or on a read, so the seeded
// list is a deliberate refresh rather than a request that silently fans out to
// five leaderboards.
func (s *SeedStore) Refresh(ctx context.Context, client *Client, limit int) (SeedDocument, error) {
	if s == nil {
		return SeedDocument{}, errors.New("FACEIT seed roster store is not configured")
	}
	if client == nil {
		return SeedDocument{}, ErrNotConfigured
	}
	players, regions, err := client.globalTop(ctx, limit)
	if err != nil {
		return SeedDocument{}, fmt.Errorf("refresh FACEIT seed roster: %w", err)
	}
	doc := SeedDocument{
		SchemaVersion: SeedSchemaVersion,
		GeneratedAt:   client.now().UTC(),
		Regions:       regions,
		Players:       seedPlayers(players),
	}
	if err := doc.Validate(); err != nil {
		return SeedDocument{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.saveLocked(doc); err != nil {
		return SeedDocument{}, err
	}
	return doc, nil
}

// seedPlayers stamps the merged global order onto the rows. Position stays the
// player's place inside their own region, which is what FACEIT reported.
func seedPlayers(players []RankedPlayer) []SeedPlayer {
	out := make([]SeedPlayer, 0, len(players))
	for i, player := range players {
		out = append(out, SeedPlayer{RankedPlayer: player, Rank: i + 1})
	}
	return out
}

func (s *SeedStore) loadLocked() (SeedDocument, error) {
	file, err := os.Open(s.path)
	if err != nil {
		return SeedDocument{}, err
	}
	defer func() { _ = file.Close() }()
	return DecodeSeed(file)
}

func (s *SeedStore) saveLocked(doc SeedDocument) error {
	data, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return fmt.Errorf("encode FACEIT seed roster: %w", err)
	}
	attempt, cleanup, err := filecommit.Attempt(s.path)
	if err != nil {
		return fmt.Errorf("stage FACEIT seed roster: %w", err)
	}
	defer cleanup()
	if err := os.WriteFile(attempt, data, 0o600); err != nil {
		return fmt.Errorf("write FACEIT seed roster: %w", err)
	}
	if err := filecommit.Commit(attempt, s.path); err != nil {
		return fmt.Errorf("commit FACEIT seed roster: %w", err)
	}
	return nil
}
