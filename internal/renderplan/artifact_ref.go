package renderplan

import (
	"fmt"
	"path"
	"strings"

	"github.com/google/uuid"

	"github.com/rechedev9/cliphub/internal/artifacts"
)

type RenderVariantArtifactKind string

const (
	RenderVariantArtifactResult            RenderVariantArtifactKind = "result"
	RenderVariantArtifactPackManifest      RenderVariantArtifactKind = "pack-manifest"
	RenderVariantArtifactEditDocument      RenderVariantArtifactKind = "edit-document"
	RenderVariantArtifactGallery           RenderVariantArtifactKind = "gallery"
	RenderVariantArtifactVideo             RenderVariantArtifactKind = "video"
	RenderVariantArtifactCover             RenderVariantArtifactKind = "cover"
	RenderVariantArtifactCaption           RenderVariantArtifactKind = "caption"
	RenderVariantArtifactFullDemoApproved  RenderVariantArtifactKind = "full-demo-approved"
	RenderVariantArtifactFullDemoEffective RenderVariantArtifactKind = "full-demo-effective"
	RenderVariantArtifactFullDemoAudio     RenderVariantArtifactKind = "full-demo-audio"
	RenderVariantArtifactFullDemoLoudness  RenderVariantArtifactKind = "full-demo-loudness"
	RenderVariantArtifactFullDemoDelivery  RenderVariantArtifactKind = "full-demo-delivery"
)

// FullDemoArtifactKind allowlists public editorial evidence in a render revision.
func FullDemoArtifactKind(name string) (RenderVariantArtifactKind, bool) {
	kind := RenderVariantArtifactKind("full-demo-" + name)
	switch kind {
	case RenderVariantArtifactFullDemoApproved, RenderVariantArtifactFullDemoEffective, RenderVariantArtifactFullDemoAudio, RenderVariantArtifactFullDemoLoudness, RenderVariantArtifactFullDemoDelivery:
		return kind, true
	default:
		return "", false
	}
}

// RenderVariantArtifactRef identifies one durable render-variant artifact.
type RenderVariantArtifactRef struct {
	Kind      RenderVariantArtifactKind
	Key       string
	SegmentID string
}

// NewRenderVariantArtifactRef derives the durable storage key for one
// render-variant artifact. Segment artifacts require a non-empty segment ID.
func NewRenderVariantArtifactRef(jobID uuid.UUID, variant string, kind RenderVariantArtifactKind, segmentID string) (RenderVariantArtifactRef, error) {
	prefix, err := artifacts.RenderVariantPrefix(jobID, variant)
	if err != nil {
		return RenderVariantArtifactRef{}, err
	}
	return renderVariantArtifactRefAtPrefix(jobID, variant, prefix, kind, segmentID)
}

// RenderVariantRevisionPrefix returns the immutable namespace for one complete
// render attempt. status.json remains outside revisions as the atomic current
// pointer.
func RenderVariantRevisionPrefix(jobID uuid.UUID, variant string, revisionID uuid.UUID) (string, error) {
	prefix, err := artifacts.RenderVariantPrefix(jobID, variant)
	if err != nil {
		return "", err
	}
	if revisionID == uuid.Nil {
		return "", fmt.Errorf("render revision id is required")
	}
	return path.Join(prefix, "revisions", revisionID.String()), nil
}

func NewRenderVariantRevisionArtifactRef(jobID uuid.UUID, variant string, revisionID uuid.UUID, kind RenderVariantArtifactKind, segmentID string) (RenderVariantArtifactRef, error) {
	prefix, err := RenderVariantRevisionPrefix(jobID, variant, revisionID)
	if err != nil {
		return RenderVariantArtifactRef{}, err
	}
	return renderVariantArtifactRefAtPrefix(jobID, variant, prefix, kind, segmentID)
}

// NewRenderVariantArtifactRefForState resolves the artifact selected by the
// durable current pointer, while accepting legacy canonical states.
func NewRenderVariantArtifactRefForState(state RenderVariantState, kind RenderVariantArtifactKind, segmentID string) (RenderVariantArtifactRef, error) {
	prefix := state.ArtifactPrefix
	if prefix == "" {
		var err error
		prefix, err = artifacts.RenderVariantPrefix(state.JobID, state.Variant)
		if err != nil {
			return RenderVariantArtifactRef{}, err
		}
	}
	if err := validateRenderArtifactPrefix(state.JobID, state.Variant, prefix); err != nil {
		return RenderVariantArtifactRef{}, err
	}
	return renderVariantArtifactRefAtPrefix(state.JobID, state.Variant, prefix, kind, segmentID)
}

func renderVariantArtifactRefAtPrefix(jobID uuid.UUID, variant, prefix string, kind RenderVariantArtifactKind, segmentID string) (RenderVariantArtifactRef, error) {
	var (
		key string
		err error
	)
	switch kind {
	case RenderVariantArtifactResult:
		key = path.Join(prefix, "render-result.json")
	case RenderVariantArtifactPackManifest:
		key = path.Join(prefix, "pack-manifest.json")
	case RenderVariantArtifactEditDocument:
		key = path.Join(prefix, "edit-document.json")
	case RenderVariantArtifactGallery:
		key = path.Join(prefix, "index.html")
	case RenderVariantArtifactFullDemoApproved, RenderVariantArtifactFullDemoEffective, RenderVariantArtifactFullDemoAudio, RenderVariantArtifactFullDemoLoudness, RenderVariantArtifactFullDemoDelivery:
		key = path.Join(prefix, string(kind)+".json")
	case RenderVariantArtifactVideo, RenderVariantArtifactCover, RenderVariantArtifactCaption:
		if err = artifacts.ValidateArtifactToken("artifact name", segmentID); err == nil {
			switch kind {
			case RenderVariantArtifactVideo:
				key = path.Join(prefix, "videos", segmentID+".mp4")
			case RenderVariantArtifactCover:
				key = path.Join(prefix, "covers", segmentID+".jpg")
			case RenderVariantArtifactCaption:
				key = path.Join(prefix, "captions", segmentID+".caption.txt")
			}
		}
	default:
		err = fmt.Errorf("unknown render artifact kind %q", kind)
	}
	if err != nil {
		return RenderVariantArtifactRef{}, err
	}
	return RenderVariantArtifactRef{
		Kind:      kind,
		Key:       key,
		SegmentID: segmentID,
	}, nil
}

func validateRenderArtifactPrefix(jobID uuid.UUID, variant, prefix string) error {
	base, err := artifacts.RenderVariantPrefix(jobID, variant)
	if err != nil {
		return err
	}
	if prefix == base {
		return nil
	}
	revisionText := strings.TrimPrefix(prefix, base+"/revisions/")
	if revisionText == prefix || revisionText == "" || strings.Contains(revisionText, "/") {
		return fmt.Errorf("render artifact prefix is outside the job revision namespace")
	}
	revisionID, err := uuid.Parse(revisionText)
	if err != nil || revisionID == uuid.Nil {
		return fmt.Errorf("render artifact prefix has an invalid revision id")
	}
	want, err := RenderVariantRevisionPrefix(jobID, variant, revisionID)
	if err != nil || want != prefix {
		return fmt.Errorf("render artifact prefix does not match its revision")
	}
	return nil
}

func renderVariantLogArtifactKey(jobID uuid.UUID, variant, segmentID string) (string, error) {
	return artifacts.RenderVariantLogKey(jobID, variant, segmentID+"-render")
}
