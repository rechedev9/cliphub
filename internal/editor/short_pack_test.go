package editor

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/rechedev9/cliphub/internal/recording"
)

func TestRunFFmpegWithOptionalLogRecordsStartFailure(t *testing.T) {
	t.Parallel()
	logPath := filepath.Join(t.TempDir(), "logs", "render.log")
	missingFFmpeg := filepath.Join(t.TempDir(), "missing-ffmpeg")

	err := runFFmpegWithOptionalLog(
		context.Background(),
		[]string{missingFFmpeg, "-version"},
		"render short",
		logPath,
	)
	if err == nil {
		t.Fatal("runFFmpegWithOptionalLog error = nil, want start failure")
	}
	content, readErr := os.ReadFile(logPath)
	if readErr != nil {
		t.Fatalf("read render log: %v", readErr)
	}
	if got := strings.TrimSpace(string(content)); got == "" {
		t.Fatal("render log is empty, want process start error")
	} else if !strings.Contains(got, "ffmpeg render short") {
		t.Fatalf("render log = %q, want labeled process error", got)
	}
}

func TestRenderSecondsPerMediaSecond(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name         string
		renderMS     int64
		mediaSeconds float64
		want         float64
	}{
		{name: "two times realtime", renderMS: 10_000, mediaSeconds: 5, want: 2},
		{name: "faster than realtime", renderMS: 1_500, mediaSeconds: 6, want: 0.25},
		{name: "missing duration", renderMS: 1_000, mediaSeconds: 0, want: 0},
		{name: "reused output", renderMS: 0, mediaSeconds: 5, want: 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := renderSecondsPerMediaSecond(tt.renderMS, tt.mediaSeconds); got != tt.want {
				t.Fatalf("renderSecondsPerMediaSecond(%d, %v) = %v, want %v", tt.renderMS, tt.mediaSeconds, got, tt.want)
			}
		})
	}
}

func TestNormalizeRenderJobs(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		jobs int
		want func(got int) bool
	}{
		{"explicit value wins", 3, func(got int) bool { return got == 3 }},
		{"one stays sequential", 1, func(got int) bool { return got == 1 }},
		{"zero selects automatic bounded limit", 0, func(got int) bool { return got >= 1 && got <= 4 }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := normalizeRenderJobs(tt.jobs); !tt.want(got) {
				t.Errorf("normalizeRenderJobs(%d) = %d, out of expected range", tt.jobs, got)
			}
		})
	}
}

