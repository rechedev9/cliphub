package streamcli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/rechedev9/cliphub/internal/streamclips"
)

// TestStreamJourneyChainsStagesMediaFree drives the stream/VOD journey through
// the in-process command layer via runStreamWithService + fakeStreamService: a
// plan preflight, the persisted plan, and the render dry-run. Each hop asserts
// the {ok, dry_run, executed} envelope and that the --out document one stage
// writes is the literal --plan the next stage consumes. It stays media-free (no
// ffmpeg/ffprobe) because the service seam fakes probe/ffmpeg/render, so it
// always runs.
func TestStreamJourneyChainsStagesMediaFree(t *testing.T) {
	ws := t.TempDir()
	editPlan := filepath.Join(ws, "edit-plan.json")

	planProbe := streamclips.SourceProbe{Width: 1920, Height: 1080, DurationSeconds: 15, VideoCodec: "h264", AudioCodec: "aac"}

	// 1. plan --dry-run validates the clip/crop contract without writing.
	stdout := runStream(t, &fakeStreamService{probe: planProbe},
		"plan", "--input", "stream.mp4", "--out", editPlan,
		"--dry-run", "--format", "json")
	var planDry streamPlanResult
	decodeStreamJSON(t, "stream plan preflight", stdout, &planDry)
	if !planDry.OK || !planDry.DryRun || planDry.Executed {
		t.Fatalf("plan preflight envelope = %#v, want a successful dry run", planDry)
	}
	assertStreamPathMissing(t, editPlan)

	// 2. plan persist writes the edit plan the render stage consumes.
	stdout = runStream(t, &fakeStreamService{probe: planProbe},
		"plan", "--input", "stream.mp4", "--out", editPlan, "--format", "json")
	var planPersist streamPlanResult
	decodeStreamJSON(t, "stream plan", stdout, &planPersist)
	if !planPersist.OK || planPersist.DryRun || !planPersist.Executed {
		t.Fatalf("plan envelope = %#v, want executed", planPersist)
	}
	assertStreamFileExists(t, editPlan)

	// 3. render --dry-run consumes the persisted plan; its --plan is the plan
	// step's --out.
	renderDir := filepath.Join(ws, "render")
	stdout = runStream(t, &fakeStreamService{probe: planProbe},
		"render", "--input", "stream.mp4", "--plan", editPlan, "--out", renderDir, "--dry-run", "--format", "json")
	var render streamRenderResult
	decodeStreamJSON(t, "stream render", stdout, &render)
	if !render.OK || !render.DryRun || render.Executed {
		t.Fatalf("render envelope = %#v, want a successful dry run", render)
	}
}

func runStream(t *testing.T, service streamService, args ...string) string {
	t.Helper()
	var stdout, stderr bytes.Buffer
	code := runStreamWithService(args, &stdout, &stderr, service)
	if code != exitSuccess || stderr.Len() != 0 {
		t.Fatalf("stream %v: code = %d, stderr = %q\nstdout: %s", args, code, stderr.String(), stdout.String())
	}
	return stdout.String()
}

func decodeStreamJSON(t *testing.T, source, stdout string, into any) {
	t.Helper()
	if err := json.Unmarshal([]byte(stdout), into); err != nil {
		t.Fatalf("%s: decode stdout: %v\n%s", source, err, stdout)
	}
}

func assertStreamFileExists(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("expected chained artifact %s: %v", path, err)
	}
}

func assertStreamPathMissing(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("path %s stat error = %v, want not exist", path, err)
	}
}
