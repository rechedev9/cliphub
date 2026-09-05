package editor

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/rechedev9/cliphub/internal/recapplan"
)

// FullDemoDocumentFiles is the public evidence serialized beside each render.
// Cache validation uses this projection too, so displayed documents cannot
// disagree with the immutable result that attests the delivered media.
func FullDemoDocumentFiles(evidence FullDemoRenderEvidence) map[string]any {
	return map[string]any{
		"full-demo-approved.json":  evidence.Approved,
		"full-demo-effective.json": evidence.Effective,
		"full-demo-audio.json": struct {
			SchemaVersion  string                  `json:"schema_version"`
			Options        recapplan.AudioOptions  `json:"options"`
			MusicIntervals []FullDemoMusicInterval `json:"music_intervals"`
			TrackLevels    []FullDemoTrackLevel    `json:"track_levels"`
		}{"1.0", evidence.Effective.Options.Audio, evidence.MusicIntervals, evidence.TrackLevels},
		"full-demo-loudness.json": evidence.ProgramLoudness,
		"full-demo-delivery.json": evidence.Delivery,
	}
}

func writeFullDemoDocuments(outDir string, shorts []ShortEdit) error {
	for _, short := range shorts {
		if short.FullDemo == nil {
			continue
		}
		for name, value := range FullDemoDocumentFiles(*short.FullDemo) {
			b, err := json.MarshalIndent(value, "", "  ")
			if err != nil {
				return err
			}
			if err := os.WriteFile(filepath.Join(outDir, name), append(b, '\n'), 0600); err != nil {
				return fmt.Errorf("write %s: %w", name, err)
			}
		}
	}
	return nil
}
