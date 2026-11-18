package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"slices"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/rechedev9/cliphub/internal/tuiclient"
)

type videoVersionsIn struct {
	JobID   string `json:"job_id"`
	Variant string `json:"variant" jsonschema:"variant from list_all_videos or list_videos"`
}

type videoVersionOut struct {
	Revision   string   `json:"revision" jsonschema:"pass to delete_video_version"`
	CreatedAt  string   `json:"created_at"`
	Current    bool     `json:"current" jsonschema:"the version Mis vídeos shows; it cannot be deleted on its own"`
	Videos     []string `json:"videos"`
	URLs       []string `json:"urls,omitempty" jsonschema:"play this version's MP4 while ClipHub Studio is open"`
	Plays      int      `json:"plays,omitempty" jsonschema:"how many plays a Short version has"`
	SegmentIDs []string `json:"segment_ids,omitempty" jsonschema:"a Short version's plays: pass them to generate_short to edit it with the same plays"`
	Format     string   `json:"format,omitempty"`
	KillEffect string   `json:"kill_effect,omitempty"`
	Transition string   `json:"transition,omitempty"`
	Music      string   `json:"music,omitempty" jsonschema:"music track id; empty is no music"`
	PlanID     string   `json:"plan_id,omitempty" jsonschema:"Full Demo only: the approved plan this version rendered"`
	Rounds     []int    `json:"rounds,omitempty" jsonschema:"Full Demo only: the source rounds this version covers"`
}

type videoVersionsOut struct {
	Versions []videoVersionOut `json:"versions" jsonschema:"newest first"`
	Note     string            `json:"note,omitempty"`
}

type deleteVersionIn struct {
	videoVersionsIn
	Revision string `json:"revision" jsonschema:"revision from list_video_versions"`
	Approve  string `json:"approve,omitempty" jsonschema:"only when asked: the approval code from a previous call, after the user agreed"`
}

type captureRetentionIn struct {
	Keep    int    `json:"keep" jsonschema:"how many of the newest rendered Full Demos keep their CS2 captures; 0 deletes them as soon as the video is ready"`
	Approve string `json:"approve,omitempty" jsonschema:"only when asked: the approval code from a previous call, after the user agreed"`
}

