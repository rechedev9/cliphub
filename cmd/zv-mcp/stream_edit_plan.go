package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/rechedev9/cliphub/internal/tuiclient"
)

type cropOut struct {
	X      float64 `json:"x"`
	Y      float64 `json:"y"`
	Width  float64 `json:"width"`
	Height float64 `json:"height"`
}

type textOverlayArg struct {
	Text         string   `json:"text" jsonschema:"one line burned into the video, at most 120 characters"`
	PositionY    float64  `json:"position_y" jsonschema:"vertical center of the line, from 0.025 (top) to 0.975 (bottom)"`
	StartSeconds *float64 `json:"start_seconds,omitempty" jsonschema:"seconds from the clip start when the line appears; empty is the clip start"`
	EndSeconds   *float64 `json:"end_seconds,omitempty" jsonschema:"seconds from the clip start when the line goes; empty is the clip end"`
	FontSize     int      `json:"font_size,omitempty" jsonschema:"24 to 120 output pixels; empty is 64"`
}

type clipEditArg struct {
	Speed          float64          `json:"speed,omitempty" jsonschema:"playback speed from 0.25 to 3; empty is normal speed"`
	SourceVolume   *float64         `json:"source_volume,omitempty" jsonschema:"gain of the stream's own audio from 0 (muted) to 2; empty is unchanged"`
	FadeInSeconds  float64          `json:"fade_in_seconds,omitempty" jsonschema:"0 to 5 seconds of fade from black at the start"`
	FadeOutSeconds float64          `json:"fade_out_seconds,omitempty" jsonschema:"0 to 5 seconds of fade to black at the end"`
	TextOverlays   []textOverlayArg `json:"text_overlays,omitempty" jsonschema:"at most 4 text lines"`
}

type planClipArg struct {
	ID           string       `json:"id,omitempty" jsonschema:"keep the id from get_stream_edit_plan for an existing moment; empty makes a new one"`
	StartSeconds float64      `json:"start_seconds" jsonschema:"start in seconds of the source video"`
	EndSeconds   float64      `json:"end_seconds" jsonschema:"end in seconds of the source video"`
	Title        string       `json:"title,omitempty"`
	Edit         *clipEditArg `json:"edit,omitempty" jsonschema:"speed, audio, fades and text of this moment; leave it out to render the moment untouched"`
}

type streamerBannerArg struct {
	Nick         string   `json:"nick,omitempty" jsonschema:"streamer name, 1 to 25 letters, numbers or underscores; empty hides the banner"`
	Platform     string   `json:"platform,omitempty" jsonschema:"twitch or kick; empty is twitch"`
	PositionY    *float64 `json:"position_y,omitempty" jsonschema:"vertical center from 0.025 to 0.975; empty is the default place"`
	SlideEnabled bool     `json:"slide_enabled,omitempty" jsonschema:"slide the banner in instead of showing it at once"`
}

type keyDropBannerArg struct {
	Family       string   `json:"family,omitempty" jsonschema:"KEYDROP or CSGOSKINS; empty is KEYDROP"`
	Style        string   `json:"style,omitempty" jsonschema:"plate of the family: operator, classic, tigerr or jcorko for KEYDROP; empty hides the banner"`
	Code         string   `json:"code,omitempty" jsonschema:"affiliate code, 1 to 16 letters, numbers, - or _"`
	PositionY    *float64 `json:"position_y,omitempty" jsonschema:"vertical center from 0.025 to 0.975"`
	SlideEnabled bool     `json:"slide_enabled,omitempty"`
	StartSeconds *float64 `json:"start_seconds,omitempty" jsonschema:"seconds from each moment's start when the plate appears; empty is the start"`
	EndSeconds   *float64 `json:"end_seconds,omitempty" jsonschema:"seconds from each moment's start when the plate goes; empty is the end"`
}

