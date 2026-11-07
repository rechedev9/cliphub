package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/rechedev9/cliphub/internal/prolineups"
	"github.com/rechedev9/cliphub/internal/tacticalplan"
)

// lineupsIndexKey is the stored key of the list of maps with a lineup index.
const lineupsIndexKey = "lineups:index"

// lineupsMaxAge is how old the lineup index may get before an index run with
// no new analyses rebuilds it anyway.
const lineupsMaxAge = 24 * time.Hour

// LineupsMapInfo is one map in the lineups index payload.
type lineupsMapInfo struct {
	Map     string `json:"map"`
	Lineups int    `json:"lineups"`
	Combos  int    `json:"combos"`
	Maps    int    `json:"maps"`
	Recent  int    `json:"recent_maps"`
}

type lineupsIndex struct {
	Version int              `json:"version"`
	BuiltAt time.Time        `json:"built_at"`
	Maps    []lineupsMapInfo `json:"maps"`
}

// buildLineups reads every archived analysis, groups the throws by map and
// stores one lineup index per map plus the list of maps.
func buildLineups(ctx context.Context, s *store, analysesDir string, now time.Time) (lineupsIndex, error) {
	files, err := filepath.Glob(filepath.Join(analysesDir, "*"+analysisSuffix))
	if err != nil {
		return lineupsIndex{}, err
	}
	slices.Sort(files)
	parts := []mapPart{}
	geometry := map[string]tacticalplan.MapGeometry{}
	for _, name := range files {
		if ctx.Err() != nil {
			return lineupsIndex{}, ctx.Err()
		}
		h, blob, err := readAnalysis(name)
		if err != nil {
			log.Printf("community stage=lineups class=unreadable file=%s error=%v", filepath.Base(name), err)
			continue
		}
		match, entry, _ := analysisName(filepath.Base(name))
		key := match + "/" + mapKey(entry)
		meta := prolineups.MatchMeta{Source: hltvDataset, MatchID: match, MapKey: key, Event: h.Match.Event}
		if t, err := time.Parse(time.RFC3339, h.Match.PlayedAt); err == nil {
			meta.PlayedAt = t
		}
		if g := h.Document.Geometry; g.SampleCount > geometry[h.Document.Demo.Map].SampleCount {
			geometry[h.Document.Demo.Map] = g
		}
		parts = append(parts, newMapPart(key, prolineups.ExtractThrows(h.Document, blob, meta)))
	}
	byMap := map[string][]prolineups.Throw{}
	for _, t := range joinParts(parts) {
		byMap[t.Map] = append(byMap[t.Map], t)
	}
	// A missing or unreadable folder must not replace the served lineups with nothing.
	if len(byMap) == 0 {
		return lineupsIndex{}, fmt.Errorf("no throws read from %d analyses in %s; the stored lineups stay", len(files), analysesDir)
	}
	index := lineupsIndex{Version: prolineups.IndexVersion, BuiltAt: now.UTC(), Maps: []lineupsMapInfo{}}
	payloads := map[string][]byte{}
	for m, throws := range byMap {
		idx := prolineups.BuildIndex(m, throws, now)
		if g, ok := geometry[m]; ok {
			idx.Geometry = &g
		}
		body, err := json.Marshal(idx)
		if err != nil {
			return lineupsIndex{}, err
		}
		payloads["lineups:"+m] = body
		index.Maps = append(index.Maps, lineupsMapInfo{Map: m, Lineups: len(idx.Lineups), Combos: len(idx.Combos), Maps: idx.Maps.All, Recent: idx.Maps.Recent})
		log.Printf("community stage=lineups class=map map=%s throws=%d lineups=%d combos=%d maps=%d", m, len(throws), len(idx.Lineups), len(idx.Combos), idx.Maps.All)
	}
	slices.SortFunc(index.Maps, func(a, b lineupsMapInfo) int { return b.Maps - a.Maps })
	body, err := json.Marshal(index)
	if err != nil {
		return lineupsIndex{}, err
	}
	payloads[lineupsIndexKey] = body
	return index, s.replacePrefixed(ctx, "lineups:", payloads)
}

// mapPart is the throws of one demo of a map; first is its lowest game round.
type mapPart struct {
	key    string
	first  int
	throws []prolineups.Throw
}

func newMapPart(key string, throws []prolineups.Throw) mapPart {
	p := mapPart{key: key, throws: throws}
	for _, t := range throws {
		if p.first == 0 || t.Round < p.first {
			p.first = t.Round
		}
	}
	return p
}

// joinParts keeps each game round of a map from one part: a restart part replays
// the round the server went down in, and the later part's copy is the one played out.
func joinParts(parts []mapPart) []prolineups.Throw {
	owner := map[string]int{}
	roundOf := func(p mapPart, t prolineups.Throw) string { return fmt.Sprintf("%s|%d", p.key, t.Round) }
	for i, p := range parts {
		for _, t := range p.throws {
			k := roundOf(p, t)
			if j, ok := owner[k]; !ok || p.first > parts[j].first {
				owner[k] = i
			}
		}
	}
	out := []prolineups.Throw{}
	for i, p := range parts {
		for _, t := range p.throws {
			if owner[roundOf(p, t)] == i {
				out = append(out, t)
			}
		}
	}
	return out
}

// replacePrefixed swaps the stored payloads whose key starts with prefix.
func (s *store) replacePrefixed(ctx context.Context, prefix string, payloads map[string][]byte) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.ExecContext(ctx, `DELETE FROM pro_reference WHERE map LIKE ? ESCAPE '\'`, strings.ReplaceAll(prefix, "_", `\_`)+"%"); err != nil {
		return err
	}
	at := now()
	for key, body := range payloads {
		if _, err := tx.ExecContext(ctx, `INSERT INTO pro_reference (map, body, etag, built_at) VALUES (?, ?, ?, ?)`, key, body, etagOf(body), at); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func writeLineupFiles(ctx context.Context, s *store, outDir string) error {
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return err
	}
	rows, err := s.db.QueryContext(ctx, `SELECT map, body FROM pro_reference WHERE map LIKE 'lineups:%'`)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var key string
		var body []byte
		if err := rows.Scan(&key, &body); err != nil {
			return err
		}
		if err := os.WriteFile(filepath.Join(outDir, strings.TrimPrefix(key, "lineups:")+".json"), body, 0o644); err != nil {
			return err
		}
	}
	return rows.Err()
}
