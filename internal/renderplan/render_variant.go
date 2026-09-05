package renderplan

import (
	"path"
	"slices"
	"time"

	"github.com/google/uuid"

	"github.com/rechedev9/cliphub/internal/artifacts"
	"github.com/rechedev9/cliphub/internal/recapplan"
)

const (
	RenderVariantStatusQueued    = "queued"
	RenderVariantStatusRendering = "rendering"
	RenderVariantStatusReady     = "ready"
	RenderVariantStatusReview    = "review_required"
	RenderVariantStatusFailed    = "failed"
)

// RenderVariantState is the durable product-level state for one materialized
// output variant. It intentionally stores artifact keys, not local paths.
type RenderVariantState struct {
	FullDemo          *recapplan.Snapshot     `json:"full_demo,omitempty"`
	SchemaVersion     string                  `json:"schema_version"`
	JobID             uuid.UUID               `json:"job_id"`
	Variant           string                  `json:"variant"`
	Status            string                  `json:"status"`
	Preset            string                  `json:"preset,omitempty"`
	EditDocumentKey   string                  `json:"edit_document_key,omitempty"`
	EditManifestKey   string                  `json:"edit_manifest_key,omitempty"`
	RenderResultKey   string                  `json:"render_result_key,omitempty"`
	PackManifestKey   string                  `json:"pack_manifest_key,omitempty"`
	GalleryKey        string                  `json:"gallery_key,omitempty"`
	PublishSummaryKey string                  `json:"publish_summary_key,omitempty"`
	ArtifactPrefix    string                  `json:"artifact_prefix,omitempty"`
	Warnings          []string                `json:"warnings,omitempty"`
	ReviewResolution  *RenderReviewResolution `json:"review_resolution,omitempty"`
	Error             string                  `json:"error,omitempty"`
	CreatedAt         time.Time               `json:"created_at"`
	UpdatedAt         time.Time               `json:"updated_at"`
}

// RenderReviewResolution is the durable audit trail for warnings a human
// inspected and accepted as intentional. ArtifactPrefix and Warnings bind the
// decision to one exact render revision; a later render cannot inherit it.
type RenderReviewResolution struct {
	ArtifactPrefix string    `json:"artifact_prefix"`
	Warnings       []string  `json:"warnings"`
	Note           string    `json:"note"`
	ReviewedAt     time.Time `json:"reviewed_at"`
}

// ReviewResolvedFor reports whether the current render's exact warnings were
// explicitly reviewed on this artifact revision.
func (s RenderVariantState) ReviewResolvedFor(warnings []string) bool {
	resolution := s.ReviewResolution
	return resolution != nil &&
		resolution.ArtifactPrefix == s.ArtifactPrefix &&
		slices.Equal(resolution.Warnings, warnings)
}

type NewRenderVariantStateOptions struct {
	FullDemo          *recapplan.Snapshot
	JobID             uuid.UUID
	Variant           string
	Status            string
	Preset            string
	EditDocumentKey   string
	EditManifestKey   string
	RenderResultKey   string
	PackManifestKey   string
	GalleryKey        string
	PublishSummaryKey string
	ArtifactPrefix    string
	Warnings          []string
	Error             string
	Now               time.Time
	Previous          *RenderVariantState
}

// NewRenderVariantStateForLoadoutOptions carries the product loadout and
// mutable status fields needed to materialize a durable render state.
type NewRenderVariantStateForLoadoutOptions struct {
	FullDemo   *recapplan.Snapshot
	JobID      uuid.UUID
	Loadout    Loadout
	Status     string
	Warnings   []string
	Error      string
	Now        time.Time
	Previous   *RenderVariantState
	RevisionID uuid.UUID
}

type renderVariantArtifacts struct {
	Prefix            string
	RenderResultKey   string
	EditDocumentKey   string
	EditManifestKey   string
	PackManifestKey   string
	GalleryKey        string
	PublishSummaryKey string
}

func renderVariantArtifactsFor(jobID uuid.UUID, variant string) (renderVariantArtifacts, error) {
	prefix, err := artifacts.RenderVariantPrefix(jobID, variant)
	if err != nil {
		return renderVariantArtifacts{}, err
	}
	resultKey, err := artifacts.RenderVariantResultKey(jobID, variant)
	if err != nil {
		return renderVariantArtifacts{}, err
	}
	editDocumentKey, err := artifacts.RenderVariantEditDocumentKey(jobID, variant)
	if err != nil {
		return renderVariantArtifacts{}, err
	}
	editManifestKey, err := artifacts.RenderVariantEditManifestKey(jobID, variant)
	if err != nil {
		return renderVariantArtifacts{}, err
	}
	packKey, err := artifacts.RenderVariantPackManifestKey(jobID, variant)
	if err != nil {
		return renderVariantArtifacts{}, err
	}
	galleryKey, err := artifacts.RenderVariantGalleryKey(jobID, variant)
	if err != nil {
		return renderVariantArtifacts{}, err
	}
	summaryKey, err := artifacts.RenderVariantPublishSummaryKey(jobID, variant)
	if err != nil {
		return renderVariantArtifacts{}, err
	}
	return renderVariantArtifacts{
		Prefix:            prefix,
		RenderResultKey:   resultKey,
		EditDocumentKey:   editDocumentKey,
		EditManifestKey:   editManifestKey,
		PackManifestKey:   packKey,
		GalleryKey:        galleryKey,
		PublishSummaryKey: summaryKey,
	}, nil
}

