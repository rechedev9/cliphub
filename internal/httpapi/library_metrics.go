package httpapi

import (
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"sort"
	"strconv"

	"github.com/google/uuid"

	"github.com/rechedev9/cliphub/internal/library"
	"github.com/rechedev9/cliphub/internal/storage"
	"github.com/rechedev9/cliphub/internal/tacticalplan"
)

// Player metrics read the player's most recent matches, at most
// library.MaxReportMatches, like a scouting report.
const playerMetricsDefaultLimit = 20

// metricsResponse is the advanced metrics of a match, a player or a report,
// with the matches that could not be read.
type metricsResponse struct {
	MatchID   *uuid.UUID     `json:"match_id,omitempty"`
	SteamID64 string         `json:"steamid64,omitempty"`
	TeamID    *uuid.UUID     `json:"team_id,omitempty"`
	TeamName  string         `json:"team_name,omitempty"`
	Skipped   []scoutSkipped `json:"skipped"`
	tacticalplan.MetricsReport
}

// inferRoles reads each player's role from the same documents the metrics
// read, so the role breakdown never mixes in matches outside the selection.
func inferRoles(steamIDs []string, docs []library.RoleDocument) map[string]string {
	roles := make(map[string]string, len(steamIDs))
	for _, id := range steamIDs {
		roles[id] = library.InferPlayerRole(id, docs).Role
	}
	return roles
}

// teamSteamIDs lists the players of a team key across the matches, or every
// player when the key is empty.
func teamSteamIDs(matches []tacticalplan.MetricsMatch) []string {
	seen := map[string]bool{}
	out := []string{}
	for _, m := range matches {
		for _, p := range m.Document.Players {
			if p.SteamID64 == "" || seen[p.SteamID64] || (m.TeamKey != "" && p.TeamKey != m.TeamKey) {
				continue
			}
			seen[p.SteamID64] = true
			out = append(out, p.SteamID64)
		}
	}
	sort.Strings(out)
	return out
}

func roleDocuments(matches []tacticalplan.MetricsMatch) []library.RoleDocument {
	docs := make([]library.RoleDocument, 0, len(matches))
	for _, m := range matches {
		docs = append(docs, library.RoleDocument{MatchID: m.MatchID, Document: m.Document})
	}
	return docs
}

// GetLibraryMatchMetrics handles GET /api/library/matches/{id}/metrics: every
// player's advanced metrics in the match, with the win probability of each round.
func (h *Handlers) GetLibraryMatchMetrics(w http.ResponseWriter, r *http.Request) {
	m, ok := h.loadLibraryMatch(w, r)
	if !ok {
		return
	}
	doc, err := h.readLibraryDocument(m.DemoID)
	if err != nil {
		if storage.IsNotExist(err) {
			writeCodedError(w, http.StatusConflict, codeTacticalNotReady, "tactical analysis is not ready")
			return
		}
		internalError(w, "read tactical document", err)
		return
	}
	matches := []tacticalplan.MetricsMatch{{
		MatchID: m.ID, PlayedAt: m.PlayedAt, Document: doc, Setup: h.readSetupFrames(m.DemoID, doc),
	}}
	report := tacticalplan.Metrics(matches, tacticalplan.MetricsOptions{
		Timelines: true, Roles: inferRoles(teamSteamIDs(matches), roleDocuments(matches)),
	})
	id := m.ID
	writeJSON(w, http.StatusOK, metricsResponse{MatchID: &id, Skipped: []scoutSkipped{}, MetricsReport: report})
}

// playerMetricsQuery reads the filter of a player's metrics.
func playerMetricsQuery(q url.Values) (library.MatchFilter, tacticalplan.Side, error) {
	f := library.MatchFilter{Limit: playerMetricsDefaultLimit}
	var err error
	if m := q.Get("map"); m != "" {
		if !mapNamePattern.MatchString(m) {
			return f, "", errors.New("map must match ^[a-z0-9_]+$")
		}
		f.Map = m
	}
	side, err := parseScoutSide(q.Get("side"))
	if err != nil {
		return f, "", err
	}
	if f.From, err = optionalTimeQuery(q, "from"); err != nil {
		return f, "", err
	}
	if f.To, err = optionalTimeQuery(q, "to"); err != nil {
		return f, "", err
	}
	if raw := q.Get("limit"); raw != "" {
		limit, err := strconv.Atoi(raw)
		if err != nil || limit < 1 || limit > library.MaxReportMatches {
			return f, "", fmt.Errorf("limit must be an integer from 1 to %d", library.MaxReportMatches)
		}
		f.Limit = limit
	}
	return f, side, nil
}

