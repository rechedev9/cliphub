package tuiclient

import (
	"context"
	"net/http"
	"net/url"
)

// DemoVoiceLine is one phrase of the transcript of a demo's team voices.
type DemoVoiceLine struct {
	ID        string `json:"id"`
	SteamID64 string `json:"steamid64"`
	Name      string `json:"name"`
	TeamKey   string `json:"team_key"`
	StartMS   int64  `json:"start_ms"`
	EndMS     int64  `json:"end_ms"`
	Tick      int    `json:"tick"`
	Round     int    `json:"round"`
	Text      string `json:"text"`
	Corrected bool   `json:"corrected"`

	OriginalText string `json:"original_text,omitempty"`
	Discarded    bool   `json:"discarded"`
}

type DemoLineDiscard struct {
	MatchID   string `json:"-"`
	LineID    string `json:"-"`
	Discarded bool   `json:"discarded"`
}

func (c *Client) DiscardDemoVoiceLine(ctx context.Context, edit DemoLineDiscard) (DemoVoiceLine, error) {
	var out DemoVoiceLine
	err := c.doJSON(ctx, http.MethodPatch, demoTranscriptPath(edit.MatchID)+"/lines/"+url.PathEscape(edit.LineID), edit, &out)
	return out, err
}

// DemoTranscript is a demo's voice transcript and the local install it needs.
type DemoTranscript struct {
	MatchID     string           `json:"match_id"`
	State       string           `json:"state"`
	Error       string           `json:"error,omitempty"`
	Progress    float64          `json:"progress,omitempty"`
	Voice       string           `json:"voice"`
	Lines       []DemoVoiceLine  `json:"lines"`
	Transcriber VoiceTranscriber `json:"transcriber"`
}

// DemoTranscribe asks for a demo's transcript; Force runs a done one again.
type DemoTranscribe struct {
	MatchID string `json:"-"`
	Force   bool   `json:"force,omitempty"`
}

// DemoLineEdit corrects a phrase's text for every later video of the demo.
type DemoLineEdit struct {
	MatchID string `json:"-"`
	LineID  string `json:"-"`
	Text    string `json:"text"`
}

func demoTranscriptPath(matchID string) string {
	return voicePath(matchID) + "/demo/transcript"
}

// DemoTranscript reads the transcript of a match demo's team voices.
func (c *Client) DemoTranscript(ctx context.Context, matchID string) (DemoTranscript, error) {
	var out DemoTranscript
	err := c.getJSON(ctx, demoTranscriptPath(matchID), &out)
	return out, err
}

// TranscribeDemoVoice queues the transcript of a demo's team voices; a done one is reused.
func (c *Client) TranscribeDemoVoice(ctx context.Context, req DemoTranscribe) (DemoTranscript, error) {
	var out DemoTranscript
	err := c.doJSON(ctx, http.MethodPost, demoTranscriptPath(req.MatchID), req, &out)
	return out, err
}

// CorrectDemoVoiceLine stores a corrected phrase.
func (c *Client) CorrectDemoVoiceLine(ctx context.Context, edit DemoLineEdit) (DemoVoiceLine, error) {
	var out DemoVoiceLine
	err := c.doJSON(ctx, http.MethodPatch, demoTranscriptPath(edit.MatchID)+"/lines/"+url.PathEscape(edit.LineID), edit, &out)
	return out, err
}
