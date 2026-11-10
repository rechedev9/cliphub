package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/rechedev9/cliphub/internal/library"
	"github.com/rechedev9/cliphub/internal/store"
	"github.com/rechedev9/cliphub/internal/teamvoice"
	"github.com/rechedev9/cliphub/internal/voiceplugin"
)

// fakePluginMedia stores the rebuilt PCM as the "encoded" track; a track named bad fails to encode.
type fakePluginMedia struct{}

func (fakePluginMedia) EncodePCM(_ context.Context, job teamvoice.PCMEncodeJob) error {
	if strings.Contains(filepath.Base(job.Dst), "-bad.") {
		return errors.New("ffmpeg: invalid data")
	}
	b, err := io.ReadAll(job.PCM)
	if err != nil {
		return err
	}
	return os.WriteFile(job.Dst, b, 0o600)
}

func newPluginTestEnv(t *testing.T) (voiceTestEnv, voiceplugin.Dir) {
	t.Helper()
	repo := store.NewMemoryLibraryRepository()
	files := newFakeStorage()
	queue := &fakeQueue{}
	dir, plugin := t.TempDir(), voiceplugin.Dir(t.TempDir())
	tr := &fakeTranscriber{state: teamvoice.TranscriberMissing}
	h := NewHandlers(newFakeRepo(), files, queue, WithLibrary(repo), WithVoice(VoiceDeps{Dir: dir, Media: fakeVoiceMedia{}, Transcriber: tr,
		Plugin: VoicePluginDeps{Dir: plugin, WebOrigin: "http://127.0.0.1:4100", Media: fakePluginMedia{}}}))
	env := libraryTestEnv{t: t, repo: repo, storage: files, queue: queue, h: h, router: Routes(h)}
	return voiceTestEnv{libraryTestEnv: env, dir: dir, transcriber: tr}, plugin
}

type pluginRecordingSpec struct {
	id      string
	start   time.Time
	m       library.Match
	rounds  []library.RecordingRound
	tracks  []string
	selfID  string
	partial bool
}

