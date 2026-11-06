package main

import (
	"context"
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type deleteVideoIn struct {
	JobID   string `json:"job_id"`
	Variant string `json:"variant" jsonschema:"variant from list_videos"`
	Name    string `json:"name" jsonschema:"video name from list_videos"`
	Approve string `json:"approve,omitempty" jsonschema:"only when asked: the approval code from a previous call, after the user agreed"`
}

type deleteStreamIn struct {
	ID      string `json:"id" jsonschema:"stream_id from list_streams"`
	Approve string `json:"approve,omitempty" jsonschema:"only when asked: the approval code from a previous call, after the user agreed"`
}

type deleteOut struct {
	NeedsApproval bool   `json:"needs_approval,omitempty"`
	ApprovalCode  string `json:"approval_code,omitempty"`
	Summary       string `json:"summary"`
	Deleted       bool   `json:"deleted,omitempty"`
}

const deleteApprovalHint = "If the result says needs_approval, show the summary, ask the user, and call again with approve set to approval_code only if they agree."

func registerDeleteTools(server *mcp.Server, s *studio) {
	destructive := &mcp.ToolAnnotations{DestructiveHint: new(true)}

	mcp.AddTool(server, &mcp.Tool{Name: "delete_video", Annotations: destructive,
		Description: "Delete one finished demo video (with its cover and captions) from the PC, as Borrar does in Mis vídeos. It cannot be undone, so the user must approve it. " + deleteApprovalHint},
		func(ctx context.Context, req *mcp.CallToolRequest, in deleteVideoIn) (*mcp.CallToolResult, deleteOut, error) {
			if err := checkUUID("job_id", in.JobID); err != nil {
				return nil, deleteOut{}, err
			}
			if err := checkArtifactName("variant", in.Variant); err != nil {
				return nil, deleteOut{}, err
			}
			if err := checkArtifactName("name", in.Name); err != nil {
				return nil, deleteOut{}, err
			}
			summary := fmt.Sprintf("Borrar del PC el vídeo %s (%s). No se puede deshacer.", in.Name, in.Variant)
			return approveAndDelete(req, approvalSpec{Tool: "delete_video", JobID: in.JobID, Request: [2]string{in.Variant, in.Name}, Summary: summary, Given: in.Approve, Question: "¿Borrar ahora?"},
				func() error {
					c, err := s.client(ctx)
					if err != nil {
						return err
					}
					return c.DeleteRenderVideo(ctx, in.JobID, in.Variant, in.Name)
				})
		})

	mcp.AddTool(server, &mcp.Tool{Name: "delete_stream", Annotations: destructive,
		Description: "Delete a stream from Studio with its downloaded video and every render of it, as Borrar does under Streams. It cannot be undone, so the user must approve it. " + deleteApprovalHint},
		func(ctx context.Context, req *mcp.CallToolRequest, in deleteStreamIn) (*mcp.CallToolResult, deleteOut, error) {
			if err := checkUUID("id", in.ID); err != nil {
				return nil, deleteOut{}, err
			}
			c, err := s.client(ctx)
			if err != nil {
				return nil, deleteOut{}, toolError(err)
			}
			j, err := c.GetStreamJob(ctx, in.ID)
			if err != nil {
				return nil, deleteOut{}, toolError(err)
			}
			summary := fmt.Sprintf("Borrar %s de Studio con su vídeo original. No se puede deshacer.", streamName(j))
			if n := len(j.RenderedOutputs); n > 0 {
				summary = fmt.Sprintf("Borrar %s de Studio con su vídeo original y %s. No se puede deshacer.",
					streamName(j), plural(n, "vídeo renderizado", "vídeos renderizados"))
			}
			return approveAndDelete(req, approvalSpec{Tool: "delete_stream", JobID: in.ID, Request: j.UpdatedAt, Summary: summary, Given: in.Approve, Question: "¿Borrar ahora?"},
				func() error { return c.DeleteStreamJob(ctx, in.ID) })
		})
}

func approveAndDelete(req *mcp.CallToolRequest, spec approvalSpec, remove func() error) (*mcp.CallToolResult, deleteOut, error) {
	ok, err := sessionApprovals.approve(req, spec)
	if err != nil {
		return nil, deleteOut{}, err
	}
	if ok.Ask != nil {
		return ok.Ask, deleteOut{}, nil
	}
	if !ok.Approved {
		return nil, deleteOut{NeedsApproval: true, ApprovalCode: ok.Code, Summary: spec.Summary}, nil
	}
	if err := remove(); err != nil {
		return nil, deleteOut{}, toolError(err)
	}
	return nil, deleteOut{Summary: spec.Summary, Deleted: true}, nil
}