type streamMusicArg struct {
	Key    string  `json:"key,omitempty" jsonschema:"music track id from list_short_options; empty means no music"`
	Volume float64 `json:"volume,omitempty" jsonschema:"music gain above 0 and up to 1, under the stream audio; empty is 0.25"`
}

type killFeedArg struct {
	Centered  bool     `json:"centered,omitempty" jsonschema:"redraw the streamer's kill feed centered on the gameplay of a vertical Short"`
	PositionY *float64 `json:"position_y,omitempty" jsonschema:"top of the newest notice from 0.025 to 0.975; empty sits it under the players HUD"`
}

type effectsArg struct {
	Grade bool `json:"grade,omitempty" jsonschema:"mild contrast and saturation lift"`
}

type streamPlanOut struct {
	StreamID         string            `json:"stream_id"`
	UpdatedAt        string            `json:"updated_at" jsonschema:"pass as expected_updated_at to edit_stream_plan"`
	Variant          string            `json:"variant" jsonschema:"layout from list_stream_variants that render_stream uses when given none"`
	SourceSeconds    float64           `json:"source_seconds,omitempty" jsonschema:"length of the source video; moments must end before it"`
	FaceCrop         *cropOut          `json:"face_crop,omitempty" jsonschema:"facecam area of the source, 0 to 1 of the frame"`
	FaceCropReviewed bool              `json:"face_crop_reviewed" jsonschema:"true once the user checked the facecam crop in Studio; facecam layouts need it"`
	GameplayCrop     *cropOut          `json:"gameplay_crop,omitempty" jsonschema:"gameplay area of the source, 0 to 1 of the frame"`
	Clips            []planClipArg     `json:"clips" jsonschema:"the moments, one video each, in render order"`
	StreamerBanner   streamerBannerArg `json:"streamer_banner"`
	KeyDropBanner    keyDropBannerArg  `json:"keydrop_banner"`
	Music            streamMusicArg    `json:"music"`
	KillFeed         killFeedArg       `json:"kill_feed"`
	Effects          effectsArg        `json:"effects"`
}

type editStreamPlanIn struct {
	ID                string             `json:"id" jsonschema:"stream_id"`
	ExpectedUpdatedAt string             `json:"expected_updated_at" jsonschema:"updated_at from get_stream_edit_plan; the save fails when the plan changed after that"`
	Clips             *[]planClipArg     `json:"clips,omitempty" jsonschema:"the full new list of moments in render order; it replaces the saved list, so send back the moments and edits to keep. Leave it out to keep the moments"`
	StreamerBanner    *streamerBannerArg `json:"streamer_banner,omitempty" jsonschema:"replaces the streamer banner; {} hides it; leave it out to keep it"`
	KeyDropBanner     *keyDropBannerArg  `json:"keydrop_banner,omitempty" jsonschema:"replaces the affiliate banner; {} hides it; leave it out to keep it"`
	Music             *streamMusicArg    `json:"music,omitempty" jsonschema:"replaces the music; {} removes it; leave it out to keep it"`
	KillFeed          *killFeedArg       `json:"kill_feed,omitempty" jsonschema:"replaces the kill feed setting; leave it out to keep it"`
	Effects           *effectsArg        `json:"effects,omitempty" jsonschema:"replaces the effects; leave it out to keep them"`
}

