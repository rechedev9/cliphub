package community

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"slices"
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

// ErrProServerDown means the server did not answer, failed or asked to slow down.
var ErrProServerDown = errors.New("community: the server did not answer")

// ProTeam is one team of a pro match as HLTV names it; Rank is 0 outside the HLTV ranking.
type ProTeam struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
	Rank int    `json:"rank,omitempty"`
}

// ProMapFile is one analysed map of a pro match. Entry names the map without a
// part suffix; a map split by a server restart lists one part per demo.
type ProMapFile struct {
	Entry  string `json:"entry"`
	Map    string `json:"map"`
	Rounds int    `json:"rounds"`
	// AnalysisVersion is the oldest part's; SHA256 changes when any part changes.
	AnalysisVersion int          `json:"analysis_version"`
	Bytes           int64        `json:"bytes"`
	SHA256          string       `json:"sha256"`
	Parts           []ProMapPart `json:"parts"`
}

// ProMapPart is one analysed demo of a map; Entry is the {map} of its file route.
type ProMapPart struct {
	Entry           string `json:"entry"`
	FirstRound      int    `json:"first_round"`
	Rounds          int    `json:"rounds"`
	AnalysisVersion int    `json:"analysis_version"`
	Bytes           int64  `json:"bytes"`
	SHA256          string `json:"sha256"`
}

// ProMatchURL is the HLTV page of a match, where its demos can be downloaded.
func ProMatchURL(matchID int) string {
	return "https://www.hltv.org/matches/" + strconv.Itoa(matchID) + "/match"
}

// ProMatch is one HLTV match of the catalog with its analysed maps in play order.
type ProMatch struct {
	ID       int          `json:"id"`
	Event    string       `json:"event"`
	PlayedAt time.Time    `json:"played_at"`
	Teams    []ProTeam    `json:"teams"`
	Maps     []ProMapFile `json:"maps"`
	// Series is empty from a server older than the series catalog.
	Series ProSeries `json:"series"`
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
	Days int
	// Since, when set, starts the period instead of Days, such as the reader's local midnight for today.
	Since time.Time
	Team  string
	Event string
	Map   string
	Page  int
}

// Start is when the period of q begins.
func (q ProMatchQuery) Start(now time.Time) time.Time {
	if !q.Since.IsZero() {
		return q.Since
	}
	return now.AddDate(0, 0, -q.Days)
}

