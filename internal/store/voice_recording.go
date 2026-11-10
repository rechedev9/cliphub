package store

import (
	"context"
	"slices"

	"github.com/google/uuid"

	"github.com/rechedev9/cliphub/internal/library"
)

// VoiceRecordingRepository stores the TeamSpeak plugin's recordings and each team's memory of
// who speaks under a TeamSpeak identity. Deleting a match leaves its recordings waiting again.
type VoiceRecordingRepository interface {
	// CreateVoiceRecording stores a new recording; an id already stored is ErrConflict.
	CreateVoiceRecording(ctx context.Context, rec library.VoiceRecording) error
	GetVoiceRecording(ctx context.Context, id string) (library.VoiceRecording, error)
	// ListVoiceRecordings returns every recording, the newest start first.
	ListVoiceRecordings(ctx context.Context) ([]library.VoiceRecording, error)
	// UpdateVoiceRecording writes the match, note and tracks; an unknown match is ErrNotFound.
	UpdateVoiceRecording(ctx context.Context, rec library.VoiceRecording) (library.VoiceRecording, error)
	DeleteVoiceRecording(ctx context.Context, id string) error
	// SaveVoiceSpeakers stores or replaces each team's speaker; an unknown team is ErrNotFound.
	SaveVoiceSpeakers(ctx context.Context, speakers []library.VoiceSpeaker) error
	// ListVoiceSpeakers returns the speakers of the given teams.
	ListVoiceSpeakers(ctx context.Context, teamIDs []uuid.UUID) ([]library.VoiceSpeaker, error)
}

// recordingCopy gives a recording its own slices.
func recordingCopy(rec library.VoiceRecording) library.VoiceRecording {
	rec.Tracks = append([]library.RecordingTrack{}, rec.Tracks...)
	rec.Rounds = append([]library.RecordingRound{}, rec.Rounds...)
	if rec.MatchID != nil {
		id := *rec.MatchID
		rec.MatchID = &id
	}
	return rec
}

func sortRecordings(recs []library.VoiceRecording) {
	slices.SortFunc(recs, func(a, b library.VoiceRecording) int {
		if c := b.StartedAt.Compare(a.StartedAt); c != 0 {
			return c
		}
		return compareStrings(a.ID, b.ID)
	})
}

func compareStrings(a, b string) int {
	switch {
	case a < b:
		return -1
	case a > b:
		return 1
	}
	return 0
}
