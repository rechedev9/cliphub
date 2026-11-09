package tuiclient

import (
	"bytes"
	"context"
	"net/url"
	"strconv"

	"github.com/rechedev9/cliphub/internal/tacticalplan"
)

// RoundFilter selects the rounds of one match a tendencies read aggregates;
// zero fields are unset. TeamKey is ct-start or t-start.
type RoundFilter struct {
	TeamKey   string
	Side      string
	Buys      []string
	Outcome   string
	RoundFrom int
	RoundTo   int
}

func (f RoundFilter) encode() string {
	values := url.Values{}
	for k, v := range map[string]string{"team": f.TeamKey, "side": f.Side, "outcome": f.Outcome} {
		if v != "" {
			values.Set(k, v)
		}
	}
	for _, b := range f.Buys {
		values.Add("buy", b)
	}
	if f.RoundFrom > 0 {
		values.Set("round_from", strconv.Itoa(f.RoundFrom))
	}
	if f.RoundTo > 0 {
		values.Set("round_to", strconv.Itoa(f.RoundTo))
	}
	return encodeValues(values)
}

func libraryMatchPath(id string) string { return "/api/library/matches/" + url.PathEscape(id) }

// MatchTendencies aggregates the filtered rounds of one library match.
func (c *Client) MatchTendencies(ctx context.Context, matchID string, f RoundFilter) (Tendencies, error) {
	var out Tendencies
	err := c.getJSON(ctx, libraryMatchPath(matchID)+"/tactical/aggregate"+f.encode(), &out)
	return out, err
}

// WinPoint is the CT win probability after one event of a round.
type WinPoint struct {
	Seconds       float64 `json:"seconds"`
	Kind          string  `json:"kind"`
	ActorSlot     *uint8  `json:"actor_slot"`
	TargetSlot    *uint8  `json:"target_slot"`
	AliveCT       int     `json:"alive_ct"`
	AliveT        int     `json:"alive_t"`
	ProbabilityCT float64 `json:"probability_ct"`
}

// MatchMetrics is GET /api/library/matches/{id}/metrics: every player's
// metrics and each round's win probability timeline.
type MatchMetrics struct {
	PlayerMetricsReport
	Timelines []struct {
		Round  int        `json:"round"`
		Winner string     `json:"winner"`
		Points []WinPoint `json:"points"`
	} `json:"timelines"`
}

// LibraryMatchMetrics returns one match's player metrics and win probability.
func (c *Client) LibraryMatchMetrics(ctx context.Context, matchID string) (MatchMetrics, error) {
	var out MatchMetrics
	err := c.getJSON(ctx, libraryMatchPath(matchID)+"/metrics", &out)
	return out, err
}

// AnticheatDossier is one player's evidence pack from a finished screening.
type AnticheatDossier struct {
	SteamID64  string  `json:"steamid64"`
	Name       string  `json:"name"`
	Verdict    string  `json:"verdict"`
	Score      float64 `json:"score"`
	Confidence float64 `json:"confidence"`
	ProfileURL string  `json:"profile_url"`
	Markdown   string  `json:"markdown"`
	Channels   []struct {
		Label        string `json:"label"`
		URL          string `json:"url"`
		Instructions string `json:"instructions"`
		Effective    bool   `json:"effective"`
	} `json:"channels"`
	Policy struct {
		Summary  string   `json:"summary"`
		Rules    []string `json:"rules"`
		Rejected string   `json:"rejected"`
	} `json:"policy"`
}

// GetAnticheatDossier returns a player's dossier; 409 means the screening is not ready.
func (c *Client) GetAnticheatDossier(ctx context.Context, jobID, steamID string) (AnticheatDossier, error) {
	var out AnticheatDossier
	err := c.getJSON(ctx, jobPath(jobID)+"/anticheat/dossier/"+url.PathEscape(steamID), &out)
	return out, err
}

// LibraryTacticalPlan returns a library match's tactical document in the
// orchestrator's own type, so a caller can run tacticalplan.Filter over it.
func (c *Client) LibraryTacticalPlan(ctx context.Context, matchID string) (tacticalplan.Document, error) {
	var out tacticalplan.Document
	err := c.getJSON(ctx, libraryMatchPath(matchID)+"/tactical", &out)
	return out, err
}

// TacticalRoundFrames is GET /api/library/matches/{id}/tactical/rounds/{round}:
// one round and its sampled positions.
type TacticalRoundFrames struct {
	Round  tacticalplan.Round   `json:"round"`
	Frames []tacticalplan.Frame `json:"frames"`
}

// LibraryTacticalRound returns one round of a library match with its position samples.
func (c *Client) LibraryTacticalRound(ctx context.Context, matchID string, round int) (TacticalRoundFrames, error) {
	var out TacticalRoundFrames
	err := c.getJSON(ctx, libraryMatchPath(matchID)+"/tactical/rounds/"+strconv.Itoa(round), &out)
	return out, err
}

