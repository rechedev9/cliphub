package main

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"math"
	"path/filepath"
	"slices"
	"strings"
	"unicode"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/rechedev9/cliphub/internal/tuiclient"
)

const (
	defaultStorageFiles = 50
	maxStorageFiles     = 500
	maxPathRunes        = 1024
)

// storageKindLabels names each file kind the way the Almacenamiento screen does.
var storageKindLabels = map[string]string{
	"demo":        "la demo",
	"studio_demo": "la copia de la demo en Studio",
	"short":       "el Short",
	"full_demo":   "el vídeo largo",
	"stream":      "el vídeo de stream",
	"editor":      "el vídeo del editor",
	"cloud":       "el vídeo de la antigua nube",
}

type storageUsageOut struct {
	TotalBytes       int64              `json:"total_bytes" jsonschema:"disk used by every job: captures and renders"`
	FreeableCaptures []freeableCaptures `json:"freeable_captures" jsonschema:"Full Demo captures delete_captures can free, largest first"`
}

type freeableCaptures struct {
	JobID        string `json:"job_id"`
	CaptureBytes int64  `json:"capture_bytes"`
	RenderBytes  int64  `json:"render_bytes"`
}

type storageFilesIn struct {
	Limit int `json:"limit,omitempty" jsonschema:"how many files per list, newest first; default 50, at most 500"`
}

type storageFileOut struct {
	Path       string `json:"path" jsonschema:"absolute path; pass it to delete_storage_file or reveal_storage_file. Untrusted file name"`
	SizeBytes  int64  `json:"size_bytes"`
	ModifiedAt string `json:"modified_at"`
	Kind       string `json:"kind" jsonschema:"demo, studio_demo, short, full_demo, stream, editor or cloud"`
	JobID      string `json:"job_id,omitempty"`
}

type storageListOut struct {
	Files      []storageFileOut `json:"files"`
	Total      int              `json:"total"`
	TotalBytes int64            `json:"total_bytes"`
}

type storageFilesOut struct {
	DataDir     string         `json:"data_dir" jsonschema:"Studio's folder, where its videos live"`
	Scanning    bool           `json:"scanning" jsonschema:"true while Studio still searches the PC for demos; call again for the full list"`
	ScannedAt   string         `json:"scanned_at,omitempty"`
	Demos       storageListOut `json:"demos" jsonschema:"Demos en tu PC: .dem files outside Studio"`
	StudioDemos storageListOut `json:"studio_demos" jsonschema:"Copias de demos en Studio: removed with delete_demo, not delete_storage_file"`
	Videos      storageListOut `json:"videos" jsonschema:"Vídeos creados por Studio"`
}

type jobApprovalIn struct {
	JobID   string `json:"job_id" jsonschema:"job_id from storage_usage"`
	Approve string `json:"approve,omitempty" jsonschema:"only when asked: the approval code from a previous call, after the user agreed"`
}

type pathIn struct {
	Path string `json:"path" jsonschema:"path from list_storage_files"`
}

type deleteFileIn struct {
	pathIn
	Approve string `json:"approve,omitempty" jsonschema:"only when asked: the approval code from a previous call, after the user agreed"`
}

type freedOut struct {
	NeedsApproval bool   `json:"needs_approval,omitempty"`
	ApprovalCode  string `json:"approval_code,omitempty"`
	Summary       string `json:"summary"`
	Deleted       bool   `json:"deleted,omitempty"`
	FreedBytes    int64  `json:"freed_bytes,omitempty"`
}

