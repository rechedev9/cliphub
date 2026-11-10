package replayvideo

import (
	"fmt"
	"strings"
	"testing"
)

func TestArgsWithoutVoice(t *testing.T) {
	cmd, err := Args(Plan{FPS: 30, Width: 720, Height: 720, Speed: 1, Segments: []Segment{{Frames: 600}}, Out: "out.mp4"})
	if err != nil || cmd.Voice {
		t.Fatalf("Args err=%v voice=%v", err, cmd.Voice)
	}
	line := strings.Join(cmd.Args, " ")
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
	cmd, err := Args(p)
	if err != nil || !cmd.Voice {
		t.Fatalf("Args err=%v voice=%v", err, cmd.Voice)
	}
	line := strings.Join(cmd.Args, " ")
	// The 10 s lead in demo time is 5 s at speed 2.
	graph := "[1:a]atrim=start=0.000:duration=10.000,asetpts=PTS-STARTPTS[c0t0];" +
		"[c0t0]amix=inputs=1:duration=longest:normalize=0,atempo=2,adelay=5000:all=1,aresample=48000," +
		"aformat=sample_fmts=fltp:channel_layouts=stereo,apad,atrim=duration=10.000[aout]"
	for _, want := range []string{"-i pipe:0 -i a.ogg -filter_complex " + graph, "-map 0:v -map [aout]", "-c:a aac", "-t 10.000"} {
		if !strings.Contains(line, want) {
			t.Errorf("args lack %q:\n%s", want, line)
		}
	}
}

func TestArgsVoiceSegmentsShareOneInputPerTrack(t *testing.T) {
	p := Plan{FPS: 24, Width: 640, Height: 480, Speed: 0.25, Out: "out.mp4",
		Segments: []Segment{
			{Frames: 48, Voice: &Range{FromTick: 6400, ToTick: 6528}},
			{Frames: 24, Voice: &Range{FromTick: 0, ToTick: 64}},
			{Frames: 24, Voice: &Range{FromTick: 7040, ToTick: 7104}},
		},
		Sync: Sync{Tickrate: 64, OffsetMS: 30_000}, Tracks: []Track{{Path: "a.ogg", DurationMS: 600_000}, {Path: "b.ogg", DurationMS: 600_000}}}
	cmd, err := Args(p)
	if err != nil || !cmd.Voice {
		t.Fatalf("Args err=%v voice=%v", err, cmd.Voice)
	}
	line := strings.Join(cmd.Args, " ")
	for _, want := range []string{
		"-i pipe:0 -i a.ogg -i b.ogg -filter_complex ",
		"[1:a]asplit=2[t0s0][t0s1];[t0s0]atrim=start=70.000:duration=2.000,asetpts=PTS-STARTPTS[c0t0];[t0s1]atrim=start=80.000:duration=1.000,asetpts=PTS-STARTPTS[c2t0]",
		"[c0t0][c0t1]amix=inputs=2:duration=longest:normalize=0,atempo=0.5,atempo=0.5,aresample=48000",
		"atrim=duration=2.000[a0];anullsrc=r=48000:cl=stereo,aresample=48000",
		"[a0][a1][a2]concat=n=3:v=0:a=1[aout]",
		"-t 4.000",
	} {
		if !strings.Contains(line, want) {
			t.Errorf("args lack %q:\n%s", want, line)
		}
	}
	if strings.Count(line, "-i ") != 3 {
		t.Errorf("want the frames and one input per track:\n%s", line)
	}
}

func TestArgsManyRoundsAndTracksStayShort(t *testing.T) {
	var tracks []Track
	for i := range 16 {
		tracks = append(tracks, Track{Path: fmt.Sprintf(`C:\Users\someone\AppData\Roaming\cliphub-studio\voice\0b6f6c2e-1f5e-4d1c-9a43-5b1d3f6e2a10\track-%02d.ogg`, i+1), DurationMS: 3_600_000})
	}
	var segments []Segment
	for r := range 30 {
		from := 64 * (60 + r*110)
		segments = append(segments, Segment{Frames: 120, Voice: &Range{FromTick: from, ToTick: from + 64*4}})
	}
	p := Plan{FPS: 30, Width: 1024, Height: 1024, Speed: 1, Out: `C:\Temp\cliphub-replay-video\x.mp4`, GraphFile: `C:\Temp\cliphub-replay-video\x.mp4.filter`,
		Segments: segments, Sync: Sync{Tickrate: 64}, Tracks: tracks}
	cmd, err := Args(p)
	if err != nil {
		t.Fatal(err)
	}
	if n := len(strings.Join(cmd.Args, " ")); n > 4000 {
		t.Fatalf("command line is %d characters, want it far under Windows' 32,767", n)
	}
	if cmd.Graph == "" || !strings.Contains(strings.Join(cmd.Args, " "), "-/filter_complex "+p.GraphFile) {
		t.Fatal("a large graph did not go to the graph file")
	}
	if strings.Count(cmd.Graph, "atrim=start=") != 30*16 {
		t.Fatalf("graph has %d cuts, want 480", strings.Count(cmd.Graph, "atrim=start="))
	}
}

func TestArgsVoiceOutsideEveryTrack(t *testing.T) {
	p := Plan{FPS: 30, Width: 720, Height: 720, Speed: 1, Out: "out.mp4",
		Segments: []Segment{{Frames: 30, Voice: &Range{FromTick: 0, ToTick: 64}}},
		Sync:     Sync{Tickrate: 64, OffsetMS: 30_000}, Tracks: []Track{{Path: "a.ogg", DurationMS: 600_000}}}
	if p.Voices() != nil {
		t.Fatal("a stretch before the recording has voices")
	}
	if cmd, err := Args(p); err != nil || cmd.Voice {
		t.Fatalf("Args err=%v voice=%v, want a silent video", err, cmd.Voice)
	}
}
