package httpapi

import (
	"net/http"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"

	"github.com/rechedev9/cliphub/internal/store"
)

// desktopOnlyRoutes are the library, map and team routes the web edition does
// not serve, each with the reason: they need the demo file, this PC or its tools.
var desktopOnlyRoutes = map[Route]string{
	post("/api/library/demos"):                                           "imports a .dem from this PC",
	post("/api/library/demos/{id}/analyze"):                              "re-reads the .dem",
	post("/api/library/demos/{id}/jobs"):                                 "starts a capture job",
	get("/api/library/jobs"):                                             "capture jobs",
	post("/api/library/jobs"):                                            "capture jobs",
	get("/api/library/jobs/{jobId}"):                                     "capture jobs",
	post("/api/library/jobs/{jobId}"):                                    "capture jobs",
	post("/api/library/matches/{id}/tactical/rounds/{round}/bot-replay"): "launches CS2",
	post("/api/library/matches/{id}/replay-video"):                       "renders a video with FFmpeg",
	get("/api/library/replay-videos/{video}"):                            "a rendered video",
	post("/api/library/matches/{id}/voice"):                              "imports audio from this PC",
	get("/api/library/matches/{id}/voice/tracks/{track}/audio"):          "the audio stays on the PC that imported it",
	post("/api/library/matches/{id}/voice/transcribe"):                   "runs Whisper on this PC",
	get("/api/library/matches/{id}/voice/demo"):                          "reads the .dem",
	post("/api/library/matches/{id}/voice/demo"):                         "reads the .dem",
	get("/api/library/matches/{id}/voice/demo/{steamid}/audio"):          "reads the .dem",
	get("/api/library/voice/transcriber"):                                "Whisper on this PC",
	post("/api/library/voice/transcriber/install"):                       "Whisper on this PC",
	get("/api/library/voice/recordings"):                                 "TeamSpeak recordings on this PC",
	post("/api/library/voice/recordings/{id}/attach"):                    "TeamSpeak recordings on this PC",
	del("/api/library/voice/recordings/{id}"):                            "TeamSpeak recordings on this PC",
	get("/api/library/voice/plugin"):                                     "the TeamSpeak plugin on this PC",
	post("/api/library/voice/plugin/setup"):                              "the TeamSpeak plugin on this PC",
}

func TestEveryLibraryRouteHasAnEdition(t *testing.T) {
	h := NewHandlers(newFakeRepo(), newFakeStorage(), &fakeQueue{},
		WithLibrary(store.NewMemoryLibraryRepository()), WithLibraryUploads(), WithSyncHub(store.NewSyncLog(nil)))
	analysis := map[Route]bool{}
	for _, r := range AnalysisRoutes() {
		if analysis[r] {
			t.Errorf("%s %s is listed twice", r.Method, r.Pattern)
		}
		if _, ok := desktopOnlyRoutes[r]; ok {
			t.Errorf("%s %s is both an analysis and a desktop route", r.Method, r.Pattern)
		}
		analysis[r] = true
	}
	served := map[Route]bool{}
	err := chi.Walk(Routes(h), func(method, pattern string, _ http.Handler, _ ...func(http.Handler) http.Handler) error {
		r := Route{method, pattern}
		served[r] = true
		scoped := strings.HasPrefix(pattern, "/api/library/") || strings.HasPrefix(pattern, "/api/maps/") ||
			strings.HasPrefix(pattern, "/api/equipment/") || strings.HasPrefix(pattern, "/api/teams/")
		_, desktop := desktopOnlyRoutes[r]
		if scoped && method != http.MethodHead && !analysis[r] && !desktop {
			t.Errorf("%s %s: add it to AnalysisRoutes, or to desktopOnlyRoutes with the reason", method, pattern)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	for r := range analysis {
		if !served[r] {
			t.Errorf("analysis route %s %s is not registered", r.Method, r.Pattern)
		}
	}
	for r := range desktopOnlyRoutes {
		if !served[r] {
			t.Errorf("desktop route %s %s is not registered", r.Method, r.Pattern)
		}
	}
}
