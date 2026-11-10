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

const voiceRecordingColumns = `SELECT id, started_at, ended_at, map, server, channel, trigger, self_steamid, score_ct, score_t,
	interrupted, tracks, rounds, match_id, note, created_at, updated_at FROM library_voice_recordings`

// recordingRoundRow is a round as stored: the wall clock in Unix milliseconds.
type recordingRoundRow struct {
	Round  int   `json:"round"`
	LiveAt int64 `json:"live_at_ms"`
}

func (r *SQLiteLibraryRepository) CreateVoiceRecording(ctx context.Context, rec library.VoiceRecording) error {
	tracks, rounds, err := encodeRecording(rec)
	if err != nil {
		return err
	}
	return r.inTx(ctx, func(tx *sql.Tx) error {
		var n int
		if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM library_voice_recordings WHERE id = ?`, rec.ID).Scan(&n); err != nil {
			return err
		}
		if n > 0 {
			return library.ErrConflict
		}
		now := millis(libraryNow())
		_, err := tx.ExecContext(ctx, `INSERT INTO library_voice_recordings
			(id, started_at, ended_at, map, server, channel, trigger, self_steamid, score_ct, score_t, interrupted, tracks, rounds, note, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			rec.ID, millis(rec.StartedAt), millis(rec.EndedAt), rec.Map, rec.Server, rec.Channel, rec.Trigger, rec.SelfSteamID64,
			rec.ScoreCT, rec.ScoreT, boolInt(rec.Interrupted), tracks, rounds, rec.Note, now, now)
		return err
	})
}

func encodeRecording(rec library.VoiceRecording) (string, string, error) {
	tracks, err := json.Marshal(append([]library.RecordingTrack{}, rec.Tracks...))
	if err != nil {
		return "", "", err
	}
	rows := make([]recordingRoundRow, 0, len(rec.Rounds))
	for _, rd := range rec.Rounds {
		rows = append(rows, recordingRoundRow{Round: rd.Round, LiveAt: millis(rd.LiveAt)})
	}
	rounds, err := json.Marshal(rows)
	return string(tracks), string(rounds), err
}

func (r *SQLiteLibraryRepository) GetVoiceRecording(ctx context.Context, id string) (library.VoiceRecording, error) {
	recs, err := r.queryRecordings(ctx, ` WHERE id = ?`, id)
	if err != nil {
		return library.VoiceRecording{}, err
	}
	if len(recs) == 0 {
		return library.VoiceRecording{}, library.ErrNotFound
	}
	return recs[0], nil
}

func (r *SQLiteLibraryRepository) ListVoiceRecordings(ctx context.Context) ([]library.VoiceRecording, error) {
	return r.queryRecordings(ctx, ` ORDER BY started_at DESC, id ASC`)
}

func (r *SQLiteLibraryRepository) queryRecordings(ctx context.Context, tail string, args ...any) ([]library.VoiceRecording, error) {
	rows, err := r.db.QueryContext(ctx, voiceRecordingColumns+tail, args...)
	if err != nil {
		return nil, fmt.Errorf("list voice recordings: %w", err)
	}
	defer rows.Close()
	out := []library.VoiceRecording{}
	for rows.Next() {
		var rec library.VoiceRecording
		var started, ended, created, updated int64
		var interrupted int
		var tracks, rounds string
		var matchID sql.NullString
		if err := rows.Scan(&rec.ID, &started, &ended, &rec.Map, &rec.Server, &rec.Channel, &rec.Trigger, &rec.SelfSteamID64,
			&rec.ScoreCT, &rec.ScoreT, &interrupted, &tracks, &rounds, &matchID, &rec.Note, &created, &updated); err != nil {
			return nil, err
		}
		rec.StartedAt, rec.EndedAt, rec.CreatedAt, rec.UpdatedAt = fromMillis(started), fromMillis(ended), fromMillis(created), fromMillis(updated)
		rec.Interrupted = interrupted != 0
		if rec.MatchID, err = parseNullUUID(matchID); err != nil {
			return nil, err
		}
		if err := json.Unmarshal([]byte(tracks), &rec.Tracks); err != nil {
			return nil, fmt.Errorf("decode recording %s tracks: %w", rec.ID, err)
		}
		var stored []recordingRoundRow
		if err := json.Unmarshal([]byte(rounds), &stored); err != nil {
			return nil, fmt.Errorf("decode recording %s rounds: %w", rec.ID, err)
		}
		rec.Rounds = make([]library.RecordingRound, 0, len(stored))
		for _, rd := range stored {
			rec.Rounds = append(rec.Rounds, library.RecordingRound{Round: rd.Round, LiveAt: fromMillis(rd.LiveAt)})
		}
		if rec.Tracks == nil {
			rec.Tracks = []library.RecordingTrack{}
		}
		out = append(out, rec)
	}
	return out, rows.Err()
}

