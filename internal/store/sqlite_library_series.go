package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/google/uuid"

	"github.com/rechedev9/cliphub/internal/library"
)

const librarySeriesColumns = `SELECT s.id, s.format, s.event, s.veto, s.veto_steps, s.team_a, ta.name, s.team_b, tb.name, s.created_at, s.updated_at
	FROM library_series s
	LEFT JOIN library_teams ta ON ta.id = s.team_a
	LEFT JOIN library_teams tb ON tb.id = s.team_b`

// seriesTeamColumns returns the team_a and team_b values of a series.
func seriesTeamColumns(ids []uuid.UUID) (any, any) {
	if len(ids) != 2 {
		return nil, nil
	}
	return ids[0].String(), ids[1].String()
}

func seriesTeamsExist(ctx context.Context, q libraryQuerier, ids []uuid.UUID) error {
	for _, id := range ids {
		var n int
		if err := q.QueryRowContext(ctx, `SELECT COUNT(*) FROM library_teams WHERE id = ?`, id.String()).Scan(&n); err != nil {
			return err
		}
		if n == 0 {
			return library.ErrNotFound
		}
	}
	return nil
}

func (r *SQLiteLibraryRepository) CreateSeries(ctx context.Context, s *library.Series) error {
	if s.ID == uuid.Nil {
		s.ID = uuid.New()
	}
	if s.Vetoes == nil {
		s.Vetoes = []library.VetoStep{}
	}
	steps, err := json.Marshal(s.Vetoes)
	if err != nil {
		return err
	}
	now := libraryNow()
	teamA, teamB := seriesTeamColumns(s.TeamIDs)
	err = r.inTx(ctx, func(tx *sql.Tx) error {
		if err := seriesTeamsExist(ctx, tx, s.TeamIDs); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO library_series (id, format, event, veto, veto_steps, team_a, team_b, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			s.ID.String(), s.Format, s.Event, s.Veto, string(steps), teamA, teamB, millis(now), millis(now)); err != nil {
			return fmt.Errorf("insert series: %w", err)
		}
		return nil
	})
	if err != nil {
		return err
	}
	stored, err := r.GetSeries(ctx, s.ID)
	if err != nil {
		return err
	}
	*s = stored
	return nil
}

func scanLibrarySeries(row sqlScanner) (library.Series, error) {
	var s library.Series
	var id, steps string
	var teamA, nameA, teamB, nameB sql.NullString
	var created, updated int64
	if err := row.Scan(&id, &s.Format, &s.Event, &s.Veto, &steps, &teamA, &nameA, &teamB, &nameB, &created, &updated); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return library.Series{}, library.ErrNotFound
		}
		return library.Series{}, err
	}
	parsed, err := uuid.Parse(id)
	if err != nil {
		return library.Series{}, err
	}
	s.ID = parsed
	s.CreatedAt, s.UpdatedAt = fromMillis(created), fromMillis(updated)
	s.MatchIDs = []uuid.UUID{}
	s.Vetoes = []library.VetoStep{}
	if err := json.Unmarshal([]byte(steps), &s.Vetoes); err != nil {
		return library.Series{}, fmt.Errorf("decode veto steps: %w", err)
	}
	s.TeamIDs, s.Teams = []uuid.UUID{}, []library.SeriesTeam{}
	for _, slot := range []struct {
		name     string
		id, team sql.NullString
	}{{library.SeriesTeamA, teamA, nameA}, {library.SeriesTeamB, teamB, nameB}} {
		teamID, err := parseNullUUID(slot.id)
		if err != nil {
			return library.Series{}, err
		}
		if teamID != nil {
			s.TeamIDs = append(s.TeamIDs, *teamID)
			s.Teams = append(s.Teams, library.SeriesTeam{Slot: slot.name, TeamID: *teamID, Name: slot.team.String})
		}
	}
	return s, nil
}

