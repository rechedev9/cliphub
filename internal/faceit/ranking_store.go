package faceit

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/rechedev9/cliphub/internal/filecommit"
)

// RankingMaxAge is how long a stored ranking answers without asking FACEIT:
// a country costs 5 requests and a zone up to 100, on a key every install shares.
const RankingMaxAge = 6 * time.Hour

const rankingSchemaVersion = "cliphub.faceit-ranking/v1"

// maxRankingFileBytes bounds a stored ranking: RankingSize rows of a few
// hundred bytes each.
const maxRankingFileBytes = 512 * 1024

// RankingList is a ranking as the store serves it. Stale marks the last stored
// copy served because FACEIT failed, and StaleErr says how.
type RankingList struct {
	Entry     RankingEntry
	FetchedAt time.Time
	Players   []RankingPlayer
	Stale     bool
	StaleErr  error
}

type rankingFile struct {
	SchemaVersion string          `json:"schema_version"`
	Kind          string          `json:"kind"`
	Code          string          `json:"code"`
	FetchedAt     time.Time       `json:"fetched_at"`
	Players       []RankingPlayer `json:"players"`
}

// RankingStore keeps one file per country, region or zone under dir. A read
// reaches FACEIT only when its copy is missing, damaged or RankingMaxAge old.
type RankingStore struct {
	dir   string
	now   func() time.Time
	mu    sync.Mutex
	gates map[RankingEntry]chan struct{}
}

func NewRankingStore(dir string, now func() time.Time) (*RankingStore, error) {
	if dir == "" {
		return nil, errors.New("FACEIT ranking store folder is required")
	}
	if now == nil {
		now = time.Now
	}
	return &RankingStore{dir: dir, now: now, gates: map[RankingEntry]chan struct{}{}}, nil
}

// Top returns entry's ranking: the stored copy while it is fresh, else a new
// FACEIT read, else, when that read fails, the stored copy marked stale.
func (s *RankingStore) Top(ctx context.Context, client *Client, entry RankingEntry) (RankingList, error) {
	if s == nil {
		return RankingList{}, errors.New("FACEIT ranking store is not configured")
	}
	// One read per entry at a time: the page asks for its next 20 while the
	// first read may still be running, and that one must not be paid twice.
	release, err := s.lock(ctx, entry)
	if err != nil {
		return RankingList{}, err
	}
	defer release()

	stored, storedErr := s.load(entry)
	// A negative age means the clock went back: read again rather than trust it.
	if age := s.now().Sub(stored.FetchedAt); storedErr == nil && age >= 0 && age < RankingMaxAge {
		return stored, nil
	}
	players, err := client.RankingTop(ctx, entry)
	if err != nil {
		if storedErr != nil {
			return RankingList{}, err
		}
		stored.Stale, stored.StaleErr = true, err
		return stored, nil
	}
	list := RankingList{Entry: entry, FetchedAt: s.now().UTC(), Players: players}
	if err := s.save(list); err != nil {
		// The read is still good; only the next one pays FACEIT again.
		log.Printf("faceit: store %s ranking: %v", entry, err)
	}
	return list, nil
}

func (s *RankingStore) lock(ctx context.Context, entry RankingEntry) (func(), error) {
	s.mu.Lock()
	gate, ok := s.gates[entry]
	if !ok {
		gate = make(chan struct{}, 1)
		s.gates[entry] = gate
	}
	s.mu.Unlock()
	select {
	case gate <- struct{}{}:
		return func() { <-gate }, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func (s *RankingStore) path(entry RankingEntry) string {
	return filepath.Join(s.dir, entry.kind+"-"+strings.ToLower(entry.code)+".json")
}

func (s *RankingStore) load(entry RankingEntry) (RankingList, error) {
	file, err := os.Open(s.path(entry))
	if err != nil {
		return RankingList{}, err
	}
	defer func() { _ = file.Close() }()
	data, err := io.ReadAll(io.LimitReader(file, maxRankingFileBytes+1))
	if err != nil {
		return RankingList{}, err
	}
	if len(data) > maxRankingFileBytes {
		return RankingList{}, errors.New("FACEIT ranking copy is too large")
	}
	var doc rankingFile
	if err := json.Unmarshal(data, &doc); err != nil {
		return RankingList{}, err
	}
	if doc.SchemaVersion != rankingSchemaVersion || doc.Kind != entry.kind || doc.Code != entry.code || doc.FetchedAt.IsZero() {
		return RankingList{}, errors.New("FACEIT ranking copy does not match its entry")
	}
	for _, player := range doc.Players {
		if !ValidPlayerID(player.PlayerID) || player.Nickname == "" {
			return RankingList{}, ErrInvalidResponse
		}
	}
	return RankingList{Entry: entry, FetchedAt: doc.FetchedAt, Players: doc.Players}, nil
}

func (s *RankingStore) save(list RankingList) error {
	data, err := json.Marshal(rankingFile{
		SchemaVersion: rankingSchemaVersion,
		Kind:          list.Entry.kind,
		Code:          list.Entry.code,
		FetchedAt:     list.FetchedAt,
		Players:       list.Players,
	})
	if err != nil {
		return fmt.Errorf("encode FACEIT ranking: %w", err)
	}
	if err := os.MkdirAll(s.dir, 0o700); err != nil {
		return fmt.Errorf("create FACEIT ranking folder: %w", err)
	}
	attempt, cleanup, err := filecommit.Attempt(s.path(list.Entry))
	if err != nil {
		return fmt.Errorf("stage FACEIT ranking: %w", err)
	}
	defer cleanup()
	if err := os.WriteFile(attempt, data, 0o600); err != nil {
		return fmt.Errorf("write FACEIT ranking: %w", err)
	}
	return filecommit.Commit(attempt, s.path(list.Entry))
}