func (r *SQLiteLibraryRepository) UpdateVoiceRecording(ctx context.Context, rec library.VoiceRecording) (library.VoiceRecording, error) {
	tracks, _, err := encodeRecording(rec)
	if err != nil {
		return library.VoiceRecording{}, err
	}
	var matchID any
	if rec.MatchID != nil {
		var exists int
		if err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM library_matches WHERE id = ?`, rec.MatchID.String()).Scan(&exists); err != nil {
			return library.VoiceRecording{}, err
		}
		if exists == 0 {
			return library.VoiceRecording{}, library.ErrNotFound
		}
		matchID = rec.MatchID.String()
	}
	res, err := r.db.ExecContext(ctx, `UPDATE library_voice_recordings SET match_id = ?, note = ?, tracks = ?, updated_at = ? WHERE id = ?`,
		matchID, rec.Note, tracks, millis(libraryNow()), rec.ID)
	if err != nil {
		return library.VoiceRecording{}, err
	}
	if n, err := res.RowsAffected(); err != nil || n == 0 {
		return library.VoiceRecording{}, errors.Join(err, library.ErrNotFound)
	}
	return r.GetVoiceRecording(ctx, rec.ID)
}

func (r *SQLiteLibraryRepository) DeleteVoiceRecording(ctx context.Context, id string) error {
	res, err := r.db.ExecContext(ctx, `DELETE FROM library_voice_recordings WHERE id = ?`, id)
	if err != nil {
		return err
	}
	if n, err := res.RowsAffected(); err != nil || n == 0 {
		return errors.Join(err, library.ErrNotFound)
	}
	return nil
}

func (r *SQLiteLibraryRepository) SaveVoiceSpeakers(ctx context.Context, speakers []library.VoiceSpeaker) error {
	return r.inTx(ctx, func(tx *sql.Tx) error {
		now := millis(libraryNow())
		for _, s := range speakers {
			var exists int
			if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM library_teams WHERE id = ?`, s.TeamID.String()).Scan(&exists); err != nil {
				return err
			}
			if exists == 0 {
				return library.ErrNotFound
			}
			if _, err := tx.ExecContext(ctx, `INSERT INTO library_voice_speakers (team_id, speaker_key, nickname, steamid64, updated_at)
				VALUES (?, ?, ?, ?, ?) ON CONFLICT(team_id, speaker_key) DO UPDATE SET nickname = excluded.nickname,
				steamid64 = excluded.steamid64, updated_at = excluded.updated_at`,
				s.TeamID.String(), s.SpeakerKey, s.Nickname, s.SteamID64, now); err != nil {
				return fmt.Errorf("store voice speaker: %w", err)
			}
		}
		return nil
	})
}

func (r *SQLiteLibraryRepository) ListVoiceSpeakers(ctx context.Context, teamIDs []uuid.UUID) ([]library.VoiceSpeaker, error) {
	out := []library.VoiceSpeaker{}
	if len(teamIDs) == 0 {
		return out, nil
	}
	marks, args := make([]string, len(teamIDs)), make([]any, len(teamIDs))
	for i, id := range teamIDs {
		marks[i], args[i] = "?", id.String()
	}
	rows, err := r.db.QueryContext(ctx, `SELECT team_id, speaker_key, nickname, steamid64, updated_at FROM library_voice_speakers
		WHERE team_id IN (`+strings.Join(marks, ", ")+`) ORDER BY team_id, speaker_key`, args...)
	if err != nil {
		return nil, fmt.Errorf("list voice speakers: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var s library.VoiceSpeaker
		var team string
		var updated int64
		if err := rows.Scan(&team, &s.SpeakerKey, &s.Nickname, &s.SteamID64, &updated); err != nil {
			return nil, err
		}
		if s.TeamID, err = uuid.Parse(team); err != nil {
			return nil, err
		}
		s.UpdatedAt = fromMillis(updated)
		out = append(out, s)
	}
	return out, rows.Err()
}
