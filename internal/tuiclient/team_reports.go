package tuiclient

import (
	"context"
	"net/url"
	"strconv"
	"time"
)

// Evidence is one round a report counted, by match, round and tick.
type Evidence struct {
	MatchID string `json:"match_id"`
	Round   int    `json:"round"`
	Tick    int    `json:"tick"`
}

// Histogram is the part of a seconds histogram the assistant reads.
type Histogram struct {
	Samples int     `json:"samples"`
	Median  float64 `json:"median"`
}

// ReportGroup is a set of rounds of a playbook or a prep; Refs holds every
// round, Examples only the first ones.
type ReportGroup struct {
	Rounds   int        `json:"rounds"`
	Share    Rate       `json:"share"`
	WinRate  Rate       `json:"win_rate"`
	Examples []Evidence `json:"examples"`
	Refs     []Evidence `json:"refs"`
}

// BombsiteDefense is the team defending one site.
type BombsiteDefense struct {
	Hit              Rate       `json:"hit"`
	Held             Rate       `json:"held"`
	PlantsAllowed    Rate       `json:"plants_allowed"`
	Won              Rate       `json:"won"`
	RetakesAttempted Rate       `json:"retakes_attempted"`
	RetakesWon       Rate       `json:"retakes_won"`
	Examples         []Evidence `json:"examples"`
}

// BombsiteAttack is the team attacking one site.
type BombsiteAttack struct {
	Attacks      Rate       `json:"attacks"`
	Plants       Rate       `json:"plants"`
	Won          Rate       `json:"won"`
	PostPlantWon Rate       `json:"post_plant_won"`
	Examples     []Evidence `json:"examples"`
}

// BombsiteMap is a team's record on both sites of one map.
type BombsiteMap struct {
	Map      string `json:"map"`
	Matches  int    `json:"matches"`
	CTRounds int    `json:"ct_rounds"`
	TRounds  int    `json:"t_rounds"`
	Sites    []struct {
		Site string          `json:"site"`
		CT   BombsiteDefense `json:"ct"`
		T    BombsiteAttack  `json:"t"`
	} `json:"sites"`
}

// BombsiteReport is GET /api/library/teams/{id}/bombsites.
type BombsiteReport struct {
	TeamName   string            `json:"team_name"`
	Skipped    []AnalysisSkipped `json:"skipped"`
	MatchCount int               `json:"match_count"`
	Maps       []BombsiteMap     `json:"maps"`
}

// TeamBombsites returns a team's record per map and bombsite.
func (c *Client) TeamBombsites(ctx context.Context, teamID string, q AnalysisQuery) (BombsiteReport, error) {
	q.Limit = 0
	var out BombsiteReport
	err := c.getJSON(ctx, teamPath(teamID, "bombsites")+q.encode(), &out)
	return out, err
}

// PlaybookGroup is a playbook category, site or zone.
type PlaybookGroup struct {
	Category string `json:"category"`
	Site     string `json:"site"`
	Zone     string `json:"zone"`
	Label    string `json:"label"`
	ReportGroup
}

// TeamPlaybook is GET /api/library/teams/{id}/playbook.
type TeamPlaybook struct {
	TeamName   string            `json:"team_name"`
	Skipped    []AnalysisSkipped `json:"skipped"`
	Map        string            `json:"map"`
	Side       string            `json:"side"`
	Matches    int               `json:"matches"`
	RoundCount int               `json:"round_count"`
	WinRate    Rate              `json:"win_rate"`
	Framed     int               `json:"framed"`
	Categories []PlaybookGroup   `json:"categories"`
	Tempo      Histogram         `json:"tempo"`
	Sites      []PlaybookGroup   `json:"sites"`
	Zones      []PlaybookGroup   `json:"zones"`
	Tactics    []PlaybookTactic  `json:"tactics"`
	Combo      PlaybookCombo     `json:"combo"`
	Rounds     []PlaybookRound   `json:"rounds"`
}

// PlaybookTactic is the rounds that show one tactic of a playbook.
type PlaybookTactic struct {
	Key     string `json:"key"`
	Kind    string `json:"kind"`
	Value   string `json:"value"`
	Grenade string `json:"grenade"`
	Label   string `json:"label"`
	ReportGroup
}

