package tuiclient

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

// Song is one music track of GET /api/songs.
type Song struct {
	ID          string `json:"id"`
	Title       string `json:"title"`
	Artist      string `json:"artist,omitempty"`
	Genre       string `json:"genre,omitempty"`
	DurationSec int    `json:"durationSec,omitempty"`
}

// ShortEdit is the "edit" object of POST /api/jobs/{id}/generate for a Short.
type ShortEdit struct {
	Format        string `json:"format"`
	KillEffect    string `json:"killEffect"`
	Transition    string `json:"transition"`
	Intro         bool   `json:"intro"`
	Outro         bool   `json:"outro"`
	HookText      bool   `json:"hook_text"`
	KillCounter   bool   `json:"kill_counter"`
	VoiceComms    bool   `json:"voice_comms"`
	NativeHUD     bool   `json:"native_hud"`
	CoverStrategy string `json:"cover_strategy"`
}

// GenerateRequest is the body of POST /api/jobs/{id}/generate for a Short.
// Music is a track id from ListSongs; empty renders without music.
type GenerateRequest struct {
	Preset     string    `json:"preset"`
	SegmentIDs []string  `json:"segment_ids"`
	Music      string    `json:"music,omitempty"`
	Edit       ShortEdit `json:"edit"`
}

// ErrAudioTooLarge is returned by SongAudio when a track exceeds the cap.
var ErrAudioTooLarge = errors.New("audio file is too large")

// ListSongs lists the music tracks a Short can use.
func (c *Client) ListSongs(ctx context.Context) ([]Song, error) {
	var out struct {
		Songs []Song `json:"songs"`
	}
	err := c.getJSON(ctx, "/api/songs", &out)
	return out.Songs, err
}

// SongAudio downloads one track and returns its bytes and MIME type. It stops
// with ErrAudioTooLarge once the body passes maxBytes.
func (c *Client) SongAudio(ctx context.Context, id string, maxBytes int64) ([]byte, string, error) {
	var buf bytes.Buffer
	contentType, err := c.stream(ctx, "/api/songs/"+url.PathEscape(id)+"/audio", &capWriter{buf: &buf, max: maxBytes})
	if err != nil {
		if errors.Is(err, ErrAudioTooLarge) {
			return nil, "", ErrAudioTooLarge
		}
		return nil, "", err
	}
	mime, _, _ := strings.Cut(contentType, ";")
	return buf.Bytes(), strings.TrimSpace(mime), nil
}

type capWriter struct {
	buf *bytes.Buffer
	max int64
}

func (w *capWriter) Write(p []byte) (int, error) {
	if int64(w.buf.Len()+len(p)) > w.max {
		return 0, ErrAudioTooLarge
	}
	return w.buf.Write(p)
}

// StartShort queues the capture and edit of the chosen plan segments.
func (c *Client) StartShort(ctx context.Context, id string, req GenerateRequest) (EnqueueResponse, error) {
	if req.Preset == "" || len(req.SegmentIDs) == 0 {
		return EnqueueResponse{}, fmt.Errorf("generate needs a preset and at least one segment")
	}
	var out EnqueueResponse
	err := c.doJSON(ctx, http.MethodPost, jobPath(id)+"/generate", req, &out)
	return out, err
}
