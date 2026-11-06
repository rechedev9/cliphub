package main

import (
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"regexp"
	"slices"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/rechedev9/cliphub/internal/tuiclient"
)

// Play scan and summary states (internal/httpapi stream_plays.go).
const (
	playScanIdle    = "idle"
	playScanRunning = "running"
	playScanDone    = "done"
	playScanFailed  = "failed"
)

const defaultStreamPlaysLimit = 20

// Plays are numbered "play-001" in time order (internal/vodplays).
var streamPlayIDPattern = regexp.MustCompile(`^play-[0-9]{1,6}$`)

type findStreamPlaysIn struct {
	ID          string `json:"id" jsonschema:"stream_id from import_stream or list_streams"`
	Limit       int    `json:"limit,omitempty" jsonschema:"how many plays to return, best first; default 20"`
	Rescan      bool   `json:"rescan,omitempty" jsonschema:"scan the video again even if it was scanned before"`
	WaitSeconds int    `json:"wait_seconds,omitempty" jsonschema:"how long to wait for the scan, at most 55; call again while status is running"`
}

type streamPlayOut struct {
	PlayID       string  `json:"play_id" jsonschema:"value for render_stream play_ids"`
	Label        string  `json:"label"`
	Kills        int     `json:"kills"`
	Score        int     `json:"score" jsonschema:"0 to 100, from the number of kills and how fast they came"`
	StartSeconds float64 `json:"start_seconds"`
	EndSeconds   float64 `json:"end_seconds"`
}

type streamPlaysOut struct {
	Status   string          `json:"status" jsonschema:"idle, running, done or failed"`
	Percent  int             `json:"percent,omitempty"`
	Plays    []streamPlayOut `json:"plays"`
	Total    int             `json:"total" jsonschema:"plays found; raise limit to see more"`
	Error    string          `json:"error,omitempty"`
	Note     string          `json:"note,omitempty"`
	Duration float64         `json:"plays_seconds,omitempty" jsonschema:"length of every play together, which is about the length of the 16:9 summary"`
}

type renderStreamSummaryIn struct {
	ID      string `json:"id" jsonschema:"stream_id whose plays find_stream_plays found"`
	Approve string `json:"approve,omitempty" jsonschema:"only when asked: the approval code from a previous call, after the user agreed"`
}

type renderStreamIn struct {
	ID      string   `json:"id" jsonschema:"stream_id"`
	Variant string   `json:"variant,omitempty" jsonschema:"layout name from list_stream_variants; empty keeps the stream's current layout"`
	PlayIDs []string `json:"play_ids,omitempty" jsonschema:"plays from find_stream_plays to add to the moments saved in Studio, one video each; a play that overlaps a saved moment is skipped. Empty renders the saved moments"`
	Approve string   `json:"approve,omitempty" jsonschema:"only when asked: the approval code from a previous call, after the user agreed"`
}

type streamRenderStartOut struct {
	NeedsApproval bool   `json:"needs_approval,omitempty"`
	ApprovalCode  string `json:"approval_code,omitempty"`
	Summary       string `json:"summary"`
	StreamID      string `json:"stream_id,omitempty"`
	Status        string `json:"status,omitempty"`
	Variant       string `json:"variant,omitempty" jsonschema:"pass to wait_for_stream_render"`
}

type waitStreamRenderIn struct {
	ID          string `json:"id" jsonschema:"stream_id"`
	Variant     string `json:"variant" jsonschema:"the variant render_stream or render_stream_summary returned; summary for the 16:9 summary"`
	WaitSeconds int    `json:"wait_seconds,omitempty" jsonschema:"how long to wait, at most 55; call again while running is true"`
}

type streamRenderOut struct {
	StreamID string   `json:"stream_id"`
	Variant  string   `json:"variant"`
	Status   string   `json:"status"`
	Running  bool     `json:"running"`
	Percent  int      `json:"percent,omitempty"`
	Progress string   `json:"progress,omitempty" jsonschema:"while rendering: the video being encoded, such as video 2 of 3"`
	Error    string   `json:"error,omitempty" jsonschema:"untrusted text"`
	Warnings []string `json:"warnings,omitempty"`
	Note     string   `json:"note,omitempty"`
}

