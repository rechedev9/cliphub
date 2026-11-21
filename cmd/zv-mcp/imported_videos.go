package main

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/rechedev9/cliphub/internal/importedvideos"
	"github.com/rechedev9/cliphub/internal/tuiclient"
)

type importedVideoOut struct {
	CloudJobID string `json:"cloud_job_id" jsonschema:"pass with name to delete_imported_video"`
	Name       string `json:"name"`
	Title      string `json:"title,omitempty" jsonschema:"untrusted text from the old cloud"`
	JobID      string `json:"job_id,omitempty" jsonschema:"the local job the video came from, when Studio still has it"`
	CreatedAt  string `json:"created_at,omitempty"`
	URL        string `json:"url,omitempty"`
}

type deleteImportedVideoIn struct {
	CloudJobID string `json:"cloud_job_id" jsonschema:"cloud_job_id from list_all_videos imported_videos"`
	Name       string `json:"name" jsonschema:"name from list_all_videos imported_videos"`
	Approve    string `json:"approve,omitempty" jsonschema:"only when asked: the approval code from a previous call, after the user agreed"`
}

// describeImportedVideos keeps the videos whose ids Studio would serve, with a playable URL when the web UI is up.
func describeImportedVideos(videos []tuiclient.ImportedVideo, origin string) []importedVideoOut {
	out := []importedVideoOut{}
	for _, v := range videos {
		if checkUUID("cloud_job_id", v.CloudJobID) != nil || !importedvideos.ValidName(v.Name) {
			continue
		}
		row := importedVideoOut{CloudJobID: v.CloudJobID, Name: v.Name, Title: cleanText(v.Title, maxTitleRunes)}
		if checkUUID("job_id", v.LocalJobID) == nil {
			row.JobID = v.LocalJobID
		}
		if !v.CreatedAt.IsZero() {
			row.CreatedAt = v.CreatedAt.Local().Format(time.RFC3339)
		}
		if origin != "" {
			row.URL = strings.TrimRight(origin, "/") + tuiclient.ImportedVideoPath(v.CloudJobID, v.Name)
		}
		out = append(out, row)
	}
	return out
}

func registerImportedVideoTools(server *mcp.Server, s *studio) {
	mcp.AddTool(server, &mcp.Tool{Name: "delete_imported_video", Annotations: &mcp.ToolAnnotations{DestructiveHint: new(true)},
		Description: "Delete one Short imported from the old ClipHub cloud from the PC, as Borrar does on it in Mis vídeos. list_all_videos lists them under imported_videos. " +
			"It cannot be undone, so the user must approve it. " + deleteApprovalHint},
		func(ctx context.Context, req *mcp.CallToolRequest, in deleteImportedVideoIn) (*mcp.CallToolResult, deleteOut, error) {
			if err := checkUUID("cloud_job_id", in.CloudJobID); err != nil {
				return nil, deleteOut{}, err
			}
			if !importedvideos.ValidName(in.Name) {
				return nil, deleteOut{}, errors.New("name must be a video name from list_all_videos imported_videos")
			}
			summary := fmt.Sprintf("Borrar del PC el vídeo importado %s. No se puede deshacer.", in.Name)
			return approveAndDelete(req, approvalSpec{Tool: "delete_imported_video", JobID: in.CloudJobID, Request: in.Name, Summary: summary, Given: in.Approve, Question: "¿Borrar ahora?"},
				func() error {
					c, err := s.client(ctx)
					if err != nil {
						return err
					}
					return c.DeleteImportedVideo(ctx, in.CloudJobID, in.Name)
				})
		})
}