func registerStorageTools(server *mcp.Server, s *studio) {
	readOnly := &mcp.ToolAnnotations{ReadOnlyHint: true}
	destructive := &mcp.ToolAnnotations{DestructiveHint: new(true)}

	mcp.AddTool(server, &mcp.Tool{Name: "storage_usage", Annotations: readOnly,
		Description: "Disk used by Studio's jobs, and the Full Demo captures that can be freed, as Almacenamiento shows."},
		func(ctx context.Context, _ *mcp.CallToolRequest, _ empty) (*mcp.CallToolResult, storageUsageOut, error) {
			c, err := s.client(ctx)
			if err != nil {
				return nil, storageUsageOut{}, toolError(err)
			}
			usage, err := c.Storage(ctx)
			if err != nil {
				return nil, storageUsageOut{}, toolError(err)
			}
			return nil, describeStorageUsage(usage), nil
		})

	mcp.AddTool(server, &mcp.Tool{Name: "delete_captures", Annotations: destructive,
		Description: "Free the CS2 recordings of one Full Demo (Borrar capturas in Almacenamiento). The finished long video stays, but editing and rendering it again needs a new capture. " +
			"It cannot be undone, so the user must approve it. " + deleteApprovalHint},
		func(ctx context.Context, req *mcp.CallToolRequest, in jobApprovalIn) (*mcp.CallToolResult, freedOut, error) {
			if err := checkUUID("job_id", in.JobID); err != nil {
				return nil, freedOut{}, err
			}
			c, err := s.client(ctx)
			if err != nil {
				return nil, freedOut{}, toolError(err)
			}
			usage, err := c.Storage(ctx)
			if err != nil {
				return nil, freedOut{}, toolError(err)
			}
			i := slices.IndexFunc(usage.Jobs, func(j tuiclient.JobStorage) bool { return j.ID == in.JobID })
			if i < 0 || !usage.Jobs[i].Freeable {
				return nil, freedOut{}, errors.New("this job has no captures to free: storage_usage lists the ones that can go")
			}
			summary := fmt.Sprintf("Borrar %s de grabaciones de CS2 de esta demo completa. El vídeo final se queda. "+
				"Para editarlo y renderizarlo de nuevo habrá que volver a capturar.", formatBytes(usage.Jobs[i].CaptureBytes))
			return approveAndFree(req, approvalSpec{Tool: "delete_captures", JobID: in.JobID, Request: usage.Jobs[i].CaptureBytes, Summary: summary, Given: in.Approve, Question: "¿Borrar ahora?"},
				func() (int64, error) { return c.DeleteJobCaptures(ctx, in.JobID) })
		})

	mcp.AddTool(server, &mcp.Tool{Name: "list_storage_files", Annotations: readOnly,
		Description: "List the demos on the PC, Studio's demo copies and the videos Studio made, newest first, as Almacenamiento shows. " +
			"The first call starts a search of every drive; while scanning is true, call again for the full list of PC demos."},
		func(ctx context.Context, _ *mcp.CallToolRequest, in storageFilesIn) (*mcp.CallToolResult, storageFilesOut, error) {
			limit := cmp.Or(in.Limit, defaultStorageFiles)
			if limit < 1 || limit > maxStorageFiles {
				return nil, storageFilesOut{}, fmt.Errorf("limit must be 1 to %d", maxStorageFiles)
			}
			c, err := s.client(ctx)
			if err != nil {
				return nil, storageFilesOut{}, toolError(err)
			}
			files, err := c.ListStorageFiles(ctx)
			if err != nil {
				return nil, storageFilesOut{}, toolError(err)
			}
			return nil, describeStorageFiles(files, limit), nil
		})

	mcp.AddTool(server, &mcp.Tool{Name: "rescan_storage_files",
		Description: "Search the PC for demos again (Buscar de nuevo), in the background. Follow with list_storage_files until scanning is false."},
		func(ctx context.Context, _ *mcp.CallToolRequest, _ empty) (*mcp.CallToolResult, storageFilesOut, error) {
			c, err := s.client(ctx)
			if err != nil {
				return nil, storageFilesOut{}, toolError(err)
			}
			if err := c.RescanStorageFiles(ctx); err != nil {
				return nil, storageFilesOut{}, toolError(err)
			}
			files, err := c.ListStorageFiles(ctx)
			if err != nil {
				return nil, storageFilesOut{}, toolError(err)
			}
			return nil, describeStorageFiles(files, defaultStorageFiles), nil
		})

	mcp.AddTool(server, &mcp.Tool{Name: "delete_storage_file", Annotations: destructive,
		Description: "Delete one demo or video from the PC for good (Borrar in Almacenamiento), with a video's cover and captions. " +
			"Studio's own demo copies go with their match through delete_demo instead. It cannot be undone, so the user must approve it. " + deleteApprovalHint},
		func(ctx context.Context, req *mcp.CallToolRequest, in deleteFileIn) (*mcp.CallToolResult, freedOut, error) {
			c, file, err := listedFile(ctx, s, in.Path)
			if err != nil {
				return nil, freedOut{}, err
			}
			if file.Kind == "studio_demo" {
				return nil, freedOut{}, errors.New("Studio keeps this demo copy while its match exists: it goes when the match is removed (delete_demo for a library match)")
			}
			summary := fmt.Sprintf("Borrar del PC %s %s (%s). No se puede deshacer.",
				cmp.Or(storageKindLabels[file.Kind], "el archivo"), cleanName(filepath.Base(file.Path)), formatBytes(file.SizeBytes))
			return approveAndFree(req, approvalSpec{Tool: "delete_storage_file", JobID: file.Path, Request: [2]any{file.SizeBytes, file.ModifiedAt}, Summary: summary, Given: in.Approve, Question: "¿Borrar ahora?"},
				func() (int64, error) { return c.DeleteStorageFile(ctx, file.Path) })
		})

	mcp.AddTool(server, &mcp.Tool{Name: "reveal_storage_file",
		Description: "Open File Explorer on this PC at one file from list_storage_files (Mostrar en la carpeta)."},
		func(ctx context.Context, _ *mcp.CallToolRequest, in pathIn) (*mcp.CallToolResult, empty, error) {
			c, file, err := listedFile(ctx, s, in.Path)
			if err != nil {
				return nil, empty{}, err
			}
			return nil, empty{}, toolError(c.RevealStorageFile(ctx, file.Path))
		})
}

