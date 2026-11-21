package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/google/uuid"

	"github.com/rechedev9/cliphub/internal/library"
	"github.com/rechedev9/cliphub/internal/streamclips"
)

const streamVoiceColumns = `SELECT job_id, state, model, error, spans, pending, updated_at FROM stream_voice`

const streamVoiceLineColumns = `SELECT id, job_id, start_ms, end_ms, text, corrected, original_text, discarded, words FROM stream_voice_lines`

type streamVoiceQuerier interface {
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
}

func (r *SQLiteStreamJobRepository) inTx(ctx context.Context, fn func(tx *sql.Tx) error) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	if err := fn(tx); err != nil {
		_ = tx.Rollback()
		return err
	}
	return tx.Commit()
}

func streamJobExists(ctx context.Context, q streamVoiceQuerier, id string) error {
	var exists int
	if err := q.QueryRowContext(ctx, `SELECT COUNT(*) FROM stream_jobs WHERE id = ?`, id).Scan(&exists); err != nil {
		return err
	}
	if exists == 0 {
		return streamclips.ErrNotFound
	}
	return nil
}

func scanStreamVoice(row sqlScanner) (streamclips.VoiceTranscript, error) {
	var t streamclips.VoiceTranscript
	var id, spans, pending string
	var at int64
	if err := row.Scan(&id, &t.State, &t.Model, &t.Error, &spans, &pending, &at); err != nil {
		return t, err
	}
	if err := json.Unmarshal([]byte(spans), &t.Spans); err != nil {
		return t, fmt.Errorf("decode stream voice spans: %w", err)
	}
	if err := json.Unmarshal([]byte(pending), &t.Pending); err != nil {
		return t, fmt.Errorf("decode stream voice pending spans: %w", err)
	}
	var err error
	t.JobID, err = uuid.Parse(id)
	t.UpdatedAt = fromMillis(at)
	return t, err
}

func scanStreamVoiceLine(row sqlScanner) (streamclips.VoiceLine, error) {
	var l streamclips.VoiceLine
	var id, jobID, words string
	if err := row.Scan(&id, &jobID, &l.StartMS, &l.EndMS, &l.Text, &l.Corrected, &l.OriginalText, &l.Discarded, &words); err != nil {
		return l, err
	}
	if words != "" {
		if err := json.Unmarshal([]byte(words), &l.Words); err != nil {
			return l, fmt.Errorf("decode stream voice words: %w", err)
		}
	}
	var err error
	if l.ID, err = uuid.Parse(id); err != nil {
		return l, err
	}
	l.JobID, err = uuid.Parse(jobID)
	return l, err
}

func listStreamVoiceLines(ctx context.Context, q streamVoiceQuerier, jobID string) ([]streamclips.VoiceLine, error) {
	rows, err := q.QueryContext(ctx, streamVoiceLineColumns+` WHERE job_id = ? ORDER BY start_ms, rowid`, jobID)
	if err != nil {
		return nil, fmt.Errorf("list stream voice lines: %w", err)
	}
	defer rows.Close()
	out := []streamclips.VoiceLine{}
	for rows.Next() {
		l, err := scanStreamVoiceLine(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, l)
	}
	return out, rows.Err()
}

func (r *SQLiteStreamJobRepository) GetStreamVoice(ctx context.Context, jobID uuid.UUID) (streamclips.VoiceTranscript, error) {
	t, err := scanStreamVoice(r.db.QueryRowContext(ctx, streamVoiceColumns+` WHERE job_id = ?`, jobID.String()))
	if !errors.Is(err, sql.ErrNoRows) {
		return t, err
	}
	if err := streamJobExists(ctx, r.db, jobID.String()); err != nil {
		return streamclips.VoiceTranscript{}, err
	}
	return streamclips.VoiceTranscript{JobID: jobID, State: library.TranscriptNone}, nil
}

