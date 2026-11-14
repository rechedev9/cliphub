package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

// deletedTeam is the id of a team a received change deletes.
func deletedTeam(change SyncChange) (string, bool) {
	if change.Table != "library_teams" || change.Op != "delete" || len(change.Key) != 1 {
		return "", false
	}
	id, ok := change.Key[0].(string)
	return id, ok
}

// teamDocumentHeirs runs before a batch. Documents never sync, so for each team the batch deletes it
// names the team that its aliases move to in the batch (a merge's target), or the owner of its name.
func teamDocumentHeirs(ctx context.Context, tx *sql.Tx, changes []SyncChange) (map[string]string, error) {
	moved := map[string]string{}
	for _, change := range changes {
		if change.Table != "library_team_aliases" || change.Op != "upsert" || len(change.Key) != 1 {
			continue
		}
		key, okKey := change.Key[0].(string)
		team, okTeam := change.Row["team_id"].(string)
		if okKey && okTeam {
			moved[key] = team
		}
	}
	heirs := map[string]string{}
	for _, change := range changes {
		id, ok := deletedTeam(change)
		if !ok {
			continue
		}
		keys, err := teamAliasKeys(ctx, tx, id)
		if err != nil {
			return nil, err
		}
		for _, key := range keys {
			if team, ok := moved[key]; ok && team != id {
				heirs[id] = team
				break
			}
		}
		if _, found := heirs[id]; found {
			continue
		}
		var owner string
		err = tx.QueryRowContext(ctx, `SELECT a.team_id FROM library_teams t JOIN library_team_aliases a ON a.alias_key = t.name_key
			WHERE t.id = ? AND a.team_id <> t.id`, id).Scan(&owner)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("read the owner of a team's name: %w", err)
		}
		heirs[id] = owner
	}
	return heirs, nil
}

func teamAliasKeys(ctx context.Context, tx *sql.Tx, id string) ([]string, error) {
	rows, err := tx.QueryContext(ctx, `SELECT name_key FROM library_teams WHERE id = ?
		UNION SELECT alias_key FROM library_team_aliases WHERE team_id = ?`, id, id)
	if err != nil {
		return nil, fmt.Errorf("read team aliases: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var keys []string
	for rows.Next() {
		var key string
		if err := rows.Scan(&key); err != nil {
			return nil, err
		}
		keys = append(keys, key)
	}
	return keys, rows.Err()
}

// teamHoldsDocuments reports a team that a received delete would take with this device's documents.
func teamHoldsDocuments(ctx context.Context, tx *sql.Tx, teamID string) (bool, error) {
	var held bool
	err := tx.QueryRowContext(ctx, `SELECT EXISTS (SELECT 1 FROM library_team_documents WHERE team_id = ?)`, teamID).Scan(&held)
	if err != nil {
		return false, fmt.Errorf("check team documents: %w", err)
	}
	return held, nil
}

// settleTeamsWithDocuments runs after the batch's upserts. A held-back team hands its documents to its heir
// and goes; one with no heir stays and is logged again, so every device keeps it rather than drift apart.
func settleTeamsWithDocuments(ctx context.Context, tx *sql.Tx, held []string, heirs map[string]string) error {
	for _, id := range held {
		if heir := heirs[id]; heir != "" {
			if _, err := tx.ExecContext(ctx, `UPDATE library_team_documents SET team_id = ?
				WHERE team_id = ? AND EXISTS (SELECT 1 FROM library_teams WHERE id = ?)`, heir, id, heir); err != nil {
				return fmt.Errorf("move team documents: %w", err)
			}
		}
		still, err := teamHoldsDocuments(ctx, tx, id)
		if err != nil {
			return err
		}
		if !still {
			if _, err := tx.ExecContext(ctx, `DELETE FROM library_teams WHERE id = ?`, id); err != nil {
				return fmt.Errorf("delete a merged team: %w", err)
			}
			continue
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO sync_changes (tbl, row_key, op, changed_at)
			VALUES ('library_teams', json_array(?), 'upsert', CAST(unixepoch('subsec') * 1000 AS INTEGER))`, id); err != nil {
			return fmt.Errorf("log a kept team: %w", err)
		}
	}
	return nil
}
