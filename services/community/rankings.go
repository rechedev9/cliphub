package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/http"
	"time"

	"github.com/rechedev9/cliphub/internal/community"
	"github.com/rechedev9/cliphub/internal/tacticalplan"
)

const (
	maxSubmitBytes = 16 << 10
	// maxSubmitsPerDay bounds one install; a resubmission replaces, so more adds nothing.
	maxSubmitsPerDay = 12
	// maxInstalls caps how many installs the table may hold.
	maxInstalls = 100000
	// rankingRetention drops installs that stopped submitting.
	rankingRetention  = 180 * 24 * time.Hour
	maxRoundsPerMatch = 80
)

// metricBounds are the values a real player can reach; anything outside is bogus.
var metricBounds = map[string][2]float64{
	tacticalplan.MetricKPR:           {0, 3},
	tacticalplan.MetricDPR:           {0, 1},
	tacticalplan.MetricADR:           {0, 250},
	tacticalplan.MetricKAST:          {0, 100},
	tacticalplan.MetricOpeningWin:    {0, 100},
	tacticalplan.MetricTrade:         {0, 100},
	tacticalplan.MetricUtilityDamage: {0, 150},
	tacticalplan.MetricFlashAssists:  {0, 2},
	tacticalplan.MetricWPA:           {-0.5, 0.5},
}

var errTooManySubmits = errors.New("too many submissions today")

// validateSubmission rejects malformed ids, unknown or repeated maps and implausible values.
func validateSubmission(sub community.RankingSubmission) error {
	if !community.InstallIDPattern.MatchString(sub.InstallID) {
		return errors.New("install_id must be a random version 4 UUID")
	}
	if len(sub.Maps) > community.MaxRankingMaps {
		return fmt.Errorf("at most %d maps", community.MaxRankingMaps)
	}
	seen := map[string]bool{}
	for _, m := range sub.Maps {
		if !community.MapPattern.MatchString(m.Map) || seen[m.Map] {
			return fmt.Errorf("map %q is invalid or repeated", m.Map)
		}
		seen[m.Map] = true
		if m.Rounds < community.MinRankingRounds || m.Rounds > 200000 || m.Matches < 1 || m.Matches > m.Rounds || m.Rounds > m.Matches*maxRoundsPerMatch {
			return fmt.Errorf("map %s: rounds and matches are implausible", m.Map)
		}
		if len(m.Values) == 0 {
			return fmt.Errorf("map %s has no values", m.Map)
		}
		for metric, v := range m.Values {
			b, ok := metricBounds[metric]
			if !ok {
				return fmt.Errorf("unknown metric %q", metric)
			}
			if math.IsNaN(v) || math.IsInf(v, 0) || v < b[0] || v > b[1] {
				return fmt.Errorf("map %s: %s is out of range", m.Map, metric)
			}
		}
	}
	return nil
}

// handleSubmit serves POST /v1/rankings/submit: it replaces the install's lines.
func (a *api) handleSubmit(w http.ResponseWriter, r *http.Request) {
	if !a.limiter.allow(clientIP(r)) {
		writeError(w, http.StatusTooManyRequests, "too many requests")
		return
	}
	var sub community.RankingSubmission
	r.Body = http.MaxBytesReader(w, r.Body, maxSubmitBytes)
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&sub); err != nil {
		writeError(w, http.StatusBadRequest, "invalid submission JSON")
		return
	}
	if err := validateSubmission(sub); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	err := a.store.replaceRankings(r.Context(), sub, time.Now().UTC())
	switch {
	case errors.Is(err, errTooManySubmits):
		writeError(w, http.StatusTooManyRequests, err.Error())
	case err != nil:
		a.internal(w, "submit", err)
	default:
		writeJSON(w, http.StatusOK, map[string]int{"maps": len(sub.Maps)})
	}
}

// handleDelete serves DELETE /v1/rankings/installs/{id}.
func (a *api) handleDelete(w http.ResponseWriter, r *http.Request) {
	if !a.limiter.allow(clientIP(r)) {
		writeError(w, http.StatusTooManyRequests, "too many requests")
		return
	}
	id := r.PathValue("id")
	if !community.InstallIDPattern.MatchString(id) {
		writeError(w, http.StatusBadRequest, "install id must be a random version 4 UUID")
		return
	}
	n, err := a.store.deleteInstall(r.Context(), id)
	if err != nil {
		a.internal(w, "delete", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]int64{"deleted": n})
}

