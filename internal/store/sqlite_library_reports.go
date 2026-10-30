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

const libraryReportColumns = `SELECT r.id, r.title, r.team_id, t.name, r.match_ids, r.side, r.notes, r.created_at, r.updated_at
	FROM library_reports r JOIN library_teams t ON t.id = r.team_id`

func scanLibraryReport(row sqlScanner) (library.Report, error) {
	var rep library.Report
	var id, teamID, matchIDs string
	var created, updated int64
	if err := row.Scan(&id, &rep.Title, &teamID, &rep.TeamName, &matchIDs, &rep.Side, &rep.Notes, &created, &updated); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return library.Report{}, library.ErrNotFound
		}
		return library.Report{}, err
	}
	var err error
	if rep.ID, err = uuid.Parse(id); err != nil {
		return library.Report{}, err
	}
	if rep.TeamID, err = uuid.Parse(teamID); err != nil {
		return library.Report{}, err
	}
	if err := json.Unmarshal([]byte(matchIDs), &rep.MatchIDs); err != nil {
		return library.Report{}, fmt.Errorf("decode report match ids: %w", err)
	}
	if rep.MatchIDs == nil {
		rep.MatchIDs = []uuid.UUID{}
	}
	rep.CreatedAt, rep.UpdatedAt = fromMillis(created), fromMillis(updated)
	return rep, nil
}

func encodeReportMatchIDs(ids []uuid.UUID) (string, error) {
	raw, err := json.Marshal(uniqueMatchIDs(ids))
	if err != nil {
		return "", fmt.Errorf("encode report match ids: %w", err)
	}
	return string(raw), nil
}

func (r *SQLiteLibraryRepository) CreateReport(ctx context.Context, rep *library.Report) error {
	if rep.ID == uuid.Nil {
		rep.ID = uuid.New()
	}
	matchIDs, err := encodeReportMatchIDs(rep.MatchIDs)
	if err != nil {
		return err
	}
	now := libraryNow()
	err = r.inTx(ctx, func(tx *sql.Tx) error {
		if err := r.teamExists(ctx, tx, rep.TeamID); err != nil {
			return err
		}
		_, err := tx.ExecContext(ctx, `INSERT INTO library_reports (id, title, team_id, match_ids, side, notes, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
			rep.ID.String(), rep.Title, rep.TeamID.String(), matchIDs, rep.Side, rep.Notes, millis(now), millis(now))
		return err
	})
	if err != nil {
		return err
	}
	stored, err := r.GetReport(ctx, rep.ID)
	if err != nil {
		return err
	}
	*rep = stored
	return nil
}

func (r *SQLiteLibraryRepository) GetReport(ctx context.Context, id uuid.UUID) (library.Report, error) {
	return scanLibraryReport(r.db.QueryRowContext(ctx, libraryReportColumns+` WHERE r.id = ?`, id.String()))
}

func (r *SQLiteLibraryRepository) ListReports(ctx context.Context, filter library.ReportFilter) ([]library.Report, error) {
	query := libraryReportColumns
	args := []any{}
	if filter.TeamID != nil {
		query += ` WHERE r.team_id = ?`
		args = append(args, filter.TeamID.String())
	}
	query += ` ORDER BY r.updated_at DESC, r.id ASC LIMIT ? OFFSET ?`
	args = append(args, library.ClampLimit(filter.Limit), max(filter.Offset, 0))
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list reports: %w", err)
	}
	defer rows.Close()
	reports := []library.Report{}
	for rows.Next() {
		rep, err := scanLibraryReport(rows)
		if err != nil {
			return nil, err
		}
		reports = append(reports, rep)
	}
	return reports, rows.Err()
}

func (r *SQLiteLibraryRepository) UpdateReport(ctx context.Context, id uuid.UUID, patch library.ReportPatch) (library.Report, error) {
	err := r.inTx(ctx, func(tx *sql.Tx) error {
		current, err := scanLibraryReport(tx.QueryRowContext(ctx, libraryReportColumns+` WHERE r.id = ?`, id.String()))
		if err != nil {
			return err
		}
		next := applyReportPatch(current, patch)
		matchIDs, err := encodeReportMatchIDs(next.MatchIDs)
		if err != nil {
			return err
		}
		_, err = tx.ExecContext(ctx, `UPDATE library_reports SET title = ?, match_ids = ?, side = ?, notes = ?, updated_at = ? WHERE id = ?`,
			next.Title, matchIDs, next.Side, next.Notes, millis(libraryNow()), id.String())
		return err
	})
	if err != nil {
		return library.Report{}, err
	}
	return r.GetReport(ctx, id)
}

func (r *SQLiteLibraryRepository) DeleteReport(ctx context.Context, id uuid.UUID) error {
	res, err := r.db.ExecContext(ctx, `DELETE FROM library_reports WHERE id = ?`, id.String())
	if err != nil {
		return fmt.Errorf("delete report: %w", err)
	}
	if n, err := res.RowsAffected(); err != nil {
		return err
	} else if n == 0 {
		return library.ErrNotFound
	}
	return nil
}
