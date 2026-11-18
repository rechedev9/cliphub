package library

import (
	"time"

	"github.com/google/uuid"
)

// DemoVoiceTranscript is the transcription of the voices a demo recorded, one per player of a team.
// English is the state of the optional translation beside it.
type DemoVoiceTranscript struct {
	MatchID      uuid.UUID `json:"match_id"`
	State        string    `json:"state"`
	Model        string    `json:"model,omitempty"`
	Error        string    `json:"error,omitempty"`
	English      string    `json:"english"`
	EnglishError string    `json:"english_error,omitempty"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// DemoVoiceLine is one phrase a player said, on the demo's clock: StartMS is demo time, Tick and Round place it.
type DemoVoiceLine struct {
	ID        uuid.UUID `json:"id"`
	MatchID   uuid.UUID `json:"match_id"`
	SteamID64 string    `json:"steamid64"`
	Name      string    `json:"name"`
	TeamKey   string    `json:"team_key"`
	StartMS   int64     `json:"start_ms"`
	EndMS     int64     `json:"end_ms"`
	Tick      int       `json:"tick"`
	Round     int       `json:"round"`
	Text      string    `json:"text"`
	// Corrected marks text a person fixed; a new transcription replaces it.
	Corrected bool   `json:"corrected"`
	English   string `json:"english,omitempty"`
}

// DemoVoiceUpdate sets a demo's transcript state. Non-nil Lines replaces every line;
// non-nil English replaces every line's translation, by line ID.
type DemoVoiceUpdate struct {
	MatchID    uuid.UUID
	Transcript DemoVoiceTranscript
	Lines      *[]DemoVoiceLine
	English    map[uuid.UUID]string
}

// DemoVoiceLinePatch corrects one phrase; nil fields are kept.
type DemoVoiceLinePatch struct {
	MatchID uuid.UUID
	ID      uuid.UUID
	Text    *string
	English *string
}

// DemoVoiceLineFilter reads phrases; empty MatchIDs means every match, Query matches the text as a phrase.
type DemoVoiceLineFilter struct {
	MatchIDs []uuid.UUID
	Query    string
	Limit    int
}
