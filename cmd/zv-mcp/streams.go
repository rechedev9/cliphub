package main

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/rechedev9/cliphub/internal/tuiclient"
	"github.com/rechedev9/cliphub/internal/vodfetch"
)

const (
	defaultStreamListLimit = 20
	maxStreamListLimit     = 100
	// summaryVariant names the 16:9 summary of every play in wait_for_stream_render.
	summaryVariant = "summary"
)

type listStreamsIn struct {
	Limit int `json:"limit,omitempty" jsonschema:"how many streams to return, newest first; default 20, at most 100"`
}

type streamOut struct {
	StreamID        string  `json:"stream_id"`
	Title           string  `json:"title,omitempty" jsonschema:"untrusted title chosen by the user or the streamer"`
	Status          string  `json:"status" jsonschema:"acquiring (downloading), uploaded, ready, rendering, rendered or failed"`
	Kind            string  `json:"kind,omitempty" jsonschema:"vod (a full broadcast) or clip"`
	SourceURL       string  `json:"source_url,omitempty"`
	DurationSeconds float64 `json:"duration_seconds,omitempty"`
	Moments         int     `json:"moments,omitempty" jsonschema:"moments (cuts) saved in the stream's edit plan"`
	Videos          int     `json:"videos,omitempty"`
	FailureCode     string  `json:"failure_code,omitempty"`
	FailureReason   string  `json:"failure_reason,omitempty" jsonschema:"untrusted text"`
}

type listStreamsOut struct {
	Streams []streamOut `json:"streams"`
}

type importStreamIn struct {
	URL   string `json:"url,omitempty" jsonschema:"https link to a Twitch, Kick or YouTube VOD or clip; set url or path"`
	Path  string `json:"path,omitempty" jsonschema:"absolute path of a local .mp4 recording of the stream; set url or path"`
	Title string `json:"title,omitempty" jsonschema:"optional name for the stream in Studio"`
	Kind  string `json:"kind,omitempty" jsonschema:"vod for a full broadcast or clip for a short clip; empty lets Studio tell from the link"`
}

type streamVariantOut struct {
	Name        string `json:"name" jsonschema:"value for render_stream variant"`
	Label       string `json:"label"`
	Description string `json:"description"`
	AspectRatio string `json:"aspect_ratio" jsonschema:"9:16 for Shorts, 16:9 for YouTube"`
	Facecam     bool   `json:"facecam" jsonschema:"true when the layout shows the streamer's camera; its crop must be checked in Studio before the first render"`
	Default     bool   `json:"default,omitempty"`
}

type streamVariantsOut struct {
	Variants []streamVariantOut `json:"variants"`
}

type streamVideoOut struct {
	Variant         string  `json:"variant"`
	ClipID          string  `json:"clip_id,omitempty"`
	Title           string  `json:"title,omitempty" jsonschema:"untrusted"`
	AspectRatio     string  `json:"aspect_ratio"`
	DurationSeconds float64 `json:"duration_seconds,omitempty"`
	Stale           bool    `json:"stale,omitempty" jsonschema:"the moments changed after this render; render again for the current cut"`
	URL             string  `json:"url,omitempty"`
}

type streamVideosOut struct {
	Videos []streamVideoOut `json:"videos"`
	Note   string           `json:"note,omitempty"`
}