func TestValidatedExistingArtifactRequiresMatchingProducerContract(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	videoPath := filepath.Join(dir, "compiled.mp4")
	coverPath := filepath.Join(dir, "compiled.cover.jpg")
	sheetPath := filepath.Join(dir, "compiled.sheet.jpg")
	for _, path := range []string{videoPath, coverPath, sheetPath} {
		if err := os.WriteFile(path, []byte("existing"), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	previousShort := ShortResult{
		SegmentID:         "demo-compilation",
		FFmpegCommand:     []string{"ffmpeg", "-i", "seg-001.mp4", "-i", "seg-002.mp4", videoPath},
		CoverCommand:      []string{"ffmpeg", "-i", videoPath, coverPath},
		CoverSheetCommand: []string{"ffmpeg", "-i", videoPath, sheetPath},
		OutputArtifact: recording.RecordingArtifact{
			Path:      videoPath,
			SizeBytes: int64(len("existing")),
		},
		CoverArtifact: recording.RecordingArtifact{
			Path:      coverPath,
			SizeBytes: int64(len("existing")),
		},
		CoverSheetArtifact: recording.RecordingArtifact{
			Path:      sheetPath,
			SizeBytes: int64(len("existing")),
		},
	}
	previous := &Result{
		Executed: true,
		Shorts:   []ShortResult{previousShort},
	}

	for _, role := range []struct {
		name string
		path string
	}{
		{name: "video", path: videoPath},
		{name: "cover", path: coverPath},
		{name: "cover-sheet", path: sheetPath},
	} {
		if !validatedExistingArtifact(previous, previousShort, role.path, role.name) {
			t.Fatalf("identical %s producer contract was not reusable", role.name)
		}
	}

	changedParts := previousShort
	changedParts.Parts = []ShortPart{{Input: "seg-003.mp4", DurationSeconds: 4}}
	if validatedExistingArtifact(previous, changedParts, videoPath, "video") {
		t.Fatal("video reused after compiled part inputs changed")
	}

	for _, changedCommand := range [][]string{
		{"ffmpeg", "-i", "seg-002.mp4", "-i", "seg-001.mp4", videoPath},
		{"ffmpeg", "-i", "seg-001.mp4", videoPath},
	} {
		current := previousShort
		current.FFmpegCommand = changedCommand
		for _, role := range []struct {
			name string
			path string
		}{
			{name: "video", path: videoPath},
			{name: "cover", path: coverPath},
			{name: "cover-sheet", path: sheetPath},
		} {
			if validatedExistingArtifact(previous, current, role.path, role.name) {
				t.Fatalf("%s reused after compiled producer changed to %v", role.name, changedCommand)
			}
		}
	}
}

// TestPublishShortReusesOutputProbeInsteadOfReprobing is B5(a): publishShort
// hardlinks (or byte-copies) short.Output to short.PublishPath, so the
// published file is byte-identical to what renderShort already probed. This
// points FFprobePath at a binary that does not exist; if publishShort still
// tried to re-probe the published file, that exec would fail and land in
// PublishArtifact.ProbeError. It must instead reuse renderShort's artifact
// (repointed at the publish path/role) and never touch ffprobe again.
func TestPublishShortReusesOutputProbeInsteadOfReprobing(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	outputPath := filepath.Join(dir, "output.mp4")
	publishPath := filepath.Join(dir, "publish.mp4")
	content := []byte("fake video bytes")
	if err := os.WriteFile(outputPath, content, 0o600); err != nil {
		t.Fatal(err)
	}

	manifest := &Manifest{Shorts: []ShortEdit{{
		SegmentID:   "seg-001",
		Output:      outputPath,
		PublishPath: publishPath,
	}}}
	result := &Result{Shorts: []ShortResult{{
		SegmentID: "seg-001",
		OutputArtifact: recording.RecordingArtifact{
			SegmentID:       "seg-001",
			Role:            "short",
			Type:            "video",
			Path:            outputPath,
			SizeBytes:       int64(len(content)),
			DurationSeconds: 3.2,
			Codec:           "h264",
			Width:           1080,
			Height:          1920,
			FrameRate:       "30/1",
		},
	}}}
	p := &shortPackRenderer{
		manifest: manifest,
		result:   result,
		opts:     shortPackOptions{FFprobePath: filepath.Join(dir, "missing-ffprobe-must-not-run")},
		shortMu:  make([]sync.Mutex, 1),
	}

	var warn []string
	if err := p.publishShort(context.Background(), 0, &manifest.Shorts[0], &warn); err != nil {
		t.Fatalf("publishShort error = %v", err)
	}

	got := result.Shorts[0].PublishArtifact
	if got.ProbeError != "" {
		t.Fatalf("PublishArtifact.ProbeError = %q, want empty: publish re-ran ffprobe instead of reusing the output probe", got.ProbeError)
	}
	if got.Role != "publish" || got.Path != publishPath {
		t.Fatalf("PublishArtifact role/path = %q/%q, want publish/%q", got.Role, got.Path, publishPath)
	}
	if got.Codec != "h264" || got.DurationSeconds != 3.2 || got.Width != 1080 || got.Height != 1920 || got.FrameRate != "30/1" {
		t.Fatalf("PublishArtifact = %#v, want probe fields copied from the output artifact", got)
	}
	if got.SizeBytes != int64(len(content)) {
		t.Fatalf("PublishArtifact.SizeBytes = %d, want %d (stat of the published path)", got.SizeBytes, len(content))
	}
	if !reflect.DeepEqual(manifest.Shorts[0].PublishArtifact, got) {
		t.Fatalf("manifest PublishArtifact = %#v, want %#v (same as result)", manifest.Shorts[0].PublishArtifact, got)
	}
}

func TestRunRendersShortsConcurrently(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	recordingResultPath := writeRecordingResultFixture(t, dir)
	ffmpegPath := fakeFFmpeg(t, dir)

	sequentialOut := filepath.Join(dir, "shorts-seq")
	sequential, err := Run(context.Background(), Config{
		RecordingResultPath: recordingResultPath,
		OutputDir:           sequentialOut,
		FFmpegPath:          ffmpegPath,
		RenderJobs:          1,
	})
	if err != nil {
		t.Fatalf("sequential Run error = %v", err)
	}

	parallelOut := filepath.Join(dir, "shorts-par")
	parallel, err := Run(context.Background(), Config{
		RecordingResultPath: recordingResultPath,
		OutputDir:           parallelOut,
		FFmpegPath:          ffmpegPath,
		RenderJobs:          4,
	})
	if err != nil {
		t.Fatalf("parallel Run error = %v", err)
	}

	if got, want := len(parallel.Shorts), len(sequential.Shorts); got != want {
		t.Fatalf("parallel shorts len = %d, want %d", got, want)
	}
	for i, short := range parallel.Shorts {
		if short.SegmentID != sequential.Shorts[i].SegmentID {
			t.Errorf("shorts[%d].SegmentID = %q, want %q", i, short.SegmentID, sequential.Shorts[i].SegmentID)
		}
		if short.OutputArtifact.SizeBytes == 0 {
			t.Errorf("shorts[%d] output artifact missing size: %#v", i, short.OutputArtifact)
		}
		if _, err := os.Stat(short.OutputArtifact.Path); err != nil {
			t.Errorf("shorts[%d] output missing: %v", i, err)
		}
		if short.PublishArtifact.SizeBytes == 0 {
			t.Errorf("shorts[%d] publish artifact missing size: %#v", i, short.PublishArtifact)
		}
	}
	if !reflect.DeepEqual(parallel.Warnings, sequential.Warnings) {
		t.Errorf("parallel warnings = %#v, want sequential order %#v", parallel.Warnings, sequential.Warnings)
	}
}

// Shorts-pack evidence must behave like the Full Demo collector: a span belongs
// to exactly one short, stage wall time is the union of active intervals while
// the elapsed sum stays a separate serial number, and the render job slot
// occupancy is split at the encode boundary so the post-encode tail is measured
// instead of guessed.
func TestShortPackTimingSplitsSlotOccupancyAtEncodeBoundary(t *testing.T) {
	t.Parallel()
	collector := newShortPackTimingCollector(3)
	base := collector.start
	at := func(ms int64) time.Time { return base.Add(time.Duration(ms) * time.Millisecond) }
	collector.slots[0] = shortPackSlotRecord{start: at(0), end: at(300), acquired: true, released: true}
	collector.slots[1] = shortPackSlotRecord{start: at(20), end: at(260), acquired: true, released: true}
	collector.slots[2] = shortPackSlotRecord{start: at(40), end: at(90), acquired: true, released: true}
	collector.recordOutcome(0, shortPackStageEncode, "", at(10), at(110), "ok")
	collector.recordOutcome(0, shortPackStageProbe, shortPackProbeOutput, at(110), at(130), "ok")
	// Publish and the quality check overlap: both start once the encode wrote
	// the output, and both run while the slot is still held.
	collector.recordOutcome(0, shortPackStagePublish, "", at(130), at(150), "ok")
	collector.recordOutcome(0, shortPackStageQualityCheck, "", at(130), at(290), "ok")
	collector.recordOutcome(1, shortPackStageEncode, "", at(30), at(200), "ok")
	// Short 2 reused its output, so it never encoded.
	collector.recordOutcome(2, shortPackStageProbe, shortPackProbeOutput, at(45), at(60), "ok")

	metrics := collector.snapshot(0)
	if metrics == nil {
		t.Fatal("snapshot(0) = nil, want timing evidence")
	}
	if metrics.Index != 0 {
		t.Fatalf("metrics.Index = %d, want 0", metrics.Index)
	}
	if len(metrics.Spans) != 4 {
		t.Fatalf("spans = %d (%+v), want the 4 stages of short 0 only", len(metrics.Spans), metrics.Spans)
	}
	for _, span := range metrics.Spans {
		if span.Index != 0 {
			t.Fatalf("span %+v leaked from another short", span)
		}
	}
	wantStages := []ShortPackTimingStage{
		{Stage: shortPackStageEncode, Spans: 1, WallMS: 100, ProcessElapsedSumMS: 100},
		{Stage: shortPackStageProbe, Spans: 1, WallMS: 20, ProcessElapsedSumMS: 20},
		{Stage: shortPackStagePublish, Spans: 1, WallMS: 20, ProcessElapsedSumMS: 20},
		{Stage: shortPackStageQualityCheck, Spans: 1, WallMS: 160, ProcessElapsedSumMS: 160},
	}
	if !reflect.DeepEqual(metrics.Stages, wantStages) {
		t.Fatalf("stages = %+v, want %+v", metrics.Stages, wantStages)
	}
	if metrics.WallMS != 280 {
		t.Fatalf("wall union = %d, want 280", metrics.WallMS)
	}
	if metrics.ProcessElapsedSumMS != 300 {
		t.Fatalf("process elapsed sum = %d, want 300", metrics.ProcessElapsedSumMS)
	}
	if metrics.WallMS >= metrics.ProcessElapsedSumMS {
		t.Fatalf("overlapping post-encode work kept the sum as wall: %+v", metrics)
	}

	slot := metrics.Slot
	if slot == nil {
		t.Fatal("slot occupancy missing")
	}
	want := ShortPackSlotOccupancy{StartMS: 0, EndMS: 300, HeldMS: 300, PreEncodeMS: 10, EncodeMS: 100, PostEncodeMS: 190}
	if *slot != want {
		t.Fatalf("slot = %+v, want %+v", *slot, want)
	}
	if slot.HeldMS < slot.EncodeMS {
		t.Fatalf("slot held %d ms < encode %d ms", slot.HeldMS, slot.EncodeMS)
	}

	second := collector.snapshot(1)
	if second == nil || second.Slot == nil {
		t.Fatal("snapshot(1) has no slot evidence")
	}
	wantSecond := ShortPackSlotOccupancy{StartMS: 20, EndMS: 260, HeldMS: 240, PreEncodeMS: 10, EncodeMS: 170, PostEncodeMS: 60}
	if *second.Slot != wantSecond {
		t.Fatalf("slot[1] = %+v, want %+v", *second.Slot, wantSecond)
	}
	if second.Slot.HeldMS < second.Slot.EncodeMS {
		t.Fatalf("slot held %d ms < encode %d ms", second.Slot.HeldMS, second.Slot.EncodeMS)
	}

	reused := collector.snapshot(2)
	if reused == nil || reused.Slot == nil {
		t.Fatal("snapshot(2) has no slot evidence")
	}
	wantReused := ShortPackSlotOccupancy{StartMS: 40, EndMS: 90, HeldMS: 50, PreEncodeMS: 0, EncodeMS: 0, PostEncodeMS: 50}
	if *reused.Slot != wantReused {
		t.Fatalf("reused slot = %+v, want %+v (no encode: the whole occupancy is post-encode)", *reused.Slot, wantReused)
	}
}

// Stages finish in whatever order the pool schedules them, so the snapshot must
// serialize identically regardless of append order.
func TestShortPackTimingOrderingIsDeterministic(t *testing.T) {
	t.Parallel()
	build := func(reverse bool) *ShortPackTimingMetrics {
		collector := newShortPackTimingCollector(1)
		base := collector.start
		collector.slots[0] = shortPackSlotRecord{start: base, end: base.Add(200 * time.Millisecond), acquired: true, released: true}
		spans := []struct {
			stage, variant string
			start, end     int64
		}{
			{shortPackStageQualityCheck, "", 60, 190},
			{shortPackStageEncode, "", 0, 50},
			{shortPackStageCover, "", 60, 80},
			{shortPackStageProbe, shortPackProbeOutput, 50, 60},
		}
		if reverse {
			for i, j := 0, len(spans)-1; i < j; i, j = i+1, j-1 {
				spans[i], spans[j] = spans[j], spans[i]
			}
		}
		for _, span := range spans {
			collector.recordOutcome(0, span.stage, span.variant,
				base.Add(time.Duration(span.start)*time.Millisecond),
				base.Add(time.Duration(span.end)*time.Millisecond), "ok")
		}
		return collector.snapshot(0)
	}
	forward, err := json.Marshal(build(false))
	if err != nil {
		t.Fatal(err)
	}
	reversed, err := json.Marshal(build(true))
	if err != nil {
		t.Fatal(err)
	}
	if string(forward) != string(reversed) {
		t.Fatalf("snapshot depends on completion order:\n%s\n%s", forward, reversed)
	}
	var metrics ShortPackTimingMetrics
	if err := json.Unmarshal(forward, &metrics); err != nil {
		t.Fatal(err)
	}
	var order []string
	for _, span := range metrics.Spans {
		order = append(order, span.Stage)
	}
	wantOrder := []string{shortPackStageEncode, shortPackStageProbe, shortPackStageCover, shortPackStageQualityCheck}
	if !reflect.DeepEqual(order, wantOrder) {
		t.Fatalf("span order = %v, want %v", order, wantOrder)
	}
}

// The slot bookkeeping is called from two goroutines (the scheduler grants,
// the worker releases) and from renderers built without a collector in tests,
// so it must be a no-op on a nil collector, ignore indices it was not sized
// for, keep the first grant, and never report a released slot it never saw.
func TestShortPackTimingSlotGuardsAndOutcomes(t *testing.T) {
	t.Parallel()
	var none *shortPackTimingCollector
	none.beginSlot(0)
	none.finishSlot(0)
	none.record(context.Background(), 0, shortPackStageEncode, "", time.Now(), time.Now(), nil)
	if none.snapshot(0) != nil {
		t.Fatal("nil collector produced a snapshot")
	}

	collector := newShortPackTimingCollector(1)
	collector.beginSlot(-1)
	collector.beginSlot(1)
	collector.finishSlot(0) // released before granted: ignored
	if got := collector.snapshot(0); got != nil {
		t.Fatalf("snapshot(0) before any grant = %+v, want nil", got)
	}
	if got := collector.snapshot(1); got != nil {
		t.Fatalf("snapshot(1) outside the pack = %+v, want nil", got)
	}
	collector.beginSlot(0)
	first := collector.slots[0].start
	collector.beginSlot(0) // a second grant keeps the first
	if collector.slots[0].start != first {
		t.Fatal("second beginSlot moved the grant time")
	}
	if got := collector.snapshot(0); got == nil || got.Slot != nil {
		t.Fatalf("snapshot(0) of a held slot = %+v, want evidence without occupancy", got)
	}
	collector.finishSlot(0)
	got := collector.snapshot(0)
	if got == nil || got.Slot == nil || got.Slot.HeldMS < 0 || got.Slot.PostEncodeMS != got.Slot.HeldMS {
		t.Fatalf("released slot without spans = %+v, want occupancy that is all post-encode", got)
	}

	cancelled, cancel := context.WithCancel(context.Background())
	cancel()
	failure := context.DeadlineExceeded
	for _, tc := range []struct {
		ctx  context.Context
		err  error
		want string
	}{
		{context.Background(), nil, "ok"},
		{cancelled, nil, "ok"},
		{context.Background(), failure, "error"},
		{nil, failure, "error"},
		{cancelled, failure, "cancelled"},
	} {
		if got := shortPackTimingOutcome(tc.ctx, tc.err); got != tc.want {
			t.Fatalf("outcome(ctx cancelled=%v, err=%v) = %q, want %q", tc.ctx != nil && tc.ctx.Err() != nil, tc.err, got, tc.want)
		}
	}
}

// A real pack render through the fake FFmpeg seam must record every stage of
// every short with its own index, keep each encode inside the slot that ran it,
// and report a slot occupancy that is never shorter than the encode. This is
// the evidence the "release the slot before the post-encode work" decision
// needs, so it is asserted on the actual render path and not on the collector.
func TestRunRecordsShortPackStageTimingPerShort(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	recordingResultPath := writeRecordingResultFixture(t, dir)
	ffmpegPath := fakeFFmpeg(t, dir)

	result, err := Run(context.Background(), Config{
		RecordingResultPath: recordingResultPath,
		OutputDir:           filepath.Join(dir, "shorts"),
		FFmpegPath:          ffmpegPath,
		RenderJobs:          2,
		QualityChecks:       true,
		CoverSheets:         true,
		CoverSheetsSet:      true,
	})
	if err != nil {
		t.Fatalf("Run error = %v", err)
	}
	if len(result.Shorts) < 2 {
		t.Fatalf("shorts = %d, want at least 2 to prove per-short indices", len(result.Shorts))
	}

	for i, short := range result.Shorts {
		performance := short.Performance
		if performance == nil {
			t.Fatalf("shorts[%d] has no performance record", i)
		}
		timing := performance.ShortPackTiming
		if timing == nil {
			t.Fatalf("shorts[%d] has no short pack timing evidence", i)
		}
		if timing.Index != i {
			t.Fatalf("shorts[%d] timing index = %d", i, timing.Index)
		}
		stages := map[string]ShortPackTimingStage{}
		for _, stage := range timing.Stages {
			stages[stage.Stage] = stage
		}
		wantStages := []string{shortPackStageEncode, shortPackStageProbe, shortPackStagePublish}
		if len(short.QualityCommand) > 0 {
			wantStages = append(wantStages, shortPackStageQualityCheck)
		}
		if len(short.CoverCommand) > 0 {
			wantStages = append(wantStages, shortPackStageCover)
		}
		if len(short.CoverSheetCommand) > 0 {
			wantStages = append(wantStages, shortPackStageCoverSheet)
		}
		for _, stage := range wantStages {
			if _, ok := stages[stage]; !ok {
				t.Fatalf("shorts[%d] stage %q missing from %+v", i, stage, timing.Stages)
			}
		}
		var encode *ShortPackTimingSpan
		probes := map[string]bool{}
		for j := range timing.Spans {
			span := &timing.Spans[j]
			if span.Index != i {
				t.Fatalf("shorts[%d] carries span of short %d: %+v", i, span.Index, span)
			}
			// The probe stage depends on a real ffprobe, which the fake FFmpeg
			// seam does not provide, so only its outcome may be an error here.
			if span.Outcome != "ok" && span.Stage != shortPackStageProbe {
				t.Fatalf("shorts[%d] span %+v outcome is not ok", i, span)
			}
			if span.Outcome != "ok" && span.Outcome != "error" && span.Outcome != "cancelled" {
				t.Fatalf("shorts[%d] span %+v has an unknown outcome", i, span)
			}
			if span.ElapsedMS < 0 || span.EndMS < span.StartMS {
				t.Fatalf("shorts[%d] span %+v has a negative interval", i, span)
			}
			if span.Stage == shortPackStageEncode {
				encode = span
			}
			if span.Stage == shortPackStageProbe {
				probes[span.Variant] = true
			}
		}
		if encode == nil {
			t.Fatalf("shorts[%d] has no encode span", i)
		}
		if encode.ElapsedMS != performance.RenderMS {
			t.Fatalf("shorts[%d] encode span %d ms != RenderMS %d ms", i, encode.ElapsedMS, performance.RenderMS)
		}
		if !probes[shortPackProbeOutput] {
			t.Fatalf("shorts[%d] has no output probe span: %+v", i, timing.Spans)
		}
		if len(short.CoverCommand) > 0 && !probes[shortPackProbeCover] {
			t.Fatalf("shorts[%d] has no cover probe span: %+v", i, timing.Spans)
		}
		// Offsets and elapsed times are truncated to milliseconds independently,
		// so the union of sequential spans can exceed the elapsed sum by at most
		// one millisecond per span; anything beyond that is a real union bug.
		if slack := int64(len(timing.Spans)); timing.WallMS > timing.ProcessElapsedSumMS+slack {
			t.Fatalf("shorts[%d] wall union %d > elapsed sum %d (+%d ms truncation slack)", i, timing.WallMS, timing.ProcessElapsedSumMS, slack)
		}
		slot := timing.Slot
		if slot == nil {
			t.Fatalf("shorts[%d] has no slot occupancy", i)
		}
		if slot.HeldMS < slot.EncodeMS {
			t.Fatalf("shorts[%d] slot held %d ms < encode %d ms", i, slot.HeldMS, slot.EncodeMS)
		}
		if slot.PostEncodeMS > slot.HeldMS {
			t.Fatalf("shorts[%d] post-encode %d ms > slot held %d ms", i, slot.PostEncodeMS, slot.HeldMS)
		}
		if encode.StartMS < slot.StartMS || encode.EndMS > slot.EndMS {
			t.Fatalf("shorts[%d] encode %+v ran outside its slot %+v", i, encode, slot)
		}
	}
}

func TestRunRejectsNegativeRenderJobs(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	recordingResultPath := writeRecordingResultFixture(t, dir)

	_, err := Run(context.Background(), Config{
		RecordingResultPath: recordingResultPath,
		OutputDir:           filepath.Join(dir, "shorts"),
		RenderJobs:          -1,
	})
	if err == nil || err.Error() != "render jobs must be >= 0" {
		t.Fatalf("Run error = %v, want render jobs validation error", err)
	}
}
