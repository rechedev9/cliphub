package httpapi

import (
	"bytes"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rechedev9/cliphub/internal/artifacts"
	"github.com/rechedev9/cliphub/internal/practice"
	"github.com/rechedev9/cliphub/internal/store"
)

func newBotReplayEnv(t *testing.T, game *fakePracticeGame) libraryTestEnv {
	t.Helper()
	repo := store.NewMemoryLibraryRepository()
	files := newFakeStorage()
	queue := &fakeQueue{}
	h := NewHandlers(newFakeRepo(), files, queue, WithLibrary(repo), WithPractice(game))
	return libraryTestEnv{t: t, repo: repo, storage: files, queue: queue, h: h, router: Routes(h)}
}

func TestLaunchLibraryBotReplayWritesCfgAndLaunches(t *testing.T) {
	game := &fakePracticeGame{dir: t.TempDir()}
	env := newBotReplayEnv(t, game)
	m := env.seedLibraryMatch("botreplay", "Vitality", "MOUZ")
	base := "/api/library/matches/" + m.ID.String() + "/tactical/rounds/"

	var resp botReplayResponse
	env.json(http.MethodPost, base+"1/bot-replay", map[string]string{"side": "T"}, http.StatusOK, &resp)
	if !resp.Launched || resp.BotSide != "CT" || len(resp.Bots) != 1 || resp.Bots[0].Name != "one" || resp.Bots[0].Index != 2 {
		t.Fatalf("response = %+v", resp)
	}
	if len(resp.Moments) != 1 || resp.Keys.Restart != "KP_ENTER" || resp.Keys.Base != "KP_DEL" || len(resp.Limits) == 0 || resp.Starts == nil {
		t.Fatalf("moments %+v keys %+v limits %v", resp.Moments, resp.Keys, resp.Limits)
	}
	if len(game.launched) != 1 || game.launched[0] != "de_mirage" {
		t.Fatalf("launched = %v", game.launched)
	}
	cfg, err := os.ReadFile(filepath.Join(game.dir, "cliphub_practice.cfg"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(cfg), "bot_add_ct") || !strings.Contains(string(cfg), "setpos_player 2 10.00 20.00 30.00\"") {
		t.Fatalf("cfg:\n%s", cfg)
	}

	for body, want := range map[string]int{
		`{"side":"CT"}`:                      http.StatusBadRequest, // round 1 has no T player
		`{"side":"X"}`:                       http.StatusBadRequest,
		`{"side":"T","replace_steamid":"1"}`: http.StatusBadRequest,
		`{"side":"T","extra":1}`:             http.StatusBadRequest,
	} {
		if rw := env.do(http.MethodPost, base+"1/bot-replay", []byte(body), "application/json"); rw.Code != want {
			t.Errorf("%s = %d, want %d: %s", body, rw.Code, want, rw.Body.String())
		}
	}
	if rw := env.do(http.MethodPost, base+"9/bot-replay", []byte(`{"side":"T"}`), "application/json"); rw.Code != http.StatusNotFound {
		t.Fatalf("missing round = %d", rw.Code)
	}
}

func TestLaunchLibraryBotReplayPositionsErrors(t *testing.T) {
	env := newBotReplayEnv(t, &fakePracticeGame{dir: t.TempDir()})
	m := env.seedLibraryMatch("botreplay-blob", "Vitality", "MOUZ")
	path := "/api/library/matches/" + m.ID.String() + "/tactical/rounds/1/bot-replay"
	key := artifacts.LibraryTacticalPositionsKey(m.DemoID)

	_ = env.storage.Put(key, bytes.NewReader([]byte("not a blob")))
	if rw := env.do(http.MethodPost, path, []byte(`{"side":"T"}`), "application/json"); rw.Code != http.StatusInternalServerError {
		t.Fatalf("broken blob = %d, want 500: %s", rw.Code, rw.Body.String())
	}
	_ = env.storage.Delete(key)
	if rw := env.do(http.MethodPost, path, []byte(`{"side":"T"}`), "application/json"); rw.Code != http.StatusConflict || !strings.Contains(rw.Body.String(), codeNoPositions) {
		t.Fatalf("missing blob = %d: %s", rw.Code, rw.Body.String())
	}
}

func TestLaunchLibraryBotReplayWithoutCS2(t *testing.T) {
	env := newBotReplayEnv(t, &fakePracticeGame{dirErr: practice.ErrCS2NotFound})
	m := env.seedLibraryMatch("botreplay-nocs2", "Vitality", "MOUZ")
	rw := env.do(http.MethodPost, "/api/library/matches/"+m.ID.String()+"/tactical/rounds/1/bot-replay", []byte(`{"side":"T"}`), "application/json")
	if rw.Code != http.StatusServiceUnavailable || !strings.Contains(rw.Body.String(), codeCS2NotFound) {
		t.Fatalf("status = %d body = %s", rw.Code, rw.Body.String())
	}
}
