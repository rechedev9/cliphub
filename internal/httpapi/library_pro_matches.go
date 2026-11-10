package httpapi

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/rechedev9/cliphub/internal/community"
	"github.com/rechedev9/cliphub/internal/library"
	"github.com/rechedev9/cliphub/internal/tacticalplan"
)

// Catalog states the Partidas pro page shows: the server answered, Studio has
// no community server, the server has no catalog yet, or it did not answer.
const (
	proCatalogOK          = "ok"
	proCatalogDisabled    = "disabled"
	proCatalogMissing     = "missing"
	proCatalogUnavailable = "unavailable"
	codeNoProMap          = "no_pro_map"
	codeProMapPending     = "pro_map_pending"
	codeProMapUnreadable  = "pro_map_unreadable"
)

// libraryProRepository is the slice of the library store the pro match pages use.
type libraryProRepository interface {
	SaveProMap(ctx context.Context, m *library.ProMap) error
	GetProMap(ctx context.Context, matchID int, entry string) (library.ProMap, error)
	ProMapForDemo(ctx context.Context, demoID uuid.UUID) (library.ProMap, error)
	ListProMaps(ctx context.Context, matchID int) ([]library.ProMap, error)
	FollowProTeam(ctx context.Context, f *library.ProFollow) error
	UnfollowProTeam(ctx context.Context, teamID int) error
	ListProFollows(ctx context.Context) ([]library.ProFollow, error)
	MarkProFollowSynced(ctx context.Context, teamID int, at time.Time) error
}

// proMatchState serializes pro imports, so the follow sync and a click never store one map twice.
type proMatchState struct {
	importMu sync.Mutex
	wake     chan struct{}
	// broken holds the catalog hashes that failed to read, so the sync does not fetch them again; importMu guards it.
	broken map[string]bool
}

type proTeamView struct {
	community.ProTeam
	Followed bool `json:"followed"`
}

type proMapView struct {
	community.ProMapFile
	// LibraryMatchID is the library match of an imported map.
	LibraryMatchID *uuid.UUID `json:"library_match_id,omitempty"`
	RadarMayBeOff  bool       `json:"radar_may_be_off"`
	// OldAnalysis is an analysis older than the current scan, which reads less from the demo.
	OldAnalysis bool `json:"old_analysis"`
}

type proMatchView struct {
	ID       int           `json:"id"`
	Event    string        `json:"event"`
	PlayedAt time.Time     `json:"played_at"`
	HLTVURL  string        `json:"hltv_url"`
	Teams    []proTeamView `json:"teams"`
	Maps     []proMapView  `json:"maps"`
	// Series is the format, veto and pick of each map, as far as the server knows them.
	Series community.ProSeries `json:"series"`
}

type proMatchesResponse struct {
	Status    string              `json:"status"`
	UpdatedAt *time.Time          `json:"updated_at,omitempty"`
	Days      int                 `json:"days"`
	Page      int                 `json:"page"`
	Pages     int                 `json:"pages"`
	Total     int                 `json:"total"`
	Matches   []proMatchView      `json:"matches"`
	Facets    community.ProFacets `json:"facets"`
	Follows   []library.ProFollow `json:"follows"`
}

