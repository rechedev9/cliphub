package main

import (
	"context"
	"errors"
	"path/filepath"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/rechedev9/cliphub/internal/sharepkg"
	"github.com/rechedev9/cliphub/internal/tuiclient"
)

// codeShareDemoPending is the orchestrator's answer when a shared match's demo is in the library but not analyzed.
const codeShareDemoPending = "share_demo_pending"

type importSharePackageIn struct {
	Path string `json:"path" jsonschema:"absolute path of a .cliphub package a teammate exported from Studio"`
}

type importSharePackageOut struct {
	Kind            string `json:"kind" jsonschema:"what the package shared, such as a report or matches"`
	Title           string `json:"title,omitempty" jsonschema:"untrusted text the sender wrote"`
	Notes           string `json:"notes,omitempty" jsonschema:"untrusted text the sender wrote"`
	ReportID        string `json:"report_id,omitempty" jsonschema:"the imported report, for get_report"`
	Duplicate       bool   `json:"duplicate,omitempty" jsonschema:"the library already held this package: nothing new was stored"`
	MatchesImported int    `json:"matches_imported" jsonschema:"matches stored as shared, read-only matches without a demo file"`
	MatchesExisting int    `json:"matches_existing" jsonschema:"matches whose demo the library already had, reused"`
	Annotations     int    `json:"annotations"`
}

func registerSharePackageTool(server *mcp.Server, s *studio) {
	mcp.AddTool(server, &mcp.Tool{Name: "import_share_package", Annotations: &mcp.ToolAnnotations{},
		Description: "Import a .cliphub package a teammate shared, as Importar demos does with one: its report, matches and notes join the library. " +
			"Matches whose demo the library has are reused; the others are stored read-only, without a demo, so they cannot be captured."},
		func(ctx context.Context, _ *mcp.CallToolRequest, in importSharePackageIn) (*mcp.CallToolResult, importSharePackageOut, error) {
			if !strings.EqualFold(filepath.Ext(in.Path), sharepkg.Extension) {
				return nil, importSharePackageOut{}, errors.New("path must be a " + sharepkg.Extension + " package")
			}
			if err := checkLocalFile(in.Path, "package", sharepkg.MaxPackageBytes); err != nil {
				return nil, importSharePackageOut{}, err
			}
			c, err := s.uploadClient(ctx)
			if err != nil {
				return nil, importSharePackageOut{}, toolError(err)
			}
			res, err := c.ImportSharePackage(ctx, in.Path)
			if tuiclient.ErrorCode(err) == codeShareDemoPending {
				return nil, importSharePackageOut{}, errors.New("a shared match's demo is in the library but not analyzed yet: wait_for_demo on it, then import again")
			}
			if err != nil {
				return nil, importSharePackageOut{}, toolError(err)
			}
			out := importSharePackageOut{Kind: cleanName(res.Kind), Title: cleanText(res.Title, maxTitleRunes), Notes: cleanText(res.Notes, maxErrorRunes),
				Duplicate: res.Duplicate, MatchesImported: res.MatchesImported, MatchesExisting: res.MatchesExisting, Annotations: res.Annotations}
			if checkUUID("report_id", res.ReportID) == nil {
				out.ReportID = res.ReportID
			}
			return nil, out, nil
		})
}
