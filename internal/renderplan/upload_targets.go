package renderplan

import (
	"fmt"
	"path/filepath"

	"github.com/google/uuid"

	"github.com/rechedev9/fragforge/internal/editor"
)

// RenderVariantUploadTarget maps one local render output file to its durable
// object-storage key.
type RenderVariantUploadTarget struct {
	Key      string
	Path     string
	Label    string
	Required bool
}

// RenderVariantReadyArtifacts names the durable artifacts that must already
// exist for a render variant to be considered reusable.
type RenderVariantReadyArtifacts struct {
	ResultKey    string
	RequiredKeys []string
}

// NewRenderVariantUploadTargetsOptions carries local render outputs and the
// rendered result metadata needed to plan durable uploads.
type NewRenderVariantUploadTargetsOptions struct {
	JobID      uuid.UUID
	Variant    string
	OutDir     string
	PublishDir string
	ResultPath string
	Result     editor.Result
	RevisionID uuid.UUID
}

// NewRenderVariantUploadTargets returns the ordered upload plan for one render
// variant. Every delivery contract is required, and the render result is last:
// its presence is the commit marker after all referenced artifacts exist.
func NewRenderVariantUploadTargets(opts NewRenderVariantUploadTargetsOptions) ([]RenderVariantUploadTarget, error) {
	var refs renderVariantArtifacts
	var err error
	if opts.RevisionID != uuid.Nil {
		refs, err = renderVariantArtifactsForRevision(opts.JobID, opts.Variant, opts.RevisionID)
	} else {
		refs, err = renderVariantArtifactsFor(opts.JobID, opts.Variant)
	}
	if err != nil {
		return nil, err
	}
	if err := ValidateRenderVariantRunResult(opts.Result); err != nil {
		return nil, err
	}
	targets := []RenderVariantUploadTarget{{
		Key:      refs.EditDocumentKey,
		Path:     filepath.Join(opts.OutDir, "edit-document.json"),
		Label:    "edit document",
		Required: true,
	}, {
		Key:      refs.EditManifestKey,
		Path:     filepath.Join(opts.OutDir, "edit-manifest.json"),
		Label:    "edit manifest",
		Required: true,
	}, {
		Key:      refs.PackManifestKey,
		Path:     filepath.Join(opts.PublishDir, "pack-manifest.json"),
		Label:    "pack manifest",
		Required: true,
	}, {
		Key:      refs.PublishSummaryKey,
		Path:     opts.Result.SummaryPath,
		Label:    "publish summary",
		Required: true,
	}, {
		Key:      refs.GalleryKey,
		Path:     opts.Result.GalleryPath,
		Label:    "gallery",
		Required: true,
	}}
	for _, short := range opts.Result.Shorts {
		if short.SegmentID == "" {
			continue
		}
		videoPath := short.PublishPath
		if videoPath == "" {
			videoPath = short.Output
		}
		if videoPath == "" {
			return nil, fmt.Errorf("render video %s has no path", short.SegmentID)
		}
		ref, err := renderVariantUploadArtifactRef(opts, RenderVariantArtifactVideo, short.SegmentID)
		if err != nil {
			return nil, err
		}
		targets = append(targets, RenderVariantUploadTarget{
			Key:      ref.Key,
			Path:     videoPath,
			Label:    "render video " + short.SegmentID,
			Required: true,
		})
		if opts.Result.CoversEnabled && short.CoverPath == "" {
			return nil, fmt.Errorf("render cover %s has no path", short.SegmentID)
		}
		if short.CoverPath != "" {
			ref, err := renderVariantUploadArtifactRef(opts, RenderVariantArtifactCover, short.SegmentID)
			if err != nil {
				return nil, err
			}
			targets = append(targets, RenderVariantUploadTarget{
				Key:      ref.Key,
				Path:     short.CoverPath,
				Label:    "render cover " + short.SegmentID,
				Required: opts.Result.CoversEnabled,
			})
		}
		if short.CaptionPath == "" {
			return nil, fmt.Errorf("render caption %s has no path", short.SegmentID)
		}
		ref, err = renderVariantUploadArtifactRef(opts, RenderVariantArtifactCaption, short.SegmentID)
		if err != nil {
			return nil, err
		}
		targets = append(targets, RenderVariantUploadTarget{
			Key:      ref.Key,
			Path:     short.CaptionPath,
			Label:    "render caption " + short.SegmentID,
			Required: true,
		})
		if short.RenderLogPath != "" {
			var key string
			if opts.RevisionID != uuid.Nil {
				key = filepath.ToSlash(filepath.Join(refs.Prefix, "logs", short.SegmentID+"-render.log"))
			} else {
				key, err = renderVariantLogArtifactKey(opts.JobID, opts.Variant, short.SegmentID)
			}
			if err != nil {
				return nil, err
			}
			targets = append(targets, RenderVariantUploadTarget{
				Key:   key,
				Path:  short.RenderLogPath,
				Label: "render log " + short.SegmentID,
			})
		}
	}
	targets = append(targets, RenderVariantUploadTarget{
		Key:      refs.RenderResultKey,
		Path:     opts.ResultPath,
		Label:    "render result",
		Required: true,
	})
	return targets, nil
}

func renderVariantUploadArtifactRef(opts NewRenderVariantUploadTargetsOptions, kind RenderVariantArtifactKind, segmentID string) (RenderVariantArtifactRef, error) {
	if opts.RevisionID != uuid.Nil {
		return NewRenderVariantRevisionArtifactRef(opts.JobID, opts.Variant, opts.RevisionID, kind, segmentID)
	}
	return NewRenderVariantArtifactRef(opts.JobID, opts.Variant, kind, segmentID)
}

// NewRenderVariantReadyArtifacts returns the minimal durable artifacts that
// prove a render variant is already materialized enough to skip rerendering.
func NewRenderVariantReadyArtifacts(jobID uuid.UUID, variant string) (RenderVariantReadyArtifacts, error) {
	refs, err := renderVariantArtifactsFor(jobID, variant)
	if err != nil {
		return RenderVariantReadyArtifacts{}, err
	}
	return RenderVariantReadyArtifacts{
		ResultKey: refs.RenderResultKey,
		RequiredKeys: []string{
			refs.PackManifestKey,
			refs.GalleryKey,
		},
	}, nil
}
