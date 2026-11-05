package httpapi

import (
	"errors"
	"io"
	"log"
	"net/http"
	"net/url"

	"github.com/google/uuid"

	"github.com/rechedev9/cliphub/internal/artifacts"
	"github.com/rechedev9/cliphub/internal/library"
	"github.com/rechedev9/cliphub/internal/tacticalplan"
)

// positionsResponse is where a player stands early in rounds, with the matches it skipped.
type positionsResponse struct {
	Skipped []scoutSkipped `json:"skipped"`
	tacticalplan.PlayerPositionsReport
}

// playerPositionsQuery reads the map and date range of a player's positions.
func playerPositionsQuery(q url.Values) (library.MatchFilter, error) {
	f := library.MatchFilter{Limit: library.MaxReportMatches}
	var err error
	if m := q.Get("map"); m != "" {
		if !mapNamePattern.MatchString(m) {
			return f, errors.New("map must match ^[a-z0-9_]+$")
		}
		f.Map = m
	}
	if f.From, err = optionalTimeQuery(q, "from"); err != nil {
		return f, err
	}
	f.To, err = optionalTimeQuery(q, "to")
	return f, err
}

// GetLibraryPlayerPositions handles GET /api/library/players/{steamid}/positions:
// per map and side, the callouts the player holds in the first seconds of a
// round and their AWP share, over their most recent matches.
func (h *Handlers) GetLibraryPlayerPositions(w http.ResponseWriter, r *http.Request) {
	steamID, ok := h.libraryPlayerParam(w, r)
	if !ok {
		return
	}
	filter, err := playerPositionsQuery(r.URL.Query())
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
	matches := make([]tacticalplan.PositionMatch, 0, len(listed))
	for _, m := range listed {
		doc, err := h.readLibraryDocument(m.DemoID)
		if err != nil {
			log.Printf("httpapi: positions of %s skip match %s: %v", steamID, m.ID, err)
			skipped = append(skipped, scoutSkipped{MatchID: m.ID, Reason: scoutSkipNotAnalyzed})
			continue
		}
		matches = append(matches, tacticalplan.PositionMatch{
			MatchID: m.ID, PlayedAt: m.PlayedAt, Document: doc, Opening: h.readOpeningFrames(m.DemoID, doc),
		})
	}
	writeJSON(w, http.StatusOK, positionsResponse{Skipped: skipped, PlayerPositionsReport: tacticalplan.PlayerPositions(steamID, matches)})
}

// readOpeningFrames reads the opening window of every round from a match's
// position blob. A missing or broken blob leaves the rounds unsampled.
func (h *Handlers) readOpeningFrames(demoID uuid.UUID, doc tacticalplan.Document) map[int][]tacticalplan.Frame {
	rc, err := h.storage.Open(artifacts.LibraryTacticalPositionsKey(demoID))
	if err != nil {
		return nil
	}
	defer rc.Close()
	blob, err := io.ReadAll(rc)
	if err != nil {
		log.Printf("httpapi: read positions of demo %s: %v", demoID, err)
		return nil
	}
	frames, err := tacticalplan.OpeningFrames(blob, doc)
	if err != nil {
		log.Printf("httpapi: opening frames of demo %s: %v", demoID, err)
		return nil
	}
	return frames
}
