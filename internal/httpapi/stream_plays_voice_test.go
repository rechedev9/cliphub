package httpapi

import (
	"testing"

	"github.com/rechedev9/cliphub/internal/vodplays"
)

func TestStartScanRequestKeepsDefaultsForMissingFields(t *testing.T) {
	off := false
	cases := []struct {
		name string
		req  startScanRequest
		want vodplays.VoiceSettings
	}{
		{"empty body", startScanRequest{}, vodplays.VoiceSettings{Enabled: true, Sensitivity: 3}},
		{"switched off", startScanRequest{Voice: &off}, vodplays.VoiceSettings{Enabled: false, Sensitivity: 3}},
		{"high sensitivity", startScanRequest{VoiceSensitivity: 5}, vodplays.VoiceSettings{Enabled: true, Sensitivity: 5}},
		{"out of range", startScanRequest{VoiceSensitivity: 40}, vodplays.VoiceSettings{Enabled: true, Sensitivity: 5}},
	}
	for _, c := range cases {
		if got := c.req.settings(); got != c.want {
			t.Errorf("%s: settings = %+v, want %+v", c.name, got, c.want)
		}
	}
}
