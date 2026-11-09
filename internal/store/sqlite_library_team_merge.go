package store

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/google/uuid"

	"github.com/rechedev9/cliphub/internal/library"
)

// MergeTeams moves every alias, match, report, play, series slot, role sheet and
// profile of from onto into and deletes from.
func (r *SQLiteLibraryRepository) MergeTeams(ctx context.Context, from, into uuid.UUID) (library.Team, error) {
	if from == into {
		return library.Team{}, library.ErrConflict
	}
	err := r.inTx(ctx, func(tx *sql.Tx) error {
		if err := r.teamExists(ctx, tx, from); err != nil {
			return err
		}
		if err := r.teamExists(ctx, tx, into); err != nil {
			return err
		}
		// A match between the two teams cannot keep both sides on one team.
		if _, err := tx.ExecContext(ctx, `UPDATE library_match_teams SET team_id = NULL, assignment = ?
			WHERE team_id = ? AND EXISTS (SELECT 1 FROM library_match_teams o WHERE o.match_id = library_match_teams.match_id
				AND o.team_key <> library_match_teams.team_key AND o.team_id = ?)`,
			library.AssignmentNone, from.String(), into.String()); err != nil {
			return fmt.Errorf("clear head to head sides: %w", err)
		}
		for _, stmt := range []string{
			`UPDATE library_match_teams SET team_id = ? WHERE team_id = ?`,
			`UPDATE library_team_aliases SET team_id = ? WHERE team_id = ?`,
			`UPDATE library_reports SET team_id = ? WHERE team_id = ?`,
			`UPDATE library_plays SET team_id = ? WHERE team_id = ?`,
			`UPDATE library_series SET team_a = ? WHERE team_a = ?`,
			`UPDATE library_series SET team_b = ? WHERE team_b = ?`,
			// Where both teams have a sheet for a map, into keeps its own.
			`UPDATE OR IGNORE library_team_roles SET team_id = ? WHERE team_id = ?`,
			`UPDATE OR IGNORE library_team_profiles SET team_id = ? WHERE team_id = ?`,
		} {
			if _, err := tx.ExecContext(ctx, stmt, into.String(), from.String()); err != nil {
				return fmt.Errorf("merge team: %w", err)
			}
		}
		// A series between the two teams keeps one slot of the merged team.
		if _, err := tx.ExecContext(ctx, `UPDATE library_series SET team_b = NULL WHERE team_a = team_b`); err != nil {
			return fmt.Errorf("merge series slots: %w", err)
		}
		if _, err := tx.ExecContext(ctx, `DELETE FROM library_teams WHERE id = ?`, from.String()); err != nil {
			return fmt.Errorf("delete merged team: %w", err)
		}
		_, err := tx.ExecContext(ctx, `UPDATE library_teams SET updated_at = ? WHERE id = ?`, millis(libraryNow()), into.String())
		return err
	})
	if err != nil {
		return library.Team{}, err
	}
	return r.GetTeam(ctx, into)
}
