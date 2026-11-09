package community

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode"
)

const (
	// ProMatchesPageSize is how many matches one catalog page lists.
	ProMatchesPageSize = 30
	// DefaultProMatchDays is the period a catalog read covers when it names none.
	DefaultProMatchDays = 7
	// MaxProMatchDays bounds the period of one catalog read.
	MaxProMatchDays = 120
	// maxProFileBytes bounds one downloaded pro analysis; a map is about 1 MB.
	maxProFileBytes = 64 << 20
	proFileTimeout  = 3 * time.Minute
	maxProFilter    = 80
)

// ProEntryPattern is a pro map's demo entry without ".dem": the {map} of its file route.
var ProEntryPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_.()-]{0,200}$`)

// ErrNoProCatalog means the server has no pro match catalog, as before it was deployed.
var ErrNoProCatalog = errors.New("community: the server has no pro match catalog")

// ErrNoProMap means the server has no analysis of that pro map.
var ErrNoProMap = errors.New("community: no such pro map")

// ProTeam is one team of a pro match as HLTV names it; Rank is 0 outside the HLTV ranking.
type ProTeam struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
	Rank int    `json:"rank,omitempty"`
}

// ProMapFile is one analysed demo of a pro match. A map split by a server
// restart has one file per part.
type ProMapFile struct {
	Entry           string `json:"entry"`
	Map             string `json:"map"`
	Rounds          int    `json:"rounds"`
	AnalysisVersion int    `json:"analysis_version"`
	Bytes           int64  `json:"bytes"`
	SHA256          string `json:"sha256"`
}

// ProMatch is one HLTV match of the catalog with its analysed maps in play order.
type ProMatch struct {
	ID       int          `json:"id"`
	Event    string       `json:"event"`
	PlayedAt time.Time    `json:"played_at"`
	Teams    []ProTeam    `json:"teams"`
	Maps     []ProMapFile `json:"maps"`
}

// ProTeamCount is a team of the period and its matches.
type ProTeamCount struct {
	ProTeam
	Matches int `json:"matches"`
}

// ProNameCount is an event or map of the period and its matches.
type ProNameCount struct {
	Name    string `json:"name"`
	Matches int    `json:"matches"`
}

// ProFacets are the teams, events and maps of the whole period, for the filter menus.
type ProFacets struct {
	Teams  []ProTeamCount `json:"teams"`
	Events []ProNameCount `json:"events"`
	Maps   []ProNameCount `json:"maps"`
}

// ProMatchesPage is GET /v1/pro-matches: one page of the catalog, newest first.
type ProMatchesPage struct {
	UpdatedAt time.Time  `json:"updated_at"`
	Days      int        `json:"days"`
	Page      int        `json:"page"`
	Pages     int        `json:"pages"`
	Total     int        `json:"total"`
	Matches   []ProMatch `json:"matches"`
	Facets    ProFacets  `json:"facets"`
}

// ProMatchQuery filters the catalog. Team is an HLTV team id or part of a name;
// Event is part of an event name; zero fields do not filter.
type ProMatchQuery struct {
	Days  int
	Team  string
	Event string
	Map   string
	Page  int
}

// ParseProMatchQuery reads days, team, event, map and page, with the defaults filled in.
func ParseProMatchQuery(v url.Values) (ProMatchQuery, error) {
	q := ProMatchQuery{Days: DefaultProMatchDays, Page: 1, Team: strings.TrimSpace(v.Get("team")),
		Event: strings.TrimSpace(v.Get("event")), Map: v.Get("map")}
	if raw := v.Get("days"); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || n < 1 || n > MaxProMatchDays {
			return q, fmt.Errorf("days must be from 1 to %d", MaxProMatchDays)
		}
		q.Days = n
	}
	if raw := v.Get("page"); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || n < 1 || n > 1000 {
			return q, errors.New("page must be a page number")
		}
		q.Page = n
	}
	if q.Map != "" && (!MapPattern.MatchString(q.Map) || q.Map == AllMaps) {
		return q, errors.New("map must be a map name like de_mirage")
	}
	if len(q.Team) > maxProFilter || len(q.Event) > maxProFilter {
		return q, fmt.Errorf("team and event take at most %d characters", maxProFilter)
	}
	return q, nil
}

// Values is the query string of q, without the defaults.
func (q ProMatchQuery) Values() url.Values {
	v := url.Values{}
	if q.Days > 0 && q.Days != DefaultProMatchDays {
		v.Set("days", strconv.Itoa(q.Days))
	}
	for key, value := range map[string]string{"team": q.Team, "event": q.Event, "map": q.Map} {
		if value != "" {
			v.Set(key, value)
		}
	}
	if q.Page > 1 {
		v.Set("page", strconv.Itoa(q.Page))
	}
	return v
}

// ProTeamKey is a team name as the side matching compares it: lower case
// letters and digits, without a leading "team".
func ProTeamKey(name string) string {
	key := strings.Map(func(r rune) rune {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			return unicode.ToLower(r)
		}
		return -1
	}, name)
	if rest, ok := strings.CutPrefix(key, "team"); ok && len(rest) >= 2 {
		return rest
	}
	return key
}

func proNameScore(clan, team string) int {
	a, b := ProTeamKey(clan), ProTeamKey(team)
	switch {
	case a == "" || b == "":
		return 0
	case a == b:
		return 3
	case len(a) >= 3 && len(b) >= 3 && (strings.Contains(a, b) || strings.Contains(b, a)):
		return 2
	}
	return 0
}

// MatchProSides pairs each clan name of a demo with the team it is: the result
// holds a team index or -1 per clan. With two of each, one named side places the other.
func MatchProSides(clans []string, teams []ProTeam) []int {
	out := make([]int, len(clans))
	for i := range out {
		out[i] = -1
	}
	if len(clans) == 2 && len(teams) == 2 {
		straight := proNameScore(clans[0], teams[0].Name) + proNameScore(clans[1], teams[1].Name)
		crossed := proNameScore(clans[0], teams[1].Name) + proNameScore(clans[1], teams[0].Name)
		switch {
		case straight > crossed:
			out[0], out[1] = 0, 1
		case crossed > straight:
			out[0], out[1] = 1, 0
		}
		return out
	}
	taken := map[int]bool{}
	for i, clan := range clans {
		best, bestScore := -1, 0
		for j, team := range teams {
			if s := proNameScore(clan, team.Name); !taken[j] && s > bestScore {
				best, bestScore = j, s
			}
		}
		if best >= 0 {
			out[i], taken[best] = best, true
		}
	}
	return out
}

// ProMatches reads one page of the pro match catalog.
func (c *Client) ProMatches(ctx context.Context, q ProMatchQuery) (ProMatchesPage, error) {
	path := "/v1/pro-matches"
	if v := q.Values(); len(v) > 0 {
		path += "?" + v.Encode()
	}
	req, err := c.request(ctx, http.MethodGet, path, nil)
	if err != nil {
		return ProMatchesPage{}, err
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return ProMatchesPage{}, fmt.Errorf("community: pro matches: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return ProMatchesPage{}, ErrNoProCatalog
	}
	body, err := readBody(resp, maxResponseBytes)
	if err != nil {
		return ProMatchesPage{}, err
	}
	var page ProMatchesPage
	if err := json.Unmarshal(body, &page); err != nil {
		return ProMatchesPage{}, fmt.Errorf("community: decode pro matches: %w", err)
	}
	return page, nil
}

// ProMapFileBytes downloads the analysis of one pro map, compressed as the server keeps it.
func (c *Client) ProMapFileBytes(ctx context.Context, matchID int, entry string) ([]byte, error) {
	if matchID <= 0 || !ProEntryPattern.MatchString(entry) {
		return nil, fmt.Errorf("community: invalid pro map %d/%q", matchID, entry)
	}
	req, err := c.request(ctx, http.MethodGet, "/v1/pro-matches/"+strconv.Itoa(matchID)+"/"+url.PathEscape(entry), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/zstd")
	long := *c.http
	long.Timeout = proFileTimeout
	resp, err := long.Do(req)
	if err != nil {
		return nil, fmt.Errorf("community: pro map: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return nil, ErrNoProMap
	}
	return readBody(resp, maxProFileBytes)
}
