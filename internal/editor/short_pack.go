package editor

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"sync"

	"github.com/rechedev9/tickcut/internal/filecommit"
	"github.com/rechedev9/tickcut/internal/recording"
)

type shortPackOptions struct {
	OutputDir      string
	ResultPath     string
	PackPath       string
	FFprobePath    string
	CoversEnabled  bool
	SkipExisting   bool
	ValidateVideos bool
	RenderJobs     int
}

func renderShortPack(ctx context.Context, manifest *Manifest, result *Result, opts shortPackOptions) error {
	pack := shortPackRenderer{
		manifest: manifest,
		result:   result,
		opts:     opts,
	}
	if err := pack.render(ctx); err != nil {
		return pack.fail(err)
	}
	if err := pack.writeOutputs(); err != nil {
		return pack.fail(err)
	}
	return nil
}

type shortPackRenderer struct {
	manifest *Manifest
	result   *Result
	opts     shortPackOptions
	previous *Result
	// shortMu guards p.result.Shorts[i]/p.manifest.Shorts[i] for each short
	// index i: renderOne's publish/quality/cover goroutines can write
	// different fields of the same struct concurrently, which the race
	// detector (rightly) treats as unsafe without this.
	shortMu []sync.Mutex
}

// normalizeRenderJobs resolves the configured concurrency: 0 means automatic.
func normalizeRenderJobs(jobs int) int {
	if jobs > 0 {
		return jobs
	}
	auto := runtime.NumCPU() / 2
	if auto < 1 {
		return 1
	}
	if auto > 4 {
		return 4
	}
	return auto
}

func (p *shortPackRenderer) render(ctx context.Context) error {
	if p.opts.SkipExisting {
		p.previous = readReusableResult(p.opts.ResultPath)
	}
	jobs := normalizeRenderJobs(p.opts.RenderJobs)
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	p.shortMu = make([]sync.Mutex, len(p.manifest.Shorts))

	// Each short writes only its own index in result.Shorts/manifest.Shorts;
	// warnings are collected per short and merged in segment order afterwards
	// so output stays deterministic regardless of scheduling.
	warnings := make([][]string, len(p.manifest.Shorts))
	var (
		wg       sync.WaitGroup
		once     sync.Once
		firstErr error
	)
	sem := make(chan struct{}, jobs)
	for i := range p.manifest.Shorts {
		if ctx.Err() != nil {
			break
		}
		wg.Add(1)
		sem <- struct{}{}
		go func(i int) {
			defer wg.Done()
			defer func() { <-sem }()
			if err := p.renderOne(ctx, i, &warnings[i]); err != nil {
				once.Do(func() {
					firstErr = err
					cancel()
				})
			}
		}(i)
	}
	wg.Wait()
	for _, w := range warnings {
		p.result.Warnings = append(p.result.Warnings, w...)
	}
	return firstErr
}

// renderOne renders one short's video, then fans the independent post-encode
// work out to sibling goroutines instead of chaining it: publish (hardlink),
// quality check, and cover/cover-sheet all only read the already-written
// short.Output, so none waits on the others. Each goroutine collects its own
// warnings in a private slice; renderOne merges them back into *warn in a
// fixed publish -> quality -> cover -> cover-sheet order once every goroutine
// has finished, so the result stays deterministic regardless of which
// goroutine actually finishes first. p.shortMu[i] serializes the handful of
// p.result.Shorts[i]/p.manifest.Shorts[i] field writes these goroutines make,
// since a struct is not otherwise safe to write from more than one goroutine
// at a time even when the fields touched differ.
func (p *shortPackRenderer) renderOne(ctx context.Context, i int, warn *[]string) error {
	short := &p.manifest.Shorts[i]
	if err := p.renderShort(ctx, i, short, warn); err != nil {
		return err
	}

	var (
		publishWarn, qaWarn, coverWarn []string
		publishErr                     error
		wg                             sync.WaitGroup
	)

	wg.Add(1)
	go func() {
		defer wg.Done()
		publishErr = p.publishShort(ctx, i, short, &publishWarn)
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		p.runQualityCheck(ctx, short, &qaWarn)
	}()

	if p.opts.CoversEnabled {
		wg.Add(1)
		go func() {
			defer wg.Done()
			p.renderCover(ctx, i, short, &coverWarn)
			p.renderCoverSheet(ctx, i, short, &coverWarn)
		}()
	}

	wg.Wait()
	*warn = append(*warn, publishWarn...)
	*warn = append(*warn, qaWarn...)
	*warn = append(*warn, coverWarn...)

	if publishErr != nil {
		return publishErr
	}
	return nil
}