// ListLibraryProMatches handles GET /api/library/pro-matches: the server's pro
// match catalog with what this library imported and follows.
func (h *Handlers) ListLibraryProMatches(w http.ResponseWriter, r *http.Request) {
	if !h.libraryReady(w) {
		return
	}
	q, err := community.ParseProMatchQuery(r.URL.Query())
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	follows, err := h.libraryRepo.ListProFollows(r.Context())
	if err != nil {
		internalError(w, "list pro follows", err)
		return
	}
	resp := proMatchesResponse{Status: proCatalogOK, Days: q.Days, Page: q.Page, Pages: 1, Matches: []proMatchView{}, Follows: follows,
		Facets: community.ProFacets{Teams: []community.ProTeamCount{}, Events: []community.ProNameCount{}, Maps: []community.ProNameCount{}}}
	client := h.references.community.Client
	if client == nil {
		resp.Status = proCatalogDisabled
		writeJSON(w, http.StatusOK, resp)
		return
	}
	page, err := client.ProMatches(r.Context(), q)
	switch {
	case errors.Is(err, community.ErrNoProCatalog):
		resp.Status = proCatalogMissing
	case err != nil:
		log.Printf("httpapi: pro matches: %v", err)
		resp.Status = proCatalogUnavailable
	default:
		views, err := h.proMatchViews(r.Context(), page.Matches, follows)
		if err != nil {
			internalError(w, "annotate pro matches", err)
			return
		}
		at := page.UpdatedAt
		resp.UpdatedAt, resp.Pages, resp.Total, resp.Matches, resp.Facets = &at, page.Pages, page.Total, views, page.Facets
	}
	writeJSON(w, http.StatusOK, resp)
}

func (h *Handlers) proMatchViews(ctx context.Context, matches []community.ProMatch, follows []library.ProFollow) ([]proMatchView, error) {
	followed := map[int]bool{}
	for _, f := range follows {
		followed[f.TeamID] = true
	}
	out := make([]proMatchView, 0, len(matches))
	for _, m := range matches {
		imported, err := h.importedProMaps(ctx, m.ID)
		if err != nil {
			return nil, err
		}
		v := proMatchView{ID: m.ID, Event: m.Event, PlayedAt: m.PlayedAt, HLTVURL: community.ProMatchURL(m.ID), Teams: []proTeamView{}, Maps: []proMapView{}, Series: m.Series}
		if v.Series.Maps == nil {
			v.Series.Maps = []community.ProSeriesMap{}
		}
		if v.Series.Veto == nil {
			v.Series.Veto = []community.ProVetoStep{}
		}
		for _, t := range m.Teams {
			v.Teams = append(v.Teams, proTeamView{ProTeam: t, Followed: followed[t.ID]})
		}
		for _, f := range m.Maps {
			v.Maps = append(v.Maps, proMapView{ProMapFile: f, LibraryMatchID: imported[f.Entry], RadarMayBeOff: library.RadarMayBeOff(f.Map, f.AnalysisVersion),
				OldAnalysis: f.AnalysisVersion < tacticalplan.AnalysisVersion})
		}
		out = append(out, v)
	}
	return out, nil
}

// importedProMaps maps each imported entry of an HLTV match to its library match.
func (h *Handlers) importedProMaps(ctx context.Context, matchID int) (map[string]*uuid.UUID, error) {
	records, err := h.libraryRepo.ListProMaps(ctx, matchID)
	if err != nil {
		return nil, err
	}
	out := map[string]*uuid.UUID{}
	for _, rec := range records {
		if rec.DemoID == nil {
			continue
		}
		d, err := h.libraryRepo.GetDemo(ctx, *rec.DemoID)
		if err == nil && d.MatchID != nil {
			out[rec.Entry] = d.MatchID
		}
	}
	return out, nil
}

type proImportResponse struct {
	MatchID       uuid.UUID `json:"match_id"`
	DemoID        uuid.UUID `json:"demo_id"`
	Created       bool      `json:"created"`
	StudioPath    string    `json:"studio_path"`
	RadarMayBeOff bool      `json:"radar_may_be_off"`
}

