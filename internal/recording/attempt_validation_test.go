package recording

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func validAttempt(t *testing.T) (RecordingPlan, RecordingResult, string) {
	t.Helper()
	return validAttemptInDir(t, t.TempDir())
}

func validAttemptInDir(t *testing.T, outDir string) (RecordingPlan, RecordingResult, string) {
	t.Helper()
	if err := os.MkdirAll(outDir, 0o750); err != nil {
		t.Fatal(err)
	}
	plan := testPlan()
	plan.OutputDir = outDir
	plan.Segments = plan.Segments[:1]
	scriptPath := filepath.Join(outDir, "recording.js")
	if err := os.WriteFile(scriptPath, []byte("script"), 0o600); err != nil {
		t.Fatal(err)
	}
	clipPath := filepath.Join(outDir, "segments", plan.Segments[0].ID+".mp4")
	if err := os.MkdirAll(filepath.Dir(clipPath), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(clipPath, []byte("clip"), 0o600); err != nil {
		t.Fatal(err)
	}
	result := RecordingResult{
		Plan:            plan,
		Script:          scriptPath,
		CaptureMode:     CaptureModeReal,
		CaptureVerified: true,
		Artifacts: []RecordingArtifact{{
			SegmentID: plan.Segments[0].ID,
			Type:      "video",
			Role:      "segment",
			Path:      clipPath,
			SizeBytes: 4,
		}},
	}
	result.CaptureInputFingerprint, _ = CaptureInputFingerprint(plan)
	return plan, result, outDir
}

func TestValidateRecordingAttemptAcceptsExactAttempt(t *testing.T) {
	plan, result, outDir := validAttempt(t)
	if err := ValidateRecordingAttempt(plan, outDir, result); err != nil {
		t.Fatalf("ValidateRecordingAttempt error = %v", err)
	}
}

func TestValidateRecordingAttemptAcceptsRelativeOutputDirectory(t *testing.T) {
	t.Chdir(t.TempDir())
	plan, result, outDir := validAttemptInDir(t, "attempt")

	if err := ValidateRecordingAttempt(plan, outDir, result); err != nil {
		t.Fatalf("ValidateRecordingAttempt with relative output directory error = %v", err)
	}
}

func TestValidateRecordingAttemptRejectsSymlinkEscape(t *testing.T) {
	plan, result, outDir := validAttempt(t)
	outsideClip := filepath.Join(t.TempDir(), "outside.mp4")
	if err := os.WriteFile(outsideClip, []byte("clip"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(result.Artifacts[0].Path); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outsideClip, result.Artifacts[0].Path); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}

	err := ValidateRecordingAttempt(plan, outDir, result)
	if err == nil || !strings.Contains(err.Error(), "resolved path escapes recording output directory") {
		t.Fatalf("ValidateRecordingAttempt error = %v, want symlink escape rejection", err)
	}
}

func TestValidateRecordingAttemptRejectsUnboundResult(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(t *testing.T, result *RecordingResult, outDir string)
		want   string
	}{
		{
			name: "changed plan",
			mutate: func(t *testing.T, result *RecordingResult, _ string) {
				result.Plan.TargetSteamID64 = "76561198148986857"
				accountID, err := AccountIDFromSteamID64(result.Plan.TargetSteamID64)
				if err != nil {
					t.Fatal(err)
				}
				result.Plan.TargetAccountID = accountID
				result.CaptureInputFingerprint, _ = CaptureInputFingerprint(result.Plan)
			},
			want: "plan does not match",
		},
		{
			name: "script outside attempt",
			mutate: func(t *testing.T, result *RecordingResult, _ string) {
				result.Script = filepath.Join(t.TempDir(), "recording.js")
				if err := os.WriteFile(result.Script, []byte("script"), 0o600); err != nil {
					t.Fatal(err)
				}
			},
			want: "does not match expected path",
		},
		{
			name: "clip outside attempt",
			mutate: func(t *testing.T, result *RecordingResult, _ string) {
				result.Artifacts[0].Path = filepath.Join(t.TempDir(), "seg-001.mp4")
				if err := os.WriteFile(result.Artifacts[0].Path, []byte("clip"), 0o600); err != nil {
					t.Fatal(err)
				}
			},
			want: "does not match expected path",
		},
		{
			name: "size mismatch",
			mutate: func(_ *testing.T, result *RecordingResult, _ string) {
				result.Artifacts[0].SizeBytes = 999
			},
			want: "declared 999",
		},
		{
			name: "empty script",
			mutate: func(t *testing.T, result *RecordingResult, _ string) {
				if err := os.WriteFile(result.Script, nil, 0o600); err != nil {
					t.Fatal(err)
				}
			},
			want: "script: file is empty",
		},
		{
			name: "unexpected segment",
			mutate: func(_ *testing.T, result *RecordingResult, _ string) {
				result.Artifacts[0].SegmentID = "seg-999"
			},
			want: "unexpected segment",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			plan, result, outDir := validAttempt(t)
			tt.mutate(t, &result, outDir)
			err := ValidateRecordingAttempt(plan, outDir, result)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("ValidateRecordingAttempt error = %v, want %q", err, tt.want)
			}
		})
	}
}
