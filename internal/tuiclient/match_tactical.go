package tuiclient

import (
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
