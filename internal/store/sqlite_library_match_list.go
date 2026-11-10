package store

import (
	"context"
	"fmt"
	"strings"

	"github.com/rechedev9/cliphub/internal/library"
)

func (r *SQLiteLibraryRepository) ListMatches(ctx context.Context, filter library.MatchFilter) ([]library.Match, error) {
	var where []string
	var args []any
	if filter.Map != "" {
		where = append(where, `m.map = ?`)
		args = append(args, filter.Map)
	}
	if filter.TeamID != nil {
		where = append(where, `EXISTS (SELECT 1 FROM library_match_teams mt WHERE mt.match_id = m.id AND mt.team_id = ?)`)
		args = append(args, filter.TeamID.String())
	}
	if filter.SteamID != "" {
		where = append(where, `EXISTS (SELECT 1 FROM library_match_players mp WHERE mp.match_id = m.id AND mp.steam_id = ?)`)
		args = append(args, filter.SteamID)
	}
	if filter.SeriesID != nil {
		where = append(where, `m.series_id = ?`)
		args = append(args, filter.SeriesID.String())
	}
	if filter.Source != "" {
		where = append(where, `d.source = ?`)
		args = append(args, string(filter.Source))
	}
	if filter.Tag != "" {
		where = append(where, `EXISTS (SELECT 1 FROM library_demo_tags t WHERE t.demo_id = m.demo_id AND t.tag_key = ?)`)
		args = append(args, libraryTagKey(library.CleanName(filter.Tag)))
	}
	if filter.TeamID != nil && len(filter.Lineup) > 0 {
		marks := strings.TrimSuffix(strings.Repeat("?,", len(filter.Lineup)), ",")
		teamPlayers := `SELECT 1 FROM library_match_players mp JOIN library_match_teams mt ON mt.match_id = mp.match_id AND mt.team_key = mp.team_key
			WHERE mp.match_id = m.id AND mt.team_id = ? AND `
		where = append(where, `EXISTS (`+teamPlayers+`mp.steam_id IN (`+marks+`))`,
			`NOT EXISTS (`+teamPlayers+`mp.steam_id NOT IN (`+marks+`) AND mp.rounds >= m.round_count)`)
		for range 2 {
			args = append(args, filter.TeamID.String())
			for _, id := range filter.Lineup {
				args = append(args, id)
			}
		}
	}
	if filter.From != nil {
		where = append(where, `m.played_at >= ?`)
		args = append(args, millis(*filter.From))
	}
	if filter.To != nil {
		where = append(where, `m.played_at <= ?`)
		args = append(args, millis(*filter.To))
	}
	query := libraryMatchColumns
	if len(where) > 0 {
		query += ` WHERE ` + strings.Join(where, ` AND `)
	}
	query += ` ORDER BY m.played_at DESC, m.id ASC LIMIT ? OFFSET ?`
	args = append(args, library.ClampLimit(filter.Limit), max(filter.Offset, 0))
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list matches: %w", err)
	}
	matches := []library.Match{}
	for rows.Next() {
		m, err := scanLibraryMatch(rows)
		if err != nil {
			_ = rows.Close()
			return nil, err
		}
		matches = append(matches, m)
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if err := r.attachMatchTeams(ctx, r.db, matches); err != nil {
		return nil, err
	}
	return matches, nil
}
