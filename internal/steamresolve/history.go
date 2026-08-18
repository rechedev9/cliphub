package steamresolve

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/rechedev9/cliphub/internal/sharecode"
)

const (
	defaultHistoryURL = "https://api.steampowered.com/ICSGOPlayers_730/GetNextMatchSharingCode/v1/"
	maxHistoryWalk    = 20
	historyTimeout    = 15 * time.Second
	maxHistoryBody    = 16 << 10
)

// ErrNeedKnownCode means the account has no share code to start the chain.
var ErrNeedKnownCode = errors.New("a known share code is required to walk match history")

// HistoryClient calls Valve's GetNextMatchSharingCode Web API.
type HistoryClient struct {
	http    *http.Client
	baseURL string
}

// NewHistoryClient builds a client. httpClient may be nil.
func NewHistoryClient(httpClient *http.Client) *HistoryClient {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: historyTimeout}
	}
	return &HistoryClient{http: httpClient, baseURL: defaultHistoryURL}
}

type historyAPIResponse struct {
	Result struct {
		NextCode string `json:"nextcode"`
	} `json:"result"`
}

// NextShareCode returns the share code after known, or ("", nil) when Valve
// reports no newer match (nextcode is empty or "n/a").
func (c *HistoryClient) NextShareCode(ctx context.Context, acc Account, known string) (string, error) {
	if c == nil {
		return "", errors.New("steam history client is not configured")
	}
	if !acc.HistoryConfigured() {
		return "", ErrAccountNotConfigured
	}
	if known == "" {
		return "", ErrNeedKnownCode
	}
	if _, err := sharecode.Decode(known); err != nil {
		return "", fmt.Errorf("%w: %v", ErrInvalidCode, err)
	}
	endpoint, err := url.Parse(c.baseURL)
	if err != nil {
		return "", fmt.Errorf("steam history url: %w", err)
	}
	query := endpoint.Query()
	query.Set("key", acc.APIKey)
	query.Set("steamid", acc.SteamID)
	query.Set("steamidkey", acc.AuthCode)
	query.Set("knowncode", known)
	endpoint.RawQuery = query.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint.String(), nil)
	if err != nil {
		return "", fmt.Errorf("steam history request: %w", err)
	}
	res, err := c.http.Do(req)
	if err != nil {
		return "", fmt.Errorf("steam history request: %w", err)
	}
	defer res.Body.Close()
	body, err := io.ReadAll(io.LimitReader(res.Body, maxHistoryBody+1))
	if err != nil {
		return "", fmt.Errorf("read steam history: %w", err)
	}
	if len(body) > maxHistoryBody {
		return "", errors.New("steam history response is too large")
	}
	if res.StatusCode == http.StatusForbidden || res.StatusCode == http.StatusUnauthorized {
		return "", fmt.Errorf("steam history authorization failed")
	}
	if res.StatusCode != http.StatusOK {
		return "", fmt.Errorf("steam history status %d", res.StatusCode)
	}
	var parsed historyAPIResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		return "", fmt.Errorf("decode steam history: %w", err)
	}
	next := strings.TrimSpace(parsed.Result.NextCode)
	if next == "" || strings.EqualFold(next, "n/a") {
		return "", nil
	}
	if _, err := sharecode.Decode(next); err != nil {
		return "", fmt.Errorf("steam history next code: %w", err)
	}
	encoded, _, err := normalizeStoredCode(next)
	if err != nil {
		return "", err
	}
	return encoded, nil
}

// Walk collects newer share codes starting after acc.KnownCode, newest last.
func (c *HistoryClient) Walk(ctx context.Context, acc Account, limit int) (known string, matches []StoredMatch, err error) {
	if limit <= 0 || limit > maxHistoryWalk {
		limit = maxHistoryWalk
	}
	cursor := acc.KnownCode
	if cursor == "" {
		return "", nil, ErrNeedKnownCode
	}
	seen := map[string]struct{}{cursor: {}}
	now := time.Now().UTC()
	for i := 0; i < limit; i++ {
		next, err := c.NextShareCode(ctx, acc, cursor)
		if err != nil {
			return cursor, matches, err
		}
		if next == "" {
			return cursor, matches, nil
		}
		if _, dup := seen[next]; dup {
			return cursor, matches, nil
		}
		seen[next] = struct{}{}
		_, matchID, err := normalizeStoredCode(next)
		if err != nil {
			return cursor, matches, err
		}
		matches = append(matches, StoredMatch{ShareCode: next, MatchID: matchID, DiscoveredAt: now})
		cursor = next
	}
	return cursor, matches, nil
}
