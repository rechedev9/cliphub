package vodplays

import (
	"bytes"
	"encoding/binary"
	"math"
	"testing"
)

// series builds a loudness series at the usual level with louder stretches
// [from, to) in seconds at the given level.
func series(seconds float64, usual float64, loud map[[2]float64]float64) []float64 {
	levels := make([]float64, int(seconds/VoiceWindowSeconds))
	for i := range levels {
		t := float64(i) * VoiceWindowSeconds
		levels[i] = usual + math.Sin(float64(i))*1.5
		for span, level := range loud {
			if t >= span[0] && t < span[1] {
				levels[i] = level
			}
		}
	}
	return levels
}

func TestDetectVoicePeaksFindsShoutsOverTheUsualLevel(t *testing.T) {
	levels := series(120, -30, map[[2]float64]float64{{40, 43}: -18, {100, 101}: -16})
	peaks := DetectVoicePeaks(levels, DefaultVoiceSensitivity)
	if len(peaks) != 2 {
		t.Fatalf("peaks = %+v, want 2", peaks)
	}
	if peaks[0].StartSeconds != 40 || peaks[0].EndSeconds != 43 {
		t.Fatalf("first peak = %+v, want 40 to 43", peaks[0])
	}
	if peaks[0].ExcessDB < 10 {
		t.Fatalf("excess = %v, want at least 10", peaks[0].ExcessDB)
	}
}

func TestDetectVoicePeaksSensitivityDecidesWhatCounts(t *testing.T) {
	levels := series(120, -30, map[[2]float64]float64{{40, 43}: -23})
	if got := DetectVoicePeaks(levels, 1); len(got) != 0 {
		t.Fatalf("lowest sensitivity found %+v, want none", got)
	}
	if got := DetectVoicePeaks(levels, 5); len(got) != 1 {
		t.Fatalf("highest sensitivity found %+v, want one", got)
	}
}

func TestDetectVoicePeaksBaselineIsTheStreamersOwn(t *testing.T) {
	quiet := series(120, -45, map[[2]float64]float64{{60, 62}: -33})
	loud := series(120, -15, map[[2]float64]float64{{60, 62}: -3})
	for name, levels := range map[string][]float64{"quiet": quiet, "loud": loud} {
		if got := DetectVoicePeaks(levels, DefaultVoiceSensitivity); len(got) != 1 {
			t.Fatalf("%s streamer: peaks = %+v, want one", name, got)
		}
	}
}

func TestDetectVoicePeaksIgnoresFlatAudioAndSilence(t *testing.T) {
	if got := DetectVoicePeaks(series(120, -30, nil), 5); len(got) != 0 {
		t.Fatalf("flat audio gave %+v", got)
	}
	silent := make([]float64, 400)
	for i := range silent {
		silent[i] = -90
	}
	if got := DetectVoicePeaks(silent, 5); len(got) != 0 {
		t.Fatalf("silence gave %+v", got)
	}
}

func TestDetectVoicePeaksSkipsBlipsAndMergesNearPeaks(t *testing.T) {
	blip := series(120, -30, map[[2]float64]float64{{40, 40.5}: -24})
	if got := DetectVoicePeaks(blip, DefaultVoiceSensitivity); len(got) != 0 {
		t.Fatalf("one mild window gave %+v", got)
	}
	near := series(120, -30, map[[2]float64]float64{{40, 42}: -18, {44, 46}: -18})
	got := DetectVoicePeaks(near, DefaultVoiceSensitivity)
	if len(got) != 1 || got[0].StartSeconds != 40 || got[0].EndSeconds != 46 {
		t.Fatalf("near peaks = %+v, want one from 40 to 46", got)
	}
}

func TestDetectVoicePeaksMarksDeliberateShouts(t *testing.T) {
	levels := series(120, -30, map[[2]float64]float64{{20, 22}: -18, {80, 83}: -10})
	peaks := DetectVoicePeaks(levels, DefaultVoiceSensitivity)
	if len(peaks) != 2 || peaks[0].Marked || !peaks[1].Marked {
		t.Fatalf("peaks = %+v, want only the second marked", peaks)
	}
}