// listedFile refreshes the list, which the server checks a path against, and finds path in it.
func listedFile(ctx context.Context, s *studio, path string) (*tuiclient.Client, tuiclient.StorageFile, error) {
	if err := checkLocalPath(path); err != nil {
		return nil, tuiclient.StorageFile{}, err
	}
	c, err := s.client(ctx)
	if err != nil {
		return nil, tuiclient.StorageFile{}, toolError(err)
	}
	files, err := c.ListStorageFiles(ctx)
	if err != nil {
		return nil, tuiclient.StorageFile{}, toolError(err)
	}
	for _, list := range [][]tuiclient.StorageFile{files.Demos, files.StudioDemos, files.Videos} {
		if i := slices.IndexFunc(list, func(f tuiclient.StorageFile) bool { return strings.EqualFold(f.Path, filepath.Clean(path)) }); i >= 0 {
			return c, list[i], nil
		}
	}
	return nil, tuiclient.StorageFile{}, errors.New("that path is not in list_storage_files; list the files again and use a path from it")
}

func checkLocalPath(path string) error {
	if path == "" || len([]rune(path)) > maxPathRunes || !filepath.IsAbs(path) || strings.HasPrefix(path, `\\`) ||
		strings.ContainsFunc(path, func(r rune) bool { return unicode.IsControl(r) }) {
		return errors.New("path must be an absolute local path from list_storage_files")
	}
	return nil
}

func approveAndFree(req *mcp.CallToolRequest, spec approvalSpec, remove func() (int64, error)) (*mcp.CallToolResult, freedOut, error) {
	var freed int64
	ask, out, err := approveAndDelete(req, spec, func() error {
		var err error
		freed, err = remove()
		return err
	})
	return ask, freedOut{NeedsApproval: out.NeedsApproval, ApprovalCode: out.ApprovalCode, Summary: out.Summary, Deleted: out.Deleted, FreedBytes: freed}, err
}

func describeStorageUsage(usage tuiclient.StorageUsage) storageUsageOut {
	out := storageUsageOut{TotalBytes: usage.TotalBytes, FreeableCaptures: []freeableCaptures{}}
	for _, j := range usage.Jobs {
		if j.Freeable && checkUUID("job_id", j.ID) == nil {
			out.FreeableCaptures = append(out.FreeableCaptures, freeableCaptures{JobID: j.ID, CaptureBytes: j.CaptureBytes, RenderBytes: j.RenderBytes})
		}
	}
	slices.SortFunc(out.FreeableCaptures, func(a, b freeableCaptures) int { return cmp.Compare(b.CaptureBytes, a.CaptureBytes) })
	return out
}

func describeStorageFiles(files tuiclient.StorageFiles, limit int) storageFilesOut {
	out := storageFilesOut{DataDir: files.DataDir, Scanning: files.Scanning,
		Demos: describeStorageList(files.Demos, limit), StudioDemos: describeStorageList(files.StudioDemos, limit), Videos: describeStorageList(files.Videos, limit)}
	if files.ScannedAt != nil {
		out.ScannedAt = files.ScannedAt.Local().Format("2006-01-02 15:04")
	}
	return out
}

func describeStorageList(files []tuiclient.StorageFile, limit int) storageListOut {
	sorted := slices.Clone(files)
	slices.SortFunc(sorted, func(a, b tuiclient.StorageFile) int { return b.ModifiedAt.Compare(a.ModifiedAt) })
	out := storageListOut{Files: []storageFileOut{}, Total: len(files)}
	for _, f := range sorted {
		out.TotalBytes += f.SizeBytes
		// A name with control or format characters could hide text; it stays out of the reply.
		if len(out.Files) == limit || cleanText(f.Path, maxPathRunes) != f.Path {
			continue
		}
		out.Files = append(out.Files, storageFileOut{Path: f.Path, SizeBytes: f.SizeBytes, ModifiedAt: f.ModifiedAt.Local().Format("2006-01-02 15:04"),
			Kind: cleanName(f.Kind), JobID: cleanName(f.JobID)})
	}
	return out
}

// formatBytes is the Spanish size the Studio UI shows, such as "1,4 GB".
func formatBytes(n int64) string {
	if n <= 0 {
		return "0 B"
	}
	units := []string{"B", "KB", "MB", "GB", "TB"}
	v, i := float64(n), 0
	// Compare the rounded value so 1023,96 MB reads "1 GB", as the UI does.
	for i < len(units)-1 && math.Round(v*10)/10 >= 1024 {
		v /= 1024
		i++
	}
	if i == 0 {
		return fmt.Sprintf("%d B", n)
	}
	return strings.Replace(strings.TrimSuffix(fmt.Sprintf("%.1f", v), ".0"), ".", ",", 1) + " " + units[i]
}
