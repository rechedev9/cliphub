package main

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/csv"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/rechedev9/cliphub/internal/killplan"
	"github.com/rechedev9/cliphub/internal/pathguard"
	"github.com/rechedev9/cliphub/internal/recording"
)

func main() {
	err := run()
	if err == nil {
		return
	}
	// Plain stderr, no log timestamps: the zv wrapper forwards this text as the
	// error field of its JSON envelope.
	var hookErr *hookIncompatibleError
	if errors.As(err, &hookErr) {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(exitHookIncompatible)
	}
	var demoErr *demoParseError
	if errors.As(err, &demoErr) {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(exitDemoIncompatible)
	}
	var startErr *unplayableStartError
	if errors.As(err, &startErr) {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(exitUnplayableStart)
	}
	fmt.Fprintf(os.Stderr, "error: %v\n", err)
	os.Exit(1)
}

func run() error {
	var (
		killPlanPath         = flag.String("killplan", "", "path to kill plan JSON")
		demoPath             = flag.String("demo", "", "path to .dem file")
		outDir               = flag.String("out", "", "recording output directory")
		hlaeExe              = flag.String("hlae", "", "path to HLAE.exe")
		cs2Exe               = flag.String("cs2", "", "path to cs2.exe")
		hudMode              = flag.String("hud", string(recording.HUDModeGameplay), "HUD mode: gameplay, clean, or deathnotices")
		portraitSafeKillfeed = flag.Bool("portrait-safe-killfeed", false, "move filtered death notices into the 9:16 center-crop safe area")
		fps                  = flag.Int("fps", 0, "recording FPS; defaults to recorder preset")
		videoCRF             = flag.Int("video-crf", 0, "HLAE stream CRF; defaults to recorder preset")
		encoder              = flag.String("encoder", "", "HLAE stream encoder: libx264, nvenc-h264, amf-h264, or qsv-h264 (empty = libx264)")
		gapTimescale         = flag.Float64("gap-timescale", 0, "demo_timescale across unrecorded gaps; 0 = default 8, 1 = disable speedup")
		settleSeconds        = flag.Float64("settle-seconds", 0, "seconds of 1x playback before each record window; 0 = default 2s")
		dryRun               = flag.Bool("dry-run", false, "generate plan and script without launching HLAE")
		format               = flag.String("format", "text", "result summary format: text or json")
		fake                 = flag.Bool("fake", false, "generate placeholder segment clips instead of launching HLAE/CS2 (e2e/CI)")
		timeout              = flag.Duration("timeout", 15*time.Minute, "maximum duration to wait for CS2")
	)
	flag.Parse()

	fakeMode := *fake

	if *killPlanPath == "" || *demoPath == "" || *outDir == "" {
		return fmt.Errorf("--killplan, --demo, and --out are required")
	}
	if *format != "text" && *format != "json" {
		return fmt.Errorf("unsupported format %q", *format)
	}
	if !*dryRun && !fakeMode && (*hlaeExe == "" || *cs2Exe == "") {
		return fmt.Errorf("--hlae and --cs2 are required unless --dry-run is set")
	}

	absKillPlanPath, err := filepath.Abs(*killPlanPath)
	if err != nil {
		return fmt.Errorf("resolve killplan path: %w", err)
	}
	absDemoPath, err := filepath.Abs(*demoPath)
	if err != nil {
		return fmt.Errorf("resolve demo path: %w", err)
	}
	absOutDir, err := filepath.Abs(*outDir)
	if err != nil {
		return fmt.Errorf("resolve output path: %w", err)
	}
	if err := validateRecordingOutputDirectory(absOutDir, absKillPlanPath, absDemoPath); err != nil {
		return err
	}
	absHLAEExe := *hlaeExe
	absCS2Exe := *cs2Exe
	if !*dryRun && !fakeMode {
		absHLAEExe, err = filepath.Abs(*hlaeExe)
		if err != nil {
			return fmt.Errorf("resolve HLAE path: %w", err)
		}
		absCS2Exe, err = filepath.Abs(*cs2Exe)
		if err != nil {
			return fmt.Errorf("resolve CS2 path: %w", err)
		}
	}

	kp, err := readKillPlan(absKillPlanPath)
	if err != nil {
		return err
	}
	if err := validateKillPlanDemo(kp, absDemoPath); err != nil {
		return err
	}
	if !*dryRun {
		if err := validateFreshOutputNamespace(absOutDir); err != nil {
			return err
		}
	}
	stream := recording.DefaultStreamConfig()
	stream.Encoder = *encoder
	stream.HUDMode = recording.HUDMode(*hudMode)
	stream.PortraitSafeKillfeed = *portraitSafeKillfeed
	if *fps > 0 {
		stream.FPS = *fps
	}
	if *videoCRF < 0 {
		return fmt.Errorf("--video-crf must be between 1 and 51, or 0 for default")
	}
	if *videoCRF > 0 {
		stream.CRF = *videoCRF
	}
	plan, err := recording.NewPlanFromKillPlan(kp, absDemoPath, absOutDir, stream)
	if err != nil {
		return err
	}
	if err := applyRuntimeFlags(&plan, *gapTimescale, *settleSeconds); err != nil {
		return err
	}
	runStarted := time.Now()

	if err := os.MkdirAll(plan.OutputDir, 0o750); err != nil {
		return err
	}
	attestationToken := ""
	var script string
	if !*dryRun && !fakeMode {
		attestationToken, err = newCaptureAttestationToken()
		if err != nil {
			return err
		}
		script, err = recording.GenerateHLAEJavaScriptWithAttestation(plan, attestationToken)
	} else {
		script, err = recording.GenerateHLAEJavaScript(plan)
	}
	if err != nil {
		return err
	}
	scriptPath := filepath.Join(plan.OutputDir, "recording.js")
	if err := os.WriteFile(scriptPath, []byte(script), 0o600); err != nil {
		return err
	}

	captureFingerprint, err := recording.CaptureInputFingerprint(plan)
	if err != nil {
		return err
	}
	result := recording.RecordingResult{
		Plan:                    plan,
		Script:                  scriptPath,
		CaptureMode:             recording.CaptureModeReal,
		CaptureInputFingerprint: captureFingerprint,
	}
	perfRun := recording.RecordingRunPerformance{
		CaptureSegmentIDs: captureSegmentIDs(plan),
		Stream:            plan.Stream,
	}
	trace := newPerformanceTrace(runStarted, &perfRun)
	finalizePerformance := func() {
		perfRun.BeforeResultWriteMS = elapsedMilliseconds(runStarted)
		perfRun.Segments = summarizeSegmentPerformance(perfRun, result.Artifacts)
		result.Performance = &recording.RecordingPerformance{
			Version: 1,
			Runs:    []recording.RecordingRunPerformance{perfRun},
		}
	}

	if *dryRun {
		result.CaptureMode = recording.CaptureModeDryRun
		return writeResultAndReport(plan.OutputDir, result, true, *format, os.Stdout)
	}

	if fakeMode {
		result.CaptureMode = recording.CaptureModeFake
		fakeCtx, cancelFake := context.WithTimeout(context.Background(), *timeout)
		defer cancelFake()
		artifacts, err := generateFakeSegments(fakeCtx, plan)
		if err != nil {
			result.Error = err.Error()
			_ = writeResult(plan.OutputDir, result)
			return err
		}
		result.Artifacts = artifacts
		result.Warnings = recording.ValidateArtifacts(plan, result.Artifacts)
		return writeResultAndReport(plan.OutputDir, result, false, *format, os.Stdout)
	}

	ffprobePath := recording.FindFFprobe()
	ffmpegPath := recording.FindFFmpeg()

	if err := validateExecutables(absHLAEExe, absCS2Exe); err != nil {
		result.Error = err.Error()
		_ = writeResult(plan.OutputDir, result)
		return err
	}
	if err := ensureDefaultAvatar(absCS2Exe); err != nil {
		result.Error = err.Error()
		_ = writeResult(plan.OutputDir, result)
		return err
	}
	if err := ensureHLAEFFmpegConfig(absHLAEExe); err != nil {
		result.Error = err.Error()
		_ = writeResult(plan.OutputDir, result)
		return err
	}
	if err := recording.CheckEncoderSupported(recording.HLAEStreamFFmpeg(absHLAEExe), plan.Stream.Encoder); err != nil {
		result.Error = err.Error()
		_ = writeResult(plan.OutputDir, result)
		return err
	}

	// CS2 must record in a real window: the player's own video settings
	// (fullscreen / borderless) override the -windowed launch flag and turn the
	// capture into a borderless topmost screen-sized window that hijacks the
	// desktop and glitches more than a plain window. Patch the saved settings
	// for the run and put the originals back afterwards. validateExecutables
	// already guaranteed cs2.exe is not running, so the file is safe to edit.
	restoreVideoConfig := forceWindowedVideoConfig(absCS2Exe)
	defer restoreVideoConfig()

	ctx, cancel := context.WithTimeout(context.Background(), *timeout)
	defer cancel()

	// Publish each segment clip while HLAE is still recording so observers (the
	// orchestrator's capture-progress poll) see segments as they finish instead
	// of only after the whole run. Owned by this run: cancelled and waited for
	// as soon as the capture process exits, and strictly best-effort — the
	// post-run MuxSegmentClips pass below re-muxes anything still missing.
	muxCtx, stopMux := context.WithCancel(ctx)
	muxDone := make(chan struct{})
	go func() {
		defer close(muxDone)
		muxer := recording.NewIncrementalMuxer(plan, ffmpegPath)
		ticker := time.NewTicker(2 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-muxCtx.Done():
				return
			case <-ticker.C:
				started := time.Now()
				for _, id := range muxer.MuxFinished(muxCtx) {
					log.Printf("segment %s recorded", id)
				}
				perfRun.IncrementalMuxMS += elapsedMilliseconds(started)
			}
		}
	}()
	stopIncrementalMux := func() {
		stopMux()
		<-muxDone
	}

	perfRun.PrepareMS = elapsedMilliseconds(runStarted)
	captureStarted := time.Now()
	if err := launchAndWait(ctx, absHLAEExe, absCS2Exe, plan, scriptPath, attestationToken, trace); err != nil {
		perfRun.LaunchAndCaptureMS = elapsedMilliseconds(captureStarted)
		stopIncrementalMux()
		result.Error = err.Error()
		// Preserve completed takes before returning the capture failure. Avoid
		// probing partial files here: a single stuck ffprobe must not consume the
		// fresh recovery budget before FFmpeg can publish usable segment clips.
		probeStarted := time.Now()
		result.Artifacts = recording.CollectArtifacts(context.Background(), plan, "")
		perfRun.ArtifactProbeMS = elapsedMilliseconds(probeStarted)
		recoveryCtx, recoveryCancel := context.WithTimeout(context.Background(), 30*time.Second)
		muxStarted := time.Now()
		result.Artifacts = append(result.Artifacts, recording.MuxSegmentClips(recoveryCtx, plan, result.Artifacts, ffmpegPath, "")...)
		perfRun.FinalMuxMS = elapsedMilliseconds(muxStarted)
		recoveryCancel()
		validationStarted := time.Now()
		result.Warnings = recording.ValidateArtifacts(plan, result.Artifacts)
		perfRun.ValidationMS = elapsedMilliseconds(validationStarted)
		finalizePerformance()
		_ = writeResult(plan.OutputDir, result)
		return err
	}
	perfRun.LaunchAndCaptureMS = elapsedMilliseconds(captureStarted)
	result.CaptureVerified = true
	stopIncrementalMux()

	// Post-processing (ffprobe/ffmpeg) runs after recording, so give it its own
	// timeout budget: bounded so a hung subprocess cannot run indefinitely, but
	// not the recording's leftover budget — a capture that legitimately consumed
	// most of --timeout would otherwise starve muxing and drop its segment clips.
	postCtx, postCancel := context.WithTimeout(context.Background(), *timeout)
	defer postCancel()
	probeStarted := time.Now()
	result.Artifacts = recording.CollectArtifacts(postCtx, plan, ffprobePath)
	perfRun.ArtifactProbeMS = elapsedMilliseconds(probeStarted)
	muxStarted := time.Now()
	result.Artifacts = append(result.Artifacts, recording.MuxSegmentClips(postCtx, plan, result.Artifacts, ffmpegPath, ffprobePath)...)
	perfRun.FinalMuxMS = elapsedMilliseconds(muxStarted)
	validationStarted := time.Now()
	result.Warnings = recording.ValidateArtifacts(plan, result.Artifacts)
	if err := validateCaptureResult(result, absCS2Exe); err != nil {
		perfRun.ValidationMS = elapsedMilliseconds(validationStarted)
		finalizePerformance()
		result.Error = err.Error()
		_ = writeResult(plan.OutputDir, result)
		return err
	}
	perfRun.ValidationMS = elapsedMilliseconds(validationStarted)
	finalizePerformance()
	return writeResultAndReport(plan.OutputDir, result, false, *format, os.Stdout)
}

