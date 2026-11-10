package httpapi

import (
	"context"
	"fmt"
	"log"
	"net/http"

	"github.com/rechedev9/cliphub/internal/community"
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

// attachLibraryRounds gives the pro rows without rounds the library's rounds of
// each player on the board's map, side, role and position, newest first, so
// "Rondas en el Replay" opens them with the player in focus.
func (h *Handlers) attachLibraryRounds(ctx context.Context, rows []tacticalplan.LeaderboardRow, in leaderboardRead) {
	if len(rows) == 0 {
		return
	}
	f := in.filters
	lib := leaderboardRead{query: in.query, filters: leaderboardFilters{side: f.side, role: f.role, mapRole: f.mapRole}}
	fold, err := h.leaderboardSample(ctx, lib)
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
