package recording

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
)

// ValidateRecordingAttempt binds a successful recorder result to the exact
// plan and output namespace that the worker launched. It must run before any
// durable artifact from the attempt is published.
func ValidateRecordingAttempt(expected RecordingPlan, outDir string, result RecordingResult) error {
	if err := ValidateRunResult(result); err != nil {
		return err
	}
	if !reflect.DeepEqual(result.Plan, expected) {
		return fmt.Errorf("recording result plan does not match the launched attempt")
	}

	expectedScript := filepath.Join(outDir, "recording.js")
	if err := validateAttemptFile(outDir, result.Script, expectedScript, 0); err != nil {
		return fmt.Errorf("recording script: %w", err)
	}

	expectedSegments := make(map[string]string, len(expected.Segments))
	for _, segment := range expected.Segments {
		expectedSegments[segment.ID] = filepath.Join(outDir, "segments", segment.ID+".mp4")
	}
	seen := make(map[string]bool, len(expectedSegments))
	for _, artifact := range result.Artifacts {
		if !isUsableSegmentClip(artifact) {
			continue
		}
		expectedPath, ok := expectedSegments[artifact.SegmentID]
		if !ok {
			return fmt.Errorf("recording result contains unexpected segment clip %q", artifact.SegmentID)
		}
		if seen[artifact.SegmentID] {
			return fmt.Errorf("recording result contains duplicate segment clip %q", artifact.SegmentID)
		}
		if err := validateAttemptFile(outDir, artifact.Path, expectedPath, artifact.SizeBytes); err != nil {
			return fmt.Errorf("segment %s: %w", artifact.SegmentID, err)
		}
		seen[artifact.SegmentID] = true
	}
	missing := make([]string, 0)
	for _, segment := range expected.Segments {
		if !seen[segment.ID] {
			missing = append(missing, segment.ID)
		}
	}
	if len(missing) > 0 {
		return fmt.Errorf("recording result missing segment clips: %s", strings.Join(missing, ", "))
	}
	return nil
}

func validateAttemptFile(outDir, actualPath, expectedPath string, expectedSize int64) error {
	if actualPath == "" {
		return fmt.Errorf("path is required")
	}
	actualAbs, err := filepath.Abs(actualPath)
	if err != nil {
		return fmt.Errorf("resolve path: %w", err)
	}
	expectedAbs, err := filepath.Abs(expectedPath)
	if err != nil {
		return fmt.Errorf("resolve expected path: %w", err)
	}
	same, err := samePath(actualAbs, expectedAbs)
	if err != nil {
		return fmt.Errorf("compare path with expected file: %w", err)
	}
	if !same {
		return fmt.Errorf("path %q does not match expected path %q", actualPath, expectedPath)
	}

	outAbs, err := filepath.Abs(outDir)
	if err != nil {
		return fmt.Errorf("resolve output directory path: %w", err)
	}
	resolvedOut, err := filepath.EvalSymlinks(outAbs)
	if err != nil {
		return fmt.Errorf("resolve output directory: %w", err)
	}
	resolvedPath, err := filepath.EvalSymlinks(actualAbs)
	if err != nil {
		return fmt.Errorf("resolve file: %w", err)
	}
	relative, err := filepath.Rel(resolvedOut, resolvedPath)
	if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) || filepath.IsAbs(relative) {
		return fmt.Errorf("resolved path escapes recording output directory")
	}
	info, err := os.Stat(resolvedPath)
	if err != nil {
		return fmt.Errorf("stat file: %w", err)
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("path is not a regular file")
	}
	if info.Size() == 0 {
		return fmt.Errorf("file is empty")
	}
	if expectedSize > 0 && info.Size() != expectedSize {
		return fmt.Errorf("size is %d bytes, result declared %d", info.Size(), expectedSize)
	}
	return nil
}

func samePath(a, b string) (bool, error) {
	a = filepath.Clean(a)
	b = filepath.Clean(b)
	if a == b {
		return true, nil
	}
	if runtime.GOOS != "windows" || !strings.EqualFold(a, b) {
		return false, nil
	}
	aInfo, err := os.Stat(a)
	if err != nil {
		return false, err
	}
	bInfo, err := os.Stat(b)
	if err != nil {
		return false, err
	}
	return os.SameFile(aInfo, bInfo), nil
}
