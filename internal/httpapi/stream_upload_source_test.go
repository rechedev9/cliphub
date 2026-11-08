package httpapi

import (
	"bytes"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/rechedev9/cliphub/internal/streamclips"
)

// mpegtsRecording makes a one-second MPEG-TS file, like an OBS recording renamed to .mp4.
func mpegtsRecording(t *testing.T, ffmpeg string) []byte {
	t.Helper()
	path := filepath.Join(t.TempDir(), "recording.ts")
	cmd := exec.Command(ffmpeg, "-hide_banner", "-nostdin", "-loglevel", "error", "-y",
		"-f", "lavfi", "-i", "testsrc=s=320x180:r=30:d=1",
		"-f", "lavfi", "-i", "sine=d=1",
		"-c:v", "mpeg2video", "-c:a", "mp2", "-f", "mpegts", path)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("make MPEG-TS: %v: %s", err, out)
	}
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func postStreamUpload(t *testing.T, router http.Handler, contents []byte) *httptest.ResponseRecorder {
	t.Helper()
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, err := writer.CreateFormFile("video", "stream.mp4")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := part.Write(contents); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodPost, "/api/stream-jobs", body)
	request.Header.Set("Content-Type", writer.FormDataContentType())
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	return response
}

func TestStreamUploadRemuxesMPEGTSToMP4(t *testing.T) {
	ffmpeg, err := exec.LookPath("ffmpeg")
	if err != nil {
		t.Skip("ffmpeg is not installed")
	}
	ffprobe, err := exec.LookPath("ffprobe")
	if err != nil {
		t.Skip("ffprobe is not installed")
	}
	recording := mpegtsRecording(t, ffmpeg)
	prober := streamclips.FFprobeProber{Path: ffprobe}

	t.Run("with ffmpeg", func(t *testing.T) {
		repo := newFakeStreamRepo()
		files := newFakeStorage()
		router := Routes(NewHandlers(newFakeRepo(), files, &fakeQueue{}, WithStreamRepository(repo), WithStreamProber(prober), WithStreamFFmpeg(ffmpeg)))
		response := postStreamUpload(t, router, recording)
		if response.Code != http.StatusCreated {
			t.Fatalf("status = %d, want %d; body=%s", response.Code, http.StatusCreated, response.Body.String())
		}
		id := uuid.MustParse(jsonStringField(t, response.Body.Bytes(), "id"))
		job, err := repo.Get(t.Context(), id)
		if err != nil {
			t.Fatal(err)
		}
		if job.Probe.NeedsRemux() || !strings.Contains(job.Probe.Container, "mp4") {
			t.Fatalf("recorded container = %q, want MP4", job.Probe.Container)
		}
		// The stored bytes, not only the recorded probe, must be the MP4.
		stored := filepath.Join(t.TempDir(), "stored.mp4")
		if err := os.WriteFile(stored, files.puts[streamclips.SourceKey(id)], 0o600); err != nil {
			t.Fatal(err)
		}
		storedProbe, err := prober.Probe(t.Context(), stored)
		if err != nil || storedProbe.NeedsRemux() || !strings.Contains(storedProbe.Container, "mp4") {
			t.Fatalf("stored source container = %q, %v; want MP4", storedProbe.Container, err)
		}
	})

	t.Run("without ffmpeg", func(t *testing.T) {
		router := Routes(NewHandlers(newFakeRepo(), newFakeStorage(), &fakeQueue{}, WithStreamRepository(newFakeStreamRepo()), WithStreamProber(prober)))
		response := postStreamUpload(t, router, recording)
		if response.Code != http.StatusBadRequest || !strings.Contains(response.Body.String(), streamclips.AcquireCodeNotPlayable) {
			t.Fatalf("status = %d body=%s, want 400 not_playable", response.Code, response.Body.String())
		}
	})
}
