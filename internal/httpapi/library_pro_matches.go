package httpapi

import (
	"context"
	"errors"
	"log"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/rechedev9/cliphub/internal/community"
	"github.com/rechedev9/cliphub/internal/library"
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
}

type proMatchView struct {
	ID       int           `json:"id"`
	Event    string        `json:"event"`
	PlayedAt time.Time     `json:"played_at"`
	Teams    []proTeamView `json:"teams"`
	Maps     []proMapView  `json:"maps"`
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
		v := proMatchView{ID: m.ID, Event: m.Event, PlayedAt: m.PlayedAt, Teams: []proTeamView{}, Maps: []proMapView{}}
		for _, t := range m.Teams {
			v.Teams = append(v.Teams, proTeamView{ProTeam: t, Followed: followed[t.ID]})
		}
		for _, f := range m.Maps {
			v.Maps = append(v.Maps, proMapView{ProMapFile: f, LibraryMatchID: imported[f.Entry], RadarMayBeOff: library.RadarMayBeOff(f.Map, f.AnalysisVersion)})
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
	res, err := h.importProMap(r.Context(), proMapRef{matchID: matchID, entry: entry, explicit: true})
	switch {
	case errors.Is(err, community.ErrNoProMap), errors.Is(err, community.ErrNoProCatalog):
		writeCodedError(w, http.StatusNotFound, codeNoProMap, "the community server has no analysis of this map")
	case errors.Is(err, errProMapPending):
		writeCodedError(w, http.StatusConflict, codeProMapPending, err.Error())
	case errors.Is(err, errProMapBroken):
		writeCodedError(w, http.StatusBadGateway, codeCommunityDown, err.Error())
	case errors.Is(err, errProDownload):
		writeCodedError(w, http.StatusBadGateway, codeCommunityDown, "the community server did not answer")
	case err != nil:
		internalError(w, "import pro map", err)
	default:
		writeJSON(w, http.StatusOK, res)
	}
}

type proMatchInfo struct {
	library.ProMap
	RadarMayBeOff bool `json:"radar_may_be_off"`
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
	writeJSON(w, http.StatusOK, proMatchInfo{ProMap: rec, RadarMayBeOff: library.RadarMayBeOff(rec.Map, rec.AnalysisVersion)})
}

func proStudioPath(matchID uuid.UUID) string { return "/partidas/" + matchID.String() + "?tab=replay" }
