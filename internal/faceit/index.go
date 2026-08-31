package faceit

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode"
	"unicode/utf8"
)

const pageSize = 100

type apiPlayer struct {
	PlayerID  string             `json:"player_id"`
	Nickname  string             `json:"nickname"`
	Avatar    string             `json:"avatar"`
	Country   string             `json:"country"`
	SteamID64 string             `json:"steam_id_64"`
	Games     map[string]apiGame `json:"games"`
}

type apiGame struct {
	Region       string `json:"region"`
	SkillLevel   int    `json:"skill_level"`
	FaceitELO    int    `json:"faceit_elo"`
	GamePlayerID string `json:"game_player_id"`
}

type apiHistoryResponse struct {
	Items []apiHistoryItem `json:"items"`
}

type apiHistoryItem struct {
	MatchID         string             `json:"match_id"`
	Region          string             `json:"region"`
	CompetitionName string             `json:"competition_name"`
	CompetitionType string             `json:"competition_type"`
	StartedAt       int64              `json:"started_at"`
	FinishedAt      int64              `json:"finished_at"`
	Teams           map[string]apiTeam `json:"teams"`
	Results         apiResults         `json:"results"`
}

type apiTeam struct {
	Players []apiTeamPlayer `json:"players"`
}

type apiTeamPlayer struct {
	PlayerID string `json:"player_id"`
}

type apiResults struct {
	Winner string         `json:"winner"`
	Score  map[string]int `json:"score"`
}

type apiStatsResponse struct {
	Items []apiStatsItem `json:"items"`
}

type apiStatsItem struct {
	Stats apiMatchStats `json:"stats"`
}

type apiMatchStats struct {
	MatchID          statValue `json:"Match Id"`
	MatchFinishedAt  statValue `json:"Match Finished At"`
	Map              statValue `json:"Map"`
	Result           statValue `json:"Result"`
	Rounds           statValue `json:"Rounds"`
	Kills            statValue `json:"Kills"`
	Deaths           statValue `json:"Deaths"`
	Assists          statValue `json:"Assists"`
	Damage           statValue `json:"Damage"`
	ADR              statValue `json:"ADR"`
	KDRatio          statValue `json:"K/D Ratio"`
	KRRatio          statValue `json:"K/R Ratio"`
	Headshots        statValue `json:"Headshots"`
	HeadshotsPercent statValue `json:"Headshots %"`
	DoubleKills      statValue `json:"Double Kills"`
	TripleKills      statValue `json:"Triple Kills"`
	QuadroKills      statValue `json:"Quadro Kills"`
	PentaKills       statValue `json:"Penta Kills"`
}

type statValue string

func (v *statValue) UnmarshalJSON(data []byte) error {
	if string(data) == "null" {
		*v = ""
		return nil
	}
	var text string
	if len(data) > 0 && data[0] == '"' {
		if err := json.Unmarshal(data, &text); err != nil {
			return err
		}
		*v = statValue(text)
		return nil
	}
	var number json.Number
	if err := json.Unmarshal(data, &number); err != nil {
		return err
	}
	*v = statValue(number.String())
	return nil
}

func (v statValue) string() string {
	return strings.TrimSpace(string(v))
}

func (v statValue) int() int {
	n, _ := strconv.Atoi(v.string())
	return n
}

func (v statValue) float() float64 {
	n, _ := strconv.ParseFloat(v.string(), 64)
	return n
}

type apiMatchDetails struct {
	DemoURLs  []string      `json:"demo_url"`
	Instances []apiInstance `json:"instances"`
}

type apiInstance struct {
	Demos []string `json:"demos"`
}

