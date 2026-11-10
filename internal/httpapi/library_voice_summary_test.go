package httpapi

import (
	"net/http"
	"strings"
	"testing"

	"github.com/rechedev9/cliphub/internal/library"
)

func TestLibraryVoiceSummariesListMatchesWithVoiceWithoutTracks(t *testing.T) {
	env := newVoiceTestEnv(t)
	with := env.seedLibraryMatch("summary-with", "Alpha", "Bravo")
	env.seedLibraryMatch("summary-without", "Charlie", "Delta")
	if res := env.upload("/api/library/matches/"+with.ID.String()+"/voice", map[string]string{"ts3.wav": "mono recording"}); res.StatusCode != http.StatusOK {
		t.Fatalf("import = %d", res.StatusCode)
	}

	var out struct {
		Voices []library.VoiceSummary `json:"voices"`
	}
	env.json(http.MethodGet, "/api/library/voice/summary", nil, http.StatusOK, &out)
	if len(out.Voices) != 1 || out.Voices[0].MatchID != with.ID || out.Voices[0].Tracks != 1 || out.Voices[0].Sync.State == "" {
		t.Fatalf("summaries = %+v, want only the match with voice", out.Voices)
	}
	raw := env.do(http.MethodGet, "/api/library/voice/summary", nil, "")
	if strings.Contains(raw.Body.String(), "file_name") {
		t.Fatalf("summary leaks track rows: %s", raw.Body.String())
	}
}
