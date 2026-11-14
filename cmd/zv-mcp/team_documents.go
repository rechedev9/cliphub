package main

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/rechedev9/cliphub/internal/library"
	"github.com/rechedev9/cliphub/internal/tuiclient"
)

type listTeamDocumentsIn struct {
	TeamID string `json:"team_id,omitempty" jsonschema:"only this team's documents, id from list_teams"`
}

type teamDocumentOut struct {
	ID         string `json:"id"`
	TeamID     string `json:"team_id"`
	TeamName   string `json:"team_name" jsonschema:"untrusted team name"`
	Title      string `json:"title" jsonschema:"untrusted text a person or an assistant wrote"`
	FileName   string `json:"file_name" jsonschema:"untrusted file name"`
	SizeBytes  int64  `json:"size_bytes"`
	Updated    string `json:"updated" jsonschema:"YYYY-MM-DD"`
	StudioPath string `json:"studio_path" jsonschema:"opens the document in the Buscador de patrones' Documentos tab; pass it to open_in_studio"`
}

type listTeamDocumentsOut struct {
	Documents []teamDocumentOut `json:"documents"`
}

type addTeamDocumentIn struct {
	TeamID   string `json:"team_id" jsonschema:"the team the document is about, id from list_teams"`
	Title    string `json:"title" jsonschema:"the name the coach sees, such as Antistrats SEGA en Cache; at most 120 characters"`
	HTML     string `json:"html" jsonschema:"the whole document: an HTML page or fragment in UTF-8, at most 5 MiB, styles and images inline; it opens in a sandbox away from the session and ClipHub's API, where it cannot fetch data or send forms"`
	FileName string `json:"file_name,omitempty" jsonschema:"the name the download keeps, ending in .html; empty uses the title"`
}

type deleteTeamDocumentIn struct {
	DocumentID string `json:"document_id" jsonschema:"document id from list_team_documents"`
	Approve    string `json:"approve,omitempty" jsonschema:"only when asked: the approval code from a previous call, after the user agreed"`
}

func registerTeamDocumentTools(server *mcp.Server, s *studio) {
	mcp.AddTool(server, &mcp.Tool{Name: "list_team_documents", Annotations: &mcp.ToolAnnotations{ReadOnlyHint: true},
		Description: "List the HTML documents attached to teams (the Documentos tab of the Buscador de patrones), newest first, optionally of one team."},
		func(ctx context.Context, _ *mcp.CallToolRequest, in listTeamDocumentsIn) (*mcp.CallToolResult, listTeamDocumentsOut, error) {
			if in.TeamID != "" {
				if err := checkUUID("team_id", in.TeamID); err != nil {
					return nil, listTeamDocumentsOut{}, err
				}
			}
			c, err := s.client(ctx)
			if err != nil {
				return nil, listTeamDocumentsOut{}, toolError(err)
			}
			docs, err := c.ListTeamDocuments(ctx, in.TeamID)
			if err != nil {
				return nil, listTeamDocumentsOut{}, toolError(err)
			}
			out := listTeamDocumentsOut{Documents: make([]teamDocumentOut, 0, len(docs))}
			for _, d := range docs {
				out.Documents = append(out.Documents, teamDocumentOf(d))
			}
			return nil, out, nil
		})

	mcp.AddTool(server, &mcp.Tool{Name: "add_team_document", Annotations: &mcp.ToolAnnotations{},
		Description: "Attach an HTML document to a team, as Subir documento does in the Documentos tab of the Buscador de patrones: " +
			"an anti-strat or match prep report the coach opens in the app's viewer. Write it as one self-contained page (inline CSS, " +
			"images as data: URLs); it runs sandboxed, away from the session and ClipHub's API, and cannot fetch data or send forms. To change one, add the new version and delete the old."},
		func(ctx context.Context, _ *mcp.CallToolRequest, in addTeamDocumentIn) (*mcp.CallToolResult, teamDocumentOut, error) {
			upload, err := in.upload()
			if err != nil {
				return nil, teamDocumentOut{}, err
			}
			c, err := s.client(ctx)
			if err != nil {
				return nil, teamDocumentOut{}, toolError(err)
			}
			doc, err := c.UploadTeamDocument(ctx, upload)
			if err != nil {
				return nil, teamDocumentOut{}, toolError(err)
			}
			return nil, teamDocumentOf(doc), nil
		})

	mcp.AddTool(server, &mcp.Tool{Name: "delete_team_document", Annotations: &mcp.ToolAnnotations{DestructiveHint: new(true)},
		Description: "Delete a team's HTML document. It cannot be undone, so the user must approve it. " + deleteApprovalHint},
		func(ctx context.Context, req *mcp.CallToolRequest, in deleteTeamDocumentIn) (*mcp.CallToolResult, deleteOut, error) {
			if err := checkUUID("document_id", in.DocumentID); err != nil {
				return nil, deleteOut{}, err
			}
			c, err := s.client(ctx)
			if err != nil {
				return nil, deleteOut{}, toolError(err)
			}
			d, err := c.GetTeamDocument(ctx, in.DocumentID)
			if err != nil {
				return nil, deleteOut{}, toolError(err)
			}
			summary := fmt.Sprintf("Borrar el documento %s de %s. No se puede deshacer.", quotedName(d.Title, "sin título"), quotedName(d.TeamName, "su equipo"))
			return approveAndDelete(req, approvalSpec{Tool: "delete_team_document", JobID: in.DocumentID, Request: d.UpdatedAt, Summary: summary, Given: in.Approve, Question: "¿Borrar ahora?"},
				func() error { return c.DeleteTeamDocument(ctx, in.DocumentID) })
		})
}

