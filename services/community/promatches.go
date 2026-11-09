package main

import (
	"cmp"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/rechedev9/cliphub/internal/community"
)

// proFileWriteTimeout covers sending one map analysis, about a megabyte, on a slow link.
const proFileWriteTimeout = 2 * time.Minute

// proMapRow is one analysed map of the catalog as pro_match_maps keeps it.
type proMapRow struct {
	matchID  int
	entry    string
	file     string
	size     int64
	modTime  string
	sha256   string
	mapName  string
	rounds   int
	version  int
	event    string
	playedAt time.Time
	teams    []community.ProTeam
	syncedAt time.Time
}

// syncProCatalog brings pro_match_maps in line with the archived analyses: new or
// changed files are read, files gone from the folder leave the catalog.
func syncProCatalog(ctx context.Context, s *store, dir string) (int, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		// A missing folder must not empty a catalog that is being served.
		return 0, err
	}
	known, err := s.proFileStamps(ctx)
	if err != nil {
		return 0, err
	}
	seen := map[string]bool{}
	changed := 0
	for _, e := range entries {
		if ctx.Err() != nil {
			return changed, ctx.Err()
		}
		match, entry, ok := analysisName(e.Name())
		if !e.Type().IsRegular() || !ok {
			continue
		}
		key := match + "/" + entry
		info, err := e.Info()
		if err != nil {
			continue
		}
		stamp := info.ModTime().UTC().Format(time.RFC3339Nano)
		if known[key] == stampOf(info.Size(), stamp) {
			seen[key] = true
			continue
		}
		row, err := readProMapRow(filepath.Join(dir, e.Name()), info)
		if err != nil {
			log.Printf("community stage=pro_matches class=unreadable file=%s error=%v", e.Name(), err)
			continue
		}
		if err := s.saveProMap(ctx, row); err != nil {
			return changed, err
		}
		seen[key] = true
		changed++
	}
	removed, err := s.dropProMapsExcept(ctx, seen)
	return changed + removed, err
}

func stampOf(size int64, modTime string) string { return strconv.FormatInt(size, 10) + "|" + modTime }

func readProMapRow(name string, info os.FileInfo) (proMapRow, error) {
	h, _, err := readAnalysis(name)
	if err != nil {
		return proMapRow{}, err
	}
	if h.Match.MatchID <= 0 || h.Document.Demo.Map == "" || len(h.Document.Rounds) == 0 {
		return proMapRow{}, errors.New("analysis names no match or holds no rounds")
	}
	sum, size, err := hashFile(name)
	if err != nil {
		return proMapRow{}, err
	}
	row := proMapRow{matchID: h.Match.MatchID, entry: h.Entry, file: filepath.Base(name), size: size,
		modTime: info.ModTime().UTC().Format(time.RFC3339Nano), sha256: sum, mapName: h.Document.Demo.Map,
		rounds: len(h.Document.Rounds), version: h.Document.AnalysisVersion, event: h.Match.Event, teams: []community.ProTeam{}}
	if row.event == "" {
		row.event = otherEvents
	}
	row.playedAt = info.ModTime().UTC()
	if t, err := time.Parse(time.RFC3339, h.Match.PlayedAt); err == nil {
		row.playedAt = t.UTC()
	}
	for _, t := range h.Match.Teams {
		team := community.ProTeam{ID: t.ID, Name: t.Name}
		if t.Rank != nil {
			team.Rank = *t.Rank
		}
		row.teams = append(row.teams, team)
	}
	return row, nil
}

