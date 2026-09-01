package streamclips

import "testing"

func TestCodeFromReasonMapsAcquireDisplayText(t *testing.T) {
	cases := []struct {
		reason string
		want   string
	}{
		{AcquireReasonNotFound, AcquireCodeNotFound},
		{AcquireReasonAuthRequired, AcquireCodeAuthRequired},
		{AcquireReasonUnavailable, AcquireCodeUnavailable},
		{AcquireReasonBlocked, AcquireCodeBlocked},
		{AcquireReasonTooLarge, AcquireCodeTooLarge},
		{AcquireReasonError, AcquireCodeError},
		{"ffmpeg exited with code 1", ""},
		{"", ""},
	}
	for _, tc := range cases {
		if got := CodeFromReason(tc.reason); got != tc.want {
			t.Fatalf("CodeFromReason(%q) = %q, want %q", tc.reason, got, tc.want)
		}
	}
}
