package editor

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/rechedev9/cliphub/internal/mediaassets"
	"github.com/rechedev9/cliphub/internal/recapplan"
	"github.com/rechedev9/cliphub/internal/recording"
)

type FullDemoDeliveryEvidence struct {
	FullDecode      bool    `json:"full_decode"`
	FrameCount      int64   `json:"frame_count"`
	SampleRate      int     `json:"sample_rate"`
	Channels        int     `json:"channels"`
	DurationSeconds float64 `json:"duration_seconds"`
	ContentSHA256   string  `json:"content_sha256"`
}

// fullDemoDeliveryDiagnostics requests the optional black/freeze quality
// filters that report diagnostics without altering the picture. A nil value
// keeps the strict decode-only behavior legacy callers rely on.
type fullDemoDeliveryDiagnostics struct {
	SegmentID string
	Filters   []string
}

// fullDemoDeliveryOutcome carries the strict delivery evidence plus everything
// the optional diagnostics produced during the same mandatory decode.
type fullDemoDeliveryOutcome struct {
	Evidence        *FullDemoDeliveryEvidence
	QualityLog      string
	QualityWarnings []string
	DecodeMS        int64
}

func verifyFullDemoDelivery(ctx context.Context, ffmpeg, ffprobe, path string, frames int64, progress fullDemoProgress) (*FullDemoDeliveryEvidence, error) {
	outcome, err := verifyFullDemoDeliveryWithDiagnostics(ctx, ffmpeg, ffprobe, path, frames, progress, nil)
	if err != nil {
		return nil, err
	}
	return outcome.Evidence, nil
}

