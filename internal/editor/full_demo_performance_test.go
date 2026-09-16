package editor

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/rechedev9/cliphub/internal/recapplan"
)

// Overlapping item encodes must expose true elapsed wall time (the union) while
// the work sum stays a clearly separate number.
func TestFullDemoTimingOverlapUnionAndWorkSum(t *testing.T) {
	collector := newFullDemoTimingCollector()
	scope := &fullDemoTimingAnnotation{collector: collector, stage: "items", index: 0, attempt: -1}
	base := collector.start
	collector.append(scope, "item-a", nil, base, base.Add(100*time.Millisecond), "ok")
	collector.append(scope, "item-b", nil, base.Add(50*time.Millisecond), base.Add(150*time.Millisecond), "ok")
	collector.append(scope, "item-c", nil, base.Add(200*time.Millisecond), base.Add(230*time.Millisecond), "ok")
	metrics := collector.snapshot()
	if metrics == nil {
		t.Fatal("no timing metrics")
	}
	stage := metrics.Stages[0]
	if stage.Stage != "items" || stage.Spans != 3 {
		t.Fatalf("stage aggregate: %+v", stage)
	}
	if stage.WallMS != 180 {
		t.Fatalf("stage wall union = %d, want 180", stage.WallMS)
	}
	if stage.ProcessElapsedSumMS != 230 {
		t.Fatalf("stage process elapsed sum = %d, want 230", stage.ProcessElapsedSumMS)
	}
	if stage.WallMS >= stage.ProcessElapsedSumMS {
		t.Fatalf("overlapping work kept the sum as wall: %+v", stage)
	}
	if metrics.WallMS != 180 || metrics.ProcessElapsedSumMS != 230 {
		t.Fatalf("render aggregate: wall=%d process_elapsed_sum=%d", metrics.WallMS, metrics.ProcessElapsedSumMS)
	}
	body, err := json.Marshal(stage)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(body), `"wall_ms":180`) || !strings.Contains(string(body), `"process_elapsed_sum_ms":230`) {
		t.Fatalf("wall and work sum are not distinct fields: %s", body)
	}
}