func (r *SQLiteLibraryRepository) attachSeriesMatches(ctx context.Context, series []library.Series) error {
	if len(series) == 0 {
		return nil
	}
	args := make([]any, len(series))
	index := make(map[string]int, len(series))
	for i, s := range series {
		args[i] = s.ID.String()
		index[s.ID.String()] = i
	}
	rows, err := r.db.QueryContext(ctx, `SELECT series_id, id FROM library_matches WHERE series_id IN (`+placeholders(len(args))+`)
		ORDER BY series_id, played_at ASC, id ASC`, args...)
	if err != nil {
		return fmt.Errorf("query series matches: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var seriesID, matchID string
		if err := rows.Scan(&seriesID, &matchID); err != nil {
			return err
		}
		id, err := uuid.Parse(matchID)
		if err != nil {
			return err
		}
		if i, ok := index[seriesID]; ok {
			series[i].MatchIDs = append(series[i].MatchIDs, id)
		}
	}
	return rows.Err()
}

func (r *SQLiteLibraryRepository) GetSeries(ctx context.Context, id uuid.UUID) (library.Series, error) {
	s, err := scanLibrarySeries(r.db.QueryRowContext(ctx, librarySeriesColumns+` WHERE s.id = ?`, id.String()))
	if err != nil {
		return library.Series{}, err
	}
	series := []library.Series{s}
	if err := r.attachSeriesMatches(ctx, series); err != nil {
		return library.Series{}, err
	}
	return series[0], nil
}

func (r *SQLiteLibraryRepository) ListSeries(ctx context.Context, filter library.SeriesFilter) ([]library.Series, error) {
	where, args := "", []any{}
	if filter.TeamID != nil {
		where = ` WHERE s.team_a = ? OR s.team_b = ?`
		args = append(args, filter.TeamID.String(), filter.TeamID.String())
	}
	args = append(args, library.ClampLimit(filter.Limit), max(filter.Offset, 0))
	rows, err := r.db.QueryContext(ctx, librarySeriesColumns+where+` ORDER BY s.created_at DESC, s.id ASC LIMIT ? OFFSET ?`, args...)
	if err != nil {
		return nil, fmt.Errorf("list series: %w", err)
	}
	series := []library.Series{}
	for rows.Next() {
		s, err := scanLibrarySeries(rows)
		if err != nil {
			_ = rows.Close()
			return nil, err
		}
		series = append(series, s)
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if err := r.attachSeriesMatches(ctx, series); err != nil {
		return nil, err
	}
	return series, nil
}

// UpdateSeries stores an edit the caller already cleaned with CleanSeries.
func (r *SQLiteLibraryRepository) UpdateSeries(ctx context.Context, id uuid.UUID, s library.Series) (library.Series, error) {
	if s.Vetoes == nil {
		s.Vetoes = []library.VetoStep{}
	}
	steps, err := json.Marshal(s.Vetoes)
	if err != nil {
		return library.Series{}, err
	}
	teamA, teamB := seriesTeamColumns(s.TeamIDs)
	err = r.inTx(ctx, func(tx *sql.Tx) error {
		if err := seriesTeamsExist(ctx, tx, s.TeamIDs); err != nil {
			return err
		}
		res, err := tx.ExecContext(ctx, `UPDATE library_series SET format = ?, event = ?, veto = ?, veto_steps = ?, team_a = ?, team_b = ?, updated_at = ? WHERE id = ?`,
			s.Format, s.Event, s.Veto, string(steps), teamA, teamB, millis(libraryNow()), id.String())
		if err != nil {
			return fmt.Errorf("update series: %w", err)
		}
		if n, err := res.RowsAffected(); err != nil {
			return err
		} else if n == 0 {
			return library.ErrNotFound
		}
		return nil
	})
	if err != nil {
		return library.Series{}, err
	}
	return r.GetSeries(ctx, id)
}

// DeleteSeries removes a series; its matches stay and lose the link.
func (r *SQLiteLibraryRepository) DeleteSeries(ctx context.Context, id uuid.UUID) error {
	res, err := r.db.ExecContext(ctx, `DELETE FROM library_series WHERE id = ?`, id.String())
	if err != nil {
		return fmt.Errorf("delete series: %w", err)
	}
	if n, err := res.RowsAffected(); err != nil {
		return err
	} else if n == 0 {
		return library.ErrNotFound
	}
	return nil
}
