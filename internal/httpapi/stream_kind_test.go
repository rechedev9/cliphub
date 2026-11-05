package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/rechedev9/cliphub/internal/streamclips"
)

func TestCreateStreamJobKeepsTheChosenKindAndRejectsAMismatchedLink(t *testing.T) {
	tests := []struct {
		name     string
		kind     string
		url      string
		wantCode int
		wantKind streamclips.SourceKind
	}{
		{name: "vod link as vod", kind: "vod", url: "https://www.twitch.tv/videos/123456789", wantCode: http.StatusAccepted, wantKind: streamclips.SourceKindVOD},
		{name: "clip link as clip", kind: "clip", url: "https://clips.twitch.tv/SomeSlug", wantCode: http.StatusAccepted, wantKind: streamclips.SourceKindClip},
		{name: "no kind on a clip link", kind: "", url: "https://clips.twitch.tv/SomeSlug", wantCode: http.StatusAccepted, wantKind: streamclips.SourceKindClip},
		{name: "no kind on a vod link", kind: "", url: "https://www.twitch.tv/videos/123456789", wantCode: http.StatusAccepted, wantKind: streamclips.SourceKindVOD},
		{name: "youtube cannot be told apart", kind: "vod", url: "https://www.youtube.com/watch?v=abc123", wantCode: http.StatusAccepted, wantKind: streamclips.SourceKindVOD},
		{name: "vod link as clip", kind: "clip", url: "https://www.twitch.tv/videos/123456789", wantCode: http.StatusBadRequest},
		{name: "clip link as vod", kind: "vod", url: "https://kick.com/aimagia/clips/clip_01K8TRRRRPK5NL1N1FFFZ7C7", wantCode: http.StatusBadRequest},
		{name: "unknown kind", kind: "reel", url: "https://clips.twitch.tv/SomeSlug", wantCode: http.StatusBadRequest},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := newFakeStreamRepo()
			router := Routes(NewHandlers(newFakeRepo(), newFakeStorage(), &fakeQueue{},
				WithStreamRepository(repo), WithCapabilities(Capabilities{YtdlpEnabled: true})))
			body, err := json.Marshal(map[string]string{"source_url": tt.url, "source_kind": tt.kind})
			if err != nil {
				t.Fatal(err)
			}
			req := httptest.NewRequest(http.MethodPost, "/api/stream-jobs", strings.NewReader(string(body)))
			req.Header.Set("Content-Type", "application/json")
			rw := httptest.NewRecorder()
			router.ServeHTTP(rw, req)
			if rw.Code != tt.wantCode {
				t.Fatalf("status = %d, want %d; body=%s", rw.Code, tt.wantCode, rw.Body.String())
			}
			if tt.wantCode != http.StatusAccepted {
				return
			}
			var created struct {
				ID uuid.UUID `json:"id"`
			}
			if err := json.Unmarshal(rw.Body.Bytes(), &created); err != nil {
				t.Fatal(err)
			}
			job, err := repo.Get(context.Background(), created.ID)
			if err != nil {
				t.Fatal(err)
			}
			if job.SourceKind != tt.wantKind {
				t.Fatalf("source kind = %q, want %q", job.SourceKind, tt.wantKind)
			}
		})
	}
}
