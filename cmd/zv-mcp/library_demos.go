package main

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/rechedev9/cliphub/internal/tuiclient"
)

// Library tag limits (internal/library NormalizeTags).
const (
	maxDemoTags     = 32
	maxDemoTagRunes = 40
)

// editableSources are the sources a person may pick in the demo panel; shared is read only.
var editableSources = []string{"valve", "faceit", "hltv", "scrim", "unknown"}

type importIn struct {
	Path string   `json:"path" jsonschema:"absolute path of a local .dem or .dem.zst file"`
	Tags []string `json:"tags,omitempty" jsonschema:"labels for the library, such as a tournament or scrim partner; at most 32 of 40 characters, no commas"`
}

type demoOut struct {
	DemoID       string   `json:"demo_id"`
	State        string   `json:"state" jsonschema:"queued, parsing, ready or failed"`
	MatchID      string   `json:"match_id,omitempty"`
	FileName     string   `json:"file_name,omitempty" jsonschema:"untrusted file name"`
	Source       string   `json:"source,omitempty" jsonschema:"valve, faceit, hltv, scrim, unknown or shared"`
	SourceManual bool     `json:"source_manual,omitempty" jsonschema:"true when a person picked the source; re-analysis keeps it"`
	Tags         []string `json:"tags,omitempty" jsonschema:"untrusted labels"`
	Duplicate    bool     `json:"duplicate,omitempty"`
	Error        string   `json:"error,omitempty"`
}

type updateDemoIn struct {
	ID     string    `json:"id" jsonschema:"demo_id from get_match, list_matches or import_demo"`
	Source string    `json:"source,omitempty" jsonschema:"valve, faceit, hltv, scrim or unknown; empty keeps the source"`
	Tags   *[]string `json:"tags,omitempty" jsonschema:"the whole new tag set, replacing the old one (at most 32 of 40 characters, no commas); an empty list clears it; omit to keep the tags"`
	// AddTags and RemoveTags are the "+ Etiqueta" and remove of a Partidas row: they change the demo's tags without sending the whole set.
	AddTags    []string `json:"add_tags,omitempty" jsonschema:"tags to add to the ones the demo has (the Etiqueta button of a row); not with tags"`
	RemoveTags []string `json:"remove_tags,omitempty" jsonschema:"tags to remove from the ones the demo has; not with tags"`
}

// maxDeleteDemos caps one delete_demo call, so the approval summary stays readable.
const maxDeleteDemos = 50

type deleteDemoIn struct {
	ID      string   `json:"id,omitempty" jsonschema:"demo_id from get_match or list_matches; use ids for several"`
	IDs     []string `json:"ids,omitempty" jsonschema:"instead of id: up to 50 demo_ids to remove with one approval"`
	Approve string   `json:"approve,omitempty" jsonschema:"only when asked: the approval code from a previous call, after the user agreed"`
}

func (in deleteDemoIn) demoIDs() ([]string, error) {
	switch {
	case in.ID != "" && len(in.IDs) > 0:
		return nil, errors.New("pass id or ids, not both")
	case in.ID != "":
		return []string{in.ID}, checkUUID("id", in.ID)
	case len(in.IDs) == 0:
		return nil, errors.New("id or ids is required")
	case len(in.IDs) > maxDeleteDemos:
		return nil, fmt.Errorf("at most %d ids per call", maxDeleteDemos)
	}
	for i, id := range in.IDs {
		if err := checkUUID("ids", id); err != nil {
			return nil, err
		}
		if slices.Contains(in.IDs[:i], id) {
			return nil, fmt.Errorf("ids lists %s twice", id)
		}
	}
	return in.IDs, nil
}

