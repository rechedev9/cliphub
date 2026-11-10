package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"time"

	"github.com/google/uuid"

	"github.com/rechedev9/cliphub/internal/library"
	"github.com/rechedev9/cliphub/internal/replayvideo"
)

const (
	// maxReplayVideoBytes is one export: the spec and up to 120 s of 60 fps JPEG frames.
	maxReplayVideoBytes = 3 << 30
	maxReplayVideoSpec  = 64 << 10
	// replayVideoKeep is how long a finished MP4 waits for its download.
	replayVideoKeep  = time.Hour
	replayVideoSweep = 15 * time.Minute

	codeReplayVideoUnavailable = "replay_video_unavailable"
	codeReplayVideoFailed      = "replay_video_failed"
	codeReplayVideoInvalid     = "invalid_request"
	codeVoiceUnsynced          = "voice_unsynced"
	codeVoiceOutside           = "voice_outside"
)

// ReplayVideoEncoder starts one MP4 encode; replayvideo.Encoder runs FFmpeg for it.
type ReplayVideoEncoder interface {
	Start(ctx context.Context, p replayvideo.Plan) (replayvideo.Sink, error)
}

// ReplayVideoDeps enables the Replay's MP4 export: Dir holds each MP4 until its download.
type ReplayVideoDeps struct {
	Encoder ReplayVideoEncoder
	Dir     string
}

// WithReplayVideo enables POST /api/library/matches/{id}/replay-video.
func WithReplayVideo(deps ReplayVideoDeps) Option {
	return func(h *Handlers) {
		if deps.Encoder != nil && deps.Dir != "" {
			h.replayVideo = &deps
		}
	}
}

// replayVideoSpec is the "spec" part that opens a Replay video upload; "frame" parts follow it.
type replayVideoSpec struct {
	FPS          int     `json:"fps"`
	Width        int     `json:"width"`
	Height       int     `json:"height"`
	Speed        float64 `json:"speed"`
	IncludeVoice bool    `json:"include_voice"`
	VoiceTracks  []int   `json:"voice_tracks"`
	// OffsetMS is the offset the user hears, an unsaved adjustment included; the saved one when nil.
	OffsetMS *int64               `json:"offset_ms"`
	Segments []replayVideoSegment `json:"segments"`
}

// replayVideoSegment is one round's stretch: the demo ticks it shows, its frame count and, when
// the voice stops earlier (only freeze time), the tick it goes quiet at.
type replayVideoSegment struct {
	Round       int `json:"round"`
	FromTick    int `json:"from_tick"`
	ToTick      int `json:"to_tick"`
	Frames      int `json:"frames"`
	VoiceToTick int `json:"voice_to_tick"`
}

type replayVideoResult struct {
	VideoID string  `json:"video_id"`
	Bytes   int64   `json:"bytes"`
	Frames  int     `json:"frames"`
	Seconds float64 `json:"seconds"`
	Voice   bool    `json:"voice"`
}

// refusal is a request the export cannot take, with the code the Replay words in Spanish.
type refusal struct {
	status    int
	code, msg string
}

func (e refusal) Error() string { return e.msg }

func invalid(msg string) refusal { return refusal{http.StatusBadRequest, codeReplayVideoInvalid, msg} }

// replayVideoPlan turns a spec into the encode plan, without the voice tracks.
func replayVideoPlan(spec replayVideoSpec, m library.Match) (replayvideo.Plan, error) {
	speed := spec.Speed
	if speed == 0 {
		speed = 1
	}
	p := replayvideo.Plan{FPS: spec.FPS, Width: spec.Width, Height: spec.Height, Speed: speed}
	for _, s := range spec.Segments {
		if s.Round < 1 || (m.RoundCount > 0 && s.Round > m.RoundCount) {
			return p, fmt.Errorf("round %d is not in this match", s.Round)
		}
		if s.FromTick < 0 || s.ToTick <= s.FromTick || (m.DurationTicks > 0 && s.ToTick > m.DurationTicks) {
			return p, errors.New("each segment needs from_tick before to_tick, inside the demo")
		}
		voiceTo := s.ToTick
		if s.VoiceToTick != 0 {
			voiceTo = min(voiceTo, s.VoiceToTick)
		}
		p.Segments = append(p.Segments, replayvideo.Segment{Frames: s.Frames, Voice: &replayvideo.Range{FromTick: s.FromTick, ToTick: voiceTo}})
	}
	return p, p.Validate()
}

// replayVideoVoice adds the match's team voice to a plan: the tracks asked for (every one by
// default) on the offset the spec carries, or the saved one.
func (h *Handlers) replayVideoVoice(r *http.Request, m library.Match, spec replayVideoSpec) ([]replayvideo.Track, replayvideo.Sync, error) {
	if h.voice == nil || h.voice.deps.Dir == "" {
		return nil, replayvideo.Sync{}, refusal{http.StatusServiceUnavailable, codeVoiceUnavailable, "team voice needs FFmpeg, which this Studio did not start with"}
	}
	v, err := h.libraryRepo.GetMatchVoice(r.Context(), m.ID)
	if errors.Is(err, library.ErrNotFound) {
		return nil, replayvideo.Sync{}, refusal{http.StatusConflict, codeNoVoice, "this match has no team voice; import a recording first"}
	}
	if err != nil {
		return nil, replayvideo.Sync{}, err
	}
	offset := v.Sync.OffsetMS
	switch {
	case spec.OffsetMS != nil && (*spec.OffsetMS < -library.MaxVoiceOffsetMS || *spec.OffsetMS > library.MaxVoiceOffsetMS):
		return nil, replayvideo.Sync{}, invalid("offset_ms must be within six hours")
	case spec.OffsetMS != nil:
		offset = *spec.OffsetMS
	case v.Sync.State == library.VoiceUnsynced:
		return nil, replayvideo.Sync{}, refusal{http.StatusConflict, codeVoiceUnsynced, "the team voice is not synced with the demo yet"}
	}
	var tracks []replayvideo.Track
	for _, t := range v.Tracks {
		if len(spec.VoiceTracks) > 0 && !slices.Contains(spec.VoiceTracks, t.Track) {
			continue
		}
		path, err := h.voice.trackPath(t)
		if err != nil {
			return nil, replayvideo.Sync{}, err
		}
		tracks = append(tracks, replayvideo.Track{Path: path, DurationMS: t.DurationMS})
	}
	if len(tracks) == 0 {
		return nil, replayvideo.Sync{}, invalid("voice_tracks names no track of this match")
	}
	rate := m.Tickrate
	if rate <= 0 {
		rate = 64
	}
	return tracks, replayvideo.Sync{Tickrate: rate, OffsetMS: offset}, nil
}

