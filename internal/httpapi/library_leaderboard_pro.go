package httpapi

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/google/uuid"

	"github.com/rechedev9/cliphub/internal/community"
	"github.com/rechedev9/cliphub/internal/library"
	"github.com/rechedev9/cliphub/internal/tacticalplan"
)

// proBoard is a pro-scope leaderboard to answer: the response so far, the filters and the row limit.
type proBoard struct {
	ctx     context.Context
	out     leaderboardResponse
	filters leaderboardFilters
	limit   int
}

// writeProLeaderboard answers a pro-scope leaderboard from the community
// server, which reads the per-player lines of its pro maps. Those maps are not
// in the library, so a row's rounds come from the library's own demos of the player.
func (h *Handlers) writeProLeaderboard(w http.ResponseWriter, b proBoard) {
	ctx, out, f, limit := b.ctx, b.out, b.filters, b.limit
	client := h.references.community.Client
	if client == nil {
		writeCodedError(w, http.StatusServiceUnavailable, codeCommunityOff, "the community server is disabled")
		return
	}
	board, err := client.ProPlayers(ctx, community.ProPlayersQuery{
		Map: out.Map, Side: string(f.side), Role: f.role, MapRole: f.mapRole, Event: f.event, Days: f.days, Top: f.top, TeamTop: f.teamTop,
		Metric: out.Metric, MinRounds: out.MinRounds, Limit: limit,
	})
	if err != nil {
		writeCodedError(w, http.StatusBadGateway, codeCommunityDown, fmt.Sprintf("the community server could not rank the pro players: %v", err))
		return
	}
	out.Source = referenceSourceServer
	out.UpdatedAt = board.BuiltAt
	out.Matches, out.Rounds, out.Players = board.Maps, board.Rounds, board.Players
	out.Events, out.MapRoles = board.Events, board.MapRoles
	if board.Maps > 0 {
		out.Description = fmt.Sprintf("%d mapas profesionales del servidor de ClipHub.", board.Maps)
	}
	h.attachLibraryRounds(ctx, out.Players, leaderboardRead{query: baselineQuery{scope: baselineScopeLibrary, mapName: out.Map}, filters: f})
	writeJSON(w, http.StatusOK, out)
}

// libraryRoundsDays is the window of the library rounds a pro row opens, the library board's own.
const libraryRoundsDays = 90

// attachLibraryRounds gives the pro rows without rounds the library's rounds of
// each player on the board's map, side, role and position over the last
// libraryRoundsDays, newest first, so "Ver sus rondas" opens them with the player in focus.
// It reads only the matches of the rows' players, and roles only when the board filters on one.
func (h *Handlers) attachLibraryRounds(ctx context.Context, rows []tacticalplan.LeaderboardRow, in leaderboardRead) {
	f := in.filters
	from := time.Now().AddDate(0, 0, -libraryRoundsDays)
	seen := map[uuid.UUID]bool{}
	var matches []library.Match
	for _, row := range rows {
		if len(row.Examples) > 0 || row.SteamID64 == "" {
			continue
		}
		listed, err := h.allLibraryMatches(ctx, library.MatchFilter{Map: in.query.mapName, SteamID: row.SteamID64, From: &from})
		if err != nil {
			log.Printf("httpapi: pro leaderboard without library rounds: %v", err)
			return
		}
		for _, m := range listed {
			if !seen[m.ID] {
				seen[m.ID] = true
				matches = append(matches, m)
			}
		}
	}
	if len(matches) == 0 {
		return
	}
	fold, err := h.foldLeaderboard(ctx, foldInput{matches: matches, side: f.side, withRoles: f.role != "" || f.mapRole != ""})
	if err != nil {
		log.Printf("httpapi: pro leaderboard without library rounds: %v", err)
		return
	}
	examples := fold.sample.PlayerExamples(tacticalplan.LeaderboardOptions{Role: f.role, MapRole: f.mapRole})
	for i := range rows {
		if len(rows[i].Examples) == 0 {
			rows[i].Examples = examples[rows[i].SteamID64]
		}
	}
}