// MapRadar returns CS2's overview image of a map as PNG, read from the user's
// install; a 404 APIError means Studio has no radar for it.
func (c *Client) MapRadar(ctx context.Context, mapName string) ([]byte, error) {
	var buf bytes.Buffer
	_, err := c.stream(ctx, "/api/maps/"+url.PathEscape(mapName)+"/radar", &buf)
	return buf.Bytes(), err
}

// RoundNote is a saved note of a round: its tick and its drawing.
type RoundNote struct {
	Tick    int `json:"tick"`
	Strokes []struct {
		Color  string       `json:"color"`
		Width  int          `json:"width"`
		Arrow  bool         `json:"arrow"`
		Points [][2]float64 `json:"points"`
	} `json:"strokes"`
}

// LibraryRoundNotes lists the notes saved on one round of a library match.
func (c *Client) LibraryRoundNotes(ctx context.Context, matchID string, round int) ([]RoundNote, error) {
	var out struct {
		Annotations []RoundNote `json:"annotations"`
	}
	q := url.Values{"match_id": {matchID}, "round": {strconv.Itoa(round)}}
	err := c.getJSON(ctx, "/api/library/annotations?"+q.Encode(), &out)
	return out.Annotations, err
}

// RoundLineups is GET /api/library/matches/{id}/tactical/rounds/{round}/lineups:
// a round's grenades against the lineup catalog.
type RoundLineups struct {
	Map         string         `json:"map"`
	CatalogSize int            `json:"catalog_size"`
	Smokes      int            `json:"smokes"`
	Matched     int            `json:"matched"`
	Grenades    []RoundGrenade `json:"grenades"`
}

// RoundGrenade is one grenade of the round and the catalog lineup it matches, if any.
type RoundGrenade struct {
	Seconds float64 `json:"seconds"`
	Kind    string  `json:"kind"`
	Side    string  `json:"side"`
	Player  string  `json:"player"`
	Callout string  `json:"callout"`
	Status  string  `json:"status"`
	Lineup  *struct {
		ID          string `json:"id"`
		Destination string `json:"destination"`
		FromArea    string `json:"from_area"`
	} `json:"lineup"`
}

// LibraryRoundLineups compares one round's grenades with the lineup catalog.
func (c *Client) LibraryRoundLineups(ctx context.Context, matchID string, round int) (RoundLineups, error) {
	var out RoundLineups
	err := c.getJSON(ctx, libraryMatchPath(matchID)+"/tactical/rounds/"+strconv.Itoa(round)+"/lineups", &out)
	return out, err
}

// MatchOverlay is GET /api/library/matches/{id}/tactical/overlay: the rounds a
// scope superposes, each with the paths of the players it counts.
type MatchOverlay struct {
	Map     string                `json:"map"`
	Overlay *tacticalplan.Overlay `json:"overlay"`
}

// LibraryMatchOverlay superposes the rounds of one match that the filter keeps.
func (c *Client) LibraryMatchOverlay(ctx context.Context, matchID string, filter url.Values) (MatchOverlay, error) {
	var out MatchOverlay
	suffix := ""
	if len(filter) > 0 {
		suffix = "?" + filter.Encode()
	}
	err := c.getJSON(ctx, libraryMatchPath(matchID)+"/tactical/overlay"+suffix, &out)
	return out, err
}

// SpatialHeatmap is the heatmap answer of a match or a team: the map, the
// matches read, the radar geometry the cells are named on, and the grid.
type SpatialHeatmap struct {
	Map     string `json:"map"`
	Matches []struct {
		MatchID string `json:"match_id"`
	} `json:"matches"`
	Skipped []struct {
		MatchID string `json:"match_id"`
	} `json:"skipped"`
	Geometry *tacticalplan.MapGeometry `json:"geometry"`
	Heatmap  *tacticalplan.Heatmap     `json:"heatmap"`
}

// LibraryMatchHeatmap counts one match's rounds that the query keeps into a heatmap.
func (c *Client) LibraryMatchHeatmap(ctx context.Context, matchID string, query url.Values) (SpatialHeatmap, error) {
	var out SpatialHeatmap
	err := c.getJSON(ctx, libraryMatchPath(matchID)+"/tactical/heatmap"+encodeValues(query), &out)
	return out, err
}

// LibraryTeamHeatmap counts a team's rounds on one map into a heatmap: q is
// the team's scope, heat the heatmap's own query and the round filter.
func (c *Client) LibraryTeamHeatmap(ctx context.Context, teamID string, q AnalysisQuery, heat url.Values) (SpatialHeatmap, error) {
	q.Limit = 0
	values := q.values()
	for key, list := range heat {
		values[key] = list
	}
	var out SpatialHeatmap
	err := c.getJSON(ctx, teamPath(teamID, "heatmap")+encodeValues(values), &out)
	return out, err
}
