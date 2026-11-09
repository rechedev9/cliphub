package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"image/png"
	"math"
	"slices"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/rechedev9/cliphub/internal/replaygif"
	"github.com/rechedev9/cliphub/internal/tuiclient"
)

type replayGIFIn struct {
	MatchID      string  `json:"match_id" jsonschema:"match id from list_matches"`
	Round        int     `json:"round" jsonschema:"round number, as in a round ref"`
	FromSecond   float64 `json:"from_second,omitempty" jsonschema:"start of the tramo, in seconds after the round went live (or after the plant with from_plant); 0 by default"`
	ToSecond     float64 `json:"to_second,omitempty" jsonschema:"end of the tramo, on the same clock; the end of the round by default"`
	FromPlant    bool    `json:"from_plant,omitempty" jsonschema:"count from_second and to_second from the bomb plant, as the Replay's tramo can"`
	Speed        float64 `json:"speed,omitempty" jsonschema:"how fast the GIF plays the tramo: 0.5, 1 (default), 2 or 4"`
	Size         int     `json:"size,omitempty" jsonschema:"side of the GIF in pixels, from 128 to 1024; 512 by default"`
	SkipDrawings bool    `json:"skip_drawings,omitempty" jsonschema:"true leaves out the drawings saved on the round's notes"`
	Path         string  `json:"path" jsonschema:"absolute path of a new .gif file on this PC"`
}

type replayGIFOut struct {
	Path     string  `json:"path"`
	Bytes    int     `json:"bytes"`
	Frames   int     `json:"frames"`
	Seconds  float64 `json:"seconds" jsonschema:"how long the GIF plays"`
	Radar    bool    `json:"radar" jsonschema:"false when Studio has no CS2 radar for the map: the players are drawn on a plain plate"`
	Drawings int     `json:"drawings" jsonschema:"saved drawings of the round, each shown at its moment"`
}

var gifSpeeds = []float64{0.5, 1, 2, 4}

// gifTicks turns the tramo's seconds into round ticks.
func gifTicks(in replayGIFIn, round tuiclient.TacticalRoundFrames, tickrate float64) (int, int, error) {
	origin := round.Round.TickFreezeEnd
	if in.FromPlant {
		if round.Round.Bomb == nil || round.Round.Bomb.PlantTick == 0 {
			return 0, 0, fmt.Errorf("round %d has no plant: pick a tramo from the start of the round", in.Round)
		}
		origin = round.Round.Bomb.PlantTick
	}
	if in.FromSecond < 0 || in.ToSecond < 0 || math.IsNaN(in.FromSecond) || math.IsNaN(in.ToSecond) {
		return 0, 0, errors.New("from_second and to_second must not be negative")
	}
	from := origin + int(in.FromSecond*tickrate)
	to := round.Round.TickEnd
	if in.ToSecond > 0 {
		to = min(to, origin+int(in.ToSecond*tickrate))
	}
	if to <= from {
		return 0, 0, fmt.Errorf("the tramo ends before it starts: round %d runs %.0f s after going live", in.Round, float64(round.Round.TickEnd-round.Round.TickFreezeEnd)/tickrate)
	}
	return from, to, nil
}

func registerReplayGIFTool(server *mcp.Server, s *studio) {
	mcp.AddTool(server, &mcp.Tool{Name: "export_replay_gif", Annotations: &mcp.ToolAnnotations{},
		Description: "Save one round's tramo of the 2D Replay as a looping GIF on this PC, as Exportar GIF does under the Replay's map bar: CS2's radar, " +
			"the players, smokes, fires, the planted bomb, the round clock and the drawings saved on the round's notes. " +
			"The Studio button records exactly what is on screen (layers, framing, superposed rounds, the drawing in progress); " +
			"this tool draws a fixed view of one round, which is what fits in a prep document or a Discord message."},
		func(ctx context.Context, _ *mcp.CallToolRequest, in replayGIFIn) (*mcp.CallToolResult, replayGIFOut, error) {
			if err := checkUUID("match_id", in.MatchID); err != nil {
				return nil, replayGIFOut{}, err
			}
			if in.Round < 1 || in.Round > maxPlayRound {
				return nil, replayGIFOut{}, fmt.Errorf("round must be from 1 to %d", maxPlayRound)
			}
			if in.Speed != 0 && !slices.Contains(gifSpeeds, in.Speed) {
				return nil, replayGIFOut{}, errors.New("speed must be 0.5, 1, 2 or 4")
			}
			if err := checkNewLocalFile(in.Path, ".gif"); err != nil {
				return nil, replayGIFOut{}, err
			}
			c, err := s.client(ctx)
			if err != nil {
				return nil, replayGIFOut{}, toolError(err)
			}
			doc, err := c.LibraryTacticalPlan(ctx, in.MatchID)
			if err != nil {
				return nil, replayGIFOut{}, toolError(err)
			}
			round, err := c.LibraryTacticalRound(ctx, in.MatchID, in.Round)
			if err != nil {
				return nil, replayGIFOut{}, toolError(err)
			}
			rate := doc.Demo.Tickrate
			if rate <= 0 {
				rate = 64
			}
			from, to, err := gifTicks(in, round, rate)
			if err != nil {
				return nil, replayGIFOut{}, err
			}
			scene := replaygif.Scene{Calibration: doc.Geometry.Calibration, Tickrate: rate, Round: round.Round,
				Frames: round.Frames, FromTick: from, ToTick: to, Speed: in.Speed, Size: in.Size}
			// Without the game's radar the GIF still shows the players, on a plain plate.
			if raw, err := c.MapRadar(ctx, doc.Demo.Map); err == nil {
				if radar, err := png.Decode(bytes.NewReader(raw)); err == nil {
					scene.Radar = radar
				}
			}
			if !in.SkipDrawings {
				notes, err := c.LibraryRoundNotes(ctx, in.MatchID, in.Round)
				if err != nil {
					return nil, replayGIFOut{}, toolError(err)
				}
				scene.Notes = gifNotes(notes)
			}
			var buf bytes.Buffer
			got, err := replaygif.Encode(&buf, scene)
			if err != nil {
				return nil, replayGIFOut{}, err
			}
			if err := writeNewFile(in.Path, buf.Bytes()); err != nil {
				return nil, replayGIFOut{}, err
			}
			return nil, replayGIFOut{Path: in.Path, Bytes: buf.Len(), Frames: got.Frames, Seconds: got.Seconds,
				Radar: scene.Radar != nil, Drawings: drawnNotes(scene.Notes)}, nil
		})
}

func gifNotes(notes []tuiclient.RoundNote) []replaygif.Note {
	out := make([]replaygif.Note, 0, len(notes))
	for _, n := range notes {
		note := replaygif.Note{Tick: n.Tick}
		for _, st := range n.Strokes {
			note.Strokes = append(note.Strokes, replaygif.Stroke{Color: st.Color, Width: st.Width, Arrow: st.Arrow, Points: st.Points})
		}
		out = append(out, note)
	}
	return out
}

func drawnNotes(notes []replaygif.Note) int {
	n := 0
	for _, note := range notes {
		if len(note.Strokes) > 0 {
			n++
		}
	}
	return n
}
