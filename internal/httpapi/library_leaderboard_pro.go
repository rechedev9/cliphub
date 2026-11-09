package httpapi

import (
	"context"
	"fmt"
	"net/http"

	"github.com/rechedev9/cliphub/internal/community"
)

// writeProLeaderboard answers a pro-scope leaderboard that names a side, a
// role, a period or a rival level from the community server, which reads the
// per-player lines of its HLTV maps. The rows carry a team and no rounds to
// open: those maps are not in the library.
func (h *Handlers) writeProLeaderboard(ctx context.Context, w http.ResponseWriter, out leaderboardResponse, f leaderboardFilters, limit int) {
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
	out.Description = fmt.Sprintf("%d mapas profesionales de HLTV del servidor de ClipHub.", board.Maps)
	writeJSON(w, http.StatusOK, out)
}
