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

const libraryVoiceColumns = `SELECT match_id, sync_state, sync_method, offset_ms, auto_offset_ms, sync_confidence, sync_note, team_key,
	transcript_state, transcript_language, transcript_model, transcript_error, transcript_at, created_at, updated_at
	FROM library_match_voice`

func (r *SQLiteLibraryRepository) GetMatchVoice(ctx context.Context, matchID uuid.UUID) (library.MatchVoice, error) {
	return r.getVoice(ctx, r.db, matchID)
}

func (r *SQLiteLibraryRepository) getVoice(ctx context.Context, q libraryQuerier, matchID uuid.UUID) (library.MatchVoice, error) {
	var v library.MatchVoice
	var id string
	var transcriptAt, created, updated int64
	err := q.QueryRowContext(ctx, libraryVoiceColumns+` WHERE match_id = ?`, matchID.String()).Scan(&id,
		&v.Sync.State, &v.Sync.Method, &v.Sync.OffsetMS, &v.Sync.AutoOffsetMS, &v.Sync.Confidence, &v.Sync.Note, &v.Sync.TeamKey,
		&v.Transcript.State, &v.Transcript.Language, &v.Transcript.Model, &v.Transcript.Error, &transcriptAt, &created, &updated)
	if errors.Is(err, sql.ErrNoRows) {
		return library.MatchVoice{}, library.ErrNotFound
	}
	if err != nil {
		return library.MatchVoice{}, err
	}
	v.MatchID = matchID
	v.Transcript.UpdatedAt, v.CreatedAt, v.UpdatedAt = fromMillis(transcriptAt), fromMillis(created), fromMillis(updated)
	rows, err := q.QueryContext(ctx, `SELECT track, label, steamid64, file_name, channel, stored_key, duration_ms, size_bytes, source, speech
		FROM library_voice_tracks WHERE match_id = ? ORDER BY track`, matchID.String())
	if err != nil {
		return library.MatchVoice{}, fmt.Errorf("list voice tracks: %w", err)
	}
	defer rows.Close()
	v.Tracks = []library.VoiceTrack{}
	for rows.Next() {
		var t library.VoiceTrack
		var speech string
		if err := rows.Scan(&t.Track, &t.Label, &t.SteamID64, &t.FileName, &t.Channel, &t.StoredKey, &t.DurationMS, &t.SizeBytes, &t.Source, &speech); err != nil {
			return library.MatchVoice{}, err
		}
		if err := json.Unmarshal([]byte(speech), &t.Speech); err != nil {
			return library.MatchVoice{}, fmt.Errorf("decode voice track %d speech: %w", t.Track, err)
		}
		if t.Speech == nil {
			t.Speech = []library.VoiceSpan{}
		}
		v.Tracks = append(v.Tracks, t)
	}
	return v, rows.Err()
}

func (r *SQLiteLibraryRepository) AddVoiceTracks(ctx context.Context, add library.VoiceAdd) (library.MatchVoice, error) {
	var out library.MatchVoice
	err := r.inTx(ctx, func(tx *sql.Tx) error {
		var exists int
		if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM library_matches WHERE id = ?`, add.MatchID.String()).Scan(&exists); err != nil {
			return err
		}
		if exists == 0 {
			return library.ErrNotFound
		}
		now := millis(libraryNow())
		s := add.Sync
		if _, err := tx.ExecContext(ctx, `INSERT INTO library_match_voice
			(match_id, sync_state, sync_method, offset_ms, auto_offset_ms, sync_confidence, sync_note, team_key, transcript_at, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
			ON CONFLICT(match_id) DO UPDATE SET updated_at = excluded.updated_at`,
			add.MatchID.String(), s.State, s.Method, s.OffsetMS, s.AutoOffsetMS, s.Confidence, s.Note, s.TeamKey, now, now, now); err != nil {
			return fmt.Errorf("store match voice: %w", err)
		}
		var count, last int
		if err := tx.QueryRowContext(ctx, `SELECT COUNT(*), COALESCE(MAX(track), 0) FROM library_voice_tracks WHERE match_id = ?`,
			add.MatchID.String()).Scan(&count, &last); err != nil {
			return err
		}
		if count+len(add.Tracks) > library.MaxVoiceTracks {
			return library.ErrConflict
		}
		for i, t := range add.Tracks {
			speech, err := json.Marshal(nonNilSpans(t.Speech))
			if err != nil {
				return err
			}
			if _, err := tx.ExecContext(ctx, `INSERT INTO library_voice_tracks
				(match_id, track, label, steamid64, file_name, channel, stored_key, duration_ms, size_bytes, source, speech)
				VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
				add.MatchID.String(), last+1+i, t.Label, t.SteamID64, t.FileName, t.Channel, t.StoredKey, t.DurationMS, t.SizeBytes, t.Source, string(speech)); err != nil {
				return fmt.Errorf("store voice track: %w", err)
			}
		}
		var err error
		out, err = r.getVoice(ctx, tx, add.MatchID)
		return err
	})
	return out, err
}

