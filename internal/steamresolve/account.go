package steamresolve

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/rechedev9/cliphub/internal/filecommit"
	"github.com/rechedev9/cliphub/internal/sharecode"
)

const (
	AccountSchemaVersion = "cliphub.steam-account/v1"
	MaxStoredMatches     = 50
)

// ErrAccountNotConfigured means no history account has been saved.
var ErrAccountNotConfigured = errors.New("steam history account is not configured")

// StoredMatch is one share code discovered from the Web API or pasted by the user.
type StoredMatch struct {
	ShareCode    string    `json:"share_code"`
	MatchID      string    `json:"match_id"`
	DiscoveredAt time.Time `json:"discovered_at"`
}

// Account is the revocable match-history credential set: SteamID, authentication
// code, Web API key, and the last known share code used to walk the chain.
// It never holds a Steam password.
type Account struct {
	SteamID   string
	AuthCode  string
	APIKey    string
	KnownCode string
	Matches   []StoredMatch
}

// HistoryConfigured reports whether the account can call GetNextMatchSharingCode.
func (a Account) HistoryConfigured() bool {
	return a.SteamID != "" && a.AuthCode != "" && a.APIKey != ""
}

type accountFile struct {
	SchemaVersion string        `json:"schema_version"`
	SteamID       string        `json:"steam_id"`
	AuthCode      string        `json:"auth_code"`
	APIKey        string        `json:"api_key"`
	KnownCode     string        `json:"known_code"`
	Matches       []StoredMatch `json:"matches"`
}

// AccountStore persists the history account under the orchestrator data dir.
type AccountStore struct {
	path string
	now  func() time.Time
	mu   sync.Mutex
}

// NewAccountStore prepares a store at path. The file is created on the first Save.
func NewAccountStore(path string, now func() time.Time) (*AccountStore, error) {
	if path == "" {
		return nil, errors.New("steam account store path is required")
	}
	if now == nil {
		now = time.Now
	}
	return &AccountStore{path: path, now: now}, nil
}

