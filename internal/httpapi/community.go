package httpapi

import (
	"context"
	"errors"
	"log"
	"net/http"
	"slices"
	"time"

	"github.com/google/uuid"

	"github.com/rechedev9/cliphub/internal/community"
	"github.com/rechedev9/cliphub/internal/library"
	"github.com/rechedev9/cliphub/internal/tacticalplan"
)

const (
	// rankingsSubmitEvery is how often an opted-in Studio refreshes its submission.
	rankingsSubmitEvery = 24 * time.Hour
	codeCommunityOff    = "community_disabled"
	codeCommunityDown   = "community_unavailable"
	codeRankingsOff     = "rankings_disabled"
	codeSteamIDRequired = "steam_id_required"
)

var (
	errRankingsOff     = errors.New("rankings are turned off")
	errSteamIDRequired = errors.New("set your SteamID in Ajustes first")
)

// CommunityDeps are the community server pieces; a nil Client means the server is disabled.
type CommunityDeps struct {
	Client    *community.Client
	Reference *community.ProReferenceCache
	Settings  *community.SettingsStore
	Lineups   *community.LineupsCache
}

// WithCommunity wires the server pro reference and the rankings opt-in.
func WithCommunity(deps CommunityDeps) Option {
	return func(h *Handlers) {
		h.references.community = deps
	}
}

// proReference is the server reference when Studio has one, else the bundled one.
func (h *Handlers) proReference() (tacticalplan.ProReference, string) {
	if c := h.references.community.Reference; c != nil {
		if ref, server := c.Current(); server {
			return ref, referenceSourceServer
		}
	}
	return tacticalplan.DefaultProReference(), referenceSourceBundled
}

// StartCommunity refreshes the pro reference and, when opted in, the
// rankings submission in the background until ctx ends.
func (h *Handlers) StartCommunity(ctx context.Context) {
	deps := h.references.community
	if deps.Reference != nil {
		go deps.Reference.Run(ctx)
	}
	if deps.Client == nil || deps.Settings == nil {
		return
	}
	go func() {
		for {
			if s, err := deps.Settings.Load(); err == nil && s.RankingsEnabled && time.Since(s.LastSubmittedAt) >= rankingsSubmitEvery {
				if n, err := h.submitRankings(ctx); err != nil {
					log.Printf("community: rankings submission failed: %v", err)
				} else {
					log.Printf("community: submitted rankings for %d maps", n)
				}
			}
			select {
			case <-ctx.Done():
				return
			case <-time.After(time.Hour):
			}
		}
	}()
}

type communitySettingsResponse struct {
	RankingsEnabled bool       `json:"rankings_enabled"`
	ServerEnabled   bool       `json:"server_enabled"`
	SteamIDSet      bool       `json:"steam_id_set"`
	LastSubmittedAt *time.Time `json:"last_submitted_at,omitempty"`
	SubmitError     string     `json:"submit_error,omitempty"`
}

func (h *Handlers) communitySettings() (communitySettingsResponse, error) {
	deps := h.references.community
	out := communitySettingsResponse{ServerEnabled: deps.Client != nil}
	if acc, err := h.loadSteamAccount(); err == nil {
		out.SteamIDSet = acc.SteamID != ""
	}
	if deps.Settings == nil {
		return out, nil
	}
	s, err := deps.Settings.Load()
	if err != nil {
		return out, err
	}
	out.RankingsEnabled = s.RankingsEnabled
	if !s.LastSubmittedAt.IsZero() {
		out.LastSubmittedAt = &s.LastSubmittedAt
	}
	return out, nil
}

