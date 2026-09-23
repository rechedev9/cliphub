package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/rechedev9/cliphub/internal/artifacts"
	"github.com/rechedev9/cliphub/internal/faceit"
	"github.com/rechedev9/cliphub/internal/parser"
	"github.com/rechedev9/cliphub/internal/storage"
)

const (
	notFaceitDemo       = "not_faceit_demo"
	faceitMatchMismatch = "faceit_match_mismatch"
)

// GetFaceitScoreboard handles GET /api/jobs/{id}/faceit-scoreboard: the FACEIT
// room scoreboard of a demo downloaded from FACEIT, so the POV picker shows the
// same numbers as the room instead of the demo's own recount. The match id
// comes from the file name FACEIT serves; the response is checked against the
// parsed roster so a renamed file can never attach another match's stats.
func (h *Handlers) GetFaceitScoreboard(w http.ResponseWriter, r *http.Request) {
	j, ok := h.loadJobMeta(w, r)
	if !ok {
		return
	}
	matchID, ok := faceit.MatchIDFromDemoFileName(j.DemoFileName)
	if !ok {
		writeCodedError(w, http.StatusNotFound, notFaceitDemo, "demo file name carries no FACEIT match id")
		return
	}
	if !h.faceitReady(w) {
		return
	}
	rc, err := h.storage.Open(artifacts.RosterKey(j.ID))
	if err != nil {
		if storage.IsNotExist(err) {
			writeError(w, http.StatusConflict, "roster not ready")
			return
		}
		internalError(w, "open roster artifact", err)
		return
	}
	var roster parser.RosterResult
	err = json.NewDecoder(rc).Decode(&roster)
	_ = rc.Close()
	if err != nil {
		internalError(w, "decode roster artifact", err)
		return
	}

	board, ok := h.faceitCache.lookupScoreboard(matchID, roster.Match.Map)
	if !ok {
		ctx, cancel := context.WithTimeout(r.Context(), 20*time.Second)
		defer cancel()
		board, err = h.faceit.MatchScoreboard(ctx, matchID, roster.Match.Map)
		if errors.Is(err, faceit.ErrMatchNotFound) {
			writeCodedError(w, http.StatusNotFound, notFaceitDemo, err.Error())
			return
		}
		if err != nil {
			writeFaceitError(w, err)
			return
		}
		h.faceitCache.storeScoreboard(matchID, roster.Match.Map, board)
	}
	board, ok = mergeDemoIntoScoreboard(board, roster)
	if !ok {
		writeCodedError(w, http.StatusNotFound, faceitMatchMismatch, "FACEIT match roster does not match the demo")
		return
	}
	for _, team := range board.Teams {
		for _, p := range team.Players {
			if p.Avatar != "" {
				h.faceitCache.storeAvatar(p.PlayerID, p.Avatar)
			}
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"scoreboard": board})
}

// mergeDemoIntoScoreboard verifies that the FACEIT match is the demo's match
// and applies the two corrections that make the Data API numbers read like
// the room: the room's DPR is the health-capped damage per round the parser
// computes (the Data API "ADR" counts overkill damage and sits up to a point
// above it), and the room's A column leaves out flash assists, which the Data
// API counts. Rosters scanned before flash assists were recorded keep the
// Data API assists.
func mergeDemoIntoScoreboard(board faceit.Scoreboard, roster parser.RosterResult) (faceit.Scoreboard, bool) {
	demo := make(map[string]parser.PlayerStat, len(roster.Players))
	for _, p := range roster.Players {
		demo[p.SteamID64] = p
	}
	total, matched := 0, 0
	teams := make([]faceit.ScoreboardTeam, len(board.Teams))
	for ti, team := range board.Teams {
		team.Players = append([]faceit.ScoreboardPlayer(nil), team.Players...)
		for pi := range team.Players {
			player := &team.Players[pi]
			total++
			if stat, ok := demo[player.SteamID64]; ok && player.SteamID64 != "" {
				matched++
				if stat.Rounds > 0 {
					player.ADR = stat.ADR
				}
				player.Assists = max(0, player.Assists-stat.FlashAssists)
			}
		}
		teams[ti] = team
	}
	board.Teams = teams
	// A substitute or a disconnect can leave a player out of either side;
	// anything beyond that is a different match. The majority floor keeps a
	// small roster (a 1v1 or 2v2) from passing with no player in common.
	if matched < total-2 || 2*matched <= total {
		return faceit.Scoreboard{}, false
	}
	return board, true
}

type cachedFaceitScoreboard struct {
	board faceit.Scoreboard
	until time.Time
}

func (c *faceitResponseCache) lookupScoreboard(matchID, mapName string) (faceit.Scoreboard, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	entry, ok := c.scoreboards[matchID+"|"+mapName]
	if !ok || !c.clock().Before(entry.until) {
		return faceit.Scoreboard{}, false
	}
	return entry.board, true
}

func (c *faceitResponseCache) storeScoreboard(matchID, mapName string, board faceit.Scoreboard) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.scoreboards == nil {
		c.scoreboards = map[string]cachedFaceitScoreboard{}
	}
	c.scoreboards[matchID+"|"+mapName] = cachedFaceitScoreboard{board: board, until: c.clock().Add(faceitCacheTTL)}
}
