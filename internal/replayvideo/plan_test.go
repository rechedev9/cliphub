package replayvideo

import (
	"reflect"
	"strings"
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

func TestArgsWithoutVoice(t *testing.T) {
	args, voice, err := Args(Plan{FPS: 30, Width: 720, Height: 720, Speed: 1, Segments: []Segment{{Frames: 600}}, Out: "out.mp4"})
	if err != nil || voice {
		t.Fatalf("Args err=%v voice=%v", err, voice)
	}
	line := strings.Join(args, " ")
	for _, want := range []string{"-f image2pipe -framerate 30 -c:v mjpeg -i pipe:0", "-map 0:v -c:v libx264", "-pix_fmt yuv420p", "-t 20.000", "-f mp4 out.mp4"} {
		if !strings.Contains(line, want) {
			t.Errorf("args lack %q:\n%s", want, line)
		}
	}
	if strings.Contains(line, "-c:a") || strings.Contains(line, "filter_complex") {
		t.Errorf("a video without voice has audio args:\n%s", line)
	}
}

func TestArgsVoiceOneSegment(t *testing.T) {
	p := Plan{FPS: 30, Width: 720, Height: 720, Speed: 2, Out: "out.mp4",
		Segments: []Segment{{Frames: 300, Voice: &Range{FromTick: 1280, ToTick: 2560}}},
		Sync:     Sync{Tickrate: 64, OffsetMS: 30_000}, Tracks: []Track{{Path: "a.ogg", DurationMS: 600_000}}}
	args, voice, err := Args(p)
	if err != nil || !voice {
		t.Fatalf("Args err=%v voice=%v", err, voice)
	}
	line := strings.Join(args, " ")
	// The 10 s lead in demo time is 5 s at speed 2.
	graph := "[1:a]amix=inputs=1:duration=longest:normalize=0,atempo=2,adelay=5000:all=1,aresample=48000," +
		"aformat=sample_fmts=fltp:channel_layouts=stereo,apad,atrim=duration=10.000[aout]"
	for _, want := range []string{"-ss 0.000 -t 10.000 -i a.ogg", "-filter_complex " + graph, "-map 0:v -map [aout]", "-c:a aac", "-t 10.000"} {
		if !strings.Contains(line, want) {
			t.Errorf("args lack %q:\n%s", want, line)
		}
	}
}

func TestArgsVoiceSegmentsConcat(t *testing.T) {
	p := Plan{FPS: 24, Width: 640, Height: 480, Speed: 1, Out: "out.mp4",
		Segments: []Segment{
			{Frames: 48, Voice: &Range{FromTick: 6400, ToTick: 6528}},
			{Frames: 24, Voice: &Range{FromTick: 0, ToTick: 64}},
		},
		Sync: Sync{Tickrate: 64, OffsetMS: 30_000}, Tracks: []Track{{Path: "a.ogg", DurationMS: 600_000}, {Path: "b.ogg", DurationMS: 600_000}}}
	args, voice, err := Args(p)
	if err != nil || !voice {
		t.Fatalf("Args err=%v voice=%v", err, voice)
	}
	line := strings.Join(args, " ")
	for _, want := range []string{
		"-ss 70.000 -t 2.000 -i a.ogg -ss 70.000 -t 2.000 -i b.ogg",
		"[1:a][2:a]amix=inputs=2:duration=longest:normalize=0,aresample=48000",
		"atrim=duration=2.000[a0];anullsrc=r=48000:cl=stereo,aresample=48000",
		"atrim=duration=1.000[a1];[a0][a1]concat=n=2:v=0:a=1[aout]",
		"-t 3.000",
	} {
		if !strings.Contains(line, want) {
			t.Errorf("args lack %q:\n%s", want, line)
		}
	}
}

func TestArgsVoiceOutsideEveryTrack(t *testing.T) {
	p := Plan{FPS: 30, Width: 720, Height: 720, Speed: 1, Out: "out.mp4",
		Segments: []Segment{{Frames: 30, Voice: &Range{FromTick: 0, ToTick: 64}}},
		Sync:     Sync{Tickrate: 64, OffsetMS: 30_000}, Tracks: []Track{{Path: "a.ogg", DurationMS: 600_000}}}
	if p.Voices() != nil {
		t.Fatal("a stretch before the recording has voices")
	}
	if _, voice, err := Args(p); err != nil || voice {
		t.Fatalf("Args err=%v voice=%v, want a silent video", err, voice)
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