// PlaybookCombo is the tactic combination a playbook was narrowed to.
type PlaybookCombo struct {
	With    []string `json:"with"`
	Without []string `json:"without"`
}

// PlaybookRound is the part of a playbook round the assistant reads.
type PlaybookRound struct {
	MatchID string `json:"match_id"`
	Round   int    `json:"round"`
	Tick    int    `json:"tick"`
	Pattern string `json:"pattern"`
	Site    string `json:"site"`
	Won     bool   `json:"won"`
}

// TeamPlaybookReport returns what a team does on one side of one map.
func (c *Client) TeamPlaybookReport(ctx context.Context, teamID string, q AnalysisQuery) (TeamPlaybook, error) {
	q.Limit = 0
	var out TeamPlaybook
	err := c.getJSON(ctx, teamPath(teamID, "playbook")+q.encode(), &out)
	return out, err
}

// PrepRow is a buy matchup or a tempo row of a prep.
type PrepRow struct {
	Side        string    `json:"side"`
	Buy         string    `json:"buy"`
	OpponentBuy string    `json:"opponent_buy"`
	Tempo       Histogram `json:"tempo"`
	Plant       Histogram `json:"plant"`
	ReportGroup
}

// PrepPlayer is one CT player of a prep and their spots per window.
type PrepPlayer struct {
	SteamID64 string `json:"steamid64"`
	Name      string `json:"name"`
	Role      string `json:"role"`
	Rounds    int    `json:"rounds"`
	Windows   []struct {
		FromSeconds int `json:"from_seconds"`
		ToSeconds   int `json:"to_seconds"`
		Rounds      int `json:"rounds"`
		Spots       []struct {
			Callout string     `json:"callout"`
			Share   Rate       `json:"share"`
			Refs    []Evidence `json:"refs"`
		} `json:"spots"`
	} `json:"windows"`
}

// TeamPrep is GET /api/library/teams/{id}/prep.
type TeamPrep struct {
	TeamName   string            `json:"team_name"`
	Skipped    []AnalysisSkipped `json:"skipped"`
	Map        string            `json:"map"`
	Matches    int               `json:"matches"`
	RoundCount int               `json:"round_count"`
	Framed     int               `json:"framed"`
	Matchups   []PrepRow         `json:"matchups"`
	Tempo      []PrepRow         `json:"tempo"`
	CTPlayers  []PrepPlayer      `json:"ct_players"`
}

// TeamPrepReport returns the match prep against a team on one map.
func (c *Client) TeamPrepReport(ctx context.Context, teamID string, q AnalysisQuery) (TeamPrep, error) {
	q.Limit, q.Side = 0, ""
	var out TeamPrep
	err := c.getJSON(ctx, teamPath(teamID, "prep")+q.encode(), &out)
	return out, err
}

// TeamGrenade is one grenade a team threw. Origin is nil for a match analysed
// before throws were recorded.
type TeamGrenade struct {
	MatchID          string      `json:"match_id"`
	Round            int         `json:"round"`
	Seconds          float64     `json:"seconds"`
	Kind             string      `json:"kind"`
	Side             string      `json:"side"`
	ThrowerSteamID64 string      `json:"thrower_steamid64"`
	ThrowerName      string      `json:"thrower_name"`
	Origin           *[3]float64 `json:"origin"`
	ThrowTick        *int        `json:"throw_tick"`
	View             *[2]float64 `json:"view"`
	Landing          [3]float64  `json:"landing"`
	RoundWon         bool        `json:"round_won"`
}

// TeamGrenades is GET /api/library/teams/{id}/grenades.
type TeamGrenades struct {
	Grenades []TeamGrenade     `json:"grenades"`
	Rounds   int               `json:"rounds"`
	Skipped  []AnalysisSkipped `json:"skipped"`
}

// TeamGrenadeList returns the team's grenades of the given kinds, all when none.
func (c *Client) TeamGrenadeList(ctx context.Context, teamID string, q AnalysisQuery, kinds []string) (TeamGrenades, error) {
	q.Limit = 0
	values := q.values()
	for _, kind := range kinds {
		values.Add("kind", kind)
	}
	path := teamPath(teamID, "grenades") + encodeValues(values)
	var out TeamGrenades
	err := c.getJSON(ctx, path, &out)
	return out, err
}