// planClip is one moment of a stream edit plan (internal/streamclips.ClipRange).
type planClip struct {
	ID           string  `json:"id"`
	StartSeconds float64 `json:"start_seconds"`
	EndSeconds   float64 `json:"end_seconds"`
	Title        string  `json:"title,omitempty"`
}

// planMoment is a saved clip kept as Studio sent it, so fields this server
// does not model (the clip's edit) survive a save.
type planMoment struct {
	clip planClip
	raw  json.RawMessage
}

// streamRenderRequest is what the user approves for render_stream.
type streamRenderRequest struct {
	Variant       string     `json:"variant"`
	Clips         []planClip `json:"clips,omitempty"`
	PlanUpdatedAt string     `json:"plan_updated_at"`
}

func registerStreamRenderTools(server *mcp.Server, s *studio) {
	mcp.AddTool(server, &mcp.Tool{Name: "find_stream_plays",
		Description: "Find the streamer's kills in a ready stream (the kill feed highlight of CS2) and list each run of kills as a play, best first: the Jugadas list of a stream in Studio. " +
			"The first call starts the scan; call again while status is running."},
		func(ctx context.Context, req *mcp.CallToolRequest, in findStreamPlaysIn) (*mcp.CallToolResult, streamPlaysOut, error) {
			if err := checkUUID("id", in.ID); err != nil {
				return nil, streamPlaysOut{}, err
			}
			c, err := s.client(ctx)
			if err != nil {
				return nil, streamPlaysOut{}, toolError(err)
			}
			scan, err := c.GetStreamPlays(ctx, in.ID)
			if err != nil {
				return nil, streamPlaysOut{}, toolError(err)
			}
			if scan.Status == playScanIdle || (in.Rescan && scan.Status != playScanRunning) {
				if scan, err = c.StartStreamPlayScan(ctx, in.ID); err != nil {
					return nil, streamPlaysOut{}, toolError(err)
				}
			}
			err = poll(ctx, in.WaitSeconds, func() (bool, error) {
				if scan.Status != playScanRunning {
					return true, nil
				}
				notify(ctx, req, int(scan.Progress*100), "buscando jugadas")
				next, err := c.GetStreamPlays(ctx, in.ID)
				scan = next
				return next.Status != playScanRunning, err
			})
			if err != nil {
				return nil, streamPlaysOut{}, toolError(err)
			}
			return nil, describePlayScan(scan, in.Limit), nil
		})

	mcp.AddTool(server, &mcp.Tool{Name: "render_stream_summary", Meta: appToolMeta(jobProgressAppURI),
		Description: "Render one 16:9 video with every play find_stream_plays found, in order: the Resumen horizontal of a VOD in Studio. " +
			"It encodes for minutes, so the user must approve it. If the result says needs_approval, show the summary, ask the user, and call again with approve set to approval_code only if they agree. " +
			"Follow with wait_for_stream_render and variant summary."},
		func(ctx context.Context, req *mcp.CallToolRequest, in renderStreamSummaryIn) (*mcp.CallToolResult, streamRenderStartOut, error) {
			if err := checkUUID("id", in.ID); err != nil {
				return nil, streamRenderStartOut{}, err
			}
			c, err := s.client(ctx)
			if err != nil {
				return nil, streamRenderStartOut{}, toolError(err)
			}
			j, err := c.GetStreamJob(ctx, in.ID)
			if err != nil {
				return nil, streamRenderStartOut{}, toolError(err)
			}
			scan, err := c.GetStreamPlays(ctx, in.ID)
			if err != nil {
				return nil, streamRenderStartOut{}, toolError(err)
			}
			if scan.Status != playScanDone || len(scan.Plays) == 0 {
				return nil, streamRenderStartOut{}, errors.New("the stream has no plays yet; run find_stream_plays until it reports done with plays")
			}
			ids := make([]string, 0, len(scan.Plays))
			for _, p := range scan.Plays {
				ids = append(ids, p.ID)
			}
			summary := fmt.Sprintf("Crear el resumen horizontal 16:9 de %s con %s seguidas, unos %s. FFmpeg trabajará durante varios minutos.",
				streamName(j), plural(len(scan.Plays), "jugada", "jugadas"), clock(playsSeconds(scan.Plays)))
			ok, err := sessionApprovals.approve(req, approvalSpec{Tool: "render_stream_summary", JobID: in.ID, Request: ids, Summary: summary, Given: in.Approve})
			if err != nil {
				return nil, streamRenderStartOut{}, err
			}
			if ok.Ask != nil {
				return ok.Ask, streamRenderStartOut{}, nil
			}
			if !ok.Approved {
				return nil, streamRenderStartOut{NeedsApproval: true, ApprovalCode: ok.Code, Summary: summary}, nil
			}
			res, err := c.StartStreamSummary(ctx, in.ID)
			if err != nil {
				return nil, streamRenderStartOut{}, toolError(err)
			}
			return nil, streamRenderStartOut{Summary: summary + " Started; follow with wait_for_stream_render and variant summary.",
				StreamID: in.ID, Status: cleanName(res.Status), Variant: summaryVariant}, nil
		})

	mcp.AddTool(server, &mcp.Tool{Name: "render_stream", Meta: appToolMeta(jobProgressAppURI),
		Description: "Render a stream's moments as videos in one layout from list_stream_variants, one MP4 per moment: vertical 9:16 Shorts or 16:9. " +
			"play_ids from find_stream_plays are added to the moments the user saved in Studio, as the Añadir button does: a play that overlaps a saved moment is skipped. Empty renders the saved moments. " +
			"A facecam layout needs the facecam crop checked once in Studio first. It encodes for minutes, so the user must approve it: " +
			"if the result says needs_approval, show the summary, ask the user, and call again with approve set to approval_code only if they agree. " +
			"Follow with wait_for_stream_render, passing the returned variant."},
		func(ctx context.Context, req *mcp.CallToolRequest, in renderStreamIn) (*mcp.CallToolResult, streamRenderStartOut, error) {
			if err := checkUUID("id", in.ID); err != nil {
				return nil, streamRenderStartOut{}, err
			}
			if in.Variant != "" {
				if err := checkArtifactName("variant", in.Variant); err != nil {
					return nil, streamRenderStartOut{}, err
				}
			}
			if err := checkStreamPlayIDs(in.PlayIDs); err != nil {
				return nil, streamRenderStartOut{}, err
			}
			c, err := s.client(ctx)
			if err != nil {
				return nil, streamRenderStartOut{}, toolError(err)
			}
			j, err := c.GetStreamJob(ctx, in.ID)
			if err != nil {
				return nil, streamRenderStartOut{}, toolError(err)
			}
			uploaded := j.Status == tuiclient.StreamUploaded
			if !uploaded && j.Status != tuiclient.StreamReady && j.Status != tuiclient.StreamRendered {
				return nil, streamRenderStartOut{}, fmt.Errorf("the stream is %s; it renders once it is ready (wait_for_stream) and no other render is running", cleanName(j.Status))
			}
			doc, err := c.GetStreamEditPlanDocument(ctx, in.ID)
			if err != nil {
				return nil, streamRenderStartOut{}, toolError(err)
			}
			variants, err := c.ListStreamVariants(ctx)
			if err != nil {
				return nil, streamRenderStartOut{}, toolError(err)
			}
			body, err := streamRenderPlan(doc, in.Variant, variants)
			if err != nil {
				return nil, streamRenderStartOut{}, err
			}
			moments, err := planMoments(doc)
			if err != nil {
				return nil, streamRenderStartOut{}, err
			}
			saved, added := len(moments), 0
			if len(in.PlayIDs) > 0 {
				scan, err := c.GetStreamPlays(ctx, in.ID)
				if err != nil {
					return nil, streamRenderStartOut{}, toolError(err)
				}
				if moments, added, err = addPlayMoments(moments, scan, in.PlayIDs); err != nil {
					return nil, streamRenderStartOut{}, err
				}
			}
			if len(moments) == 0 {
				return nil, streamRenderStartOut{}, errors.New("the stream has no moments saved; pass play_ids from find_stream_plays")
			}
			for _, m := range moments {
				body.Clips = append(body.Clips, m.clip)
			}
			summary := fmt.Sprintf("Renderizar %s de %s con el diseño %s%s: %s. FFmpeg trabajará durante varios minutos.",
				plural(len(moments), "vídeo", "vídeos"), streamName(j), variantLabel(body.Variant, variants),
				momentOrigin(saved, added, len(in.PlayIDs)), describeClips(body.Clips))
			ok, err := sessionApprovals.approve(req, approvalSpec{Tool: "render_stream", JobID: in.ID, Request: body, Summary: summary, Given: in.Approve})
			if err != nil {
				return nil, streamRenderStartOut{}, err
			}
			if ok.Ask != nil {
				return ok.Ask, streamRenderStartOut{}, nil
			}
			if !ok.Approved {
				return nil, streamRenderStartOut{NeedsApproval: true, ApprovalCode: ok.Code, Summary: summary}, nil
			}
			updatedAt, err := saveStreamRenderPlan(ctx, c, streamPlanSave{ID: in.ID, Doc: doc, Body: body, Moments: moments, Force: uploaded || added > 0})
			if err != nil {
				return nil, streamRenderStartOut{}, err
			}
			res, err := c.StartStreamRenderOfPlan(ctx, in.ID, body.Variant, updatedAt)
			if err != nil {
				return nil, streamRenderStartOut{}, toolError(err)
			}
			return nil, streamRenderStartOut{Summary: summary + fmt.Sprintf(" Started; follow with wait_for_stream_render and variant %q.", body.Variant),
				StreamID: in.ID, Status: cleanName(res.Status), Variant: body.Variant}, nil
		})

	mcp.AddTool(server, &mcp.Tool{Name: "wait_for_stream_render", Annotations: &mcp.ToolAnnotations{ReadOnlyHint: true}, Meta: appToolMeta(jobProgressAppURI),
		Description: "Wait for a stream render or the 16:9 summary to finish. Reports progress; call again while running is true. Then list_stream_videos has the videos."},
		func(ctx context.Context, req *mcp.CallToolRequest, in waitStreamRenderIn) (*mcp.CallToolResult, streamRenderOut, error) {
			if err := checkUUID("id", in.ID); err != nil {
				return nil, streamRenderOut{}, err
			}
			if err := checkArtifactName("variant", in.Variant); err != nil {
				return nil, streamRenderOut{}, err
			}
			c, err := s.client(ctx)
			if err != nil {
				return nil, streamRenderOut{}, toolError(err)
			}
			var out streamRenderOut
			err = poll(ctx, in.WaitSeconds, func() (bool, error) {
				if in.Variant == summaryVariant {
					sum, err := c.GetStreamSummary(ctx, in.ID)
					if err != nil {
						return false, err
					}
					out = describeSummaryRender(in.ID, sum)
				} else {
					state, err := c.GetStreamRender(ctx, in.ID, in.Variant)
					if err != nil {
						return false, err
					}
					out = describeStreamRender(in.ID, in.Variant, state)
				}
				notify(ctx, req, donePercent(!out.Running, out.Percent), streamRenderMessage(out))
				return !out.Running, nil
			})
			return nil, out, toolError(err)
		})
}