// verifyFullDemoDeliveryWithDiagnostics runs the mandatory complete decode and,
// when diagnostics are requested, folds the existing blackdetect/freezedetect
// filters into that same process instead of paying for a second full-video
// decode. Delivery stays strict: -xerror, passed-through frame timing, the
// canonical frame count, zero duplicates/drops, probe/durations and the digest
// are unchanged. When the optional diagnostic setup fails on its own, the
// strict decode is retried without the filters so an optional check never
// weakens or fails delivery; the setup problem is reported as a warning.
func verifyFullDemoDeliveryWithDiagnostics(ctx context.Context, ffmpeg, ffprobe, path string, frames int64, progress fullDemoProgress, diagnostics *fullDemoDeliveryDiagnostics) (*fullDemoDeliveryOutcome, error) {
	if ffprobe == "" {
		return nil, fmt.Errorf("full_demo_output_invalid: ffprobe is required")
	}
	type digestResult struct {
		hash string
		err  error
	}
	digestCh := make(chan digestResult, 1)
	go func() {
		hash, err := mediaassets.FileDigest(ctx, path, 64<<30)
		digestCh <- digestResult{hash, err}
	}()
	progress.report("Comprobando formato del vídeo", 0)
	output, err := runFFmpegOutput(ctx, []string{ffprobe, "-v", "error", "-show_entries", "stream=codec_type,codec_name,width,height,r_frame_rate,sample_rate,channels,duration", "-of", "json", path}, "Full Demo delivery probe")
	if err != nil {
		return nil, err
	}
	var probe struct {
		Streams []struct {
			Type       string `json:"codec_type"`
			Codec      string `json:"codec_name"`
			Width      int    `json:"width"`
			Height     int    `json:"height"`
			FrameRate  string `json:"r_frame_rate"`
			SampleRate string `json:"sample_rate"`
			Channels   int    `json:"channels"`
			Duration   string `json:"duration"`
		} `json:"streams"`
	}
	if len(output) > 1<<20 {
		return nil, fmt.Errorf("full demo delivery probe exceeds resource limit")
	}
	if err := json.Unmarshal([]byte(output), &probe); err != nil {
		return nil, err
	}
	e := &FullDemoDeliveryEvidence{DurationSeconds: float64(frames) / recapplan.OutputFPS}
	video, audio := false, false
	for _, stream := range probe.Streams {
		duration, parseErr := strconv.ParseFloat(stream.Duration, 64)
		if parseErr != nil || math.IsNaN(duration) || math.IsInf(duration, 0) || math.Abs(duration-e.DurationSeconds) > 1.0/recapplan.OutputFPS {
			return nil, fmt.Errorf("full_demo_output_invalid: stream duration differs from the frame/sample timeline")
		}
		switch stream.Type {
		case "video":
			if video || stream.Codec != "h264" || stream.Width != 1920 || stream.Height != 1080 || !frameRateMatches(stream.FrameRate, 60) {
				return nil, fmt.Errorf("full_demo_output_invalid: delivered video differs from 1080p60 H.264 (got codec=%s, size=%dx%d, fps=%s, duplicate=%t)", stream.Codec, stream.Width, stream.Height, stream.FrameRate, video)
			}
			video = true
		case "audio":
			if audio || stream.Codec != "aac" || stream.SampleRate != "48000" || stream.Channels != 2 {
				return nil, fmt.Errorf("full_demo_output_invalid: delivered audio is not stereo AAC at 48 kHz")
			}
			audio, e.SampleRate, e.Channels = true, 48000, 2
		default:
			return nil, fmt.Errorf("full_demo_output_invalid: unexpected delivery stream")
		}
	}
	if !video || !audio {
		return nil, fmt.Errorf("full_demo_output_invalid: missing video or audio")
	}
	// Count frames during the mandatory complete decode, rather than decoding
	// once in ffprobe -count_frames and a second time here. Passthrough forbids
	// output frame duplication/dropping from hiding a noncanonical source count.
	// The optional black/freeze quality filters run on the same decode: they do
	// not change frames or samples, so the canonical evidence is unchanged.
	filters := []string(nil)
	if diagnostics != nil {
		filters = diagnostics.Filters
	}
	strictCommand := []string{ffmpeg, "-v", "error", "-xerror", "-i", path, "-map", "0:v:0", "-map", "0:a:0", "-fps_mode", "passthrough", "-f", "null", "-"}
	combinedCommand := strictCommand
	if len(filters) > 0 {
		// blackdetect/freezedetect emit their event lines at info level.
		combinedCommand = []string{ffmpeg, "-v", "info", "-xerror", "-i", path, "-map", "0:v:0", "-map", "0:a:0", "-vf", strings.Join(filters, ","), "-fps_mode", "passthrough", "-f", "null", "-"}
	}
	var decoded bytes.Buffer
	decodeStarted := time.Now()
	setupLog, err := runFFmpegOutputProgressTo(ctx, combinedCommand, "Full Demo complete delivery decode", e.DurationSeconds, progress.pass("Verificando fotogramas, vídeo y audio", .1, .9), &decoded)
	qualityLog := setupLog
	diagnosticSetupWarning := ""
	if err != nil && len(filters) > 0 && fullDemoDeliveryDiagnosticSetupError(err) {
		diagnosticSetupWarning = fmt.Sprintf("quality check %s: %v", diagnostics.SegmentID, err)
		// The optional diagnostics must never fail or weaken the mandatory
		// strict decode. Retry without them, and keep BOTH the original setup
		// stderr (the real cause) and the retry log instead of overwriting it.
		decoded.Reset()
		retryLog, retryErr := runFFmpegOutputProgressTo(ctx, strictCommand, "Full Demo complete delivery decode", e.DurationSeconds, progress.pass("Verificando fotogramas, vídeo y audio", .1, .9), &decoded)
		qualityLog = joinFullDemoDeliveryLogs(setupLog, retryLog)
		err = retryErr
	}
	decodeMS := time.Since(decodeStarted).Milliseconds()
	if err != nil {
		return nil, err
	}
	count, err := decodedDeliveryFrames(decoded.String())
	if err != nil || count != frames {
		return nil, fmt.Errorf("full_demo_output_invalid: complete decode did not certify %d frames (got %d): %v", frames, count, err)
	}
	e.FrameCount = count
	if info, err := os.Stat(path); err != nil || info.Size() == 0 {
		return nil, fmt.Errorf("full_demo_output_invalid: missing delivered file")
	}
	progress.report("Verificando archivo final", .9)
	digest := <-digestCh
	if digest.err != nil {
		return nil, digest.err
	}
	e.ContentSHA256 = digest.hash
	e.FullDecode = true
	outcome := &fullDemoDeliveryOutcome{Evidence: e, DecodeMS: decodeMS}
	if len(filters) > 0 {
		outcome.QualityLog = qualityLog
		outcome.QualityWarnings = QualityWarningsFromFFmpegLog(diagnostics.SegmentID, qualityLog)
		if diagnosticSetupWarning != "" {
			outcome.QualityWarnings = append(outcome.QualityWarnings, diagnosticSetupWarning)
		}
	}
	return outcome, nil
}

// joinFullDemoDeliveryLogs keeps the diagnostic setup failure and the strict
// retry output together. The decoder that parses warnings only looks for event
// substrings, so concatenation is safe and preserves the original cause.
func joinFullDemoDeliveryLogs(setup, retry string) string {
	switch {
	case setup == "":
		return retry
	case retry == "":
		return setup
	default:
		return setup + "\n" + retry
	}
}

// fullDemoDeliveryDiagnosticSetupError distinguishes an optional filter graph
// that failed to configure from a genuine media/decode error, so only the
// former may fall back to the strict decode. The filters are fixed and known
// good; the fallback is a safety net for a broken or replaced FFmpeg build.
func fullDemoDeliveryDiagnosticSetupError(err error) bool {
	if err == nil {
		return false
	}
	message := err.Error()
	for _, marker := range []string{
		"No such filter",
		"Error initializing filter",
		"Error reinitializing filters",
		"Invalid filter",
		"Failed to configure output pad",
		"Error initializing complex filters",
		"Error parsing filterchain",
		"Error parsing a filter description",
	} {
		if strings.Contains(message, marker) {
			return true
		}
	}
	return false
}

