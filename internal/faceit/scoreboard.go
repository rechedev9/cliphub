package faceit

import (
	"context"
	"errors"
	"fmt"
	"math"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"sync"
)

// ErrMatchNotFound reports a match id the Data API does not know, or a match
// without statistics for the requested map.
var ErrMatchNotFound = errors.New("FACEIT match not found")

// FACEIT names its demo downloads after the match id plus a map suffix, e.g.
// "1-2009a7d4-6c68-46e1-aebc-46629bd92649-1-1.dem.zst".
var demoFileMatchID = regexp.MustCompile(`(?i)^(1-[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12})(?:-\d+)*\.dem\b`)

// MatchIDFromDemoFileName extracts the FACEIT match id from a demo file name
// as FACEIT serves it. A renamed file yields false.
func MatchIDFromDemoFileName(name string) (string, bool) {
	m := demoFileMatchID.FindStringSubmatch(strings.TrimSpace(name))
	if m == nil {
		return "", false
	}
	return strings.ToLower(m[1]), true
}

// Scoreboard is one map of a FACEIT match exactly as the Data API reports it.
type Scoreboard struct {
	MatchID string           `json:"match_id"`
	RoomURL string           `json:"room_url"`
	Map     string           `json:"map"`
	Rounds  int              `json:"rounds"`
	Teams   []ScoreboardTeam `json:"teams"`
}

type ScoreboardTeam struct {
	Name       string `json:"name"`
	Score      int    `json:"score"`
	FirstHalf  int    `json:"first_half"`
	SecondHalf int    `json:"second_half"`
	Overtime   int    `json:"overtime"`
	Won        bool   `json:"won"`
	// AverageELO is the mean current ELO of the players whose ELO resolved.
	AverageELO *int               `json:"average_elo,omitempty"`
	Players    []ScoreboardPlayer `json:"players"`
}

type ScoreboardPlayer struct {
	PlayerID   string `json:"player_id"`
	Nickname   string `json:"nickname"`
	SteamID64  string `json:"steam_id64,omitempty"`
	Avatar     string `json:"avatar,omitempty"`
	SkillLevel int    `json:"skill_level,omitempty"`
	// ELO is the player's current ELO, as the FACEIT room shows it.
	ELO       *int    `json:"elo,omitempty"`
	Kills     int     `json:"kills"`
	Deaths    int     `json:"deaths"`
	Assists   int     `json:"assists"`
	Headshots int     `json:"headshots"`
	HSPct     float64 `json:"hs_pct"`
	ADR       float64 `json:"adr"`
	KD        float64 `json:"kd"`
	KR        float64 `json:"kr"`
	MVPs      int     `json:"mvps"`
	Double    int     `json:"double_kills"`
	Triple    int     `json:"triple_kills"`
	Quadro    int     `json:"quadro_kills"`
	Penta     int     `json:"penta_kills"`
}

type apiMatchStatsResponse struct {
	Rounds []struct {
		RoundStats map[string]statValue `json:"round_stats"`
		Teams      []struct {
			TeamID    string               `json:"team_id"`
			TeamStats map[string]statValue `json:"team_stats"`
			Players   []struct {
				PlayerID    string               `json:"player_id"`
				Nickname    string               `json:"nickname"`
				PlayerStats map[string]statValue `json:"player_stats"`
			} `json:"players"`
		} `json:"teams"`
	} `json:"rounds"`
}

type apiMatchRoster struct {
	Teams map[string]struct {
		Roster []struct {
			PlayerID       string `json:"player_id"`
			Avatar         string `json:"avatar"`
			GamePlayerID   string `json:"game_player_id"`
			GameSkillLevel int    `json:"game_skill_level"`
		} `json:"roster"`
	} `json:"teams"`
}

type rosterEntry struct {
	steamID    string
	avatar     string
	skillLevel int
}

// MatchScoreboard returns the FACEIT scoreboard of matchID for mapName (a
// de_* map name; empty accepts a single-map match). Statistics and roster are
// required; a player's current ELO is best effort and left unset on failure.
func (c *Client) MatchScoreboard(ctx context.Context, matchID, mapName string) (Scoreboard, error) {
	if !validIdentifier(matchID) {
		return Scoreboard{}, ErrMatchNotFound
	}
	endpoint := "/matches/" + url.PathEscape(matchID)
	var (
		wg                  sync.WaitGroup
		stats               apiMatchStatsResponse
		details             apiMatchRoster
		statsErr, detailErr error
	)
	wg.Add(2)
	go func() {
		defer wg.Done()
		statsErr = c.getJSON(ctx, endpoint+"/stats", nil, &stats)
	}()
	go func() {
		defer wg.Done()
		detailErr = c.getJSON(ctx, endpoint, nil, &details)
	}()
	wg.Wait()
	for _, err := range []error{statsErr, detailErr} {
		var apiErr *APIError
		if errors.As(err, &apiErr) && apiErr.StatusCode == http.StatusNotFound {
			return Scoreboard{}, ErrMatchNotFound
		}
		if err != nil {
			return Scoreboard{}, err
		}
	}

	roster := make(map[string]rosterEntry)
	for _, team := range details.Teams {
		for _, p := range team.Roster {
			roster[p.PlayerID] = rosterEntry{steamID: p.GamePlayerID, avatar: cleanAvatarURL(p.Avatar), skillLevel: p.GameSkillLevel}
		}
	}

	board, err := buildScoreboard(matchID, mapName, stats, roster)
	if err != nil {
		return Scoreboard{}, err
	}
	c.fillCurrentELO(ctx, &board)
	return board, nil
}

