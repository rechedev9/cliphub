package timelineplan

import (
	"fmt"
	"path"

	"github.com/google/uuid"
)

func ProjectPrefix(id uuid.UUID) string {
	return path.Join("editor-jobs", id.String())
}

func PlanKey(id uuid.UUID) string {
	return path.Join(ProjectPrefix(id), "timeline.json")
}

func RenderPrefix(id uuid.UUID) string {
	return path.Join(ProjectPrefix(id), "renders")
}

func RenderStateKey(id uuid.UUID) string {
	return path.Join(RenderPrefix(id), "status.json")
}

func ProgressKey(id uuid.UUID) string {
	return path.Join(RenderPrefix(id), "progress.json")
}

func RenderRevisionPrefix(id, revision uuid.UUID) (string, error) {
	if revision == uuid.Nil {
		return "", fmt.Errorf("editor render revision id is required")
	}
	return path.Join(RenderPrefix(id), "revisions", revision.String()), nil
}

func RenderRevisionVideoKey(id, revision uuid.UUID) (string, error) {
	prefix, err := RenderRevisionPrefix(id, revision)
	if err != nil {
		return "", err
	}
	return path.Join(prefix, "final.mp4"), nil
}

func RenderRevisionCoverKey(id, revision uuid.UUID) (string, error) {
	prefix, err := RenderRevisionPrefix(id, revision)
	if err != nil {
		return "", err
	}
	return path.Join(prefix, "cover.jpg"), nil
}

func RenderRevisionResultKey(id, revision uuid.UUID) (string, error) {
	prefix, err := RenderRevisionPrefix(id, revision)
	if err != nil {
		return "", err
	}
	return path.Join(prefix, "render-result.json"), nil
}

func RenderRevisionDeliveryDir(id, revision uuid.UUID) (string, error) {
	prefix, err := RenderRevisionPrefix(id, revision)
	if err != nil {
		return "", err
	}
	return path.Join(prefix, "shortslistosparasubir"), nil
}