// Completion order is not deterministic under concurrency, so the snapshot must
// sort by the recorded interval and fixed fields.
func TestFullDemoTimingOrderingIsDeterministic(t *testing.T) {
	build := func(reverse bool) *FullDemoTimingMetrics {
		collector := newFullDemoTimingCollector()
		scope := &fullDemoTimingAnnotation{collector: collector, stage: "items", index: 0, attempt: -1}
		base := collector.start
		spans := []struct {
			label      string
			start, end int64
		}{
			{"late", 100, 200},
			{"early", 0, 50},
			{"mid", 50, 120},
		}
		if reverse {
			for i, j := 0, len(spans)-1; i < j; i, j = i+1, j-1 {
				spans[i], spans[j] = spans[j], spans[i]
			}
		}
		for _, span := range spans {
			collector.append(scope, span.label, nil, base.Add(time.Duration(span.start)*time.Millisecond), base.Add(time.Duration(span.end)*time.Millisecond), "ok")
		}
		return collector.snapshot()
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
	var metrics FullDemoTimingMetrics
	if err := json.Unmarshal(forward, &metrics); err != nil {
		t.Fatal(err)
	}
	var labels []string
	for _, span := range metrics.Spans {
		labels = append(labels, span.Label)
	}
	if strings.Join(labels, ",") != "early,mid,late" {
		t.Fatalf("span order = %v", labels)
	}
	if metrics.Stages[0].WallMS != 200 {
		t.Fatalf("union = %d, want 200", metrics.Stages[0].WallMS)
	}
}

// runDiagnosticFFmpeg must record the same interval/outcome as its trace, with
// the attempt annotation of the scope, an inferred encoder and the native vs
// recovery variant. A failed attempt is "error"; a cancelled context is
// "cancelled" so they stay distinguishable.
func TestFullDemoTimingRecordsOutcomeAndAttempt(t *testing.T) {
	ffmpeg := fullDemoTestFFmpeg(t)
	ctx, collector := withFullDemoTimingCollector(context.Background())

	okCtx := fullDemoTimingScope(ctx, "assembly", 3, -1, 2.5)
	if _, err := runFFmpegOutput(okCtx, []string{ffmpeg, "-version"}, "timing ok"); err != nil {
		t.Fatal(err)
	}
	missing := filepath.Join(t.TempDir(), "missing-input.nut")
	errorCtx := fullDemoTimingStageVariant(fullDemoTimingScope(ctx, "audio_candidate_encode", 3, 1, 2.5), "audio_candidate_encode", 1, "", "native")
	// The encoder is not named by the scope; it must be inferred from the args.
	if _, err := runFFmpegOutput(errorCtx, []string{ffmpeg, "-c:a", "aac", "-i", missing, "-f", "null", "-"}, "timing error"); err == nil {
		t.Fatal("missing input unexpectedly succeeded")
	}
	cancelledCtx, cancel := context.WithCancel(ctx)
	cancelledCtx = fullDemoTimingStageVariant(fullDemoTimingScope(cancelledCtx, "audio_candidate_encode", 3, 2, 2.5), "audio_candidate_encode", 2, "aac_mf", "recovery")
	cancel()
	if _, err := runFFmpegOutput(cancelledCtx, []string{ffmpeg, "-version"}, "timing cancelled"); err == nil {
		t.Fatal("cancelled command unexpectedly succeeded")
	}

	metrics := collector.snapshot()
	if metrics == nil {
		t.Fatal("no timing metrics")
	}
	byLabel := map[string]FullDemoTimingSpan{}
	for _, span := range metrics.Spans {
		byLabel[span.Label] = span
		if span.Index != 3 || span.MediaDuration != 2.5 {
			t.Fatalf("span lost its index or media duration: %+v", span)
		}
		if span.ElapsedMS < 0 || span.EndMS < span.StartMS {
			t.Fatalf("invalid span interval: %+v", span)
		}
	}
	for _, want := range []struct {
		label   string
		attempt int
		outcome string
		stage   string
		variant string
		encoder string
	}{
		{"timing ok", -1, "ok", "assembly", "", ""},
		{"timing error", 1, "error", "audio_candidate_encode", "native", "aac"},
		{"timing cancelled", 2, "cancelled", "audio_candidate_encode", "recovery", "aac_mf"},
	} {
		span, ok := byLabel[want.label]
		if !ok {
			t.Fatalf("missing span %q: %+v", want.label, metrics.Spans)
		}
		if span.Attempt != want.attempt || span.Outcome != want.outcome || span.Stage != want.stage || span.Variant != want.variant || span.Encoder != want.encoder {
			t.Fatalf("span %q = %+v, want attempt=%d outcome=%s stage=%s variant=%s encoder=%s", want.label, span, want.attempt, want.outcome, want.stage, want.variant, want.encoder)
		}
	}
}

func TestFullDemoTimingEncoderInference(t *testing.T) {
	for _, tc := range []struct {
		name string
		args []string
		want string
	}{
		{"nvenc", []string{"ffmpeg", "-c:v", "h264_nvenc", "-i", "in", "out.nut"}, "h264_nvenc"},
		{"software", []string{"ffmpeg", "-c:v", "libx264", "out.nut"}, "libx264"},
		{"aac", []string{"ffmpeg", "-map", "0:a", "-c:a", "aac", "out.m4a"}, "aac"},
		{"aac_mf", []string{"ffmpeg", "-c:a", "aac_mf", "out.m4a"}, "aac_mf"},
		{"unknown codec", []string{"ffmpeg", "-c:v", "mystery", "out.nut"}, ""},
		{"no codec", []string{"ffmpeg", "-version"}, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := fullDemoTimingEncoder(tc.args); got != tc.want {
				t.Fatalf("encoder = %q, want %q", got, tc.want)
			}
		})
	}
}

// Without an attached collector the helpers must be exact no-ops so non-FullDemo
// renders and direct unit calls keep their behavior.
func TestFullDemoTimingNilCollectorPreservesBehavior(t *testing.T) {
	base := context.Background()
	if got := fullDemoTimingScope(base, "items", 0, -1, 1); got != base {
		t.Fatal("fullDemoTimingScope changed a context without a collector")
	}
	if got := fullDemoTimingStage(base, "items", 0); got != base {
		t.Fatal("fullDemoTimingStage changed a context without a collector")
	}
	if got := fullDemoTimingStageEncoder(base, "items", 0, "aac"); got != base {
		t.Fatal("fullDemoTimingStageEncoder changed a context without a collector")
	}
	fullDemoTimingRecord(base, "no collector", nil, time.Now(), time.Now(), nil)
	ffmpeg := fullDemoTestFFmpeg(t)
	if _, err := runFFmpegOutput(base, []string{ffmpeg, "-version"}, "no collector"); err != nil {
		t.Fatal(err)
	}
}