func (in addTeamDocumentIn) upload() (tuiclient.TeamDocumentUpload, error) {
	if err := checkUUID("team_id", in.TeamID); err != nil {
		return tuiclient.TeamDocumentUpload{}, err
	}
	title, err := checkUserText("title", in.Title)
	if err != nil {
		return tuiclient.TeamDocumentUpload{}, err
	}
	content := []byte(in.HTML)
	if len(content) > library.MaxTeamDocumentBytes {
		return tuiclient.TeamDocumentUpload{}, fmt.Errorf("html takes at most %d MiB", library.MaxTeamDocumentBytes>>20)
	}
	if !library.LooksLikeHTML(content) {
		return tuiclient.TeamDocumentUpload{}, fmt.Errorf("html must be an HTML page or fragment")
	}
	name := strings.TrimSpace(in.FileName)
	if name == "" {
		// The server keeps only the last path element, so a slash in the title would cut the name.
		name = strings.NewReplacer("/", "-", `\`, "-").Replace(title)
	}
	if !strings.HasSuffix(strings.ToLower(name), ".html") && !strings.HasSuffix(strings.ToLower(name), ".htm") {
		name += ".html"
	}
	return tuiclient.TeamDocumentUpload{TeamID: in.TeamID, Title: title, FileName: name, Content: content}, nil
}

func teamDocumentOf(d tuiclient.TeamDocument) teamDocumentOut {
	return teamDocumentOut{ID: d.ID, TeamID: d.TeamID, TeamName: cleanName(d.TeamName), Title: cleanName(d.Title), FileName: cleanName(d.FileName),
		SizeBytes: d.SizeBytes, Updated: d.UpdatedAt.Format(time.DateOnly), StudioPath: teamDocumentPath(d.TeamID, d.ID)}
}

// teamDocumentPath opens the Buscador de patrones of the team on its Documentos tab with the document in the viewer.
func teamDocumentPath(teamID, id string) string {
	return "/patrones/" + url.PathEscape(teamID) + "?" + url.Values{"panel": {"documentos"}, "documento": {id}}.Encode()
}