// ParseProfile accepts a FACEIT player URL or a nickname and returns the
// nickname used by the Data API lookup endpoint.
func ParseProfile(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", fmt.Errorf("FACEIT profile is required")
	}
	if strings.Contains(raw, "://") {
		u, err := url.Parse(raw)
		if err != nil || u.Scheme != "https" || u.User != nil || u.Hostname() == "" {
			return "", fmt.Errorf("FACEIT profile URL must be an absolute HTTPS URL")
		}
		host := strings.ToLower(u.Hostname())
		if host != "faceit.com" && host != "www.faceit.com" {
			return "", fmt.Errorf("FACEIT profile URL host must be faceit.com")
		}
		parts := strings.Split(strings.Trim(u.EscapedPath(), "/"), "/")
		for i := 0; i+1 < len(parts); i++ {
			if parts[i] != "players" {
				continue
			}
			nickname, err := url.PathUnescape(parts[i+1])
			if err != nil {
				return "", fmt.Errorf("FACEIT profile URL contains an invalid nickname")
			}
			return validateNickname(nickname)
		}
		return "", fmt.Errorf("FACEIT profile URL must contain /players/<nickname>")
	}
	return validateNickname(raw)
}

func validateNickname(nickname string) (string, error) {
	nickname = strings.TrimSpace(nickname)
	if nickname == "" || !utf8.ValidString(nickname) || utf8.RuneCountInString(nickname) > 64 {
		return "", fmt.Errorf("FACEIT nickname must contain between 1 and 64 characters")
	}
	for _, r := range nickname {
		if unicode.IsSpace(r) || unicode.IsControl(r) || strings.ContainsRune(`/\\?#%`, r) {
			return "", fmt.Errorf("FACEIT nickname contains an unsupported character")
		}
	}
	return nickname, nil
}

func (c *Client) Index(ctx context.Context, request IndexRequest) (Index, error) {
	if c == nil || c.apiKey == "" {
		return Index{}, ErrNotConfigured
	}
	nickname, err := ParseProfile(request.Profile)
	if err != nil {
		return Index{}, err
	}
	from := request.From.UTC().Truncate(time.Second)
	to := request.To.UTC().Truncate(time.Second)
	if from.IsZero() || to.IsZero() || to.Before(from) {
		return Index{}, fmt.Errorf("FACEIT index range must have a valid from time on or before to")
	}

	player, err := c.fetchPlayer(ctx, nickname)
	if err != nil {
		return Index{}, fmt.Errorf("look up FACEIT player: %w", err)
	}
	if !validIdentifier(player.PlayerID) || player.Nickname == "" {
		return Index{}, ErrInvalidResponse
	}
	history, err := c.fetchHistory(ctx, player.PlayerID, from, to)
	if err != nil {
		return Index{}, fmt.Errorf("fetch FACEIT match history: %w", err)
	}
	stats, err := c.fetchStats(ctx, player.PlayerID, from, to)
	if err != nil {
		return Index{}, fmt.Errorf("fetch FACEIT match statistics: %w", err)
	}

	index := buildIndex(c.now().UTC(), playerFromAPI(player), from, to, history, stats)
	if err := c.resolveDemos(ctx, index.Matches); err != nil {
		return Index{}, fmt.Errorf("resolve FACEIT demo availability: %w", err)
	}
	finalizeIndex(&index)
	if indexContainsCredential(index, c.apiKey) {
		return Index{}, ErrInvalidResponse
	}
	return index, nil
}

func (c *Client) LookupBySteamID(ctx context.Context, steamID64 string) (Player, error) {
	if c == nil || c.apiKey == "" {
		return Player{}, ErrNotConfigured
	}
	steamID64 = strings.TrimSpace(steamID64)
	if steamID64 == "" || !validIdentifier(steamID64) {
		return Player{}, fmt.Errorf("SteamID64 is invalid")
	}
	query := url.Values{"game_player_id": {steamID64}, "game": {"cs2"}}
	var raw apiPlayer
	if err := c.getJSON(ctx, "/players", query, &raw); err != nil {
		var apiErr *APIError
		if errors.As(err, &apiErr) && apiErr.StatusCode == http.StatusNotFound {
			return Player{}, ErrPlayerNotFound
		}
		return Player{}, fmt.Errorf("look up FACEIT player by steamid: %w", err)
	}
	player := playerFromAPI(raw)
	if !ValidPlayerID(player.ID) || player.Nickname == "" {
		return Player{}, ErrInvalidResponse
	}
	if playerContainsCredential(player, c.apiKey) {
		return Player{}, ErrInvalidResponse
	}
	return player, nil
}