func (s *store) proFileStamps(ctx context.Context) (map[string]string, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT match_id, entry, size, mod_time FROM pro_match_maps`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]string{}
	for rows.Next() {
		var match int
		var entry, modTime string
		var size int64
		if err := rows.Scan(&match, &entry, &size, &modTime); err != nil {
			return nil, err
		}
		out[strconv.Itoa(match)+"/"+entry] = stampOf(size, modTime)
	}
	return out, rows.Err()
}

func (s *store) saveProMap(ctx context.Context, r proMapRow) error {
	teams, err := json.Marshal(r.teams)
	if err != nil {
		return err
	}
	_, err = s.db.ExecContext(ctx, `INSERT INTO pro_match_maps (match_id, entry, file, size, mod_time, sha256, map, rounds, analysis_version, event, played_at, teams, synced_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT (match_id, entry) DO UPDATE SET file = excluded.file, size = excluded.size, mod_time = excluded.mod_time,
			sha256 = excluded.sha256, map = excluded.map, rounds = excluded.rounds, analysis_version = excluded.analysis_version,
			event = excluded.event, played_at = excluded.played_at, teams = excluded.teams, synced_at = excluded.synced_at`,
		r.matchID, r.entry, r.file, r.size, r.modTime, r.sha256, r.mapName, r.rounds, r.version, r.event, r.playedAt.Format(time.RFC3339), string(teams), now())
	return err
}

func (s *store) dropProMapsExcept(ctx context.Context, keep map[string]bool) (int, error) {
	stamps, err := s.proFileStamps(ctx)
	if err != nil {
		return 0, err
	}
	removed := 0
	for key := range stamps {
		if keep[key] {
			continue
		}
		match, entry, _ := strings.Cut(key, "/")
		if _, err := s.db.ExecContext(ctx, `DELETE FROM pro_match_maps WHERE match_id = ? AND entry = ?`, match, entry); err != nil {
			return removed, err
		}
		removed++
	}
	return removed, nil
}

// proMapRows reads the catalog maps played at or after since.
func (s *store) proMapRows(ctx context.Context, since time.Time) ([]proMapRow, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT match_id, entry, file, size, sha256, map, rounds, analysis_version, event, played_at, teams, synced_at
		FROM pro_match_maps WHERE played_at >= ?`, since.UTC().Format(time.RFC3339))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []proMapRow{}
	for rows.Next() {
		var r proMapRow
		var playedAt, teams, syncedAt string
		if err := rows.Scan(&r.matchID, &r.entry, &r.file, &r.size, &r.sha256, &r.mapName, &r.rounds, &r.version, &r.event, &playedAt, &teams, &syncedAt); err != nil {
			return nil, err
		}
		r.playedAt, _ = time.Parse(time.RFC3339, playedAt)
		r.syncedAt, _ = time.Parse(time.RFC3339, syncedAt)
		if err := json.Unmarshal([]byte(teams), &r.teams); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func (s *store) proMapFile(ctx context.Context, match int, entry string) (string, string, error) {
	var file, sum string
	err := s.db.QueryRowContext(ctx, `SELECT file, sha256 FROM pro_match_maps WHERE match_id = ? AND entry = ?`, match, entry).Scan(&file, &sum)
	return file, sum, err
}

// proCatalogPage groups the rows into matches, newest first, and applies the query.
func proCatalogPage(rows []proMapRow, q community.ProMatchQuery) community.ProMatchesPage {
	byID := map[int]*community.ProMatch{}
	page := community.ProMatchesPage{Days: q.Days, Page: q.Page, Matches: []community.ProMatch{}}
	for _, r := range rows {
		page.UpdatedAt = maxTime(page.UpdatedAt, r.syncedAt)
		m := byID[r.matchID]
		if m == nil {
			m = &community.ProMatch{ID: r.matchID, Event: r.event, PlayedAt: r.playedAt, Teams: r.teams}
			byID[r.matchID] = m
		}
		m.Maps = append(m.Maps, community.ProMapFile{Entry: strings.TrimSuffix(r.entry, ".dem"), Map: r.mapName, Rounds: r.rounds,
			AnalysisVersion: r.version, Bytes: r.size, SHA256: r.sha256})
	}
	all := make([]community.ProMatch, 0, len(byID))
	for _, m := range byID {
		slices.SortFunc(m.Maps, func(a, b community.ProMapFile) int { return strings.Compare(a.Entry, b.Entry) })
		all = append(all, *m)
	}
	slices.SortFunc(all, func(a, b community.ProMatch) int {
		return cmp.Or(b.PlayedAt.Compare(a.PlayedAt), b.ID-a.ID)
	})
	page.Facets = proFacets(all)
	kept := []community.ProMatch{}
	for _, m := range all {
		if proMatchWanted(m, q) {
			kept = append(kept, m)
		}
	}
	page.Total = len(kept)
	page.Pages = max(1, (len(kept)+community.ProMatchesPageSize-1)/community.ProMatchesPageSize)
	start := min((q.Page-1)*community.ProMatchesPageSize, len(kept))
	page.Matches = append(page.Matches, kept[start:min(start+community.ProMatchesPageSize, len(kept))]...)
	return page
}

func maxTime(a, b time.Time) time.Time {
	if b.After(a) {
		return b
	}
	return a
}

func proMatchWanted(m community.ProMatch, q community.ProMatchQuery) bool {
	if q.Event != "" && !strings.Contains(strings.ToLower(m.Event), strings.ToLower(q.Event)) {
		return false
	}
	if q.Map != "" && !slices.ContainsFunc(m.Maps, func(f community.ProMapFile) bool { return f.Map == q.Map }) {
		return false
	}
	if q.Team == "" {
		return true
	}
	id, err := strconv.Atoi(q.Team)
	key := community.ProTeamKey(q.Team)
	return slices.ContainsFunc(m.Teams, func(t community.ProTeam) bool {
		if err == nil {
			return t.ID == id
		}
		return key != "" && strings.Contains(community.ProTeamKey(t.Name), key)
	})
}

func proFacets(matches []community.ProMatch) community.ProFacets {
	teams := map[int]*community.ProTeamCount{}
	events, maps := map[string]int{}, map[string]int{}
	for _, m := range matches {
		events[m.Event]++
		inMatch := map[string]bool{}
		for _, f := range m.Maps {
			inMatch[f.Map] = true
		}
		for name := range inMatch {
			maps[name]++
		}
		for _, t := range m.Teams {
			if t.ID <= 0 {
				continue
			}
			if teams[t.ID] == nil {
				// Matches are newest first, so the first name and rank seen are the current ones.
				teams[t.ID] = &community.ProTeamCount{ProTeam: t}
			}
			teams[t.ID].Matches++
		}
	}
	out := community.ProFacets{Teams: []community.ProTeamCount{}, Events: counted(events), Maps: counted(maps)}
	for _, t := range teams {
		out.Teams = append(out.Teams, *t)
	}
	slices.SortFunc(out.Teams, func(a, b community.ProTeamCount) int {
		return cmp.Or(b.Matches-a.Matches, strings.Compare(a.Name, b.Name))
	})
	return out
}

func counted(counts map[string]int) []community.ProNameCount {
	out := make([]community.ProNameCount, 0, len(counts))
	for name, n := range counts {
		out = append(out, community.ProNameCount{Name: name, Matches: n})
	}
	slices.SortFunc(out, func(a, b community.ProNameCount) int {
		return cmp.Or(b.Matches-a.Matches, strings.Compare(a.Name, b.Name))
	})
	return out
}

func (a *api) handleProMatches(w http.ResponseWriter, r *http.Request) {
	if a.analysesDir == "" {
		writeError(w, http.StatusNotFound, "no pro match catalog")
		return
	}
	q, err := community.ParseProMatchQuery(r.URL.Query())
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	rows, err := a.store.proMapRows(r.Context(), time.Now().AddDate(0, 0, -q.Days))
	if err != nil {
		a.internal(w, "pro_matches", err)
		return
	}
	w.Header().Set("Cache-Control", "public, max-age=300")
	writeJSON(w, http.StatusOK, proCatalogPage(rows, q))
}

// handleProMatchFile serves the derived analysis of one map, never a demo.
func (a *api) handleProMatchFile(w http.ResponseWriter, r *http.Request) {
	match, err := strconv.Atoi(r.PathValue("match"))
	entry := r.PathValue("map")
	if a.analysesDir == "" || err != nil || match <= 0 || !community.ProEntryPattern.MatchString(entry) {
		writeError(w, http.StatusNotFound, "no such pro map")
		return
	}
	file, sum, err := a.store.proMapFile(r.Context(), match, entry+".dem")
	if errors.Is(err, sql.ErrNoRows) {
		writeError(w, http.StatusNotFound, "no such pro map")
		return
	}
	if err != nil {
		a.internal(w, "pro_match_file", err)
		return
	}
	f, err := os.Open(filepath.Join(a.analysesDir, filepath.Base(file)))
	if err != nil {
		writeError(w, http.StatusNotFound, "no such pro map")
		return
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil || !info.Mode().IsRegular() {
		writeError(w, http.StatusNotFound, "no such pro map")
		return
	}
	_ = http.NewResponseController(w).SetWriteDeadline(time.Now().Add(proFileWriteTimeout))
	w.Header().Set("Content-Type", "application/zstd")
	w.Header().Set("ETag", `"`+sum[:min(len(sum), 24)]+`"`)
	w.Header().Set("Cache-Control", "public, max-age=604800")
	http.ServeContent(w, r, file, info.ModTime(), f)
}
