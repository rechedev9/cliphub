package library

import (
	"time"

	"github.com/google/uuid"
)

// Voice sync states. Unsynced plays with a guessed offset the user has not confirmed.
const (
	VoiceUnsynced = "unsynced"
	VoiceAuto     = "auto"
	VoiceManual   = "manual"
)

// How a voice offset was found.
const (
	VoiceMethodDemoVoice  = "demo_voice"
	VoiceMethodKills      = "kills"
	VoiceMethodRoundStart = "round_start"
	VoiceMethodManual     = "manual"
)

// Transcript states of a match's voice.
const (
	TranscriptNone    = "none"
	TranscriptQueued  = "queued"
	TranscriptRunning = "running"
	TranscriptReady   = "ready"
	TranscriptFailed  = "failed"
)

// Where a voice track came from: an upload today, the TeamSpeak plugin folder later (#424).
const (
	VoiceSourceUpload = "upload"
	VoiceSourcePlugin = "plugin"
)

// Caps on a match's voice.
const (
	MaxVoiceTracks     = 16
	MaxVoiceLabelRunes = 60
	MaxVoiceLineRunes  = 2000
	MaxVoiceLineList   = 5000
	// MaxVoiceOffsetMS bounds an offset to six hours either way.
	MaxVoiceOffsetMS = 6 * 3600 * 1000
)

// VoiceSpan is a stretch of audio time, in milliseconds from the start of the recording.
type VoiceSpan struct {
	StartMS int64 `json:"start_ms"`
	EndMS   int64 `json:"end_ms"`
}

// VoiceTrack is one speaker or mix of a match's recording: one stored audio file.
type VoiceTrack struct {
	Track      int    `json:"track"`
	Label      string `json:"label"`
	SteamID64  string `json:"steamid64,omitempty"`
	FileName   string `json:"file_name"`
	Channel    int    `json:"channel"`
	StoredKey  string `json:"-"`
	DurationMS int64  `json:"duration_ms"`
	SizeBytes  int64  `json:"size_bytes"`
	Source     string `json:"source"`
	// Speech is where the track has voice, found on import; talk time and sync read it.
	Speech []VoiceSpan `json:"-"`
}

// VoiceSync places the recording on the demo: audio time 0 plays at demo time OffsetMS.
type VoiceSync struct {
	State    string `json:"state"`
	Method   string `json:"method"`
	OffsetMS int64  `json:"offset_ms"`
	// AutoOffsetMS is the automatic estimate, which Reset goes back to.
	AutoOffsetMS int64   `json:"auto_offset_ms"`
	Confidence   float64 `json:"confidence"`
	Note         string  `json:"note,omitempty"`
	// TeamKey is the team whose comms these are, when the sync could tell.
	TeamKey string `json:"team_key,omitempty"`
}

// VoiceTranscript is the state of a match's local transcription.
type VoiceTranscript struct {
	State     string    `json:"state"`
	Language  string    `json:"language,omitempty"`
	Model     string    `json:"model,omitempty"`
	Error     string    `json:"error,omitempty"`
	UpdatedAt time.Time `json:"updated_at"`
}

// MatchVoice is the team voice imported for one match.
type MatchVoice struct {
	MatchID    uuid.UUID       `json:"match_id"`
	Tracks     []VoiceTrack    `json:"tracks"`
	Sync       VoiceSync       `json:"sync"`
	Transcript VoiceTranscript `json:"transcript"`
	CreatedAt  time.Time       `json:"created_at"`
	UpdatedAt  time.Time       `json:"updated_at"`
}

// DurationMS is the length of the longest track.
func (v MatchVoice) DurationMS() int64 {
	var longest int64
	for _, t := range v.Tracks {
		longest = max(longest, t.DurationMS)
	}
	return longest
}

// VoiceAdd stores new tracks on a match; a match without voice takes Sync too.
type VoiceAdd struct {
	MatchID uuid.UUID
	Tracks  []VoiceTrack
	Sync    VoiceSync
}

// VoiceTrackPatch names a track's speaker; nil fields are kept.
type VoiceTrackPatch struct {
	MatchID   uuid.UUID
	Track     int
	Label     *string
	SteamID64 *string
}

// VoiceLine is one transcribed phrase of a track, in audio time.
type VoiceLine struct {
	MatchID uuid.UUID `json:"match_id"`
	Track   int       `json:"track"`
	StartMS int64     `json:"start_ms"`
	EndMS   int64     `json:"end_ms"`
	Text    string    `json:"text"`
}

// VoiceTranscriptUpdate sets the transcript state; non-nil Lines replaces every line.
type VoiceTranscriptUpdate struct {
	MatchID    uuid.UUID
	Transcript VoiceTranscript
	Lines      *[]VoiceLine
}

// VoiceLineFilter reads lines; empty MatchIDs means every match, Query matches words case-insensitively.
type VoiceLineFilter struct {
	MatchIDs []uuid.UUID
	Query    string
	Limit    int
}