func (c *Client) RankingPosition(ctx context.Context, region, playerID string) (int, error) {
	if c == nil || c.apiKey == "" {
		return 0, ErrNotConfigured
	}
	if !ValidPlayerID(playerID) {
		return 0, fmt.Errorf("FACEIT player id is invalid")
	}
	region = strings.TrimSpace(region)
	if region == "" {
		region = "EU"
	}
	if !validIdentifier(region) {
		return 0, fmt.Errorf("FACEIT region is invalid")
	}
	var raw rankingResponse
	endpoint := "/rankings/games/cs2/regions/" + url.PathEscape(region) + "/players/" + url.PathEscape(playerID)
	if err := c.getJSON(ctx, endpoint, nil, &raw); err != nil {
		var apiErr *APIError
		if errors.As(err, &apiErr) && apiErr.StatusCode == http.StatusNotFound {
			return 0, nil
		}
		return 0, fmt.Errorf("look up FACEIT ranking: %w", err)
	}
	if raw.Position > 0 {
		return raw.Position, nil
	}
	if len(raw.Items) > 0 && raw.Items[0].Position > 0 {
		return raw.Items[0].Position, nil
	}
	return 0, nil
}

type rankingResponse struct {
	Position int `json:"position"`
	Items    []struct {
		Position int `json:"position"`
	} `json:"items"`
}

func (c *Client) LookupPlayer(ctx context.Context, profile string) (Player, error) {
	if c == nil || c.apiKey == "" {
		return Player{}, ErrNotConfigured
	}
	nickname, err := ParseProfile(profile)
	if err != nil {
		return Player{}, err
	}
	raw, err := c.fetchPlayer(ctx, nickname)
	if err != nil {
		var apiErr *APIError
		if errors.As(err, &apiErr) && apiErr.StatusCode == http.StatusNotFound {
			return Player{}, ErrPlayerNotFound
		}
		return Player{}, fmt.Errorf("look up FACEIT player: %w", err)
	}
	player := playerFromAPI(raw)
	if !ValidPlayerID(player.ID) || player.Nickname == "" {
		return Player{}, ErrInvalidResponse
	}
	if playerContainsCredential(player, c.apiKey) {
		return Player{}, ErrInvalidResponse
	}
	return player, nil
}

const (
	defaultRecentMatchLimit = 10
	maxRecentMatchLimit     = 30
)

func clampRecentMatchLimit(limit int) int {
	if limit <= 0 {
		return defaultRecentMatchLimit
	}
	if limit > maxRecentMatchLimit {
		return maxRecentMatchLimit
	}
	return limit
}

func (c *Client) RecentMatches(ctx context.Context, playerID string, limit int) ([]RecentMatch, error) {
	if c == nil || c.apiKey == "" {
		return nil, ErrNotConfigured
	}
	if !ValidPlayerID(playerID) {
		return nil, fmt.Errorf("FACEIT player id is invalid")
	}
	limit = clampRecentMatchLimit(limit)
	history, err := c.fetchRecentHistory(ctx, playerID, limit)
	if err != nil {
		return nil, fmt.Errorf("fetch FACEIT match history: %w", err)
	}
	stats, err := c.fetchRecentStats(ctx, playerID, limit)
	if err != nil {
		return nil, fmt.Errorf("fetch FACEIT match statistics: %w", err)
	}
	matches := buildRecentMatches(playerID, history, stats, limit)
	if recentMatchesContainCredential(matches, c.apiKey) {
		return nil, ErrInvalidResponse
	}
	return matches, nil
}

func (c *Client) fetchPlayer(ctx context.Context, nickname string) (apiPlayer, error) {
	query := url.Values{"nickname": {nickname}, "game": {"cs2"}}
	var player apiPlayer
	if err := c.getJSON(ctx, "/players", query, &player); err != nil {
		return apiPlayer{}, err
	}
	return player, nil
}

