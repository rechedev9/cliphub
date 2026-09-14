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

// This decoded canary keeps the game/voice priority contract after the retired
// music bed mixer is removed. It also proves that no third input can enter the
// Full Demo audio graph.
func TestFullDemoDecodedGameVoiceMix(t *testing.T) {
	ffmpeg := fullDemoTestFFmpeg(t)
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	dir := t.TempDir()
	makeAudio := func(name, signal string) string {
		t.Helper()
		path := filepath.Join(dir, name+".wav")
		if _, err := runFFmpegOutput(ctx, []string{ffmpeg, "-v", "error", "-f", "lavfi", "-i", signal, "-c:a", "pcm_f32le", "-ac", "2", path}, "game voice source"); err != nil {
			t.Fatal(err)
		}
		return path
	}
	game := makeAudio("game", "aevalsrc=0.2*sin(2*PI*440*t):s=48000:d=6")
	voice := makeAudio("voice", "aevalsrc=0.5*sin(2*PI*880*t)*gte(t\\,1)*lt(t\\,2):s=44100:d=6")
	type bands struct{ GameDuring, VoiceDuring float64 }
	measurements := map[string]bands{}
	for _, name := range []string{"normal", "voice disabled", "game zero", "game voice priority", "all zero"} {
		t.Run(name, func(t *testing.T) {
			o := recapplan.DefaultOptions().Audio
			voiceCount := 1
			switch name {
			case "voice disabled":
				o.Voice.Enabled = false
				voiceCount = 0
			case "game zero":
				o.Game.Gain = 0
			case "game voice priority":
				o.Game.VoicePriority = true
			case "all zero":
				o.Game.Gain, o.Voice.Gain = 0, 0
			}
			path := filepath.Join(dir, name+".wav")
			cmd := []string{ffmpeg, "-v", "error", "-i", game}
			if voiceCount > 0 {
				cmd = append(cmd, "-i", voice)
			}
			cmd = append(cmd, "-filter_complex", fullDemoRoundAudio(o, 0, 6*48000, voiceCount), "-map", "[a]", "-c:a", "pcm_f32le", path)
			if _, err := runFFmpegOutput(ctx, cmd, "decoded game voice canary"); err != nil {
				t.Fatal(err)
			}
			pcm := fullDemoReadAudio(t, ctx, ffmpeg, path, 1.7, .1)
			measurements[name] = bands{fullDemoFrequencyPower(pcm, 440), fullDemoFrequencyPower(pcm, 880)}
		})
	}
	if t.Failed() {
		return
	}
	normal := measurements["normal"]
	if normal.GameDuring == 0 || normal.VoiceDuring == 0 {
		t.Fatalf("game or voice missing: %+v", normal)
	}
	if measurements["voice disabled"].VoiceDuring >= normal.VoiceDuring*1e-5 {
		t.Fatal("disabled team voice is audible")
	}
	if measurements["game zero"].GameDuring >= normal.GameDuring*1e-5 {
		t.Fatal("explicit zero game gain was not preserved")
	}
	if measurements["game voice priority"].GameDuring >= normal.GameDuring*.2 {
		t.Fatal("voice priority did not reduce gameplay")
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
		if err := os.WriteFile(filepath.Join(root, "decoded-game-voice-bands.json"), body, 0600); err != nil {
			t.Fatal(err)
		}
	}
}
