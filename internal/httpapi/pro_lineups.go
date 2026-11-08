package httpapi

import (
	"errors"
	"fmt"
	"net/http"
	"os"

	"github.com/go-chi/chi/v5"

	"github.com/rechedev9/cliphub/internal/community"
	"github.com/rechedev9/cliphub/internal/library"
	"github.com/rechedev9/cliphub/internal/prolineups"
)

const (
	codeNoProLineups = "no_pro_lineups"
	codeNoProClip    = "no_pro_clip"
)

type proLineupMapsResponse struct {
	community.LineupMaps
	// Fresh is false when the server did not answer and this is the last copy Studio kept.
	Fresh bool `json:"fresh"`
}

type proLineupIndexResponse struct {
	prolineups.MapIndex
	Fresh bool `json:"fresh"`
}

func (h *Handlers) proLineupsCache(w http.ResponseWriter) (*community.LineupsCache, bool) {
	c := h.references.community.Lineups
	if c == nil || h.references.community.Client == nil {
		writeCodedError(w, http.StatusServiceUnavailable, codeCommunityOff, "the community server is disabled")
		return nil, false
	}
	return c, true
}

func writeProLineupsError(w http.ResponseWriter, err error) {
	if errors.Is(err, community.ErrNoLineups) {
		writeCodedError(w, http.StatusNotFound, codeNoProLineups, "there are no pro lineups for this map yet")
		return
	}
	writeCodedError(w, http.StatusBadGateway, codeCommunityDown, "the community server did not answer and Studio has no copy yet")
}

// ListProLineupMaps serves GET /api/library/pro-lineups: the maps with pro lineups.
func (h *Handlers) ListProLineupMaps(w http.ResponseWriter, r *http.Request) {
	c, ok := h.proLineupsCache(w)
	if !ok {
		return
	}
	maps, fresh, err := c.Maps(r.Context())
	if err != nil {
		writeProLineupsError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, proLineupMapsResponse{LineupMaps: maps, Fresh: fresh})
}

// GetProLineups serves GET /api/library/pro-lineups/{map}: every lineup and combo of the map.
func (h *Handlers) GetProLineups(w http.ResponseWriter, r *http.Request) {
	c, ok := h.proLineupsCache(w)
	if !ok {
		return
	}
	mapName := chi.URLParam(r, "map")
	if !community.MapPattern.MatchString(mapName) || mapName == community.AllMaps {
		writeError(w, http.StatusBadRequest, "map must be a map name like de_mirage")
		return
	}
	idx, fresh, err := c.Map(r.Context(), mapName)
	if err != nil {
		writeProLineupsError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, proLineupIndexResponse{MapIndex: idx, Fresh: fresh})
}

// SaveProLineup serves POST /api/library/pro-lineups/{map}/lineups/{id}/save: it copies
// a pro lineup into the Libreta so the team can tag, note and practise it.
func (h *Handlers) SaveProLineup(w http.ResponseWriter, r *http.Request) {
	if !h.libraryReady(w) {
		return
	}
	c, ok := h.proLineupsCache(w)
	if !ok {
		return
	}
	mapName := chi.URLParam(r, "map")
	if !community.MapPattern.MatchString(mapName) || mapName == community.AllMaps {
		writeError(w, http.StatusBadRequest, "map must be a map name like de_mirage")
		return
	}
	idx, _, err := c.Map(r.Context(), mapName)
	if err != nil {
		writeProLineupsError(w, err)
		return
	}
	id := chi.URLParam(r, "id")
	for _, l := range idx.Lineups {
		if l.ID != id {
			continue
		}
		lineup, err := library.CleanLineup(libretaLineup(idx.Map, l))
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		lineup.ProLineupID = l.ID
		existing, err := h.savedProLineup(r, lineup)
		if err != nil {
			writeLibraryError(w, "save pro lineup", err)
			return
		}
		if existing != nil {
			writeJSON(w, http.StatusOK, existing)
			return
		}
		if err := h.libraryRepo.CreateLineup(r.Context(), &lineup); err != nil {
			writeLibraryError(w, "save pro lineup", err)
			return
		}
		writeJSON(w, http.StatusCreated, lineup)
		return
	}
	writeCodedError(w, http.StatusNotFound, codeNoProLineups, "no pro lineup with this id on the map")
}

// savedProLineup is the Libreta copy saved from this pro lineup id, so a second save adds nothing.
// The id survives index rebuilds that move the lineup's mean spot and aim a little.
func (h *Handlers) savedProLineup(r *http.Request, want library.Lineup) (*library.Lineup, error) {
	filter := library.LineupFilter{Map: want.Map, Side: want.Side, Grenade: want.Grenade, Limit: library.MaxLineupList}
	for {
		saved, err := h.libraryRepo.ListLineups(r.Context(), filter)
		if err != nil {
			return nil, err
		}
		for i := range saved {
			if saved[i].ProLineupID == want.ProLineupID {
				return &saved[i], nil
			}
		}
		if len(saved) < filter.Limit {
			return nil, nil
		}
		filter.Offset += len(saved)
	}
}

// libretaLineup is a pro lineup as a Libreta lineup; its technique is estimated.
func libretaLineup(mapName string, l prolineups.Lineup) library.Lineup {
	landing := l.Landing
	return library.Lineup{
		Title: l.Title, Map: mapName, Side: string(l.Side), Grenade: string(l.Kind), Status: library.StatusActive, Tags: []string{"pro"},
		Notes:  fmt.Sprintf("Lineup pro: %d lanzamientos en %d mapas, %.0f%% de rondas ganadas. Aire %.1f s.", l.Throws.All, l.Matches.All, 100*l.RoundWin, l.AirTime),
		Origin: l.Origin, Pitch: l.Pitch, Yaw: l.Yaw, Landing: &landing, ThrowCallout: l.ThrowPlace, LandingCallout: l.LandPlace,
		Movement: l.Movement, Jump: l.Jump, TechniqueEstimated: true,
	}
}

// GetProLineupClip serves GET /api/library/pro-lineups/clips/{match}/{file}: a recorded
// throw clip, kept on disk after the first download.
func (h *Handlers) GetProLineupClip(w http.ResponseWriter, r *http.Request) {
	c, ok := h.proLineupsCache(w)
	if !ok {
		return
	}
	name := chi.URLParam(r, "match") + "/" + chi.URLParam(r, "file")
	if !prolineups.ValidClipName(name) {
		writeError(w, http.StatusBadRequest, "clip must be a clip name from a pro lineup")
		return
	}
	path, err := c.Clip(r.Context(), name)
	switch {
	case errors.Is(err, community.ErrNoClip):
		writeCodedError(w, http.StatusNotFound, codeNoProClip, "this throw has no recorded clip")
		return
	case err != nil:
		writeCodedError(w, http.StatusBadGateway, codeCommunityDown, fmt.Sprintf("Studio has no copy of this clip and could not get it from the community server: %v", err))
		return
	}
	f, err := os.Open(path)
	if err != nil {
		internalError(w, "open pro clip", err)
		return
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		internalError(w, "stat pro clip", err)
		return
	}
	w.Header().Set("Content-Type", "video/mp4")
	w.Header().Set("Cache-Control", "private, max-age=604800, immutable")
	http.ServeContent(w, r, chi.URLParam(r, "file"), info.ModTime(), f)
}