func elapsedMilliseconds(started time.Time) int64 {
	return time.Since(started).Milliseconds()
}

// applyRuntimeFlags validates the --gap-timescale and --settle-seconds values
// and overwrites the plan's runtime timing overrides. Zero means "keep the
// default": the plan already normalized PlaybackTimescale to the default, and
// PlaybackSettleSeconds stays 0 so Normalized() maps it to the default.
func applyRuntimeFlags(plan *recording.RecordingPlan, gapTimescale, settleSeconds float64) error {
	if gapTimescale < 0 {
		return fmt.Errorf("--gap-timescale must be non-negative")
	}
	if settleSeconds < 0 {
		return fmt.Errorf("--settle-seconds must be non-negative")
	}
	if gapTimescale > 0 {
		plan.Runtime.PlaybackTimescale = gapTimescale
	}
	if settleSeconds > 0 {
		plan.Runtime.PlaybackSettleSeconds = settleSeconds
	}
	return nil
}

func captureSegmentIDs(plan recording.RecordingPlan) []string {
	ids := make([]string, 0, len(plan.Segments))
	for _, segment := range plan.Segments {
		if segment.ID != "" {
			ids = append(ids, segment.ID)
		}
	}
	return ids
}

func summarizeSegmentPerformance(run recording.RecordingRunPerformance, artifacts []recording.RecordingArtifact) []recording.RecordingSegmentPerformance {
	summaries := make(map[string]recording.RecordingSegmentPerformance, len(run.CaptureSegmentIDs))
	for _, id := range run.CaptureSegmentIDs {
		summaries[id] = recording.RecordingSegmentPerformance{SegmentID: id}
	}
	for _, event := range run.Events {
		summary, ok := summaries[event.SegmentID]
		if !ok {
			continue
		}
		switch event.Kind {
		case "record_start_requested_observed":
			summary.RecordStartObservedMS = event.ElapsedMS
		case "record_end_requested_observed":
			summary.RecordEndObservedMS = event.ElapsedMS
		}
		summaries[event.SegmentID] = summary
	}
	video := make(map[string]recording.RecordingArtifact, len(summaries))
	for _, artifact := range artifacts {
		if artifact.SegmentID == "" || artifact.Type != "video" {
			continue
		}
		current, exists := video[artifact.SegmentID]
		if !exists || (current.Role != "segment" && artifact.Role == "segment") {
			video[artifact.SegmentID] = artifact
		}
	}
	out := make([]recording.RecordingSegmentPerformance, 0, len(run.CaptureSegmentIDs))
	for _, id := range run.CaptureSegmentIDs {
		summary := summaries[id]
		if summary.RecordEndObservedMS > summary.RecordStartObservedMS {
			summary.RequestedActiveMS = summary.RecordEndObservedMS - summary.RecordStartObservedMS
		}
		if artifact, ok := video[id]; ok {
			summary.VideoFrameCount = artifact.FrameCount
			summary.VideoDurationSeconds = artifact.DurationSeconds
		}
		if summary.VideoFrameCount > 0 && summary.RequestedActiveMS > 0 {
			summary.ObservedFramesPerSecond = float64(summary.VideoFrameCount) * 1000 / float64(summary.RequestedActiveMS)
		}
		out = append(out, summary)
	}
	return out
}

