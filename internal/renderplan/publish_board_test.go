package renderplan

import (
	"testing"

	"github.com/google/uuid"

	"github.com/rechedev9/cliphub/internal/editor"
)

func TestNewPublishBoardStatus(t *testing.T) {
	const warning = "frozen frame at 00:07"
	tests := []struct {
		name            string
		coversRequired  bool
		warnings        []string
		err             string
		item            PublishBoardItem
		wantBoard       string
		wantItem        string // "" when a failed render leaves items ungraded
		wantRenderReady bool
	}{
		{
			name:            "all items ready",
			coversRequired:  true,
			item:            PublishBoardItem{VideoReady: true, CoverReady: true, CaptionReady: true},
			wantBoard:       "ready",
			wantItem:        "ready",
			wantRenderReady: true,
		},
		{
			name:           "missing cover surfaces before caption",
			coversRequired: true,
			item:           PublishBoardItem{VideoReady: true},
			wantBoard:      "needs_cover",
			wantItem:       "needs_cover",
		},
		{
			name:      "needs caption",
			item:      PublishBoardItem{VideoReady: true, CoverReady: true},
			wantBoard: "needs_caption",
			wantItem:  "needs_caption",
		},
		{
			name:      "render error fails the board",
			err:       "render failed",
			item:      PublishBoardItem{},
			wantBoard: "failed",
		},
		{
			name:            "warnings stay informational",
			warnings:        []string{warning},
			item:            PublishBoardItem{VideoReady: true, CaptionReady: true},
			wantBoard:       "ready",
			wantItem:        "ready",
			wantRenderReady: true,
		},
		{
			name:      "missing video surfaces before warnings",
			warnings:  []string{warning},
			item:      PublishBoardItem{CaptionReady: true},
			wantBoard: "draft",
			wantItem:  "missing_video",
		},
		{
			name:           "missing required cover surfaces before warnings",
			coversRequired: true,
			warnings:       []string{warning},
			item:           PublishBoardItem{VideoReady: true, CaptionReady: true},
			wantBoard:      "needs_cover",
			wantItem:       "needs_cover",
		},
		{
			name:      "missing caption surfaces before warnings",
			warnings:  []string{warning},
			item:      PublishBoardItem{VideoReady: true, CoverReady: true},
			wantBoard: "needs_caption",
			wantItem:  "needs_caption",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			item := test.item
			item.SegmentID = "seg-001"
			board := NewPublishBoard(NewPublishBoardOptions{
				JobID:          uuid.New(),
				Variant:        "viral-60-clean",
				CoversRequired: test.coversRequired,
				Warnings:       test.warnings,
				Error:          test.err,
				Items:          []PublishBoardItem{item},
			})
			if board.Status != test.wantBoard || board.RenderReady != test.wantRenderReady {
				t.Fatalf("status/render_ready = %q/%v, want %q/%v", board.Status, board.RenderReady, test.wantBoard, test.wantRenderReady)
			}
			if board.Items[0].Status != test.wantItem {
				t.Fatalf("item status = %q, want %q", board.Items[0].Status, test.wantItem)
			}
			if len(board.Warnings) != len(test.warnings) {
				t.Fatalf("warnings = %#v, want %#v preserved", board.Warnings, test.warnings)
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
