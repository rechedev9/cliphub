package voiceplugin

import (
	"bytes"
	"encoding/binary"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/rechedev9/cliphub/internal/library"
)

const manifestJSON = `{"format":1,"plugin_version":"1.0.0","state":"complete","interrupted":false,
 "started_at_ms":1762718400000,"ended_at_ms":1762720800000,"sample_rate":48000,"trigger":"auto",
 "server":"Scrims  ES","channel":"Equipo","map":"de_mirage","self_steamid64":"76561198000000001","session":"3f2a9c",
 "score_ct":13,"score_t":9,"rounds":[{"round":1,"live_at_ms":1762718500000},{"round":0,"live_at_ms":5}],
 "tracks":[{"file":"01-mopoz","nickname":"mopoz","uid":"u1=","self":true},{"file":"02-Ñu","nickname":"","uid":"","self":false}]}`

func writeRecording(t *testing.T, inbox, id, manifest string) string {
	t.Helper()
	dir := filepath.Join(inbox, id)
	if err := os.MkdirAll(dir, 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "manifest.json"), []byte(manifest), 0o600); err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestFinishedReadsCompleteRecordingsOnly(t *testing.T) {
	d := Dir(t.TempDir())
	writeRecording(t, d.Inbox(), "20261109-200000-abcd", manifestJSON)
	writeRecording(t, d.Inbox(), "20261109-210000-0000", strings.Replace(manifestJSON, `"complete"`, `"recording"`, 1))
	writeRecording(t, d.Inbox(), "20261109-220000-ffff", strings.Replace(manifestJSON, `"01-mopoz"`, `"../evil"`, 1))
	writeRecording(t, d.Inbox(), "not-a-recording", manifestJSON)
	found, errs := d.Finished()
	if len(found) != 1 || found[0].ID != "20261109-200000-abcd" || len(errs) != 1 {
		t.Fatalf("Finished = %+v, %v", found, errs)
	}
	rec := found[0].Manifest.Recording(found[0].ID)
	if rec.Server != "Scrims ES" || rec.Map != "de_mirage" || rec.ScoreT != 9 || len(rec.Rounds) != 1 ||
		!rec.Rounds[0].LiveAt.Equal(time.UnixMilli(1762718500000)) || rec.Tracks[1].Nickname != "02-Ñu" || rec.Tracks[0].SpeakerKey() != "uid:u1=" {
		t.Fatalf("Recording = %+v", rec)
	}
	if _, err := d.Recording("../x"); err == nil {
		t.Fatal("a path outside the inbox is a recording")
	}
}

func samples(vals ...int16) []byte {
	var b bytes.Buffer
	_ = binary.Write(&b, binary.LittleEndian, vals)
	return b.Bytes()
}

