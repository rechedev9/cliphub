package tuiclient

import (
	"context"
	"net/http"
	"time"
)

// Orchestrator error codes of the Steam routes (internal/httpapi/steam.go).
const (
	SteamCredentialsRequired  = "steam_credentials_required"
	SteamHistoryNotConfigured = "history_not_configured"
	SteamNeedKnownCode        = "need_known_code"
	SteamDemoUnavailable      = "demo_unavailable"
)

// SteamAccount is GET /api/steam/account. Secrets never leave the orchestrator: only whether they are set.
type SteamAccount struct {
	SteamID           string             `json:"steamId"`
	AuthCodeSet       bool               `json:"authCodeSet"`
	HistoryConfigured bool               `json:"historyConfigured"`
	GCConfigured      bool               `json:"gcConfigured"`
	Matches           []SteamStoredMatch `json:"matches"`
}

// SteamStoredMatch is one recent match found in the Steam match history.
type SteamStoredMatch struct {
	ShareCode    string    `json:"shareCode"`
	MatchID      string    `json:"matchId"`
	DiscoveredAt time.Time `json:"discoveredAt"`
}

// SteamImport is the body of a 201 from POST /api/steam/import: a Crear job.
type SteamImport struct {
	JobID   string `json:"id"`
	Status  string `json:"status"`
	MatchID string `json:"matchId"`
}

// SteamAccount reads the linked Steam account and its recent matches.
func (c *Client) SteamAccount(ctx context.Context) (SteamAccount, error) {
	var out SteamAccount
	err := c.getJSON(ctx, "/api/steam/account", &out)
	return out, err
}

// SyncSteamMatches walks the Steam match history for new matches.
func (c *Client) SyncSteamMatches(ctx context.Context) (SteamAccount, error) {
	var out SteamAccount
	err := c.doJSON(ctx, http.MethodPost, "/api/steam/matches/sync", nil, &out)
	return out, err
}

// ImportShareCode downloads a match's demo from Valve with the Steam session Studio
// already holds; without one it fails with SteamCredentialsRequired.
func (c *Client) ImportShareCode(ctx context.Context, code string) (SteamImport, error) {
	var out SteamImport
	err := c.doJSON(ctx, http.MethodPost, "/api/steam/import", map[string]string{"code": code}, &out)
	return out, err
}
