package vodfetch

import "testing"

func TestParseYtdlpProgressLine(t *testing.T) {
	cases := []struct {
		line      string
		wantDone  int64
		wantTotal int64
		wantOK    bool
	}{
		{line: "[download]  50.0% of  1000B at  1B/s ETA 00:07", wantDone: 500, wantTotal: 1000, wantOK: true},
		{line: "[download]  45.2% of ~ 10.00MiB at  1.23MiB/s ETA 00:07", wantDone: 4739564, wantTotal: 10485760, wantOK: true},
		{line: "[download] 100% of 1.00KiB in 00:01", wantDone: 1024, wantTotal: 1024, wantOK: true},
		{line: "[download] Destination: clip.mp4", wantOK: false},
		{line: "not a progress line", wantOK: false},
	}
	for _, tc := range cases {
		done, total, ok := ParseYtdlpProgressLine(tc.line)
		if ok != tc.wantOK {
			t.Fatalf("ok(%q) = %v, want %v", tc.line, ok, tc.wantOK)
		}
		if !ok {
			continue
		}
		if done != tc.wantDone || total != tc.wantTotal {
			t.Fatalf("ParseYtdlpProgressLine(%q) = %d/%d, want %d/%d", tc.line, done, total, tc.wantDone, tc.wantTotal)
		}
	}
}