func renderVariantArtifactsForRevision(jobID uuid.UUID, variant string, revisionID uuid.UUID) (renderVariantArtifacts, error) {
	prefix, err := RenderVariantRevisionPrefix(jobID, variant, revisionID)
	if err != nil {
		return renderVariantArtifacts{}, err
	}
	return renderVariantArtifacts{
		Prefix:            prefix,
		RenderResultKey:   path.Join(prefix, "render-result.json"),
		EditDocumentKey:   path.Join(prefix, "edit-document.json"),
		EditManifestKey:   path.Join(prefix, "edit-manifest.json"),
		PackManifestKey:   path.Join(prefix, "pack-manifest.json"),
		GalleryKey:        path.Join(prefix, "index.html"),
		PublishSummaryKey: path.Join(prefix, "publish-summary.md"),
	}, nil
}

// RenderVariantStateKey returns the durable storage key for the render
// variant state document.
func RenderVariantStateKey(jobID uuid.UUID, variant string) (string, error) {
	return artifacts.RenderVariantStatusKey(jobID, variant)
}

// NewRenderVariantStateForLoadout derives artifact keys from the loadout's
// variant and returns the durable render state document for API and worker
// boundaries.
func NewRenderVariantStateForLoadout(opts NewRenderVariantStateForLoadoutOptions) (RenderVariantState, error) {
	var refs renderVariantArtifacts
	var err error
	if opts.RevisionID != uuid.Nil {
		refs, err = renderVariantArtifactsForRevision(opts.JobID, opts.Loadout.Variant, opts.RevisionID)
	} else {
		refs, err = renderVariantArtifactsFor(opts.JobID, opts.Loadout.Variant)
	}
	if err != nil {
		return RenderVariantState{}, err
	}
	if opts.RevisionID == uuid.Nil &&
		opts.Previous != nil &&
		opts.Previous.ArtifactPrefix != "" &&
		(opts.Status == RenderVariantStatusQueued ||
			opts.Status == RenderVariantStatusRendering ||
			opts.Status == RenderVariantStatusFailed) {
		// A pending/failed replacement still points at the last committed
		// revision. This keeps the old reel addressable until a new immutable
		// revision commits and gives the worker the exact prefix to retire.
		refs = renderVariantArtifacts{
			Prefix:            opts.Previous.ArtifactPrefix,
			RenderResultKey:   opts.Previous.RenderResultKey,
			EditDocumentKey:   opts.Previous.EditDocumentKey,
			EditManifestKey:   opts.Previous.EditManifestKey,
			PackManifestKey:   opts.Previous.PackManifestKey,
			GalleryKey:        opts.Previous.GalleryKey,
			PublishSummaryKey: opts.Previous.PublishSummaryKey,
		}
	}
	return NewRenderVariantState(NewRenderVariantStateOptions{
		FullDemo:          opts.FullDemo,
		JobID:             opts.JobID,
		Variant:           opts.Loadout.Variant,
		Status:            opts.Status,
		Preset:            opts.Loadout.Preset,
		EditDocumentKey:   refs.EditDocumentKey,
		EditManifestKey:   refs.EditManifestKey,
		RenderResultKey:   refs.RenderResultKey,
		PackManifestKey:   refs.PackManifestKey,
		GalleryKey:        refs.GalleryKey,
		PublishSummaryKey: refs.PublishSummaryKey,
		ArtifactPrefix:    refs.Prefix,
		Warnings:          opts.Warnings,
		Error:             opts.Error,
		Now:               opts.Now,
		Previous:          opts.Previous,
	}), nil
}

func NewRenderVariantState(opts NewRenderVariantStateOptions) RenderVariantState {
	now := opts.Now.UTC()
	if now.IsZero() {
		now = time.Now().UTC()
	}
	createdAt := now
	if opts.Previous != nil && !opts.Previous.CreatedAt.IsZero() {
		createdAt = opts.Previous.CreatedAt
	}
	warnings := append([]string(nil), opts.Warnings...)
	fullDemo := opts.FullDemo
	if fullDemo == nil && opts.Status != RenderVariantStatusQueued && opts.Previous != nil {
		fullDemo = opts.Previous.FullDemo
	}
	return RenderVariantState{
		FullDemo:          fullDemo,
		SchemaVersion:     "1.0",
		JobID:             opts.JobID,
		Variant:           opts.Variant,
		Status:            opts.Status,
		Preset:            opts.Preset,
		EditDocumentKey:   opts.EditDocumentKey,
		EditManifestKey:   opts.EditManifestKey,
		RenderResultKey:   opts.RenderResultKey,
		PackManifestKey:   opts.PackManifestKey,
		GalleryKey:        opts.GalleryKey,
		PublishSummaryKey: opts.PublishSummaryKey,
		ArtifactPrefix:    opts.ArtifactPrefix,
		Warnings:          warnings,
		Error:             opts.Error,
		CreatedAt:         createdAt,
		UpdatedAt:         now,
	}
}
