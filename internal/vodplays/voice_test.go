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
	// -10 is 20 dB over the usual level: loud, but gunfire and music get there; only -1 is a shout.
	levels := series(120, -30, map[[2]float64]float64{{20, 22}: -18, {50, 53}: -10, {80, 83}: -1})
	peaks := DetectVoicePeaks(levels, DefaultVoiceSensitivity)
	if len(peaks) != 3 || peaks[0].Marked || peaks[1].Marked || !peaks[2].Marked {
		t.Fatalf("peaks = %+v, want only the last marked", peaks)
	}
}

func TestDetectVoicePeaksNeedsAHeldShoutToMark(t *testing.T) {
	// One window at -1 is a blip, not a held shout.
	levels := series(120, -30, map[[2]float64]float64{{80, 80.5}: -1, {40, 42}: -18})
	for _, p := range DetectVoicePeaks(levels, 5) {
		if p.Marked {
			t.Fatalf("a single loud window was marked: %+v", p)
		}
	}
}

func TestDetectVoicePeaksBaselineFollowsTheStreamerOverTime(t *testing.T) {
	quiet := series(1000, -45, map[[2]float64]float64{{500, 503}: -25})
	loud := series(1000, -20, nil)
	got := DetectVoicePeaks(append(quiet, loud...), DefaultVoiceSensitivity)
	found := false
	for _, p := range got {
		if p.StartSeconds >= 1200 {
			t.Fatalf("a normal level of the loud hour is a peak: %+v", p)
		}
		found = found || (p.StartSeconds >= 500 && p.StartSeconds < 503)
	}
	if !found {
		t.Fatalf("the shout in the quiet part was missed: %+v", got)
	}
}

func TestDetectVoicePeaksKeepsFewMarkedPerHour(t *testing.T) {
	loud := map[[2]float64]float64{}
	for i := 0; i < 8; i++ {
		start := float64(100 + 300*i)
		loud[[2]float64{start, start + 2}] = -1
	}
	marked := 0
	for _, p := range DetectVoicePeaks(series(3000, -30, loud), DefaultVoiceSensitivity) {
		if p.Marked {
			marked++
		}
	}
	if marked != maxMarkedPerHour {
		t.Fatalf("marked = %d, want %d", marked, maxMarkedPerHour)
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
	if withVoice.ID != "play-001" || killOnly.ID != "play-002" || reaction.ID != "voice-001" {
		t.Fatalf("ids = %q %q %q, want kill ids kept and a voice sequence", withVoice.ID, killOnly.ID, reaction.ID)
	}
}

func TestAddVoicePeaksScoresVoiceOnlyBelowAKillWithAShout(t *testing.T) {
	oneKill := AddVoicePeaks(GroupPlays([]Kill{{Seconds: 50}}, 200), []VoicePeak{{StartSeconds: 52, EndSeconds: 53, ExcessDB: 9}}, 200)[0]
	for _, peak := range []VoicePeak{{StartSeconds: 130, EndSeconds: 132, ExcessDB: 40}, {StartSeconds: 130, EndSeconds: 132, ExcessDB: 40, Marked: true}} {
		got := AddVoicePeaks(nil, []VoicePeak{peak}, 200)
		if len(got) != 1 || got[0].Score >= oneKill.Score {
			t.Fatalf("voice-only %+v must score under a kill with a shout (%d)", got, oneKill.Score)
		}
		if got[0].Marked != peak.Marked {
			t.Fatalf("marked = %v, want %v", got[0].Marked, peak.Marked)
		}
	}
}

func TestAddVoicePeaksKeepsTheLoudestVoiceOnlyPlaysPerHour(t *testing.T) {
	var peaks []VoicePeak
	for i := 0; i < 10; i++ {
		start := float64(100 + 200*i)
		peaks = append(peaks, VoicePeak{StartSeconds: start, EndSeconds: start + 2, ExcessDB: float64(10 + i)})
	}
	// The ten peaks sit in the first hour; a lone peak in the second hour is kept on its own.
	peaks = append(peaks, VoicePeak{StartSeconds: 4000, EndSeconds: 4002, ExcessDB: 9})
	got := AddVoicePeaks(nil, peaks, 5000)
	if len(got) != maxVoiceOnlyPerHour+1 {
		t.Fatalf("plays = %d, want %d", len(got), maxVoiceOnlyPerHour+1)
	}
	for _, p := range got[:maxVoiceOnlyPerHour] {
		if p.VoiceDB < 14 {
			t.Fatalf("kept a quiet peak (%v dB) over a louder one", p.VoiceDB)
		}
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
	if track, mic := VoiceTrack(nil, 1); track != 0 || mic {
		t.Fatalf("single track = %d, %v", track, mic)
	}
	if track, mic := VoiceTrack([]string{"", ""}, 2); track != 1 || !mic {
		t.Fatalf("two untitled tracks = %d, %v", track, mic)
	}
	if track, mic := VoiceTrack([]string{"Game", "Chat", "Microphone"}, 3); track != 2 || !mic {
		t.Fatalf("titled mic = %d, %v", track, mic)
	}
}

func TestVoicedEnoughNeedsRealAudio(t *testing.T) {
	silent := make([]float64, 400)
	for i := range silent {
		silent[i] = -90
	}
	if VoicedEnough(silent) || VoicedEnough(nil) {
		t.Fatal("silence has no voice")
	}
	if !VoicedEnough(series(120, -30, nil)) {
		t.Fatal("steady speech has a voice")
	}
}

func TestAddVoicePeaksJoinsTouchingPeaksIntoOneMoment(t *testing.T) {
	peaks := []VoicePeak{
		{StartSeconds: 800, EndSeconds: 804, ExcessDB: 9},
		{StartSeconds: 812, EndSeconds: 815, ExcessDB: 14},
		{StartSeconds: 822, EndSeconds: 826, ExcessDB: 11},
		{StartSeconds: 834, EndSeconds: 837, ExcessDB: 10},
	}
	got := AddVoicePeaks(nil, peaks, 2000)
	if len(got) != 1 {
		t.Fatalf("plays = %+v, want one moment", got)
	}
	if got[0].VoiceDB != 14 {
		t.Fatalf("level = %v, want the highest peak (14)", got[0].VoiceDB)
	}
	if length := got[0].EndSeconds - got[0].StartSeconds; length > voiceMaxMomentSeconds+0.001 {
		t.Fatalf("moment lasts %v s, want at most %v", length, voiceMaxMomentSeconds)
	}
	if got[0].StartSeconds > 813 || got[0].EndSeconds < 813 {
		t.Fatalf("moment %v to %v lost the loudest point", got[0].StartSeconds, got[0].EndSeconds)
	}
}

func TestAddVoicePeaksFoldsAReactionThatTouchesAKillPlay(t *testing.T) {
	plays := GroupPlays([]Kill{{Seconds: 100}}, 500)
	// The kill play ends at 103; the peak clip starts at 108, 5 s after the end: not touching.
	apart := AddVoicePeaks(plays, []VoicePeak{{StartSeconds: 113, EndSeconds: 115, ExcessDB: 10}}, 500)
	if len(apart) != 2 {
		t.Fatalf("a distant reaction should stay apart: %+v", apart)
	}
	// Starting at 107 its clip begins at 102, inside the kill play.
	got := AddVoicePeaks(plays, []VoicePeak{{StartSeconds: 107, EndSeconds: 109, ExcessDB: 10}}, 500)
	if len(got) != 1 || len(got[0].VoiceSeconds) != 1 || got[0].Kind != KillKind {
		t.Fatalf("touching reaction should fold into the kill play: %+v", got)
	}
}
