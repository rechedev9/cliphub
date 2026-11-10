package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"image"
	"image/jpeg"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"sync"
	"testing"

	"github.com/rechedev9/cliphub/internal/library"
	"github.com/rechedev9/cliphub/internal/replayvideo"
	"github.com/rechedev9/cliphub/internal/store"
	"github.com/rechedev9/cliphub/internal/teamvoice"
)

// fakeVideoEncoder stands in for FFmpeg: it counts frames and writes a stub MP4 on Finish.
type fakeVideoEncoder struct {
	mu    sync.Mutex
	plans []replayvideo.Plan
	sinks []*fakeVideoSink
}

func (f *fakeVideoEncoder) Start(_ context.Context, p replayvideo.Plan) (replayvideo.Sink, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	s := &fakeVideoSink{plan: p}
	f.plans = append(f.plans, p)
	f.sinks = append(f.sinks, s)
	return s, nil
}

func (f *fakeVideoEncoder) last() (replayvideo.Plan, *fakeVideoSink) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.plans[len(f.plans)-1], f.sinks[len(f.sinks)-1]
}

type fakeVideoSink struct {
	plan    replayvideo.Plan
	frames  int
	aborted bool
}

func (s *fakeVideoSink) Frame(src io.Reader) error {
	raw, err := io.ReadAll(src)
	if err != nil {
		return errors.Join(replayvideo.ErrFrameRead, err)
	}
	if len(raw) < 2 || raw[0] != 0xff || raw[1] != 0xd8 {
		return replayvideo.ErrNotJPEG
	}
	s.frames++
	// The real sink has FFmpeg write as frames arrive; the stub file stands for that output.
	return os.WriteFile(s.plan.Out, []byte("\x00\x00\x00\x18ftypisom partial"), 0o600)
}

func (s *fakeVideoSink) Finish() (bool, error) {
	return len(s.plan.Tracks) > 0, os.WriteFile(s.plan.Out, []byte("\x00\x00\x00\x18ftypisom stub mp4"), 0o600)
}

func (s *fakeVideoSink) Abort() {
	s.aborted = true
	_ = os.Remove(s.plan.Out)
}

type videoTestEnv struct {
	voiceTestEnv
	encoder *fakeVideoEncoder
	dir     string
	match   library.Match
}

func newVideoTestEnv(t *testing.T) videoTestEnv {
	t.Helper()
	repo := store.NewMemoryLibraryRepository()
	files := newFakeStorage()
	enc := &fakeVideoEncoder{}
	dir := t.TempDir()
	h := NewHandlers(newFakeRepo(), files, &fakeQueue{}, WithLibrary(repo), WithReplayVideo(ReplayVideoDeps{Encoder: enc, Dir: dir}),
		WithVoice(VoiceDeps{Dir: t.TempDir(), Media: fakeVoiceMedia{}, Transcriber: &fakeTranscriber{state: teamvoice.TranscriberMissing}}))
	env := voiceTestEnv{libraryTestEnv: libraryTestEnv{t: t, repo: repo, storage: files, h: h, router: Routes(h)}}
	return videoTestEnv{voiceTestEnv: env, encoder: enc, dir: dir, match: env.seedLibraryMatch("video-sha", "Alpha", "Bravo")}
}

func (e videoTestEnv) path() string {
	return "/api/library/matches/" + e.match.ID.String() + "/replay-video"
}

func videoSpec(change map[string]any) map[string]any {
	out := map[string]any{"fps": 30, "width": 64, "height": 64, "speed": 1,
		"segments": []any{map[string]any{"round": 1, "from_tick": 0, "to_tick": 64, "frames": 3}}}
	for k, v := range change {
		out[k] = v
	}
	return out
}

func testJPEG(t *testing.T) []byte {
	t.Helper()
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, image.NewRGBA(image.Rect(0, 0, 64, 64)), nil); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// videoBody is the spec and then the frames, as the Replay sends them.
func videoBody(t *testing.T, spec map[string]any, frames [][]byte) ([]byte, string) {
	t.Helper()
	body := &bytes.Buffer{}
	mw := multipart.NewWriter(body)
	raw, err := json.Marshal(spec)
	if err != nil {
		t.Fatal(err)
	}
	_ = mw.WriteField("spec", string(raw))
	for _, f := range frames {
		part, _ := mw.CreateFormFile("frame", "frame.jpg")
		_, _ = part.Write(f)
	}
	_ = mw.Close()
	return body.Bytes(), mw.FormDataContentType()
}