func validateRecordingOutputDirectory(outDir, killPlanPath, demoPath string) error {
	return pathguard.RejectInputsWithinDirectory(outDir,
		pathguard.Input{Flag: "--killplan", Path: killPlanPath},
		pathguard.Input{Flag: "--demo", Path: demoPath},
	)
}

// generateFakeSegments produces one instrumented MP4 per plan segment so the
// downstream compose/render pipeline and Capture Lab media oracle can run
// without launching HLAE/CS2. The explicit --fake flag is the only entry point;
// results retain capture_mode=fake and remain ineligible for production reuse.
func generateFakeSegments(ctx context.Context, plan recording.RecordingPlan) ([]recording.RecordingArtifact, error) {
	ffmpeg := recording.FindFFmpeg()
	if ffmpeg == "" {
		return nil, fmt.Errorf("ffmpeg not found (required for fake recording)")
	}
	segDir := filepath.Join(plan.OutputDir, "segments")
	if err := os.MkdirAll(segDir, 0o750); err != nil {
		return nil, err
	}
	const fakeDurationSec = 5
	width, height, fps := plan.Stream.Width, plan.Stream.Height, plan.Stream.FPS
	if width <= 0 || height <= 0 {
		width, height = 1920, 1080
	}
	if fps <= 0 {
		fps = 60
	}
	instrumentation := captureLabInstrumentation{SchemaVersion: 1, CaptureMode: string(recording.CaptureModeFake)}
	out := make([]recording.RecordingArtifact, 0, len(plan.Segments))
	for index, seg := range plan.Segments {
		clip := filepath.Join(segDir, seg.ID+".mp4")
		identity := captureLabSegmentIdentity(seg.ID, index)
		eventOffsets := captureLabEventOffsets(seg, plan, fakeDurationSec)
		videoFilter := fmt.Sprintf(
			"testsrc2=size=%dx%d:rate=%d:duration=%d,drawbox=x=0:y=0:w=iw:h=80:color=0x%s:t=fill",
			width, height, fps, fakeDurationSec, identity.ColorHex,
		)
		for _, offset := range eventOffsets {
			videoFilter += fmt.Sprintf(",drawbox=x=0:y=ih-100:w=iw:h=100:color=white:t=fill:enable='between(t,%.3f,%.3f)'", offset, offset+0.12)
		}
		// #nosec G204 -- ffmpeg path is discovered locally; args are not shell-interpolated.
		cmd := exec.CommandContext(ctx, ffmpeg, "-y", "-v", "error",
			"-f", "lavfi", "-i", videoFilter,
			"-f", "lavfi", "-i", fmt.Sprintf("sine=frequency=%d:sample_rate=48000:duration=%d", identity.ToneHz, fakeDurationSec),
			"-metadata", "comment=cliphub-capturelab:"+seg.ID,
			"-metadata:s:v:0", "title=cliphub-capturelab:"+seg.ID,
			"-c:v", "libx264", "-pix_fmt", "yuv420p", "-preset", "ultrafast",
			"-c:a", "aac", "-shortest", "-movflags", "+faststart", clip,
		)
		if combined, err := cmd.CombinedOutput(); err != nil {
			return nil, fmt.Errorf("ffmpeg fake clip %q: %w: %q", seg.ID, err, strings.TrimSpace(string(combined)))
		}
		info, err := os.Stat(clip)
		if err != nil {
			return nil, err
		}
		out = append(out, recording.RecordingArtifact{
			SegmentID:       seg.ID,
			Role:            "segment",
			Type:            "video",
			Path:            clip,
			SizeBytes:       info.Size(),
			DurationSeconds: fakeDurationSec,
			FrameRate:       fmt.Sprintf("%d/1", fps),
			Codec:           "h264",
			Width:           width,
			Height:          height,
			SampleRate:      48000,
			Channels:        1,
		})
		instrumentation.Segments = append(instrumentation.Segments, captureLabInstrumentedSegment{
			ID:              seg.ID,
			Path:            clip,
			ColorRGB:        identity.ColorHex,
			ToneHz:          identity.ToneHz,
			DurationSeconds: fakeDurationSec,
			EventOffsets:    eventOffsets,
		})
	}
	instrumentationPath := filepath.Join(plan.OutputDir, "capturelab-instrumentation.json")
	encoded, err := json.MarshalIndent(instrumentation, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("encode Capture Lab instrumentation: %w", err)
	}
	if err := os.WriteFile(instrumentationPath, append(encoded, '\n'), 0o600); err != nil {
		return nil, fmt.Errorf("write Capture Lab instrumentation: %w", err)
	}
	return out, nil
}

type captureLabInstrumentation struct {
	SchemaVersion int                             `json:"schema_version"`
	CaptureMode   string                          `json:"capture_mode"`
	Segments      []captureLabInstrumentedSegment `json:"segments"`
}

type captureLabInstrumentedSegment struct {
	ID              string    `json:"id"`
	Path            string    `json:"path"`
	ColorRGB        string    `json:"color_rgb"`
	ToneHz          int       `json:"tone_hz"`
	DurationSeconds float64   `json:"duration_seconds"`
	EventOffsets    []float64 `json:"event_offsets"`
}

type captureLabIdentity struct {
	ColorHex string
	ToneHz   int
}

func captureLabSegmentIdentity(segmentID string, index int) captureLabIdentity {
	sum := sha256.Sum256([]byte(segmentID))
	// Avoid nearly-black identity bars so decoded-pixel checks remain robust
	// after H.264 chroma subsampling and color-space conversion.
	colorHex := fmt.Sprintf("%02x%02x%02x", 64+sum[0]%160, 64+sum[1]%160, 64+sum[2]%160)
	return captureLabIdentity{ColorHex: colorHex, ToneHz: 300 + (int(sum[3])+index*97)%600}
}

func captureLabEventOffsets(segment recording.RecordingSegment, plan recording.RecordingPlan, duration float64) []float64 {
	start := recording.EffectiveRecordStartTick(segment, plan.Tickrate)
	end := recording.EffectiveRecordEndTick(segment, plan)
	if end <= start || duration <= 0 {
		return nil
	}
	var offsets []float64
	for _, kill := range segment.Kills {
		offset := float64(kill.Tick-start) / float64(end-start) * duration
		if offset < 0 {
			offset = 0
		}
		if maximum := duration - 0.15; offset > maximum {
			offset = maximum
		}
		offsets = append(offsets, offset)
	}
	return offsets
}

func readKillPlan(path string) (killplan.Plan, error) {
	// #nosec G304 -- kill plan path is an explicit local CLI input.
	b, err := os.ReadFile(path)
	if err != nil {
		return killplan.Plan{}, err
	}
	var p killplan.Plan
	if err := json.Unmarshal(b, &p); err != nil {
		return killplan.Plan{}, err
	}
	return p, nil
}

