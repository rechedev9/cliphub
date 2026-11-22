package tuiclient

import (
	"context"
	"net/http"
	"net/url"
)

type StreamVoiceSpan struct {
	StartMS int64 `json:"start_ms"`
	EndMS   int64 `json:"end_ms"`
}

type StreamVoiceLine struct {
	ID           string            `json:"id"`
	StartMS      int64             `json:"start_ms"`
	EndMS        int64             `json:"end_ms"`
	Text         string            `json:"text"`
	Corrected    bool              `json:"corrected"`
	OriginalText string            `json:"original_text,omitempty"`
	VocabText    string            `json:"vocab_text,omitempty"`
	VocabChanges []DemoVocabChange `json:"vocab_changes,omitempty"`
	Discarded    bool              `json:"discarded"`
}

type StreamSubtitles struct {
	Shorts  bool                `json:"shorts"`
	Summary bool                `json:"summary"`
	Hidden  map[string][]string `json:"hidden"`
}

type StreamTranscript struct {
	JobID        string               `json:"job_id"`
	State        string               `json:"state"`
	Error        string               `json:"error,omitempty"`
	Progress     float64              `json:"progress,omitempty"`
	Availability string               `json:"availability"`
	Spans        []StreamVoiceSpan    `json:"spans"`
	Pending      []StreamVoiceSpan    `json:"pending"`
	Lines        []StreamVoiceLine    `json:"lines"`
	Transcriber  VoiceTranscriber     `json:"transcriber"`
	Subtitles    StreamSubtitles      `json:"subtitles"`
	Stale        StreamSubtitlesStale `json:"stale"`
}

type StreamSubtitlesStale struct {
	Shorts  bool `json:"shorts"`
	Summary bool `json:"summary"`
}

type StreamStretch struct {
	StartSeconds float64 `json:"start_seconds"`
	EndSeconds   float64 `json:"end_seconds"`
}

type StreamTranscribe struct {
	JobID string          `json:"-"`
	Spans []StreamStretch `json:"spans"`
	Force bool            `json:"force,omitempty"`
}

type StreamLineEdit struct {
	JobID     string  `json:"-"`
	LineID    string  `json:"-"`
	Text      *string `json:"text,omitempty"`
	Discarded *bool   `json:"discarded,omitempty"`
}

type StreamSubtitlesEdit struct {
	JobID   string `json:"-"`
	Shorts  *bool  `json:"shorts,omitempty"`
	Summary *bool  `json:"summary,omitempty"`
	Video   string `json:"video,omitempty"`
	LineID  string `json:"line_id,omitempty"`
	Hidden  *bool  `json:"hidden,omitempty"`
}

func streamTranscriptPath(jobID string) string {
	return "/api/stream-jobs/" + url.PathEscape(jobID) + "/transcript"
}

func (c *Client) StreamTranscript(ctx context.Context, jobID string) (StreamTranscript, error) {
	var out StreamTranscript
	err := c.getJSON(ctx, streamTranscriptPath(jobID), &out)
	return out, err
}

func (c *Client) TranscribeStream(ctx context.Context, req StreamTranscribe) (StreamTranscript, error) {
	var out StreamTranscript
	err := c.doJSON(ctx, http.MethodPost, streamTranscriptPath(req.JobID), req, &out)
	return out, err
}

func (c *Client) EditStreamVoiceLine(ctx context.Context, edit StreamLineEdit) (StreamVoiceLine, error) {
	var out StreamVoiceLine
	err := c.doJSON(ctx, http.MethodPatch, streamTranscriptPath(edit.JobID)+"/lines/"+url.PathEscape(edit.LineID), edit, &out)
	return out, err
}

func (c *Client) EditStreamSubtitles(ctx context.Context, edit StreamSubtitlesEdit) (StreamSubtitles, error) {
	var out StreamSubtitles
	err := c.doJSON(ctx, http.MethodPatch, "/api/stream-jobs/"+url.PathEscape(edit.JobID)+"/subtitles", edit, &out)
	return out, err
}
