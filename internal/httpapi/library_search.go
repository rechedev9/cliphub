package httpapi

import (
	"cmp"
	"context"
	"net/http"
	"strconv"
	"strings"

	"github.com/rechedev9/cliphub/internal/library"
)

// Search sizes: results per kind, and how many matches and players are read.
const (
	defaultSearchPerKind = 8
	maxSearchPerKind     = 25
	searchMatchWindow    = library.MaxListLimit
)

// Kinds of a search result.
const (
	searchPlay   = "play"
	searchLineup = "lineup"
	searchCall   = "call"
	searchTeam   = "team"
	searchPlayer = "player"
	searchMatch  = "match"
)

// searchResult is one record a search found, with what a result row shows.
type searchResult struct {
	Kind     string `json:"kind"`
	ID       string `json:"id"`
	Title    string `json:"title"`
	Map      string `json:"map,omitempty"`
	Side     string `json:"side,omitempty"`
	Status   string `json:"status,omitempty"`
	Detail   string `json:"detail,omitempty"`
	TeamName string `json:"team_name,omitempty"`
}

type searchResponse struct {
	Query   string         `json:"query"`
	Map     string         `json:"map,omitempty"`
	Grenade string         `json:"grenade,omitempty"`
	Results []searchResult `json:"results"`
}

// SearchLibrary handles GET /api/library/search?q=: plays, lineups, calls,
// teams, players and matches whose text holds every word of q. A map name in
// q keeps records of that map; a grenade word keeps lineups of that grenade.
func (h *Handlers) SearchLibrary(w http.ResponseWriter, r *http.Request) {
	if !h.libraryReady(w) {
		return
	}
	raw := strings.TrimSpace(r.URL.Query().Get("q"))
	if len([]rune(raw)) > library.MaxSearchRunes {
		writeError(w, http.StatusBadRequest, "q is too long")
		return
	}
	perKind := defaultSearchPerKind
	if v := r.URL.Query().Get("limit"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 1 || n > maxSearchPerKind {
			writeError(w, http.StatusBadRequest, "limit is out of range")
			return
		}
		perKind = n
	}
	q := library.ParseSearch(raw)
	resp := searchResponse{Query: raw, Map: q.Map, Grenade: q.Grenade, Results: []searchResult{}}
	if q.Empty() {
		writeJSON(w, http.StatusOK, resp)
		return
	}
	for _, kind := range []func(context.Context, library.SearchQuery, int) ([]searchResult, error){
		h.searchPlays, h.searchLineups, h.searchCalls, h.searchTeams, h.searchPlayers, h.searchMatches,
	} {
		found, err := kind(r.Context(), q, perKind)
		if err != nil {
			internalError(w, "search library", err)
			return
		}
		resp.Results = append(resp.Results, found...)
	}
	writeJSON(w, http.StatusOK, resp)
}

func (h *Handlers) searchPlays(ctx context.Context, q library.SearchQuery, limit int) ([]searchResult, error) {
	if q.Grenade != "" {
		return nil, nil
	}
	plays, err := h.libraryRepo.ListPlays(ctx, library.PlayFilter{Map: q.Map, Limit: library.MaxPlayList})
	if err != nil {
		return nil, err
	}
	out := []searchResult{}
	for _, p := range plays {
		if len(out) == limit {
			break
		}
		if q.Holds(p.Title, p.Notes, p.TeamName, p.Kind, p.Site, strings.Join(p.Tags, " ")) {
			out = append(out, searchResult{Kind: searchPlay, ID: p.ID.String(), Title: p.Title, Map: p.Map, Side: p.Side,
				Status: p.Status, Detail: p.Kind, TeamName: p.TeamName})
		}
	}
	return out, nil
}

func (h *Handlers) searchLineups(ctx context.Context, q library.SearchQuery, limit int) ([]searchResult, error) {
	lineups, err := h.libraryRepo.ListLineups(ctx, library.LineupFilter{Map: q.Map, Grenade: q.Grenade, Limit: library.MaxLineupList})
	if err != nil {
		return nil, err
	}
	out := []searchResult{}
	for _, l := range lineups {
		if len(out) == limit {
			break
		}
		if q.Holds(l.Title, l.Notes, l.ThrowCallout, l.LandingCallout, strings.Join(l.Tags, " ")) {
			out = append(out, searchResult{Kind: searchLineup, ID: l.ID.String(), Title: l.Title, Map: l.Map, Side: l.Side,
				Status: l.Status, Detail: l.Grenade})
		}
	}
	return out, nil
}

