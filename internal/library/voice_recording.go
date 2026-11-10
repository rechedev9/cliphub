package library

import (
	"strings"
	"time"

	"github.com/google/uuid"
)

// VoiceMethodGameClock places a plugin recording by the wall-clock time each round went live in CS2.
const VoiceMethodGameClock = "game_clock"

// MaxVoiceRecordingIDBytes bounds a recording id, the name of its inbox folder.
const MaxVoiceRecordingIDBytes = 64

// VoiceRecording is one recording the TeamSpeak plugin left in Studio's inbox: one track per person,
// with the server, the map and the round times CS2 reported while it ran.
type VoiceRecording struct {
	ID            string           `json:"id"`
	StartedAt     time.Time        `json:"started_at"`
	EndedAt       time.Time        `json:"ended_at"`
	Map           string           `json:"map,omitempty"`
	Server        string           `json:"server,omitempty"`
	Channel       string           `json:"channel,omitempty"`
	Trigger       string           `json:"trigger"`
	SelfSteamID64 string           `json:"self_steamid64,omitempty"`
	ScoreCT       int              `json:"score_ct"`
	ScoreT        int              `json:"score_t"`
	Interrupted   bool             `json:"interrupted,omitempty"`
	Tracks        []RecordingTrack `json:"tracks"`
	Rounds        []RecordingRound `json:"rounds"`
	// MatchID is the match the recording's tracks were added to; nil while it waits for one.
	MatchID   *uuid.UUID `json:"match_id,omitempty"`
	Note      string     `json:"note,omitempty"`
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
}

// RecordingTrack is one person of a plugin recording.
type RecordingTrack struct {
	// File is the track's base name in the recording folder, without extension.
	File     string `json:"file"`
	Nickname string `json:"nickname"`
	// UID is the TeamSpeak client unique identifier, stable across nickname changes.
	UID        string `json:"uid,omitempty"`
	Self       bool   `json:"self,omitempty"`
	DurationMS int64  `json:"duration_ms"`
	// MatchTrack is the voice track it became in the attached match; 0 while unattached.
	MatchTrack int `json:"match_track,omitempty"`
}

// SpeakerKey is how a team remembers who a track's person is: the TeamSpeak identity, else the nickname.
func (t RecordingTrack) SpeakerKey() string {
	if t.UID != "" {
		return "uid:" + t.UID
	}
	return "nick:" + strings.ToLower(CleanName(t.Nickname))
}

// RecordingRound is when a round went live in CS2, by the wall clock of this PC.
type RecordingRound struct {
	Round  int       `json:"round"`
	LiveAt time.Time `json:"live_at"`
}

// VoiceSpeaker is a team's memory of who speaks under a TeamSpeak identity.
type VoiceSpeaker struct {
	TeamID     uuid.UUID `json:"team_id"`
	SpeakerKey string    `json:"speaker_key"`
	Nickname   string    `json:"nickname"`
	SteamID64  string    `json:"steamid64"`
	UpdatedAt  time.Time `json:"updated_at"`
}
