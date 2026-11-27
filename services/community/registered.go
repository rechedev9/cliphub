package main

import (
	"context"
	"net/http"
	"slices"
	"strconv"
	"strings"
)

const maxRegisteredQuery = 500

type registeredMatches struct {
	Registered []int `json:"registered"`
}

func parseMatchIDs(raw string) ([]int, bool) {
	if raw == "" {
		return nil, false
	}
	parts := strings.Split(raw, ",")
	if len(parts) > maxRegisteredQuery {
		return nil, false
	}
	ids := make([]int, 0, len(parts))
	for _, p := range parts {
		id, err := strconv.Atoi(strings.TrimSpace(p))
		if err != nil || id <= 0 {
			return nil, false
		}
		ids = append(ids, id)
	}
	slices.Sort(ids)
	return slices.Compact(ids), true
}

func (s *store) registeredMatchIDs(ctx context.Context, ids []int) ([]int, error) {
	marks := strings.TrimSuffix(strings.Repeat("?,", len(ids)), ",")
	args := make([]any, len(ids))
	for i, id := range ids {
		args[i] = id
	}
	rows, err := s.db.QueryContext(ctx, `SELECT DISTINCT match_id FROM pro_match_maps WHERE match_id IN (`+marks+`) ORDER BY match_id`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []int{}
	for rows.Next() {
		var id int
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out = append(out, id)
	}
	return out, rows.Err()
}

func (a *api) handleRegisteredMatches(w http.ResponseWriter, r *http.Request) {
	if a.analysesDir == "" {
		writeError(w, http.StatusNotFound, "no pro match catalog")
		return
	}
	ids, ok := parseMatchIDs(r.URL.Query().Get("ids"))
	if !ok {
		writeError(w, http.StatusBadRequest, "ids must be 1 to "+strconv.Itoa(maxRegisteredQuery)+" HLTV match ids separated by commas")
		return
	}
	found, err := a.store.registeredMatchIDs(r.Context(), ids)
	if err != nil {
		a.internal(w, "pro_match_ids", err)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, http.StatusOK, registeredMatches{Registered: found})
}