// ParseProMatchQuery reads days, since, team, event, map and page, with the defaults filled in.
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
	if raw := v.Get("since"); raw != "" {
		t, err := time.Parse(time.RFC3339, raw)
		if err != nil || time.Since(t) > MaxProMatchDays*24*time.Hour || time.Until(t) > 24*time.Hour {
			return q, fmt.Errorf("since must be an RFC 3339 time within the last %d days", MaxProMatchDays)
		}
		q.Since = t.UTC()
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
	if !q.Since.IsZero() {
		v.Set("since", q.Since.UTC().Format(time.RFC3339))
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

// ProSidePairing is what pairs the sides of a demo with the teams of its HLTV match.
type ProSidePairing struct {
	// Clans holds each side's clan name, empty for a fallback name.
	Clans   []string
	Rosters [][]string
	Teams   []ProTeam
	// Known holds the SteamIDs earlier imports put on each HLTV team id.
	Known map[int][]string
}

// minRosterOverlap is how many known players a side needs to be taken for a team.
const minRosterOverlap = 2

// PairProSides pairs sides by clan name first, then by the players earlier
// imports of each HLTV team fielded. The result holds a team index or -1 per side.
func PairProSides(p ProSidePairing) []int {
	out := MatchProSides(p.Clans, p.Teams)
	if !slices.Contains(out, -1) || len(p.Rosters) != len(out) {
		return out
	}
	overlap := func(side, team int) int {
		known := p.Known[p.Teams[team].ID]
		n := 0
		for _, sid := range p.Rosters[side] {
			if slices.Contains(known, sid) {
				n++
			}
		}
		return n
	}
	if len(out) == 2 && len(p.Teams) == 2 {
		straight, crossed := overlap(0, 0)+overlap(1, 1), overlap(0, 1)+overlap(1, 0)
		switch {
		case straight > crossed && straight >= minRosterOverlap:
			out[0], out[1] = 0, 1
		case crossed > straight && crossed >= minRosterOverlap:
			out[0], out[1] = 1, 0
		}
		return out
	}
	taken := map[int]bool{}
	for _, idx := range out {
		taken[idx] = true
	}
	for i := range out {
		if out[i] >= 0 {
			continue
		}
		best, bestN := -1, minRosterOverlap-1
		for j := range p.Teams {
			if n := overlap(i, j); !taken[j] && n > bestN {
				best, bestN = j, n
			}
		}
		if best >= 0 {
			out[i], taken[best] = best, true
		}
	}
	return out
}

// proRequest is one GET of the pro match routes.
type proRequest struct {
	path    string
	accept  string
	timeout time.Duration
	limit   int64
	missing error
}

func (c *Client) proGet(ctx context.Context, r proRequest) ([]byte, error) {
	req, err := c.request(ctx, http.MethodGet, r.path, nil)
	if err != nil {
		return nil, err
	}
	if r.accept != "" {
		req.Header.Set("Accept", r.accept)
	}
	client := *c.http
	if r.timeout > 0 {
		client.Timeout = r.timeout
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrProServerDown, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return nil, r.missing
	}
	body, err := readBody(resp, r.limit)
	if err != nil && (resp.StatusCode >= 500 || resp.StatusCode == http.StatusTooManyRequests) {
		return nil, fmt.Errorf("%w: %w", ErrProServerDown, err)
	}
	return body, err
}

// ProMatches reads one page of the pro match catalog.
func (c *Client) ProMatches(ctx context.Context, q ProMatchQuery) (ProMatchesPage, error) {
	path := "/v1/pro-matches"
	if v := q.Values(); len(v) > 0 {
		path += "?" + v.Encode()
	}
	body, err := c.proGet(ctx, proRequest{path: path, limit: maxResponseBytes, missing: ErrNoProCatalog})
	if err != nil {
		return ProMatchesPage{}, err
	}
	var page ProMatchesPage
	if err := json.Unmarshal(body, &page); err != nil {
		return ProMatchesPage{}, fmt.Errorf("community: decode pro matches: %w", err)
	}
	return page, nil
}

// ProMatch reads one match of the catalog, whatever its date.
func (c *Client) ProMatch(ctx context.Context, matchID int) (ProMatch, error) {
	if matchID <= 0 {
		return ProMatch{}, fmt.Errorf("community: invalid pro match %d", matchID)
	}
	body, err := c.proGet(ctx, proRequest{path: "/v1/pro-matches/" + strconv.Itoa(matchID), limit: maxResponseBytes, missing: ErrNoProMap})
	if err != nil {
		return ProMatch{}, err
	}
	var m ProMatch
	if err := json.Unmarshal(body, &m); err != nil {
		return ProMatch{}, fmt.Errorf("community: decode pro match: %w", err)
	}
	return m, nil
}

// ProMapFileBytes downloads the analysis of one part of a pro map, compressed as the server keeps it.
func (c *Client) ProMapFileBytes(ctx context.Context, matchID int, entry string) ([]byte, error) {
	if matchID <= 0 || !ProEntryPattern.MatchString(entry) {
		return nil, fmt.Errorf("community: invalid pro map %d/%q", matchID, entry)
	}
	return c.proGet(ctx, proRequest{path: "/v1/pro-matches/" + strconv.Itoa(matchID) + "/" + url.PathEscape(entry),
		accept: "application/zstd", timeout: proFileTimeout, limit: maxProFileBytes, missing: ErrNoProMap})
}