func registerStreamPlanTools(server *mcp.Server, s *studio) {
	mcp.AddTool(server, &mcp.Tool{Name: "get_stream_edit_plan", Annotations: &mcp.ToolAnnotations{ReadOnlyHint: true},
		Description: "Read a stream's edit plan, the stream editor in Studio: the moments (each with its speed, audio, fades and text), " +
			"the streamer and affiliate banners, music, kill feed and effects, the layout, and the facecam and gameplay crops with whether the user checked the facecam. " +
			"Pass its updated_at to edit_stream_plan."},
		func(ctx context.Context, _ *mcp.CallToolRequest, in idIn) (*mcp.CallToolResult, streamPlanOut, error) {
			if err := checkUUID("id", in.ID); err != nil {
				return nil, streamPlanOut{}, err
			}
			c, err := s.client(ctx)
			if err != nil {
				return nil, streamPlanOut{}, toolError(err)
			}
			j, err := c.GetStreamJob(ctx, in.ID)
			if err != nil {
				return nil, streamPlanOut{}, toolError(err)
			}
			doc, err := c.GetStreamEditPlanDocument(ctx, in.ID)
			if err != nil {
				return nil, streamPlanOut{}, toolError(err)
			}
			out, err := describeStreamPlan(in.ID, doc)
			out.SourceSeconds = j.Probe.DurationSeconds
			return nil, out, err
		})

	mcp.AddTool(server, &mcp.Tool{Name: "edit_stream_plan",
		Description: "Change a stream's edit plan without rendering, as the stream editor in Studio does: add, remove, trim, retitle or reorder the moments " +
			"and set each moment's speed, audio, fades and text; set the streamer banner, the affiliate banner, the music, the kill feed and the effects. " +
			"Each field given replaces that part of the plan; fields left out stay as saved. Call get_stream_edit_plan first and pass its updated_at as expected_updated_at: " +
			"the save fails if the plan changed since, so read it again and redo the change. render_stream then renders the saved plan. Returns the saved plan."},
		func(ctx context.Context, _ *mcp.CallToolRequest, in editStreamPlanIn) (*mcp.CallToolResult, streamPlanOut, error) {
			if err := checkUUID("id", in.ID); err != nil {
				return nil, streamPlanOut{}, err
			}
			if _, err := planTime(in.ExpectedUpdatedAt); err != nil {
				return nil, streamPlanOut{}, errors.New("expected_updated_at must be the updated_at get_stream_edit_plan returned")
			}
			if in.Clips == nil && in.StreamerBanner == nil && in.KeyDropBanner == nil && in.Music == nil && in.KillFeed == nil && in.Effects == nil {
				return nil, streamPlanOut{}, errors.New("nothing to change: set clips, streamer_banner, keydrop_banner, music, kill_feed or effects")
			}
			c, err := s.client(ctx)
			if err != nil {
				return nil, streamPlanOut{}, toolError(err)
			}
			if in.Music != nil && in.Music.Key != "" {
				songs, err := c.ListSongs(ctx)
				if err != nil {
					return nil, streamPlanOut{}, toolError(err)
				}
				if err := checkMusic(in.Music.Key, songs); err != nil {
					return nil, streamPlanOut{}, err
				}
			}
			doc, err := c.GetStreamEditPlanDocument(ctx, in.ID)
			if err != nil {
				return nil, streamPlanOut{}, toolError(err)
			}
			next, err := applyStreamPlanEdit(doc, in)
			if err != nil {
				return nil, streamPlanOut{}, err
			}
			saved, err := c.PutStreamEditPlanDocument(ctx, tuiclient.StreamPlanPut{ID: in.ID, Plan: next, ExpectedUpdatedAt: in.ExpectedUpdatedAt})
			if err != nil {
				return nil, streamPlanOut{}, toolError(err)
			}
			out, err := describeStreamPlan(in.ID, saved)
			return nil, out, err
		})
}