func (c *Client) fetchHistory(ctx context.Context, playerID string, from, to time.Time) ([]apiHistoryItem, error) {
	byID := make(map[string]apiHistoryItem)
	for _, interval := range monthlyIntervals(from, to) {
		for offset := 0; offset <= 1000; offset += pageSize {
			query := url.Values{
				"game":   {"cs2"},
				"from":   {strconv.FormatInt(interval.from.Unix(), 10)},
				"to":     {strconv.FormatInt(interval.to.Unix(), 10)},
				"offset": {strconv.Itoa(offset)},
				"limit":  {strconv.Itoa(pageSize)},
			}
			var page apiHistoryResponse
			endpoint := "/players/" + url.PathEscape(playerID) + "/history"
			if err := c.getJSON(ctx, endpoint, query, &page); err != nil {
				return nil, err
			}
			for _, item := range page.Items {
				if !validIdentifier(item.MatchID) {
					return nil, ErrInvalidResponse
				}
				byID[item.MatchID] = item
			}
			if len(page.Items) < pageSize {
				break
			}
			if offset == 1000 {
				return nil, fmt.Errorf("more than 1100 FACEIT matches in one month; narrow the date range")
			}
		}
	}
	items := make([]apiHistoryItem, 0, len(byID))
	for _, item := range byID {
		items = append(items, item)
	}
	return items, nil
}

func (c *Client) fetchStats(ctx context.Context, playerID string, from, to time.Time) ([]apiMatchStats, error) {
	byID := make(map[string]apiMatchStats)
	for _, interval := range monthlyIntervals(from, to) {
		items, err := c.fetchStatsInterval(ctx, playerID, interval.from, interval.to, 0)
		if err != nil {
			return nil, err
		}
		for _, item := range items {
			if validIdentifier(item.MatchID.string()) {
				byID[item.MatchID.string()] = item
			}
		}
	}
	items := make([]apiMatchStats, 0, len(byID))
	for _, item := range byID {
		items = append(items, item)
	}
	return items, nil
}

func (c *Client) fetchStatsInterval(ctx context.Context, playerID string, from, to time.Time, depth int) ([]apiMatchStats, error) {
	var items []apiMatchStats
	for offset := 0; offset <= 200; offset += pageSize {
		query := url.Values{
			"from":   {strconv.FormatInt(from.UnixMilli(), 10)},
			"to":     {strconv.FormatInt(to.UnixMilli(), 10)},
			"offset": {strconv.Itoa(offset)},
			"limit":  {strconv.Itoa(pageSize)},
		}
		var page apiStatsResponse
		endpoint := "/players/" + url.PathEscape(playerID) + "/games/cs2/stats"
		if err := c.getJSON(ctx, endpoint, query, &page); err != nil {
			return nil, err
		}
		for _, item := range page.Items {
			items = append(items, item.Stats)
		}
		if len(page.Items) < pageSize {
			return items, nil
		}
	}

	// FACEIT caps this endpoint at offset 200. Split a dense interval instead
	// of silently dropping matches beyond the third full page.
	if depth >= 20 || !to.After(from.Add(time.Millisecond)) {
		return nil, fmt.Errorf("FACEIT stats interval exceeds the 300-match pagination window")
	}
	mid := from.Add(to.Sub(from) / 2).Truncate(time.Millisecond)
	left, err := c.fetchStatsInterval(ctx, playerID, from, mid, depth+1)
	if err != nil {
		return nil, err
	}
	right, err := c.fetchStatsInterval(ctx, playerID, mid.Add(time.Millisecond), to, depth+1)
	if err != nil {
		return nil, err
	}
	return append(left, right...), nil
}

type timeInterval struct {
	from time.Time
	to   time.Time
}

func monthlyIntervals(from, to time.Time) []timeInterval {
	from = from.UTC()
	to = to.UTC()
	var intervals []timeInterval
	for cursor := from; !cursor.After(to); {
		nextMonth := time.Date(cursor.Year(), cursor.Month()+1, 1, 0, 0, 0, 0, time.UTC)
		end := nextMonth.Add(-time.Millisecond)
		if end.After(to) {
			end = to
		}
		intervals = append(intervals, timeInterval{from: cursor, to: end})
		cursor = end.Add(time.Millisecond)
	}
	return intervals
}

