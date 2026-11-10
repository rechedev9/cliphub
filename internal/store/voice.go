package store

import (
	"context"
	"slices"
	"strings"

	"github.com/google/uuid"

	"github.com/rechedev9/cliphub/internal/library"
)

// VoiceRepository stores the team voice of a match: its tracks, sync and
// transcript. Deleting the match deletes its voice.
type VoiceRepository interface {
	// GetMatchVoice returns a match's voice; a match without voice is ErrNotFound.
	GetMatchVoice(ctx context.Context, matchID uuid.UUID) (library.MatchVoice, error)
	// AddVoiceTracks numbers the tracks after the ones stored; an unknown match is ErrNotFound.
	AddVoiceTracks(ctx context.Context, add library.VoiceAdd) (library.MatchVoice, error)
	SetVoiceSync(ctx context.Context, matchID uuid.UUID, sync library.VoiceSync) (library.MatchVoice, error)
	UpdateVoiceTrack(ctx context.Context, patch library.VoiceTrackPatch) (library.MatchVoice, error)
	SetVoiceTranscript(ctx context.Context, update library.VoiceTranscriptUpdate) error
	// ListVoiceLines returns lines by match, then audio time.
	ListVoiceLines(ctx context.Context, filter library.VoiceLineFilter) ([]library.VoiceLine, error)
	// ListVoicesByTranscript returns the matches whose transcript is in one of states.
	ListVoicesByTranscript(ctx context.Context, states []string) ([]uuid.UUID, error)
	// ListVoiceSummaries returns the sync state and track count of every match with voice, by match id.
	ListVoiceSummaries(ctx context.Context) ([]library.VoiceSummary, error)
	// DeleteMatchVoice removes a match's voice; none stored is not an error.
	DeleteMatchVoice(ctx context.Context, matchID uuid.UUID) error
}

func clampVoiceLineLimit(limit int) int {
	if limit <= 0 || limit > library.MaxVoiceLineList {
		return library.MaxVoiceLineList
	}
	return limit
}

// voiceLineKey is what a query matches, the same text SQLite stores: lower case, spaces collapsed.
func voiceLineKey(text string) string {
	return strings.Join(strings.Fields(strings.ToLower(text)), " ")
}

// voiceLineMatches is the line filter in memory: the match set, then the query as a phrase.
func voiceLineMatches(l library.VoiceLine, f library.VoiceLineFilter) bool {
	if len(f.MatchIDs) > 0 && !slices.Contains(f.MatchIDs, l.MatchID) {
		return false
	}
	return strings.Contains(voiceLineKey(l.Text), voiceLineKey(f.Query))
}

// voiceCopy gives a voice its own track and speech slices.
func voiceCopy(v library.MatchVoice) library.MatchVoice {
	v.Tracks = append([]library.VoiceTrack{}, v.Tracks...)
	for i, t := range v.Tracks {
		v.Tracks[i].Speech = append([]library.VoiceSpan{}, t.Speech...)
	}
	return v
}