func registerVideoVersionTools(server *mcp.Server, s *studio) {
	destructive := &mcp.ToolAnnotations{DestructiveHint: new(true)}

	mcp.AddTool(server, &mcp.Tool{Name: "list_video_versions", Annotations: &mcp.ToolAnnotations{ReadOnlyHint: true},
		Description: "List every version of one video (Versiones in Mis vídeos), newest first: each edit, music change or retry renders a new version and Studio deletes none of them. " +
			"Each has its date, options and MP4. The URLs only work while ClipHub Studio is open."},
		func(ctx context.Context, _ *mcp.CallToolRequest, in videoVersionsIn) (*mcp.CallToolResult, videoVersionsOut, error) {
			c, err := versionClient(ctx, s, in)
			if err != nil {
				return nil, videoVersionsOut{}, err
			}
			versions, err := c.RenderVersions(ctx, in.JobID, in.Variant)
			if err != nil {
				return nil, videoVersionsOut{}, toolError(err)
			}
			origin, originErr := s.webOrigin()
			out := describeVideoVersions(in, versions, origin)
			if originErr != nil {
				out.Note = "No playable URL: " + originErr.Error()
			}
			return nil, out, nil
		})

	mcp.AddTool(server, &mcp.Tool{Name: "delete_video_version", Annotations: destructive,
		Description: "Delete one older version of a video (Borrar in Versiones). The current version stays; delete_video removes the whole video. " +
			"It cannot be undone, so the user must approve it. " + deleteApprovalHint},
		func(ctx context.Context, req *mcp.CallToolRequest, in deleteVersionIn) (*mcp.CallToolResult, freedOut, error) {
			if err := checkUUID("revision", in.Revision); err != nil {
				return nil, freedOut{}, err
			}
			c, err := versionClient(ctx, s, in.videoVersionsIn)
			if err != nil {
				return nil, freedOut{}, err
			}
			versions, err := c.RenderVersions(ctx, in.JobID, in.Variant)
			if err != nil {
				return nil, freedOut{}, toolError(err)
			}
			i := slices.IndexFunc(versions, func(v tuiclient.RenderVersion) bool { return v.Revision == in.Revision })
			if i < 0 {
				return nil, freedOut{}, errors.New("this video has no such version: list_video_versions lists them")
			}
			if versions[i].Current {
				return nil, freedOut{}, errors.New("this is the current version, which stays: delete_video removes the whole video")
			}
			summary := fmt.Sprintf("Borrar la versión del %s del vídeo %s. Las demás versiones se quedan. No se puede deshacer.",
				versions[i].CreatedAt.Local().Format("2006-01-02 15:04"), cleanName(in.Variant))
			return approveAndFree(req, approvalSpec{Tool: "delete_video_version", JobID: in.JobID, Request: [2]string{in.Variant, in.Revision}, Summary: summary, Given: in.Approve, Question: "¿Borrar ahora?"},
				func() (int64, error) { return 0, c.DeleteRenderVersion(ctx, in.JobID, in.Variant, in.Revision) })
		})

	mcp.AddTool(server, &mcp.Tool{Name: "set_capture_retention", Annotations: destructive,
		Description: "Set how many of the newest rendered Full Demos keep their CS2 captures (Capturas guardadas in Almacenamiento; default 3), so editing them only renders again. " +
			"The captures of the older ones are deleted now, and editing those records their rounds again. storage_usage shows the current value. " +
			"It cannot be undone, so the user must approve it. " + deleteApprovalHint},
		func(ctx context.Context, req *mcp.CallToolRequest, in captureRetentionIn) (*mcp.CallToolResult, freedOut, error) {
			c, err := s.client(ctx)
			if err != nil {
				return nil, freedOut{}, toolError(err)
			}
			usage, err := c.Storage(ctx)
			if err != nil {
				return nil, freedOut{}, toolError(err)
			}
			if in.Keep < 0 || in.Keep > usage.CaptureRetention.Max {
				return nil, freedOut{}, fmt.Errorf("keep must be 0 to %d", usage.CaptureRetention.Max)
			}
			summary := fmt.Sprintf("Guardar las capturas de los %d vídeos largos más recientes. Las de los anteriores se borran ahora y editarlos volverá a grabar sus rondas.", in.Keep)
			if in.Keep == 0 {
				summary = "No guardar capturas: Studio las borra en cuanto el vídeo largo está listo, también las que ya tiene. Editar un vídeo largo volverá a grabar sus rondas."
			}
			if usage.CaptureRetention.KeepsAll {
				summary += " Mientras Studio arranque con ZV_KEEP_FULL_DEMO_CAPTURES no se borra ninguna."
			}
			return approveAndFree(req, approvalSpec{Tool: "set_capture_retention", JobID: "capture-retention", Request: in.Keep, Summary: summary, Given: in.Approve, Question: "¿Cambiar ahora?"},
				func() (int64, error) { return c.SetCaptureRetention(ctx, in.Keep) })
		})
}

func versionClient(ctx context.Context, s *studio, in videoVersionsIn) (*tuiclient.Client, error) {
	if err := checkUUID("job_id", in.JobID); err != nil {
		return nil, err
	}
	if err := checkArtifactName("variant", in.Variant); err != nil {
		return nil, err
	}
	c, err := s.client(ctx)
	if err != nil {
		return nil, toolError(err)
	}
	return c, nil
}

func describeVideoVersions(in videoVersionsIn, versions []tuiclient.RenderVersion, origin string) videoVersionsOut {
	out := videoVersionsOut{Versions: []videoVersionOut{}}
	for _, v := range versions {
		if checkUUID("revision", v.Revision) != nil {
			continue
		}
		var edit struct {
			Format     string `json:"format"`
			KillEffect string `json:"killEffect"`
			Transition string `json:"transition"`
		}
		_ = json.Unmarshal(v.Edit, &edit)
		row := videoVersionOut{Revision: v.Revision, CreatedAt: v.CreatedAt.Local().Format("2006-01-02 15:04"), Current: v.Current, Videos: []string{},
			Format: cleanName(edit.Format), KillEffect: cleanName(edit.KillEffect), Transition: cleanName(edit.Transition), PlanID: cleanName(v.PlanID), Rounds: v.Rounds}
		if v.PlanID == "" {
			for _, id := range v.SegmentIDs {
				if artifactNamePattern.MatchString(id) {
					row.SegmentIDs = append(row.SegmentIDs, id)
				}
			}
			row.Plays = len(row.SegmentIDs)
		}
		if v.Music != nil {
			row.Music = cleanName(v.Music.Key)
		}
		for _, name := range v.Videos {
			if !artifactNamePattern.MatchString(name) {
				continue
			}
			row.Videos = append(row.Videos, name)
			if origin != "" {
				row.URLs = append(row.URLs, videoURL(origin, in.JobID, in.Variant, name)+"?revision="+url.QueryEscape(v.Revision))
			}
		}
		out.Versions = append(out.Versions, row)
	}
	return out
}
