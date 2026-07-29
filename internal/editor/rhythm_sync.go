package editor

import (
	"encoding/json"
	"fmt"
	"os"
	"reflect"

	"github.com/rechedev9/fragforge/internal/killplan"
	"github.com/rechedev9/fragforge/internal/rhythm"
)

func loadRhythmSync(path string) (map[string]rhythm.SegmentSync, error) {
	analysis, err := readRhythmAnalysis(path)
	if err != nil || analysis == nil {
		return nil, err
	}
	return indexRhythmSync(*analysis)
}

func loadRhythmSyncForRender(path, musicPath string, plan killplan.Plan, tailTrimSeconds float64) (map[string]rhythm.SegmentSync, error) {
	analysisPtr, err := readRhythmAnalysis(path)
	if err != nil || analysisPtr == nil {
		return nil, err
	}
	analysis := *analysisPtr
	tailTrimSeconds = rhythm.NormalizeTailTrimSeconds(tailTrimSeconds)
	if analysis.SchemaVersion != rhythm.SchemaVersion {
		return nil, fmt.Errorf("rhythm json schema %q is stale; regenerate with schema %s", analysis.SchemaVersion, rhythm.SchemaVersion)
	}
	musicSHA, err := rhythm.SourceSHA256(musicPath)
	if err != nil {
		return nil, fmt.Errorf("hash rhythm music source: %w", err)
	}
	if analysis.SourceSHA256 == "" || analysis.SourceSHA256 != musicSHA {
		return nil, fmt.Errorf("rhythm json music fingerprint does not match; regenerate rhythm")
	}
	if analysis.TailTrimSeconds != tailTrimSeconds {
		return nil, fmt.Errorf("rhythm json tail trim %.3f does not match render %.3f; regenerate rhythm", analysis.TailTrimSeconds, tailTrimSeconds)
	}
	expected := rhythm.BuildSegmentSyncWithTrim(plan, analysis.BeatTimesSeconds, analysis.KillOffsetSeconds, tailTrimSeconds)
	if len(analysis.SegmentSync) < len(expected) ||
		!reflect.DeepEqual(analysis.SegmentSync[:len(expected)], expected) {
		return nil, fmt.Errorf("rhythm json segment timing does not match the ordered render plan; regenerate rhythm")
	}
	analysis.SegmentSync = analysis.SegmentSync[:len(expected)]
	return indexRhythmSync(analysis)
}

func readRhythmAnalysis(path string) (*rhythm.Analysis, error) {
	if path == "" {
		return nil, nil
	}
	// #nosec G304 -- rhythm path is an explicit local CLI/config input.
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read rhythm json: %w", err)
	}
	var analysis rhythm.Analysis
	if err := json.Unmarshal(b, &analysis); err != nil {
		return nil, fmt.Errorf("decode rhythm json: %w", err)
	}
	return &analysis, nil
}

func indexRhythmSync(analysis rhythm.Analysis) (map[string]rhythm.SegmentSync, error) {
	if len(analysis.SegmentSync) == 0 {
		return nil, fmt.Errorf("rhythm json has no segment_sync entries")
	}
	indexed := make(map[string]rhythm.SegmentSync, len(analysis.SegmentSync))
	for _, entry := range analysis.SegmentSync {
		if entry.SegmentID == "" {
			return nil, fmt.Errorf("rhythm json contains segment_sync entry without segment_id")
		}
		indexed[entry.SegmentID] = entry
	}
	return indexed, nil
}