func validateKillPlanDemo(plan killplan.Plan, demoPath string) error {
	if plan.SchemaVersion != killplan.SchemaVersion {
		return fmt.Errorf("kill plan schema_version must be %q", killplan.SchemaVersion)
	}
	if len(plan.Demo.SHA256) != sha256.Size*2 {
		return fmt.Errorf("kill plan demo sha256 must be a 64-character digest")
	}
	// #nosec G304 -- demoPath is the explicit local capture input and its content is SHA-bound below.
	file, err := os.Open(demoPath)
	if err != nil {
		return fmt.Errorf("open demo for SHA-256 validation: %w", err)
	}
	hash := sha256.New()
	_, copyErr := io.Copy(hash, file)
	closeErr := file.Close()
	if copyErr != nil {
		return fmt.Errorf("hash demo: %w", copyErr)
	}
	if closeErr != nil {
		return fmt.Errorf("close demo after hashing: %w", closeErr)
	}
	actual := hex.EncodeToString(hash.Sum(nil))
	if plan.Demo.SHA256 != actual {
		return fmt.Errorf("kill plan demo sha256 does not match --demo")
	}
	return nil
}

func validateFreshOutputNamespace(outDir string) error {
	entries, err := os.ReadDir(outDir)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("inspect recording output directory: %w", err)
	}
	allowed := map[string]bool{
		"recording.js":          true,
		"recording-result.json": true,
	}
	for _, entry := range entries {
		if allowed[entry.Name()] && entry.Type().IsRegular() {
			continue
		}
		return fmt.Errorf("recording output directory contains stale artifact %q; use a fresh output directory", entry.Name())
	}
	return nil
}

func validateExecutables(hlaeExe, cs2Exe string) error {
	if _, err := os.Stat(hlaeExe); err != nil {
		return fmt.Errorf("HLAE not found: %w", err)
	}
	if _, err := os.Stat(cs2Exe); err != nil {
		return fmt.Errorf("CS2 not found: %w", err)
	}
	if _, err := locateHookDLL(hlaeExe); err != nil {
		return err
	}
	if runtime.GOOS == "windows" {
		running, err := processRunning("cs2.exe")
		if err != nil {
			return err
		}
		if running {
			return fmt.Errorf("cs2.exe is already running; close it before recording")
		}
	}
	return nil
}

func ensureDefaultAvatar(cs2Exe string) error {
	gameDir := filepath.Clean(filepath.Join(filepath.Dir(cs2Exe), "..", ".."))
	avatarPath := filepath.Join(gameDir, "csgo", "avatars", "default.png")
	if _, err := os.Stat(avatarPath); err == nil {
		return nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("inspect default CS2 avatar: %w", err)
	}

	avatarDir := filepath.Dir(avatarPath)
	if err := os.MkdirAll(avatarDir, 0o750); err != nil {
		return fmt.Errorf("create default CS2 avatar directory: %w", err)
	}

	file, err := os.CreateTemp(avatarDir, "default-*.png")
	if err != nil {
		return fmt.Errorf("create default CS2 avatar: %w", err)
	}
	tempPath := file.Name()
	defer os.Remove(tempPath)

	img := image.NewNRGBA(image.Rect(0, 0, 32, 32))
	draw.Draw(img, img.Bounds(), &image.Uniform{C: color.NRGBA{R: 64, G: 72, B: 88, A: 255}}, image.Point{}, draw.Src)
	if err := png.Encode(file, img); err != nil {
		_ = file.Close()
		return fmt.Errorf("encode default CS2 avatar: %w", err)
	}
	if err := file.Close(); err != nil {
		return fmt.Errorf("close default CS2 avatar: %w", err)
	}
	if err := os.Rename(tempPath, avatarPath); err != nil {
		return fmt.Errorf("install default CS2 avatar: %w", err)
	}
	log.Printf("installed missing CS2 default avatar at %s", avatarPath)
	return nil
}

func newCaptureAttestationToken() (string, error) {
	var token [32]byte
	if _, err := rand.Read(token[:]); err != nil {
		return "", fmt.Errorf("generate capture attestation token: %w", err)
	}
	return hex.EncodeToString(token[:]), nil
}

func launchAndWait(ctx context.Context, hlaeExe, cs2Exe string, plan recording.RecordingPlan, scriptPath, attestationToken string, trace *performanceTrace) error {
	if runtime.GOOS != "windows" {
		return fmt.Errorf("HLAE/CS2 capture is supported only on Windows")
	}
	if attestationToken == "" {
		return fmt.Errorf("capture attestation token is required")
	}
	running, err := processRunning("cs2.exe")
	if err != nil {
		return fmt.Errorf("check for an existing cs2.exe before capture: %w", err)
	}
	if running {
		return fmt.Errorf("cs2.exe is already running; close it before starting an HLAE capture")
	}
	hook, err := locateHookDLL(hlaeExe)
	if err != nil {
		return err
	}
	consoleLogPath := cs2ConsoleLogPath(cs2Exe)
	if err := prepareCS2ConsoleLog(consoleLogPath); err != nil {
		return err
	}
	consoleLog := newCS2ConsoleLogMonitor(consoleLogPath, attestationToken)
	consoleLog.trace = trace
	cs2CmdLine := cs2LaunchCommandLine(plan, scriptPath)

	// A kill-on-close job object owns the launcher and, through it, cs2.exe. Its
	// deferred close guarantees the whole capture tree is torn down on every
	// return path, backstopping the grace-gate close below. Creation failure is
	// non-fatal: the grace gate stays the primary deterministic teardown.
	job, err := newCaptureJob()
	if err != nil {
		log.Printf("capture job object unavailable, relying on grace-gate close: %v", err)
	}
	defer func() {
		if err := job.close(); err != nil {
			log.Printf("capture job cleanup: %v", err)
		}
	}()

	// #nosec G204 -- HLAE/CS2 paths are explicit local tool paths and args are not shell-interpolated.
	cmd := exec.CommandContext(ctx, hlaeExe,
		"-customLoader",
		"-noGui",
		"-autoStart",
		"-hookDllPath", hook,
		"-programPath", cs2Exe,
		"-cmdLine", cs2CmdLine,
	)
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start HLAE: %w", err)
	}
	// Assign the launcher to the job before it spawns cs2.exe so the descendant
	// inherits the job. Assignment failure is non-fatal for the same reason.
	if err := job.assign(cmd.Process.Pid); err != nil {
		log.Printf("capture job assignment failed, relying on grace-gate close: %v", err)
	}
	if err := cmd.Wait(); err != nil {
		// A post-failure process-name lookup cannot prove that a newly visible
		// cs2.exe belongs to this launch. Never terminate by image name here:
		// doing so could close a process started concurrently by another user.
		// A cs2.exe this failed launcher did spawn is in the job, so the deferred
		// job close still tears it down — by membership, never by image name.
		return launcherFailure(err)
	}
	if err := waitForWindowsProcessRunAndExit(ctx, "cs2.exe", consoleLog); err != nil {
		return err
	}
	return consoleLog.requireCaptureVerified()
}

func cs2LaunchCommandLine(plan recording.RecordingPlan, scriptPath string) string {
	return fmt.Sprintf(`-insecure -condebug -windowed -w %d -h %d +cl_demo_predict 0 +playdemo "%s" +mirv_script_load "%s"`, plan.Stream.Width, plan.Stream.Height, plan.DemoPath, scriptPath)
}

// windowedVideoSettings are the CS2 saved video settings that must be off for
// the -windowed launch flag to yield a real bordered window instead of a
// borderless topmost screen-sized one.
var windowedVideoSettings = []string{
	"setting.fullscreen",
	"setting.coop_fullscreen",
	"setting.nowindowborder",
}

