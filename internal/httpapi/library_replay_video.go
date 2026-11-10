package httpapi

import (
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
	"strings"
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
	replayVideoKeep = time.Hour

	codeReplayVideoUnavailable = "replay_video_unavailable"
	codeReplayVideoFailed      = "replay_video_failed"
	codeVoiceUnsynced          = "voice_unsynced"
	codeVoiceOutside           = "voice_outside"
)

// replayVideoSpec is the "spec" part that opens a Replay video upload; "frame" parts follow it.
type replayVideoSpec struct {
	FPS          int                  `json:"fps"`
	Width        int                  `json:"width"`
	Height       int                  `json:"height"`
	Speed        float64              `json:"speed"`
	IncludeVoice bool                 `json:"include_voice"`
	VoiceTracks  []int                `json:"voice_tracks"`
	Segments     []replayVideoSegment `json:"segments"`
}

// replayVideoSegment is one round's stretch: the demo ticks it shows and its frame count.
type replayVideoSegment struct {
	Round    int `json:"round"`
	FromTick int `json:"from_tick"`
	ToTick   int `json:"to_tick"`
	Frames   int `json:"frames"`
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

func replayVideoDir() string { return filepath.Join(os.TempDir(), "cliphub-replay-video") }

// replayVideoPlan turns a spec into the encode plan, without the voice.
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
		p.Segments = append(p.Segments, replayvideo.Segment{Frames: s.Frames, Voice: &replayvideo.Range{FromTick: s.FromTick, ToTick: s.ToTick}})
	}
	return p, p.Validate()
}