func nonNilSpans(spans []library.VoiceSpan) []library.VoiceSpan {
	if spans == nil {
		return []library.VoiceSpan{}
	}
	return spans
}

func (r *SQLiteLibraryRepository) SetVoiceSync(ctx context.Context, matchID uuid.UUID, s library.VoiceSync) (library.MatchVoice, error) {
	return r.editVoice(ctx, matchID, func(tx *sql.Tx) (sql.Result, error) {
		return tx.ExecContext(ctx, `UPDATE library_match_voice SET sync_state = ?, sync_method = ?, offset_ms = ?, auto_offset_ms = ?,
			sync_confidence = ?, sync_note = ?, team_key = ?, updated_at = ? WHERE match_id = ?`,
			s.State, s.Method, s.OffsetMS, s.AutoOffsetMS, s.Confidence, s.Note, s.TeamKey, millis(libraryNow()), matchID.String())
	})
}

func (r *SQLiteLibraryRepository) UpdateVoiceTrack(ctx context.Context, patch library.VoiceTrackPatch) (library.MatchVoice, error) {
	return r.editVoice(ctx, patch.MatchID, func(tx *sql.Tx) (sql.Result, error) {
		sets, args := []string{}, []any{}
		if patch.Label != nil {
			sets, args = append(sets, "label = ?"), append(args, *patch.Label)
		}
		if patch.SteamID64 != nil {
			sets, args = append(sets, "steamid64 = ?"), append(args, *patch.SteamID64)
		}
		if len(sets) == 0 {
			sets = append(sets, "track = track")
		}
		res, err := tx.ExecContext(ctx, `UPDATE library_voice_tracks SET `+strings.Join(sets, ", ")+` WHERE match_id = ? AND track = ?`,
			append(args, patch.MatchID.String(), patch.Track)...)
		if err != nil {
			return res, err
		}
		if n, err := res.RowsAffected(); err != nil || n == 0 {
			return res, errors.Join(err, library.ErrNotFound)
		}
		return tx.ExecContext(ctx, `UPDATE library_match_voice SET updated_at = ? WHERE match_id = ?`, millis(libraryNow()), patch.MatchID.String())
	})
}

// editVoice runs one write that must touch the match's voice row, then reads the voice back.
func (r *SQLiteLibraryRepository) editVoice(ctx context.Context, matchID uuid.UUID, write func(tx *sql.Tx) (sql.Result, error)) (library.MatchVoice, error) {
	var out library.MatchVoice
	err := r.inTx(ctx, func(tx *sql.Tx) error {
		res, err := write(tx)
		if err != nil {
			return err
		}
		if n, err := res.RowsAffected(); err != nil || n == 0 {
			return errors.Join(err, library.ErrNotFound)
		}
		out, err = r.getVoice(ctx, tx, matchID)
		return err
	})
	return out, err
}