func (h *Handlers) searchCalls(ctx context.Context, q library.SearchQuery, limit int) ([]searchResult, error) {
	if q.Narrowed() {
		return nil, nil
	}
	calls, err := h.libraryRepo.ListCalls(ctx, library.CallFilter{Limit: library.MaxCallList})
	if err != nil {
		return nil, err
	}
	out := []searchResult{}
	for _, c := range calls {
		if len(out) == limit {
			break
		}
		if q.Holds(c.Name, c.Meaning, strings.Join(c.Tags, " ")) {
			out = append(out, searchResult{Kind: searchCall, ID: c.ID.String(), Title: c.Name, Side: c.Side, Status: c.Status, Detail: c.Meaning})
		}
	}
	return out, nil
}

func (h *Handlers) searchTeams(ctx context.Context, q library.SearchQuery, limit int) ([]searchResult, error) {
	if q.Narrowed() {
		return nil, nil
	}
	teams, err := h.libraryRepo.ListTeams(ctx, library.TeamFilter{Limit: library.MaxListLimit})
	if err != nil {
		return nil, err
	}
	out := []searchResult{}
	for _, t := range teams {
		if len(out) == limit {
			break
		}
		if q.Holds(t.Name, strings.Join(t.Aliases, " ")) {
			out = append(out, searchResult{Kind: searchTeam, ID: t.ID.String(), Title: t.Name, Detail: strconv.Itoa(t.MatchCount)})
		}
	}
	return out, nil
}

// searchPlayers asks the repository for nicks holding the first word, then
// keeps the players holding every word. Nicks are stored with their accents,
// so a word typed with one is also looked up as typed.
func (h *Handlers) searchPlayers(ctx context.Context, q library.SearchQuery, limit int) ([]searchResult, error) {
	if q.Narrowed() || len(q.Words) == 0 {
		return nil, nil
	}
	lookups := []string{q.Words[0]}
	if q.Typed[0] != q.Words[0] {
		lookups = append(lookups, q.Typed[0])
	}
	var players []library.Player
	seen := map[string]bool{}
	for _, lookup := range lookups {
		found, err := h.libraryRepo.ListPlayers(ctx, library.PlayerFilter{Query: lookup, Limit: library.MaxListLimit})
		if err != nil {
			return nil, err
		}
		for _, p := range found {
			if !seen[p.SteamID64] {
				seen[p.SteamID64] = true
				players = append(players, p)
			}
		}
	}
	out := []searchResult{}
	for _, p := range players {
		if len(out) == limit {
			break
		}
		if q.Holds(p.Nick, p.SteamID64, p.FaceitNickname) {
			out = append(out, searchResult{Kind: searchPlayer, ID: p.SteamID64, Title: p.Nick, Detail: strconv.Itoa(p.MatchCount)})
		}
	}
	return out, nil
}

// searchMatches reads the newest matches, of the query's map when it names one.
func (h *Handlers) searchMatches(ctx context.Context, q library.SearchQuery, limit int) ([]searchResult, error) {
	if q.Grenade != "" {
		return nil, nil
	}
	matches, err := h.libraryRepo.ListMatches(ctx, library.MatchFilter{Map: q.Map, Limit: searchMatchWindow})
	if err != nil {
		return nil, err
	}
	out := []searchResult{}
	for _, m := range matches {
		if len(out) == limit {
			break
		}
		names := make([]string, 0, 2*len(m.Teams))
		for _, t := range m.Teams {
			names = append(names, t.ClanName, t.TeamName)
		}
		if q.Holds(append(names, m.Map)...) {
			out = append(out, searchResult{Kind: searchMatch, ID: m.ID.String(), Title: matchTitle(m), Map: m.Map,
				Detail: m.PlayedAt.UTC().Format("2006-01-02")})
		}
	}
	return out, nil
}

// matchTitle names a match by its sides, "Vitality vs MOUZ".
func matchTitle(m library.Match) string {
	names := make([]string, 0, len(m.Teams))
	for _, t := range m.Teams {
		names = append(names, cmp.Or(t.TeamName, t.ClanName, t.Key))
	}
	return strings.Join(names, " vs ")
}
