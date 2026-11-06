package main

import "testing"

func TestNormalizeShareCodeAcceptsWhatTheStudioFieldAccepts(t *testing.T) {
	const want = "CSGO-GADqf-jjyJ8-cSP2r-smZRo-TO2xK"
	for _, raw := range []string{want, "GADqf-jjyJ8-cSP2r-smZRo-TO2xK", " csgo-GADqf-jjyJ8 -cSP2r-smZRo-TO2xK\n", "CSGO-GADqfjjyJ8cSP2rsmZRoTO2xK"} {
		if got, ok := normalizeShareCode(raw); !ok || got != want {
			t.Errorf("normalizeShareCode(%q) = %q, %v; want %q", raw, got, ok, want)
		}
	}
	// 0, O, I and l are not in the share code alphabet.
	for _, raw := range []string{"", "CSGO-GADqf-jjyJ8-cSP2r-smZRo-TO2x", "CSGO-GADq0-jjyJ8-cSP2r-smZRo-TO2xK", "CSGO-GADqf-jjyJ8-cSP2r-smZRo-TO2xK-AAAAA"} {
		if got, ok := normalizeShareCode(raw); ok {
			t.Errorf("normalizeShareCode(%q) = %q, want a refusal", raw, got)
		}
	}
}