// GetCommunitySettings handles GET /api/library/community/settings.
func (h *Handlers) GetCommunitySettings(w http.ResponseWriter, r *http.Request) {
	out, err := h.communitySettings()
	if err != nil {
		internalError(w, "community settings", err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

// PutCommunitySettings handles PUT /api/library/community/settings; turning
// rankings on submits right away.
func (h *Handlers) PutCommunitySettings(w http.ResponseWriter, r *http.Request) {
	var req struct {
		RankingsEnabled *bool `json:"rankings_enabled"`
	}
	if err := decodeSingleJSONBody(w, r, &req, true); err != nil || req.RankingsEnabled == nil {
		writeError(w, http.StatusBadRequest, "body must be {\"rankings_enabled\": boolean}")
		return
	}
	deps := h.references.community
	if deps.Settings == nil || (deps.Client == nil && *req.RankingsEnabled) {
		writeCodedError(w, http.StatusServiceUnavailable, codeCommunityOff, "the community server is disabled")
		return
	}
	if _, err := deps.Settings.Update(func(s *community.Settings) { s.RankingsEnabled = *req.RankingsEnabled }); err != nil {
		internalError(w, "community settings", err)
		return
	}
	submitErr := ""
	if *req.RankingsEnabled {
		if _, err := h.submitRankings(r.Context()); err != nil {
			log.Printf("community: rankings submission failed: %v", err)
			submitErr = submitErrorCode(err)
		}
	}
	out, err := h.communitySettings()
	if err != nil {
		internalError(w, "community settings", err)
		return
	}
	out.SubmitError = submitErr
	writeJSON(w, http.StatusOK, out)
}

func submitErrorCode(err error) string {
	switch {
	case errors.Is(err, errSteamIDRequired):
		return codeSteamIDRequired
	case errors.Is(err, errRankingsOff):
		return codeRankingsOff
	}
	return codeCommunityDown
}

// SubmitCommunityRankings handles POST /api/library/community/rankings/submit.
func (h *Handlers) SubmitCommunityRankings(w http.ResponseWriter, r *http.Request) {
	if !h.libraryReady(w) {
		return
	}
	n, err := h.submitRankings(r.Context())
	if err != nil {
		writeSubmitError(w, err)
		return
	}
	out, err := h.communitySettings()
	if err != nil {
		internalError(w, "community settings", err)
		return
	}
	writeJSON(w, http.StatusOK, struct {
		communitySettingsResponse
		Maps int `json:"maps"`
	}{out, n})
}

func writeSubmitError(w http.ResponseWriter, err error) {
	switch code := submitErrorCode(err); code {
	case codeSteamIDRequired, codeRankingsOff:
		writeCodedError(w, http.StatusConflict, code, err.Error())
	default:
		log.Printf("community: rankings: %v", err)
		writeCodedError(w, http.StatusBadGateway, code, "the community server did not answer")
	}
}

// DeleteCommunityRankings handles DELETE /api/library/community/rankings: the
// server forgets the install, rankings turn off and the install id is replaced.
func (h *Handlers) DeleteCommunityRankings(w http.ResponseWriter, r *http.Request) {
	deps := h.references.community
	if deps.Client == nil || deps.Settings == nil {
		writeCodedError(w, http.StatusServiceUnavailable, codeCommunityOff, "the community server is disabled")
		return
	}
	s, err := deps.Settings.Load()
	if err != nil {
		internalError(w, "community settings", err)
		return
	}
	if err := deps.Client.Delete(r.Context(), s.InstallID); err != nil {
		writeSubmitError(w, err)
		return
	}
	if _, err := deps.Settings.Update(func(s *community.Settings) {
		*s = community.Settings{InstallID: uuid.NewString()}
	}); err != nil {
		internalError(w, "community settings", err)
		return
	}
	log.Printf("community: rankings deleted on the server; a new install id replaces the old one")
	out, err := h.communitySettings()
	if err != nil {
		internalError(w, "community settings", err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

// submitRankings sends the own player's line of every map with enough rounds.
func (h *Handlers) submitRankings(ctx context.Context) (int, error) {
	deps := h.references.community
	if deps.Client == nil || deps.Settings == nil {
		return 0, errors.New("community server disabled")
	}
	s, err := deps.Settings.Load()
	if err != nil {
		return 0, err
	}
	if !s.RankingsEnabled {
		return 0, errRankingsOff
	}
	maps, err := h.ownRankingMaps(ctx)
	if err != nil {
		return 0, err
	}
	if err := deps.Client.Submit(ctx, community.RankingSubmission{InstallID: s.InstallID, Maps: maps}); err != nil {
		return 0, err
	}
	_, err = deps.Settings.Update(func(s *community.Settings) { s.LastSubmittedAt = time.Now().UTC() })
	return len(maps), err
}

// ownSteamID is the SteamID set in Ajustes, the player whose numbers rankings use.
func (h *Handlers) ownSteamID() (string, error) {
	acc, err := h.loadSteamAccount()
	if err != nil || acc.SteamID == "" {
		return "", errSteamIDRequired
	}
	return acc.SteamID, nil
}

// ownRankingMaps is the own player's line on every map, and on all maps pooled.
func (h *Handlers) ownRankingMaps(ctx context.Context) ([]community.RankingMap, error) {
	if h.libraryRepo == nil {
		return nil, errors.New("library is not configured")
	}
	steamID, err := h.ownSteamID()
	if err != nil {
		return nil, err
	}
	matches, err := h.allLibraryMatches(ctx, library.MatchFilter{})
	if err != nil {
		return nil, err
	}
	names := []string{""}
	for _, m := range matches {
		if m.Map != "" && !slices.Contains(names, m.Map) && community.MapPattern.MatchString(m.Map) && m.Map != community.AllMaps {
			names = append(names, m.Map)
		}
	}
	slices.Sort(names[1:])
	out := []community.RankingMap{}
	for _, name := range names {
		line, ok, err := h.ownLine(ctx, steamID, name)
		if err != nil {
			return nil, err
		}
		if !ok || line.Rounds < community.MinRankingRounds {
			continue
		}
		out = append(out, community.RankingMap{Map: rankingMapKey(name), Rounds: line.Rounds, Matches: line.Matches, Values: line.Values})
		if len(out) == community.MaxRankingMaps {
			break
		}
	}
	return out, nil
}

func (h *Handlers) ownLine(ctx context.Context, steamID, mapName string) (tacticalplan.PlayerLine, bool, error) {
	entry, err := h.baselineSample(ctx, baselineQuery{scope: baselineScopeLibrary, mapName: mapName, minRounds: tacticalplan.BaselineMinRounds})
	if err != nil {
		return tacticalplan.PlayerLine{}, false, err
	}
	line, ok := entry.sample.Player(steamID)
	return line, ok, nil
}

func rankingMapKey(mapName string) string {
	if mapName == "" {
		return community.AllMaps
	}
	return mapName
}

type globalMetric struct {
	Metric   string   `json:"metric"`
	Value    *float64 `json:"value"`
	Installs int      `json:"installs"`
	// Percentile is the share of installs this one beats, from 0 to 100.
	Percentile *float64 `json:"percentile"`
	Median     *float64 `json:"median"`
}

type globalRankingsResponse struct {
	Map             string         `json:"map"`
	RankingsEnabled bool           `json:"rankings_enabled"`
	SteamIDSet      bool           `json:"steam_id_set"`
	MinInstalls     int            `json:"min_installs"`
	MinRounds       int            `json:"min_rounds"`
	Rounds          int            `json:"rounds"`
	Matches         int            `json:"matches"`
	Metrics         []globalMetric `json:"metrics"`
}

// GetCommunityRankings handles GET /api/library/community/rankings: the own
// player's percentile on each metric against every opted-in install.
func (h *Handlers) GetCommunityRankings(w http.ResponseWriter, r *http.Request) {
	if !h.libraryReady(w) {
		return
	}
	mapName := r.URL.Query().Get("map")
	if mapName != "" && (!mapNamePattern.MatchString(mapName) || !community.MapPattern.MatchString(mapName)) {
		writeError(w, http.StatusBadRequest, "map must be a map name like de_mirage")
		return
	}
	settings, err := h.communitySettings()
	if err != nil {
		internalError(w, "community settings", err)
		return
	}
	out := globalRankingsResponse{
		Map: mapName, RankingsEnabled: settings.RankingsEnabled, SteamIDSet: settings.SteamIDSet,
		MinInstalls: community.MinRankingInstalls, MinRounds: community.MinRankingRounds, Metrics: []globalMetric{},
	}
	if !settings.RankingsEnabled || h.references.community.Client == nil {
		writeJSON(w, http.StatusOK, out)
		return
	}
	dist, err := h.references.community.Client.Rankings(r.Context(), rankingMapKey(mapName))
	if err != nil {
		writeSubmitError(w, err)
		return
	}
	var own tacticalplan.PlayerLine
	if steamID, err := h.ownSteamID(); err == nil {
		line, ok, err := h.ownLine(r.Context(), steamID, mapName)
		if err != nil {
			internalError(w, "community rankings", err)
			return
		}
		if ok {
			own = line
		}
	}
	out.Rounds, out.Matches = own.Rounds, own.Matches
	out.Metrics = globalMetrics(dist, own)
	writeJSON(w, http.StatusOK, out)
}

// globalMetrics places the own values in the server distributions; a player
// under the round minimum gets no value, as the server never ranked them.
func globalMetrics(dist community.RankingsResponse, own tacticalplan.PlayerLine) []globalMetric {
	out := make([]globalMetric, 0, len(tacticalplan.ReferenceMetrics))
	for _, metric := range tacticalplan.ReferenceMetrics {
		d := dist.Metrics[metric]
		row := globalMetric{Metric: metric, Installs: d.Installs}
		if len(d.Quantiles) == community.QuantilePoints {
			median := d.Quantiles[community.QuantilePoints/2]
			row.Median = &median
		}
		v, ok := own.Values[metric]
		if ok && own.Rounds >= community.MinRankingRounds {
			row.Value = &v
			if row.Median != nil {
				p := community.Percentile(d.Quantiles, v)
				if tacticalplan.LowerIsBetter(metric) {
					p = 100 - p
				}
				row.Percentile = &p
			}
		}
		out = append(out, row)
	}
	return out
}