func checkStreamPlayIDs(ids []string) error {
	if len(ids) > maxSegmentIDs {
		return fmt.Errorf("play_ids takes at most %d ids", maxSegmentIDs)
	}
	seen := make(map[string]bool, len(ids))
	for _, id := range ids {
		if !streamPlayIDPattern.MatchString(id) {
			return fmt.Errorf("play_ids must come from find_stream_plays, not %q", cleanName(id))
		}
		if seen[id] {
			return fmt.Errorf("play id %s is listed twice", id)
		}
		seen[id] = true
	}
	return nil
}

// describePlayScan ranks plays the way Studio's Jugadas list does: best score, then earliest.
func describePlayScan(scan tuiclient.StreamPlayScan, limit int) streamPlaysOut {
	out := streamPlaysOut{Status: cleanName(scan.Status), Plays: []streamPlayOut{}, Total: len(scan.Plays), Error: cleanText(scan.Error, maxErrorRunes)}
	switch scan.Status {
	case playScanRunning:
		out.Percent = int(scan.Progress * 100)
		return out
	case playScanDone:
		if len(scan.Plays) == 0 {
			out.Note = "No kills of the streamer were found. The CS2 kill feed has to be visible at the top right of the video."
		}
	}
	ranked := slices.Clone(scan.Plays)
	slices.SortStableFunc(ranked, func(a, b tuiclient.StreamPlay) int {
		return cmp.Or(cmp.Compare(b.Score, a.Score), cmp.Compare(a.StartSeconds, b.StartSeconds))
	})
	if limit <= 0 {
		limit = defaultStreamPlaysLimit
	}
	for _, p := range ranked[:min(limit, len(ranked))] {
		out.Plays = append(out.Plays, streamPlayOut{PlayID: cleanName(p.ID), Label: cleanName(p.Label), Kills: len(p.KillSeconds), Score: p.Score,
			StartSeconds: p.StartSeconds, EndSeconds: p.EndSeconds})
	}
	out.Duration = playsSeconds(scan.Plays)
	return out
}

