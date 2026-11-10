package replayvideo

import (
	"reflect"

	"testing"
)

func TestCutVoice(t *testing.T) {
	tracks := []Track{{Path: "a.ogg", DurationMS: 600_000}, {Path: "b.ogg", DurationMS: 100_000}}
	sync := Sync{Tickrate: 64, OffsetMS: 30_000}
	for _, tc := range []struct {
		name string
		r    Range
		want SegmentVoice
	}{
		// Demo 100 s to 120 s is audio 70 s to 90 s: both tracks recorded it.
		{"inside", Range{FromTick: 6400, ToTick: 7680}, SegmentVoice{Cuts: []Cut{{0, 70_000, 20_000}, {1, 70_000, 20_000}}}},
		// Demo 20 s to 40 s: the recording starts at demo 30 s, so 10 s of silence lead.
		{"starts inside", Range{FromTick: 1280, ToTick: 2560}, SegmentVoice{LeadMS: 10_000, Cuts: []Cut{{0, 0, 10_000}, {1, 0, 10_000}}}},
		// Demo 125 s to 135 s: track b ended at audio 100 s, halfway.
		{"short track ends", Range{FromTick: 8000, ToTick: 8640}, SegmentVoice{Cuts: []Cut{{0, 95_000, 10_000}, {1, 95_000, 5_000}}}},
		{"before the recording", Range{FromTick: 0, ToTick: 640}, SegmentVoice{}},
		{"after every track", Range{FromTick: 64 * 700, ToTick: 64 * 710}, SegmentVoice{}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := CutVoice(tc.r, sync, tracks); !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("CutVoice = %+v, want %+v", got, tc.want)
			}
		})
	}
}

func TestCutVoiceNegativeOffset(t *testing.T) {
	// The recording began 5 s before the demo: demo 10 s is audio 15 s.
	got := CutVoice(Range{FromTick: 640, ToTick: 1280}, Sync{Tickrate: 64, OffsetMS: -5000}, []Track{{DurationMS: 60_000}})
	want := SegmentVoice{Cuts: []Cut{{0, 15_000, 10_000}}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("CutVoice = %+v, want %+v", got, want)
	}
}

func TestValidate(t *testing.T) {
	ok := Plan{FPS: 30, Width: 720, Height: 720, Speed: 1, Segments: []Segment{{Frames: 1}}}
	if err := ok.Validate(); err != nil {
		t.Fatal(err)
	}
	for name, p := range map[string]Plan{
		"odd width":   {FPS: 30, Width: 721, Height: 720, Speed: 1, Segments: ok.Segments},
		"fps":         {FPS: 25, Width: 720, Height: 720, Speed: 1, Segments: ok.Segments},
		"speed":       {FPS: 30, Width: 720, Height: 720, Speed: 3, Segments: ok.Segments},
		"no segments": {FPS: 30, Width: 720, Height: 720, Speed: 1},
		"too long":    {FPS: 30, Width: 720, Height: 720, Speed: 1, Segments: []Segment{{Frames: MaxSeconds*30 + 1}}},
	} {
		if p.Validate() == nil {
			t.Errorf("%s: Validate accepted %+v", name, p)
		}
	}
}
