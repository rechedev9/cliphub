package tuiclient

import (
	"context"
	"net/url"
	"strconv"
	"time"
)

// LibraryTeam is one team of the library.
type LibraryTeam struct {
	ID         string   `json:"id"`
	Name       string   `json:"name"`
	Aliases    []string `json:"aliases"`
	MatchCount int      `json:"match_count"`
}

// ListLibraryTeams lists the library's teams, optionally those whose name or
// alias contains search.
func (c *Client) ListLibraryTeams(ctx context.Context, search string) ([]LibraryTeam, error) {
	var out struct {
		Teams []LibraryTeam `json:"teams"`
	}
	err := c.getJSON(ctx, "/api/library/teams"+query(map[string]string{"q": search, "limit": "200"}), &out)
	return out.Teams, err
}

// AnalysisQuery filters a scouting report or a player's metrics; zero fields
// are unset. Limit only applies to player metrics; Opponents and Matches
// (team ids and match ids) only to team reports.
type AnalysisQuery struct {
	Map       string
	Side      string
	From      time.Time
	To        time.Time
	Limit     int
	Opponents []string
	Matches   []string
}

// encode builds the "?k=v" suffix of the query.
func (q AnalysisQuery) encode() string { return encodeValues(q.values()) }

// values holds the set fields, with opponent_id and match_id repeated.
func (q AnalysisQuery) values() url.Values {
	values := url.Values{}
	set := func(k, v string) {
		if v != "" {
			values.Set(k, v)
		}
	}
	set("map", q.Map)
	set("side", q.Side)
	if !q.From.IsZero() {
		set("from", q.From.UTC().Format(time.RFC3339))
	}
	if !q.To.IsZero() {
		set("to", q.To.UTC().Format(time.RFC3339))
	}
	if q.Limit > 0 {
		set("limit", strconv.Itoa(q.Limit))
	}
	for _, id := range q.Opponents {
		values.Add("opponent_id", id)
	}
	for _, id := range q.Matches {
		values.Add("match_id", id)
	}
	return values
}

func encodeValues(values url.Values) string {
	if len(values) == 0 {
		return ""
	}
	return "?" + values.Encode()
}

// Rate is a count over a total, with its percentage.
type Rate struct {
	Count    int     `json:"count"`
	Total    int     `json:"total"`
	Pct      float64 `json:"pct"`
	Reliable bool    `json:"reliable"`
}

// AnalysisSkipped is a match a report could not read.
type AnalysisSkipped struct {
	MatchID string `json:"match_id"`
	Reason  string `json:"reason"`
}

// ScoutReport is the part of GET /api/library/teams/{id}/scout the assistant reads.
type ScoutReport struct {
	TeamName string            `json:"team_name"`
	Skipped  []AnalysisSkipped `json:"skipped"`
	Matches  []struct {
		MatchID       string    `json:"match_id"`
		Map           string    `json:"map"`
		PlayedAt      time.Time `json:"played_at"`
		Opponent      string    `json:"opponent"`
		Score         int       `json:"score"`
		OpponentScore int       `json:"opponent_score"`
	} `json:"matches"`
	Maps []struct {
		Map       string `json:"map"`
		Matches   int    `json:"matches"`
		MatchWins int    `json:"match_wins"`
		Rounds    int    `json:"rounds"`
		WinRate   Rate   `json:"win_rate"`
		CTWinRate Rate   `json:"ct_win_rate"`
		TWinRate  Rate   `json:"t_win_rate"`
	} `json:"maps"`
	Tendencies struct {
		RoundCount int           `json:"round_count"`
		Wins       int           `json:"wins"`
		Buys       []ScoutBucket `json:"buys"`
		Sites      []ScoutBucket `json:"sites"`
		TPatterns  []ScoutBucket `json:"t_patterns"`
		CTPatterns []ScoutBucket `json:"ct_patterns"`
		Openings   struct {
			Rounds       int  `json:"rounds"`
			Won          Rate `json:"won"`
			TradedOnLoss Rate `json:"traded_on_loss"`
		} `json:"openings"`
		Players []struct {
			SteamID64     string  `json:"steamid64"`
			Name          string  `json:"name"`
			Rounds        int     `json:"rounds"`
			Kills         int     `json:"kills"`
			Deaths        int     `json:"deaths"`
			ADR           float64 `json:"adr"`
			OpeningKills  int     `json:"opening_kills"`
			OpeningDeaths int     `json:"opening_deaths"`
		} `json:"players"`
	} `json:"tendencies"`
	Executes []struct {
		Site    string `json:"site"`
		Rounds  int    `json:"rounds"`
		WinRate Rate   `json:"win_rate"`
		Plants  Rate   `json:"plants"`
	} `json:"executes"`
	AWP struct {
		Sides []struct {
			Side         string `json:"side"`
			Rounds       int    `json:"rounds"`
			Kills        int    `json:"kills"`
			OpeningKills int    `json:"opening_kills"`
		} `json:"sides"`
	} `json:"awp"`
}

// ScoutBucket is one buy, site or round pattern of a team's rounds. Exactly
// one of Buy, Site and Pattern is set.
type ScoutBucket struct {
	Buy     string `json:"buy,omitempty"`
	Site    string `json:"site,omitempty"`
	Pattern string `json:"pattern,omitempty"`
	Rounds  int    `json:"rounds"`
	Share   Rate   `json:"share"`
	WinRate Rate   `json:"win_rate"`
}

