package main

import (
	"strings"
	"testing"

	"github.com/rechedev9/cliphub/internal/tuiclient"
)

func TestDescribePlayScanListsVoicePeaksAndKeepsMarkedOnes(t *testing.T) {
	scan := tuiclient.StreamPlayScan{
		Status: playScanDone,
		Plays: []tuiclient.StreamPlay{
			{ID: "play-001", Label: "Doble kill", Kind: "kill", KillSeconds: []float64{10, 12}, Score: 60, VoiceSeconds: []float64{13}, VoiceDB: 11},
			{ID: "play-002", Label: "1 kill", Kind: "kill", KillSeconds: []float64{40}, Score: 20},
			{ID: "play-003", Label: "Pico de voz", Kind: "voice_peak", KillSeconds: []float64{}, Score: 70, Marked: true, VoiceSeconds: []float64{90}, VoiceDB: 17},
			{ID: "play-004", Label: "Pico de voz", Kind: "voice_peak", KillSeconds: []float64{}, Score: 70, Marked: true, VoiceSeconds: []float64{200}, VoiceDB: 16},
		},
		Voice: &tuiclient.StreamVoiceScan{Enabled: true, Sensitivity: 3, Status: "done", Peaks: 2},
	}
	out := describePlayScan(scan, 1)
	if out.Total != 4 || len(out.Plays) != 2 {
		t.Fatalf("plays = %+v, want both marked peaks past the limit of 1", out.Plays)
	}
	first := out.Plays[0]
	if first.PlayID != "play-003" || first.Kind != "voice_peak" || !first.Marked || first.Kills != 0 || !strings.Contains(first.Note, "no kill") {
		t.Fatalf("marked peak = %+v", first)
	}
	out = describePlayScan(scan, 3)
	if len(out.Plays) != 3 || out.Plays[2].PlayID != "play-001" || !out.Plays[2].VoicePeak || out.Plays[2].Kind != "kill" || !strings.Contains(out.Plays[2].Note, "raised the score") {
		t.Fatalf("kill with shout = %+v", out.Plays)
	}
	if out.Voice != "done: 2 voice peaks at sensitivity 3" {
		t.Fatalf("voice = %q", out.Voice)
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
