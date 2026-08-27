package main

import "testing"

func TestValidateMusicVolume(t *testing.T) {
	cases := []struct {
		name    string
		volume  float64
		wantErr bool
	}{
		{"default", 1.0, false},
		{"low bound", 0.01, false},
		{"midrange", 0.35, false},
		{"zero rejected", 0, true},
		{"negative rejected", -0.5, true},
		{"above one rejected", 1.5, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := validateMusicVolume(tc.volume)
			if (err != nil) != tc.wantErr {
				t.Fatalf("validateMusicVolume(%v) error = %v, wantErr %v", tc.volume, err, tc.wantErr)
			}
		})
	}
}

func TestValidateThreads(t *testing.T) {
	cases := []struct {
		name    string
		threads int
		wantErr bool
	}{
		{"unset", 0, false},
		{"single", 1, false},
		{"multi", 8, false},
		{"negative rejected", -1, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := validateThreads(tc.threads)
			if (err != nil) != tc.wantErr {
				t.Fatalf("validateThreads(%d) error = %v, wantErr %v", tc.threads, err, tc.wantErr)
			}
		})
	}
}

func TestValidateOptionalMixVolume(t *testing.T) {
	cases := []struct {
		name    string
		volume  float64
		wantErr bool
		wantSet bool
		want    float64
	}{
		{name: "unset sentinel", volume: -1, wantSet: false},
		{name: "mute", volume: 0, wantSet: true, want: 0},
		{name: "midrange", volume: 0.2, wantSet: true, want: 0.2},
		{name: "full", volume: 1, wantSet: true, want: 1},
		{name: "above one rejected", volume: 1.5, wantErr: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := validateOptionalMixVolume("game-volume", tc.volume)
			if (err != nil) != tc.wantErr {
				t.Fatalf("validateOptionalMixVolume(%v) error = %v, wantErr %v", tc.volume, err, tc.wantErr)
			}
			if tc.wantErr {
				return
			}
			got := optionalMixVolume(tc.volume)
			if tc.wantSet {
				if got == nil || *got != tc.want {
					t.Fatalf("optionalMixVolume(%v) = %v, want %v", tc.volume, got, tc.want)
				}
				return
			}
			if got != nil {
				t.Fatalf("optionalMixVolume(%v) = %v, want nil", tc.volume, *got)
			}
		})
	}
}
