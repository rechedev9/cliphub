package tuiclient

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
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
	VictimReplay  bool   `json:"victim_replay,omitempty"`
	CoverStrategy string `json:"cover_strategy"`
	DeathReel     bool   `json:"death_reel,omitempty"`
	// CenteredKillfeed is omitted when off, like the server's own field.
	CenteredKillfeed bool `json:"centered_killfeed,omitempty"`
	// IntroText and OutroText replace the generated card text; empty keeps it.
	IntroText string `json:"intro_text,omitempty"`
	OutroText string `json:"outro_text,omitempty"`
	// The affiliate banner (renderplan.EditRequest keydrop_*); an empty style means no banner.
	KeyDropFamily       string   `json:"keydrop_family,omitempty"`
	KeyDropStyle        string   `json:"keydrop_style,omitempty"`
	KeyDropCode         string   `json:"keydrop_code,omitempty"`
	KeyDropStartSeconds *float64 `json:"keydrop_start_seconds,omitempty"`
	KeyDropEndSeconds   *float64 `json:"keydrop_end_seconds,omitempty"`

	// VictimReplaySpeed is 0.25 or 1; 0 keeps the default half speed.
	VictimReplaySpeed float64 `json:"victim_replay_speed,omitempty"`
	PlayBadges        bool    `json:"play_badges,omitempty"`
	StatsCard         bool    `json:"stats_card,omitempty"`
	// BrandWatermark and BrandBanner draw the saved brand kit (GetBrandKit).
	BrandWatermark bool `json:"brand_watermark,omitempty"`
	BrandBanner    bool `json:"brand_banner,omitempty"`
	// SeparateShorts renders each segment as its own Short from one capture (Auto-Shorts).
	SeparateShorts bool `json:"separate_shorts,omitempty"`
}

// ShortMusic is the "music" object of a Short: a track id from ListSongs and its mix.
// Volume is in (0,1], 0 keeps full volume; a nil GameVolume keeps the 0.70 game mix.
type ShortMusic struct {
	Key        string   `json:"key"`
	Volume     float64  `json:"volume,omitempty"`
	GameVolume *float64 `json:"game_volume,omitempty"`
}

// GenerateRequest is the body of POST /api/jobs/{id}/generate for a Short.
// A nil Music renders without music.
type GenerateRequest struct {
	Preset     string      `json:"preset"`
	SegmentIDs []string    `json:"segment_ids"`
	Music      *ShortMusic `json:"music,omitempty"`
	Edit       ShortEdit   `json:"edit"`
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

// ShortsBatch is GET /api/jobs/{id}/shorts-batch: the best play of each round
// over MinScore, at most Limit of them, in match order.
type ShortsBatch struct {
	MinScore   float64  `json:"min_score"`
	Limit      int      `json:"limit"`
	SegmentIDs []string `json:"segment_ids"`
	Moments    []Moment `json:"moments"`
}

// ShortsBatchQuery asks for one job's Auto-Shorts choice; zero options keep the server defaults.
type ShortsBatchQuery struct {
	JobID    string
	MinScore float64
	Limit    int
}

// GetShortsBatch reads the Auto-Shorts choice of a job.
func (c *Client) GetShortsBatch(ctx context.Context, q ShortsBatchQuery) (ShortsBatch, error) {
	values := url.Values{}
	if q.MinScore > 0 {
		values.Set("min_score", strconv.FormatFloat(q.MinScore, 'f', -1, 64))
	}
	if q.Limit > 0 {
		values.Set("limit", strconv.Itoa(q.Limit))
	}
	path := jobPath(q.JobID) + "/shorts-batch"
	if len(values) > 0 {
		path += "?" + values.Encode()
	}
	var out ShortsBatch
	err := c.getJSON(ctx, path, &out)
	return out, err
}
