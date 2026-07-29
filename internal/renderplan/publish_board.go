package renderplan

import (
	"fmt"
	"time"

	"github.com/google/uuid"
)

const PublishBoardSchemaVersion = "1.0"

type PublishBoard struct {
	SchemaVersion   string             `json:"schema_version"`
	JobID           uuid.UUID          `json:"job_id"`
	Variant         string             `json:"variant"`
	Status          string             `json:"status"`
	UploadReadyRoot string             `json:"upload_ready_root"`
	RenderReady     bool               `json:"render_ready"`
	CoversRequired  bool               `json:"covers_required"`
	RenderResultKey string             `json:"render_result_key,omitempty"`
	PackManifestKey string             `json:"pack_manifest_key,omitempty"`
	GalleryKey      string             `json:"gallery_key,omitempty"`
	PublishSummary  string             `json:"publish_summary_key,omitempty"`
	Items           []PublishBoardItem `json:"items"`
	Warnings        []string           `json:"warnings,omitempty"`
	Error           string             `json:"error,omitempty"`
	UpdatedAt       time.Time          `json:"updated_at"`
}

type PublishBoardItem struct {
	SegmentID     string `json:"segment_id"`
	Status        string `json:"status"`
	VideoKey      string `json:"video_key,omitempty"`
	CoverKey      string `json:"cover_key,omitempty"`
	CaptionKey    string `json:"caption_key,omitempty"`
	VideoReady    bool   `json:"video_ready"`
	CoverReady    bool   `json:"cover_ready"`
	CoverRequired bool   `json:"cover_required"`
	CaptionReady  bool   `json:"caption_ready"`
}

type NewPublishBoardOptions struct {
	JobID           uuid.UUID
	Variant         string
	UploadReadyRoot string
	RenderResultKey string
	PackManifestKey string
	GalleryKey      string
	PublishSummary  string
	Items           []PublishBoardItem
	Warnings        []string
	Error           string
	CoversRequired  bool
}

// ArtifactExistsFunc reports whether an artifact key currently exists.
type ArtifactExistsFunc func(key string) (bool, error)

// NewPublishBoardForVariantOptions carries the render result summary plus the
// storage readiness probe needed to build a publish board.
type NewPublishBoardForVariantOptions struct {
	JobID           uuid.UUID
	Variant         string
	UploadReadyRoot string
	SegmentIDs      []string
	Warnings        []string
	Error           string
	CoversRequired  bool
	ArtifactPrefix  string
	ArtifactExists  ArtifactExistsFunc
}

