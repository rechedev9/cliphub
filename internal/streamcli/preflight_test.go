package streamcli

import (
	"context"
	"strings"
	"testing"
)

func TestLocalStreamServiceRejectsMissingFFmpeg(t *testing.T) {
	err := (localStreamService{}).ValidateFFmpeg(context.Background(), "cliphub-ffmpeg-that-does-not-exist")
	if err == nil || !strings.Contains(err.Error(), "not accessible") {
		t.Fatalf("ValidateFFmpeg error = %v, want inaccessible executable", err)
	}
}
