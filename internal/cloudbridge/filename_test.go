package cloudbridge

import (
	"strings"
	"testing"
)

func TestCloudFileName(t *testing.T) {
	cases := []struct {
		name           string
		requestID      string
		submitterLabel string
		originalName   string
		want           string
	}{
		{
			name:           "typical request",
			requestID:      "5fa7627c-a94f-4855-9bbe-6af9997e6f3e",
			submitterLabel: "alice@example.com",
			originalName:   "mirage.dem",
			want:           "cloud-5fa7627c-alice_example.com-mirage.dem",
		},
		{
			name:           "short request id kept as-is",
			requestID:      "abc",
			submitterLabel: "bob",
			originalName:   "dust2.dem",
			want:           "cloud-abc-bob-dust2.dem",
		},
		{
			name:           "empty submitter falls back to unknown",
			requestID:      "5fa7627c-a94f-4855-9bbe-6af9997e6f3e",
			submitterLabel: "",
			originalName:   "match.dem",
			want:           "cloud-5fa7627c-unknown-match.dem",
		},
		{
			name:           "empty original name falls back to demo.dem",
			requestID:      "5fa7627c-a94f-4855-9bbe-6af9997e6f3e",
			submitterLabel: "carol",
			originalName:   "",
			want:           "cloud-5fa7627c-carol-demo.dem",
		},
		{
			name:           "a path in the original name keeps only its last element",
			requestID:      "5fa7627c-a94f-4855-9bbe-6af9997e6f3e",
			submitterLabel: "mallory",
			originalName:   `..\..\Windows/System32/evil.dem`,
			want:           "cloud-5fa7627c-mallory-evil.dem",
		},
		{
			name:           "control and invisible format characters are dropped",
			requestID:      "5fa7627c-a94f-4855-9bbe-6af9997e6f3e",
			submitterLabel: "mallory",
			originalName:   "ma\u202etch\u200b\x00\n.dem",
			want:           "cloud-5fa7627c-mallory-match.dem",
		},
		{
			name:           "a name that is only separators falls back to demo.dem",
			requestID:      "5fa7627c-a94f-4855-9bbe-6af9997e6f3e",
			submitterLabel: "mallory",
			originalName:   "../",
			want:           "cloud-5fa7627c-mallory-demo.dem",
		},
		{
			name:           "a very long original name is capped",
			requestID:      "5fa7627c-a94f-4855-9bbe-6af9997e6f3e",
			submitterLabel: "mallory",
			originalName:   strings.Repeat("\u00f1", 200),
			want:           "cloud-5fa7627c-mallory-" + strings.Repeat("\u00f1", maxOriginalNameRunes),
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := cloudFileName(tc.requestID, tc.submitterLabel, tc.originalName)
			if got != tc.want {
				t.Fatalf("cloudFileName(%q, %q, %q) = %q, want %q", tc.requestID, tc.submitterLabel, tc.originalName, got, tc.want)
			}
		})
	}
}

func TestSanitizeHandleComponentCapsLength(t *testing.T) {
	long := ""
	for range 100 {
		long += "a"
	}
	got := sanitizeHandleComponent(long)
	if len(got) != 40 {
		t.Fatalf("sanitizeHandleComponent length = %d, want 40", len(got))
	}
}

func TestSanitizeHandleComponentReplacesUnsafeRunes(t *testing.T) {
	got := sanitizeHandleComponent("Alice Smith/../<script>")
	for _, r := range got {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '.', r == '-', r == '_':
		default:
			t.Fatalf("unexpected unsafe rune %q in sanitized output %q", r, got)
		}
	}
}