func playsSeconds(plays []tuiclient.StreamPlay) float64 {
	total := 0.0
	for _, p := range plays {
		total += p.EndSeconds - p.StartSeconds
	}
	return total
}

// streamRenderPlan picks the layout and refuses a facecam layout whose crop no
// one has checked: the render would refuse it too, after the user approved.
func streamRenderPlan(doc map[string]json.RawMessage, variant string, variants []tuiclient.StreamVariant) (streamRenderRequest, error) {
	var current struct {
		Variant   string `json:"variant"`
		Reviewed  bool   `json:"face_crop_reviewed"`
		UpdatedAt string `json:"updated_at"`
	}
	for key, dst := range map[string]any{"variant": &current.Variant, "face_crop_reviewed": &current.Reviewed, "updated_at": &current.UpdatedAt} {
		if raw, ok := doc[key]; ok {
			if err := json.Unmarshal(raw, dst); err != nil {
				return streamRenderRequest{}, fmt.Errorf("Studio returned an edit plan with an invalid %s", key)
			}
		}
	}
	updatedAt, err := planTime(current.UpdatedAt)
	if err != nil {
		return streamRenderRequest{}, err
	}
	if variant == "" {
		variant = current.Variant
	}
	idx := slices.IndexFunc(variants, func(v tuiclient.StreamVariant) bool { return v.Name == variant })
	if idx < 0 {
		names := make([]string, 0, len(variants))
		for _, v := range variants {
			names = append(names, v.Name)
		}
		return streamRenderRequest{}, fmt.Errorf("unknown variant %q; valid variants: %s", cleanName(variant), strings.Join(names, ", "))
	}
	if !variants[idx].FullFrame && !current.Reviewed {
		return streamRenderRequest{}, errors.New("this layout shows the facecam, and its crop has not been checked yet: ask the user to open the stream in Studio and confirm the facecam crop, " +
			"or pick a layout without facecam from list_stream_variants")
	}
	return streamRenderRequest{Variant: variant, PlanUpdatedAt: updatedAt}, nil
}