func buildIndex(generatedAt time.Time, player Player, from, to time.Time, history []apiHistoryItem, stats []apiMatchStats) Index {
	index := Index{
		SchemaVersion: SchemaVersion,
		GeneratedAt:   generatedAt,
		Source:        "FACEIT Data API v4",
		Player:        player,
		Range:         DateRange{From: from, To: to},
		Acquisition: Acquisition{
			Mode:                         "manual_faceit_room",
			Instructions:                 "Open room_url and use FACEIT's Watch/Demo download until Download API access is approved.",
			DownloadAPIAutomationPending: true,
		},
		HighlightRankBasis: []string{"penta_kills desc", "quadro_kills desc", "triple_kills desc", "kills desc", "adr desc", "finished_at desc"},
		Matches:            make([]Match, 0, len(history)),
	}
	statsByMatch := make(map[string]apiMatchStats, len(stats))
	for _, item := range stats {
		statsByMatch[item.MatchID.string()] = item
	}
	for _, item := range history {
		match := historyMatch(player.ID, item)
		if raw, ok := statsByMatch[item.MatchID]; ok {
			converted := convertStats(raw)
			match.Stats = &converted
			if match.FinishedAt.IsZero() {
				match.FinishedAt = statFinishedAt(raw.MatchFinishedAt)
			}
		}
		index.Matches = append(index.Matches, match)
	}
	sort.Slice(index.Matches, func(i, j int) bool {
		if !index.Matches[i].FinishedAt.Equal(index.Matches[j].FinishedAt) {
			return index.Matches[i].FinishedAt.After(index.Matches[j].FinishedAt)
		}
		return index.Matches[i].ID < index.Matches[j].ID
	})
	return index
}

func historyMatch(playerID string, item apiHistoryItem) Match {
	playerTeam := ""
	for teamName, team := range item.Teams {
		for _, member := range team.Players {
			if member.PlayerID == playerID {
				playerTeam = teamName
				break
			}
		}
	}
	match := Match{
		ID:               item.MatchID,
		RoomURL:          canonicalRoomURL(item.MatchID),
		Competition:      item.CompetitionName,
		CompetitionType:  item.CompetitionType,
		Region:           item.Region,
		Score:            MatchScore{PlayerTeam: playerTeam, WinnerTeam: item.Results.Winner},
		DemoLookupStatus: "pending",
		DemoResourceURLs: []string{},
	}
	if item.StartedAt > 0 {
		match.StartedAt = time.Unix(item.StartedAt, 0).UTC()
	}
	if item.FinishedAt > 0 {
		match.FinishedAt = time.Unix(item.FinishedAt, 0).UTC()
	}
	if playerTeam != "" {
		match.Score.For = item.Results.Score[playerTeam]
		for teamName, score := range item.Results.Score {
			if teamName != playerTeam {
				match.Score.Against = score
				break
			}
		}
	}
	return match
}

func convertStats(raw apiMatchStats) MatchStats {
	return MatchStats{
		Map:              raw.Map.string(),
		Result:           normalizeResult(raw.Result.string()),
		Rounds:           raw.Rounds.int(),
		Kills:            raw.Kills.int(),
		Deaths:           raw.Deaths.int(),
		Assists:          raw.Assists.int(),
		Damage:           raw.Damage.int(),
		ADR:              raw.ADR.float(),
		KDRatio:          raw.KDRatio.float(),
		KRRatio:          raw.KRRatio.float(),
		Headshots:        raw.Headshots.int(),
		HeadshotsPercent: raw.HeadshotsPercent.float(),
		DoubleKills:      raw.DoubleKills.int(),
		TripleKills:      raw.TripleKills.int(),
		QuadroKills:      raw.QuadroKills.int(),
		PentaKills:       raw.PentaKills.int(),
	}
}

func normalizeResult(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "1", "true", "win", "won":
		return "win"
	case "0", "false", "loss", "lost":
		return "loss"
	default:
		return "unknown"
	}
}

func statFinishedAt(raw statValue) time.Time {
	value, err := strconv.ParseInt(raw.string(), 10, 64)
	if err != nil || value <= 0 {
		return time.Time{}
	}
	if value > 10_000_000_000 {
		return time.UnixMilli(value).UTC()
	}
	return time.Unix(value, 0).UTC()
}

