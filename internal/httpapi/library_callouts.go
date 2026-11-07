package httpapi

import (
	"net/http"
	"net/url"
	"sort"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/rechedev9/cliphub/internal/library"
	"github.com/rechedev9/cliphub/internal/radarmap"
	"github.com/rechedev9/cliphub/internal/tacticalplan"
)

// calloutReferenceMatches is how many of the map's newest matches are read for its geometry.
const calloutReferenceMatches = 10

// mapCallout is a CS2 place of a map with the team's own name, if it has one.
type mapCallout struct {
	Token  string      `json:"token"`
	Name   string      `json:"name,omitempty"`
	Level  string      `json:"level,omitempty"`
	Center *[2]float64 `json:"center,omitempty"`
}

type mapCalloutsResponse struct {
	Map              string                    `json:"map"`
	ReferenceMatchID *uuid.UUID                `json:"reference_match_id,omitempty"`
	Geometry         *tacticalplan.MapGeometry `json:"geometry,omitempty"`
	Callouts         []mapCallout              `json:"callouts"`
}

func mapParam(w http.ResponseWriter, r *http.Request) (string, bool) {
	mapName := chi.URLParam(r, "map")
	if !mapNamePattern.MatchString(mapName) {
		writeError(w, http.StatusBadRequest, "map must match ^[a-z0-9_]+$")
		return "", false
	}
	return mapName, true
}

// GetLibraryMapCallouts handles GET /api/library/maps/{map}/callouts: the
// places of the map's reference geometry and the team's names for them.
// geometry=true also returns the geometry to draw the map.
func (h *Handlers) GetLibraryMapCallouts(w http.ResponseWriter, r *http.Request) {
	if !h.libraryReady(w) {
		return
	}
	mapName, ok := mapParam(w, r)
	if !ok {
		return
	}
	names, err := h.libraryRepo.ListCalloutNames(r.Context(), mapName)
	if err != nil {
		internalError(w, "list callout names", err)
		return
	}
	matches, err := h.libraryRepo.ListMatches(r.Context(), library.MatchFilter{Map: mapName, Limit: calloutReferenceMatches})
	if err != nil {
		internalError(w, "list map matches", err)
		return
	}
	resp := mapCalloutsResponse{Map: mapName}
	if id, geometry := h.mapGeometry(matches); geometry != nil {
		resp.ReferenceMatchID = &id
		resp.Callouts = mergeCallouts(geometry.Callouts, names)
		if r.URL.Query().Get("geometry") == "true" {
			resp.Geometry = geometry
		}
	} else {
		resp.Callouts = mergeCallouts(nil, names)
	}
	writeJSON(w, http.StatusOK, resp)
}

// mapGeometry is the geometry of the newest match calibrated from the map's
// overview, else of the newest one with callouts; nil when none is analysed.
func (h *Handlers) mapGeometry(matches []library.Match) (uuid.UUID, *tacticalplan.MapGeometry) {
	var fallbackID uuid.UUID
	var fallback *tacticalplan.MapGeometry
	for _, m := range matches {
		doc, err := h.readLibraryDocument(m.DemoID)
		if err != nil || len(doc.Geometry.Callouts) == 0 {
			continue
		}
		geometry := doc.Geometry
		if geometry.Calibration.Source == radarmap.SourceOverview {
			return m.ID, &geometry
		}
		if fallback == nil {
			fallbackID, fallback = m.ID, &geometry
		}
	}
	return fallbackID, fallback
}

// mergeCallouts keeps each place once, at its most sampled level, and adds
// the named places the geometry does not have, sorted by token.
func mergeCallouts(callouts []tacticalplan.Callout, names []library.CalloutName) []mapCallout {
	byToken := map[string]mapCallout{}
	samples := map[string]int{}
	for _, c := range callouts {
		if c.Name == "" || (samples[c.Name] >= c.Samples && byToken[c.Name].Token != "") {
			continue
		}
		center := c.Center
		byToken[c.Name], samples[c.Name] = mapCallout{Token: c.Name, Level: c.Level, Center: &center}, c.Samples
	}
	for _, n := range names {
		entry := byToken[n.Token]
		entry.Token, entry.Name = n.Token, n.Name
		byToken[n.Token] = entry
	}
	out := make([]mapCallout, 0, len(byToken))
	for _, c := range byToken {
		out = append(out, c)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Token < out[j].Token })
	return out
}

func calloutToken(w http.ResponseWriter, r *http.Request) (string, bool) {
	token, err := url.PathUnescape(chi.URLParam(r, "token"))
	if err != nil || !library.ValidCalloutToken(token) {
		writeError(w, http.StatusBadRequest, "token must be a CS2 place name of at most 64 characters")
		return "", false
	}
	return token, true
}

// PutLibraryMapCallout handles PUT /api/library/maps/{map}/callouts/{token}: the team's name for the place.
func (h *Handlers) PutLibraryMapCallout(w http.ResponseWriter, r *http.Request) {
	if !h.libraryReady(w) {
		return
	}
	mapName, ok := mapParam(w, r)
	if !ok {
		return
	}
	token, ok := calloutToken(w, r)
	if !ok {
		return
	}
	var req struct {
		Name string `json:"name"`
	}
	if err := decodeSingleJSONBody(w, r, &req, true); err != nil {
		writeError(w, http.StatusBadRequest, "invalid callout JSON")
		return
	}
	c, err := library.CleanCalloutName(library.CalloutName{Map: mapName, Token: token, Name: req.Name})
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := h.libraryRepo.SetCalloutName(r.Context(), &c); err != nil {
		internalError(w, "set callout name", err)
		return
	}
	writeJSON(w, http.StatusOK, c)
}

// DeleteLibraryMapCallout handles DELETE /api/library/maps/{map}/callouts/{token}: back to CS2's name.
func (h *Handlers) DeleteLibraryMapCallout(w http.ResponseWriter, r *http.Request) {
	if !h.libraryReady(w) {
		return
	}
	mapName, ok := mapParam(w, r)
	if !ok {
		return
	}
	token, ok := calloutToken(w, r)
	if !ok {
		return
	}
	if err := h.libraryRepo.DeleteCalloutName(r.Context(), mapName, token); err != nil {
		writeLibraryError(w, "delete callout name", err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
