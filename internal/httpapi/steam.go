package httpapi

import (
	"bytes"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/rechedev9/cliphub/internal/demozstd"
	"github.com/rechedev9/cliphub/internal/rules"
	"github.com/rechedev9/cliphub/internal/steamresolve"
)

const (
	// #nosec G101 -- API status constants, never credentials.
	steamCredentialsRequired  = "steam_credentials_required"
	steamHistoryNotConfigured = "history_not_configured"
	steamNeedKnownCode        = "need_known_code"
	steamDemoUnavailable      = "demo_unavailable"
	steamAccountInvalid       = "steam_account_invalid"
)

// WithSteamResolver wires the share-code resolver. Without this option the
// handlers fall back to a decode-only resolver (no Game Coordinator
// transport), so no existing constructor call site has to change.
func WithSteamResolver(resolver *steamresolve.Service) Option {
	return func(h *Handlers) {
		h.steamResolver = resolver
	}
}

func (h *Handlers) rememberSteamSession(session steamresolve.Session) {
	if !session.Complete() {
		return
	}
	h.steamSessionMu.Lock()
	h.steamSessionCache = session
	h.steamSessionMu.Unlock()
}

func (h *Handlers) cachedSteamSession() steamresolve.Session {
	h.steamSessionMu.Lock()
	defer h.steamSessionMu.Unlock()
	return h.steamSessionCache
}

func (h *Handlers) steamSession(req steamresolve.Session) steamresolve.Session {
	if req.Complete() {
		return req
	}
	if env := steamresolve.SessionFromEnv(); env.Complete() {
		return env
	}
	return h.cachedSteamSession()
}

func (h *Handlers) gcConfigured() bool {
	return h.steamSession(steamresolve.Session{}).Complete()
}

func (h *Handlers) resolverFor(session steamresolve.Session) *steamresolve.Service {
	if h.steamTransport != nil {
		return steamresolve.NewService(h.steamTransport)
	}
	if session.Complete() && h.steamFactory != nil {
		return steamresolve.NewService(h.steamFactory(session))
	}
	if h.steamResolver != nil {
		return h.steamResolver
	}
	return steamresolve.NewService(nil)
}

func (h *Handlers) steamResolveService() *steamresolve.Service {
	return h.resolverFor(h.steamSession(steamresolve.Session{}))
}

// ResolveShareCode handles POST /api/steam/sharecode: it decodes a CS2 share
// code and, when a Game Coordinator session is available, resolves the match's
// demo URL. The two 64-bit identifiers are emitted as strings: they exceed
// JavaScript's 2^53 integer precision.
func (h *Handlers) ResolveShareCode(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Code string `json:"code"`
	}
	if err := decodeSingleJSONBody(w, r, &req, false); err != nil {
		writeError(w, http.StatusBadRequest, "invalid share code request JSON")
		return
	}
	res, err := h.steamResolveService().Resolve(r.Context(), req.Code)
	if errors.Is(err, steamresolve.ErrInvalidCode) {
		writeCodedError(w, http.StatusBadRequest, "invalid_share_code", err.Error())
		return
	}
	if err != nil {
		writeCodedError(w, http.StatusBadGateway, "steam_unavailable", "Steam Game Coordinator request failed")
		return
	}
	if h.steamAccounts != nil {
		_ = h.steamAccounts.RememberCode(req.Code)
	}
	writeShareCodeResult(w, res)
}

func writeShareCodeResult(w http.ResponseWriter, res steamresolve.Result) {
	status := "decoded"
	if res.Resolved {
		status = "resolved"
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"status":    status,
		"matchId":   strconv.FormatUint(res.MatchID, 10),
		"outcomeId": strconv.FormatUint(res.OutcomeID, 10),
		"tokenId":   res.TokenID,
		"demoUrl":   res.DemoURL,
	})
}

// GetSteamAccount handles GET /api/steam/account.
func (h *Handlers) GetSteamAccount(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, h.steamAccountPayload())
}