// addPlayMoments adds the plays to the saved moments as Studio's "Añadir" does
// (clipsWithPlays): a play overlapping a moment is skipped, then all sort by start.
func addPlayMoments(moments []planMoment, scan tuiclient.StreamPlayScan, ids []string) ([]planMoment, int, error) {
	if scan.Status != playScanDone {
		return nil, 0, errors.New("the play scan is not done; run find_stream_plays first")
	}
	byID := make(map[string]tuiclient.StreamPlay, len(scan.Plays))
	for _, p := range scan.Plays {
		byID[p.ID] = p
	}
	next := slices.Clone(moments)
	added := 0
	for _, id := range ids {
		p, ok := byID[id]
		if !ok {
			return nil, 0, fmt.Errorf("play id %s is not a play of this stream; use the ids from find_stream_plays", id)
		}
		if slices.ContainsFunc(next, func(m planMoment) bool {
			return m.clip.StartSeconds < p.EndSeconds && p.StartSeconds < m.clip.EndSeconds
		}) {
			continue
		}
		clip := planClip{ID: freeClipID(next, p.ID), StartSeconds: p.StartSeconds, EndSeconds: p.EndSeconds, Title: p.Label}
		raw, err := json.Marshal(clip)
		if err != nil {
			return nil, 0, err
		}
		next = append(next, planMoment{clip: clip, raw: raw})
		added++
	}
	slices.SortStableFunc(next, func(a, b planMoment) int { return cmp.Compare(a.clip.StartSeconds, b.clip.StartSeconds) })
	return next, added, nil
}