func (p *shortPackRenderer) renderShort(ctx context.Context, i int, short *ShortEdit, warn *[]string) error {
	if err := os.MkdirAll(filepath.Dir(short.Output), 0o750); err != nil {
		return err
	}
	if validatedExistingArtifact(p.previous, p.result.Shorts[i], short.Output, "video") {
		p.result.Shorts[i].RenderSkipped = true
	} else if err := runFFmpegAtomic(ctx, short.FFmpegCommand, "short edit", short.RenderLogPath, short.Output); err != nil {
		return err
	}
	artifact := p.probeArtifact(ctx, short.SegmentID, "short", "video", short.Output)
	p.result.Shorts[i].OutputArtifact = artifact
	p.manifest.Shorts[i].OutputArtifact = artifact
	if p.opts.ValidateVideos {
		*warn = append(*warn, validateShortArtifact(artifact, outputFPS(*short), short.OutputFormat)...)
	}
	return nil
}

// publishShort hardlinks (or, on a cross-device fallback, byte-copies) the
// rendered short to its publish path: same bytes, same inode when hardlinked.
// A second ffprobe over that identical content would only ever re-derive
// what renderShort's probe of short.Output already measured, so this reuses
// that artifact and just repoints it at the publish path and role instead of
// spawning another ffprobe process.
func (p *shortPackRenderer) publishShort(ctx context.Context, i int, short *ShortEdit, warn *[]string) error {
	if err := publishShort(*short); err != nil {
		return err
	}
	p.shortMu[i].Lock()
	source := p.result.Shorts[i].OutputArtifact
	p.shortMu[i].Unlock()
	artifact := p.derivedArtifact(source, "publish", short.PublishPath)
	p.shortMu[i].Lock()
	p.result.Shorts[i].PublishArtifact = artifact
	p.manifest.Shorts[i].PublishArtifact = artifact
	p.shortMu[i].Unlock()
	if p.opts.ValidateVideos {
		*warn = append(*warn, validateShortArtifact(artifact, outputFPS(*short), short.OutputFormat)...)
	}
	return nil
}

// derivedArtifact repoints an already-probed artifact at a different role and
// path that is known to hold identical bytes (a hardlink or byte-for-byte
// copy of the original), re-statting only the size at the new path instead of
// re-running ffprobe.
func (p *shortPackRenderer) derivedArtifact(base recording.RecordingArtifact, role, path string) recording.RecordingArtifact {
	artifact := base
	artifact.Role = role
	artifact.Path = path
	artifact.SizeBytes = 0
	if info, err := os.Stat(path); err == nil {
		artifact.SizeBytes = info.Size()
	}
	return artifact
}

func (p *shortPackRenderer) runQualityCheck(ctx context.Context, short *ShortEdit, warn *[]string) {
	if len(short.QualityCommand) == 0 {
		return
	}
	output, err := runFFmpegOutput(ctx, short.QualityCommand, "quality check")
	if short.QualityLogPath != "" {
		if writeErr := writeLogFile(short.QualityLogPath, output); writeErr != nil {
			*warn = append(*warn, fmt.Sprintf("quality log %s: %v", short.SegmentID, writeErr))
		}
	}
	if err != nil {
		*warn = append(*warn, fmt.Sprintf("quality check %s: %v", short.SegmentID, err))
		return
	}
	*warn = append(*warn, QualityWarningsFromFFmpegLog(short.SegmentID, output)...)
}

