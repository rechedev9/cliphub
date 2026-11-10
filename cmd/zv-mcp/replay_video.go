package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"image"
	"image/jpeg"
	"image/png"
	"io"
	"slices"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/rechedev9/cliphub/internal/replaygif"
	"github.com/rechedev9/cliphub/internal/replayvideo"
	"github.com/rechedev9/cliphub/internal/tuiclient"
)

type replayVideoIn struct {
	MatchID      string   `json:"match_id" jsonschema:"match id from list_matches"`
	Round        int      `json:"round,omitempty" jsonschema:"round number, as in a round ref; or use rounds"`
	Rounds       []int    `json:"rounds,omitempty" jsonschema:"several rounds of the match, played one after another with the same tramo, as a playlist"`
	FromSecond   float64  `json:"from_second,omitempty" jsonschema:"start of the tramo, in seconds after the round went live (or after the plant with from_plant); 0 by default"`
	ToSecond     float64  `json:"to_second,omitempty" jsonschema:"end of the tramo, on the same clock; the end of the round by default"`
	FromPlant    bool     `json:"from_plant,omitempty" jsonschema:"count from_second and to_second from the bomb plant"`
	Speed        float64  `json:"speed,omitempty" jsonschema:"how fast the video plays the tramo: 0.5, 1 (default), 2 or 4; the voice follows"`
	FPS          int      `json:"fps,omitempty" jsonschema:"frames a second: 24, 30 (default) or 60"`
	Size         int      `json:"size,omitempty" jsonschema:"side of the square video in pixels, from 256 to 1024; 1024 by default"`
	IncludeVoice bool     `json:"include_voice,omitempty" jsonschema:"true adds the match's synced team voice (TeamSpeak recording), cut to each round's tramo"`
	Layers       []string `json:"layers,omitempty" jsonschema:"what to draw: players, utility, bomb, drawings and clock; every layer by default"`
	Path         string   `json:"path" jsonschema:"absolute path of a new .mp4 file on this PC"`
}

type replayVideoOut struct {
	Path     string  `json:"path"`
	Bytes    int64   `json:"bytes"`
	Frames   int     `json:"frames"`
	FPS      int     `json:"fps"`
	Seconds  float64 `json:"seconds" jsonschema:"how long the video plays"`
	Rounds   []int   `json:"rounds"`
	Voice    bool    `json:"voice" jsonschema:"whether the MP4 carries the team voice"`
	Radar    bool    `json:"radar" jsonschema:"false when Studio has no CS2 radar for the map: the players are drawn on a plain plate"`
	Drawings int     `json:"drawings" jsonschema:"saved drawings of the rounds, each shown at its moment"`
}

var videoLayers = []string{"players", "utility", "bomb", "drawings", "clock"}

// hiddenLayers turns the layers asked for into what the frames leave out.
func hiddenLayers(layers []string) (replaygif.Layers, bool, error) {
	if len(layers) == 0 {
		return replaygif.Layers{}, true, nil
	}
	for _, l := range layers {
		if !slices.Contains(videoLayers, l) {
			return replaygif.Layers{}, false, fmt.Errorf("layer %q is not one of players, utility, bomb, drawings, clock", l)
		}
	}
	has := func(name string) bool { return slices.Contains(layers, name) }
	return replaygif.Layers{Players: !has("players"), Utility: !has("utility"), Bomb: !has("bomb"), Clock: !has("clock")}, has("drawings"), nil
}

// videoRounds is the rounds the video plays, in order.
func videoRounds(in replayVideoIn) ([]int, error) {
	rounds := in.Rounds
	if in.Round != 0 {
		rounds = append([]int{in.Round}, rounds...)
	}
	if len(rounds) == 0 {
		return nil, errors.New("pass round or rounds")
	}
	if len(rounds) > 30 {
		return nil, errors.New("a video takes at most 30 rounds")
	}
	for _, r := range rounds {
		if r < 1 || r > maxPlayRound {
			return nil, fmt.Errorf("round must be from 1 to %d", maxPlayRound)
		}
	}
	return rounds, nil
}

func checkVideoIn(in replayVideoIn) (replayVideoIn, error) {
	if err := checkUUID("match_id", in.MatchID); err != nil {
		return in, err
	}
	if in.Speed == 0 {
		in.Speed = 1
	}
	if !slices.Contains(replayvideo.Speeds, in.Speed) {
		return in, errors.New("speed must be 0.5, 1, 2 or 4")
	}
	if in.FPS == 0 {
		in.FPS = 30
	}
	if !slices.Contains(replayvideo.FrameRates, in.FPS) {
		return in, errors.New("fps must be 24, 30 or 60")
	}
	if in.Size == 0 {
		in.Size = 1024
	}
	if in.Size < 256 || in.Size > 1024 {
		return in, errors.New("size must be from 256 to 1024")
	}
	in.Size -= in.Size % 2
	return in, checkNewLocalFile(in.Path, ".mp4")
}