func (e videoTestEnv) post(spec map[string]any, frames [][]byte) (int, string, []byte) {
	e.t.Helper()
	body, ct := videoBody(e.t, spec, frames)
	rw := e.do(http.MethodPost, e.path(), body, ct)
	var got struct {
		Code string `json:"code"`
	}
	_ = json.Unmarshal(rw.Body.Bytes(), &got)
	return rw.Code, got.Code, rw.Body.Bytes()
}

func TestReplayVideoFullFlow(t *testing.T) {
	env := newVideoTestEnv(t)
	frame := testJPEG(t)
	status, _, body := env.post(videoSpec(nil), [][]byte{frame, frame, frame})
	if status != http.StatusOK {
		t.Fatalf("POST = %d %s", status, body)
	}
	var res replayVideoResult
	if err := json.Unmarshal(body, &res); err != nil || res.Frames != 3 || res.Voice || res.Seconds != 0.1 {
		t.Fatalf("result = %+v err=%v", res, err)
	}
	get := "/api/library/replay-videos/" + res.VideoID
	if rw := env.do(http.MethodHead, get, nil, ""); rw.Code != http.StatusOK {
		t.Fatalf("HEAD = %d", rw.Code)
	}
	ranged := httptest.NewRequest(http.MethodGet, get, nil)
	ranged.Header.Set("Range", "bytes=0-3")
	if rw := httptest.NewRecorder(); func() int { env.router.ServeHTTP(rw, ranged); return rw.Code }() != http.StatusPartialContent {
		t.Fatalf("range GET = %d", rw.Code)
	}
	rw := env.do(http.MethodGet, get, nil, "")
	if rw.Code != http.StatusOK || !bytes.Contains(rw.Body.Bytes(), []byte("ftypisom")) || rw.Header().Get("Content-Type") != "video/mp4" {
		t.Fatalf("GET = %d %q", rw.Code, rw.Body.String())
	}
	if rw := env.do(http.MethodGet, get, nil, ""); rw.Code != http.StatusNotFound {
		t.Fatalf("second GET = %d, want the file gone", rw.Code)
	}
	if entries, _ := os.ReadDir(env.dir); len(entries) != 0 {
		t.Fatalf("left %d files", len(entries))
	}
}

func TestReplayVideoBadFramesAbort(t *testing.T) {
	env := newVideoTestEnv(t)
	frame := testJPEG(t)
	for name, frames := range map[string][][]byte{
		"too few":           {frame, frame},
		"too many":          {frame, frame, frame, frame},
		"not a JPEG midway": {frame, []byte("GIF89a not a frame"), frame},
	} {
		status, code, body := env.post(videoSpec(nil), frames)
		if status != http.StatusBadRequest || code != codeReplayVideoInvalid {
			t.Fatalf("%s: POST = %d %s", name, status, body)
		}
		plan, sink := env.encoder.last()
		if !sink.aborted {
			t.Fatalf("%s: the encode was not aborted", name)
		}
		if _, err := os.Stat(plan.Out); !os.IsNotExist(err) {
			t.Fatalf("%s: the output stayed: %v", name, err)
		}
	}
}

type brokenBody struct{ r io.Reader }

func (b brokenBody) Read(p []byte) (int, error) {
	n, err := b.r.Read(p)
	if errors.Is(err, io.EOF) {
		return n, errors.New("client went away")
	}
	return n, err
}

func TestReplayVideoClientGoneCleansUp(t *testing.T) {
	env := newVideoTestEnv(t)
	frame := testJPEG(t)
	body, ct := videoBody(t, videoSpec(nil), [][]byte{frame, frame, frame})
	// The upload breaks after the spec and the first frame.
	cut := bytes.Index(body, frame) + len(frame) + 10
	req := httptest.NewRequest(http.MethodPost, env.path(), brokenBody{r: bytes.NewReader(body[:cut])})
	req.Header.Set("Content-Type", ct)
	rw := httptest.NewRecorder()
	env.router.ServeHTTP(rw, req)
	plan, sink := env.encoder.last()
	if rw.Code != http.StatusBadRequest || !sink.aborted {
		t.Fatalf("broken upload = %d, aborted=%v frames=%d", rw.Code, sink.aborted, sink.frames)
	}
	if _, err := os.Stat(plan.Out); !os.IsNotExist(err) {
		t.Fatalf("the output stayed: %v", err)
	}
}