func (r *SQLiteStreamJobRepository) SetStreamVoice(ctx context.Context, update streamclips.VoiceUpdate) error {
	t, id := storedStreamVoice(update), update.JobID.String()
	spans, err := json.Marshal(append([]library.VoiceSpan{}, t.Spans...))
	if err != nil {
		return err
	}
	pending, err := json.Marshal(append([]library.VoiceSpan{}, t.Pending...))
	if err != nil {
		return err
	}
	return r.inTx(ctx, func(tx *sql.Tx) error {
		if err := streamJobExists(ctx, tx, id); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO stream_voice (job_id, state, model, error, spans, pending, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, ?) ON CONFLICT(job_id) DO UPDATE SET state = excluded.state, model = excluded.model, error = excluded.error,
			spans = excluded.spans, pending = excluded.pending, updated_at = excluded.updated_at`,
			id, t.State, t.Model, t.Error, string(spans), string(pending), millis(t.UpdatedAt)); err != nil {
			return fmt.Errorf("store stream voice: %w", err)
		}
		if update.Lines == nil {
			return nil
		}
		existing, err := listStreamVoiceLines(ctx, tx, id)
		if err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `DELETE FROM stream_voice_lines WHERE job_id = ?`, id); err != nil {
			return err
		}
		for _, l := range replaceStreamVoiceLines(existing, update) {
			words, err := encodeDemoVoiceWords(l.Words)
			if err != nil {
				return fmt.Errorf("encode stream voice words: %w", err)
			}
			if _, err := tx.ExecContext(ctx, `INSERT INTO stream_voice_lines (id, job_id, start_ms, end_ms, text, corrected, original_text, discarded, words)
				VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`, l.ID.String(), id, l.StartMS, l.EndMS, l.Text, l.Corrected, l.OriginalText, l.Discarded, words); err != nil {
				return fmt.Errorf("store stream voice line: %w", err)
			}
		}
		return nil
	})
}

func (r *SQLiteStreamJobRepository) ListStreamVoiceLines(ctx context.Context, jobID uuid.UUID) ([]streamclips.VoiceLine, error) {
	return listStreamVoiceLines(ctx, r.db, jobID.String())
}

func (r *SQLiteStreamJobRepository) UpdateStreamVoiceLine(ctx context.Context, patch streamclips.VoiceLinePatch) (streamclips.VoiceLine, error) {
	var out streamclips.VoiceLine
	err := r.inTx(ctx, func(tx *sql.Tx) error {
		cur, err := scanStreamVoiceLine(tx.QueryRowContext(ctx, streamVoiceLineColumns+` WHERE job_id = ? AND id = ?`, patch.JobID.String(), patch.ID.String()))
		if errors.Is(err, sql.ErrNoRows) {
			return streamclips.ErrVoiceLineNotFound
		}
		if err != nil {
			return err
		}
		out = patchStreamVoiceLine(cur, patch)
		words, err := encodeDemoVoiceWords(out.Words)
		if err != nil {
			return err
		}
		_, err = tx.ExecContext(ctx, `UPDATE stream_voice_lines SET text = ?, corrected = ?, original_text = ?, discarded = ?, words = ? WHERE id = ?`,
			out.Text, out.Corrected, out.OriginalText, out.Discarded, words, patch.ID.String())
		return err
	})
	return out, err
}

func (r *SQLiteStreamJobRepository) ListStreamVoices(ctx context.Context, states []string) ([]streamclips.VoiceTranscript, error) {
	out := []streamclips.VoiceTranscript{}
	if len(states) == 0 {
		return out, nil
	}
	args := make([]any, 0, len(states))
	for _, s := range states {
		args = append(args, s)
	}
	rows, err := r.db.QueryContext(ctx, streamVoiceColumns+` WHERE state IN (`+placeholders(len(states))+`) ORDER BY job_id`, args...)
	if err != nil {
		return nil, fmt.Errorf("list stream voices: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		t, err := scanStreamVoice(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

func (r *SQLiteStreamJobRepository) GetStreamSubtitles(ctx context.Context, jobID uuid.UUID) (streamclips.Subtitles, error) {
	s := streamclips.Subtitles{JobID: jobID, Hidden: map[string][]uuid.UUID{}}
	var hidden string
	err := r.db.QueryRowContext(ctx, `SELECT shorts, summary, hidden FROM stream_subtitles WHERE job_id = ?`, jobID.String()).Scan(&s.Shorts, &s.Summary, &hidden)
	if errors.Is(err, sql.ErrNoRows) {
		return s, streamJobExists(ctx, r.db, jobID.String())
	}
	if err != nil {
		return s, err
	}
	if err := json.Unmarshal([]byte(hidden), &s.Hidden); err != nil {
		return s, fmt.Errorf("decode stream subtitles: %w", err)
	}
	return s, nil
}

func (r *SQLiteStreamJobRepository) SetStreamSubtitles(ctx context.Context, subtitles streamclips.Subtitles) error {
	s, id := cloneStreamSubtitles(subtitles), subtitles.JobID.String()
	hidden, err := json.Marshal(s.Hidden)
	if err != nil {
		return err
	}
	return r.inTx(ctx, func(tx *sql.Tx) error {
		if err := streamJobExists(ctx, tx, id); err != nil {
			return err
		}
		_, err := tx.ExecContext(ctx, `INSERT INTO stream_subtitles (job_id, shorts, summary, hidden) VALUES (?, ?, ?, ?)
			ON CONFLICT(job_id) DO UPDATE SET shorts = excluded.shorts, summary = excluded.summary, hidden = excluded.hidden`, id, s.Shorts, s.Summary, string(hidden))
		return err
	})
}