// Only a completed progress record with unmodified output frames is evidence.
// Container nb_frames is deliberately never trusted as a decoded frame count.
func decodedDeliveryFrames(output string) (int64, error) {
	scanner := bufio.NewScanner(strings.NewReader(output))
	count, duplicates, dropped := int64(-1), int64(-1), int64(-1)
	for scanner.Scan() {
		key, value, ok := strings.Cut(strings.TrimSpace(scanner.Text()), "=")
		if !ok {
			continue
		}
		switch key {
		case "frame", "dup_frames", "drop_frames":
			n, err := strconv.ParseInt(strings.TrimSpace(value), 10, 64)
			if err != nil || n < 0 {
				return 0, fmt.Errorf("invalid decode progress %s", key)
			}
			switch key {
			case "frame":
				count = n
			case "dup_frames":
				duplicates = n
			case "drop_frames":
				dropped = n
			}
		case "progress":
			if value == "end" && count > 0 && duplicates == 0 && dropped == 0 {
				return count, nil
			}
			count, duplicates, dropped = -1, -1, -1
		}
	}
	return 0, fmt.Errorf("incomplete decode progress")
}

// validateFullDemoCaptureTailPads accepts only bounded, self-consistent pads of
// distinct effective rounds. The delivery frame count check below then proves
// the padded items still produced the canonical program length.
func validateFullDemoCaptureTailPads(pads []recording.FullDemoTailPad, effective recapplan.Document) error {
	rounds := map[string]bool{}
	for _, round := range effective.Rounds {
		rounds[round.ID] = true
	}
	seen := map[string]bool{}
	for _, pad := range pads {
		if err := pad.Validate(); err != nil {
			return err
		}
		if !rounds[pad.SegmentID] || seen[pad.SegmentID] {
			return fmt.Errorf("full_demo_capture_incomplete: %s: tail pad does not match one effective round", pad.SegmentID)
		}
		seen[pad.SegmentID] = true
	}
	return nil
}

func (e *FullDemoRenderEvidence) ValidateCompleted() error {
	if e == nil || e.SchemaVersion != "1.0" {
		return fmt.Errorf("missing Full Demo render evidence")
	}
	if err := e.Approved.Validate(); err != nil {
		return err
	}
	if err := e.Effective.Validate(); err != nil {
		return err
	}
	ends := map[string]int{}
	for _, round := range e.Effective.Rounds {
		ends[round.ID] = round.EffectiveEndTick
	}
	expected, err := recapplan.ApplyCertifiedEnds(e.Approved, ends)
	if err != nil {
		return err
	}
	if expected.PlanHash != e.Effective.PlanHash {
		return fmt.Errorf("full demo effective plan differs from approved changes")
	}
	if err := e.validateTransitions(); err != nil {
		return err
	}
	if err := validateFullDemoHUD(e.HUD, e.Effective); err != nil {
		return err
	}
	if err := validateFullDemoCaptureTailPads(e.CaptureTailPads, e.Effective); err != nil {
		return err
	}
	frames := e.Effective.Timeline[len(e.Effective.Timeline)-1].EndFrame
	if e.Delivery == nil || !e.Delivery.FullDecode || e.Delivery.FrameCount != frames || e.Delivery.SampleRate != 48000 || e.Delivery.Channels != 2 || !recapplan.ValidHash(e.Delivery.ContentSHA256) || math.IsNaN(e.Delivery.DurationSeconds) || math.IsInf(e.Delivery.DurationSeconds, 0) || math.Abs(e.Delivery.DurationSeconds-float64(frames)/recapplan.OutputFPS) > 1.0/recapplan.OutputFPS {
		return fmt.Errorf("full_demo_output_invalid: missing complete delivery evidence")
	}
	if e.ProgramLoudness == nil || len(e.ProgramLoudness.DecodedAAC) == 0 {
		return fmt.Errorf("audio_loudness_failed: missing decoded AAC evidence")
	}
	last := e.ProgramLoudness.DecodedAAC[len(e.ProgramLoudness.DecodedAAC)-1]
	a := e.Effective.Options.Audio
	if last.Status == "silent" && e.ProgramLoudness.Status == "silent-approved" && a.Game.Gain == 0 && (!a.Voice.Enabled || a.Voice.Gain == 0) && !a.Music.Enabled && !e.Effective.Options.HasBumpers() && !e.Effective.HasTransitionSFX() {
		return nil
	}
	if e.ProgramLoudness.Status != "verified-decoded-aac" || last.Status != "measured" || last.IntegratedLUFS == nil || last.TruePeakDBTP == nil || math.IsNaN(*last.IntegratedLUFS) || math.IsNaN(*last.TruePeakDBTP) || math.IsInf(*last.IntegratedLUFS, 0) || math.IsInf(*last.TruePeakDBTP, 0) || math.Abs(*last.IntegratedLUFS-a.Loudness.TargetILUFS) > .5 || *last.TruePeakDBTP > a.Loudness.TargetTPDBTP {
		return fmt.Errorf("audio_loudness_failed: final decoded AAC misses its approved targets")
	}
	return nil
}