// writePluginRecording leaves a finished recording in the inbox as the plugin would.
func writePluginRecording(t *testing.T, inbox string, s pluginRecordingSpec) string {
	t.Helper()
	dir := filepath.Join(inbox, s.id)
	if err := os.MkdirAll(dir, 0o750); err != nil {
		t.Fatal(err)
	}
	var rounds, tracks []string
	for _, r := range s.rounds {
		rounds = append(rounds, fmt.Sprintf(`{"round":%d,"live_at_ms":%d}`, r.Round, r.LiveAt.UnixMilli()))
	}
	for i, nick := range s.tracks {
		file := fmt.Sprintf("%02d-%s", i+1, nick)
		tracks = append(tracks, fmt.Sprintf(`{"file":%q,"nickname":%q,"uid":"uid-%s","self":%t}`, file, nick, nick, i == 0))
		if err := os.WriteFile(filepath.Join(dir, file+".pcm"), make([]byte, 9600), 0o600); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, file+".idx"), []byte("48000 4800\n"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	a, b := s.m.Teams[0].Score, s.m.Teams[1].Score
	manifest := fmt.Sprintf(`{"format":1,"state":"complete","started_at_ms":%d,"ended_at_ms":%d,"sample_rate":48000,"trigger":"auto",
		"server":"Scrims","channel":"Equipo","map":%q,"self_steamid64":%q,"score_ct":%d,"score_t":%d,"interrupted":%t,"rounds":[%s],"tracks":[%s]}`,
		s.start.UnixMilli(), s.start.Add(40*time.Minute).UnixMilli(), s.m.Map, s.selfID, b, a, s.partial, strings.Join(rounds, ","), strings.Join(tracks, ","))
	if err := os.WriteFile(filepath.Join(dir, "manifest.json"), []byte(manifest), 0o600); err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestPluginRecordingIsStoredAttachedTiedAndPlacedByTheGameClock(t *testing.T) {
	env, plugin := newPluginTestEnv(t)
	ctx := context.Background()
	m := env.seedLibraryMatch("plugin-sha", "Alpha", "Bravo")
	m, _ = env.repo.GetMatch(ctx, m.ID)
	doc, err := env.h.readLibraryDocument(m.DemoID)
	if err != nil {
		t.Fatal(err)
	}
	tl := teamvoice.TimelineOf(doc)
	if len(tl.Rounds) < 2 {
		t.Fatalf("the fixture has %d rounds, the game clock needs two", len(tl.Rounds))
	}
	// The plugin starts 60 s before the demo's tick 0: audio 0 plays at demo -60 s.
	start := m.PlayedAt.Add(-time.Hour)
	var rounds []library.RecordingRound
	for _, r := range tl.Rounds {
		rounds = append(rounds, library.RecordingRound{Round: r.Number, LiveAt: start.Add(time.Duration(r.FreezeEndMS+60_000) * time.Millisecond)})
	}
	dir := writePluginRecording(t, plugin.Inbox(), pluginRecordingSpec{id: "20261109-200000-abcd", start: start, m: m, rounds: rounds,
		tracks: []string{"mopoz", "dav1g"}, selfID: "76561198000000001"})

	env.h.scanVoiceInbox(ctx)

	if _, err := os.Stat(filepath.Join(dir, "01-mopoz.pcm")); !os.IsNotExist(err) {
		t.Fatalf("raw speech kept after encoding: %v", err)
	}
	if info, err := os.Stat(filepath.Join(dir, "01-mopoz.ogg")); err != nil || info.Size() != 2*(48000+4800) {
		t.Fatalf("encoded track = %v, %v; want the rebuilt 1.1 s", info, err)
	}
	var list struct {
		Recordings []voiceRecordingView `json:"recordings"`
	}
	env.json(http.MethodGet, "/api/library/voice/recordings", nil, http.StatusOK, &list)
	if len(list.Recordings) != 1 || list.Recordings[0].Status != "attached" || *list.Recordings[0].MatchID != m.ID || list.Recordings[0].Tracks[1].MatchTrack != 2 {
		t.Fatalf("recordings = %+v", list.Recordings)
	}
	var v voiceStatus
	env.json(http.MethodGet, "/api/library/matches/"+m.ID.String()+"/voice", nil, http.StatusOK, &v)
	if !v.Present || len(v.Tracks) != 2 || v.Tracks[0].Label != "mopoz" || v.Tracks[0].SteamID64 != "76561198000000001" || v.Tracks[1].SteamID64 != "" ||
		v.Tracks[0].Source != library.VoiceSourcePlugin || v.Sync.Method != library.VoiceMethodGameClock || v.Sync.OffsetMS != -60_000 {
		t.Fatalf("match voice = %+v", v)
	}

	// The user says who dav1g is; his team keeps it for the next recording.
	env.json(http.MethodPut, "/api/library/matches/"+m.ID.String()+"/voice/speakers",
		map[string]any{"speakers": []map[string]any{{"track": 2, "steamid64": "76561198000000002"}}}, http.StatusOK, &v)
	if v.Tracks[1].SteamID64 != "76561198000000002" {
		t.Fatalf("speakers = %+v", v.Tracks)
	}
	team, ok := voiceplugin.TeamOf(m, "76561198000000002")
	if !ok {
		t.Fatal("the fixture's T side has no library team")
	}
	saved, err := env.repo.ListVoiceSpeakers(ctx, []uuid.UUID{team})
	if err != nil || len(saved) != 1 || saved[0].SpeakerKey != "uid:uid-dav1g" {
		t.Fatalf("saved speakers = %+v, %v", saved, err)
	}
	env.json(http.MethodPut, "/api/library/matches/"+m.ID.String()+"/voice/speakers",
		map[string]any{"speakers": []map[string]any{{"track": 9, "steamid64": "76561198000000002"}}}, http.StatusBadRequest, nil)

	// Attaching to the same match is a no-op; to another while this one keeps the voice is refused.
	other := env.seedLibraryMatch("plugin-sha-2", "Alpha", "Bravo")
	env.json(http.MethodPost, "/api/library/voice/recordings/20261109-200000-abcd/attach", map[string]any{"match_id": other.ID}, http.StatusConflict, nil)
	env.json(http.MethodDelete, "/api/library/voice/recordings/20261109-200000-abcd", nil, http.StatusNoContent, nil)
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Fatalf("recording folder after delete: %v", err)
	}
	env.json(http.MethodGet, "/api/library/matches/"+m.ID.String()+"/voice", nil, http.StatusOK, &v)
	if len(v.Tracks) != 2 {
		t.Fatal("deleting the recording took the match's voice")
	}
}

func TestPluginRecordingWaitsWithANoteAndIsAttachedByHand(t *testing.T) {
	env, plugin := newPluginTestEnv(t)
	ctx := context.Background()
	m := env.seedLibraryMatch("wait-sha", "Alpha", "Bravo")
	m, _ = env.repo.GetMatch(ctx, m.ID)
	other := m
	other.Map = "de_nuke"
	dir := writePluginRecording(t, plugin.Inbox(), pluginRecordingSpec{id: "20261109-200000-0001", start: m.PlayedAt.Add(-time.Hour), m: other, tracks: []string{"solo", "bad"}})
	// A track that fails to encode is stored as failed, once; the rest of the recording still goes in.
	env.h.scanVoiceInbox(ctx)
	env.h.scanVoiceInbox(ctx)
	rec, err := env.repo.GetVoiceRecording(ctx, "20261109-200000-0001")
	if err != nil || rec.MatchID != nil || !strings.Contains(rec.Note, "de_nuke") {
		t.Fatalf("waiting recording = %+v, %v", rec, err)
	}
	if len(rec.Tracks) != 2 || rec.Tracks[0].Failed != "" || rec.Tracks[1].Failed == "" {
		t.Fatalf("tracks = %+v, want the second one failed", rec.Tracks)
	}
	if _, err := os.Stat(filepath.Join(dir, "02-bad.pcm")); err != nil {
		t.Fatalf("the failed track's raw speech is gone: %v", err)
	}
	var plugged voicePluginStatus
	env.json(http.MethodGet, "/api/library/voice/plugin", nil, http.StatusOK, &plugged)
	if plugged.Pending != 1 || plugged.CS2Found || plugged.GameState != "missing" || plugged.Inbox != plugin.Inbox() {
		t.Fatalf("plugin status = %+v", plugged)
	}
	var view voiceRecordingView
	env.json(http.MethodPost, "/api/library/voice/recordings/20261109-200000-0001/attach", map[string]any{"match_id": m.ID}, http.StatusOK, &view)
	if view.Status != "attached" || view.Note != "Asociada a mano." {
		t.Fatalf("attach by hand = %+v", view)
	}
	var v voiceStatus
	env.json(http.MethodGet, "/api/library/matches/"+m.ID.String()+"/voice", nil, http.StatusOK, &v)
	if len(v.Tracks) != 1 || v.Sync.Method == library.VoiceMethodGameClock {
		t.Fatalf("a recording without rounds = %+v; want the usual estimate", v.Sync)
	}
	env.json(http.MethodPost, "/api/library/voice/recordings/bad/attach", map[string]any{"match_id": m.ID}, http.StatusBadRequest, nil)
	env.json(http.MethodPost, "/api/library/voice/plugin/setup", nil, http.StatusNotFound, nil)
}

func TestGameStatePostsNeedTheTokenAndFeedTheLiveAnswer(t *testing.T) {
	env, plugin := newPluginTestEnv(t)
	tok, err := plugin.Token()
	if err != nil {
		t.Fatal(err)
	}
	post := func(token string) int {
		body, _ := json.Marshal(map[string]any{"provider": map[string]any{"steamid": "76561198000000001"},
			"map": map[string]any{"name": "de_ancient", "mode": "competitive", "phase": "warmup"}, "auth": map[string]any{"token": token}})
		return env.do(http.MethodPost, "/api/voice-plugin/gamestate", body, "application/json").Code
	}
	if code := post("wrong"); code != http.StatusUnauthorized {
		t.Fatalf("a wrong token = %d", code)
	}
	if body := env.do(http.MethodGet, "/api/voice-plugin/live", nil, "").Body.String(); body != "live 0\n" {
		t.Fatalf("live before CS2 = %q", body)
	}
	if code := post(tok); code != http.StatusNoContent {
		t.Fatalf("the right token = %d", code)
	}
	live := env.do(http.MethodGet, "/api/voice-plugin/live", nil, "")
	if !strings.HasPrefix(live.Body.String(), "live 1\nsession ") || !strings.Contains(live.Body.String(), "map de_ancient\n") ||
		!strings.HasPrefix(live.Header().Get("Content-Type"), "text/plain") {
		t.Fatalf("live = %q (%s)", live.Body.String(), live.Header().Get("Content-Type"))
	}
}