func (p *shortPackRenderer) renderCover(ctx context.Context, i int, short *ShortEdit, warn *[]string) {
	p.shortMu[i].Lock()
	current := p.result.Shorts[i]
	p.shortMu[i].Unlock()
	if validatedExistingArtifact(p.previous, current, short.CoverPath, "cover") {
		artifact := p.probeCover(ctx, short.SegmentID, "cover", short.CoverPath, short.OutputFormat, warn)
		p.shortMu[i].Lock()
		p.result.Shorts[i].CoverSkipped = true
		p.result.Shorts[i].CoverArtifact = artifact
		p.shortMu[i].Unlock()
		return
	}
	if err := runFFmpegAtomic(ctx, short.CoverCommand, "cover extract", "", short.CoverPath); err != nil {
		*warn = append(*warn, fmt.Sprintf("cover %s: %v", short.SegmentID, err))
		return
	}
	artifact := p.probeCover(ctx, short.SegmentID, "cover", short.CoverPath, short.OutputFormat, warn)
	p.shortMu[i].Lock()
	p.result.Shorts[i].CoverArtifact = artifact
	p.shortMu[i].Unlock()
}

func (p *shortPackRenderer) renderCoverSheet(ctx context.Context, i int, short *ShortEdit, warn *[]string) {
	if short.CoverSheetPath == "" {
		return
	}
	p.shortMu[i].Lock()
	current := p.result.Shorts[i]
	p.shortMu[i].Unlock()
	if validatedExistingArtifact(p.previous, current, short.CoverSheetPath, "cover-sheet") {
		artifact := p.probeCover(ctx, short.SegmentID, "cover-sheet", short.CoverSheetPath, short.OutputFormat, warn)
		p.shortMu[i].Lock()
		p.result.Shorts[i].CoverSheetSkipped = true
		p.result.Shorts[i].CoverSheetArtifact = artifact
		p.shortMu[i].Unlock()
		return
	}
	if err := runFFmpegAtomic(ctx, short.CoverSheetCommand, "cover sheet", "", short.CoverSheetPath); err != nil {
		*warn = append(*warn, fmt.Sprintf("cover sheet %s: %v", short.SegmentID, err))
		return
	}
	artifact := p.probeCover(ctx, short.SegmentID, "cover-sheet", short.CoverSheetPath, short.OutputFormat, warn)
	p.shortMu[i].Lock()
	p.result.Shorts[i].CoverSheetArtifact = artifact
	p.shortMu[i].Unlock()
}

func (p *shortPackRenderer) probeCover(ctx context.Context, segmentID, role, path, outputFormat string, warn *[]string) recording.RecordingArtifact {
	artifact := p.probeArtifact(ctx, segmentID, role, "image", path)
	*warn = append(*warn, ValidateCoverArtifact(artifact, outputFormat)...)
	return artifact
}

func (p *shortPackRenderer) probeArtifact(ctx context.Context, segmentID, role, artifactType, path string) recording.RecordingArtifact {
	artifact := recording.RecordingArtifact{
		SegmentID: segmentID,
		Role:      role,
		Type:      artifactType,
		Path:      path,
	}
	if info, err := os.Stat(path); err == nil {
		artifact.SizeBytes = info.Size()
	}
	if p.opts.FFprobePath != "" {
		recording.ProbeArtifact(ctx, p.opts.FFprobePath, &artifact)
	}
	return artifact
}