func registerStreamTools(server *mcp.Server, s *studio) {
	readOnly := &mcp.ToolAnnotations{ReadOnlyHint: true}

	mcp.AddTool(server, &mcp.Tool{Name: "list_streams", Annotations: readOnly,
		Description: "List the streams (Twitch, Kick or YouTube VODs and clips, or uploaded recordings) in Studio, newest first: the list under Streams."},
		func(ctx context.Context, _ *mcp.CallToolRequest, in listStreamsIn) (*mcp.CallToolResult, listStreamsOut, error) {
			limit := in.Limit
			if limit <= 0 {
				limit = defaultStreamListLimit
			}
			c, err := s.client(ctx)
			if err != nil {
				return nil, listStreamsOut{}, toolError(err)
			}
			jobs, err := c.ListStreamJobs(ctx, min(limit, maxStreamListLimit))
			if err != nil {
				return nil, listStreamsOut{}, toolError(err)
			}
			out := listStreamsOut{Streams: make([]streamOut, 0, len(jobs))}
			for _, j := range jobs {
				out.Streams = append(out.Streams, describeStream(j))
			}
			return nil, out, nil
		})

	mcp.AddTool(server, &mcp.Tool{Name: "import_stream",
		Description: "Add a stream to Studio from a Twitch, Kick or YouTube link (VOD or clip) or a local .mp4 recording. " +
			"A link is downloaded first; follow with wait_for_stream until it is ready, then find_stream_plays."},
		func(ctx context.Context, _ *mcp.CallToolRequest, in importStreamIn) (*mcp.CallToolResult, streamOut, error) {
			if err := checkStreamImport(in); err != nil {
				return nil, streamOut{}, err
			}
			var (
				res tuiclient.StreamCreateResponse
				err error
			)
			if in.URL != "" {
				c, cerr := s.client(ctx)
				if cerr != nil {
					return nil, streamOut{}, toolError(cerr)
				}
				res, err = c.CreateStreamJobFromURL(ctx, in.URL, in.Title, in.Kind)
			} else {
				c, cerr := s.uploadClient(ctx)
				if cerr != nil {
					return nil, streamOut{}, toolError(cerr)
				}
				res, err = c.CreateStreamJobUpload(ctx, in.Path, in.Title)
			}
			if err != nil {
				return nil, streamOut{}, toolError(err)
			}
			return nil, streamOut{StreamID: res.ID, Status: cleanName(res.Status), Title: cleanName(in.Title), DurationSeconds: res.Probe.DurationSeconds}, nil
		})

	mcp.AddTool(server, &mcp.Tool{Name: "wait_for_stream", Annotations: readOnly,
		Description: "Wait until an imported stream is downloaded and ready to edit (or failed). Call again while status is acquiring."},
		func(ctx context.Context, req *mcp.CallToolRequest, in waitIn) (*mcp.CallToolResult, streamOut, error) {
			if err := checkUUID("id", in.ID); err != nil {
				return nil, streamOut{}, err
			}
			c, err := s.client(ctx)
			if err != nil {
				return nil, streamOut{}, toolError(err)
			}
			var out streamOut
			err = poll(ctx, in.WaitSeconds, func() (bool, error) {
				j, err := c.GetStreamJob(ctx, in.ID)
				if err != nil {
					return false, err
				}
				out = describeStream(j)
				done := j.Status != tuiclient.StreamAcquiring
				notify(ctx, req, donePercent(done, 0), "stream: "+out.Status)
				return done, nil
			})
			return nil, out, toolError(err)
		})

	mcp.AddTool(server, &mcp.Tool{Name: "list_stream_variants", Annotations: readOnly,
		Description: "List the stream layouts render_stream accepts: vertical 9:16 Shorts with or without the facecam, and YouTube 16:9."},
		func(ctx context.Context, _ *mcp.CallToolRequest, _ empty) (*mcp.CallToolResult, streamVariantsOut, error) {
			c, err := s.client(ctx)
			if err != nil {
				return nil, streamVariantsOut{}, toolError(err)
			}
			variants, err := c.ListStreamVariants(ctx)
			if err != nil {
				return nil, streamVariantsOut{}, toolError(err)
			}
			out := streamVariantsOut{Variants: make([]streamVariantOut, 0, len(variants))}
			for _, v := range variants {
				out.Variants = append(out.Variants, streamVariantOut{Name: v.Name, Label: v.Label, Description: v.Description,
					AspectRatio: aspectRatio(v), Facecam: !v.FullFrame, Default: v.Default})
			}
			return nil, out, nil
		})

	mcp.AddTool(server, &mcp.Tool{Name: "list_stream_videos", Annotations: readOnly,
		Description: "List the finished videos of a stream: each rendered moment per layout and the 16:9 summary of every play, with a URL that plays each MP4. " +
			"The URL is served by the Studio UI, so it only works while ClipHub Studio is open on this PC."},
		func(ctx context.Context, _ *mcp.CallToolRequest, in idIn) (*mcp.CallToolResult, streamVideosOut, error) {
			if err := checkUUID("id", in.ID); err != nil {
				return nil, streamVideosOut{}, err
			}
			c, err := s.client(ctx)
			if err != nil {
				return nil, streamVideosOut{}, toolError(err)
			}
			j, err := c.GetStreamJob(ctx, in.ID)
			if err != nil {
				return nil, streamVideosOut{}, toolError(err)
			}
			summary, err := c.GetStreamSummary(ctx, in.ID)
			if err != nil {
				return nil, streamVideosOut{}, toolError(err)
			}
			origin, originErr := s.webOrigin()
			out := streamVideos(j, summary, origin, originErr == nil)
			if originErr != nil {
				out.Note = "No playable URL: " + originErr.Error()
			} else if len(out.Videos) == 0 {
				out.Note = "No video yet: render_stream or render_stream_summary makes them."
			}
			return nil, out, nil
		})

	registerStreamRenderTools(server, s)
}