// freeClipID keeps the play id unless a saved moment already uses it: Studio refuses duplicate clip ids.
func freeClipID(moments []planMoment, id string) string {
	taken := func(c string) bool {
		return slices.ContainsFunc(moments, func(m planMoment) bool { return m.clip.ID == c })
	}
	candidate := id
	for n := 2; taken(candidate); n++ {
		candidate = fmt.Sprintf("%s-%d", id, n)
	}
	return candidate
}

func planMoments(doc map[string]json.RawMessage) ([]planMoment, error) {
	var raws []json.RawMessage
	if raw, ok := doc["clips"]; ok {
		if err := json.Unmarshal(raw, &raws); err != nil {
			return nil, errors.New("Studio returned an edit plan with invalid clips")
		}
	}
	moments := make([]planMoment, 0, len(raws))
	for _, raw := range raws {
		var clip planClip
		if err := json.Unmarshal(raw, &clip); err != nil {
			return nil, errors.New("Studio returned an edit plan with invalid clips")
		}
		moments = append(moments, planMoment{clip: clip, raw: raw})
	}
	return moments, nil
}

// momentOrigin says which videos come from saved moments and which from new plays.
func momentOrigin(saved, added, asked int) string {
	if asked == 0 {
		return ""
	}
	out := fmt.Sprintf(" (%s ya %s y %s %s", plural(saved, "momento", "momentos"), pluralWord(saved, "guardado", "guardados"),
		plural(added, "jugada", "jugadas"), pluralWord(added, "nueva", "nuevas"))
	if skipped := asked - added; skipped > 0 {
		out += fmt.Sprintf("; %s ya %s un momento", plural(skipped, "jugada", "jugadas"), pluralWord(skipped, "tenía", "tenían"))
	}
	return out + ")"
}

func pluralWord(n int, one, many string) string {
	if n == 1 {
		return one
	}
	return many
}

func countPlanClips(raw json.RawMessage) int {
	var plan struct {
		Clips []json.RawMessage `json:"clips"`
	}
	if len(raw) == 0 || json.Unmarshal(raw, &plan) != nil {
		return 0
	}
	return len(plan.Clips)
}

type streamPlanSave struct {
	ID      string
	Doc     map[string]json.RawMessage
	Body    streamRenderRequest
	Moments []planMoment
	Force   bool // an uploaded stream turns ready only when its plan is saved
}

// saveStreamRenderPlan saves the approved layout and moments, keeping every
// other field of the plan, and returns the updated_at the render must match.
func saveStreamRenderPlan(ctx context.Context, c *tuiclient.Client, in streamPlanSave) (string, error) {
	var current string
	_ = json.Unmarshal(in.Doc["variant"], &current)
	if !in.Force && current == in.Body.Variant {
		return in.Body.PlanUpdatedAt, nil
	}
	next := make(map[string]json.RawMessage, len(in.Doc))
	for k, v := range in.Doc {
		next[k] = v
	}
	variant, err := json.Marshal(in.Body.Variant)
	if err != nil {
		return "", err
	}
	next["variant"] = variant
	raws := make([]json.RawMessage, 0, len(in.Moments))
	for _, m := range in.Moments {
		raws = append(raws, m.raw)
	}
	clips, err := json.Marshal(raws)
	if err != nil {
		return "", err
	}
	next["clips"] = clips
	saved, err := c.PutStreamEditPlanDocument(ctx, in.ID, next)
	if err != nil {
		return "", toolError(err)
	}
	var updatedAt string
	if err := json.Unmarshal(saved["updated_at"], &updatedAt); err != nil {
		return "", errors.New("Studio saved the edit plan without an updated_at")
	}
	return planTime(updatedAt)
}