// ImportLibraryProMatch handles POST /api/library/pro-matches/{match}/{entry}/import:
// it stores one pro map's analysis as a library match, or returns the one stored before.
func (h *Handlers) ImportLibraryProMatch(w http.ResponseWriter, r *http.Request) {
	if !h.libraryReady(w) {
		return
	}
	matchID, err := strconv.Atoi(chi.URLParam(r, "match"))
	entry := chi.URLParam(r, "entry")
	if err != nil || matchID <= 0 || !community.ProEntryPattern.MatchString(entry) {
		writeError(w, http.StatusBadRequest, "match must be an HLTV match id and entry a map file of it")
		return
	}
	if h.references.community.Client == nil {
		writeCodedError(w, http.StatusServiceUnavailable, codeCommunityOff, "the community server is disabled")
		return
	}
	res, err := h.importCatalogMap(r.Context(), matchID, entry)
	switch {
	case errors.Is(err, community.ErrNoProMap), errors.Is(err, community.ErrNoProCatalog):
		writeCodedError(w, http.StatusNotFound, codeNoProMap, "the community server has no analysis of this map")
	case errors.Is(err, errProMapPending):
		writeCodedError(w, http.StatusConflict, codeProMapPending, err.Error())
	case errors.Is(err, errProMapBroken):
		log.Printf("httpapi: pro map %d/%s: %v", matchID, entry, err)
		writeCodedError(w, http.StatusBadGateway, codeProMapUnreadable, "El servidor de la comunidad envió un análisis de este mapa que Studio no puede leer.")
	case errors.Is(err, errProDownload):
		writeCodedError(w, http.StatusBadGateway, codeCommunityDown, "the community server did not answer")
	case err != nil:
		internalError(w, "import pro map", err)
	default:
		writeJSON(w, http.StatusOK, res)
	}
}

// importCatalogMap reads the map from the catalog, so every part of it is imported.
func (h *Handlers) importCatalogMap(ctx context.Context, matchID int, entry string) (proImportResponse, error) {
	m, err := h.references.community.Client.ProMatch(ctx, matchID)
	if errors.Is(err, community.ErrNoProMap) {
		return proImportResponse{}, err
	}
	if err != nil {
		return proImportResponse{}, fmt.Errorf("%w: %w", errProDownload, err)
	}
	for _, f := range m.Maps {
		if f.Entry == entry {
			return h.importProMap(ctx, proMapRef{matchID: matchID, file: f, explicit: true})
		}
	}
	return proImportResponse{}, community.ErrNoProMap
}

type proMatchInfo struct {
	library.ProMap
	RadarMayBeOff bool   `json:"radar_may_be_off"`
	OldAnalysis   bool   `json:"old_analysis"`
	HLTVURL       string `json:"hltv_url"`
}

// GetLibraryMatchPro handles GET /api/library/matches/{id}/pro: the HLTV match a
// pro import came from, or 404 for any other match.
func (h *Handlers) GetLibraryMatchPro(w http.ResponseWriter, r *http.Request) {
	m, ok := h.loadLibraryMatch(w, r)
	if !ok {
		return
	}
	rec, err := h.libraryRepo.ProMapForDemo(r.Context(), m.DemoID)
	if err != nil {
		writeLibraryError(w, "get pro map", err)
		return
	}
	writeJSON(w, http.StatusOK, proMatchInfo{ProMap: rec, RadarMayBeOff: library.RadarMayBeOff(rec.Map, rec.AnalysisVersion),
		OldAnalysis: rec.AnalysisVersion < tacticalplan.AnalysisVersion, HLTVURL: community.ProMatchURL(rec.MatchID)})
}

func proStudioPath(matchID uuid.UUID) string { return "/partidas/" + matchID.String() + "?tab=replay" }

// proRivalRank is the rival's HLTV rank on the day of a pro match, as the catalog
// knew it; without one the rival keeps its team profile's rank.
func (h *Handlers) proRivalRank(ctx context.Context, c scoutCandidate) int {
	rec, err := h.libraryRepo.ProMapForDemo(ctx, c.match.DemoID)
	if err != nil {
		return c.scout.OpponentRank
	}
	for _, t := range rec.Teams {
		if t.Rank > 0 && library.NormalizeName(t.Name) == library.NormalizeName(c.scout.Opponent) {
			return t.Rank
		}
	}
	return c.scout.OpponentRank
}