func (r *SQLiteLibraryRepository) SetVoiceTranscript(ctx context.Context, update library.VoiceTranscriptUpdate) error {
	t := update.Transcript
	at := libraryNow()
	if !t.UpdatedAt.IsZero() {
		at = libraryTime(t.UpdatedAt)
	}
	return r.inTx(ctx, func(tx *sql.Tx) error {
		res, err := tx.ExecContext(ctx, `UPDATE library_match_voice SET transcript_state = ?, transcript_language = ?, transcript_model = ?,
			transcript_error = ?, transcript_at = ? WHERE match_id = ?`, t.State, t.Language, t.Model, t.Error, millis(at), update.MatchID.String())
		if err != nil {
			return err
		}
		if n, err := res.RowsAffected(); err != nil || n == 0 {
			return errors.Join(err, library.ErrNotFound)
		}
		if update.Lines == nil {
			return nil
		}
		if _, err := tx.ExecContext(ctx, `DELETE FROM library_voice_lines WHERE match_id = ?`, update.MatchID.String()); err != nil {
			return err
		}
		stmt, err := tx.PrepareContext(ctx, `INSERT INTO library_voice_lines (match_id, track, start_ms, end_ms, text, text_key) VALUES (?, ?, ?, ?, ?, ?)`)
		if err != nil {
			return err
		}
		defer stmt.Close()
		for _, l := range *update.Lines {
			if _, err := stmt.ExecContext(ctx, update.MatchID.String(), l.Track, l.StartMS, l.EndMS, l.Text, voiceLineKey(l.Text)); err != nil {
				return fmt.Errorf("store voice line: %w", err)
			}
		}
		return nil
	})
}

func (r *SQLiteLibraryRepository) ListVoiceLines(ctx context.Context, filter library.VoiceLineFilter) ([]library.VoiceLine, error) {
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
	rows, err := r.db.QueryContext(ctx, `SELECT match_id, track, start_ms, end_ms, text FROM library_voice_lines WHERE `+
		strings.Join(where, " AND ")+` ORDER BY match_id, start_ms, track LIMIT ?`, append(args, clampVoiceLineLimit(filter.Limit))...)
	if err != nil {
		return nil, fmt.Errorf("list voice lines: %w", err)
	}
	defer rows.Close()
	out := []library.VoiceLine{}
	for rows.Next() {
		var l library.VoiceLine
		var id string
		if err := rows.Scan(&id, &l.Track, &l.StartMS, &l.EndMS, &l.Text); err != nil {
			return nil, err
		}
		if l.MatchID, err = uuid.Parse(id); err != nil {
			return nil, err
		}
		out = append(out, l)
	}
	return out, rows.Err()
}

func (r *SQLiteLibraryRepository) ListVoicesByTranscript(ctx context.Context, states []string) ([]uuid.UUID, error) {
	out := []uuid.UUID{}
	if len(states) == 0 {
		return out, nil
	}
	args := make([]any, len(states))
	for i, s := range states {
		args[i] = s
	}
	ids, err := queryStrings(ctx, r.db, `SELECT match_id FROM library_match_voice WHERE transcript_state IN (`+placeholders(len(args))+`) ORDER BY match_id`, args...)
	if err != nil {
		return nil, err
	}
	for _, raw := range ids {
		id, err := uuid.Parse(raw)
		if err != nil {
			return nil, err
		}
		out = append(out, id)
	}
	return out, nil
}

func (r *SQLiteLibraryRepository) ListVoiceSummaries(ctx context.Context) ([]library.VoiceSummary, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT v.match_id, v.sync_state, v.sync_method, v.offset_ms, v.auto_offset_ms, v.sync_confidence, v.sync_note, v.team_key,
		(SELECT COUNT(*) FROM library_voice_tracks t WHERE t.match_id = v.match_id)
		FROM library_match_voice v ORDER BY v.match_id`)
	if err != nil {
		return nil, fmt.Errorf("list voice summaries: %w", err)
	}
	defer rows.Close()
	out := []library.VoiceSummary{}
	for rows.Next() {
		var s library.VoiceSummary
		var id string
		if err := rows.Scan(&id, &s.Sync.State, &s.Sync.Method, &s.Sync.OffsetMS, &s.Sync.AutoOffsetMS, &s.Sync.Confidence, &s.Sync.Note, &s.Sync.TeamKey, &s.Tracks); err != nil {
			return nil, err
		}
		if s.MatchID, err = uuid.Parse(id); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

func (r *SQLiteLibraryRepository) DeleteMatchVoice(ctx context.Context, matchID uuid.UUID) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM library_match_voice WHERE match_id = ?`, matchID.String())
	return err
}