// GetLibraryPlayerMetrics handles GET /api/library/players/{steamid}/metrics:
// the player's advanced metrics over their most recent matches.
func (h *Handlers) GetLibraryPlayerMetrics(w http.ResponseWriter, r *http.Request) {
	steamID, ok := h.libraryPlayerParam(w, r)
	if !ok {
		return
	}
	filter, side, err := playerMetricsQuery(r.URL.Query())
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if _, err := h.libraryRepo.GetPlayer(r.Context(), steamID); err != nil {
		writeLibraryError(w, "get player", err)
		return
	}
	filter.SteamID = steamID
	listed, err := h.libraryRepo.ListMatches(r.Context(), filter)
	if err != nil {
		internalError(w, "list player matches", err)
		return
	}
	skipped := []scoutSkipped{}
	matches := make([]tacticalplan.MetricsMatch, 0, len(listed))
	for _, m := range listed {
		doc, err := h.readLibraryDocument(m.DemoID)
		if err != nil {
			log.Printf("httpapi: metrics of %s skip match %s: %v", steamID, m.ID, err)
			skipped = append(skipped, scoutSkipped{MatchID: m.ID, Reason: scoutSkipNotAnalyzed})
			continue
		}
		matches = append(matches, tacticalplan.MetricsMatch{
			MatchID: m.ID, PlayedAt: m.PlayedAt, Document: doc, Setup: h.readSetupFrames(m.DemoID, doc),
		})
	}
	report := tacticalplan.Metrics(matches, tacticalplan.MetricsOptions{
		SteamIDs: []string{steamID}, Side: side, Roles: inferRoles([]string{steamID}, roleDocuments(matches)),
	})
	writeJSON(w, http.StatusOK, metricsResponse{SteamID64: steamID, Skipped: skipped, MetricsReport: report})
}

// GetLibraryReportMetrics handles GET /api/library/reports/{id}/metrics: the
// advanced metrics of the scouted team's players over the report's matches.
func (h *Handlers) GetLibraryReportMetrics(w http.ResponseWriter, r *http.Request) {
	report, ok := h.loadLibraryReport(w, r)
	if !ok {
		return
	}
	team, err := h.libraryRepo.GetTeam(r.Context(), report.TeamID)
	if err != nil {
		writeLibraryError(w, "get team", err)
		return
	}
	side := tacticalplan.Side(report.Side)
	teamID := team.ID
	resp := metricsResponse{TeamID: &teamID, TeamName: team.Name, Skipped: []scoutSkipped{}}
	if len(report.MatchIDs) == 0 {
		resp.MetricsReport = tacticalplan.Metrics(nil, tacticalplan.MetricsOptions{Side: side})
		writeJSON(w, http.StatusOK, resp)
		return
	}
	loaded, skipped, err := h.loadTeamMatches(r.Context(), team, report.MatchIDs, tacticalplan.ScoutFilter{Side: side})
	if err != nil {
		internalError(w, "load report matches", err)
		return
	}
	matches := make([]tacticalplan.MetricsMatch, 0, len(loaded))
	for _, sm := range loaded {
		matches = append(matches, tacticalplan.MetricsMatch{
			MatchID: sm.MatchID, PlayedAt: sm.PlayedAt, Document: sm.Document, Setup: sm.Setup, TeamKey: sm.TeamKey,
		})
	}
	resp.Skipped = skipped
	resp.MetricsReport = tacticalplan.Metrics(matches, tacticalplan.MetricsOptions{
		Side: side, Roles: inferRoles(teamSteamIDs(matches), roleDocuments(matches)),
	})
	writeJSON(w, http.StatusOK, resp)
}
