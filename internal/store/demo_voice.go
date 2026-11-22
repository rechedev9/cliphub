package store

import (
	"context"
	"slices"
	"strings"

	"github.com/google/uuid"

	"github.com/rechedev9/cliphub/internal/library"
)

// DemoVoiceRepository stores the transcript of the voices a demo recorded. Deleting the match deletes it.
type DemoVoiceRepository interface {
	// GetDemoVoice returns a match's transcript state; none stored is ErrNotFound.
	GetDemoVoice(ctx context.Context, matchID uuid.UUID) (library.DemoVoiceTranscript, error)
	// SetDemoVoice stores the state and, when given, the lines; an unknown match is ErrNotFound.
	SetDemoVoice(ctx context.Context, update library.DemoVoiceUpdate) error
	// ListDemoVoiceLines returns phrases by match, then demo time.
	ListDemoVoiceLines(ctx context.Context, filter library.DemoVoiceLineFilter) ([]library.DemoVoiceLine, error)
	UpdateDemoVoiceLine(ctx context.Context, patch library.DemoVoiceLinePatch) (library.DemoVoiceLine, error)
	// ListDemoVoices returns the transcripts whose state is one of states, by match id.
	ListDemoVoices(ctx context.Context, states []string) ([]library.DemoVoiceTranscript, error)
	ListDemoVoiceCorrections(ctx context.Context) ([]library.VoiceCorrection, error)
}

func voiceCorrection(source uuid.UUID, l library.DemoVoiceLine) library.VoiceCorrection {
	return library.VoiceCorrection{SourceID: source, Heard: l.HeardText(), Vocab: l.VocabText, Text: l.Text}
}

func demoVoiceLineMatches(l library.DemoVoiceLine, f library.DemoVoiceLineFilter) bool {
	if len(f.MatchIDs) > 0 && !slices.Contains(f.MatchIDs, l.MatchID) {
		return false
	}
	return strings.Contains(voiceLineKey(l.Text), voiceLineKey(f.Query))
}

// demoVoiceLineCompare orders phrases by match id, then demo time and player, as SQLite does.
func demoVoiceLineCompare(a, b library.DemoVoiceLine) int {
	if c := strings.Compare(a.MatchID.String(), b.MatchID.String()); c != 0 {
		return c
	}
	if a.StartMS != b.StartMS {
		return int(a.StartMS - b.StartMS)
	}
	return strings.Compare(a.SteamID64, b.SteamID64)
}
