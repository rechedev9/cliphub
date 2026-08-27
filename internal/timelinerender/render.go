package timelinerender

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/rechedev9/cliphub/internal/editor"
	"github.com/rechedev9/cliphub/internal/timelineplan"
)

type Result struct {
	OutputPath string
	CoverPath  string
	Duration   float64
	Width      int
	Height     int
	Warnings   []string
	Log        string
}

func Render(ctx context.Context, ffmpegPath string, in Inputs, doc timelineplan.Document) (Result, error) {
	if ffmpegPath == "" {
		return Result{}, fmt.Errorf("ffmpeg path is required")
	}
	args, err := BuildFFmpegArgs(in, doc)
	if err != nil {
		return Result{}, err
	}
	if err := os.MkdirAll(filepath.Dir(in.OutputPath), 0o750); err != nil {
		return Result{}, fmt.Errorf("create output directory: %w", err)
	}
	logBuf, err := runFFmpeg(ctx, ffmpegPath, args)
	if err != nil {
		return Result{}, fmt.Errorf("render timeline: %w", err)
	}
	coverPath := strings.TrimSuffix(in.OutputPath, filepath.Ext(in.OutputPath)) + ".jpg"
	if err := extractCover(ctx, ffmpegPath, in.OutputPath, coverPath); err != nil {
		return Result{}, err
	}
	qaLog, qaErr := runQA(ctx, ffmpegPath, in.OutputPath)
	warnings := editor.QualityWarningsFromFFmpegLog("timeline", string(qaLog))
	if qaErr != nil && len(warnings) == 0 {
		warnings = append(warnings, "quality timeline: qa probe failed")
	}
	return Result{
		OutputPath: in.OutputPath,
		CoverPath:  coverPath,
		Duration:   doc.DurationSeconds(),
		Width:      doc.Canvas.Width,
		Height:     doc.Canvas.Height,
		Warnings:   warnings,
		Log:        string(logBuf),
	}, nil
}

func runFFmpeg(ctx context.Context, ffmpegPath string, args []string) ([]byte, error) {
	// #nosec G204 -- ffmpegPath comes from local tool detection; args are built
	// from the persisted timeline plan, never from remote input.
	cmd := exec.CommandContext(ctx, ffmpegPath, args...)
	var buf bytes.Buffer
	cmd.Stdout = &buf
	cmd.Stderr = &buf
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(buf.String())
		if msg != "" {
			return buf.Bytes(), fmt.Errorf("%w: %s", err, truncate(msg, 2000))
		}
		return buf.Bytes(), err
	}
	return buf.Bytes(), nil
}

func extractCover(ctx context.Context, ffmpegPath, videoPath, coverPath string) error {
	args := []string{"-hide_banner", "-y", "-ss", "0.2", "-i", videoPath, "-frames:v", "1", "-q:v", "2", coverPath}
	if _, err := runFFmpeg(ctx, ffmpegPath, args); err != nil {
		return fmt.Errorf("extract cover: %w", err)
	}
	info, err := os.Stat(coverPath)
	if err != nil || info.Size() == 0 {
		return fmt.Errorf("cover artifact is missing or empty")
	}
	return nil
}

func runQA(ctx context.Context, ffmpegPath, videoPath string) ([]byte, error) {
	args := []string{
		"-hide_banner",
		"-i", videoPath,
		"-vf", "blackdetect=d=0.5:pix_th=0.10,freezedetect=n=0.003:d=0.5",
		"-f", "null",
		"-",
	}
	return runFFmpeg(ctx, ffmpegPath, args)
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}
