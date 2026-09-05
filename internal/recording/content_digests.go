package recording

import (
	"context"
	"fmt"

	"github.com/rechedev9/cliphub/internal/mediaassets"
)

func (r *RecordingResult) DigestSegmentFiles(ctx context.Context) error {
	for i := range r.Artifacts {
		artifact := &r.Artifacts[i]
		if !isUsableSegmentClip(*artifact) {
			continue
		}
		digest, err := mediaassets.FileDigest(ctx, artifact.Path, 8<<30)
		if err != nil {
			return fmt.Errorf("digest captured segment %s: %w", artifact.SegmentID, err)
		}
		if artifact.ContentSHA256 != "" && digest != artifact.ContentSHA256 {
			return fmt.Errorf("captured segment %s content changed", artifact.SegmentID)
		}
		artifact.ContentSHA256 = digest
	}
	return nil
}
