package main

import (
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/rechedev9/cliphub/internal/tuiclient"
)

const (
	defaultVideoJobs = 30
	maxVideoJobs     = 100
	// recordingNotReusable prefixes a render failure whose capture must be redone (web/lib/api/failure-reason.ts).
	recordingNotReusable = "recording_not_reusable:"
)

// recaptureReasons are the render failures that only a new capture fixes, as requiresRecapture in the UI.
var recaptureReasons = []string{
	`capture_mode must be "real"`,
	"lacks completed POV verification",
	"capture input fingerprint does not match",
	"legacy recording result contains fields from a newer capture contract",
	"recording result publication is pending",
}

type listAllVideosIn struct {
	Jobs int `json:"jobs,omitempty" jsonschema:"how many of the newest jobs to look through; default 30, at most 100"`
}

type libraryVideoOut struct {
	JobID   string `json:"job_id"`
	Variant string `json:"variant"`
	Name    string `json:"name"`
	Status  string `json:"status"`
	URL     string `json:"url,omitempty"`
}

type libraryRenderOut struct {
	JobID   string `json:"job_id"`
	Variant string `json:"variant"`
	Status  string `json:"status" jsonschema:"queued, rendering or failed"`
	Error   string `json:"error,omitempty" jsonschema:"untrusted text, may quote demo content"`
}

type failedCaptureOut struct {
	JobID  string `json:"job_id"`
	Reason string `json:"reason,omitempty" jsonschema:"untrusted text, may quote demo content"`
}

type listAllVideosOut struct {
	Videos       []libraryVideoOut  `json:"videos" jsonschema:"finished videos, newest job first"`
	OtherRenders []libraryRenderOut `json:"other_renders,omitempty" jsonschema:"renders still queued or rendering, and failed ones retry_video may fix"`
	FailedJobs   []failedCaptureOut `json:"failed_jobs,omitempty" jsonschema:"jobs whose parse or CS2 capture failed; generate_short records again"`
	StreamVideos []libraryStreamOut `json:"stream_videos" jsonschema:"rendered stream moments, newest stream first; list_stream_videos also has a stream's 16:9 summary"`
	Note         string             `json:"note,omitempty"`
}

type libraryStreamOut struct {
	StreamID string `json:"stream_id"`
	streamVideoOut
}

type videoVariantIn struct {
	JobID   string `json:"job_id"`
	Variant string `json:"variant" jsonschema:"variant from list_all_videos or list_videos"`
	Approve string `json:"approve,omitempty" jsonschema:"only when asked: the approval code from a previous call, after the user agreed"`
}

type changeMusicIn struct {
	videoVariantIn
	Music       string `json:"music,omitempty" jsonschema:"music track id from list_short_options; empty removes the music"`
	MusicVolume *int   `json:"music_volume,omitempty" jsonschema:"music level in percent, 5 to 100; default 100"`
	GameVolume  *int   `json:"game_volume,omitempty" jsonschema:"game sound level under the music in percent, 0 to 100; default 70"`
}

