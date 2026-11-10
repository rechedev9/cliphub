package main

import (
	"strings"
	"testing"

	"github.com/rechedev9/cliphub/internal/tuiclient"
)

func TestDescribePlayScanListsVoicePeaksAndNeverExceedsTheLimit(t *testing.T) {
	scan := tuiclient.StreamPlayScan{
		Status: playScanDone,
		Plays: []tuiclient.StreamPlay{
			{ID: "play-001", Label: "Doble kill", Kind: "kill", KillSeconds: []float64{10, 12}, Score: 60, VoiceSeconds: []float64{13}, VoiceDB: 11},
			{ID: "play-002", Label: "1 kill", Kind: "kill", KillSeconds: []float64{40}, Score: 20},
			{ID: "voice-001", Label: "Pico de voz", Kind: "voice_peak", KillSeconds: []float64{}, Score: 30, Marked: true, VoiceSeconds: []float64{90}, VoiceDB: 31},
			{ID: "voice-002", Label: "Pico de voz", Kind: "voice_peak", KillSeconds: []float64{}, Score: 30, Marked: true, VoiceSeconds: []float64{200}, VoiceDB: 30},
		},
		Voice: &tuiclient.StreamVoiceScan{Enabled: true, Sensitivity: 3, Status: "done", Peaks: 2, Mix: true},
	}
	out := describePlayScan(scan, 1)
	if out.Total != 4 || len(out.Plays) != 1 {
		t.Fatalf("plays = %+v, want exactly the limit of 1", out.Plays)
	}
	if out.Plays[0].PlayID != "play-001" || !out.Plays[0].VoicePeak || !strings.Contains(out.Plays[0].Note, "raised the score") {
		t.Fatalf("best play = %+v", out.Plays[0])
	}
	out = describePlayScan(scan, 4)
	voice := out.Plays[1]
	if voice.PlayID != "voice-001" || voice.Kind != "voice_peak" || !voice.Marked || voice.Kills != 0 || !strings.Contains(voice.Note, "no kill") {
		t.Fatalf("marked peak = %+v", voice)
	}
	if !strings.Contains(out.Voice, "2 voice peaks at sensitivity 3") || !strings.Contains(out.Voice, "game mix") {
		t.Fatalf("voice = %q", out.Voice)
	}
}

func TestStreamPlayIDPatternAcceptsKillAndVoicePlays(t *testing.T) {
	for id, want := range map[string]bool{"play-001": true, "voice-012": true, "voice-": false, "../x": false, "other-001": false} {
		if got := streamPlayIDPattern.MatchString(id); got != want {
			t.Errorf("%q matches = %v, want %v", id, got, want)
		}
	}
}

func TestFindStreamPlaysSchemaBoundsTheSensitivity(t *testing.T) {
	prop := findStreamPlaysSchema().Properties["voice_sensitivity"]
	if prop.Minimum == nil || *prop.Minimum != 1 || prop.Maximum == nil || *prop.Maximum != 5 {
		t.Fatalf("voice_sensitivity schema = %+v", prop)
	}
}

func TestVoiceChangedOnlyForADoneScanWithOtherSettings(t *testing.T) {
	on, off := true, false
	done := tuiclient.StreamPlayScan{Status: playScanDone, Voice: &tuiclient.StreamVoiceScan{Enabled: true, Sensitivity: 3}}
	cases := []struct {
		name string
		scan tuiclient.StreamPlayScan
		opts tuiclient.StreamPlayScanOptions
		want bool
	}{
		{"nothing asked", done, tuiclient.StreamPlayScanOptions{}, false},
		{"same settings", done, tuiclient.StreamPlayScanOptions{Voice: &on, VoiceSensitivity: 3}, false},
		{"other sensitivity", done, tuiclient.StreamPlayScanOptions{VoiceSensitivity: 5}, true},
		{"switched off", done, tuiclient.StreamPlayScanOptions{Voice: &off}, true},
		{"old scan without voice", tuiclient.StreamPlayScan{Status: playScanDone}, tuiclient.StreamPlayScanOptions{Voice: &on}, true},
		{"still running", tuiclient.StreamPlayScan{Status: playScanRunning}, tuiclient.StreamPlayScanOptions{VoiceSensitivity: 5}, false},
	}
	for _, c := range cases {
		if got := voiceChanged(c.scan, c.opts); got != c.want {
			t.Errorf("%s: voiceChanged = %v, want %v", c.name, got, c.want)
		}
	}
}
