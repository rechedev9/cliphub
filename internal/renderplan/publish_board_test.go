package renderplan

import (
	"testing"

	"github.com/google/uuid"

	"github.com/rechedev9/fragforge/internal/editor"
)

func TestNewPublishBoardMarksReadyWhenAllItemsReady(t *testing.T) {
	board := NewPublishBoard(NewPublishBoardOptions{
		JobID:          uuid.New(),
		Variant:        "viral-60-clean",
		CoversRequired: true,
		Items: []PublishBoardItem{{
			SegmentID:    "seg-001",
			VideoReady:   true,
			CoverReady:   true,
			CaptionReady: true,
		}},
	})

	if board.Status != "ready" || !board.RenderReady {
		t.Fatalf("status/render_ready = %q/%v, want ready/true", board.Status, board.RenderReady)
	}
	if board.Items[0].Status != "ready" {
		t.Fatalf("item status = %q, want ready", board.Items[0].Status)
	}
}

func TestNewPublishBoardSurfacesMissingCoverBeforeCaption(t *testing.T) {
	board := NewPublishBoard(NewPublishBoardOptions{
		JobID:          uuid.New(),
		Variant:        "viral-60-clean",
		CoversRequired: true,
		Items: []PublishBoardItem{{
			SegmentID:    "seg-001",
			VideoReady:   true,
			CaptionReady: false,
		}},
	})

	if board.Status != "needs_cover" {
		t.Fatalf("status = %q, want needs_cover", board.Status)
	}
	if board.Items[0].Status != "needs_cover" {
		t.Fatalf("item status = %q, want needs_cover", board.Items[0].Status)
	}
}

func TestNewPublishBoardMarksNeedsCaption(t *testing.T) {
	board := NewPublishBoard(NewPublishBoardOptions{
		JobID:   uuid.New(),
		Variant: "viral-60-clean",
		Items: []PublishBoardItem{{
			SegmentID:    "seg-001",
			VideoReady:   true,
			CoverReady:   true,
			CaptionReady: false,
		}},
	})

	if board.Status != "needs_caption" {
		t.Fatalf("status = %q, want needs_caption", board.Status)
	}
}

func TestNewPublishBoardMarksFailedFromRenderError(t *testing.T) {
	board := NewPublishBoard(NewPublishBoardOptions{
		JobID:   uuid.New(),
		Variant: "viral-60-clean",
		Error:   "render failed",
		Items: []PublishBoardItem{{
			SegmentID: "seg-001",
		}},
	})

	if board.Status != "failed" || board.RenderReady {
		t.Fatalf("status/render_ready = %q/%v, want failed/false", board.Status, board.RenderReady)
	}
}

func TestNewPublishBoardRequiresReviewForWarnings(t *testing.T) {
	board := NewPublishBoard(NewPublishBoardOptions{
		JobID:    uuid.New(),
		Variant:  "viral-60-clean",
		Warnings: []string{"frozen frame at 00:07"},
		Items: []PublishBoardItem{{
			SegmentID:    "seg-001",
			VideoReady:   true,
			CaptionReady: true,
		}},
	})

	if board.Status != "review_required" || board.RenderReady {
		t.Fatalf("status/render_ready = %q/%v, want review_required/false", board.Status, board.RenderReady)
	}
}

func TestNewPublishBoardSurfacesMissingArtifactsBeforeWarnings(t *testing.T) {
	tests := []struct {
		name           string
		coversRequired bool
		item           PublishBoardItem
		wantBoard      string
		wantItem       string
	}{
		{
			name:      "missing video",
			item:      PublishBoardItem{CaptionReady: true},
			wantBoard: "draft",
			wantItem:  "missing_video",
		},
		{
			name:           "missing required cover",
			coversRequired: true,
			item:           PublishBoardItem{VideoReady: true, CaptionReady: true},
			wantBoard:      "needs_cover",
			wantItem:       "needs_cover",
		},
		{
			name:      "missing caption",
			item:      PublishBoardItem{VideoReady: true, CoverReady: true},
			wantBoard: "needs_caption",
			wantItem:  "needs_caption",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			board := NewPublishBoard(NewPublishBoardOptions{
				JobID:          uuid.New(),
				Variant:        "viral-60-clean",
				CoversRequired: test.coversRequired,
				Warnings:       []string{"frozen frame at 00:07"},
				Items:          []PublishBoardItem{test.item},
			})
			if board.Status != test.wantBoard || board.RenderReady {
				t.Fatalf("status/render_ready = %q/%v, want %q/false", board.Status, board.RenderReady, test.wantBoard)
			}
			if board.Items[0].Status != test.wantItem {
				t.Fatalf("item status = %q, want %q", board.Items[0].Status, test.wantItem)
			}
		})
	}
}

func TestNewPublishBoardForVariantDerivesArtifactKeysAndReadiness(t *testing.T) {
	id := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	ready := map[string]bool{
		"jobs/11111111-1111-1111-1111-111111111111/renders/viral-60-clean/videos/seg-001.mp4":           true,
		"jobs/11111111-1111-1111-1111-111111111111/renders/viral-60-clean/covers/seg-001.jpg":           true,
		"jobs/11111111-1111-1111-1111-111111111111/renders/viral-60-clean/captions/seg-001.caption.txt": true,
		"jobs/11111111-1111-1111-1111-111111111111/renders/viral-60-clean/videos/seg-002.mp4":           true,
		"jobs/11111111-1111-1111-1111-111111111111/renders/viral-60-clean/covers/seg-002.jpg":           true,
		"jobs/11111111-1111-1111-1111-111111111111/renders/viral-60-clean/captions/seg-002.caption.txt": false,
	}

	board, err := NewPublishBoardForVariant(NewPublishBoardForVariantOptions{
		JobID:      id,
		Variant:    editor.PresetViral60Clean,
		SegmentIDs: []string{"seg-001", "", "seg-002"},
		ArtifactExists: func(key string) (bool, error) {
			return ready[key], nil
		},
	})
	if err != nil {
		t.Fatalf("NewPublishBoardForVariant error = %v", err)
	}

	wantPrefix := "jobs/11111111-1111-1111-1111-111111111111/renders/viral-60-clean"
	if board.RenderResultKey != wantPrefix+"/render-result.json" {
		t.Fatalf("render result key = %q", board.RenderResultKey)
	}
	if board.PackManifestKey != wantPrefix+"/pack-manifest.json" {
		t.Fatalf("pack manifest key = %q", board.PackManifestKey)
	}
	if board.GalleryKey != wantPrefix+"/index.html" {
		t.Fatalf("gallery key = %q", board.GalleryKey)
	}
	if board.PublishSummary != wantPrefix+"/publish-summary.md" {
		t.Fatalf("publish summary key = %q", board.PublishSummary)
	}
	if len(board.Items) != 2 {
		t.Fatalf("items = %#v, want two non-empty segments", board.Items)
	}
	if board.Items[0].Status != "ready" || !board.Items[0].VideoReady || !board.Items[0].CoverReady || !board.Items[0].CaptionReady {
		t.Fatalf("first item = %#v, want ready", board.Items[0])
	}
	if board.Items[1].Status != "needs_caption" || !board.Items[1].VideoReady || !board.Items[1].CoverReady || board.Items[1].CaptionReady {
		t.Fatalf("second item = %#v, want needs caption", board.Items[1])
	}
	if board.Status != "needs_caption" || board.RenderReady {
		t.Fatalf("board status/render_ready = %q/%v, want needs_caption/false", board.Status, board.RenderReady)
	}
}
