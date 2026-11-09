package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/google/uuid"

	"github.com/rechedev9/cliphub/internal/community"
	"github.com/rechedev9/cliphub/internal/tacticalplan"
)

// proPlayersPrefix keys the stored per-map payloads of pro players; the index
// lists the maps. They live in pro_reference next to the lineups.
const (
	proPlayersPrefix   = "pro-players:"
	proPlayersIndexKey = proPlayersPrefix + "index"

	proPlayersDefaultDays = 90
	proPlayersMaxLimit    = 500
	proPlayersMaxDays     = 365
)

// proPlayersPayload is the stored lines of one map.
type proPlayersPayload struct {
	BuiltAt time.Time                      `json:"built_at"`
	Matches []tacticalplan.ProPlayersMatch `json:"matches"`
}

type proPlayersIndex struct {
	BuiltAt time.Time `json:"built_at"`
	Maps    []string  `json:"maps"`
}

// normTeam lowers a team name to its letters and digits, so "Team Vitality" and "vitality" meet.
func normTeam(name string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(name) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func sameTeam(a, b string) bool {
	x, y := normTeam(a), normTeam(b)
	if x == "" || y == "" {
		return false
	}
	return x == y || (len(x) >= 3 && len(y) >= 3 && (strings.Contains(x, y) || strings.Contains(y, x)))
}

// resolveTeams pairs the document's teams (their clan names) with HLTV's two
// teams. A team no name reaches takes the one HLTV team left; otherwise it
// keeps its clan name and no rank.
func resolveTeams(doc []tacticalplan.Team, hltv []hltvTeam) map[string]tacticalplan.ProPlayersTeam {
	out := map[string]tacticalplan.ProPlayersTeam{}
	used := map[int]bool{}
	for _, t := range doc {
		out[t.Key] = tacticalplan.ProPlayersTeam{Name: t.Name}
		for i, h := range hltv {
			if !used[i] && sameTeam(t.Name, h.Name) {
				used[i] = true
				out[t.Key] = hltvProTeam(h)
				break
			}
		}
	}
	free := []int{}
	for i := range hltv {
		if !used[i] {
			free = append(free, i)
		}
	}
	unmatched := []string{}
	for _, t := range doc {
		if out[t.Key].Rank == 0 && out[t.Key].Name == t.Name {
			unmatched = append(unmatched, t.Key)
		}
	}
	if len(unmatched) == 1 && len(free) == 1 {
		out[unmatched[0]] = hltvProTeam(hltv[free[0]])
	}
	return out
}

func hltvProTeam(h hltvTeam) tacticalplan.ProPlayersTeam {
	t := tacticalplan.ProPlayersTeam{Name: h.Name}
	if h.Rank != nil && *h.Rank > 0 {
		t.Rank = *h.Rank
	}
	return t
}

// proPart is one analysis file of a map: its document and the opening frames of
// its rounds, read once so the file's position blob can go.
type proPart struct {
	header  analysisHeader
	opening map[int][]tacticalplan.Frame
	// first is the part's lowest game round; the later part owns a replayed round.
	first int
}

// ownedRounds keeps each game round of a map in one part. A restart part replays
// the round the server went down in, and the later part's copy is the one played
// out; a re-upload of the same demo owns nothing the first copy does not.
func ownedRounds(parts []proPart) []map[int]bool {
	owner := map[int]int{}
	for i, p := range parts {
		for _, r := range p.header.Document.Rounds {
			g := tacticalplan.GameRound(r)
			if j, ok := owner[g]; !ok || p.first > parts[j].first {
				owner[g] = i
			}
		}
	}
	out := make([]map[int]bool, len(parts))
	for i := range parts {
		out[i] = map[int]bool{}
	}
	for g, i := range owner {
		out[i][g] = true
	}
	return out
}

// readProPlayersGroup reads the parts of one map (one HLTV match, one demo entry) and joins them.
func readProPlayersGroup(files []string) (tacticalplan.ProPlayersMatch, bool) {
	parts := []proPart{}
	for _, name := range files {
		h, blob, err := readAnalysis(name)
		if err != nil {
			log.Printf("community stage=pro-players class=unreadable file=%s error=%v", filepath.Base(name), err)
			continue
		}
		opening, err := tacticalplan.OpeningFrames(blob, h.Document)
		if err != nil {
			log.Printf("community stage=pro-players class=no_positions file=%s error=%v", filepath.Base(name), err)
			opening = nil
		}
		p := proPart{header: h, opening: opening}
		for _, r := range h.Document.Rounds {
			if g := tacticalplan.GameRound(r); p.first == 0 || g < p.first {
				p.first = g
			}
		}
		parts = append(parts, p)
	}
	if len(parts) == 0 {
		return tacticalplan.ProPlayersMatch{}, false
	}
	owned := ownedRounds(parts)
	read := []tacticalplan.ProPlayersMatch{}
	for i, p := range parts {
		doc := p.header.Document
		doc.Rounds = nil
		for _, r := range p.header.Document.Rounds {
			if owned[i][tacticalplan.GameRound(r)] {
				doc.Rounds = append(doc.Rounds, r)
			}
		}
		if len(doc.Rounds) == 0 {
			continue
		}
		info := tacticalplan.ProPlayersInfo{
			MatchID: strconv.Itoa(p.header.Match.MatchID), Event: p.header.Match.Event, Teams: resolveTeams(doc.Teams, p.header.Match.Teams),
		}
		if t, err := time.Parse(time.RFC3339, p.header.Match.PlayedAt); err == nil {
			info.PlayedAt = t
		}
		read = append(read, tacticalplan.ReadProPlayersMatch(tacticalplan.MetricsMatch{MatchID: uuid.UUID{1}, PlayedAt: info.PlayedAt, Document: doc}, p.opening, info))
	}
	if len(read) == 0 {
		return tacticalplan.ProPlayersMatch{}, false
	}
	merged := tacticalplan.MergeProPlayersMatches(read)
	return merged, merged.Rounds >= minMapRounds
}

// readProPlayersMatches reads every archived analysis into its pro players
// lines, by map. Files of one match and map (restart parts, re-uploads) join into one map.
func readProPlayersMatches(ctx context.Context, analysesDir string) (map[string][]tacticalplan.ProPlayersMatch, error) {
	files, err := filepath.Glob(filepath.Join(analysesDir, "*"+analysisSuffix))
	if err != nil {
		return nil, err
	}
	slices.Sort(files)
	groups := map[string][]string{}
	keys := []string{}
	for _, name := range files {
		match, entry, ok := analysisName(filepath.Base(name))
		if !ok {
			continue
		}
		key := match + "/" + mapKey(entry)
		if groups[key] == nil {
			keys = append(keys, key)
		}
		groups[key] = append(groups[key], name)
	}
	out := map[string][]tacticalplan.ProPlayersMatch{}
	for _, key := range keys {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		if m, ok := readProPlayersGroup(groups[key]); ok {
			out[m.Map] = append(out[m.Map], m)
		}
	}
	return out, nil
}

// buildProPlayers stores one payload per map and the list of maps. Like the
// lineups, a folder with nothing readable leaves the stored payloads alone.
func buildProPlayers(ctx context.Context, s *store, analysesDir string, at time.Time) (proPlayersIndex, error) {
	byMap, err := readProPlayersMatches(ctx, analysesDir)
	if err != nil {
		return proPlayersIndex{}, err
	}
	if len(byMap) == 0 {
		return proPlayersIndex{}, fmt.Errorf("no pro players read from %s; the stored payloads stay", analysesDir)
	}
	index := proPlayersIndex{BuiltAt: at.UTC(), Maps: []string{}}
	payloads := map[string][]byte{}
	for m, matches := range byMap {
		body, err := json.Marshal(proPlayersPayload{BuiltAt: index.BuiltAt, Matches: matches})
		if err != nil {
			return proPlayersIndex{}, err
		}
		payloads[proPlayersPrefix+m] = body
		index.Maps = append(index.Maps, m)
		log.Printf("community stage=pro-players class=map map=%s maps=%d", m, len(matches))
	}
	slices.Sort(index.Maps)
	body, err := json.Marshal(index)
	if err != nil {
		return proPlayersIndex{}, err
	}
	payloads[proPlayersIndexKey] = body
	return index, s.replacePrefixed(ctx, proPlayersPrefix, payloads)
}

// proPlayersCache keeps the decoded payload of each map while its stored ETag holds.
type proPlayersCache struct {
	mu      sync.Mutex
	entries map[string]proPlayersEntry
}

type proPlayersEntry struct {
	etag    string
	payload proPlayersPayload
}

// proPlayersFor returns one map's lines, decoding it only when it changed.
func (a *api) proPlayersFor(ctx context.Context, mapName string) (proPlayersPayload, error) {
	key := proPlayersPrefix + mapName
	body, etag, err := a.store.referenceBody(ctx, key)
	if err != nil {
		return proPlayersPayload{}, err
	}
	c := &a.proPlayers
	c.mu.Lock()
	defer c.mu.Unlock()
	if e, ok := c.entries[key]; ok && e.etag == etag {
		return e.payload, nil
	}
	var p proPlayersPayload
	if err := json.Unmarshal(body, &p); err != nil {
		return proPlayersPayload{}, fmt.Errorf("decode %s: %w", key, err)
	}
	if c.entries == nil {
		c.entries = map[string]proPlayersEntry{}
	}
	c.entries[key] = proPlayersEntry{etag: etag, payload: p}
	return p, nil
}

type proPlayersRequest struct {
	mapName   string
	side      tacticalplan.Side
	role      string
	mapRole   string
	event     string
	days      int
	top       int
	teamTop   int
	metric    string
	minRounds int
	limit     int
}

func parseProPlayersRequest(r *http.Request) (proPlayersRequest, error) {
	q := r.URL.Query()
	req := proPlayersRequest{
		mapName: q.Get("map"), metric: q.Get("metric"), role: q.Get("role"), mapRole: q.Get("map_role"), event: q.Get("event"),
		days: proPlayersDefaultDays, minRounds: tacticalplan.BaselineMinRounds, limit: community.ProPlayersDefaultLimit,
	}
	if req.mapName != "" && (!community.MapPattern.MatchString(req.mapName) || req.mapName == community.AllMaps) {
		return req, errors.New("map must be a map name like de_mirage")
	}
	switch strings.ToUpper(q.Get("side")) {
	case "":
	case "CT":
		req.side = tacticalplan.SideCT
	case "T":
		req.side = tacticalplan.SideT
	default:
		return req, errors.New("side must be CT or T")
	}
	var err error
	if req.side, err = tacticalplan.RoleFilterSide(req.role, req.mapRole, req.side, req.mapName != ""); err != nil {
		return req, err
	}
	if req.metric == "" {
		req.metric = tacticalplan.MetricRating
	}
	if !slices.Contains(tacticalplan.LeaderboardMetrics, req.metric) {
		return req, errors.New("metric must be one of " + strings.Join(tacticalplan.LeaderboardMetrics, ", "))
	}
	maxTop := community.ProPlayersTops[len(community.ProPlayersTops)-1]
	ints := []struct {
		name     string
		into     *int
		min, max int
	}{
		{"days", &req.days, 1, proPlayersMaxDays}, {"top", &req.top, 1, maxTop}, {"team_top", &req.teamTop, 1, maxTop},
		{"min_rounds", &req.minRounds, 1, 10000}, {"limit", &req.limit, 1, proPlayersMaxLimit},
	}
	for _, f := range ints {
		raw := q.Get(f.name)
		if raw == "" {
			continue
		}
		n, err := strconv.Atoi(raw)
		if err != nil || n < f.min || n > f.max {
			return req, fmt.Errorf("%s must be an integer from %d to %d", f.name, f.min, f.max)
		}
		*f.into = n
	}
	return req, nil
}

// handleProPlayers serves GET /v1/pro-players: the pro players of the indexed
// maps ranked on one metric, over one map or all, by side, role, position,
// period, event and the HLTV rank of their team and of the team they faced.
// Without an index yet it answers an empty board.
func (a *api) handleProPlayers(w http.ResponseWriter, r *http.Request) {
	req, err := parseProPlayersRequest(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	board, err := a.proPlayersBoard(r.Context(), req, time.Now())
	if err != nil {
		a.internal(w, "pro players", err)
		return
	}
	body, err := json.Marshal(board)
	if err != nil {
		a.internal(w, "pro players", err)
		return
	}
	etag := etagOf(body)
	w.Header().Set("ETag", etag)
	w.Header().Set("Cache-Control", "public, max-age=3600")
	if match := r.Header.Get("If-None-Match"); match != "" && strings.Contains(match, etag) {
		w.WriteHeader(http.StatusNotModified)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write(body)
}

// proPlayersMaps lists the maps to read: the named one, or every indexed map.
func (a *api) proPlayersMaps(ctx context.Context, mapName string) ([]string, error) {
	if mapName != "" {
		return []string{mapName}, nil
	}
	raw, _, err := a.store.referenceBody(ctx, proPlayersIndexKey)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var index proPlayersIndex
	if err := json.Unmarshal(raw, &index); err != nil {
		return nil, err
	}
	return index.Maps, nil
}

func (a *api) proPlayersBoard(ctx context.Context, req proPlayersRequest, now time.Time) (community.ProPlayersBoard, error) {
	mapNames, err := a.proPlayersMaps(ctx, req.mapName)
	if err != nil {
		return community.ProPlayersBoard{}, err
	}
	matches := []tacticalplan.ProPlayersMatch{}
	var builtAt time.Time
	for _, name := range mapNames {
		p, err := a.proPlayersFor(ctx, name)
		// A map without pro maps is an empty board, not a failure.
		if errors.Is(err, sql.ErrNoRows) {
			continue
		}
		if err != nil {
			return community.ProPlayersBoard{}, err
		}
		matches = append(matches, p.Matches...)
		builtAt = p.BuiltAt
	}
	res := tacticalplan.ProPlayersBoard(matches,
		tacticalplan.ProPlayersQuery{Side: req.side, Days: req.days, Now: now, Top: req.top, TeamTop: req.teamTop, Event: req.event, Roles: req.mapName != ""},
		tacticalplan.LeaderboardOptions{Metric: req.metric, MinRounds: req.minRounds, Limit: req.limit, Role: req.role, MapRole: req.mapRole, MapRoleSide: req.side})
	board := community.ProPlayersBoard{
		Map: req.mapName, Side: string(req.side), Role: req.role, MapRole: req.mapRole, Event: req.event, Days: req.days, Top: req.top, TeamTop: req.teamTop,
		Metric: req.metric, HigherIsBetter: !tacticalplan.LowerIsBetter(req.metric), MinRounds: req.minRounds, Maps: res.Maps, Rounds: res.Rounds,
		Events: res.Events, MapRoles: res.MapRoles, Players: res.Rows,
	}
	if !builtAt.IsZero() {
		board.BuiltAt = builtAt.Format(time.RFC3339)
	}
	return board, nil
}
