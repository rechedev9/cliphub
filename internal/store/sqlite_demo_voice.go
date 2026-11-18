package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"

	"github.com/rechedev9/cliphub/internal/library"
)

const demoVoiceColumns = `SELECT match_id, state, model, error, english, english_error, updated_at FROM library_demo_voice`

const demoVoiceLineColumns = `SELECT id, match_id, steamid64, name, team_key, start_ms, end_ms, tick, round, text, corrected, english
	FROM library_demo_voice_lines`

func scanDemoVoice(row interface{ Scan(...any) error }) (library.DemoVoiceTranscript, error) {
	var t library.DemoVoiceTranscript
	var id string
	var at int64
	if err := row.Scan(&id, &t.State, &t.Model, &t.Error, &t.English, &t.EnglishError, &at); err != nil {
		return t, err
	}
	var err error
	t.MatchID, err = uuid.Parse(id)
	t.UpdatedAt = fromMillis(at)
	return t, err
}

func scanDemoVoiceLine(row interface{ Scan(...any) error }) (library.DemoVoiceLine, error) {
	var l library.DemoVoiceLine
	var id, matchID string
	if err := row.Scan(&id, &matchID, &l.SteamID64, &l.Name, &l.TeamKey, &l.StartMS, &l.EndMS, &l.Tick, &l.Round, &l.Text, &l.Corrected, &l.English); err != nil {
		return l, err
	}
	var err error
	if l.ID, err = uuid.Parse(id); err != nil {
		return l, err
	}
	l.MatchID, err = uuid.Parse(matchID)
	return l, err
}

func (r *SQLiteLibraryRepository) GetDemoVoice(ctx context.Context, matchID uuid.UUID) (library.DemoVoiceTranscript, error) {
	t, err := scanDemoVoice(r.db.QueryRowContext(ctx, demoVoiceColumns+` WHERE match_id = ?`, matchID.String()))
	if errors.Is(err, sql.ErrNoRows) {
		return library.DemoVoiceTranscript{}, library.ErrNotFound
	}
	return t, err
}

func (r *SQLiteLibraryRepository) SetDemoVoice(ctx context.Context, update library.DemoVoiceUpdate) error {
	t, id := update.Transcript, update.MatchID.String()
	return r.inTx(ctx, func(tx *sql.Tx) error {
		var exists int
		if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM library_matches WHERE id = ?`, id).Scan(&exists); err != nil {
			return err
		}
		if exists == 0 {
			return library.ErrNotFound
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO library_demo_voice (match_id, state, model, error, english, english_error, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, ?) ON CONFLICT(match_id) DO UPDATE SET state = excluded.state, model = excluded.model, error = excluded.error,
			english = excluded.english, english_error = excluded.english_error, updated_at = excluded.updated_at`,
			id, t.State, t.Model, t.Error, t.English, t.EnglishError, millis(libraryNow())); err != nil {
			return fmt.Errorf("store demo voice: %w", err)
		}
		if update.Lines != nil {
			if _, err := tx.ExecContext(ctx, `DELETE FROM library_demo_voice_lines WHERE match_id = ?`, id); err != nil {
				return err
			}
			for _, l := range *update.Lines {
				if _, err := tx.ExecContext(ctx, `INSERT INTO library_demo_voice_lines
					(id, match_id, steamid64, name, team_key, start_ms, end_ms, tick, round, text, text_key, corrected, english)
					VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
					l.ID.String(), id, l.SteamID64, l.Name, l.TeamKey, l.StartMS, l.EndMS, l.Tick, l.Round, l.Text, voiceLineKey(l.Text), l.Corrected, l.English); err != nil {
					return fmt.Errorf("store demo voice line: %w", err)
				}
			}
		}
		if update.English == nil {
			return nil
		}
		if _, err := tx.ExecContext(ctx, `UPDATE library_demo_voice_lines SET english = '' WHERE match_id = ?`, id); err != nil {
			return err
		}
		for lineID, english := range update.English {
			if _, err := tx.ExecContext(ctx, `UPDATE library_demo_voice_lines SET english = ? WHERE match_id = ? AND id = ?`, english, id, lineID.String()); err != nil {
				return fmt.Errorf("store demo voice translation: %w", err)
			}
		}
		return nil
	})
}

func (r *SQLiteLibraryRepository) ListDemoVoiceLines(ctx context.Context, filter library.DemoVoiceLineFilter) ([]library.DemoVoiceLine, error) {
	where, args := []string{"1 = 1"}, []any{}
	if len(filter.MatchIDs) > 0 {
		ids := make([]any, len(filter.MatchIDs))
		for i, id := range filter.MatchIDs {
			ids[i] = id.String()
		}
		where, args = append(where, `match_id IN (`+placeholders(len(ids))+`)`), append(args, ids...)
	}
	if key := voiceLineKey(filter.Query); key != "" {
		where, args = append(where, `instr(text_key, ?) > 0`), append(args, key)
	}
	rows, err := r.db.QueryContext(ctx, demoVoiceLineColumns+` WHERE `+strings.Join(where, " AND ")+
		` ORDER BY match_id, start_ms, steamid64 LIMIT ?`, append(args, clampVoiceLineLimit(filter.Limit))...)
	if err != nil {
		return nil, fmt.Errorf("list demo voice lines: %w", err)
	}
	defer rows.Close()
	out := []library.DemoVoiceLine{}
	for rows.Next() {
		l, err := scanDemoVoiceLine(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, l)
	}
	return out, rows.Err()
}

func (r *SQLiteLibraryRepository) UpdateDemoVoiceLine(ctx context.Context, patch library.DemoVoiceLinePatch) (library.DemoVoiceLine, error) {
	var out library.DemoVoiceLine
	err := r.inTx(ctx, func(tx *sql.Tx) error {
		sets, args := []string{"id = id"}, []any{}
		if patch.Text != nil {
			sets, args = append(sets, "text = ?", "text_key = ?", "corrected = 1"), append(args, *patch.Text, voiceLineKey(*patch.Text))
		}
		if patch.English != nil {
			sets, args = append(sets, "english = ?"), append(args, *patch.English)
		}
		res, err := tx.ExecContext(ctx, `UPDATE library_demo_voice_lines SET `+strings.Join(sets, ", ")+` WHERE match_id = ? AND id = ?`,
			append(args, patch.MatchID.String(), patch.ID.String())...)
		if err != nil {
			return err
		}
		if n, err := res.RowsAffected(); err != nil || n == 0 {
			return errors.Join(err, library.ErrNotFound)
		}
		out, err = scanDemoVoiceLine(tx.QueryRowContext(ctx, demoVoiceLineColumns+` WHERE id = ?`, patch.ID.String()))
		return err
	})
	return out, err
}

func (r *SQLiteLibraryRepository) ListDemoVoices(ctx context.Context, states []string) ([]library.DemoVoiceTranscript, error) {
	out := []library.DemoVoiceTranscript{}
	if len(states) == 0 {
		return out, nil
	}
	args := make([]any, 0, 2*len(states))
	for range 2 {
		for _, s := range states {
			args = append(args, s)
		}
	}
	in := placeholders(len(states))
	rows, err := r.db.QueryContext(ctx, demoVoiceColumns+` WHERE state IN (`+in+`) OR english IN (`+in+`) ORDER BY match_id`, args...)
	if err != nil {
		return nil, fmt.Errorf("list demo voices: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		t, err := scanDemoVoice(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}
