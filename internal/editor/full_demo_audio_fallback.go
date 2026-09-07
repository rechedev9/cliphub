package editor

import (
	"context"
	"fmt"
	"math"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"

	"github.com/rechedev9/cliphub/internal/recapplan"
)

// ProgramAACFallbackMaster records the extra processing applied only when all
// ordinary masters failed. Acceptance still belongs to the decoded AAC.
type ProgramAACFallbackMaster struct {
	Encoder     string  `json:"encoder"`
	GainDB      float64 `json:"gain_db"`
	CeilingDBFS float64 `json:"ceiling_dbfs"`
}

func hasMediaFoundationAAC(ctx context.Context, ffmpeg string) bool {
	if runtime.GOOS != "windows" {
		return false
	}
	output, err := runFFmpegOutput(ctx, []string{ffmpeg, "-hide_banner", "-encoders"}, "AAC recovery encoder probe")
	if err != nil {
		return false
	}
	for _, line := range strings.Split(output, "\n") {
		fields := strings.Fields(line)
		if len(fields) >= 2 && fields[1] == "aac_mf" {
			return true
		}
	}
	return false
}

func (m ProgramAACFallbackMaster) filter(base string) string {
	// Explicit oversampling and disabled automatic level compensation preserve
	// the peak ceiling; latency compensation preserves the approved timeline.
	return base + ",aresample=48000,aformat=channel_layouts=stereo,volume=" + decimal(m.GainDB) + "dB,aresample=192000,alimiter=limit=" + decimal(math.Pow(10, m.CeilingDBFS/20)) + ":attack=1:release=5:level=false:latency=true,aresample=48000"
}

func (m ProgramAACFallbackMaster) corrected(decoded LoudnessMeasurement, target recapplan.LoudnessOptions) ProgramAACFallbackMaster {
	m.GainDB = max(-3, min(6, m.GainDB+max(-1.5, min(1.5, target.TargetILUFS-*decoded.IntegratedLUFS))))
	if *decoded.TruePeakDBTP > target.TargetTPDBTP {
		m.CeilingDBFS = max(target.TargetTPDBTP-8, m.CeilingDBFS-min(2, *decoded.TruePeakDBTP-target.TargetTPDBTP+.2))
	}
	return m
}

func recoverFullDemoAAC(ctx context.Context, ffmpeg, input, output, logDir string, target recapplan.LoudnessOptions, duration float64, e ProgramLoudnessEvidence, progress fullDemoProgress) (ProgramLoudnessEvidence, error) {
	if !hasMediaFoundationAAC(ctx, ffmpeg) {
		if err := ctx.Err(); err != nil {
			return e, err
		}
		return e, fullDemoAACFailure("Media Foundation AAC recovery is unavailable after three masters", e)
	}
	// Keep the initial normalization fixed. Feeding lossy peak overshoot back
	// into loudnorm's target TP can lower the whole mix and defeat its LUFS
	// target. Here gain and the post-normalization limiter are independent.
	baseTarget := target
	baseTarget.TargetTPDBTP -= .3
	base, err := measuredLoudnessFilter(baseTarget, e.Input)
	if err != nil {
		return e, err
	}
	master := ProgramAACFallbackMaster{Encoder: "aac_mf", GainDB: 3, CeilingDBFS: target.TargetTPDBTP - 3.5}
	// Rebuild timestamps from the canonical sample clock as well as bounding
	// sample count: filter timestamps can otherwise extend the stream duration.
	// AAC packet padding is checked separately by delivery validation.
	samples := strconv.FormatInt(int64(math.Round(duration*recapplan.SampleRate)), 10)
	for attempt := 0; attempt < 3; attempt++ {
		start := float64(attempt) / 3
		stage := fmt.Sprintf("Recuperando audio final (%d/3)", attempt+1)
		e.MasterTargets = append(e.MasterTargets, baseTarget)
		e.FallbackMasters = append(e.FallbackMasters, master)
		// Media Foundation reports a full 1024-sample duration for its padded
		// final packet. Mark only its real samples as playable, matching native
		// AAC's short final packet duration instead of extending the MP4 track.
		packetDuration := "setts=duration=min(DURATION\\,max(0\\," + samples + "/48000/TB-PTS))"
		command := []string{ffmpeg, "-y", "-hide_banner", "-nostats", "-v", "info", "-i", input, "-map", "0:v:0", "-map", "0:a:0", "-c:v", "copy", "-af", master.filter(base) + ",apad=whole_len=" + samples + ",atrim=end_sample=" + samples + ",asetpts=N/SR/TB", "-c:a", master.Encoder, "-b:a", "192k", "-ar", "48000", "-ac", "2", "-bsf:a", packetDuration, "-movflags", "+faststart", output}
		if err := runFFmpegAtomicWithProgress(ctx, command, "Full Demo AAC recovery", filepath.Join(logDir, fmt.Sprintf("program-aac-recovery-%d.txt", attempt)), output, duration, progress.pass(stage, start, start+1.0/6)); err != nil {
			return e, fmt.Errorf("audio_loudness_failed: AAC recovery: %w", err)
		}
		decoded, err := measureLoudness(ctx, ffmpeg, output, target, filepath.Join(logDir, fmt.Sprintf("decoded-aac-recovery-%d.txt", attempt)), duration, progress.pass(fmt.Sprintf("Comprobando audio recuperado (%d/3)", attempt+1), start+1.0/6, start+1.0/3))
		if err != nil {
			return e, err
		}
		e.DecodedAAC = append(e.DecodedAAC, decoded)
		if decoded.Status != "measured" {
			return e, fmt.Errorf("audio_loudness_failed: recovered AAC is not measurable")
		}
		if math.Abs(*decoded.IntegratedLUFS-target.TargetILUFS) <= .5 && *decoded.TruePeakDBTP <= target.TargetTPDBTP {
			e.Status = "verified-decoded-aac"
			return e, nil
		}
		master = master.corrected(decoded, target)
	}
	return e, fullDemoAACFailure("approved targets remain unmet after three masters and three recovery attempts", e)
}

func fullDemoAACFailure(reason string, e ProgramLoudnessEvidence) error {
	var attempts []string
	for i, decoded := range e.DecodedAAC {
		if decoded.IntegratedLUFS != nil && decoded.TruePeakDBTP != nil {
			attempts = append(attempts, fmt.Sprintf("#%d %.2f LUFS / %.2f dBTP", i+1, *decoded.IntegratedLUFS, *decoded.TruePeakDBTP))
		}
	}
	return fmt.Errorf("audio_loudness_failed: %s; decoded AAC: %s", reason, strings.Join(attempts, "; "))
}