func (c *Client) resolveDemos(ctx context.Context, matches []Match) error {
	if len(matches) == 0 {
		return nil
	}
	workerCount := min(c.detailWorkers, len(matches))
	workerCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	jobs := make(chan int)
	var wg sync.WaitGroup
	var firstErr error
	var errOnce sync.Once

	worker := func() {
		defer wg.Done()
		for {
			select {
			case <-workerCtx.Done():
				return
			case index, ok := <-jobs:
				if !ok {
					return
				}
				if err := c.resolveDemo(workerCtx, &matches[index]); err != nil {
					errOnce.Do(func() {
						firstErr = err
						cancel()
					})
					return
				}
			}
		}
	}
	for range workerCount {
		wg.Add(1)
		go worker()
	}

sendLoop:
	for i := range matches {
		select {
		case jobs <- i:
		case <-workerCtx.Done():
			break sendLoop
		}
	}
	close(jobs)
	wg.Wait()
	if firstErr != nil {
		return firstErr
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	return nil
}

func (c *Client) resolveDemo(ctx context.Context, match *Match) error {
	var details apiMatchDetails
	endpoint := "/matches/" + url.PathEscape(match.ID)
	err := c.getJSON(ctx, endpoint, nil, &details)
	if err != nil {
		var apiErr *APIError
		if errors.As(err, &apiErr) && apiErr.StatusCode == http.StatusNotFound {
			match.DemoLookupStatus = "match_not_found"
			return nil
		}
		return fmt.Errorf("match %s: %w", match.ID, err)
	}
	seen := make(map[string]struct{})
	for _, raw := range details.DemoURLs {
		if strings.Contains(raw, c.apiKey) {
			continue
		}
		if cleaned, ok := cleanDemoResourceURL(raw); ok {
			seen[cleaned] = struct{}{}
		}
	}
	for _, instance := range details.Instances {
		for _, raw := range instance.Demos {
			if strings.Contains(raw, c.apiKey) {
				continue
			}
			if cleaned, ok := cleanDemoResourceURL(raw); ok {
				seen[cleaned] = struct{}{}
			}
		}
	}
	match.DemoResourceURLs = make([]string, 0, len(seen))
	for resourceURL := range seen {
		match.DemoResourceURLs = append(match.DemoResourceURLs, resourceURL)
	}
	sort.Strings(match.DemoResourceURLs)
	match.DemoAvailable = len(match.DemoResourceURLs) > 0
	if match.DemoAvailable {
		match.DemoLookupStatus = "available"
	} else {
		match.DemoLookupStatus = "unavailable"
	}
	return nil
}

func indexContainsCredential(index Index, credential string) bool {
	if credential == "" {
		return false
	}
	body, err := json.Marshal(index)
	return err != nil || bytes.Contains(body, []byte(credential))
}

func cleanDemoResourceURL(raw string) (string, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" || len(raw) > 4096 || strings.ContainsAny(raw, "\r\n\t\\") {
		return "", false
	}
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "https" || u.Hostname() == "" || u.User != nil || u.Fragment != "" {
		return "", false
	}
	return u.String(), true
}