// Load returns the saved account, or a zero Account when the file is missing.
func (s *AccountStore) Load() (Account, error) {
	if s == nil {
		return Account{}, ErrAccountNotConfigured
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.loadLocked()
}

// Save writes acc, keeping existing secrets when the incoming field is empty
// so a form can update SteamID without re-sending the API key.
func (s *AccountStore) Save(acc Account) (Account, error) {
	if s == nil {
		return Account{}, ErrAccountNotConfigured
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	current, err := s.loadLocked()
	if err != nil {
		return Account{}, err
	}
	merged := mergeAccount(current, acc)
	if err := validateAccount(merged); err != nil {
		return Account{}, err
	}
	if err := s.writeLocked(merged); err != nil {
		return Account{}, err
	}
	return merged, nil
}

// RememberCode records a well-formed share code as known and prepends it to
// the stored match list. Invalid codes are ignored so a decode-only check
// cannot poison the chain.
func (s *AccountStore) RememberCode(code string) error {
	if s == nil {
		return ErrAccountNotConfigured
	}
	normalized, matchID, err := normalizeStoredCode(code)
	if err != nil {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	current, err := s.loadLocked()
	if err != nil {
		return err
	}
	if !current.HistoryConfigured() {
		return nil
	}
	current.KnownCode = normalized
	current.Matches = prependMatch(current.Matches, StoredMatch{
		ShareCode:    normalized,
		MatchID:      matchID,
		DiscoveredAt: s.now().UTC(),
	})
	return s.writeLocked(current)
}

// ReplaceMatches writes the walked match list and the newest known code.
func (s *AccountStore) ReplaceMatches(known string, matches []StoredMatch) error {
	if s == nil {
		return ErrAccountNotConfigured
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	current, err := s.loadLocked()
	if err != nil {
		return err
	}
	if known != "" {
		current.KnownCode = known
	}
	if matches != nil {
		if len(matches) > MaxStoredMatches {
			matches = matches[:MaxStoredMatches]
		}
		current.Matches = matches
	}
	return s.writeLocked(current)
}

// Clear removes the saved account.
func (s *AccountStore) Clear() error {
	if s == nil {
		return ErrAccountNotConfigured
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := os.Remove(s.path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("remove steam account: %w", err)
	}
	return nil
}

func (s *AccountStore) loadLocked() (Account, error) {
	data, err := os.ReadFile(s.path)
	if errors.Is(err, os.ErrNotExist) {
		return Account{}, nil
	}
	if err != nil {
		return Account{}, fmt.Errorf("read steam account: %w", err)
	}
	var file accountFile
	if err := json.Unmarshal(data, &file); err != nil {
		return Account{}, fmt.Errorf("decode steam account: %w", err)
	}
	if file.SchemaVersion != "" && file.SchemaVersion != AccountSchemaVersion {
		return Account{}, fmt.Errorf("steam account schema %q is not supported", file.SchemaVersion)
	}
	matches := file.Matches
	if matches == nil {
		matches = []StoredMatch{}
	}
	return Account{
		SteamID:   file.SteamID,
		AuthCode:  file.AuthCode,
		APIKey:    file.APIKey,
		KnownCode: file.KnownCode,
		Matches:   matches,
	}, nil
}

func (s *AccountStore) writeLocked(acc Account) error {
	// #nosec G117 -- account.json intentionally persists the revocable auth code
	// and Web API key (documented design); writeLocked stores it via filecommit
	// with 0600 permissions.
	data, err := json.MarshalIndent(accountFile{
		SchemaVersion: AccountSchemaVersion,
		SteamID:       acc.SteamID,
		AuthCode:      acc.AuthCode,
		APIKey:        acc.APIKey,
		KnownCode:     acc.KnownCode,
		Matches:       acc.Matches,
	}, "", "  ")
	if err != nil {
		return fmt.Errorf("encode steam account: %w", err)
	}
	attempt, cleanup, err := filecommit.Attempt(s.path)
	if err != nil {
		return fmt.Errorf("stage steam account: %w", err)
	}
	defer cleanup()
	if err := os.WriteFile(attempt, data, 0o600); err != nil {
		return fmt.Errorf("write steam account: %w", err)
	}
	if err := filecommit.Commit(attempt, s.path); err != nil {
		return fmt.Errorf("commit steam account: %w", err)
	}
	return nil
}

func mergeAccount(current, incoming Account) Account {
	out := current
	if incoming.SteamID != "" {
		out.SteamID = incoming.SteamID
	}
	if incoming.AuthCode != "" {
		out.AuthCode = incoming.AuthCode
	}
	if incoming.APIKey != "" {
		out.APIKey = incoming.APIKey
	}
	if incoming.KnownCode != "" {
		out.KnownCode = incoming.KnownCode
	}
	if incoming.Matches != nil {
		out.Matches = incoming.Matches
	}
	if out.Matches == nil {
		out.Matches = []StoredMatch{}
	}
	return out
}

func validateAccount(acc Account) error {
	if acc.SteamID != "" {
		if _, err := ParseSteamID(acc.SteamID); err != nil {
			return err
		}
	}
	if acc.AuthCode != "" {
		if err := validateAuthCode(acc.AuthCode); err != nil {
			return err
		}
	}
	if acc.APIKey != "" {
		if err := validateAPIKey(acc.APIKey); err != nil {
			return err
		}
	}
	if acc.KnownCode != "" {
		if _, err := sharecode.Decode(acc.KnownCode); err != nil {
			return fmt.Errorf("known share code: %w", err)
		}
	}
	return nil
}

// ParseSteamID accepts a 17-digit SteamID64 or a profiles/ URL that contains one.
func ParseSteamID(raw string) (string, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return "", errors.New("steam id is required")
	}
	if id, ok := steamIDFromText(trimmed); ok {
		return id, nil
	}
	return "", fmt.Errorf("steam id %q is not a 64-bit SteamID or profiles URL", trimmed)
}

func steamIDFromText(raw string) (string, bool) {
	if i := strings.Index(raw, "/profiles/"); i >= 0 {
		rest := raw[i+len("/profiles/"):]
		if slash := strings.IndexAny(rest, "/?#"); slash >= 0 {
			rest = rest[:slash]
		}
		raw = rest
	}
	if len(raw) != 17 {
		return "", false
	}
	if _, err := strconv.ParseUint(raw, 10, 64); err != nil {
		return "", false
	}
	if !strings.HasPrefix(raw, "7656") {
		return "", false
	}
	return raw, true
}

func validateAuthCode(raw string) error {
	trimmed := strings.TrimSpace(raw)
	if len(trimmed) < 5 || len(trimmed) > 64 {
		return errors.New("authentication code must be between 5 and 64 characters")
	}
	return nil
}

func validateAPIKey(raw string) error {
	trimmed := strings.TrimSpace(raw)
	if len(trimmed) < 8 || len(trimmed) > 64 {
		return errors.New("steam web api key must be between 8 and 64 characters")
	}
	return nil
}

func normalizeStoredCode(code string) (string, string, error) {
	m, err := sharecode.Decode(code)
	if err != nil {
		return "", "", err
	}
	return sharecode.Encode(m), strconv.FormatUint(m.MatchID, 10), nil
}

func prependMatch(existing []StoredMatch, next StoredMatch) []StoredMatch {
	out := make([]StoredMatch, 0, 1+len(existing))
	out = append(out, next)
	for _, item := range existing {
		if item.ShareCode == next.ShareCode {
			continue
		}
		out = append(out, item)
		if len(out) == MaxStoredMatches {
			break
		}
	}
	return out
}
