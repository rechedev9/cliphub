package httpapi

import (
	"context"
	"fmt"
	"net/http"

	"github.com/rechedev9/cliphub/internal/community"
)

// proBoard is a pro-scope leaderboard to answer: the response so far, the filters and the row limit.
type proBoard struct {
	ctx     context.Context
	out     leaderboardResponse
	filters leaderboardFilters
	limit   int
}

// writeProLeaderboard answers a pro-scope leaderboard from the community
// server, which reads the per-player lines of its pro maps. The rows carry a
// team and no rounds to open: those maps are not in the library.
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
	writeJSON(w, http.StatusOK, out)
}