func registerLibraryDemoTools(server *mcp.Server, s *studio) {
	readOnly := &mcp.ToolAnnotations{ReadOnlyHint: true}

	mcp.AddTool(server, &mcp.Tool{Name: "import_demo", Meta: appToolMeta(jobProgressAppURI),
		Description: "Import a local CS2 demo into the Studio library, optionally with tags. Analysis starts on its own; follow with wait_for_demo. " +
			"A duplicate (the same file already in the library) keeps its own tags; change them with update_demo."},
		func(ctx context.Context, _ *mcp.CallToolRequest, in importIn) (*mcp.CallToolResult, demoOut, error) {
			if err := checkDemoPath(in.Path); err != nil {
				return nil, demoOut{}, err
			}
			if err := checkDemoTags(in.Tags); err != nil {
				return nil, demoOut{}, err
			}
			c, err := s.uploadClient(ctx)
			if err != nil {
				return nil, demoOut{}, toolError(err)
			}
			res, err := c.ImportLibraryDemo(ctx, in.Path, in.Tags)
			if err != nil {
				return nil, demoOut{}, toolError(err)
			}
			if res.Demo == nil {
				return nil, demoOut{}, fmt.Errorf("Studio rejected the demo (%s): %s", cleanName(res.ErrorCode), cleanText(res.Error, maxErrorRunes))
			}
			out := describeDemo(*res.Demo)
			out.Duplicate = res.Status == "duplicate"
			return nil, out, nil
		})

	mcp.AddTool(server, &mcp.Tool{Name: "wait_for_demo", Annotations: readOnly, Meta: appToolMeta(jobProgressAppURI),
		Description: "Wait until an imported demo is analysed (state ready or failed). Returns its match_id when ready."},
		func(ctx context.Context, req *mcp.CallToolRequest, in waitIn) (*mcp.CallToolResult, demoOut, error) {
			if err := checkUUID("id", in.ID); err != nil {
				return nil, demoOut{}, err
			}
			c, err := s.client(ctx)
			if err != nil {
				return nil, demoOut{}, toolError(err)
			}
			var out demoOut
			err = poll(ctx, in.WaitSeconds, func() (bool, error) {
				d, err := c.GetLibraryDemo(ctx, in.ID)
				if err != nil {
					return false, err
				}
				out = describeDemo(d)
				done := d.State == "ready" || d.State == "failed"
				notify(ctx, req, donePercent(done, 0), "analysing demo: "+d.State)
				return done, nil
			})
			return nil, out, toolError(err)
		})

	mcp.AddTool(server, &mcp.Tool{Name: "update_demo", Annotations: &mcp.ToolAnnotations{IdempotentHint: true},
		Description: "Change a library demo's source (Fuente) or its tags (Etiquetas), as the Demo panel of a match and the Etiqueta button of a Partidas row do: " +
			"tags replaces the whole set, add_tags and remove_tags change single tags. " +
			"A source picked here survives every re-analysis. A shared match keeps its source."},
		func(ctx context.Context, _ *mcp.CallToolRequest, in updateDemoIn) (*mcp.CallToolResult, demoOut, error) {
			if err := checkUUID("id", in.ID); err != nil {
				return nil, demoOut{}, err
			}
			if in.Tags != nil && (len(in.AddTags) > 0 || len(in.RemoveTags) > 0) {
				return nil, demoOut{}, errors.New("pass tags, or add_tags and remove_tags, not both")
			}
			if err := checkDemoTags(in.AddTags); err != nil {
				return nil, demoOut{}, err
			}
			patch := tuiclient.LibraryDemoPatch{Tags: in.Tags}
			if in.Source != "" {
				if !slices.Contains(editableSources, in.Source) {
					return nil, demoOut{}, fmt.Errorf("source must be one of %s", strings.Join(editableSources, ", "))
				}
				patch.Source = &in.Source
			}
			if in.Tags != nil {
				if err := checkDemoTags(*in.Tags); err != nil {
					return nil, demoOut{}, err
				}
			}
			editsTags := len(in.AddTags) > 0 || len(in.RemoveTags) > 0
			if patch.Source == nil && patch.Tags == nil && !editsTags {
				return nil, demoOut{}, errors.New("nothing to change: pass source, tags, add_tags or remove_tags")
			}
			c, err := s.client(ctx)
			if err != nil {
				return nil, demoOut{}, toolError(err)
			}
			if editsTags {
				current, err := c.GetLibraryDemo(ctx, in.ID)
				if err != nil {
					return nil, demoOut{}, toolError(err)
				}
				next := editTags(current.Tags, in.AddTags, in.RemoveTags)
				if err := checkDemoTags(next); err != nil {
					return nil, demoOut{}, err
				}
				patch.Tags = &next
			}
			d, err := c.UpdateLibraryDemo(ctx, in.ID, patch)
			if err != nil {
				return nil, demoOut{}, toolError(err)
			}
			return nil, describeDemo(d), nil
		})

	mcp.AddTool(server, &mcp.Tool{Name: "reanalyze_demo",
		Description: "Analyse a library demo again (Volver a analizar): after a failure, or to refresh a match after a Studio update. " +
			"A demo already queued or parsing is left as it is. Follow with wait_for_demo."},
		func(ctx context.Context, _ *mcp.CallToolRequest, in idIn) (*mcp.CallToolResult, demoOut, error) {
			if err := checkUUID("id", in.ID); err != nil {
				return nil, demoOut{}, err
			}
			c, err := s.client(ctx)
			if err != nil {
				return nil, demoOut{}, toolError(err)
			}
			d, err := c.AnalyzeLibraryDemo(ctx, in.ID)
			if err != nil {
				return nil, demoOut{}, toolError(err)
			}
			return nil, describeDemo(d), nil
		})

	mcp.AddTool(server, &mcp.Tool{Name: "delete_demo", Annotations: &mcp.ToolAnnotations{DestructiveHint: new(true)},
		Description: "Remove demos from the library with their match, analysis and corrections, as Quitar de la biblioteca does for one " +
			"and Borrar does for the partidas chosen in Partidas: id for one demo, ids for several under one approval. " +
			"Videos already made and their jobs stay. It cannot be undone, so the user must approve it. " + deleteApprovalHint},
		func(ctx context.Context, req *mcp.CallToolRequest, in deleteDemoIn) (*mcp.CallToolResult, deleteOut, error) {
			ids, err := in.demoIDs()
			if err != nil {
				return nil, deleteOut{}, err
			}
			c, err := s.client(ctx)
			if err != nil {
				return nil, deleteOut{}, toolError(err)
			}
			names := make([]string, 0, len(ids))
			imported := make([]time.Time, 0, len(ids))
			for _, id := range ids {
				d, err := c.GetLibraryDemo(ctx, id)
				if err != nil {
					return nil, deleteOut{}, toolError(err)
				}
				names = append(names, cleanName(d.FileName))
				imported = append(imported, d.Imported)
			}
			spec := approvalSpec{Tool: "delete_demo", JobID: strings.Join(ids, ","), Request: imported,
				Summary: deleteDemosSummary(names), Given: in.Approve, Question: "¿Quitar ahora?"}
			return approveAndDelete(req, spec, func() error {
				for i, id := range ids {
					if err := c.DeleteLibraryDemo(ctx, id); err != nil {
						return fmt.Errorf("removed %d of %d demos, then %s failed: %w", i, len(ids), names[i], err)
					}
				}
				return nil
			})
		})
}