func (p *shortPackRenderer) writeOutputs() error {
	p.manifest.Warnings = append([]string(nil), p.result.Warnings...)
	if err := WriteManifest(filepath.Join(p.opts.OutputDir, "edit-manifest.json"), *p.manifest); err != nil {
		return err
	}
	if err := WritePackManifest(p.opts.PackPath, PackManifestFromManifest(*p.manifest, *p.result)); err != nil {
		return err
	}
	if err := WritePublishGallery(p.manifest.GalleryPath, *p.manifest); err != nil {
		return err
	}
	return WriteResult(p.opts.ResultPath, *p.result)
}

func (p *shortPackRenderer) fail(err error) error {
	p.result.Error = err.Error()
	// A real render that failed did not complete, so it is not executed even
	// though it started; keep the artifact honest about partial work.
	p.result.Executed = false
	_ = WriteResult(p.opts.ResultPath, *p.result)
	return err
}

func runFFmpegWithOptionalLog(ctx context.Context, command []string, label, logPath string) error {
	output, err := runFFmpegOutput(ctx, command, label)
	if err != nil && logPath != "" {
		if strings.TrimSpace(output) == "" {
			output = err.Error() + "\n"
		}
		_ = writeLogFile(logPath, output)
	}
	return err
}

func runFFmpegAtomic(ctx context.Context, command []string, label, logPath, destination string) error {
	if len(command) == 0 || destination == "" {
		return fmt.Errorf("%s output path is required", label)
	}
	attempt, cleanup, err := filecommit.Attempt(destination)
	if err != nil {
		return fmt.Errorf("%s attempt: %w", label, err)
	}
	defer cleanup()
	attemptCommand := append([]string(nil), command...)
	if attemptCommand[len(attemptCommand)-1] != destination {
		return fmt.Errorf("%s command output does not match destination", label)
	}
	attemptCommand[len(attemptCommand)-1] = attempt
	if err := runFFmpegWithOptionalLog(ctx, attemptCommand, label, logPath); err != nil {
		return err
	}
	if err := filecommit.Commit(attempt, destination); err != nil {
		return fmt.Errorf("%s publish: %w", label, err)
	}
	return nil
}

func readReusableResult(resultPath string) *Result {
	if resultPath == "" {
		return nil
	}
	// #nosec G304 -- resultPath is the editor's own canonical result artifact.
	body, err := os.ReadFile(resultPath)
	if err != nil {
		return nil
	}
	var previous Result
	if json.Unmarshal(body, &previous) != nil || previous.Error != "" || !previous.Executed || len(previous.Warnings) > 0 {
		return nil
	}
	return &previous
}

func validatedExistingArtifact(previous *Result, current ShortResult, artifactPath, role string) bool {
	if previous == nil || artifactPath == "" {
		return false
	}
	for _, short := range previous.Shorts {
		if short.SegmentID != current.SegmentID || !artifactProducerContractMatches(short, current, role) {
			continue
		}
		var artifact recording.RecordingArtifact
		switch role {
		case "video":
			artifact = short.OutputArtifact
		case "cover":
			artifact = short.CoverArtifact
		case "cover-sheet":
			artifact = short.CoverSheetArtifact
		default:
			return false
		}
		info, statErr := os.Stat(artifactPath)
		return statErr == nil && artifact.Path == artifactPath && artifact.SizeBytes > 0 &&
			artifact.SizeBytes == info.Size() && artifact.ProbeError == ""
	}
	return false
}

func artifactProducerContractMatches(previous, current ShortResult, role string) bool {
	videoMatches := len(current.FFmpegCommand) > 0 &&
		slices.Equal(previous.FFmpegCommand, current.FFmpegCommand)
	switch role {
	case "video":
		return videoMatches
	case "cover":
		return videoMatches &&
			len(current.CoverCommand) > 0 &&
			slices.Equal(previous.CoverCommand, current.CoverCommand)
	case "cover-sheet":
		return videoMatches &&
			len(current.CoverSheetCommand) > 0 &&
			slices.Equal(previous.CoverSheetCommand, current.CoverSheetCommand)
	default:
		return false
	}
}

func writeLogFile(path, content string) error {
	if path == "" {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(content), 0o600)
}