func describeClips(clips []planClip) string {
	parts := make([]string, 0, min(len(clips), maxSummaryPlays+1))
	for i, c := range clips {
		switch {
		case i < maxSummaryPlays:
			parts = append(parts, fmt.Sprintf("%s (%s a %s)", nonEmptyLabel(cleanName(c.Title), cleanName(c.ID)), clock(c.StartSeconds), clock(c.EndSeconds)))
		case i == maxSummaryPlays:
			parts = append(parts, fmt.Sprintf("y %d más", len(clips)-maxSummaryPlays))
		}
	}
	return strings.Join(parts, "; ")
}

func nonEmptyLabel(label, fallback string) string {
	if label == "" {
		return fallback
	}
	return label
}

func variantLabel(name string, variants []tuiclient.StreamVariant) string {
	for _, v := range variants {
		if v.Name == name {
			return fmt.Sprintf("%s (%s)", cleanName(v.Label), aspectRatio(v))
		}
	}
	return name
}

// streamName drops guillemets and quotes so a title cannot close the quote and read as approval text.
func streamName(j tuiclient.StreamJob) string {
	return quotedName(j.Title, "este stream")
}

// clock spells seconds as m:ss, or h:mm:ss for a long VOD.
func clock(seconds float64) string {
	total := max(int(seconds+0.5), 0)
	h, m, s := total/3600, total/60%60, total%60
	if h > 0 {
		return fmt.Sprintf("%d:%02d:%02d", h, m, s)
	}
	return fmt.Sprintf("%d:%02d", m, s)
}

func describeSummaryRender(id string, sum tuiclient.StreamSummary) streamRenderOut {
	out := streamRenderOut{StreamID: id, Variant: summaryVariant, Status: cleanName(sum.Status), Running: sum.Status == playScanRunning,
		Error: cleanText(sum.Error, maxErrorRunes)}
	switch sum.Status {
	case playScanRunning:
		out.Percent = int(sum.Progress * 100)
	case playScanDone:
		out.Note = "The summary is ready; list_stream_videos has its URL."
	case playScanIdle:
		out.Note = "No summary was started; call render_stream_summary."
	}
	return out
}

func describeStreamRender(id, variant string, state tuiclient.StreamRenderState) streamRenderOut {
	out := streamRenderOut{StreamID: id, Variant: variant, Status: cleanName(state.Status), Error: cleanText(state.Error, maxErrorRunes),
		Running: state.Status == "queued" || state.Status == tuiclient.StreamRendering}
	for _, w := range state.Warnings[:min(len(state.Warnings), maxNotices)] {
		out.Warnings = append(out.Warnings, cleanText(w, maxErrorRunes))
	}
	switch state.Status {
	case tuiclient.StreamRendering:
		// Clamped the way Studio's export stage does (web/lib/streams/render-progress.ts).
		if p := state.RenderProgress; p != nil && p.Clips >= 1 && !math.IsNaN(p.Percent) {
			out.Percent = int(math.Round(min(max(p.Percent, 0), 100)))
			out.Progress = fmt.Sprintf("video %d of %d", min(max(p.Clip, 1), p.Clips), p.Clips)
		}
	case tuiclient.StreamRendered:
		out.Note = "The render finished; list_stream_videos has the videos."
	case tuiclient.StreamFailed:
		out.Note = "The render failed."
	case tuiclient.StreamReady:
		out.Note = "No render of this variant was started; call render_stream."
	}
	return out
}

func streamRenderMessage(out streamRenderOut) string {
	if out.Progress != "" {
		return "render: " + out.Status + ", " + out.Progress
	}
	return "render: " + out.Status
}