// forceWindowedVideoConfig patches every Steam cs2_video.txt so the capture
// runs in a real window, returning a restore func that puts the original
// bytes back after the run (a hard-killed recorder leaves the settings
// windowed, which the next capture re-patches harmlessly). Failures only log:
// a missing config means CS2 already follows the launch flags.
func forceWindowedVideoConfig(cs2Exe string) func() {
	originals := map[string][]byte{}
	for _, path := range cs2VideoConfigPaths(cs2Exe) {
		// #nosec G304 -- paths are discovered under the local Steam install.
		b, err := os.ReadFile(path)
		if err != nil {
			log.Printf("windowed capture: read %s: %v", path, err)
			continue
		}
		patched, changed := patchWindowedVideoSettings(string(b))
		if !changed {
			continue
		}
		// #nosec G703 -- paths are enumerated beneath the detected Steam userdata root.
		if err := os.WriteFile(path, []byte(patched), 0o600); err != nil {
			log.Printf("windowed capture: patch %s: %v", path, err)
			continue
		}
		log.Printf("windowed capture: patched %s (fullscreen/borderless off for this run)", path)
		originals[path] = b
	}
	return func() {
		for path, b := range originals {
			if err := os.WriteFile(path, b, 0o600); err != nil {
				log.Printf("windowed capture: restore %s: %v", path, err)
			}
		}
	}
}

// patchWindowedVideoSettings forces the fullscreen/borderless settings to "0"
// in a cs2_video.txt body, reporting whether anything changed. Settings that
// are absent are left absent; CS2 then follows the launch flags.
func patchWindowedVideoSettings(content string) (string, bool) {
	changed := false
	for _, key := range windowedVideoSettings {
		pattern := regexp.MustCompile(`("` + regexp.QuoteMeta(key) + `"\s+")([^"]*)(")`)
		next := pattern.ReplaceAllStringFunc(content, func(match string) string {
			groups := pattern.FindStringSubmatch(match)
			if groups[2] == "0" {
				return match
			}
			changed = true
			return groups[1] + "0" + groups[3]
		})
		content = next
	}
	return content, changed
}

// cs2VideoConfigPaths finds every cs2_video.txt under the Steam userdata
// roots: the install that owns cs2.exe (walking up from the executable) plus
// the default Steam location, since cs2 may live in a secondary library while
// userdata stays in the main install.
func cs2VideoConfigPaths(cs2Exe string) []string {
	var roots []string
	if dir := steamRootFromCS2Path(cs2Exe); dir != "" {
		roots = append(roots, dir)
	}
	if pf := os.Getenv("ProgramFiles(x86)"); pf != "" {
		roots = append(roots, filepath.Join(pf, "Steam"))
	}
	roots = append(roots, `C:\Program Files (x86)\Steam`)

	seen := map[string]bool{}
	var paths []string
	for _, root := range roots {
		matches, _ := filepath.Glob(filepath.Join(root, "userdata", "*", "730", "local", "cfg", "cs2_video.txt"))
		for _, match := range matches {
			if seen[match] {
				continue
			}
			seen[match] = true
			paths = append(paths, match)
		}
	}
	return paths
}

// steamRootFromCS2Path walks up from cs2.exe to the directory containing
// steamapps, i.e. the Steam (library) root. Returns "" when cs2.exe does not
// live under a steamapps tree.
func steamRootFromCS2Path(cs2Exe string) string {
	dir := filepath.Dir(cs2Exe)
	for {
		parent := filepath.Dir(dir)
		if parent == dir {
			return ""
		}
		if strings.EqualFold(filepath.Base(dir), "steamapps") {
			return parent
		}
		dir = parent
	}
}

func cs2ConsoleLogPath(cs2Exe string) string {
	gameDir := filepath.Dir(filepath.Dir(filepath.Dir(cs2Exe)))
	return filepath.Join(gameDir, "csgo", "console.log")
}

const demoParseFailureMarker = "NETWORK_DISCONNECT_MESSAGE_PARSE_ERROR"

type cs2ConsoleLogMonitor struct {
	path             string
	offset           int64
	tail             string
	lineTail         string
	trace            *performanceTrace
	captureVerified  bool
	failedMarker     string
	verifiedMarker   string
	armedAtZero      bool
	sawSeekLanded    bool
	sawResetBreakpad bool
}

func prepareCS2ConsoleLog(path string) error {
	if err := os.WriteFile(path, nil, 0o600); err != nil {
		return fmt.Errorf("reset cs2 console log %q: %w", path, err)
	}
	return nil
}

func newCS2ConsoleLogMonitor(path, attestationToken string) *cs2ConsoleLogMonitor {
	monitor := &cs2ConsoleLogMonitor{
		path:           path,
		failedMarker:   recording.CaptureFailedAttestation(attestationToken),
		verifiedMarker: recording.CaptureVerifiedAttestation(attestationToken),
	}
	if info, err := os.Stat(path); err == nil {
		monitor.offset = info.Size()
	}
	return monitor
}

// failure checks only console output written after this monitor was created.
// CS2 can truncate console.log at startup, so a smaller file resets the cursor.
func (m *cs2ConsoleLogMonitor) failure() error {
	file, err := os.Open(m.path)
	if err != nil {
		return nil
	}
	defer file.Close()

	info, err := file.Stat()
	if err != nil {
		return nil
	}
	if info.Size() < m.offset {
		m.offset = 0
		m.tail = ""
		m.lineTail = ""
	}
	if _, err := file.Seek(m.offset, io.SeekStart); err != nil {
		return nil
	}
	data, err := io.ReadAll(file)
	if err != nil {
		return nil
	}
	m.offset += int64(len(data))
	if len(data) == 0 {
		return nil
	}

	content := m.tail + string(data)
	m.consumePerformanceMarkers(string(data))
	m.consumePlayabilitySignals(content)
	if strings.Contains(content, m.failedMarker) {
		return &captureVerificationError{path: m.path, reason: m.failedReason(content)}
	}
	if strings.Contains(content, m.verifiedMarker) {
		m.captureVerified = true
	}
	if strings.Contains(content, demoParseFailureMarker) {
		return &demoParseError{path: m.path}
	}
	keep := max(
		len(demoParseFailureMarker),
		len(m.failedMarker),
		len(m.verifiedMarker),
	) - 1
	if len(content) > keep {
		m.tail = content[len(content)-keep:]
	} else {
		m.tail = content
	}
	return nil
}

// failedReason returns the reason from the "[zackvideo] capture_failed:" line
// in content, falling back to the generic HLAE rejection message when the
// script's reason line is missing from the log.
func (m *cs2ConsoleLogMonitor) failedReason(content string) string {
	if match := captureFailedPattern.FindStringSubmatch(content); match != nil {
		return strings.TrimSpace(match[1])
	}
	return "HLAE runtime rejected the observer POV"
}

func (m *cs2ConsoleLogMonitor) requireCaptureVerified() error {
	if m.captureVerified {
		return nil
	}
	if m.unplayableStart() {
		return &unplayableStartError{path: m.path}
	}
	return &captureVerificationError{
		path:   m.path,
		reason: "CS2 exited without the completed POV verification marker",
	}
}

func (m *cs2ConsoleLogMonitor) consumePlayabilitySignals(chunk string) {
	if strings.Contains(chunk, "ResetBreakpadAppId") {
		m.sawResetBreakpad = true
	}
	if strings.Contains(chunk, "[zackvideo] seek-landed") {
		m.sawSeekLanded = true
	}
	for _, line := range strings.Split(chunk, "\n") {
		match := armedMarker.FindStringSubmatch(strings.TrimSpace(line))
		if match != nil && match[1] == "0" {
			m.armedAtZero = true
		}
	}
}

func (m *cs2ConsoleLogMonitor) unplayableStart() bool {
	return m.armedAtZero && !m.sawSeekLanded && m.sawResetBreakpad
}

type unplayableStartError struct {
	path string
}

func (e *unplayableStartError) Error() string {
	return fmt.Sprintf("unplayable_start: CS2 crashed rewinding playdemo to tick 0; check CS2 console log %q", e.path)
}