// videoScenes draws each round's tramo: the frames, the radar and the drawings asked for.
func videoScenes(ctx context.Context, c *tuiclient.Client, in replayVideoIn, rounds []int) ([]replaygif.Scene, []tuiclient.ReplayVideoSegment, error) {
	hide, drawings, err := hiddenLayers(in.Layers)
	if err != nil {
		return nil, nil, err
	}
	doc, err := c.LibraryTacticalPlan(ctx, in.MatchID)
	if err != nil {
		return nil, nil, toolError(err)
	}
	rate := doc.Demo.Tickrate
	if rate <= 0 {
		rate = 64
	}
	var radar image.Image
	if raw, err := c.MapRadar(ctx, doc.Demo.Map); err == nil {
		if img, err := png.Decode(bytes.NewReader(raw)); err == nil {
			radar = img
		}
	}
	var scenes []replaygif.Scene
	var segments []tuiclient.ReplayVideoSegment
	for _, n := range rounds {
		round, err := c.LibraryTacticalRound(ctx, in.MatchID, n)
		if err != nil {
			return nil, nil, toolError(err)
		}
		from, to, err := gifTicks(replayGIFIn{Round: n, FromSecond: in.FromSecond, ToSecond: in.ToSecond, FromPlant: in.FromPlant}, round, rate)
		if err != nil {
			return nil, nil, err
		}
		scene := replaygif.Scene{Calibration: doc.Geometry.Calibration, Radar: radar, Tickrate: rate, Round: round.Round,
			Frames: round.Frames, FromTick: from, ToTick: to, Speed: in.Speed, Size: in.Size, Hide: hide}
		if drawings {
			notes, err := c.LibraryRoundNotes(ctx, in.MatchID, n)
			if err != nil {
				return nil, nil, toolError(err)
			}
			scene.Notes = gifNotes(notes)
		}
		frames, err := replaygif.VideoFrames(scene, in.FPS)
		if err != nil {
			return nil, nil, err
		}
		scenes = append(scenes, scene)
		segments = append(segments, tuiclient.ReplayVideoSegment{Round: n, FromTick: from, ToTick: to, Frames: frames})
	}
	return scenes, segments, nil
}

// renderFrames hands the encoder every scene's frames as JPEGs.
func renderFrames(ctx context.Context, scenes []replaygif.Scene, fps int) tuiclient.FrameWriter {
	return func(write func(io.WriterTo) error) error {
		var buf bytes.Buffer
		for _, s := range scenes {
			err := replaygif.Render(ctx, s, fps, func(img image.Image) error {
				buf.Reset()
				if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 92}); err != nil {
					return err
				}
				return write(&buf)
			})
			if err != nil {
				return err
			}
		}
		return nil
	}
}

func registerReplayVideoTool(server *mcp.Server, s *studio) {
	mcp.AddTool(server, &mcp.Tool{Name: "export_replay_video", Annotations: &mcp.ToolAnnotations{},
		Description: "Save a tramo of the 2D Replay as an H.264 MP4 on this PC, as Exportar MP4 does under the Replay's map bar: CS2's radar, " +
			"the players, smokes, fires, the planted bomb, the round clock and the drawings saved on the rounds' notes (pick them with layers). " +
			"rounds plays several rounds one after another with the same tramo. include_voice adds the team's TeamSpeak recording, " +
			"cut to each round's tramo with the match's saved sync offset; the match needs a synced recording (import_voice, sync_voice). " +
			"Studio encodes it with FFmpeg; 120 s at most."},
		func(ctx context.Context, _ *mcp.CallToolRequest, in replayVideoIn) (*mcp.CallToolResult, replayVideoOut, error) {
			in, err := checkVideoIn(in)
			if err != nil {
				return nil, replayVideoOut{}, err
			}
			rounds, err := videoRounds(in)
			if err != nil {
				return nil, replayVideoOut{}, err
			}
			c, err := s.uploadClient(ctx)
			if err != nil {
				return nil, replayVideoOut{}, toolError(err)
			}
			scenes, segments, err := videoScenes(ctx, c, in, rounds)
			if err != nil {
				return nil, replayVideoOut{}, err
			}
			spec := tuiclient.ReplayVideoSpec{FPS: in.FPS, Width: in.Size, Height: in.Size, Speed: in.Speed, IncludeVoice: in.IncludeVoice, Segments: segments}
			got, err := c.CreateReplayVideo(ctx, in.MatchID, spec, renderFrames(ctx, scenes, in.FPS))
			if err != nil {
				return nil, replayVideoOut{}, toolError(err)
			}
			var mp4 bytes.Buffer
			if err := c.ReplayVideoFile(ctx, got.VideoID, &mp4); err != nil {
				return nil, replayVideoOut{}, toolError(err)
			}
			if err := writeNewFile(in.Path, mp4.Bytes()); err != nil {
				return nil, replayVideoOut{}, err
			}
			drawings := 0
			for _, sc := range scenes {
				drawings += drawnNotes(sc.Notes)
			}
			return nil, replayVideoOut{Path: in.Path, Bytes: int64(mp4.Len()), Frames: got.Frames, FPS: in.FPS, Seconds: got.Seconds,
				Rounds: rounds, Voice: got.Voice, Radar: scenes[0].Radar != nil, Drawings: drawings}, nil
		})
}
