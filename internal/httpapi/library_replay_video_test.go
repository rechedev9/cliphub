package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"testing"

	"github.com/rechedev9/cliphub/internal/library"
	"github.com/rechedev9/cliphub/internal/store"
	"github.com/rechedev9/cliphub/internal/teamvoice"
)

func replayVideoBody(t *testing.T, spec map[string]any) ([]byte, string) {
	t.Helper()
	body := &bytes.Buffer{}
	mw := multipart.NewWriter(body)
	raw, err := json.Marshal(spec)
	if err != nil {
		t.Fatal(err)
	}
	_ = mw.WriteField("spec", string(raw))
	_ = mw.Close()
	return body.Bytes(), mw.FormDataContentType()
}

// The refusals come before FFmpeg starts, so the configured path is never run.
func TestCreateLibraryReplayVideoRefusals(t *testing.T) {
	repo := store.NewMemoryLibraryRepository()
	files := newFakeStorage()
	h := NewHandlers(newFakeRepo(), files, &fakeQueue{}, WithLibrary(repo), WithStreamFFmpeg("ffmpeg-not-run"),
		WithVoice(VoiceDeps{Dir: t.TempDir(), Media: fakeVoiceMedia{}, Transcriber: &fakeTranscriber{state: teamvoice.TranscriberMissing}}))
	env := voiceTestEnv{libraryTestEnv: libraryTestEnv{t: t, repo: repo, storage: files, h: h, router: Routes(h)}}
	m := env.seedLibraryMatch("video-sha", "Alpha", "Bravo")
	path := "/api/library/matches/" + m.ID.String() + "/replay-video"
	segment := map[string]any{"round": 1, "from_tick": 0, "to_tick": 64, "frames": 30}
	spec := func(change map[string]any) map[string]any {
		out := map[string]any{"fps": 30, "width": 720, "height": 720, "speed": 1, "segments": []any{segment}}
		for k, v := range change {
			out[k] = v
		}
		return out
	}
	post := func(s map[string]any, wantStatus int, wantCode string) {
		t.Helper()
		body, ct := replayVideoBody(t, s)
		rw := env.do(http.MethodPost, path, body, ct)
		var got struct {
			Code string `json:"code"`
		}
		_ = json.Unmarshal(rw.Body.Bytes(), &got)
		if rw.Code != wantStatus || got.Code != wantCode {
			t.Fatalf("POST %v = %d %s, want %d %s", s, rw.Code, rw.Body.String(), wantStatus, wantCode)
		}
	}

	post(spec(map[string]any{"width": 721}), http.StatusBadRequest, "bad_request")
	post(spec(map[string]any{"segments": []any{map[string]any{"round": 99, "from_tick": 0, "to_tick": 64, "frames": 30}}}), http.StatusBadRequest, "bad_request")
	post(spec(map[string]any{"include_voice": true}), http.StatusConflict, codeNoVoice)

	if res := env.upload("/api/library/matches/"+m.ID.String()+"/voice", map[string]string{"ts3.wav": "mono recording"}); res.StatusCode != http.StatusOK {
		t.Fatalf("voice import = %d", res.StatusCode)
	}
	if _, err := repo.SetVoiceSync(context.Background(), m.ID, library.VoiceSync{State: library.VoiceUnsynced}); err != nil {
		t.Fatal(err)
	}
	post(spec(map[string]any{"include_voice": true}), http.StatusConflict, codeVoiceUnsynced)
	// Synced 5 s after the demo starts, the recording misses the demo's first second.
	if _, err := repo.SetVoiceSync(context.Background(), m.ID, library.VoiceSync{State: library.VoiceManual, OffsetMS: 5000}); err != nil {
		t.Fatal(err)
	}
	post(spec(map[string]any{"include_voice": true}), http.StatusConflict, codeVoiceOutside)
	post(spec(map[string]any{"include_voice": true, "voice_tracks": []int{9}}), http.StatusBadRequest, "bad_request")
}

func TestGetLibraryReplayVideoUnknown(t *testing.T) {
	env := newLibraryTestEnv(t)
	if rw := env.do(http.MethodGet, "/api/library/replay-videos/7f3c2b8e-0d7a-4a43-9f39-3c0d7c6a2b11", nil, ""); rw.Code != http.StatusNotFound {
		t.Fatalf("unknown video = %d", rw.Code)
	}
}