// PutSteamAccount handles PUT /api/steam/account.
func (h *Handlers) PutSteamAccount(w http.ResponseWriter, r *http.Request) {
	if h.steamAccounts == nil {
		writeCodedError(w, http.StatusServiceUnavailable, steamHistoryNotConfigured, "Steam history store is not configured")
		return
	}
	var req struct {
		SteamID   string `json:"steamId"`
		AuthCode  string `json:"authCode"`
		APIKey    string `json:"apiKey"`
		KnownCode string `json:"knownCode"`
	}
	if err := decodeSingleJSONBody(w, r, &req, false); err != nil {
		writeError(w, http.StatusBadRequest, "invalid steam account JSON")
		return
	}
	acc := steamresolve.Account{
		AuthCode:  strings.TrimSpace(req.AuthCode),
		APIKey:    strings.TrimSpace(req.APIKey),
		KnownCode: strings.TrimSpace(req.KnownCode),
	}
	if strings.TrimSpace(req.SteamID) != "" {
		id, err := steamresolve.ParseSteamID(req.SteamID)
		if err != nil {
			writeCodedError(w, http.StatusBadRequest, steamAccountInvalid, err.Error())
			return
		}
		acc.SteamID = id
	}
	if _, err := h.steamAccounts.Save(acc); err != nil {
		writeCodedError(w, http.StatusBadRequest, steamAccountInvalid, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, h.steamAccountPayload())
}

// DeleteSteamAccount handles DELETE /api/steam/account.
func (h *Handlers) DeleteSteamAccount(w http.ResponseWriter, r *http.Request) {
	if h.steamAccounts == nil {
		writeCodedError(w, http.StatusServiceUnavailable, steamHistoryNotConfigured, "Steam history store is not configured")
		return
	}
	if err := h.steamAccounts.Clear(); err != nil {
		internalError(w, "clear steam account", err)
		return
	}
	writeJSON(w, http.StatusOK, h.steamAccountPayload())
}

// SyncSteamMatches handles POST /api/steam/matches/sync.
func (h *Handlers) SyncSteamMatches(w http.ResponseWriter, r *http.Request) {
	if h.steamAccounts == nil || h.steamHistory == nil {
		writeCodedError(w, http.StatusServiceUnavailable, steamHistoryNotConfigured, "Steam history is not configured")
		return
	}
	acc, err := h.steamAccounts.Load()
	if err != nil {
		internalError(w, "load steam account", err)
		return
	}
	if !acc.HistoryConfigured() {
		writeCodedError(w, http.StatusConflict, steamHistoryNotConfigured, "Connect a Steam authentication code before syncing matches")
		return
	}
	known, discovered, err := h.steamHistory.Walk(r.Context(), acc, 0)
	if errors.Is(err, steamresolve.ErrNeedKnownCode) {
		writeCodedError(w, http.StatusConflict, steamNeedKnownCode, "Pega primero un código de partida para arrancar el historial")
		return
	}
	if err != nil {
		writeCodedError(w, http.StatusBadGateway, "steam_history_unavailable", "Steam match history request failed")
		return
	}
	merged := acc.Matches
	for i := len(discovered) - 1; i >= 0; i-- {
		merged = prependStoredMatch(merged, discovered[i])
	}
	if err := h.steamAccounts.ReplaceMatches(known, merged); err != nil {
		internalError(w, "save steam matches", err)
		return
	}
	writeJSON(w, http.StatusOK, h.steamAccountPayload())
}

// ImportShareCode handles POST /api/steam/import: resolve, download, enqueue.
func (h *Handlers) ImportShareCode(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Code     string `json:"code"`
		Username string `json:"username"`
		Password string `json:"password"`
		Guard    string `json:"guard"`
	}
	if err := decodeSingleJSONBody(w, r, &req, false); err != nil {
		writeError(w, http.StatusBadRequest, "invalid steam import JSON")
		return
	}
	session := h.steamSession(steamresolve.Session{
		Username: strings.TrimSpace(req.Username),
		Password: strings.TrimSpace(req.Password),
		Guard:    strings.TrimSpace(req.Guard),
	})
	if !session.Complete() {
		writeCodedError(w, http.StatusConflict, steamCredentialsRequired, "Steam login is required to download this demo")
		return
	}
	res, err := h.resolverFor(session).Resolve(r.Context(), req.Code)
	if errors.Is(err, steamresolve.ErrInvalidCode) {
		writeCodedError(w, http.StatusBadRequest, "invalid_share_code", err.Error())
		return
	}
	if err != nil {
		writeCodedError(w, http.StatusBadGateway, "steam_unavailable", "Steam Game Coordinator request failed")
		return
	}
	if !res.Resolved || res.DemoURL == "" {
		writeCodedError(w, http.StatusConflict, steamDemoUnavailable, "Valve has no downloadable demo for this match")
		return
	}
	if h.steamFetcher == nil {
		writeCodedError(w, http.StatusServiceUnavailable, "steam_unavailable", "Steam demo download is not configured")
		return
	}
	body, fileName, err := h.steamFetcher.Open(r.Context(), res.DemoURL, maxDemoBytes)
	if err != nil {
		writeCodedError(w, http.StatusBadGateway, "steam_unavailable", "Could not download the demo from Valve")
		return
	}
	defer body.Close()

	opened, fileName, err := demozstd.Open(body, fileName, maxDemoBytes)
	if err != nil {
		writeError(w, http.StatusBadRequest, "could not read downloaded demo")
		return
	}
	defer opened.Close()

	var header [8]byte
	n, err := io.ReadFull(opened, header[:])
	if err != nil && err != io.ErrUnexpectedEOF && err != io.EOF {
		internalError(w, "read downloaded demo header", err)
		return
	}
	if !isDemoHeader(header[:n]) {
		writeError(w, http.StatusBadRequest, "downloaded file is not a CS2 demo")
		return
	}
	demo := io.MultiReader(bytes.NewReader(header[:n]), opened)

	target := ""
	if acc, err := h.loadSteamAccount(); err == nil {
		target = acc.SteamID
	}
	created, err := h.persistAndEnqueueDemo(r.Context(), demo, fileName, target, "", rules.Default())
	if err != nil {
		internalError(w, "admit downloaded demo", err)
		return
	}
	h.rememberSteamSession(session)
	if h.steamAccounts != nil {
		_ = h.steamAccounts.RememberCode(req.Code)
	}
	writeJSON(w, http.StatusCreated, map[string]any{
		"id":      created.ID,
		"status":  created.Status,
		"matchId": strconv.FormatUint(res.MatchID, 10),
	})
}

