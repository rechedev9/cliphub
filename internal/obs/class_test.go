package obs

import "testing"

// The reason below is the one the 2026-09-23 incident recorded when a CS2
// update broke HLAE; the cloud worker pauses itself on this class.
func TestClassOfCaptureIncompatible(t *testing.T) {
	const incident = `HLAE hook crashed with a native error dialog ("Error - AfxHookSource2")`
	cases := []struct {
		name    string
		message string
		want    string
	}{
		{name: "incident text", message: incident, want: ClassCaptureIncompatible},
		{
			name:    "wrapped by the record worker",
			message: `C:\Studio\zv-recorder.exe failed: exit status 6: ` + incident + `; the installed HLAE/AfxHookSource2 build is likely incompatible`,
			want:    ClassCaptureIncompatible,
		},
		{
			name:    "restart reason still wins",
			message: "interrupted: orchestrator restarted during recording",
			want:    ClassInterrupted,
		},
		{
			name:    "other capture failures keep their class",
			message: "observer target 5 drifted from 3",
			want:    ClassCaptureFlake,
		},
		{name: "unrelated failure", message: "ffmpeg exited with status 1", want: ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := ClassOf(tc.message); got != tc.want {
				t.Fatalf("ClassOf(%q) = %q, want %q", tc.message, got, tc.want)
			}
		})
	}
}
