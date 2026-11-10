package tuiclient

import (
	"context"
	"net/http"
	"net/url"
	"time"
)

// RecordingTrack is one person of a TeamSpeak plugin recording.
type RecordingTrack struct {
	File       string `json:"file"`
	Nickname   string `json:"nickname"`
	UID        string `json:"uid,omitempty"`
	Self       bool   `json:"self,omitempty"`
	DurationMS int64  `json:"duration_ms"`
	MatchTrack int    `json:"match_track,omitempty"`
	Failed     string `json:"failed,omitempty"`
}

// VoiceRecording is a recording the TeamSpeak plugin left for Studio.
type VoiceRecording struct {
	ID            string           `json:"id"`
	Status        string           `json:"status"`
	StartedAt     time.Time        `json:"started_at"`
	EndedAt       time.Time        `json:"ended_at"`
	DurationMS    int64            `json:"duration_ms"`
	Map           string           `json:"map,omitempty"`
	Server        string           `json:"server,omitempty"`
	Channel       string           `json:"channel,omitempty"`
	Trigger       string           `json:"trigger"`
	SelfSteamID64 string           `json:"self_steamid64,omitempty"`
	ScoreCT       int              `json:"score_ct"`
	ScoreT        int              `json:"score_t"`
	Interrupted   bool             `json:"interrupted,omitempty"`
	Tracks        []RecordingTrack `json:"tracks"`
	MatchID       string           `json:"match_id,omitempty"`
	Note          string           `json:"note,omitempty"`
}

// VoiceSpeakerEdit says which match player speaks on a track.
type VoiceSpeakerEdit struct {
	Track     int    `json:"track"`
	SteamID64 string `json:"steamid64"`
}

// VoicePluginStatus is the TeamSpeak plugin's side in Studio: the inbox, CS2's config and the live match.
type VoicePluginStatus struct {
	Inbox      string `json:"inbox"`
	WebOrigin  string `json:"web_origin"`
	CS2Found   bool   `json:"cs2_found"`
	GameState  string `json:"game_state"`
	ConfigPath string `json:"config_path,omitempty"`
	Live       struct {
		Live    bool   `json:"live"`
		Map     string `json:"map,omitempty"`
		ScoreCT int    `json:"score_ct"`
		ScoreT  int    `json:"score_t"`
		Rounds  []struct {
			Round int `json:"round"`
		} `json:"rounds"`
	} `json:"live"`
	Pending int `json:"pending"`
}

// ListVoiceRecordings reads the plugin's recordings, newest first.
func (c *Client) ListVoiceRecordings(ctx context.Context) ([]VoiceRecording, error) {
	var out struct {
		Recordings []VoiceRecording `json:"recordings"`
	}
	err := c.getJSON(ctx, "/api/library/voice/recordings", &out)
	return out.Recordings, err
}

// AttachVoiceRecording adds a recording's tracks to a match and places them on its demo.
func (c *Client) AttachVoiceRecording(ctx context.Context, id, matchID string) (VoiceRecording, error) {
	var out VoiceRecording
	err := c.doJSON(ctx, http.MethodPost, "/api/library/voice/recordings/"+url.PathEscape(id)+"/attach", map[string]string{"match_id": matchID}, &out)
	return out, err
}

// DeleteVoiceRecording removes a recording's folder; a match keeps the tracks it took.
func (c *Client) DeleteVoiceRecording(ctx context.Context, id string) error {
	return c.doJSON(ctx, http.MethodDelete, "/api/library/voice/recordings/"+url.PathEscape(id), nil, nil)
}

// SetVoiceSpeakers ties tracks to players; the players' teams remember it for the next recordings.
func (c *Client) SetVoiceSpeakers(ctx context.Context, matchID string, speakers []VoiceSpeakerEdit) (MatchVoice, error) {
	var out MatchVoice
	err := c.doJSON(ctx, http.MethodPut, voicePath(matchID)+"/speakers", map[string][]VoiceSpeakerEdit{"speakers": speakers}, &out)
	return out, err
}

// VoicePlugin reads the plugin's status; setup installs CS2's game state config first.
func (c *Client) VoicePlugin(ctx context.Context, setup bool) (VoicePluginStatus, error) {
	var out VoicePluginStatus
	if setup {
		err := c.doJSON(ctx, http.MethodPost, "/api/library/voice/plugin/setup", nil, &out)
		return out, err
	}
	err := c.getJSON(ctx, "/api/library/voice/plugin", &out)
	return out, err
}