func registerVideoLibraryTools(server *mcp.Server, s *studio) {
	mcp.AddTool(server, &mcp.Tool{Name: "list_all_videos", Annotations: &mcp.ToolAnnotations{ReadOnlyHint: true},
		Description: "List the demo videos of every recent job, as Mis vídeos does: finished ones with a URL that plays the MP4, renders still running, " +
			"failed renders (retry_video) and jobs that failed before a video existed. The URL only works while ClipHub Studio is open."},
		func(ctx context.Context, _ *mcp.CallToolRequest, in listAllVideosIn) (*mcp.CallToolResult, listAllVideosOut, error) {
			limit := cmp.Or(in.Jobs, defaultVideoJobs)
			if limit < 1 || limit > maxVideoJobs {
				return nil, listAllVideosOut{}, fmt.Errorf("jobs must be 1 to %d", maxVideoJobs)
			}
			c, err := s.client(ctx)
			if err != nil {
				return nil, listAllVideosOut{}, toolError(err)
			}
			rows, failed, err := readVideoLibrary(ctx, c, limit)
			if err != nil {
				return nil, listAllVideosOut{}, toolError(err)
			}
			streams, err := c.ListStreamJobs(ctx, min(limit, maxStreamListLimit))
			if err != nil {
				return nil, listAllVideosOut{}, toolError(err)
			}
			origin, originErr := s.webOrigin()
			out := describeVideoLibrary(rows, failed, origin)
			out.StreamVideos = describeStreamLibrary(streams, origin, originErr == nil)
			if originErr != nil {
				out.Note = "No playable URL: " + originErr.Error()
				for i := range out.Videos {
					out.Videos[i].URL = ""
				}
			}
			return nil, out, nil
		})

	mcp.AddTool(server, &mcp.Tool{Name: "retry_video",
		Description: "Render a failed Short again without a new capture, as Reintentar does in Mis vídeos. Studio keeps only the plays and options of the last " +
			"finished version, so this renders that version again: after a failed change_video_music, call change_video_music again instead. " +
			"A Short that never finished, a failed capture or a render that needs a new capture is redone with generate_short, and a Full Demo with render_full_demo. " +
			"Rendering takes minutes, so the user must approve it. " + deleteApprovalHint},
		func(ctx context.Context, req *mcp.CallToolRequest, in videoVariantIn) (*mcp.CallToolResult, generateOut, error) {
			c, state, err := loadShortRender(ctx, s, in)
			if err != nil {
				return nil, generateOut{}, err
			}
			if state.Status != tuiclient.RenderFailed {
				return nil, generateOut{}, fmt.Errorf("nothing to retry: this render is %s", cleanName(state.Status))
			}
			if needsRecapture(state.Error) {
				return nil, generateOut{}, errors.New("this render needs a new capture: call generate_short again with the same plays and options")
			}
			body := tuiclient.RenderRequest{Music: state.Music, SegmentIDs: state.SegmentIDs, Edit: state.Edit}
			summary := fmt.Sprintf("Volver a renderizar el vídeo %s con las jugadas y opciones de su última versión terminada. La captura no se repite.", cleanName(in.Variant))
			return approveAndRender(req, approvalSpec{Tool: "retry_video", JobID: in.JobID, Request: [2]any{in.Variant, state.UpdatedAt}, Summary: summary, Given: in.Approve},
				func() (tuiclient.EnqueueResponse, error) { return c.Rerender(ctx, in.JobID, in.Variant, body) }, in.Variant)
		})

	mcp.AddTool(server, &mcp.Tool{Name: "change_video_music",
		Description: "Add, change or remove the music of a finished Short, or its volume, as Cambiar música does in Mis vídeos. The capture is not repeated; " +
			"the Short is rendered again and the new MP4 replaces the current one. It also retries a failed music change. " +
			"Rendering takes minutes, so the user must approve it. " + deleteApprovalHint},
		func(ctx context.Context, req *mcp.CallToolRequest, in changeMusicIn) (*mcp.CallToolResult, generateOut, error) {
			music, err := resolveMusic(shortChoices{Music: in.Music, MusicVolume: in.MusicVolume, GameVolume: in.GameVolume})
			if err != nil {
				return nil, generateOut{}, err
			}
			c, state, err := loadShortRender(ctx, s, in.videoVariantIn)
			if err != nil {
				return nil, generateOut{}, err
			}
			// A failed replacement keeps the last finished version's choices, so its music can be set again.
			retryable := state.Status == tuiclient.RenderFailed && !needsRecapture(state.Error)
			if !renderReady(state.Status) && !retryable {
				return nil, generateOut{}, fmt.Errorf("the Short must be finished to change its music; this render is %s", cleanName(state.Status))
			}
			if music != nil {
				songs, err := c.ListSongs(ctx)
				if err != nil {
					return nil, generateOut{}, toolError(err)
				}
				if err := checkMusic(music.Key, songs); err != nil {
					return nil, generateOut{}, err
				}
			}
			if sameMix(state.Music, music) {
				return nil, generateOut{}, errors.New("this is the current mix: choose another track or volume")
			}
			body := tuiclient.RenderRequest{Music: music, SegmentIDs: state.SegmentIDs, Edit: state.Edit}
			summary := fmt.Sprintf("Volver a renderizar el Short con %s. La captura no se repite y el vídeo nuevo sustituye al actual.",
				describeMix(music))
			return approveAndRender(req, approvalSpec{Tool: "change_video_music", JobID: in.JobID, Request: [3]any{in.Variant, state.UpdatedAt, music}, Summary: summary, Given: in.Approve},
				func() (tuiclient.EnqueueResponse, error) { return c.Rerender(ctx, in.JobID, in.Variant, body) }, in.Variant)
		})
}