// sweepReplayVideos removes the MP4s nobody downloaded.
func sweepReplayVideos(dir string, now time.Time) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	for _, e := range entries {
		info, err := e.Info()
		if err == nil && now.Sub(info.ModTime()) > replayVideoKeep {
			_ = os.Remove(filepath.Join(dir, e.Name()))
		}
	}
}

// StartReplayVideoSweep removes stale MP4s now and every 15 minutes until ctx ends.
func (h *Handlers) StartReplayVideoSweep(ctx context.Context) {
	if h.replayVideo == nil {
		return
	}
	dir := h.replayVideo.Dir
	sweepReplayVideos(dir, time.Now())
	go func() {
		tick := time.NewTicker(replayVideoSweep)
		defer tick.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case now := <-tick.C:
				sweepReplayVideos(dir, now)
			}
		}
	}()
}

func writeRefusal(w http.ResponseWriter, err error) bool {
	var refused refusal
	if !errors.As(err, &refused) {
		return false
	}
	writeCodedError(w, refused.status, refused.code, refused.msg)
	return true
}

// CreateLibraryReplayVideo handles POST /api/library/matches/{id}/replay-video: a "spec" part,
// then the frames as "frame" JPEG parts in order. It answers once the MP4 is encoded.
func (h *Handlers) CreateLibraryReplayVideo(w http.ResponseWriter, r *http.Request) {
	if !h.libraryReady(w) {
		return
	}
	if h.replayVideo == nil {
		writeCodedError(w, http.StatusServiceUnavailable, codeReplayVideoUnavailable, "the Replay video needs FFmpeg, which this Studio did not start with")
		return
	}
	id, ok := libraryUUIDParam(w, r, "id")
	if !ok {
		return
	}
	m, err := h.libraryRepo.GetMatch(r.Context(), id)
	if err != nil {
		writeLibraryError(w, "get match", err)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxReplayVideoBytes)
	reader, err := r.MultipartReader()
	if err != nil {
		writeCodedError(w, http.StatusBadRequest, codeReplayVideoInvalid, "body must be multipart/form-data")
		return
	}
	plan, err := h.readReplayVideoSpec(r, reader, m)
	if err != nil {
		if !writeRefusal(w, err) {
			internalError(w, "replay video voice", err)
		}
		return
	}
	dir := h.replayVideo.Dir
	if err := os.MkdirAll(dir, 0o750); err != nil {
		internalError(w, "replay video dir", err)
		return
	}
	videoID := uuid.New()
	plan.Out = filepath.Join(dir, videoID.String()+".mp4")
	res, err := encodeReplayVideo(replayVideoEncode{r: r, reader: reader, plan: plan, encoder: h.replayVideo.Encoder})
	if err != nil {
		_ = os.Remove(plan.Out)
		if writeRefusal(w, err) {
			return
		}
		log.Printf("httpapi: replay video of match %s: %v", m.ID, err)
		writeCodedError(w, http.StatusInternalServerError, codeReplayVideoFailed, "the MP4 could not be encoded")
		return
	}
	res.VideoID = videoID.String()
	writeJSON(w, http.StatusOK, res)
}

// readReplayVideoSpec reads the first part: the plan, with the voice when asked for.
func (h *Handlers) readReplayVideoSpec(r *http.Request, reader *multipart.Reader, m library.Match) (replayvideo.Plan, error) {
	part, err := reader.NextPart()
	if err != nil || part.FormName() != "spec" {
		return replayvideo.Plan{}, invalid("the first part must be the spec")
	}
	raw, err := io.ReadAll(io.LimitReader(part, maxReplayVideoSpec+1))
	_ = part.Close()
	if err != nil || len(raw) > maxReplayVideoSpec {
		return replayvideo.Plan{}, invalid("the spec takes at most 64 KiB")
	}
	var spec replayVideoSpec
	if err := decodeStrictJSON(raw, &spec); err != nil {
		return replayvideo.Plan{}, invalid("spec: " + err.Error())
	}
	plan, err := replayVideoPlan(spec, m)
	if err != nil {
		return replayvideo.Plan{}, invalid(err.Error())
	}
	if !spec.IncludeVoice {
		return plan, nil
	}
	plan.Tracks, plan.Sync, err = h.replayVideoVoice(r, m, spec)
	if err != nil {
		return replayvideo.Plan{}, err
	}
	if plan.Voices() == nil {
		return replayvideo.Plan{}, refusal{http.StatusConflict, codeVoiceOutside, "the team voice recording does not cover this stretch"}
	}
	return plan, nil
}

func decodeStrictJSON(raw []byte, dst any) error {
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	return dec.Decode(dst)
}