func (h *Handlers) loadSteamAccount() (steamresolve.Account, error) {
	if h.steamAccounts == nil {
		return steamresolve.Account{}, steamresolve.ErrAccountNotConfigured
	}
	return h.steamAccounts.Load()
}

func (h *Handlers) steamAccountPayload() map[string]any {
	acc, err := h.loadSteamAccount()
	if err != nil {
		acc = steamresolve.Account{}
	}
	matches := make([]map[string]any, 0, len(acc.Matches))
	for _, m := range acc.Matches {
		matches = append(matches, map[string]any{
			"shareCode":    m.ShareCode,
			"matchId":      m.MatchID,
			"discoveredAt": m.DiscoveredAt,
		})
	}
	return map[string]any{
		"steamId":           acc.SteamID,
		"authCodeSet":       acc.AuthCode != "",
		"apiKeySet":         acc.APIKey != "",
		"knownCode":         acc.KnownCode,
		"historyConfigured": acc.HistoryConfigured(),
		"gcConfigured":      h.gcConfigured(),
		"matches":           matches,
	}
}

func prependStoredMatch(existing []steamresolve.StoredMatch, next steamresolve.StoredMatch) []steamresolve.StoredMatch {
	out := make([]steamresolve.StoredMatch, 0, 1+len(existing))
	out = append(out, next)
	for _, item := range existing {
		if item.ShareCode == next.ShareCode {
			continue
		}
		out = append(out, item)
		if len(out) == steamresolve.MaxStoredMatches {
			break
		}
	}
	return out
}