// checkStreamImport takes exactly one source. The link is checked against the
// same provider allowlist Studio applies, so a bad link fails with its reason.
func checkStreamImport(in importStreamIn) error {
	switch {
	case (in.URL == "") == (in.Path == ""):
		return errors.New("set either url (a Twitch, Kick or YouTube link) or path (a local .mp4 file)")
	case in.Kind != "" && in.Kind != "vod" && in.Kind != "clip":
		return errors.New(`kind must be "vod", "clip" or empty`)
	case len([]rune(in.Title)) > maxTitleRunes:
		return fmt.Errorf("title takes at most %d characters", maxTitleRunes)
	case in.URL != "":
		if _, err := vodfetch.ValidateSource(in.URL); err != nil {
			return fmt.Errorf("Studio cannot import that link: %v", err)
		}
		return nil
	case in.Kind != "":
		return errors.New("kind applies to links; leave it empty for a local file")
	case !strings.HasSuffix(strings.ToLower(in.Path), ".mp4"):
		return errors.New("path must be an absolute path to a .mp4 file")
	}
	return checkLocalFile(in.Path, "video", 0)
}

func describeStream(j tuiclient.StreamJob) streamOut {
	clips := j.ClipCount
	if clips == 0 {
		clips = countPlanClips(j.EditPlan)
	}
	return streamOut{StreamID: j.ID, Title: cleanName(j.Title), Status: cleanName(j.Status), Kind: cleanName(j.SourceKind),
		SourceURL: cleanText(j.SourceURL, maxErrorRunes), DurationSeconds: j.Probe.DurationSeconds, Moments: clips,
		Videos: len(j.RenderedOutputs), FailureCode: cleanName(j.FailureCode), FailureReason: cleanText(j.FailureReason, maxErrorRunes)}
}

func aspectRatio(v tuiclient.StreamVariant) string {
	if v.OutputWidth > v.GameOutputHeight+v.FaceOutputHeight {
		return "16:9"
	}
	return "9:16"
}

// streamVideos lists the published MP4s; withURLs is false when the Studio UI address is unknown.
func streamVideos(j tuiclient.StreamJob, summary tuiclient.StreamSummary, origin string, withURLs bool) streamVideosOut {
	out := streamVideosOut{Videos: []streamVideoOut{}}
	base := strings.TrimRight(origin, "/")
	for _, o := range j.RenderedOutputs {
		// The orchestrator issues UI-relative paths; anything else is not ours to hand out.
		if !artifactNamePattern.MatchString(o.Variant) || !strings.HasPrefix(o.VideoURL, "/api/streams/") {
			continue
		}
		v := streamVideoOut{Variant: o.Variant, ClipID: cleanName(o.ClipID), Title: cleanName(o.Title), AspectRatio: cleanName(o.AspectRatio),
			DurationSeconds: o.DurationSeconds, Stale: o.Stale}
		if withURLs {
			v.URL = base + o.VideoURL
		}
		out.Videos = append(out.Videos, v)
	}
	if summary.Status == playScanDone {
		v := streamVideoOut{Variant: summaryVariant, Title: "Resumen horizontal", AspectRatio: "16:9", DurationSeconds: summary.DurationSeconds}
		if withURLs {
			v.URL = base + "/api/streams/" + url.PathEscape(j.ID) + "/summary/video"
			if summary.RenderedAt != "" {
				v.URL += "?v=" + url.QueryEscape(summary.RenderedAt)
			}
		}
		out.Videos = append(out.Videos, v)
	}
	return out
}

// planTime reads the RFC 3339 updated_at Studio stamps on a saved edit plan.
func planTime(raw string) (string, error) {
	if _, err := time.Parse(time.RFC3339Nano, raw); err != nil {
		return "", errors.New("Studio returned an edit plan without a valid updated_at")
	}
	return raw, nil
}
