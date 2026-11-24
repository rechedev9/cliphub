package editor

import (
	"context"
	"fmt"
	"os"

	"github.com/rechedev9/cliphub/internal/customhud"
	"github.com/rechedev9/cliphub/internal/mediafont"
	"github.com/rechedev9/cliphub/internal/pathguard"
	"github.com/rechedev9/cliphub/internal/radarmap"
	"github.com/rechedev9/cliphub/internal/radaroverlay"
	"github.com/rechedev9/cliphub/internal/recapplan"
)

type FullDemoLocalRadar struct {
	Rounds map[string]FullDemoLocalHUD `json:"rounds"`
	Upper  FullDemoLocalHUD            `json:"upper"`
	Lower  *FullDemoLocalHUD           `json:"lower,omitempty"`
}

type fullDemoRadar struct {
	rounds       map[string]radaroverlay.Track
	upper, lower []byte
}

func verifyFullDemoRadar(ctx context.Context, execution FullDemoExecution) ([]pathguard.Input, error) {
	d := execution.Approved.Document
	if !d.Options.Capture.ClipHubRadar {
		if execution.Radar != nil {
			return nil, fmt.Errorf("full demo without ClipHub radar must not carry radar tracks")
		}
		return nil, nil
	}
	r := execution.Radar
	if r == nil {
		return nil, fmt.Errorf("ClipHub radar requires materialized radar tracks")
	}
	files := []FullDemoLocalHUD{r.Upper}
	if r.Lower != nil {
		files = append(files, *r.Lower)
	}
	rounds := map[string]bool{}
	for _, item := range d.Timeline {
		if item.Role == "round" {
			rounds[item.SourceRef] = true
		}
	}
	if len(rounds) != len(r.Rounds) {
		return nil, fmt.Errorf("ClipHub radar tracks differ from the approved rounds")
	}
	for round := range rounds {
		track, ok := r.Rounds[round]
		if !ok {
			return nil, fmt.Errorf("ClipHub radar has no track for %s", round)
		}
		files = append(files, track)
	}
	inputs := make([]pathguard.Input, 0, len(files))
	for _, file := range files {
		if err := verifyFullDemoLocalFile(ctx, file.Path, file.SHA256); err != nil {
			return nil, err
		}
		inputs = append(inputs, pathguard.Input{Flag: "Full Demo radar", Path: file.Path})
	}
	return inputs, nil
}

func loadFullDemoRadar(local FullDemoLocalRadar, d recapplan.Document) (*fullDemoRadar, error) {
	out := &fullDemoRadar{rounds: map[string]radaroverlay.Track{}}
	for round, file := range local.Rounds {
		track, err := radaroverlay.Load(file.Path)
		if err != nil {
			return nil, err
		}
		if track.DemoSHA256 != d.Input.DemoSHA256 || track.TickRate != d.Clock.TickRate || len(track.Segments) != 1 || track.Segments[0].ID != round {
			return nil, fmt.Errorf("ClipHub radar track of %s differs from the approved demo", round)
		}
		out.rounds[round] = track
	}
	art, err := os.ReadFile(local.Upper.Path)
	if err != nil {
		return nil, err
	}
	out.upper = art
	if local.Lower != nil {
		if out.lower, err = os.ReadFile(local.Lower.Path); err != nil {
			return nil, err
		}
	}
	return out, nil
}

type fullDemoRadarStep struct {
	short   ShortEdit
	item    recapplan.TimelineItem
	command []string
	video   string
	output  string
}

func fullDemoRadarOverlay(s fullDemoRadarStep) ([]string, string, error) {
	if s.item.Role != "round" || !s.short.FullDemo.Effective.Options.Capture.ClipHubRadar {
		return s.command, s.video, nil
	}
	if s.short.fullDemo == nil || s.short.fullDemo.radar == nil {
		return nil, "", fmt.Errorf("ClipHub radar tracks are missing")
	}
	radar := s.short.fullDemo.radar
	track, ok := radar.rounds[s.item.SourceRef]
	if !ok {
		return nil, "", fmt.Errorf("ClipHub radar has no track for %s", s.item.SourceRef)
	}
	segment := track.Segments[0]
	level := segment.POVLevel(track.Calibration)
	box, err := fullDemoRadarBox(s.short)
	if err != nil {
		return nil, "", err
	}
	opacity := radaroverlay.EffectiveOpacity(0)
	span := radaroverlay.Span{
		Segment: segment, StartTick: s.item.SourceStartTick, Offset: -float64(s.item.SourceOffsetFrames) / recapplan.OutputFPS,
		Until: float64(s.item.EndFrame-s.item.StartFrame) / recapplan.OutputFPS, Level: level,
	}
	script, err := radaroverlay.Script(radaroverlay.Layout{
		Width: customhud.Width, Height: customhud.Height, TickRate: track.TickRate, Calibration: track.Calibration, Box: box, Opacity: opacity,
	}, []radaroverlay.Span{span})
	if err != nil {
		return nil, "", err
	}
	art := radar.upper
	if level == radarmap.LevelLower && len(radar.lower) > 0 {
		art = radar.lower
	}
	plate := s.output + ".radar.png"
	if err := radaroverlay.WritePlate(plate, art, box.W, opacity); err != nil {
		return nil, "", fmt.Errorf("radar plate: %w", err)
	}
	index := fullDemoInputCount(s.command)
	command := append(s.command, "-i", plate)
	video := s.video + fmt.Sprintf("[radarbase];[%d:v]format=rgba,setsar=1[radarplate];[radarbase][radarplate]overlay=x=%d:y=%d:eof_action=repeat:repeatlast=1:format=auto,format=yuv420p", index, box.X, box.Y)
	if script == "" {
		return command, video, nil
	}
	path := s.output + ".radar.ass"
	if err := os.WriteFile(path, []byte(script), 0o600); err != nil {
		return nil, "", err
	}
	fonts, err := mediafont.MaterializeHUD()
	if err != nil {
		return nil, "", fmt.Errorf("radar fonts: %w", err)
	}
	return command, video + "," + customhud.ASSFilter(path, fonts, false), nil
}

func fullDemoRadarBox(short ShortEdit) (radaroverlay.Rect, error) {
	place := radaroverlay.Placement{Position: radaroverlay.PositionTopLeft, Size: radaroverlay.SizeSmall, Width: customhud.Width, Height: customhud.Height}
	if wm := short.FullDemo.Effective.Options.Overlays.Brand.Watermark; wm != nil {
		path, err := short.fullDemo.execution.assetPath(recapplan.AssetRef{ID: wm.Image.ID, SHA256: wm.Image.SHA256})
		if err != nil {
			return radaroverlay.Rect{}, err
		}
		width, height, err := brandImageSize(path)
		if err != nil {
			return radaroverlay.Rect{}, err
		}
		w, h, x, y := watermarkPlacement(wm.Corner, width, height, customhud.Width, customhud.Height)
		place.Avoid = []radaroverlay.Rect{{X: x, Y: y, W: w, H: h}}
	}
	return radaroverlay.Place(place), nil
}