func buildScoreboard(matchID, mapName string, stats apiMatchStatsResponse, roster map[string]rosterEntry) (Scoreboard, error) {
	round := -1
	for i, r := range stats.Rounds {
		if (mapName == "" && len(stats.Rounds) == 1) || strings.EqualFold(r.RoundStats["Map"].string(), mapName) {
			round = i
			break
		}
	}
	if round < 0 {
		return Scoreboard{}, fmt.Errorf("%w: no statistics for map %q", ErrMatchNotFound, mapName)
	}
	r := stats.Rounds[round]
	board := Scoreboard{
		MatchID: matchID,
		RoomURL: canonicalRoomURL(matchID),
		Map:     r.RoundStats["Map"].string(),
		Rounds:  r.RoundStats["Rounds"].int(),
		Teams:   make([]ScoreboardTeam, 0, len(r.Teams)),
	}
	for _, t := range r.Teams {
		team := ScoreboardTeam{
			Name:       t.TeamStats["Team"].string(),
			Score:      t.TeamStats["Final Score"].int(),
			FirstHalf:  t.TeamStats["First Half Score"].int(),
			SecondHalf: t.TeamStats["Second Half Score"].int(),
			Overtime:   t.TeamStats["Overtime score"].int(),
			Won:        t.TeamStats["Team Win"].int() == 1,
			Players:    make([]ScoreboardPlayer, 0, len(t.Players)),
		}
		for _, p := range t.Players {
			s := p.PlayerStats
			entry := roster[p.PlayerID]
			kills, deaths, headshots := s["Kills"].int(), s["Deaths"].int(), s["Headshots"].int()
			// The room derives its ratios from the counts with more precision
			// than the rounded strings the Data API returns ("Headshots %": "79"
			// is 78.6 % in the room).
			hsPct := s["Headshots %"].float()
			if kills > 0 {
				hsPct = roundTo(100*float64(headshots)/float64(kills), 1)
			}
			kd := s["K/D Ratio"].float()
			if deaths > 0 {
				kd = roundTo(float64(kills)/float64(deaths), 2)
			}
			kr := s["K/R Ratio"].float()
			if board.Rounds > 0 {
				kr = roundTo(float64(kills)/float64(board.Rounds), 2)
			}
			team.Players = append(team.Players, ScoreboardPlayer{
				PlayerID:   p.PlayerID,
				Nickname:   p.Nickname,
				SteamID64:  entry.steamID,
				Avatar:     entry.avatar,
				SkillLevel: entry.skillLevel,
				Kills:      kills,
				Deaths:     deaths,
				Assists:    s["Assists"].int(),
				Headshots:  headshots,
				HSPct:      hsPct,
				ADR:        s["ADR"].float(),
				KD:         kd,
				KR:         kr,
				MVPs:       s["MVPs"].int(),
				Double:     s["Double Kills"].int(),
				Triple:     s["Triple Kills"].int(),
				Quadro:     s["Quadro Kills"].int(),
				Penta:      s["Penta Kills"].int(),
			})
		}
		board.Teams = append(board.Teams, team)
	}
	// FACEIT lists the winner first.
	if len(board.Teams) == 2 && !board.Teams[0].Won && board.Teams[1].Won {
		board.Teams[0], board.Teams[1] = board.Teams[1], board.Teams[0]
	}
	return board, nil
}

// fillCurrentELO resolves each player's current ELO, which is what the FACEIT
// room shows next to the level, and the team averages derived from it.
func (c *Client) fillCurrentELO(ctx context.Context, board *Scoreboard) {
	var wg sync.WaitGroup
	for ti := range board.Teams {
		for pi := range board.Teams[ti].Players {
			player := &board.Teams[ti].Players[pi]
			wg.Add(1)
			go func() {
				defer wg.Done()
				var raw apiPlayer
				if err := c.getJSON(ctx, "/players/"+url.PathEscape(player.PlayerID), nil, &raw); err != nil {
					return
				}
				if game, ok := raw.Games["cs2"]; ok && game.FaceitELO > 0 {
					elo := game.FaceitELO
					player.ELO = &elo
					// The room shows the current level beside the current ELO.
					if game.SkillLevel > 0 {
						player.SkillLevel = game.SkillLevel
					}
				}
			}()
		}
	}
	wg.Wait()
	for ti := range board.Teams {
		team := &board.Teams[ti]
		sum, n := 0, 0
		for _, p := range team.Players {
			if p.ELO != nil {
				sum += *p.ELO
				n++
			}
		}
		if n > 0 {
			avg := int(math.Round(float64(sum) / float64(n)))
			team.AverageELO = &avg
		}
	}
}

func roundTo(v float64, decimals int) float64 {
	scale := math.Pow(10, float64(decimals))
	return math.Round(v*scale) / scale
}