// The first snapshot freezes the collector: a late callback after publication
// must not mutate the persisted metrics.
func TestFullDemoTimingSnapshotClosesCollector(t *testing.T) {
	collector := newFullDemoTimingCollector()
	scope := &fullDemoTimingAnnotation{collector: collector, stage: "items", index: 0, attempt: -1}
	collector.append(scope, "first", nil, collector.start, collector.start.Add(10*time.Millisecond), "ok")
	first := collector.snapshot()
	if first == nil || len(first.Spans) != 1 {
		t.Fatalf("first snapshot: %+v", first)
	}
	collector.append(scope, "late", nil, collector.start.Add(20*time.Millisecond), collector.start.Add(30*time.Millisecond), "ok")
	if second := collector.snapshot(); second != nil {
		t.Fatal("closed collector produced a second snapshot")
	}
	if len(first.Spans) != 1 {
		t.Fatalf("late span leaked into the frozen snapshot: %+v", first.Spans)
	}
}

func TestRenderPerformanceOmitsTimingWithoutFullDemo(t *testing.T) {
	body, err := json.Marshal(RenderPerformance{RenderMS: 5})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(body), "full_demo_timing") {
		t.Fatalf("non-FullDemo performance carries timing: %s", body)
	}
	body, err = json.Marshal(RenderPerformance{FullDemoTiming: &FullDemoTimingMetrics{
		Stages: []FullDemoTimingStage{{Stage: "items", Spans: 1, WallMS: 4, ProcessElapsedSumMS: 4}},
	}})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(body), `"full_demo_timing"`) {
		t.Fatalf("FullDemo performance omitted timing: %s", body)
	}
}

// End-to-end proof: a real Full Demo master records every audio stage from the
// existing diagnostic execution, with the render index and attempt.
func TestFullDemoTimingAudioStagesRecorded(t *testing.T) {
	ffmpeg := fullDemoTestFFmpeg(t)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	dir := t.TempDir()
	const frames = 240
	input, duration := fullDemoAudioTestProgram(t, ctx, ffmpeg, dir, frames, fullDemoAudioTestSteady, "320x180")
	ctx, collector := withFullDemoTimingCollector(ctx)
	ctx = fullDemoTimingScope(ctx, "full_demo", 0, -1, duration)
	target := recapplan.DefaultOptions().Audio.Loudness
	output := filepath.Join(dir, "final.mp4")
	if _, err := masterFullDemoProgram(ctx, ffmpeg, input, output, filepath.Join(dir, "logs"), target, false, duration, nil); err != nil {
		t.Fatal(err)
	}
	metrics := collector.snapshot()
	if metrics == nil {
		t.Fatal("no timing metrics were recorded")
	}
	stages := map[string]FullDemoTimingStage{}
	for _, stage := range metrics.Stages {
		stages[stage.Stage] = stage
	}
	for _, want := range []string{"audio_input_analysis", "audio_candidate_encode", "audio_candidate_analysis", "final_mux", "final_audio_analysis"} {
		if _, ok := stages[want]; !ok {
			t.Fatalf("missing stage %q: %+v", want, stages)
		}
	}
	for _, span := range metrics.Spans {
		if span.Outcome != "ok" {
			t.Fatalf("unexpected outcome: %+v", span)
		}
		if span.Index != 0 || span.MediaDuration != duration {
			t.Fatalf("audio span lost render identity: %+v", span)
		}
	}
	// Native candidates must be clearly native, with the encoder inferred from
	// the command when the stage did not name it.
	var encode, encodeAnalysis FullDemoTimingSpan
	for _, span := range metrics.Spans {
		switch span.Stage {
		case "audio_candidate_encode":
			encode = span
		case "audio_candidate_analysis":
			encodeAnalysis = span
		}
	}
	if encode.Variant != "native" || encode.Encoder != "aac" || encode.Attempt != 0 {
		t.Fatalf("native candidate encode identity: %+v", encode)
	}
	if encodeAnalysis.Variant != "native" || encodeAnalysis.Encoder != "aac" || encodeAnalysis.Attempt != 0 {
		t.Fatalf("native candidate analysis identity: %+v", encodeAnalysis)
	}
	// WallMS is the interval union and ProcessElapsedSumMS the process-elapsed
	// sum. A single serial span may differ by at most a millisecond from
	// independent rounding of the start/end offsets; the union can never exceed
	// the sum by more than that per span.
	candidateStage := stages["audio_candidate_encode"]
	if candidateStage.WallMS <= 0 || candidateStage.ProcessElapsedSumMS <= 0 {
		t.Fatalf("candidate stage has no timing: %+v", candidateStage)
	}
	if candidateStage.WallMS > candidateStage.ProcessElapsedSumMS+int64(candidateStage.Spans)+1 {
		t.Fatalf("interval union exceeds process elapsed sum: %+v", candidateStage)
	}
	// The published output must still exist; timing evidence never replaces it.
	if _, err := os.Stat(output); err != nil {
		t.Fatal(err)
	}
}