// PlayerPositions is GET /api/library/players/{steamid}/positions.
type PlayerPositions struct {
	MatchCount    int               `json:"match_count"`
	WindowSeconds int               `json:"window_seconds"`
	MinReliable   int               `json:"min_reliable_sample"`
	Skipped       []AnalysisSkipped `json:"skipped"`
	Maps          []struct {
		Map     string `json:"map"`
		Matches int    `json:"matches"`
		Sides   []struct {
			Side    string `json:"side"`
			Rounds  int    `json:"rounds"`
			Sampled int    `json:"sampled"`
			Role    string `json:"role"`
			Zones   []struct {
				Zone   string `json:"zone"`
				Rounds Rate   `json:"rounds"`
			} `json:"zones"`
			Places []struct {
				Place  string `json:"place"`
				Label  string `json:"label"`
				Rounds Rate   `json:"rounds"`
			} `json:"places"`
			AWPKills  Rate `json:"awp_kills"`
			AWPRounds Rate `json:"awp_rounds"`
		} `json:"sides"`
	} `json:"maps"`
}

// PlayerPositionsReport returns where a player stands early in their rounds.
func (c *Client) PlayerPositionsReport(ctx context.Context, steamID string, q AnalysisQuery) (PlayerPositions, error) {
	q.Limit, q.Side = 0, ""
	var out PlayerPositions
	err := c.getJSON(ctx, "/api/library/players/"+url.PathEscape(steamID)+"/positions"+q.encode(), &out)
	return out, err
}

// LeaderboardQuery selects the players a leaderboard ranks; zero fields are unset.
type LeaderboardQuery struct {
	Metric    string
	Scope     string
	Map       string
	MinRounds int
	Limit     int
	// Side, Role, From, To and Source narrow a library leaderboard; zero means unset.
	Side   string
	Role   string
	From   time.Time
	To     time.Time
	Source string
	// Days and Top narrow a pro leaderboard: the period and the rival's HLTV rank.
	Days int
	Top  int
}

// Leaderboard is GET /api/library/leaderboard.
type Leaderboard struct {
	Scope          string              `json:"scope"`
	Map            string              `json:"map"`
	Metric         string              `json:"metric"`
	HigherIsBetter bool                `json:"higher_is_better"`
	MinRounds      int                 `json:"min_rounds"`
	Matches        int                 `json:"matches"`
	Rounds         int                 `json:"rounds"`
	Skipped        int                 `json:"skipped"`
	Players        []LeaderboardPlayer `json:"players"`
}

// LeaderboardPlayer is one ranked player.
type LeaderboardPlayer struct {
	Rank      int                `json:"rank"`
	SteamID64 string             `json:"steamid64"`
	Name      string             `json:"name"`
	Team      string             `json:"team"`
	Rounds    int                `json:"rounds"`
	Matches   int                `json:"matches"`
	Value     float64            `json:"value"`
	Metrics   map[string]float64 `json:"metrics"`
	Roles     *struct {
		CT string `json:"ct"`
		T  string `json:"t"`
	} `json:"roles"`
	Examples []struct {
		MatchID string `json:"match_id"`
		Round   int    `json:"round"`
	} `json:"examples"`
}

// LibraryLeaderboard ranks the library's players on one reference metric.
func (c *Client) LibraryLeaderboard(ctx context.Context, q LeaderboardQuery) (Leaderboard, error) {
	params := map[string]string{"metric": q.Metric, "scope": q.Scope, "map": q.Map, "side": q.Side, "role": q.Role, "source": q.Source}
	if q.Days > 0 {
		params["days"] = strconv.Itoa(q.Days)
	}
	if q.Top > 0 {
		params["top"] = strconv.Itoa(q.Top)
	}
	if !q.From.IsZero() {
		params["from"] = q.From.UTC().Format(time.RFC3339)
	}
	if !q.To.IsZero() {
		params["to"] = q.To.UTC().Format(time.RFC3339)
	}
	if q.MinRounds > 0 {
		params["min_rounds"] = strconv.Itoa(q.MinRounds)
	}
	if q.Limit > 0 {
		params["limit"] = strconv.Itoa(q.Limit)
	}
	var out Leaderboard
	err := c.getJSON(ctx, "/api/library/leaderboard"+query(params), &out)
	return out, err
}

func teamPath(teamID, report string) string {
	return "/api/library/teams/" + url.PathEscape(teamID) + "/" + report
}
