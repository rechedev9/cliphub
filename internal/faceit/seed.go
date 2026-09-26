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

// SeedSchemaVersion identifies the default-roster document. It replaced
// cliphub.faceit-top10/v1, the single global top 10, when the roster split
// into zones; an old override file now fails validation and falls back to the
// embedded zones.
const SeedSchemaVersion = "cliphub.faceit-zones/v1"

// SeedZoneLimit is how many players each zone roster holds.
const SeedZoneLimit = 20

// maxSeedFileBytes bounds the on-disk override. The document holds a handful of
// leaderboard rows; anything larger is not one.
const maxSeedFileBytes = 256 * 1024

// defaultSeedJSON is the roster the Players section shows next to the user's
// own follows: the top SeedZoneLimit FACEIT players by ELO for each zone
// (CIS, LATAM), measured against the live Data API. It lives as data rather
// than as literals so refreshing it is a file swap
// (FACEIT_API_KEY=... go generate ./internal/faceit) and its provenance
// (generated_at, regions) travels with the numbers.
//
//go:generate go run gen_zones.go
//go:embed zones_default.json
var defaultSeedJSON []byte

// loadDefaultSeed parses the embedded roster once. A malformed embedded file is
// a build-time mistake, not a runtime condition, so it panics rather than
// silently shipping an empty Players section.
var loadDefaultSeed = sync.OnceValue(func() SeedDocument {
	doc, err := DecodeSeed(bytes.NewReader(defaultSeedJSON))
	if err != nil {
		panic("faceit: embedded default zone roster is invalid: " + err.Error())
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
		if _, ok := zoneCountries(player.Zone); !ok {
			return fmt.Errorf("FACEIT seed roster player %d has an unknown zone %q", i, player.Zone)
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
// never reaches the network, so a list read never waits on the Data API and
// cannot fail because it is down.
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

// Refresh replaces the on-disk roster with the live top `limit` of every zone.
// Document never calls it: it fans out to every zone ladder (about 160
// requests), so the orchestrator runs it in the background when a Players
// list read finds the roster stale (httpapi faceitRosterMaxAge).
func (s *SeedStore) Refresh(ctx context.Context, client *Client, limit int) (SeedDocument, error) {
	if s == nil {
		return SeedDocument{}, errors.New("FACEIT seed roster store is not configured")
	}
	if client == nil {
		return SeedDocument{}, ErrNotConfigured
	}
	doc := SeedDocument{
		SchemaVersion: SeedSchemaVersion,
		GeneratedAt:   client.now().UTC(),
		// ZoneTop fails unless every region answered for every country.
		Regions: append([]string(nil), rankingRegions...),
	}
	for _, zone := range Zones() {
		players, err := client.ZoneTop(ctx, zone, limit)
		if err != nil {
			return SeedDocument{}, fmt.Errorf("refresh FACEIT seed roster: %w", err)
		}
		doc.Players = append(doc.Players, seedPlayers(zone, players)...)
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

// seedPlayers stamps the zone and the merged zone order onto the rows.
// Position stays the player's place inside their own region ladder, which is
// what FACEIT reported.
func seedPlayers(zone string, players []RankedPlayer) []SeedPlayer {
	out := make([]SeedPlayer, 0, len(players))
	for i, player := range players {
		out = append(out, SeedPlayer{RankedPlayer: player, Zone: zone, Rank: i + 1})
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