// loadShortRender reads a Short's render variant with the choices of its last render.
func loadShortRender(ctx context.Context, s *studio, in videoVariantIn) (*tuiclient.Client, tuiclient.RenderVideos, error) {
	if err := checkUUID("job_id", in.JobID); err != nil {
		return nil, tuiclient.RenderVideos{}, err
	}
	if err := checkArtifactName("variant", in.Variant); err != nil {
		return nil, tuiclient.RenderVideos{}, err
	}
	if in.Variant == fullDemoVariant {
		return nil, tuiclient.RenderVideos{}, errors.New("this is a Full Demo: plan_full_demo and render_full_demo make it again")
	}
	c, err := s.client(ctx)
	if err != nil {
		return nil, tuiclient.RenderVideos{}, toolError(err)
	}
	st, err := c.GetJobStatus(ctx, in.JobID)
	if err != nil {
		return nil, tuiclient.RenderVideos{}, toolError(err)
	}
	if st.Status == tuiclient.StatusFailed {
		return nil, tuiclient.RenderVideos{}, errors.New("the CS2 capture of this job failed: call generate_short again with the same plays and options")
	}
	state, err := c.GetRenderVideos(ctx, in.JobID, in.Variant)
	if err != nil {
		return nil, tuiclient.RenderVideos{}, toolError(err)
	}
	if len(state.Edit) == 0 || string(state.Edit) == "null" {
		return nil, tuiclient.RenderVideos{}, errors.New("this Short never finished, and Studio keeps the plays and options only of a finished render: make it again with generate_short")
	}
	var edit struct {
		FullDemo json.RawMessage `json:"full_demo"`
	}
	if json.Unmarshal(state.Edit, &edit) == nil && len(edit.FullDemo) > 0 && string(edit.FullDemo) != "null" {
		return nil, tuiclient.RenderVideos{}, errors.New("this is a Full Demo: plan_full_demo and render_full_demo make it again")
	}
	return c, state, nil
}

func approveAndRender(req *mcp.CallToolRequest, spec approvalSpec, start func() (tuiclient.EnqueueResponse, error), variant string) (*mcp.CallToolResult, generateOut, error) {
	ok, err := sessionApprovals.approve(req, spec)
	if err != nil {
		return nil, generateOut{}, err
	}
	if ok.Ask != nil {
		return ok.Ask, generateOut{}, nil
	}
	if !ok.Approved {
		return nil, generateOut{NeedsApproval: true, ApprovalCode: ok.Code, Summary: spec.Summary}, nil
	}
	res, err := start()
	if err != nil {
		return nil, generateOut{}, toolError(err)
	}
	if res.Duplicate {
		return nil, generateOut{}, errors.New("Studio is already rendering this video; wait for it with wait_for_job and the variant")
	}
	return nil, startedOut(spec.Summary, res, variant), nil
}

func needsRecapture(reason string) bool {
	if strings.HasPrefix(reason, recordingNotReusable) {
		return true
	}
	return slices.ContainsFunc(recaptureReasons, func(r string) bool { return strings.Contains(reason, r) })
}

// sameMix compares two mixes the way musicChoicesEqual does in the UI: full volume and the 70 % game mix are the defaults.
func sameMix(a, b *tuiclient.ShortMusic) bool {
	key := func(m *tuiclient.ShortMusic) string {
		if m == nil {
			return ""
		}
		return m.Key
	}
	if key(a) != key(b) {
		return false
	}
	if key(a) == "" {
		return true
	}
	volume := func(m *tuiclient.ShortMusic) float64 { return cmp.Or(m.Volume, 1) }
	game := func(m *tuiclient.ShortMusic) float64 {
		if m.GameVolume == nil {
			return 0.7
		}
		return *m.GameVolume
	}
	return volume(a) == volume(b) && game(a) == game(b)
}