type captureVerificationError struct {
	path   string
	reason string
}

func (e *captureVerificationError) Error() string {
	return fmt.Sprintf("capture POV verification failed: %s; check CS2 console log %q", e.reason, e.path)
}

var (
	armedMarker          = regexp.MustCompile(`^\[zackvideo\] armed at tick (\d+)$`)
	seekMarker           = regexp.MustCompile(`^\[zackvideo\] seek \d+ -> (\d+) attempt \d+ \(at (\d+)\)$`)
	seekLandedMarker     = regexp.MustCompile(`^\[zackvideo\] seek-landed -> (\d+) \(at (\d+)\)$`)
	recordMarker         = regexp.MustCompile(`^\[zackvideo\] record-(start|end)-(.+): mirv_streams record (start|end)$`)
	captureFailedPattern = regexp.MustCompile(`(?m)\[zackvideo\] capture_failed:\s*(.*?)\s*(?:\\n|$)`)
)

type performanceTrace struct {
	started time.Time
	run     *recording.RecordingRunPerformance
}

func newPerformanceTrace(started time.Time, run *recording.RecordingRunPerformance) *performanceTrace {
	return &performanceTrace{started: started, run: run}
}

func (m *cs2ConsoleLogMonitor) consumePerformanceMarkers(chunk string) {
	if m.trace == nil || m.trace.run == nil {
		return
	}
	content := m.lineTail + chunk
	lines := strings.Split(content, "\n")
	m.lineTail = lines[len(lines)-1]
	for _, line := range lines[:len(lines)-1] {
		m.trace.consume(strings.TrimSpace(line))
	}
}

func (t *performanceTrace) consume(line string) {
	event := recording.RecordingPerformanceEvent{ElapsedMS: elapsedMilliseconds(t.started)}
	switch match := armedMarker.FindStringSubmatch(line); {
	case match != nil:
		event.Kind = "demo_armed_observed"
		event.ObservedTick, _ = strconv.Atoi(match[1])
	case seekMarker.MatchString(line):
		match = seekMarker.FindStringSubmatch(line)
		event.Kind = "seek_requested_observed"
		event.TargetTick, _ = strconv.Atoi(match[1])
		event.ObservedTick, _ = strconv.Atoi(match[2])
	case seekLandedMarker.MatchString(line):
		match = seekLandedMarker.FindStringSubmatch(line)
		event.Kind = "seek_landed_observed"
		event.TargetTick, _ = strconv.Atoi(match[1])
		event.ObservedTick, _ = strconv.Atoi(match[2])
	case recordMarker.MatchString(line):
		match = recordMarker.FindStringSubmatch(line)
		if match[1] != match[3] {
			return
		}
		event.Kind = "record_" + match[1] + "_requested_observed"
		event.SegmentID = match[2]
	default:
		return
	}
	t.run.Events = append(t.run.Events, event)
}

type demoParseError struct {
	path string
}

func (e *demoParseError) Error() string {
	return fmt.Sprintf("demo incompatible with current cs2 build: playback disconnected with %s (demo likely recorded on an older game version); check console log %q", demoParseFailureMarker, e.path)
}

func validateCaptureResult(result recording.RecordingResult, cs2Exe string) error {
	if err := recording.ValidateUploadResult(result); err != nil {
		return fmt.Errorf("%w; check HLAE capture output and CS2 console log %q", err, cs2ConsoleLogPath(cs2Exe))
	}
	if err := recording.ValidateCaptureCoverage(result.Plan, result.Artifacts); err != nil {
		return fmt.Errorf("%w; check HLAE capture output and CS2 console log %q", err, cs2ConsoleLogPath(cs2Exe))
	}
	return nil
}

func ensureHLAEFFmpegConfig(hlaeExe string) error {
	dir := filepath.Join(filepath.Dir(hlaeExe), "ffmpeg")
	if _, err := os.Stat(dir); err != nil {
		return nil
	}
	if _, err := os.Stat(filepath.Join(dir, "bin", "ffmpeg.exe")); err == nil {
		return nil
	}
	ini := filepath.Join(dir, "ffmpeg.ini")
	if _, err := os.Stat(ini); err == nil {
		return nil
	}
	ffmpegPath, err := exec.LookPath("ffmpeg")
	if err != nil {
		return fmt.Errorf("HLAE FFmpeg config missing: install ffmpeg under %s or create %s", filepath.Join(dir, "bin"), ini)
	}
	content := fmt.Sprintf("[Ffmpeg]\r\nPath=%s\r\n", ffmpegPath)
	return os.WriteFile(ini, []byte(content), 0o600)
}

func locateHookDLL(hlaeExe string) (string, error) {
	dir := filepath.Dir(hlaeExe)
	candidates := []string{
		filepath.Join(dir, "AfxHookSource2.dll"),
		filepath.Join(dir, "x64", "AfxHookSource2.dll"),
	}
	for _, c := range candidates {
		if _, err := os.Stat(c); err == nil {
			return c, nil
		}
	}
	return "", fmt.Errorf("AfxHookSource2.dll not found next to HLAE.exe or under x64")
}

// captureCloseGracePeriod bounds how long the recorder waits for the in-engine
// soft-quit (scriptgen's mirv.exec("quit")) to close cs2.exe on its own once the
// capture-verified attestation marker has appeared. CS2 occasionally hangs on
// shutdown — a stalled render thread, a native modal, or client frames that stop
// advancing before the quit countdown lands — so relying on the in-engine quit
// alone makes the close non-deterministic. After the attestation marker proves
// this launch owns cs2.exe and the capture is complete, force-closing the
// process past this grace window is deterministic and safe, instead of blocking
// until the recorder's global timeout fires.
const captureCloseGracePeriod = 15 * time.Second

// windowTitlePollInterval bounds how often waitForWindowsProcessRunAndExit
// shells out to the expensive `tasklist /V` window-title lookup. The cheap
// `tasklist` (no /V) liveness check still runs on every pollInterval tick;
// only the verbose title lookup used to detect the "Error - Afx*" hook-crash
// dialog is throttled, since `/V` enumerates every windowed process on the
// system and otherwise runs ~2000 times per capture in contention with the
// single cs2.exe process and the encoder.
const windowTitlePollInterval = 5 * time.Second

func waitForWindowsProcessRunAndExit(ctx context.Context, image string, consoleLog *cs2ConsoleLogMonitor) error {
	titlePoller := newWindowTitlePoller(windowTitlePollInterval, nil, nil, nil)
	return waitForWindowsProcessRunAndExitWith(
		ctx,
		image,
		60*time.Second,
		500*time.Millisecond,
		captureCloseGracePeriod,
		func() bool { return consoleLog != nil && consoleLog.captureVerified },
		func(image string) (bool, string, error) {
			running, title, err := titlePoller.status(image)
			if err != nil {
				return running, title, err
			}
			if consoleLog != nil {
				if err := consoleLog.failure(); err != nil {
					return running, title, err
				}
			}
			return running, title, nil
		},
		terminateWindowsProcess,
	)
}

// windowTitlePoller throttles tasklistWindowTitle's `/V` window-title lookup
// to titleInterval, falling back to the cheap processRunning liveness check
// (no `/V`, no title) on the ticks in between. status() reports an empty
// title on the cheap ticks, so a hook-crash dialog is detected on the next
// slow tick rather than immediately; that trades up to titleInterval of
// detection latency for far fewer `tasklist /V` invocations per capture.
type windowTitlePoller struct {
	titleInterval time.Duration
	now           func() time.Time
	titleStatus   func(string) (bool, string, error)
	cheapStatus   func(string) (bool, error)
	lastChecked   time.Time
	haveChecked   bool
}