// replayVideoVoice adds the match's team voice to a plan: the tracks asked for (every one by
// default) on the saved sync offset.
func (h *Handlers) replayVideoVoice(r *http.Request, m library.Match, want []int) ([]replayvideo.Track, replayvideo.Sync, error) {
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
	if v.Sync.State == library.VoiceUnsynced {
		return nil, replayvideo.Sync{}, refusal{http.StatusConflict, codeVoiceUnsynced, "the team voice is not synced with the demo yet"}
	}
	var tracks []replayvideo.Track
	for _, t := range v.Tracks {
		if len(want) > 0 && !slices.Contains(want, t.Track) {
			continue
		}
		path, err := h.voice.trackPath(t)
		if err != nil {
			return nil, replayvideo.Sync{}, err
		}
		tracks = append(tracks, replayvideo.Track{Path: path, DurationMS: t.DurationMS})
	}
	if len(tracks) == 0 {
		return nil, replayvideo.Sync{}, refusal{http.StatusBadRequest, "bad_request", "voice_tracks names no track of this match"}
	}
	rate := m.Tickrate
	if rate <= 0 {
		rate = 64
	}
	return tracks, replayvideo.Sync{Tickrate: rate, OffsetMS: v.Sync.OffsetMS}, nil
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

// CreateLibraryReplayVideo handles POST /api/library/matches/{id}/replay-video: a "spec" part,
// then the frames as "frame" JPEG parts in order. It answers once the MP4 is encoded.
func (h *Handlers) CreateLibraryReplayVideo(w http.ResponseWriter, r *http.Request) {
	if !h.libraryReady(w) {
		return
	}
	if h.streamFFmpeg == "" {
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
		writeError(w, http.StatusBadRequest, "body must be multipart/form-data")
		return
	}
	plan, err := h.readReplayVideoSpec(r, reader, m)
	var refused refusal
	switch {
	case errors.As(err, &refused):
		writeCodedError(w, refused.status, refused.code, refused.msg)
		return
	case err != nil:
		internalError(w, "replay video voice", err)
		return
	}
	dir := replayVideoDir()
	if err := os.MkdirAll(dir, 0o750); err != nil {
		internalError(w, "replay video dir", err)
		return
	}
	sweepReplayVideos(dir, time.Now())
	videoID := uuid.New()
	plan.Out = filepath.Join(dir, videoID.String()+".mp4")
	res, err := encodeReplayVideo(replayVideoEncode{r: r, reader: reader, plan: plan, encoder: replayvideo.Encoder{FFmpeg: h.streamFFmpeg}})
	if err != nil {
		_ = os.Remove(plan.Out)
		if errors.As(err, &refused) {
			writeCodedError(w, refused.status, refused.code, refused.msg)
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
	bad := func(msg string) refusal { return refusal{http.StatusBadRequest, "bad_request", msg} }
	part, err := reader.NextPart()
	if err != nil || part.FormName() != "spec" {
		return replayvideo.Plan{}, bad("the first part must be the spec")
	}
	raw, err := io.ReadAll(io.LimitReader(part, maxReplayVideoSpec+1))
	_ = part.Close()
	if err != nil || len(raw) > maxReplayVideoSpec {
		return replayvideo.Plan{}, bad("the spec takes at most 64 KiB")
	}
	var spec replayVideoSpec
	dec := json.NewDecoder(strings.NewReader(string(raw)))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&spec); err != nil {
		return replayvideo.Plan{}, bad("spec: " + err.Error())
	}
	plan, err := replayVideoPlan(spec, m)
	if err != nil {
		return replayvideo.Plan{}, bad(err.Error())
	}
	if !spec.IncludeVoice {
		return plan, nil
	}
	plan.Tracks, plan.Sync, err = h.replayVideoVoice(r, m, spec.VoiceTracks)
	if err != nil {
		return replayvideo.Plan{}, err
	}
	if plan.Voices() == nil {
		return replayvideo.Plan{}, refusal{http.StatusConflict, codeVoiceOutside, "the team voice recording does not cover this stretch"}
	}
	return plan, nil
}

type replayVideoEncode struct {
	r       *http.Request
	reader  *multipart.Reader
	plan    replayvideo.Plan
	encoder replayvideo.Encoder
}

// encodeReplayVideo feeds the "frame" parts to FFmpeg as they arrive and waits for the MP4.
func encodeReplayVideo(e replayVideoEncode) (replayVideoResult, error) {
	sink, err := e.encoder.Start(e.r.Context(), e.plan)
	if err != nil {
		return replayVideoResult{}, err
	}
	want, got := e.plan.Frames(), 0
	fail := func(err error) (replayVideoResult, error) {
		sink.Abort()
		return replayVideoResult{}, err
	}
	for {
		part, err := e.reader.NextPart()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return fail(refusal{http.StatusBadRequest, "bad_request", "reading multipart body: " + err.Error()})
		}
		if part.FormName() != "frame" {
			_ = part.Close()
			continue
		}
		if got == want {
			_ = part.Close()
			return fail(refusal{http.StatusBadRequest, "bad_request", fmt.Sprintf("the spec takes %d frames", want)})
		}
		err = sink.Frame(part)
		_ = part.Close()
		if errors.Is(err, replayvideo.ErrNotJPEG) {
			return fail(refusal{http.StatusBadRequest, "bad_request", fmt.Sprintf("frame %d is not a JPEG", got+1)})
		}
		if err != nil {
			return fail(err)
		}
		got++
	}
	if got != want {
		return fail(refusal{http.StatusBadRequest, "bad_request", fmt.Sprintf("got %d frames, the spec takes %d", got, want)})
	}
	voice, err := sink.Finish()
	if err != nil {
		return replayVideoResult{}, err
	}
	info, err := os.Stat(e.plan.Out)
	if err != nil {
		return replayVideoResult{}, err
	}
	return replayVideoResult{Bytes: info.Size(), Frames: want, Seconds: e.plan.Seconds(), Voice: voice}, nil
}

// GetLibraryReplayVideo handles GET /api/library/replay-videos/{video}: the encoded MP4, once.
func (h *Handlers) GetLibraryReplayVideo(w http.ResponseWriter, r *http.Request) {
	id, ok := libraryUUIDParam(w, r, "video")
	if !ok {
		return
	}
	path := filepath.Join(replayVideoDir(), id.String()+".mp4")
	f, err := os.Open(path) // #nosec G304 -- the name is a parsed UUID inside Studio's own folder.
	if err != nil {
		writeError(w, http.StatusNotFound, "no such replay video; export it again")
		return
	}
	info, err := f.Stat()
	if err != nil {
		_ = f.Close()
		internalError(w, "stat replay video", err)
		return
	}
	w.Header().Set("Content-Type", "video/mp4")
	w.Header().Set("Cache-Control", "no-store")
	http.ServeContent(w, r, "", info.ModTime(), f)
	_ = f.Close()
	// A range read is a player seeking; only the whole file is the download that ends its life.
	if r.Header.Get("Range") == "" {
		_ = os.Remove(path)
	}
}
