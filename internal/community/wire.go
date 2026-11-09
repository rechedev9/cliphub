// Package community is the contract between ClipHub Studio and the community
// server (services/community): the server-built pro reference, the pro lineups
// and the wire types of the server's rankings, plus the Studio side that fetches
// and caches. Studio no longer submits rankings.
package community

import (
	"math"
	"regexp"
	"sort"
)

// DefaultBaseURL is the community server Studio reaches when nothing overrides it.
const DefaultBaseURL = "https://cliphub.rechedev.cloud"

const (
	// AllMaps keys the rankings of every map pooled.
	AllMaps = "all"
	// MinRankingInstalls is how many installs a metric needs before percentiles show.
	MinRankingInstalls = 20
	// MinRankingRounds is the rounds a map needs before an install submits it.
	MinRankingRounds = 20
	// MaxRankingMaps bounds one submission.
	MaxRankingMaps = 24
	// QuantilePoints is p0 to p100 in steps of one.
	QuantilePoints = 101
)

// MapPattern is the map name a submission or query may carry.
var MapPattern = regexp.MustCompile(`^(all|[a-z]{2,4}_[a-z0-9_]{1,32})$`)

// InstallIDPattern is a random version 4 UUID in lower case.
var InstallIDPattern = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)

// ProReferenceMapInfo is the sample of one map in the server reference.
type ProReferenceMapInfo struct {
	Map     string `json:"map"`
	Matches int    `json:"matches"`
	Rounds  int    `json:"rounds"`
	Players int    `json:"players"`
}

// ProReferenceIndex lists the maps the server reference covers.
type ProReferenceIndex struct {
	ID          string                `json:"id"`
	Description string                `json:"description"`
	UpdatedAt   string                `json:"updated_at"`
	Matches     int                   `json:"matches"`
	Rounds      int                   `json:"rounds"`
	Players     int                   `json:"players"`
	Maps        []ProReferenceMapInfo `json:"maps"`
}

// RankingMap is one install's own line on one map: no names, ids or paths.
type RankingMap struct {
	Map     string             `json:"map"`
	Rounds  int                `json:"rounds"`
	Matches int                `json:"matches"`
	Values  map[string]float64 `json:"values"`
}

// RankingSubmission replaces everything an install has submitted.
type RankingSubmission struct {
	InstallID string       `json:"install_id"`
	Maps      []RankingMap `json:"maps"`
}

// RankingDistribution is how every install spreads on one metric of one map.
// Quantiles is empty until Installs reaches MinInstalls.
type RankingDistribution struct {
	Installs  int       `json:"installs"`
	Quantiles []float64 `json:"quantiles,omitempty"`
}

// RankingsResponse is GET /v1/rankings.
type RankingsResponse struct {
	Map         string                         `json:"map"`
	MinInstalls int                            `json:"min_installs"`
	Metrics     map[string]RankingDistribution `json:"metrics"`
}

// Quantiles returns p0 to p100 of values by linear interpolation, or nil
// for an empty sample.
func Quantiles(values []float64) []float64 {
	if len(values) == 0 {
		return nil
	}
	sorted := append([]float64(nil), values...)
	sort.Float64s(sorted)
	out := make([]float64, QuantilePoints)
	last := float64(len(sorted) - 1)
	for i := range out {
		pos := last * float64(i) / float64(QuantilePoints-1)
		lo := int(math.Floor(pos))
		hi := min(lo+1, len(sorted)-1)
		v := sorted[lo] + (sorted[hi]-sorted[lo])*(pos-float64(lo))
		out[i] = math.Round(v*1e4) / 1e4
	}
	return out
}