// newWindowTitlePoller builds a windowTitlePoller. titleStatus and
// cheapStatus default to tasklistWindowTitle and processRunning respectively
// when nil, and now defaults to time.Now; tests inject fakes for all three so
// the throttling cadence is verifiable without shelling out or sleeping.
func newWindowTitlePoller(
	titleInterval time.Duration,
	now func() time.Time,
	titleStatus func(string) (bool, string, error),
	cheapStatus func(string) (bool, error),
) *windowTitlePoller {
	if now == nil {
		now = time.Now
	}
	if titleStatus == nil {
		titleStatus = tasklistWindowTitle
	}
	if cheapStatus == nil {
		cheapStatus = processRunning
	}
	return &windowTitlePoller{
		titleInterval: titleInterval,
		now:           now,
		titleStatus:   titleStatus,
		cheapStatus:   cheapStatus,
	}
}

func (p *windowTitlePoller) status(image string) (running bool, title string, err error) {
	if !p.haveChecked || p.now().Sub(p.lastChecked) >= p.titleInterval {
		running, title, err = p.titleStatus(image)
		if err != nil {
			return running, title, err
		}
		p.haveChecked = true
		p.lastChecked = p.now()
		return running, title, nil
	}
	running, err = p.cheapStatus(image)
	return running, "", err
}

func waitForWindowsProcessRunAndExitWith(
	ctx context.Context,
	image string,
	firstWait time.Duration,
	pollInterval time.Duration,
	closeGrace time.Duration,
	verified func() bool,
	status func(string) (bool, string, error),
	terminate func(string) error,
) error {
	seen := false
	firstDeadline := time.NewTimer(firstWait)
	defer firstDeadline.Stop()
	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()

	// The grace timer arms only once the capture-verified marker proves this run
	// owns cs2.exe and recording is complete. Until then graceC stays nil so the
	// select never takes that branch, and an unverified process is never
	// force-closed. When the timer fires, cs2.exe has outlived the in-engine
	// soft-quit and is force-closed deterministically.
	graceTimer := time.NewTimer(closeGrace)
	if !graceTimer.Stop() {
		select {
		case <-graceTimer.C:
		default:
		}
	}
	defer graceTimer.Stop()
	var graceC <-chan time.Time
	graceArmed := false
	armGraceIfVerified := func() {
		if graceArmed || verified == nil || !verified() {
			return
		}
		graceArmed = true
		graceTimer.Reset(closeGrace)
		graceC = graceTimer.C
	}

	for {
		select {
		case <-ctx.Done():
			_, _, statusErr := status(image)
			if statusErr != nil {
				cause := errors.Join(ctx.Err(), fmt.Errorf("inspect %s during cancellation: %w", image, statusErr))
				return stopProcessAfterWaitFailure(
					image,
					cause,
					shouldTerminateAfterStatusFailure(statusErr, seen),
					terminate,
				)
			}
			return stopProcessAfterWaitFailure(image, ctx.Err(), seen, terminate)
		case <-firstDeadline.C:
			if !seen {
				running, title, err := status(image)
				if running {
					seen = true
				}
				if err != nil {
					return stopProcessAfterWaitFailure(image, err, shouldTerminateAfterStatusFailure(err, seen), terminate)
				}
				if isHookErrorWindowTitle(title) {
					return stopProcessAfterWaitFailure(image, &hookIncompatibleError{windowTitle: title}, true, terminate)
				}
				if !running {
					return fmt.Errorf("%s did not appear within %s", image, firstWait)
				}
				armGraceIfVerified()
			}
		case <-graceC:
			// Capture is verified but cs2.exe is still up past the grace window:
			// the in-engine quit did not land. Force-close the proven-owned
			// process so the close is deterministic rather than waiting on the
			// recorder's global timeout.
			running, _, err := status(image)
			if err != nil {
				return stopProcessAfterWaitFailure(image, err, shouldTerminateAfterStatusFailure(err, seen), terminate)
			}
			if !running {
				return nil
			}
			if err := terminate(image); err != nil {
				return fmt.Errorf("force-close %s after verified capture: %w", image, err)
			}
			return nil
		case <-ticker.C:
			running, title, err := status(image)
			if running {
				seen = true
			}
			if err != nil {
				return stopProcessAfterWaitFailure(image, err, shouldTerminateAfterStatusFailure(err, seen), terminate)
			}
			if isHookErrorWindowTitle(title) {
				return stopProcessAfterWaitFailure(image, &hookIncompatibleError{windowTitle: title}, true, terminate)
			}
			if running {
				armGraceIfVerified()
				continue
			}
			if seen {
				return nil
			}
		}
	}
}

func launcherFailure(waitErr error) error {
	return fmt.Errorf("HLAE launcher failed: %w", waitErr)
}

func shouldTerminateAfterStatusFailure(err error, processSeen bool) bool {
	var parseErr *demoParseError
	// The console marker can only come from the CS2 instance launched for this
	// capture. It is therefore ownership evidence even when tasklist polling
	// has not successfully observed the process yet.
	return processSeen || errors.As(err, &parseErr)
}

func stopProcessAfterWaitFailure(image string, cause error, processMayBeRunning bool, terminate func(string) error) error {
	if !processMayBeRunning {
		return cause
	}
	if err := terminate(image); err != nil {
		return fmt.Errorf("%w; stop %s after capture failure: %v", cause, image, err)
	}
	return cause
}

func terminateWindowsProcess(image string) error {
	if runtime.GOOS != "windows" {
		return nil
	}
	// #nosec G204 -- taskkill is fixed and image is the recorder-owned CS2 executable name.
	out, err := exec.Command("taskkill", "/IM", image, "/T", "/F").CombinedOutput()
	if err == nil {
		return nil
	}
	running, stateErr := processRunning(image)
	if stateErr == nil && !running {
		return nil
	}
	detail := strings.TrimSpace(string(out))
	if detail == "" {
		return fmt.Errorf("taskkill %s: %w", image, err)
	}
	return fmt.Errorf("taskkill %s: %w: %s", image, err, detail)
}

func processRunning(image string) (bool, error) {
	if runtime.GOOS != "windows" {
		return false, nil
	}
	// #nosec G204 -- tasklist executable is fixed and image is derived from a local executable path.
	out, err := exec.Command("tasklist", "/FI", "IMAGENAME eq "+image, "/FO", "CSV", "/NH").Output()
	if err != nil {
		return false, err
	}
	text := strings.TrimSpace(string(out))
	return strings.Contains(strings.ToLower(text), strings.ToLower(image)), nil
}

// exitHookIncompatible is the process exit code used when HLAE's injected
// hook crashes with a native "Error - Afx*" dialog instead of capturing.
// Keep this in sync with the zv-recorder case in cmd/zv/obs_record.go's
// shortStageClass, which maps this code to the "capture_incompatible"
// observability class.
const exitHookIncompatible = 6

// exitDemoIncompatible is the process exit code used when CS2 cannot replay the
// demo — playback disconnects with NETWORK_DISCONNECT_MESSAGE_PARSE_ERROR,
// which almost always means the demo was recorded on an older game build.
// Keep this in sync with the zv-recorder case in cmd/zv/obs_record.go's
// shortStageClass, which maps this code to the "demo_incompatible"
// observability class.
const exitDemoIncompatible = 7

// exitUnplayableStart is used when CS2 crashes on the playdemo rewind to
// demo tick 0 (armed@0, no seek-landed, ResetBreakpadAppId). Keep in sync
// with cmd/zv/obs_record.go shortStageClass.
const exitUnplayableStart = 8

// hookErrorWindowTitlePattern matches the native MessageBox titles
// advancedfx's hook modules (AfxHookSource2, AfxHookSource, ...) use when a
// memory signature scan fails to resolve an address in the current game
// binary — almost always caused by a CS2 update landing after the installed
// HLAE build was released.
var hookErrorWindowTitlePattern = regexp.MustCompile(`^Error - Afx`)