// handleRankings serves GET /v1/rankings?map=&metric=: the quantiles of each
// metric, empty until enough installs submitted the map.
func (a *api) handleRankings(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	mapName := q.Get("map")
	if mapName == "" {
		mapName = community.AllMaps
	}
	if !community.MapPattern.MatchString(mapName) {
		writeError(w, http.StatusBadRequest, "map must be all or a map name like de_mirage")
		return
	}
	metric := q.Get("metric")
	if _, ok := metricBounds[metric]; metric != "" && !ok {
		writeError(w, http.StatusBadRequest, "unknown metric")
		return
	}
	values, err := a.store.rankingValues(r.Context(), mapName)
	if err != nil {
		a.internal(w, "rankings", err)
		return
	}
	w.Header().Set("Cache-Control", "public, max-age=300")
	writeJSON(w, http.StatusOK, distributions(mapName, metric, values))
}

// distributions turns each metric's values into its quantile table once enough installs have it.
func distributions(mapName, metric string, values map[string][]float64) community.RankingsResponse {
	out := community.RankingsResponse{Map: mapName, MinInstalls: community.MinRankingInstalls, Metrics: map[string]community.RankingDistribution{}}
	for _, m := range tacticalplan.ReferenceMetrics {
		if metric != "" && m != metric {
			continue
		}
		d := community.RankingDistribution{Installs: len(values[m])}
		if d.Installs >= community.MinRankingInstalls {
			d.Quantiles = community.Quantiles(values[m])
		}
		out.Metrics[m] = d
	}
	return out
}

func (s *store) replaceRankings(ctx context.Context, sub community.RankingSubmission, at time.Time) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	day := at.Format(time.DateOnly)
	var count int
	err = tx.QueryRowContext(ctx, `SELECT count FROM ranking_submits WHERE install_id = ? AND day = ?`, sub.InstallID, day).Scan(&count)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	if count >= maxSubmitsPerDay {
		return errTooManySubmits
	}
	var known, installs int
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM ranking_lines WHERE install_id = ?`, sub.InstallID).Scan(&known); err != nil {
		return err
	}
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(DISTINCT install_id) FROM ranking_lines`).Scan(&installs); err != nil {
		return err
	}
	if known == 0 && installs >= maxInstalls {
		return errTooManySubmits
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO ranking_submits (install_id, day, count) VALUES (?, ?, 1)
		ON CONFLICT (install_id, day) DO UPDATE SET count = count + 1`, sub.InstallID, day); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM ranking_lines WHERE install_id = ?`, sub.InstallID); err != nil {
		return err
	}
	stamp := at.Format(time.RFC3339)
	for _, m := range sub.Maps {
		vals, err := json.Marshal(m.Values)
		if err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO ranking_lines (install_id, map, rounds, matches, vals, updated_at) VALUES (?, ?, ?, ?, ?, ?)`,
			sub.InstallID, m.Map, m.Rounds, m.Matches, string(vals), stamp); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (s *store) deleteInstall(ctx context.Context, id string) (int64, error) {
	res, err := s.db.ExecContext(ctx, `DELETE FROM ranking_lines WHERE install_id = ?`, id)
	if err != nil {
		return 0, err
	}
	if _, err := s.db.ExecContext(ctx, `DELETE FROM ranking_submits WHERE install_id = ?`, id); err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

// rankingValues is every install's value of each metric on one map.
func (s *store) rankingValues(ctx context.Context, mapName string) (map[string][]float64, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT vals FROM ranking_lines WHERE map = ?`, mapName)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string][]float64{}
	for rows.Next() {
		var raw string
		if err := rows.Scan(&raw); err != nil {
			return nil, err
		}
		var vals map[string]float64
		if err := json.Unmarshal([]byte(raw), &vals); err != nil {
			continue
		}
		for m, v := range vals {
			out[m] = append(out[m], v)
		}
	}
	return out, rows.Err()
}

// purgeRankings drops lines not refreshed within the retention and old submit counters.
func (s *store) purgeRankings(ctx context.Context, at time.Time) (int64, error) {
	res, err := s.db.ExecContext(ctx, `DELETE FROM ranking_lines WHERE updated_at < ?`, at.Add(-rankingRetention).Format(time.RFC3339))
	if err != nil {
		return 0, err
	}
	if _, err := s.db.ExecContext(ctx, `DELETE FROM ranking_submits WHERE day < ?`, at.AddDate(0, 0, -2).Format(time.DateOnly)); err != nil {
		return 0, err
	}
	return res.RowsAffected()
}
