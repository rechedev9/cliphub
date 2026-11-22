package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"

	"github.com/rechedev9/cliphub/internal/library"
)

const demoVoiceColumns = `SELECT match_id, state, model, error, updated_at FROM library_demo_voice`

const demoVoiceLineColumns = `SELECT id, match_id, steamid64, name, team_key, start_ms, end_ms, tick, round, text, corrected, original_text, vocab_text, discarded, words
	FROM library_demo_voice_lines`

func encodeDemoVoiceWords(words []library.DemoVoiceWord) (string, error) {
	if len(words) == 0 {
		return "", nil
	}
	raw, err := json.Marshal(words)
	return string(raw), err
}

func scanDemoVoice(row interface{ Scan(...any) error }) (library.DemoVoiceTranscript, error) {
	var t library.DemoVoiceTranscript
	var id string
	var at int64
	if err := row.Scan(&id, &t.State, &t.Model, &t.Error, &at); err != nil {
		return t, err
	}
	var err error
	t.MatchID, err = uuid.Parse(id)
	t.UpdatedAt = fromMillis(at)
	return t, err
}

func scanDemoVoiceLine(row interface{ Scan(...any) error }) (library.DemoVoiceLine, error) {
	var l library.DemoVoiceLine
	var id, matchID, words string
	if err := row.Scan(&id, &matchID, &l.SteamID64, &l.Name, &l.TeamKey, &l.StartMS, &l.EndMS, &l.Tick, &l.Round, &l.Text, &l.Corrected, &l.OriginalText, &l.VocabText, &l.Discarded, &words); err != nil {
		return l, err
	}
	if words != "" {
		if err := json.Unmarshal([]byte(words), &l.Words); err != nil {
			return l, fmt.Errorf("decode demo voice words: %w", err)
		}
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
		if _, err := tx.ExecContext(ctx, `INSERT INTO library_demo_voice (match_id, state, model, error, updated_at)
			VALUES (?, ?, ?, ?, ?) ON CONFLICT(match_id) DO UPDATE SET state = excluded.state, model = excluded.model, error = excluded.error,
			updated_at = excluded.updated_at`,
			id, t.State, t.Model, t.Error, millis(libraryNow())); err != nil {
			return fmt.Errorf("store demo voice: %w", err)
		}
		if update.Lines == nil {
			return nil
		}
		prev, err := editedDemoVoiceLines(ctx, tx, id)
		if err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `DELETE FROM library_demo_voice_lines WHERE match_id = ?`, id); err != nil {
			return err
		}
		for _, l := range *update.Lines {
			if old, ok := prev[l.ID]; ok {
				l = keepDemoVoiceEdits(old, l)
			}
			words, err := encodeDemoVoiceWords(l.Words)
			if err != nil {
				return fmt.Errorf("encode demo voice words: %w", err)
			}
			if _, err := tx.ExecContext(ctx, `INSERT INTO library_demo_voice_lines
				(id, match_id, steamid64, name, team_key, start_ms, end_ms, tick, round, text, text_key, corrected, original_text, vocab_text, discarded, words)
				VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
				l.ID.String(), id, l.SteamID64, l.Name, l.TeamKey, l.StartMS, l.EndMS, l.Tick, l.Round, l.Text, voiceLineKey(l.Text), l.Corrected, l.OriginalText, l.VocabText, l.Discarded, words); err != nil {
				return fmt.Errorf("store demo voice line: %w", err)
			}
		}
		return nil
	})
}

func editedDemoVoiceLines(ctx context.Context, tx *sql.Tx, matchID string) (map[uuid.UUID]library.DemoVoiceLine, error) {
	rows, err := tx.QueryContext(ctx, demoVoiceLineColumns+` WHERE match_id = ? AND (corrected = 1 OR discarded = 1)`, matchID)
	if err != nil {
		return nil, fmt.Errorf("list edited demo voice lines: %w", err)
	}
	defer rows.Close()
	out := map[uuid.UUID]library.DemoVoiceLine{}
	for rows.Next() {
		l, err := scanDemoVoiceLine(rows)
		if err != nil {
			return nil, err
		}
		out[l.ID] = l
	}
	return out, rows.Err()
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
	limit := clampVoiceLineLimit(filter.Limit)
	if filter.All {
		limit = -1 // SQLite reads a negative LIMIT as none
	}
	rows, err := r.db.QueryContext(ctx, demoVoiceLineColumns+` WHERE `+strings.Join(where, " AND ")+
		` ORDER BY match_id, start_ms, steamid64 LIMIT ?`, append(args, limit)...)
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
		cur, err := scanDemoVoiceLine(tx.QueryRowContext(ctx, demoVoiceLineColumns+` WHERE match_id = ? AND id = ?`, patch.MatchID.String(), patch.ID.String()))
		if errors.Is(err, sql.ErrNoRows) {
			return library.ErrNotFound
		}
		if err != nil {
			return err
		}
		out = patchDemoVoiceLine(cur, patch)
		words, err := encodeDemoVoiceWords(out.Words)
		if err != nil {
			return err
		}
		_, err = tx.ExecContext(ctx, `UPDATE library_demo_voice_lines SET text = ?, text_key = ?, corrected = ?, original_text = ?, discarded = ?, words = ? WHERE id = ?`,
			out.Text, voiceLineKey(out.Text), out.Corrected, out.OriginalText, out.Discarded, words, patch.ID.String())
		return err
	})
	return out, err
}

func (r *SQLiteLibraryRepository) ListDemoVoiceCorrections(ctx context.Context) ([]library.VoiceCorrection, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT match_id, text, original_text, vocab_text FROM library_demo_voice_lines
		WHERE corrected = 1 ORDER BY match_id, start_ms, steamid64`)
	if err != nil {
		return nil, fmt.Errorf("list demo voice corrections: %w", err)
	}
	return scanVoiceCorrections(rows)
}

func scanVoiceCorrections(rows *sql.Rows) ([]library.VoiceCorrection, error) {
	defer rows.Close()
	out := []library.VoiceCorrection{}
	for rows.Next() {
		var source string
		var l library.DemoVoiceLine
		if err := rows.Scan(&source, &l.Text, &l.OriginalText, &l.VocabText); err != nil {
			return nil, err
		}
		id, err := uuid.Parse(source)
		if err != nil {
			return nil, err
		}
		out = append(out, voiceCorrection(id, l))
	}
	return out, rows.Err()
}

func (r *SQLiteLibraryRepository) ListDemoVoices(ctx context.Context, states []string) ([]library.DemoVoiceTranscript, error) {
	out := []library.DemoVoiceTranscript{}
	if len(states) == 0 {
		return out, nil
	}
	args := make([]any, 0, len(states))
	for _, s := range states {
		args = append(args, s)
	}
	rows, err := r.db.QueryContext(ctx, demoVoiceColumns+` WHERE state IN (`+placeholders(len(states))+`) ORDER BY match_id`, args...)
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
