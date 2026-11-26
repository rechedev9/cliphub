package main

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/rechedev9/cliphub/internal/overlaylang"
	"github.com/rechedev9/cliphub/internal/renderplan"
	"github.com/rechedev9/cliphub/internal/tuiclient"
)

type droppedOption struct {
	ID    string `json:"id"`
	Label string `json:"label"`
}

type horizontalDraftOut struct {
	GenerateShort     generateIn      `json:"generate_short" jsonschema:"the arguments for generate_short, which creates the horizontal as a new video of the same job after the user approves it"`
	Dropped           []droppedOption `json:"dropped,omitempty" jsonschema:"the Short's 9:16 options the horizontal leaves out; tell the user"`
	NeedsCapture      bool            `json:"needs_capture" jsonschema:"true when generate_short opens CS2 because some of the Short's recorded plays are no longer on disk"`
	MissingSegmentIDs []string        `json:"missing_segment_ids,omitempty" jsonschema:"the plays recorded again"`
	Summary           string          `json:"summary"`
}

// draftEdit reads the brand kit copies of the draft's edit as on or off; generate_short copies the saved kit again.
type draftEdit struct {
	tuiclient.ShortEdit
	BrandWatermark  json.RawMessage `json:"brand_watermark"`
	BrandBanner     json.RawMessage `json:"brand_banner"`
	SubscribeBanner json.RawMessage `json:"subscribe_banner"`
}

func registerHorizontalTool(server *mcp.Server, s *studio) {
	mcp.AddTool(server, &mcp.Tool{Name: "horizontal_draft", Annotations: &mcp.ToolAnnotations{ReadOnlyHint: true},
		Description: "Prepare the horizontal 16:9 version of a finished Short, as Crear horizontal does in Mis vídeos: the summary of the same plays in the same order, " +
			"with the Short's capture style, kill feed framing and pre-roll, its music and levels, its team comms, and its effects, transitions and texts. It creates nothing: " +
			"pass generate_short to generate_short, which needs the user's approval. needs_capture says whether that opens CS2; with the Short's style and plays " +
			"it renders from the Short's recorded plays. dropped lists the 9:16 options it leaves out. Changing capture_hud or adding plays records again. The Short stays as it is."},
		func(ctx context.Context, _ *mcp.CallToolRequest, in videoVersionsIn) (*mcp.CallToolResult, horizontalDraftOut, error) {
			c, err := versionClient(ctx, s, in)
			if err != nil {
				return nil, horizontalDraftOut{}, err
			}
			draft, err := c.HorizontalDraft(ctx, in.JobID, in.Variant)
			if err != nil {
				return nil, horizontalDraftOut{}, toolError(err)
			}
			return nil, describeHorizontalDraft(in.JobID, draft), nil
		})
}

func describeHorizontalDraft(jobID string, d tuiclient.HorizontalDraft) horizontalDraftOut {
	args := horizontalArguments(jobID, d)
	out := horizontalDraftOut{GenerateShort: args, NeedsCapture: d.Capture.Needed, MissingSegmentIDs: d.Capture.MissingSegmentIDs}
	labels := make([]string, 0, len(d.Dropped))
	for _, id := range d.Dropped {
		label := cleanName(id)
		if id == renderplan.DroppedPlayerCards {
			label = "Cartas de jugador"
		}
		for _, t := range toggleOptions {
			if t.ID == id {
				label = t.Label
			}
		}
		out.Dropped = append(out.Dropped, droppedOption{ID: cleanName(id), Label: label})
		labels = append(labels, label)
	}
	style := "HUD completo"
	for _, o := range captureHUDOptions {
		if o.ID == args.CaptureHUD {
			style = o.Label
		}
	}
	summary := fmt.Sprintf("Horizontal del Short con sus %s en el mismo orden, captura %s", plural(len(args.SegmentIDs), "jugada", "jugadas"), style)
	if args.Music != "" {
		summary += ", música " + cleanName(args.Music)
	}
	summary += "."
	if len(labels) > 0 {
		summary += " No lleva: " + strings.Join(labels, ", ") + "."
	}
	if d.Capture.Needed {
		summary += fmt.Sprintf(" Las grabaciones de %s ya no están: generate_short abrirá CS2 para grabarlas.", plural(len(d.Capture.MissingSegmentIDs), "jugada", "jugadas"))
	} else {
		summary += " Se monta con las jugadas ya grabadas, sin abrir CS2."
	}
	out.Summary = summary
	return out
}

// horizontalArguments turns the draft into generate_short arguments that send the same edit and music.
func horizontalArguments(jobID string, d tuiclient.HorizontalDraft) generateIn {
	var e draftEdit
	_ = json.Unmarshal(d.Edit, &e)
	c := shortChoices{Format: e.Format, KillEffect: e.KillEffect, Transition: e.Transition, IntroText: e.IntroText, OutroText: e.OutroText, CaptureHUD: e.CaptureHUD}
	for _, f := range []struct {
		on  bool
		dst **bool
	}{{e.Intro, &c.Intro}, {e.Outro, &c.Outro}, {e.HookText, &c.HookText}, {e.KillCounter, &c.KillCounter}, {e.VoiceComms, &c.VoiceComms}, {e.NativeHUD, &c.NativeHUD},
		{e.VictimReplay, &c.VictimReplay}, {e.PlayBadges, &c.PlayBadges}, {e.WinProbability, &c.WinProbability}, {e.PlayerLabels, &c.PlayerLabels},
		{e.GrenadeTrails, &c.GrenadeTrails}, {e.PortraitSafeKillfeed, &c.PortraitSafeKillfeed}, {e.Subtitles, &c.Subtitles}, {kitCopied(e.BrandWatermark), &c.BrandWatermark}, {kitCopied(e.BrandBanner), &c.BrandBanner}, {kitCopied(e.SubscribeBanner), &c.SubscribeBanner}} {
		if f.on {
			*f.dst = new(true)
		}
	}
	if e.CoverStrategy == coverNone {
		c.Cover = new(false)
	}
	c.SubtitlesHidden = e.SubtitlesHidden
	if e.OverlayLanguage != overlaylang.Spanish {
		c.OverlayLanguage = e.OverlayLanguage.String()
	}
	if e.VictimReplaySpeed != 0 {
		c.VictimReplaySpeed = new(e.VictimReplaySpeed)
	}
	if e.PreRollSeconds != 0 {
		c.PreRollSeconds = new(e.PreRollSeconds)
	}
	if e.KeyDropStyle != "" {
		c.BannerFamily, c.BannerStyle, c.BannerCode, c.BannerStart, c.BannerEnd = e.KeyDropFamily, e.KeyDropStyle, e.KeyDropCode, e.KeyDropStartSeconds, e.KeyDropEndSeconds
	}
	if m := d.Music; m != nil && m.Key != "" {
		c.Music = m.Key
		if m.Volume > 0 && m.Volume < 1 {
			c.MusicVolume = new(int(math.Round(m.Volume * 100)))
		}
		if m.GameVolume != nil {
			c.GameVolume = new(int(math.Round(*m.GameVolume * 100)))
		}
	}
	return generateIn{JobID: jobID, SegmentIDs: d.SegmentIDs, Preset: d.Preset, shortChoices: c}
}

func kitCopied(raw json.RawMessage) bool {
	s := strings.TrimSpace(string(raw))
	return s != "" && s != "null" && s != "false"
}
