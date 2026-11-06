package main

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/rechedev9/cliphub/internal/community"
	"github.com/rechedev9/cliphub/internal/tacticalplan"
)

const referenceIndexKey = "index"

func etagOf(body []byte) string {
	sum := sha256.Sum256(body)
	return `"` + hex.EncodeToString(sum[:12]) + `"`
}

// mapReference is GET /v1/pro-reference?map=<map>: one map's reference and the matches behind it.
type mapReference struct {
	ID          string                           `json:"id"`
	Description string                           `json:"description"`
	UpdatedAt   string                           `json:"updated_at"`
	Map         string                           `json:"map"`
	Matches     []tacticalplan.ProReferenceMatch `json:"matches"`
	Baseline    tacticalplan.Baseline            `json:"baseline"`
	Players     []tacticalplan.ProReferenceRow   `json:"players"`
}

// rebuildReference measures the reference from every indexed demo and stores
// the full document, the index and one payload per map.
func rebuildReference(ctx context.Context, s *store) (community.ProReferenceIndex, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT event, summary FROM pro_demos WHERE status = 'ok' ORDER BY dataset, path`)
	if err != nil {
		return community.ProReferenceIndex{}, err
	}
	defer rows.Close()
	build := tacticalplan.ProReferenceBuild{UpdatedAt: time.Now().UTC().Format(time.RFC3339)}
	events := []string{}
	for rows.Next() {
		var event, raw string
		if err := rows.Scan(&event, &raw); err != nil {
			return community.ProReferenceIndex{}, err
		}
		var d tacticalplan.ProDemoSummary
		if err := json.Unmarshal([]byte(raw), &d); err != nil {
			return community.ProReferenceIndex{}, fmt.Errorf("decode summary: %w", err)
		}
		build.Demos = append(build.Demos, d)
		if !slices.Contains(events, event) {
			events = append(events, event)
		}
	}
	if err := rows.Err(); err != nil {
		return community.ProReferenceIndex{}, err
	}
	if len(build.Demos) == 0 {
		return community.ProReferenceIndex{}, errors.New("no demo indexed yet")
	}
	build.ID = "server-" + time.Now().UTC().Format("20060102")
	build.Description = fmt.Sprintf("%d %s de %s.", len(build.Demos), plural(len(build.Demos), "mapa profesional", "mapas profesionales"), joinSpanish(events))
	ref, err := tacticalplan.BuildProReference(build)
	if err != nil {
		return community.ProReferenceIndex{}, err
	}
	payloads, index, err := referencePayloads(ref)
	if err != nil {
		return community.ProReferenceIndex{}, err
	}
	return index, s.replaceReference(ctx, payloads)
}

// referencePayloads renders the stored JSON documents of one reference.
func referencePayloads(ref tacticalplan.ProReference) (map[string][]byte, community.ProReferenceIndex, error) {
	index := community.ProReferenceIndex{
		ID: ref.ID, Description: ref.Description, UpdatedAt: ref.UpdatedAt,
		Matches: ref.Baseline.Matches, Rounds: ref.Baseline.Rounds, Players: ref.Baseline.Players, Maps: []community.ProReferenceMapInfo{},
	}
	payloads := map[string][]byte{}
	for name, m := range ref.Maps {
		index.Maps = append(index.Maps, community.ProReferenceMapInfo{Map: name, Matches: m.Baseline.Matches, Rounds: m.Baseline.Rounds, Players: m.Baseline.Players})
		matches := []tacticalplan.ProReferenceMatch{}
		for _, match := range ref.Matches {
			if match.Map == name {
				matches = append(matches, match)
			}
		}
		body, err := json.Marshal(mapReference{ID: ref.ID, Description: ref.Description, UpdatedAt: ref.UpdatedAt, Map: name, Matches: matches, Baseline: m.Baseline, Players: m.Players})
		if err != nil {
			return nil, index, err
		}
		payloads[name] = body
	}
	sort.Slice(index.Maps, func(i, j int) bool {
		if index.Maps[i].Matches != index.Maps[j].Matches {
			return index.Maps[i].Matches > index.Maps[j].Matches
		}
		return index.Maps[i].Map < index.Maps[j].Map
	})
	full, err := json.Marshal(ref)
	if err != nil {
		return nil, index, err
	}
	payloads[community.AllMaps] = full
	indexBody, err := json.Marshal(index)
	if err != nil {
		return nil, index, err
	}
	payloads[referenceIndexKey] = indexBody
	return payloads, index, nil
}

// handleProReference serves GET /v1/pro-reference: the index without a map,
// the full reference for map=all, else one map.
func (a *api) handleProReference(w http.ResponseWriter, r *http.Request) {
	key := r.URL.Query().Get("map")
	if key == "" {
		key = referenceIndexKey
	} else if !community.MapPattern.MatchString(key) {
		writeError(w, http.StatusBadRequest, "map must be all or a map name like de_mirage")
		return
	}
	body, etag, err := a.store.referenceBody(r.Context(), key)
	if errors.Is(err, sql.ErrNoRows) {
		writeError(w, http.StatusNotFound, "no pro reference for this map")
		return
	}
	if err != nil {
		a.internal(w, "pro reference", err)
		return
	}
	w.Header().Set("ETag", etag)
	w.Header().Set("Cache-Control", "public, max-age=3600")
	if match := r.Header.Get("If-None-Match"); match != "" && strings.Contains(match, etag) {
		w.WriteHeader(http.StatusNotModified)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write(body)
}

func plural(n int, one, many string) string {
	if n == 1 {
		return one
	}
	return many
}

// joinSpanish joins names as "a, b y c".
func joinSpanish(names []string) string {
	switch len(names) {
	case 0:
		return "torneos profesionales"
	case 1:
		return names[0]
	}
	return strings.Join(names[:len(names)-1], ", ") + " y " + names[len(names)-1]
}