func TestReplayVideoRefusalsAndVoice(t *testing.T) {
	env := newVideoTestEnv(t)
	expect := func(spec map[string]any, wantStatus int, wantCode string) {
		t.Helper()
		if status, code, body := env.post(spec, nil); status != wantStatus || code != wantCode {
			t.Fatalf("POST %v = %d %s, want %d %s", spec, status, body, wantStatus, wantCode)
		}
	}
	expect(videoSpec(map[string]any{"width": 65}), http.StatusBadRequest, codeReplayVideoInvalid)
	expect(videoSpec(map[string]any{"width": 4000}), http.StatusBadRequest, codeReplayVideoInvalid)
	expect(videoSpec(map[string]any{"speed": 3}), http.StatusBadRequest, codeReplayVideoInvalid)
	expect(videoSpec(map[string]any{"segments": []any{map[string]any{"round": 99, "from_tick": 0, "to_tick": 64, "frames": 3}}}), http.StatusBadRequest, codeReplayVideoInvalid)
	expect(videoSpec(map[string]any{"include_voice": true}), http.StatusConflict, codeNoVoice)

	if res := env.upload("/api/library/matches/"+env.match.ID.String()+"/voice", map[string]string{"ts3.wav": "mono recording"}); res.StatusCode != http.StatusOK {
		t.Fatalf("voice import = %d", res.StatusCode)
	}
	if _, err := env.repo.SetVoiceSync(context.Background(), env.match.ID, library.VoiceSync{State: library.VoiceUnsynced}); err != nil {
		t.Fatal(err)
	}
	expect(videoSpec(map[string]any{"include_voice": true}), http.StatusConflict, codeVoiceUnsynced)
	// The offset the user hears wins over the saved state; a 0.25x export carries it too.
	expect(videoSpec(map[string]any{"include_voice": true, "offset_ms": -5000, "speed": 0.25}), http.StatusBadRequest, codeReplayVideoInvalid)
	if plan, _ := env.encoder.last(); plan.Sync.OffsetMS != -5000 || plan.Speed != 0.25 || len(plan.Tracks) != 1 {
		t.Fatalf("plan = %+v, want the sent offset", plan)
	}
	if _, err := env.repo.SetVoiceSync(context.Background(), env.match.ID, library.VoiceSync{State: library.VoiceManual, OffsetMS: 5000}); err != nil {
		t.Fatal(err)
	}
	// Synced 5 s after the demo starts, the recording misses the demo's first second.
	expect(videoSpec(map[string]any{"include_voice": true}), http.StatusConflict, codeVoiceOutside)
	expect(videoSpec(map[string]any{"include_voice": true, "voice_tracks": []int{9}}), http.StatusBadRequest, codeReplayVideoInvalid)
	// Only freeze time: the voice stops at voice_to_tick while the video goes on.
	seg := map[string]any{"round": 1, "from_tick": 0, "to_tick": 640, "frames": 3, "voice_to_tick": 384}
	expect(videoSpec(map[string]any{"include_voice": true, "offset_ms": 0, "segments": []any{seg}}), http.StatusBadRequest, codeReplayVideoInvalid)
	if plan, _ := env.encoder.last(); plan.Segments[0].Voice.ToTick != 384 {
		t.Fatalf("voice range = %+v, want it to stop at 384", plan.Segments[0].Voice)
	}
}

func TestReplayVideoUnavailableAndUnknown(t *testing.T) {
	env := newLibraryTestEnv(t)
	body, ct := videoBody(t, videoSpec(nil), nil)
	if rw := env.do(http.MethodPost, "/api/library/matches/7f3c2b8e-0d7a-4a43-9f39-3c0d7c6a2b11/replay-video", body, ct); rw.Code != http.StatusServiceUnavailable {
		t.Fatalf("without FFmpeg = %d", rw.Code)
	}
	video := newVideoTestEnv(t)
	if rw := video.do(http.MethodGet, "/api/library/replay-videos/7f3c2b8e-0d7a-4a43-9f39-3c0d7c6a2b11", nil, ""); rw.Code != http.StatusNotFound {
		t.Fatalf("unknown video = %d", rw.Code)
	}
}
