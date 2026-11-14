package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

// moveDocumentsOfMergedTeams runs before a batch, whose deletes come before its upserts. Documents never
// sync, so a merge on another device could not move them: they follow the team's name to its new owner.
func moveDocumentsOfMergedTeams(ctx context.Context, tx *sql.Tx, changes []SyncChange) error {
	heirs := map[string]string{}
	for _, change := range changes {
		if change.Table != "library_team_aliases" || change.Op != "upsert" || len(change.Key) != 1 {
			continue
		}
		if key, ok := change.Key[0].(string); ok {
			if team, ok := change.Row["team_id"].(string); ok {
				heirs[key] = team
			}
		}
	}
	for _, change := range changes {
		if change.Table != "library_teams" || change.Op != "delete" || len(change.Key) != 1 {
			continue
		}
		var nameKey, held string
		err := tx.QueryRowContext(ctx, `SELECT name_key, COALESCE((SELECT team_id FROM library_team_aliases WHERE alias_key = name_key), '')
			FROM library_teams WHERE id = ?`, change.Key[0]).Scan(&nameKey, &held)
		if errors.Is(err, sql.ErrNoRows) {
			continue
		}
		if err != nil {
			return fmt.Errorf("read a merged team: %w", err)
		}
		heir, ok := heirs[nameKey]
		if !ok {
			heir = held
		}
		if heir == "" || heir == change.Key[0] {
			continue
		}
		if _, err := tx.ExecContext(ctx, `UPDATE library_team_documents SET team_id = ?
			WHERE team_id = ? AND EXISTS (SELECT 1 FROM library_teams WHERE id = ?)`, heir, change.Key[0], heir); err != nil {
			return fmt.Errorf("move team documents: %w", err)
		}
	}
	return nil
}

// teamHoldsDocuments keeps a team that a synced delete would take with this device's documents.
func teamHoldsDocuments(ctx context.Context, tx *sql.Tx, teamID any) (bool, error) {
	var held bool
	err := tx.QueryRowContext(ctx, `SELECT EXISTS (SELECT 1 FROM library_team_documents WHERE team_id = ?)`, teamID).Scan(&held)
	if err != nil {
		return false, fmt.Errorf("check team documents: %w", err)
	}
	return held, nil
}