func finalizeIndex(index *Index) {
	var adrTotal float64
	var adrCount int
	for i := range index.Matches {
		match := &index.Matches[i]
		index.Summary.Matches++
		if match.DemoAvailable {
			index.Summary.DemoAvailable++
		} else {
			index.Summary.DemoUnavailable++
		}
		if match.Stats == nil {
			continue
		}
		index.Summary.StatsEnriched++
		index.Summary.Kills += match.Stats.Kills
		index.Summary.Deaths += match.Stats.Deaths
		index.Summary.Assists += match.Stats.Assists
		switch match.Stats.Result {
		case "win":
			index.Summary.Wins++
		case "loss":
			index.Summary.Losses++
		}
		if match.Stats.ADR > 0 {
			adrTotal += match.Stats.ADR
			adrCount++
		}
	}
	if adrCount > 0 {
		index.Summary.AverageADR = adrTotal / float64(adrCount)
	}

	candidates := make([]*Match, 0, index.Summary.StatsEnriched)
	for i := range index.Matches {
		if index.Matches[i].Stats != nil {
			candidates = append(candidates, &index.Matches[i])
		}
	}
	sort.Slice(candidates, func(i, j int) bool {
		left, right := candidates[i], candidates[j]
		comparisons := [][2]int{
			{left.Stats.PentaKills, right.Stats.PentaKills},
			{left.Stats.QuadroKills, right.Stats.QuadroKills},
			{left.Stats.TripleKills, right.Stats.TripleKills},
			{left.Stats.Kills, right.Stats.Kills},
		}
		for _, values := range comparisons {
			if values[0] != values[1] {
				return values[0] > values[1]
			}
		}
		if left.Stats.ADR != right.Stats.ADR {
			return left.Stats.ADR > right.Stats.ADR
		}
		if !left.FinishedAt.Equal(right.FinishedAt) {
			return left.FinishedAt.After(right.FinishedAt)
		}
		return left.ID < right.ID
	})
	index.HighlightMatchIDs = make([]string, 0, len(candidates))
	for _, match := range candidates {
		index.HighlightMatchIDs = append(index.HighlightMatchIDs, match.ID)
	}
}

func (c *Client) fetchRecentHistory(ctx context.Context, playerID string, limit int) ([]apiHistoryItem, error) {
	query := url.Values{
		"game":   {"cs2"},
		"offset": {"0"},
		"limit":  {strconv.Itoa(limit)},
	}
	var page apiHistoryResponse
	endpoint := "/players/" + url.PathEscape(playerID) + "/history"
	if err := c.getJSON(ctx, endpoint, query, &page); err != nil {
		return nil, err
	}
	items := make([]apiHistoryItem, 0, len(page.Items))
	for _, item := range page.Items {
		if !validIdentifier(item.MatchID) {
			return nil, ErrInvalidResponse
		}
		items = append(items, item)
	}
	return items, nil
}

func (c *Client) fetchRecentStats(ctx context.Context, playerID string, limit int) ([]apiMatchStats, error) {
	query := url.Values{
		"offset": {"0"},
		"limit":  {strconv.Itoa(limit)},
	}
	var page apiStatsResponse
	endpoint := "/players/" + url.PathEscape(playerID) + "/games/cs2/stats"
	if err := c.getJSON(ctx, endpoint, query, &page); err != nil {
		return nil, err
	}
	items := make([]apiMatchStats, 0, len(page.Items))
	for _, item := range page.Items {
		if validIdentifier(item.Stats.MatchID.string()) {
			items = append(items, item.Stats)
		}
	}
	return items, nil
}

func playerFromAPI(player apiPlayer) Player {
	game := player.Games["cs2"]
	return Player{
		ID:         player.PlayerID,
		Nickname:   player.Nickname,
		Avatar:     cleanAvatarURL(player.Avatar),
		SteamID64:  firstNonEmpty(player.SteamID64, game.GamePlayerID),
		ProfileURL: canonicalProfileURL(player.Nickname),
		Country:    player.Country,
		Region:     game.Region,
		SkillLevel: game.SkillLevel,
		ELO:        game.FaceitELO,
	}
}

func buildRecentMatches(playerID string, history []apiHistoryItem, stats []apiMatchStats, limit int) []RecentMatch {
	statsByMatch := make(map[string]apiMatchStats, len(stats))
	for _, item := range stats {
		statsByMatch[item.MatchID.string()] = item
	}
	matches := make([]RecentMatch, 0, len(history))
	for _, item := range history {
		full := historyMatch(playerID, item)
		match := RecentMatch{
			ID:          full.ID,
			RoomURL:     full.RoomURL,
			StartedAt:   full.StartedAt,
			FinishedAt:  full.FinishedAt,
			Competition: full.Competition,
			Score:       full.Score,
		}
		if raw, ok := statsByMatch[item.MatchID]; ok {
			converted := convertStats(raw)
			match.Stats = &converted
			if match.FinishedAt.IsZero() {
				match.FinishedAt = statFinishedAt(raw.MatchFinishedAt)
			}
		}
		if match.Stats == nil {
			match.Stats = &MatchStats{Result: resultFromScore(match.Score)}
		} else if match.Stats.Result == "unknown" {
			if derived := resultFromScore(match.Score); derived != "unknown" {
				match.Stats.Result = derived
			}
		}
		matches = append(matches, match)
	}
	sort.Slice(matches, func(i, j int) bool {
		if !matches[i].FinishedAt.Equal(matches[j].FinishedAt) {
			return matches[i].FinishedAt.After(matches[j].FinishedAt)
		}
		return matches[i].ID < matches[j].ID
	})
	if len(matches) > limit {
		matches = matches[:limit]
	}
	return matches
}