// NewPublishBoardForVariant derives the publish-board artifact keys for a
// render variant and probes each upload-ready item for storage readiness.
func NewPublishBoardForVariant(opts NewPublishBoardForVariantOptions) (PublishBoard, error) {
	refs, err := renderVariantArtifactsFor(opts.JobID, opts.Variant)
	if err != nil {
		return PublishBoard{}, err
	}
	state := RenderVariantState{JobID: opts.JobID, Variant: opts.Variant, ArtifactPrefix: opts.ArtifactPrefix}
	if opts.ArtifactPrefix != "" {
		resultRef, refErr := NewRenderVariantArtifactRefForState(state, RenderVariantArtifactResult, "")
		if refErr != nil {
			return PublishBoard{}, refErr
		}
		refs.Prefix = opts.ArtifactPrefix
		refs.RenderResultKey = resultRef.Key
		refs.PackManifestKey = opts.ArtifactPrefix + "/pack-manifest.json"
		refs.GalleryKey = opts.ArtifactPrefix + "/index.html"
		refs.PublishSummaryKey = opts.ArtifactPrefix + "/publish-summary.md"
	}
	exists := opts.ArtifactExists
	if exists == nil {
		exists = func(string) (bool, error) { return false, nil }
	}
	items := make([]PublishBoardItem, 0, len(opts.SegmentIDs))
	for _, segmentID := range opts.SegmentIDs {
		if segmentID == "" {
			continue
		}
		videoRef, err := publishBoardArtifactRef(state, RenderVariantArtifactVideo, segmentID)
		if err != nil {
			return PublishBoard{}, err
		}
		videoKey := videoRef.Key
		coverRef, err := publishBoardArtifactRef(state, RenderVariantArtifactCover, segmentID)
		if err != nil {
			return PublishBoard{}, err
		}
		coverKey := coverRef.Key
		captionRef, err := publishBoardArtifactRef(state, RenderVariantArtifactCaption, segmentID)
		if err != nil {
			return PublishBoard{}, err
		}
		captionKey := captionRef.Key
		videoReady, err := exists(videoKey)
		if err != nil {
			return PublishBoard{}, fmt.Errorf("check video artifact %s: %w", segmentID, err)
		}
		coverReady, err := exists(coverKey)
		if err != nil {
			return PublishBoard{}, fmt.Errorf("check cover artifact %s: %w", segmentID, err)
		}
		captionReady, err := exists(captionKey)
		if err != nil {
			return PublishBoard{}, fmt.Errorf("check caption artifact %s: %w", segmentID, err)
		}
		items = append(items, PublishBoardItem{
			SegmentID:     segmentID,
			VideoKey:      videoKey,
			CoverKey:      coverKey,
			CaptionKey:    captionKey,
			VideoReady:    videoReady,
			CoverReady:    coverReady,
			CoverRequired: opts.CoversRequired,
			CaptionReady:  captionReady,
		})
	}
	return NewPublishBoard(NewPublishBoardOptions{
		JobID:           opts.JobID,
		Variant:         opts.Variant,
		UploadReadyRoot: opts.UploadReadyRoot,
		RenderResultKey: refs.RenderResultKey,
		PackManifestKey: refs.PackManifestKey,
		GalleryKey:      refs.GalleryKey,
		PublishSummary:  refs.PublishSummaryKey,
		Items:           items,
		Warnings:        opts.Warnings,
		Error:           opts.Error,
		CoversRequired:  opts.CoversRequired,
	}), nil
}

func publishBoardArtifactRef(state RenderVariantState, kind RenderVariantArtifactKind, segmentID string) (RenderVariantArtifactRef, error) {
	if state.ArtifactPrefix != "" {
		return NewRenderVariantArtifactRefForState(state, kind, segmentID)
	}
	return NewRenderVariantArtifactRef(state.JobID, state.Variant, kind, segmentID)
}

func NewPublishBoard(opts NewPublishBoardOptions) PublishBoard {
	root := opts.UploadReadyRoot
	if root == "" {
		root = "shortslistosparasubir"
	}
	board := PublishBoard{
		SchemaVersion:   PublishBoardSchemaVersion,
		JobID:           opts.JobID,
		Variant:         opts.Variant,
		UploadReadyRoot: root,
		RenderResultKey: opts.RenderResultKey,
		PackManifestKey: opts.PackManifestKey,
		GalleryKey:      opts.GalleryKey,
		PublishSummary:  opts.PublishSummary,
		Items:           append([]PublishBoardItem(nil), opts.Items...),
		Warnings:        append([]string(nil), opts.Warnings...),
		Error:           opts.Error,
		CoversRequired:  opts.CoversRequired,
		UpdatedAt:       time.Now().UTC(),
	}
	for i := range board.Items {
		board.Items[i].CoverRequired = opts.CoversRequired
	}
	board.RenderReady, board.Status = summarizePublishBoard(board.Items, board.Warnings, board.Error)
	return board
}

func summarizePublishBoard(items []PublishBoardItem, warnings []string, resultError string) (bool, string) {
	if resultError != "" {
		return false, "failed"
	}
	if len(items) == 0 {
		return false, "draft"
	}
	allReady := true
	needsCover := false
	needsCaption := false
	for i := range items {
		item := &items[i]
		switch {
		case !item.VideoReady:
			item.Status = "missing_video"
			allReady = false
		case item.CoverRequired && !item.CoverReady:
			item.Status = "needs_cover"
			needsCover = true
			allReady = false
		case !item.CaptionReady:
			item.Status = "needs_caption"
			needsCaption = true
			allReady = false
		default:
			item.Status = "ready"
		}
	}
	switch {
	case needsCover:
		return false, "needs_cover"
	case needsCaption:
		return false, "needs_caption"
	case !allReady:
		return false, "draft"
	case len(warnings) > 0:
		return false, "review_required"
	default:
		return true, "ready"
	}
}
