package streamclips

import "testing"

func TestSourceProbeNeedsRemuxOnlyForContainersTheEditorCannotPlay(t *testing.T) {
	for container, want := range map[string]bool{
		"mov,mp4,m4a,3gp,3g2,mj2": false,
		"matroska,webm":           false,
		"":                        false, // ffprobe not configured: nothing to judge
		"mpegts":                  true,
		"flv":                     true,
	} {
		if got := (SourceProbe{Container: container}).NeedsRemux(); got != want {
			t.Errorf("NeedsRemux(%q) = %v, want %v", container, got, want)
		}
	}
}