// deleteDemosSummary names up to five files, then counts the rest.
func deleteDemosSummary(names []string) string {
	const shown = 5
	const effect = "Los vídeos ya creados no se tocan. No se puede deshacer."
	if len(names) == 1 {
		return fmt.Sprintf("Quitar %s de la biblioteca: se borran la copia de la demo, su análisis y sus correcciones. %s", names[0], effect)
	}
	list := strings.Join(names[:min(len(names), shown)], ", ")
	if len(names) > shown {
		list += fmt.Sprintf(" y %d más", len(names)-shown)
	}
	return fmt.Sprintf("Quitar %d partidas de la biblioteca (%s): se borran las copias de las demos, sus análisis y sus correcciones. %s",
		len(names), list, effect)
}

// editTags removes, then appends, keeping the order and leaving no duplicate.
func editTags(current, add, remove []string) []string {
	next := make([]string, 0, len(current)+len(add))
	for _, tag := range current {
		if !slices.Contains(remove, tag) && !slices.Contains(next, tag) {
			next = append(next, tag)
		}
	}
	for _, tag := range add {
		if !slices.Contains(next, tag) {
			next = append(next, tag)
		}
	}
	return next
}

func checkDemoTags(tags []string) error {
	if len(tags) > maxDemoTags {
		return fmt.Errorf("at most %d tags", maxDemoTags)
	}
	for _, tag := range tags {
		if strings.TrimSpace(tag) == "" || utf8.RuneCountInString(tag) > maxDemoTagRunes {
			return fmt.Errorf("each tag needs 1 to %d characters", maxDemoTagRunes)
		}
		// The Demo panel edits tags as one comma-separated text, so a comma would split the tag there.
		if strings.Contains(tag, ",") {
			return errors.New("a tag cannot contain a comma; pass each tag as its own item")
		}
	}
	return nil
}

func describeDemo(d tuiclient.LibraryDemo) demoOut {
	out := demoOut{DemoID: d.ID, State: cleanName(d.State), FileName: cleanName(d.FileName), Source: cleanName(d.Source),
		SourceManual: d.SourceManual, Tags: cleanNames(d.Tags), Error: cleanText(d.Error, maxErrorRunes)}
	if d.MatchID != nil {
		out.MatchID = *d.MatchID
	}
	return out
}
