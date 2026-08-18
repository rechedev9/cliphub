package sharecode

import (
	"strings"
	"testing"
)

// goldenMatch is the known-good vector that pins the byte order; the
// round-trip table alone cannot validate it.
var goldenMatch = Match{
	MatchID:   3230642215713767580,
	OutcomeID: 3230647599455273103,
	TokenID:   55788,
}

func TestDecodeGolden(t *testing.T) {
	tests := []struct {
		name string
		code string
		want Match
	}{
		{name: "with prefix", code: "CSGO-GADqf-jjyJ8-cSP2r-smZRo-TO2xK", want: goldenMatch},
		{name: "without prefix", code: "GADqf-jjyJ8-cSP2r-smZRo-TO2xK", want: goldenMatch},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Decode(tt.code)
			if err != nil {
				t.Fatalf("Decode(%q) returned error: %v", tt.code, err)
			}
			if got != tt.want {
				t.Fatalf("Decode(%q) = %+v, want %+v", tt.code, got, tt.want)
			}
		})
	}
}

func TestDecodeErrors(t *testing.T) {
	tests := []struct {
		name    string
		code    string
		wantMsg string
	}{
		{name: "empty string", code: "", wantMsg: "expected 25 characters"},
		{name: "24 characters", code: "CSGO-GADqf-jjyJ8-cSP2r-smZRo-TO2x", wantMsg: "expected 25 characters"},
		{name: "26 characters", code: "CSGO-GADqf-jjyJ8-cSP2r-smZRo-TO2xKA", wantMsg: "expected 25 characters"},
		{name: "excluded uppercase I", code: "CSGO-GADqf-jjyJ8-cSP2r-smZRo-TO2xI", wantMsg: "not in the share code dictionary"},
		{name: "excluded lowercase g", code: "CSGO-GADqf-jjyJ8-cSP2r-smZRo-TO2xg", wantMsg: "not in the share code dictionary"},
		{name: "excluded lowercase l", code: "CSGO-GADqf-jjyJ8-cSP2r-smZRo-TO2xl", wantMsg: "not in the share code dictionary"},
		{name: "excluded digit 0", code: "CSGO-GADqf-jjyJ8-cSP2r-smZRo-TO2x0", wantMsg: "not in the share code dictionary"},
		{name: "excluded digit 1", code: "CSGO-GADqf-jjyJ8-cSP2r-smZRo-TO2x1", wantMsg: "not in the share code dictionary"},
		{name: "misplaced separator in prefix", code: "CSG-OGADqf-jjyJ8-cSP2r-smZRo-TO2xK", wantMsg: "expected 25 characters"},
		{name: "value exceeds 144 bits", code: "CSGO-99999-99999-99999-99999-99999", wantMsg: "exceeds 144 bits"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Decode(tt.code)
			if err == nil {
				t.Fatalf("Decode(%q) succeeded, want error containing %q", tt.code, tt.wantMsg)
			}
			if !strings.Contains(err.Error(), tt.wantMsg) {
				t.Fatalf("Decode(%q) error = %q, want it to contain %q", tt.code, err, tt.wantMsg)
			}
		})
	}
}

// TestRoundTrip is a consistency check only: Decode(Encode(m)) == m holds for
// any internally consistent byte order, so it does NOT validate the byte
// order. The golden vector in TestDecodeGolden is what does that.
func TestRoundTrip(t *testing.T) {
	tests := []struct {
		name string
		m    Match
	}{
		{name: "golden vector", m: goldenMatch},
		{name: "small values", m: Match{MatchID: 1, OutcomeID: 2, TokenID: 3}},
		{name: "large values", m: Match{MatchID: 18446744073709551615, OutcomeID: 9876543210123456789, TokenID: 65535}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			code := Encode(tt.m)
			got, err := Decode(code)
			if err != nil {
				t.Fatalf("Decode(Encode(%+v)) = Decode(%q) returned error: %v", tt.m, code, err)
			}
			if got != tt.m {
				t.Fatalf("Decode(Encode(%+v)) = %+v via code %q", tt.m, got, code)
			}
		})
	}
}
