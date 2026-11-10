package gamestate

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

func post(t *testing.T, body string) Payload {
	t.Helper()
	p, err := Parse([]byte(body))
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func mapPost(phase, roundPhase string, round int) string {
	return `{"provider":{"steamid":"76561198000000001"},"map":{"name":"de_mirage","mode":"competitive","phase":"` + phase +
		`","round":` + strconv.Itoa(round) + `,"team_ct":{"score":` + strconv.Itoa(round) + `},"team_t":{"score":0}},"round":{"phase":"` + roundPhase + `"},"auth":{"token":"t"}}`
}

func TestTrackerMarksRoundsFromFreezeTimeAndEndsTheMatch(t *testing.T) {
	var tr Tracker
	at := time.Date(2026, 11, 9, 20, 0, 0, 0, time.UTC)
	tr.Observe(post(t, mapPost("warmup", "live", 0)), at)
	warm := tr.State(at)
	if !warm.Live || warm.Map != "de_mirage" || warm.SteamID != "76561198000000001" || len(warm.Rounds) != 0 {
		t.Fatalf("warmup = %+v, want live without rounds", warm)
	}
	tr.Observe(post(t, mapPost("live", "freezetime", 0)), at.Add(time.Minute))
	tr.Observe(post(t, mapPost("live", "live", 0)), at.Add(time.Minute+20*time.Second))
	tr.Observe(post(t, mapPost("live", "live", 0)), at.Add(time.Minute+30*time.Second))
	tr.Observe(post(t, mapPost("live", "over", 0)), at.Add(3*time.Minute))
	tr.Observe(post(t, mapPost("live", "freezetime", 1)), at.Add(3*time.Minute+5*time.Second))
	tr.Observe(post(t, mapPost("live", "live", 1)), at.Add(3*time.Minute+25*time.Second))
	got := tr.State(at.Add(4 * time.Minute))
	if got.Session != warm.Session || len(got.Rounds) != 2 || got.Rounds[0].Round != 1 || !got.Rounds[0].LiveAt.Equal(at.Add(time.Minute+20*time.Second)) ||
		got.Rounds[1].Round != 2 || got.ScoreCT != 1 {
		t.Fatalf("state = %+v", got)
	}
	text := got.Text()
	for _, want := range []string{"live 1\n", "session " + got.Session + "\n", "map de_mirage\n", "score 1 0\n",
		"round 2 " + strconv.FormatInt(at.Add(3*time.Minute+25*time.Second).UnixMilli(), 10) + "\n"} {
		if !strings.Contains(text, want) {
			t.Fatalf("text %q lacks %q", text, want)
		}
	}
	// A restarted game counts round 1 again and drops the later rounds.
	tr.Observe(post(t, mapPost("live", "freezetime", 0)), at.Add(5*time.Minute))
	tr.Observe(post(t, mapPost("live", "live", 0)), at.Add(5*time.Minute+15*time.Second))
	if again := tr.State(at.Add(5 * time.Minute)); len(again.Rounds) != 1 || !again.Rounds[0].LiveAt.Equal(at.Add(5*time.Minute+15*time.Second)) {
		t.Fatalf("after a restart = %+v", again.Rounds)
	}
	tr.Observe(post(t, mapPost("gameover", "over", 3)), at.Add(40*time.Minute))
	if over := tr.State(at.Add(40 * time.Minute)); over.Live || over.Text() != "live 0\n" || over.LastSeen.IsZero() {
		t.Fatalf("game over = %+v", over)
	}
}

func TestTrackerIgnoresMenusAndCasualAndGoesStale(t *testing.T) {
	var tr Tracker
	at := time.Date(2026, 11, 9, 20, 0, 0, 0, time.UTC)
	tr.Observe(post(t, `{"provider":{"steamid":"1"}}`), at)
	if tr.State(at).Live {
		t.Fatal("the main menu is not a match")
	}
	tr.Observe(post(t, `{"provider":{"steamid":"1"},"map":{"name":"de_dust2","mode":"deathmatch","phase":"live"}}`), at)
	if tr.State(at).Live {
		t.Fatal("deathmatch is not a match")
	}
	tr.Observe(post(t, mapPost("live", "live", 4)), at)
	if s := tr.State(at.Add(StaleAfter)); !s.Live || len(s.Rounds) != 0 {
		t.Fatalf("a round first seen live = %+v, want live without a mark", s)
	}
	if tr.State(at.Add(StaleAfter + time.Second)).Live {
		t.Fatal("a match CS2 stopped reporting stays live")
	}
}

func TestInstallWritesTheConfigIntoCS2(t *testing.T) {
	root := t.TempDir()
	exe := filepath.Join(root, "game", "bin", "win64", "cs2.exe")
	if _, err := Install(exe, ConfigOptions{URL: "http://127.0.0.1:1/x", Token: "abc"}); err != ErrNoCS2 {
		t.Fatalf("Install without CS2 = %v, want ErrNoCS2", err)
	}
	if err := os.MkdirAll(filepath.Join(root, "game", "csgo"), 0o755); err != nil {
		t.Fatal(err)
	}
	opts := ConfigOptions{URL: "http://127.0.0.1:4000/api/voice-plugin/gamestate", Token: `a"b`}
	path, err := Install(exe, opts)
	if err != nil || path != filepath.Join(root, "game", "csgo", "cfg", ConfigName) {
		t.Fatalf("Install = %q, %v", path, err)
	}
	body, _ := os.ReadFile(path)
	if !strings.Contains(string(body), `"uri" "http://127.0.0.1:4000/api/voice-plugin/gamestate"`) || !strings.Contains(string(body), `"token" "ab"`) {
		t.Fatalf("cfg = %s", body)
	}
	if st, err := Check(exe, opts); err != nil || !st.Present || !st.Current {
		t.Fatalf("Check = %+v, %v", st, err)
	}
	if st, _ := Check(exe, ConfigOptions{URL: "http://127.0.0.1:4001/api/voice-plugin/gamestate", Token: "ab"}); !st.Present || st.Current {
		t.Fatalf("Check on a new port = %+v, want present and stale", st)
	}
}
