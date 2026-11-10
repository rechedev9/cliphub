package httpapi

import (
	"context"
	"log"
	"net/http"
	"strconv"
	"sync"

	"github.com/google/uuid"

	"github.com/rechedev9/cliphub/internal/library"
	"github.com/rechedev9/cliphub/internal/patterndetect"
	"github.com/rechedev9/cliphub/internal/tacticalplan"
)

// patternDetection runs a saved pattern over the library after it is created or edited, off the request.
type patternDetection struct {
	mu    sync.Mutex
	queue chan uuid.UUID
}

func (h *Handlers) patternDetector() *patterndetect.Detector {
	return patterndetect.New(h.libraryRepo, h.storage)
}

// StartPatternDetection starts the background pass that detects a created or edited pattern
// over the library. Without it, a pattern's rows are filled the first time something reads them.
func (h *Handlers) StartPatternDetection(ctx context.Context) {
	if h.libraryRepo == nil {
		return
	}
	d := &h.references.detection
	d.mu.Lock()
	if d.queue != nil {
		d.mu.Unlock()
		return
	}
	queue := make(chan uuid.UUID, 64)
	d.queue = queue
	d.mu.Unlock()
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case id := <-queue:
				p, err := h.libraryRepo.GetPattern(ctx, id)
				if err != nil {
					continue
				}
				if err := h.patternDetector().DetectPattern(ctx, p); err != nil {
					log.Printf("httpapi: detect pattern %s: %v", id, err)
				}
			}
		}
	}()
}

// queuePatternDetection asks the background pass for a pattern; a full queue drops it, since
// the next read fills the rows anyway.
func (h *Handlers) queuePatternDetection(id uuid.UUID) {
	d := &h.references.detection
	d.mu.Lock()
	queue := d.queue
	d.mu.Unlock()
	if queue == nil {
		return
	}
	select {
	case queue <- id:
	default:
	}
}

// matchPatternHit is a saved pattern that holds in some rounds of a match, from one team's side.
type matchPatternHit struct {
	Pattern library.Pattern             `json:"pattern"`
	TeamKey string                      `json:"team_key"`
	Team    string                      `json:"team,omitempty"`
	Rounds  []tacticalplan.PatternRound `json:"rounds"`
}

type matchPatternsResponse struct {
	MatchID uuid.UUID `json:"match_id"`
	Map     string    `json:"map"`
	// Total is how many saved patterns the map has, matched or not.
	Total    int               `json:"total"`
	Patterns []matchPatternHit `json:"patterns"`
}

// GetLibraryMatchPatterns handles GET /api/library/matches/{id}/patterns?round=: the saved
// patterns of the match's map that hold in its rounds (one round with round), as the Replay lists them.
func (h *Handlers) GetLibraryMatchPatterns(w http.ResponseWriter, r *http.Request) {
	if !h.libraryReady(w) {
		return
	}
	id, ok := libraryUUIDParam(w, r, "id")
	if !ok {
		return
	}
	round := 0
	if raw := r.URL.Query().Get("round"); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || n < 1 {
			writeError(w, http.StatusBadRequest, "round must be a round number from 1")
			return
		}
		round = n
	}
	m, err := h.libraryRepo.GetMatch(r.Context(), id)
	if err != nil {
		writeLibraryError(w, "get match", err)
		return
	}
	hits, total, err := h.patternDetector().MatchHits(r.Context(), m)
	if err != nil {
		internalError(w, "match patterns", err)
		return
	}
	resp := matchPatternsResponse{MatchID: m.ID, Map: m.Map, Total: total, Patterns: []matchPatternHit{}}
	for _, hit := range hits {
		for _, row := range hit.Rows {
			rounds := []tacticalplan.PatternRound{}
			for _, pr := range row.Read.Match.Matched {
				if round == 0 || pr.Round == round {
					rounds = append(rounds, pr)
				}
			}
			if len(rounds) > 0 {
				resp.Patterns = append(resp.Patterns, matchPatternHit{Pattern: hit.Pattern, TeamKey: row.TeamKey, Team: row.Read.Match.Team, Rounds: rounds})
			}
		}
	}
	writeJSON(w, http.StatusOK, resp)
}