// applyStreamPlanEdit replaces the fields the edit sets and keeps every other field of doc.
func applyStreamPlanEdit(doc map[string]json.RawMessage, in editStreamPlanIn) (map[string]json.RawMessage, error) {
	next := make(map[string]json.RawMessage, len(doc)+1)
	for k, v := range doc {
		next[k] = v
	}
	set := func(key string, v any) error {
		raw, err := json.Marshal(v)
		if err != nil {
			return err
		}
		next[key] = raw
		return nil
	}
	if in.Clips != nil {
		clips, err := withClipIDs(*in.Clips)
		if err != nil {
			return nil, err
		}
		if err := set("clips", clips); err != nil {
			return nil, err
		}
	}
	parts := []struct {
		key string
		set bool
		v   any
	}{
		{"streamer_banner", in.StreamerBanner != nil, in.StreamerBanner}, {"keydrop_banner", in.KeyDropBanner != nil, in.KeyDropBanner},
		{"music", in.Music != nil, in.Music}, {"kill_feed", in.KillFeed != nil, in.KillFeed}, {"effects", in.Effects != nil, in.Effects},
	}
	for _, part := range parts {
		if !part.set {
			continue
		}
		if err := set(part.key, part.v); err != nil {
			return nil, err
		}
	}
	return next, nil
}

// withClipIDs names the new moments clip-001, clip-002... past the ids already taken.
func withClipIDs(clips []planClipArg) ([]planClipArg, error) {
	if len(clips) > maxSegmentIDs {
		return nil, fmt.Errorf("clips takes at most %d moments", maxSegmentIDs)
	}
	out := slices.Clone(clips)
	taken := make(map[string]bool, len(out))
	for _, c := range out {
		if c.ID == "" {
			continue
		}
		if taken[c.ID] {
			return nil, fmt.Errorf("clip id %s is listed twice", cleanName(c.ID))
		}
		taken[c.ID] = true
	}
	n := 1
	for i := range out {
		if len([]rune(out[i].Title)) > maxTitleRunes {
			return nil, fmt.Errorf("a clip title takes at most %d characters", maxTitleRunes)
		}
		if out[i].ID != "" {
			continue
		}
		for taken[fmt.Sprintf("clip-%03d", n)] {
			n++
		}
		out[i].ID = fmt.Sprintf("clip-%03d", n)
		taken[out[i].ID] = true
	}
	return out, nil
}

// describeStreamPlan reads the plan Studio saved; names and text in it are untrusted.
func describeStreamPlan(id string, doc map[string]json.RawMessage) (streamPlanOut, error) {
	out := streamPlanOut{StreamID: id, Clips: []planClipArg{}}
	var updatedAt string
	fields := map[string]any{"updated_at": &updatedAt, "variant": &out.Variant, "face_crop": &out.FaceCrop,
		"face_crop_reviewed": &out.FaceCropReviewed, "gameplay_crop": &out.GameplayCrop, "clips": &out.Clips,
		"streamer_banner": &out.StreamerBanner, "keydrop_banner": &out.KeyDropBanner, "music": &out.Music,
		"kill_feed": &out.KillFeed, "effects": &out.Effects}
	for key, dst := range fields {
		if raw, ok := doc[key]; ok && string(raw) != "null" {
			if err := json.Unmarshal(raw, dst); err != nil {
				return streamPlanOut{}, fmt.Errorf("Studio returned an edit plan with an invalid %s", key)
			}
		}
	}
	var err error
	if out.UpdatedAt, err = planTime(updatedAt); err != nil {
		return streamPlanOut{}, err
	}
	out.Variant = cleanName(out.Variant)
	for i := range out.Clips {
		clip := &out.Clips[i]
		clip.ID, clip.Title = cleanName(clip.ID), cleanName(clip.Title)
		if clip.Edit != nil {
			for j := range clip.Edit.TextOverlays {
				clip.Edit.TextOverlays[j].Text = cleanText(clip.Edit.TextOverlays[j].Text, maxErrorRunes)
			}
		}
	}
	out.StreamerBanner.Nick, out.StreamerBanner.Platform = cleanName(out.StreamerBanner.Nick), cleanName(out.StreamerBanner.Platform)
	out.KeyDropBanner.Family, out.KeyDropBanner.Style = cleanName(out.KeyDropBanner.Family), cleanName(out.KeyDropBanner.Style)
	out.KeyDropBanner.Code, out.Music.Key = cleanName(out.KeyDropBanner.Code), cleanName(out.Music.Key)
	return out, nil
}
