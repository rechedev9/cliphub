package tuiclient

import (
	"context"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

// VoiceTrack is one speaker or mix of a match's team voice recording.
type VoiceTrack struct {
	Track      int    `json:"track"`
	Label      string `json:"label"`
	SteamID64  string `json:"steamid64,omitempty"`
	FileName   string `json:"file_name"`
	Channel    int    `json:"channel"`
	DurationMS int64  `json:"duration_ms"`
	SizeBytes  int64  `json:"size_bytes"`
	Source     string `json:"source"`
}

// VoiceSync places the recording on the demo: audio time 0 plays at demo time OffsetMS.
type VoiceSync struct {
	State        string  `json:"state"`
	Method       string  `json:"method"`
	OffsetMS     int64   `json:"offset_ms"`
	AutoOffsetMS int64   `json:"auto_offset_ms"`
	Confidence   float64 `json:"confidence"`
	Note         string  `json:"note,omitempty"`
	TeamKey      string  `json:"team_key,omitempty"`
}

// VoiceTranscript is the state of a match's local transcription.
type VoiceTranscript struct {
	State     string    `json:"state"`
	Language  string    `json:"language,omitempty"`
	Model     string    `json:"model,omitempty"`
	Error     string    `json:"error,omitempty"`
	UpdatedAt time.Time `json:"updated_at"`
	Progress  float64   `json:"progress,omitempty"`
}

// VoiceTranscriber is the local whisper.cpp install.
type VoiceTranscriber struct {
	State         string `json:"state"`
	Version       string `json:"version"`
	Model         string `json:"model"`
	DownloadBytes int64  `json:"download_bytes"`
	DoneBytes     int64  `json:"done_bytes,omitempty"`
	Error         string `json:"error,omitempty"`
}

// VoiceFileResult is the outcome of one imported file.
type VoiceFileResult struct {
	FileName  string `json:"file_name"`
	Status    string `json:"status"`
	Tracks    int    `json:"tracks"`
	ErrorCode string `json:"error_code,omitempty"`
	Error     string `json:"error,omitempty"`
}

// MatchVoice is a match's team voice: its tracks, sync and transcript.
type MatchVoice struct {
	MatchID     string            `json:"match_id"`
	Present     bool              `json:"present"`
	Tracks      []VoiceTrack      `json:"tracks"`
	DurationMS  int64             `json:"duration_ms"`
	SpeechMS    int64             `json:"speech_ms"`
	Sync        VoiceSync         `json:"sync"`
	Transcript  VoiceTranscript   `json:"transcript"`
	Transcriber VoiceTranscriber  `json:"transcriber"`
	Files       []VoiceFileResult `json:"files,omitempty"`
}

// VoicePhaseTalk is milliseconds of speech in each phase of a round.
type VoicePhaseTalk struct {
	FreezetimeMS int64 `json:"freezetime_ms"`
	MidroundMS   int64 `json:"midround_ms"`
	PostPlantMS  int64 `json:"post_plant_ms"`
	DeadMS       int64 `json:"dead_ms"`
	AfterRoundMS int64 `json:"after_round_ms"`
}

// VoiceSpeakerTalk is one track's talk time.
type VoiceSpeakerTalk struct {
	Track     int    `json:"track"`
	Label     string `json:"label"`
	SteamID64 string `json:"steamid64,omitempty"`
	VoicePhaseTalk
}

// VoiceRoundTalk is every track's talk time in one round.
type VoiceRoundTalk struct {
	Round    int                `json:"round"`
	Speakers []VoiceSpeakerTalk `json:"speakers"`
}

// VoiceLine is one transcribed phrase placed on the demo.
type VoiceLine struct {
	Track     int    `json:"track"`
	Label     string `json:"label"`
	SteamID64 string `json:"steamid64,omitempty"`
	StartMS   int64  `json:"start_ms"`
	EndMS     int64  `json:"end_ms"`
	Tick      int    `json:"tick"`
	Round     int    `json:"round"`
	RoundMS   int64  `json:"round_ms"`
	Phase     string `json:"phase,omitempty"`
	Text      string `json:"text"`
}

// VoiceComms is a match's talk time by round and phase and its transcript.
type VoiceComms struct {
	MatchID    string             `json:"match_id"`
	Sync       VoiceSync          `json:"sync"`
	Transcript VoiceTranscript    `json:"transcript"`
	Rounds     []VoiceRoundTalk   `json:"rounds"`
	Totals     []VoiceSpeakerTalk `json:"totals"`
	Lines      []VoiceLine        `json:"lines"`
}

// VoiceSearchHit is a phrase found in a match's comms.
type VoiceSearchHit struct {
	MatchID  string    `json:"match_id"`
	Map      string    `json:"map"`
	PlayedAt time.Time `json:"played_at"`
	Teams    []string  `json:"teams"`
	VoiceLine
}

// VoiceSearchQuery searches every transcript; empty fields are unset.
type VoiceSearchQuery struct {
	Query   string
	MatchID string
	TeamID  string
	From    string
	To      string
	Limit   int
}

// VoiceTrackEdit names who speaks on a track; nil fields are kept.
type VoiceTrackEdit struct {
	MatchID   string  `json:"-"`
	Track     int     `json:"-"`
	Label     *string `json:"label,omitempty"`
	SteamID64 *string `json:"steamid64,omitempty"`
}

func voicePath(matchID string) string {
	return "/api/library/matches/" + url.PathEscape(matchID) + "/voice"
}

// ImportMatchVoice uploads a TeamSpeak recording to a match; the first import places it on its own.
func (c *Client) ImportMatchVoice(ctx context.Context, matchID, audioPath string) (MatchVoice, error) {
	var out MatchVoice
	err := c.uploadMultipart(ctx, upload{Path: voicePath(matchID), FileField: "audio", FilePath: audioPath}, &out)
	return out, err
}

// GetMatchVoice reads a match's voice; Present is false when it has none.
func (c *Client) GetMatchVoice(ctx context.Context, matchID string) (MatchVoice, error) {
	var out MatchVoice
	err := c.getJSON(ctx, voicePath(matchID), &out)
	return out, err
}

// SetVoiceOffset confirms an offset in milliseconds: audio time 0 plays at demo time offset.
func (c *Client) SetVoiceOffset(ctx context.Context, matchID string, offsetMS int64) (MatchVoice, error) {
	var out MatchVoice
	err := c.doJSON(ctx, http.MethodPatch, voicePath(matchID), map[string]int64{"offset_ms": offsetMS}, &out)
	return out, err
}

// AutoSyncVoice estimates the offset again.
func (c *Client) AutoSyncVoice(ctx context.Context, matchID string) (MatchVoice, error) {
	var out MatchVoice
	err := c.doJSON(ctx, http.MethodPatch, voicePath(matchID), map[string]bool{"auto": true}, &out)
	return out, err
}

// UpdateVoiceTrack sets a track's label or player.
func (c *Client) UpdateVoiceTrack(ctx context.Context, edit VoiceTrackEdit) (MatchVoice, error) {
	var out MatchVoice
	err := c.doJSON(ctx, http.MethodPatch, voicePath(edit.MatchID)+"/tracks/"+strconv.Itoa(edit.Track), edit, &out)
	return out, err
}

// DeleteMatchVoice removes a match's tracks, sync and transcript.
func (c *Client) DeleteMatchVoice(ctx context.Context, matchID string) error {
	return c.doJSON(ctx, http.MethodDelete, voicePath(matchID), nil, nil)
}

// TranscribeMatchVoice queues a local transcription in a language such as es or en.
func (c *Client) TranscribeMatchVoice(ctx context.Context, matchID, language string) (MatchVoice, error) {
	var out MatchVoice
	err := c.doJSON(ctx, http.MethodPost, voicePath(matchID)+"/transcribe", map[string]string{"language": language}, &out)
	return out, err
}

// MatchVoiceComms reads talk time and the transcript; round 0 reads every round.
func (c *Client) MatchVoiceComms(ctx context.Context, matchID string, round int) (VoiceComms, error) {
	path := voicePath(matchID) + "/comms"
	if round > 0 {
		path += "?round=" + strconv.Itoa(round)
	}
	var out VoiceComms
	err := c.getJSON(ctx, path, &out)
	return out, err
}

// SearchVoice finds a phrase in the library's transcripts.
func (c *Client) SearchVoice(ctx context.Context, q VoiceSearchQuery) ([]VoiceSearchHit, bool, error) {
	v := url.Values{"q": {q.Query}}
	for k, s := range map[string]string{"match_id": q.MatchID, "team_id": q.TeamID, "from": q.From, "to": q.To} {
		if s != "" {
			v.Set(k, s)
		}
	}
	if q.Limit > 0 {
		v.Set("limit", strconv.Itoa(q.Limit))
	}
	var out struct {
		Hits      []VoiceSearchHit `json:"hits"`
		Truncated bool             `json:"truncated"`
	}
	err := c.getJSON(ctx, "/api/library/voice/search?"+v.Encode(), &out)
	return out.Hits, out.Truncated, err
}

// VoiceTranscriberStatus reads the local transcription install.
func (c *Client) VoiceTranscriberStatus(ctx context.Context) (VoiceTranscriber, error) {
	var out VoiceTranscriber
	err := c.getJSON(ctx, "/api/library/voice/transcriber", &out)
	return out, err
}

// InstallVoiceTranscriber starts the whisper.cpp download in Studio.
func (c *Client) InstallVoiceTranscriber(ctx context.Context) (VoiceTranscriber, error) {
	var out VoiceTranscriber
	err := c.doJSON(ctx, http.MethodPost, "/api/library/voice/transcriber/install", nil, &out)
	return out, err
}