func describeMix(m *tuiclient.ShortMusic) string {
	if m == nil {
		return "sin música"
	}
	game := 0.7
	if m.GameVolume != nil {
		game = *m.GameVolume
	}
	return fmt.Sprintf("la música %s (música %.0f %%, juego %.0f %%)", cleanName(m.Key), cmp.Or(m.Volume, 1)*100, game*100)
}

// readVideoLibrary reads every preset of the newest jobs that can have renders, in batches.
func readVideoLibrary(ctx context.Context, c *tuiclient.Client, limit int) ([]tuiclient.BatchStatusRow, []tuiclient.Job, error) {
	jobs, err := c.ListJobs(ctx, limit)
	if err != nil {
		return nil, nil, err
	}
	presets, err := c.Presets(ctx)
	if err != nil {
		return nil, nil, err
	}
	var items []tuiclient.BatchItem
	var failed []tuiclient.Job
	for _, j := range jobs {
		if checkUUID("job_id", j.ID) != nil || !slices.Contains(renderableStatuses, j.Status) {
			continue
		}
		if j.Status == tuiclient.StatusFailed {
			failed = append(failed, j)
		}
		for _, p := range presets.Presets {
			if artifactNamePattern.MatchString(p.Name) {
				items = append(items, tuiclient.BatchItem{JobID: j.ID, Variant: p.Name})
			}
		}
	}
	var rows []tuiclient.BatchStatusRow
	for chunk := range slices.Chunk(items, tuiclient.BatchStatusMaxItems) {
		part, err := c.BatchStatus(ctx, chunk)
		if err != nil {
			return nil, nil, err
		}
		rows = append(rows, part...)
	}
	return rows, failed, nil
}

// renderableStatuses are the job statuses that can hold render state (internal/job CanHaveRenderState).
var renderableStatuses = []string{tuiclient.StatusRecorded, "composing", tuiclient.StatusComposed, "review_required", tuiclient.StatusDone, tuiclient.StatusFailed}

func describeVideoLibrary(rows []tuiclient.BatchStatusRow, failed []tuiclient.Job, origin string) listAllVideosOut {
	out := listAllVideosOut{Videos: []libraryVideoOut{}}
	for _, row := range rows {
		r := row.Render
		if row.Error != nil || row.Job == nil || r == nil {
			continue
		}
		if !renderReady(r.Status) {
			out.OtherRenders = append(out.OtherRenders, libraryRenderOut{JobID: row.JobID, Variant: cleanName(row.Variant), Status: cleanName(r.Status), Error: cleanText(r.Error, maxErrorRunes)})
			continue
		}
		for _, name := range r.Videos {
			if !artifactNamePattern.MatchString(name) {
				continue
			}
			v := libraryVideoOut{JobID: row.JobID, Variant: row.Variant, Name: name, Status: r.Status}
			if origin != "" {
				v.URL = videoURL(origin, row.JobID, row.Variant, name)
			}
			out.Videos = append(out.Videos, v)
		}
	}
	// A render failure leaves the job status alone, so a failed job is a parse or capture failure.
	for _, j := range failed {
		out.FailedJobs = append(out.FailedJobs, failedCaptureOut{JobID: j.ID, Reason: cleanText(j.FailureReason, maxErrorRunes)})
	}
	return out
}

// describeStreamLibrary lists the rendered stream moments Mis vídeos shows beside the demo videos.
func describeStreamLibrary(streams []tuiclient.StreamJob, origin string, withURLs bool) []libraryStreamOut {
	out := []libraryStreamOut{}
	for _, j := range streams {
		if checkUUID("id", j.ID) != nil {
			continue
		}
		for _, v := range streamVideos(j, tuiclient.StreamSummary{}, origin, withURLs).Videos {
			out = append(out, libraryStreamOut{StreamID: j.ID, streamVideoOut: v})
		}
	}
	return out
}