func TestOpenTrackRebuildsSilenceBetweenRunsAndKeepsSpeechPastTheIndex(t *testing.T) {
	dir := t.TempDir()
	// Runs at 2 and 6 (the second overlaps nothing), then 2 samples the index never got.
	if err := os.WriteFile(filepath.Join(dir, "01-a.pcm"), samples(1, 2, 3, 4, 5, 6, 7), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "01-a.idx"), []byte("2 3\nbad line\n6 2\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	tr, err := OpenTrack(dir, "01-a")
	if err != nil {
		t.Fatal(err)
	}
	defer tr.Close()
	got, err := io.ReadAll(tr.PCM())
	if err != nil {
		t.Fatal(err)
	}
	want := samples(0, 0, 1, 2, 3, 0, 4, 5, 6, 7)
	if !bytes.Equal(got, want) || tr.Samples != 10 {
		t.Fatalf("rebuilt %v (%d samples), want %v", got, tr.Samples, want)
	}
	if _, err := OpenTrack(dir, "../01-a"); err == nil {
		t.Fatal("a track outside its folder opened")
	}
}

func TestPickMatchByMapTimeAndScore(t *testing.T) {
	start := time.Date(2026, 11, 9, 20, 0, 0, 0, time.UTC)
	rec := library.VoiceRecording{Map: "de_mirage", StartedAt: start, EndedAt: start.Add(time.Hour), ScoreCT: 13, ScoreT: 9}
	match := func(mapName string, played time.Time, a, b int) library.Match {
		return library.Match{ID: uuid.New(), Map: mapName, PlayedAt: played, Teams: []library.MatchTeam{{Score: a}, {Score: b}}}
	}
	right := match("de_mirage", start.Add(2*time.Hour), 9, 13)
	others := []library.Match{
		match("de_inferno", start.Add(time.Hour), 13, 9),
		match("de_mirage", start.Add(-5*time.Hour), 13, 9),
		match("de_mirage", start.Add(30*24*time.Hour), 13, 9),
		match("de_mirage", start.Add(time.Hour), 13, 2),
	}
	if p := PickMatch(rec, append(others, right)); p.MatchID == nil || *p.MatchID != right.ID {
		t.Fatalf("pick = %+v, want the mirage 13-9 after the recording", p)
	}
	if p := PickMatch(rec, others); p.MatchID != nil || !strings.Contains(p.Note, "Esperando la demo") {
		t.Fatalf("no fit = %+v", p)
	}
	twin := match("de_mirage", start.Add(3*time.Hour), 13, 9)
	if p := PickMatch(rec, []library.Match{right, twin}); p.MatchID != nil || !strings.Contains(p.Note, "2 partidas") {
		t.Fatalf("two fits = %+v", p)
	}
	cut := rec
	cut.Interrupted, cut.ScoreCT, cut.ScoreT = true, 5, 4
	if p := PickMatch(cut, []library.Match{right}); p.MatchID == nil {
		t.Fatalf("an interrupted recording with a partial score = %+v, want the only mirage", p)
	}
	if p := PickMatch(library.VoiceRecording{StartedAt: start, EndedAt: start}, []library.Match{right}); p.MatchID != nil || p.Note == "" {
		t.Fatalf("a recording without map = %+v, want it waiting", p)
	}
}

func TestTiePlayersUsesTheOwnSteamIDThenTheTeamsMemory(t *testing.T) {
	team := uuid.New()
	m := library.Match{
		Teams:   []library.MatchTeam{{Key: "ct-start", TeamID: &team}, {Key: "t-start"}},
		Players: []library.MatchPlayer{{SteamID64: "1", TeamKey: "ct-start"}, {SteamID64: "2", TeamKey: "ct-start"}, {SteamID64: "9", TeamKey: "t-start"}},
	}
	rec := library.VoiceRecording{SelfSteamID64: "1", Tracks: []library.RecordingTrack{
		{File: "01-me", Nickname: "me", UID: "a", Self: true},
		{File: "02-mate", Nickname: "Mate", UID: ""},
		{File: "03-guest", Nickname: "guest", UID: "g"},
		{File: "04-clone", Nickname: "clone", UID: "c"},
	}}
	saved := []library.VoiceSpeaker{
		{TeamID: team, SpeakerKey: "nick:mate", SteamID64: "2"},
		{TeamID: team, SpeakerKey: "uid:g", SteamID64: "77"}, // not in this match
		{TeamID: team, SpeakerKey: "uid:c", SteamID64: "1"},  // already the user's own track
	}
	got := TiePlayers(rec, m, saved)
	if len(got) != 2 || got["01-me"] != "1" || got["02-mate"] != "2" {
		t.Fatalf("ties = %v", got)
	}
	if s, ok := Remember(m, rec.Tracks[1], "2"); !ok || s.TeamID != team || s.SpeakerKey != "nick:mate" {
		t.Fatalf("Remember = %+v, %v", s, ok)
	}
	if _, ok := Remember(m, rec.Tracks[2], "9"); ok {
		t.Fatal("a side without a library team remembered a speaker")
	}
}

func TestPublishAndTokenStayInTheFolder(t *testing.T) {
	d := Dir(t.TempDir())
	if err := d.Publish("http://127.0.0.1:4000"); err != nil {
		t.Fatal(err)
	}
	body, _ := os.ReadFile(filepath.Join(string(d), "studio.ini"))
	if !strings.Contains(string(body), "\nweb_origin=http://127.0.0.1:4000\n") {
		t.Fatalf("studio.ini = %q", body)
	}
	first, err := d.Token()
	second, err2 := d.Token()
	if err != nil || err2 != nil || len(first) != 32 || first != second {
		t.Fatalf("tokens %q %q, %v %v", first, second, err, err2)
	}
}