// Last20 is an honest aggregate of recent FACEIT matches (up to 30).
// Rating and swing are omitted because match stats do not provide them.
type Last20 struct {
	Matches *int
	WinPct  *float64
	Kills   *int
	Deaths  *int
	Assists *int
	KD      *float64
	KR      *float64
	ADR     *float64
}

func AggregateLast20(matches []RecentMatch) Last20 {
	if len(matches) == 0 {
		return Last20{}
	}
	n := len(matches)
	matchesN := n
	out := Last20{Matches: &matchesN}
	var wins, withResult, kills, deaths, assists, withKD, withKR, withADR int
	var kdSum, krSum, adrSum float64
	for _, match := range matches {
		if match.Stats == nil {
			continue
		}
		if match.Stats.Result == "win" || match.Stats.Result == "loss" {
			withResult++
			if match.Stats.Result == "win" {
				wins++
			}
		}
		kills += match.Stats.Kills
		deaths += match.Stats.Deaths
		assists += match.Stats.Assists
		if match.Stats.KDRatio > 0 {
			kdSum += match.Stats.KDRatio
			withKD++
		}
		if match.Stats.KRRatio > 0 {
			krSum += match.Stats.KRRatio
			withKR++
		}
		if match.Stats.ADR > 0 {
			adrSum += match.Stats.ADR
			withADR++
		}
	}
	if withResult > 0 {
		pct := 100 * float64(wins) / float64(withResult)
		out.WinPct = &pct
	}
	if kills > 0 || deaths > 0 || assists > 0 {
		out.Kills = &kills
		out.Deaths = &deaths
		out.Assists = &assists
	}
	if withKD > 0 {
		kd := kdSum / float64(withKD)
		out.KD = &kd
	}
	if withKR > 0 {
		kr := krSum / float64(withKR)
		out.KR = &kr
	}
	if withADR > 0 {
		adr := adrSum / float64(withADR)
		out.ADR = &adr
	}
	return out
}

func resultFromScore(score MatchScore) string {
	if score.PlayerTeam == "" || score.WinnerTeam == "" {
		return "unknown"
	}
	if score.PlayerTeam == score.WinnerTeam {
		return "win"
	}
	return "loss"
}

func cleanAvatarURL(raw string) string {
	cleaned, ok := cleanDemoResourceURL(raw)
	if !ok {
		return ""
	}
	return cleaned
}

func playerContainsCredential(player Player, credential string) bool {
	if credential == "" {
		return false
	}
	body, err := json.Marshal(player)
	return err != nil || bytes.Contains(body, []byte(credential))
}

func recentMatchesContainCredential(matches []RecentMatch, credential string) bool {
	if credential == "" {
		return false
	}
	body, err := json.Marshal(matches)
	return err != nil || bytes.Contains(body, []byte(credential))
}

func ValidPlayerID(value string) bool {
	return validIdentifier(value)
}

func validIdentifier(value string) bool {
	if value == "" || len(value) > 128 {
		return false
	}
	for _, r := range value {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_' {
			continue
		}
		return false
	}
	return true
}

func canonicalRoomURL(matchID string) string {
	return "https://www.faceit.com/en/cs2/room/" + url.PathEscape(matchID)
}

func canonicalProfileURL(nickname string) string {
	return "https://www.faceit.com/en/players/" + url.PathEscape(nickname)
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}