// TeamScout returns a team's scouting report over its matches.
func (c *Client) TeamScout(ctx context.Context, teamID string, q AnalysisQuery) (ScoutReport, error) {
	q.Limit = 0
	var out ScoutReport
	err := c.getJSON(ctx, "/api/library/teams/"+url.PathEscape(teamID)+"/scout"+q.encode(), &out)
	return out, err
}

// MetricCount is a count; its example rounds are left out.
type MetricCount struct {
	Count int `json:"count"`
}

// GrenadeDamage is the enemy damage of a player's HEs or molotovs.
type GrenadeDamage struct {
	EnemyDamage int `json:"enemy_damage"`
}

// PlayerMetricsReport is the part of GET /api/library/players/{steamid}/metrics
// the assistant reads.
type PlayerMetricsReport struct {
	Matches int               `json:"matches"`
	Rounds  int               `json:"rounds"`
	Skipped []AnalysisSkipped `json:"skipped"`
	Players []PlayerMetrics   `json:"players"`
}

// PlayerMetrics is one player's line of a metrics report.
type PlayerMetrics struct {
	SteamID64 string `json:"steamid64"`
	Name      string `json:"name"`
	Role      string `json:"role"`
	Matches   int    `json:"matches"`
	Rounds    int    `json:"rounds"`
	Kills     int    `json:"kills"`
	Deaths    int    `json:"deaths"`
	Swing     struct {
		WPA      float64 `json:"wpa"`
		PerRound float64 `json:"per_round"`
	} `json:"swing"`
	Trades struct {
		TradeKills   MetricCount `json:"trade_kills"`
		TradedDeaths MetricCount `json:"traded_deaths"`
	} `json:"trades"`
	Utility struct {
		Measured bool `json:"measured"`
		Flash    struct {
			Thrown  int         `json:"thrown"`
			Assists MetricCount `json:"assists"`
		} `json:"flash"`
		HE      GrenadeDamage `json:"he"`
		Molotov GrenadeDamage `json:"molotov"`
	} `json:"utility"`
	Box struct {
		Damage        int `json:"damage"`
		KASTRounds    int `json:"kast_rounds"`
		OpeningKills  int `json:"opening_kills"`
		OpeningDeaths int `json:"opening_deaths"`
		UtilityRounds int `json:"utility_rounds"`
	} `json:"box"`
	Buys []struct {
		Buy    string  `json:"buy"`
		Rounds int     `json:"rounds"`
		Won    int     `json:"won"`
		Kills  int     `json:"kills"`
		Deaths int     `json:"deaths"`
		WPA    float64 `json:"wpa"`
	} `json:"buys"`
}

// PlayerMetrics returns a player's advanced metrics over their recent matches.
func (c *Client) PlayerMetrics(ctx context.Context, steamID string, q AnalysisQuery) (PlayerMetricsReport, error) {
	var out PlayerMetricsReport
	err := c.getJSON(ctx, "/api/library/players/"+url.PathEscape(steamID)+"/metrics"+q.encode(), &out)
	return out, err
}

// TacticalDocument is the part of a match's tactical document that tells who
// played which side and buy in each round.
type TacticalDocument struct {
	Demo struct {
		Map string `json:"map"`
	} `json:"demo"`
	Players []struct {
		Slot      uint8  `json:"slot"`
		SteamID64 string `json:"steamid64"`
		Name      string `json:"name"`
		TeamKey   string `json:"team_key"`
	} `json:"players"`
	Rounds []struct {
		Number  int    `json:"number"`
		Winner  string `json:"winner"`
		Economy struct {
			CTBuy string `json:"ct_buy"`
			TBuy  string `json:"t_buy"`
		} `json:"economy"`
		Class struct {
			TSide  string `json:"t_side"`
			CTSide string `json:"ct_side"`
			Site   string `json:"site"`
		} `json:"class"`
		Players []struct {
			Slot   uint8  `json:"slot"`
			Side   string `json:"side"`
			Kills  int    `json:"kills"`
			Deaths int    `json:"deaths"`
		} `json:"players"`
	} `json:"rounds"`
}

// LibraryTactical returns a library match's tactical document.
func (c *Client) LibraryTactical(ctx context.Context, matchID string) (TacticalDocument, error) {
	var out TacticalDocument
	err := c.getJSON(ctx, "/api/library/matches/"+url.PathEscape(matchID)+"/tactical", &out)
	return out, err
}

// BaselineValue is the mean of one reference metric over the players that have it.
type BaselineValue struct {
	Mean    float64 `json:"mean"`
	Players int     `json:"players"`
	Stddev  float64 `json:"stddev"`
}

// Baseline is GET /api/library/baseline.
type Baseline struct {
	Scope       string                   `json:"scope"`
	Map         string                   `json:"map"`
	Source      string                   `json:"source"`
	Description string                   `json:"description"`
	MinRounds   int                      `json:"min_rounds"`
	Players     int                      `json:"players"`
	Rounds      int                      `json:"rounds"`
	Matches     int                      `json:"matches"`
	Skipped     int                      `json:"skipped"`
	Metrics     map[string]BaselineValue `json:"metrics"`
}

// BaselineQuery selects the players a baseline averages; zero fields are unset.
type BaselineQuery struct {
	Scope     string
	Map       string
	MinRounds int
}

// LibraryBaseline returns the per-player averages of the reference metrics.
func (c *Client) LibraryBaseline(ctx context.Context, q BaselineQuery) (Baseline, error) {
	params := map[string]string{"scope": q.Scope, "map": q.Map}
	if q.MinRounds > 0 {
		params["min_rounds"] = strconv.Itoa(q.MinRounds)
	}
	var out Baseline
	err := c.getJSON(ctx, "/api/library/baseline"+query(params), &out)
	return out, err
}