func TestAddVoicePeaksRaisesAKillPlayAndAddsAReactionPlay(t *testing.T) {
	kills := []Kill{{Seconds: 50}, {Seconds: 80}}
	plays := GroupPlays(kills, 200)
	peaks := []VoicePeak{
		{StartSeconds: 52, EndSeconds: 54, ExcessDB: 12},
		{StartSeconds: 120, EndSeconds: 123, ExcessDB: 14},
	}
	got := AddVoicePeaks(plays, peaks, 200)
	if len(got) != 3 {
		t.Fatalf("plays = %+v, want 3", got)
	}
	withVoice, killOnly, reaction := got[0], got[1], got[2]
	if len(withVoice.VoiceSeconds) != 1 || withVoice.Score <= killOnly.Score {
		t.Fatalf("kill with shout %+v must outrank the kill alone %+v", withVoice, killOnly)
	}
	if reaction.Kind != VoiceKind || reaction.Label != VoiceLabel || len(reaction.KillSeconds) != 0 {
		t.Fatalf("reaction = %+v", reaction)
	}
	if reaction.StartSeconds != 115 || reaction.EndSeconds != 126 {
		t.Fatalf("reaction window = %v to %v, want 115 to 126", reaction.StartSeconds, reaction.EndSeconds)
	}
	for i, p := range got {
		if want := "play-00" + string(rune('1'+i)); p.ID != want {
			t.Fatalf("id %d = %q, want %q", i, p.ID, want)
		}
	}
}

func TestAddVoicePeaksMarkedPeakScoresHigh(t *testing.T) {
	got := AddVoicePeaks(nil, []VoicePeak{{StartSeconds: 30, EndSeconds: 32, ExcessDB: 16, Marked: true}}, 100)
	if len(got) != 1 || !got[0].Marked || got[0].Score < markedScoreFloor {
		t.Fatalf("plays = %+v", got)
	}
}

func TestAddVoicePeaksTrimsReactionAgainstNeighbours(t *testing.T) {
	plays := GroupPlays([]Kill{{Seconds: 50}}, 200)
	// The kill play ends at 53; a peak at 62 would lead in from 57 and must not overlap the next play.
	next := GroupPlays([]Kill{{Seconds: 69}}, 200)
	all := append(plays, next...)
	got := AddVoicePeaks(all, []VoicePeak{{StartSeconds: 62, EndSeconds: 63, ExcessDB: 12}}, 200)
	for i := 1; i < len(got); i++ {
		if got[i].StartSeconds < got[i-1].EndSeconds {
			t.Fatalf("plays %d and %d overlap: %+v", i-1, i, got)
		}
	}
}

func TestWindowLevelsMeasuresRMSInDecibels(t *testing.T) {
	perWindow := int(VoiceWindowSeconds * voiceSampleRate)
	var pcm bytes.Buffer
	for _, amplitude := range []int16{16384, 0, 3277} {
		for i := 0; i < perWindow; i++ {
			_ = binary.Write(&pcm, binary.LittleEndian, amplitude)
		}
	}
	// A partial trailing window is ignored.
	pcm.Write([]byte{1, 2, 3, 4})
	levels, err := windowLevels(&pcm)
	if err != nil {
		t.Fatal(err)
	}
	if len(levels) != 3 {
		t.Fatalf("levels = %v, want 3 windows", levels)
	}
	// A constant signal's RMS is its amplitude: half scale is -6.02 dBFS, a tenth is -20 dBFS.
	if math.Abs(levels[0]+6.02) > 0.05 || levels[1] != -90 || math.Abs(levels[2]+20) > 0.05 {
		t.Fatalf("levels = %v", levels)
	}
}

func TestVoiceSettingsNormalizedAndTrack(t *testing.T) {
	if got := (VoiceSettings{Enabled: true}).Normalized().Sensitivity; got != DefaultVoiceSensitivity {
		t.Fatalf("default sensitivity = %d", got)
	}
	if got := (VoiceSettings{Sensitivity: 99}).Normalized().Sensitivity; got != MaxVoiceSensitivity {
		t.Fatalf("clamped sensitivity = %d", got)
	}
	if VoiceTrack(1) != 0 || VoiceTrack(0) != 0 || VoiceTrack(3) != 1 {
		t.Fatal("track pick")
	}
}