// isHookErrorWindowTitle reports whether title is a native HLAE hook crash
// dialog, e.g. "Error - AfxHookSource2".
func isHookErrorWindowTitle(title string) bool {
	return hookErrorWindowTitlePattern.MatchString(title)
}

// hookIncompatibleError reports that HLAE's injected hook crashed with a
// native error dialog instead of capturing. In practice this means the
// installed HLAE/AfxHookSource2 build does not match the currently installed
// CS2 version.
type hookIncompatibleError struct {
	windowTitle string
}

func (e *hookIncompatibleError) Error() string {
	return fmt.Sprintf(
		"HLAE hook crashed with a native error dialog (%q) instead of capturing: "+
			"the installed HLAE/AfxHookSource2 build is likely incompatible with the current CS2 version "+
			"(CS2 updates regularly break AfxHookSource2's signature scan until advancedfx ships a new build); "+
			"check https://github.com/advancedfx/advancedfx/releases for a newer HLAE build",
		e.windowTitle,
	)
}

// tasklistWindowTitle reports whether image is currently running and its
// current main window title (which is the dialog title when a modal error
// box has replaced the game window), by shelling out to `tasklist /V`. It
// mirrors processRunning's contract but also extracts the "Window Title"
// verbose column.
func tasklistWindowTitle(image string) (running bool, title string, err error) {
	// #nosec G204 -- tasklist executable is fixed and image is derived from a local executable path.
	out, err := exec.Command("tasklist", "/V", "/FI", "IMAGENAME eq "+image, "/FO", "CSV", "/NH").Output()
	if err != nil {
		return false, "", err
	}
	running, title = parseTasklistVerboseCSV(string(out), image)
	return running, title, nil
}

// parseTasklistVerboseCSV extracts the running state and window title for
// image from `tasklist /V /FO CSV /NH` output. Isolated from the exec call so
// it is testable against captured sample output. tasklist prints an
// "INFO: No tasks..." line (not valid multi-field CSV) when nothing matches;
// that line simply fails the image-name comparison and is skipped.
func parseTasklistVerboseCSV(out, image string) (running bool, title string) {
	r := csv.NewReader(strings.NewReader(out))
	for {
		record, err := r.Read()
		if err != nil {
			return running, title
		}
		if len(record) == 0 || !strings.EqualFold(record[0], image) {
			continue
		}
		running = true
		if len(record) >= 9 {
			title = record[8]
		}
		return running, title
	}
}

func writeResult(outDir string, result recording.RecordingResult) error {
	b, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(outDir, "recording-result.json"), append(b, '\n'), 0o600)
}

type recordingSummary struct {
	OK            bool     `json:"ok"`
	DryRun        bool     `json:"dry_run"`
	Executed      bool     `json:"executed"`
	ResultPath    string   `json:"result_path"`
	ScriptPath    string   `json:"script_path"`
	SegmentCount  int      `json:"segment_count"`
	ArtifactCount int      `json:"artifact_count"`
	Warnings      []string `json:"warnings"`

	// Capture-run performance metrics (only when a run recorded them).
	PrepareMS           int64     `json:"prepare_ms,omitempty"`
	LaunchAndCaptureMS  int64     `json:"launch_and_capture_ms,omitempty"`
	IncrementalMuxMS    int64     `json:"incremental_mux_ms,omitempty"`
	ArtifactProbeMS     int64     `json:"artifact_probe_ms,omitempty"`
	FinalMuxMS          int64     `json:"final_mux_ms,omitempty"`
	ValidationMS        int64     `json:"validation_ms,omitempty"`
	BeforeResultWriteMS int64     `json:"before_result_write_ms,omitempty"`
	ObservedFPS         []float64 `json:"observed_fps,omitempty"`
	SeekCount           int       `json:"seek_count,omitempty"`
	SeekElapsedMS       int64     `json:"seek_elapsed_ms,omitempty"`
}

func writeResultAndReport(outDir string, result recording.RecordingResult, dryRun bool, format string, w io.Writer) error {
	if err := writeResult(outDir, result); err != nil {
		return err
	}
	summary := recordingSummary{
		OK:            true,
		DryRun:        dryRun,
		Executed:      !dryRun,
		ResultPath:    filepath.Join(outDir, "recording-result.json"),
		ScriptPath:    result.Script,
		SegmentCount:  len(result.Plan.Segments),
		ArtifactCount: len(result.Artifacts),
		Warnings:      append([]string{}, result.Warnings...),
	}
	if result.Performance != nil && len(result.Performance.Runs) > 0 {
		run := result.Performance.Runs[0]
		summary.PrepareMS = run.PrepareMS
		summary.LaunchAndCaptureMS = run.LaunchAndCaptureMS
		summary.IncrementalMuxMS = run.IncrementalMuxMS
		summary.ArtifactProbeMS = run.ArtifactProbeMS
		summary.FinalMuxMS = run.FinalMuxMS
		summary.ValidationMS = run.ValidationMS
		summary.BeforeResultWriteMS = run.BeforeResultWriteMS
		for _, segment := range run.Segments {
			summary.ObservedFPS = append(summary.ObservedFPS, segment.ObservedFramesPerSecond)
		}
		summary.SeekCount, summary.SeekElapsedMS = aggregateSeekMetrics(run.Events)
	}
	if format == "json" {
		encoder := json.NewEncoder(w)
		encoder.SetEscapeHTML(false)
		encoder.SetIndent("", "  ")
		return encoder.Encode(summary)
	}
	fmt.Fprintf(w, "recording_result\t%s\n", summary.ResultPath)
	fmt.Fprintf(w, "recording_script\t%s\n", summary.ScriptPath)
	fmt.Fprintf(w, "segments\t%d\n", summary.SegmentCount)
	fmt.Fprintf(w, "artifacts\t%d\n", summary.ArtifactCount)
	fmt.Fprintf(w, "dry_run\t%t\n", summary.DryRun)
	if result.Performance != nil && len(result.Performance.Runs) > 0 {
		fmt.Fprintf(w, "prepare_ms\t%d\n", summary.PrepareMS)
		fmt.Fprintf(w, "launch_and_capture_ms\t%d\n", summary.LaunchAndCaptureMS)
		fmt.Fprintf(w, "incremental_mux_ms\t%d\n", summary.IncrementalMuxMS)
		fmt.Fprintf(w, "artifact_probe_ms\t%d\n", summary.ArtifactProbeMS)
		fmt.Fprintf(w, "final_mux_ms\t%d\n", summary.FinalMuxMS)
		fmt.Fprintf(w, "validation_ms\t%d\n", summary.ValidationMS)
		fmt.Fprintf(w, "before_result_write_ms\t%d\n", summary.BeforeResultWriteMS)
		fmt.Fprintf(w, "seek_count\t%d\n", summary.SeekCount)
		fmt.Fprintf(w, "seek_elapsed_ms\t%d\n", summary.SeekElapsedMS)
	}
	return nil
}

// aggregateSeekMetrics reduces the observed seek events into a total issued
// count and the summed wall-clock time from the last request for a target to
// its landing. Re-issued attempts share a target, so each request overwrites
// the pending timestamp and the final landed event consumes the last attempt.
func aggregateSeekMetrics(events []recording.RecordingPerformanceEvent) (count int, elapsedMS int64) {
	pending := map[int]int64{}
	for _, e := range events {
		switch e.Kind {
		case "seek_requested_observed":
			count++
			pending[e.TargetTick] = e.ElapsedMS
		case "seek_landed_observed":
			if requested, ok := pending[e.TargetTick]; ok {
				elapsedMS += e.ElapsedMS - requested
			}
			delete(pending, e.TargetTick)
		}
	}
	return count, elapsedMS
}
