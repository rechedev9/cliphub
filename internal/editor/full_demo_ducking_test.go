package editor

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/rechedev9/cliphub/internal/recapplan"
)

// Measure separated frequency bands from decoded output. The oracle knows only
// input frequencies and windows, not the compressor's transfer function.
func TestFullDemoDecodedDuckingAndExplicitZero(t *testing.T) {
	ffmpeg := fullDemoTestFFmpeg(t)
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	dir := t.TempDir()
	makeAudio := func(name, signal string) string {
		t.Helper()
		path := filepath.Join(dir, name+".wav")
		if _, err := runFFmpegOutput(ctx, []string{ffmpeg, "-v", "error", "-f", "lavfi", "-i", signal, "-c:a", "pcm_f32le", "-ac", "2", path}, "ducking source"); err != nil {
			t.Fatal(err)
		}
		return path
	}
	game := makeAudio("game", "aevalsrc=0.2*sin(2*PI*440*t):s=48000:d=6")
	voice := makeAudio("voice", "aevalsrc=0.5*sin(2*PI*880*t)*gte(t\\,1)*lt(t\\,2):s=44100:d=6")
	music := makeAudio("music", "aevalsrc=0.5*sin(2*PI*220*t):s=48000:d=6")
	type bands struct{ MusicBefore, MusicAttack, MusicDuring, MusicRelease, MusicRecovered, GameDuring, VoiceDuring float64 }
	measurements := map[string]bands{}
	for _, name := range []string{"off", "voice duck", "slow attack", "fast release", "voice zero", "voice disabled", "game contribution", "game zero", "game voice priority", "music disabled", "all zero"} {
		t.Run(name, func(t *testing.T) {
			o := recapplan.DefaultOptions().Audio
			voiceCount, musicInput := 1, 2
			switch name {
			case "off":
				o.Music.Ducking.Enabled = false
			case "slow attack":
				o.Music.Ducking.AttackMS = 2000
			case "fast release":
				o.Music.Ducking.ReleaseMS = 20
			case "voice zero":
				o.Voice.Gain = 0
			case "voice disabled":
				o.Voice.Enabled = false
				voiceCount, musicInput = 0, 1
			case "game contribution":
				o.Voice.Gain = 0
				o.Music.Ducking.GameContribution = 1
			case "game zero":
				o.Game.Gain = 0
			case "game voice priority":
				o.Game.VoicePriority = true
			case "music disabled":
				o.Music.Enabled = false
				musicInput = -1
			case "all zero":
				o.Game.Gain, o.Voice.Gain = 0, 0
				o.Music.Enabled = false
				musicInput = -1
			}
			path := filepath.Join(dir, name+".wav")
			cmd := []string{ffmpeg, "-v", "error", "-i", game}
			if voiceCount > 0 {
				cmd = append(cmd, "-i", voice)
			}
			if musicInput >= 0 {
				cmd = append(cmd, "-i", music)
			}
			cmd = append(cmd, "-filter_complex", fullDemoRoundAudio(o, 0, 6*48000, voiceCount, musicInput), "-map", "[a]", "-c:a", "pcm_f32le", path)
			if _, err := runFFmpegOutput(ctx, cmd, "decoded ducking canary"); err != nil {
				t.Fatal(err)
			}
			pcm := fullDemoReadAudio(t, ctx, ffmpeg, path, 0, 6)
			if len(pcm) != 6*48000*4 {
				t.Fatalf("sample clock: %d bytes", len(pcm))
			}
			power := func(start, frequency float64) float64 {
				offset := int(start*48000) * 4
				return fullDemoFrequencyPower(pcm[offset:offset+4800*4], frequency)
			}
			measurements[name] = bands{power(.5, 220), power(1.02, 220), power(1.7, 220), power(2.05, 220), power(4.5, 220), power(1.7, 440), power(1.7, 880)}
		})
	}
	if t.Failed() {
		return
	}
	off, duck := measurements["off"], measurements["voice duck"]
	if duck.MusicDuring >= off.MusicDuring*.1 || duck.MusicRecovered < off.MusicRecovered*.85 {
		t.Fatalf("music did not duck and recover: off=%+v duck=%+v", off, duck)
	}
	if duck.GameDuring < off.GameDuring*.95 || duck.VoiceDuring < off.VoiceDuring*.95 {
		t.Fatal("ducking damaged game or voice")
	}
	if measurements["slow attack"].MusicAttack <= duck.MusicAttack*1.2 {
		t.Fatal("attack choice did not change decoded music envelope")
	}
	if measurements["fast release"].MusicRelease <= duck.MusicRelease*1.2 {
		t.Fatal("release choice did not change decoded music recovery")
	}
	for _, name := range []string{"voice zero", "voice disabled"} {
		m := measurements[name]
		if m.VoiceDuring >= off.VoiceDuring*1e-5 || m.MusicDuring < off.MusicDuring*.95 {
			t.Fatalf("%s: voice or sidechain not disabled: %+v", name, m)
		}
	}
	if measurements["game contribution"].MusicBefore >= off.MusicBefore*.2 {
		t.Fatal("game contribution did not duck the music without voice")
	}
	if measurements["game zero"].GameDuring >= off.GameDuring*1e-5 {
		t.Fatal("explicit zero game gain was not preserved")
	}
	if measurements["game voice priority"].GameDuring >= off.GameDuring*.2 {
		t.Fatal("voice priority did not reduce gameplay")
	}
	if measurements["music disabled"].MusicDuring >= off.MusicDuring*1e-5 {
		t.Fatal("disabled music is audible")
	}
	if measurements["all zero"] != (bands{}) {
		t.Fatalf("all zero was not silent: %+v", measurements["all zero"])
	}
	if root := os.Getenv("FULL_DEMO_EVIDENCE_DIR"); root != "" {
		if err := os.MkdirAll(root, 0700); err != nil {
			t.Fatal(err)
		}
		body, err := json.MarshalIndent(measurements, "", "  ")
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, "decoded-ducking-bands.json"), body, 0600); err != nil {
			t.Fatal(err)
		}
	}
}
