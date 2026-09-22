package editor

import (
	"context"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"

	"github.com/rechedev9/cliphub/internal/recapplan"
)

const fullDemoCaptureSeekPrerollFrames int64 = 2 * recapplan.OutputFPS

func fullDemoCaptureSeek(sourceOffset int64) (seekFrames, trimStart int64) {
	if sourceOffset <= fullDemoCaptureSeekPrerollFrames {
		return 0, sourceOffset
	}
	return sourceOffset - fullDemoCaptureSeekPrerollFrames, fullDemoCaptureSeekPrerollFrames
}

func fullDemoItemJobs() int {
	jobs := normalizeRenderJobs(0)
	if jobs > 3 {
		return 3
	}
	return jobs
}

// fullDemoAudioItemJobsMax bounds the audio-only item pool. Audio items run no
// encoder: each one demuxes its capture segment plus the prepared voice WAVs
// and writes lossless PCM through a filter graph, so a worker is far cheaper
// than an item encoder and the pool can be wider than the encoder pool. Four is
// measured, not guessed: on the saved replay every four-worker repeat beat every
// three-worker repeat (median 3.169 s versus 4.492 s), while six workers saved
// only a further ~0.8 s and ran six processes against the concurrent video
// encodes. The whole stage is a few seconds of a multi-minute render, so the
// ceiling stays small (see docs/full-demo-render-performance-audit.md).
const fullDemoAudioItemJobsMax = 4

// fullDemoAudioItemJobs bounds the audio-only item pool, which runs no encoder:
// every audio item is a filter graph writing lossless PCM. It never drops below
// the encoder budget and never exceeds the CPU count.
func fullDemoAudioItemJobs() int {
	jobs := fullDemoAudioItemJobsMax
	if cpus := runtime.NumCPU(); cpus > 0 && cpus < jobs {
		jobs = cpus
	}
	if encoders := fullDemoItemJobs(); jobs < encoders {
		jobs = encoders
	}
	return jobs
}

func sampleWindow(input string, start, count int64, gain float64, label string) string {
	return fmt.Sprintf("%saresample=48000:first_pts=0,aformat=channel_layouts=stereo,atrim=start_sample=%d:end_sample=%d,asetpts=PTS-STARTPTS,apad=whole_len=%d,atrim=end_sample=%d,volume=%s[%s]", input, start, start+count, count, count, decimal(gain), label)
}

func silentBus(samples int64, label string) string {
	return fmt.Sprintf("anullsrc=r=48000:cl=stereo,atrim=end_sample=%d[%s]", samples, label)
}

func sidechainFilter(options recapplan.DuckingOptions) string {
	return "sidechaincompress=threshold=" + decimal(options.Threshold) + ":ratio=" + decimal(options.Ratio) + ":attack=" + decimal(options.AttackMS) + ":release=" + decimal(options.ReleaseMS) + ":makeup=1:detection=rms:link=maximum"
}

// fullDemoRoundAudio uses one canonical frame/sample window for every bus.
// All mixing is unnormalized float audio; the full program owns mastering.
func fullDemoRoundAudio(options recapplan.AudioOptions, gameStartSample, samples int64, voiceCount int) string {
	return fullDemoRoundAudioWithTransitions(options, gameStartSample, samples, voiceCount, fullDemoTransitionEdges{})
}

func fullDemoRoundAudioWithTransitions(options recapplan.AudioOptions, gameStartSample, samples int64, voiceCount int, edges fullDemoTransitionEdges) string {
	clauses := []string{sampleWindow("[0:a]", gameStartSample, samples, options.Game.Gain, "graw")}
	if filter := fullDemoGameTransitionFilter(edges, samples); filter != "" {
		clauses[0] = sampleWindow("[0:a]", gameStartSample, samples, options.Game.Gain, "gwindow")
		clauses = append(clauses, "[gwindow]"+filter+"[graw]")
	}
	voices := []string{}
	for i := 0; i < voiceCount; i++ {
		label := fmt.Sprintf("voice%d", i)
		clauses = append(clauses, sampleWindow(fmt.Sprintf("[%d:a]", 1+i), 0, samples, options.Voice.Gain, label))
		voices = append(voices, "["+label+"]")
	}
	for i := 0; i < edges.tailCount; i++ {
		label := fmt.Sprintf("vtail%d", i)
		clauses = append(clauses, sampleWindow(fmt.Sprintf("[%d:a]", edges.tailInput+i), 0, edges.tailSamples, options.Voice.Gain, label+"raw"),
			fmt.Sprintf("[%sraw]afade=t=in:ss=0:ns=%d,afade=t=out:ss=0:ns=%d:curve=qsin[%s]", label, min(int64(96), edges.tailSamples/2), edges.tailSamples, label))
		voices = append(voices, "["+label+"]")
	}
	if len(voices) == 0 {
		clauses = append(clauses, silentBus(samples, "vraw"))
	} else {
		clauses = append(clauses, fmt.Sprintf("%samix=inputs=%d:duration=longest:normalize=0:dropout_transition=0[vraw]", strings.Join(voices, ""), len(voices)))
	}
	clauses = append(clauses, "[graw]asplit=2[gm][gsc]", "[vraw]asplit=3[vm][vscg][vscm]")
	if options.Game.VoicePriority {
		// The sidechain may end on a different decoder packet boundary. Its
		// silence must outlive the main bus so framesync cannot truncate audio.
		clauses = append(clauses, "[vscg]apad[vscgpad]", "[gm][vscgpad]"+sidechainFilter(options.Music.Ducking)+"[game]")
	} else {
		clauses = append(clauses, "[gm]anull[game]", "[vscg]anullsink")
	}
	clauses = append(clauses, "[gsc]anullsink", "[vscm]anullsink", "[game][vm]amix=inputs=2:duration=first:normalize=0:dropout_transition=0[mixed]")
	clauses = append(clauses, fmt.Sprintf("[mixed]apad=whole_len=%d,atrim=end_sample=%d,asetpts=N/SR/TB[a]", samples, samples))
	return strings.Join(clauses, ";")
}

// fullDemoVoiceJobsMax bounds independent voice pipelines. Voice preparation no
// longer runs before item encoding: since the video and audio branches run
// concurrently it overlaps the item encoders whatever its size, and it is the
// head of the audio critical path, so holding it to three workers only left the
// last track running alone while the whole branch waited. On the saved replay's
// five real tracks, under a concurrent video item load, the stage took
// 105.8/132.8 s with three workers and 73.4/72.8 s with five. Five is also a
// full CS2 team's voice tracks, the ceiling the team-voice policy can select,
// so a wider pool would never be used (see
// docs/full-demo-render-performance-audit.md).
const fullDemoVoiceJobsMax = 5

// fullDemoVoiceJobs bounds independent voice pipelines with its own small,
// CPU-aware pool instead of sharing the item budget.
func fullDemoVoiceJobs(trackCount int) int {
	if trackCount <= 0 {
		return 0
	}
	jobs := fullDemoVoiceJobsMax
	if cpus := runtime.NumCPU(); cpus > 0 && cpus < jobs {
		jobs = cpus
	}
	if trackCount < jobs {
		jobs = trackCount
	}
	return jobs
}

type fullDemoVoiceInput struct {
	Path       string
	StorageKey string
}

// preparedFullDemoVoice is the ordered result of one track's full pipeline.
type preparedFullDemoVoice struct {
	Path        string
	Measurement LoudnessMeasurement
	GainDB      float64
	Ref         string
}

// prepareFullDemoVoiceTracks keeps each track's current full-track measurement,
// normalization policy, gain bound, peak headroom and PCM materialization, but
// runs independent tracks in a small worker pool. Results are stored by
// original index, so voicePaths/TrackLevels stay in approved order regardless
// of which track finishes first. Cancellation stops scheduling, the first error
// wins, and each WAV is still committed atomically per track.
func prepareFullDemoVoiceTracks(ctx context.Context, ffmpeg, workDir string, voices []fullDemoVoiceInput, options recapplan.AudioOptions, voiceDuration float64, progress fullDemoProgress, jobs int) ([]preparedFullDemoVoice, error) {
	reference := options.Loudness
	reference.TargetILUFS, reference.TargetTPDBTP = -16, -1.5
	prepare := func(ctx context.Context, index int, analysisProgress, renderProgress func(float64)) (preparedFullDemoVoice, error) {
		voice := voices[index]
		measurement, err := measureLoudness(fullDemoTimingScope(ctx, "voice_analysis", index, -1, voiceDuration), ffmpeg, voice.Path, reference, filepath.Join(workDir, fmt.Sprintf("voice-%d-reference.txt", index)), voiceDuration, analysisProgress)
		if err != nil {
			return preparedFullDemoVoice{}, err
		}
		// Complete the analysis phase explicitly instead of trusting FFmpeg's
		// last progress record.
		analysisProgress(1)
		gainDB := 0.0
		if options.Voice.Normalization == "bounded-activity-v1" && measurement.Status == "measured" {
			// Gated integrated loudness ignores long silent spans; bound gain to
			// 9 dB and preserve 3 dB peak headroom instead of boosting silence.
			gainDB = min(9.0, max(-9.0, -20-*measurement.IntegratedLUFS))
			gainDB = min(gainDB, -3-*measurement.TruePeakDBTP)
		}
		path := filepath.Join(workDir, fmt.Sprintf("voice-%d.wav", index))
		command := []string{ffmpeg, "-y", "-v", "error", "-i", voice.Path, "-map", "0:a:0", "-af", "aresample=48000,aformat=channel_layouts=stereo,volume=" + decimal(gainDB) + "dB", "-c:a", "pcm_f32le", "-rf64", "auto", path}
		if err := runFFmpegAtomicWithProgress(fullDemoTimingScope(ctx, "voice_prepare", index, -1, voiceDuration), command, "Full Demo voice reference", "", path, voiceDuration, renderProgress); err != nil {
			return preparedFullDemoVoice{}, err
		}
		renderProgress(1)
		return preparedFullDemoVoice{Path: path, Measurement: measurement, GainDB: gainDB, Ref: voice.StorageKey}, nil
	}
	return runFullDemoVoicePool(ctx, len(voices), jobs, progress, prepare)
}

// runFullDemoVoicePool runs an independent per-track pipeline in a bounded
// worker pool. Each track owns two equally weighted phases (analysis and
// materialization); progress is the aggregate average across every track, so a
// short track can never report completion while another is unfinished. The
// parent progress channel is called under the aggregator lock, and fraction 1
// is reserved for the successful completion of every track.
func runFullDemoVoicePool(ctx context.Context, count, jobs int, progress fullDemoProgress, prepare func(ctx context.Context, index int, analysisProgress, renderProgress func(float64)) (preparedFullDemoVoice, error)) ([]preparedFullDemoVoice, error) {
	if count <= 0 {
		return nil, nil
	}
	if jobs < 1 {
		jobs = 1
	}
	if jobs > count {
		jobs = count
	}
	results := make([]preparedFullDemoVoice, count)
	aggregate := newFullDemoVoiceProgress(count, progress)
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	var (
		wg       sync.WaitGroup
		once     sync.Once
		firstErr error
	)
	sem := make(chan struct{}, jobs)
scheduling:
	for index := 0; index < count; index++ {
		// Never wait on a saturated pool without watching cancellation, and
		// recheck the context after acquiring a slot so a queued track cannot
		// start once the pool is cancelled.
		select {
		case sem <- struct{}{}:
		case <-ctx.Done():
			break scheduling
		}
		if ctx.Err() != nil {
			<-sem
			break scheduling
		}
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			defer func() { <-sem }()
			analysisStage := fmt.Sprintf("Analizando voces (%d/%d)", index+1, count)
			materialStage := fmt.Sprintf("Preparando voces (%d/%d)", index+1, count)
			analysis := func(fraction float64) { aggregate.phase(index, false, fraction, analysisStage) }
			render := func(fraction float64) { aggregate.phase(index, true, fraction, materialStage) }
			result, err := prepare(ctx, index, analysis, render)
			if err != nil {
				once.Do(func() {
					firstErr = err
					cancel()
				})
				return
			}
			aggregate.complete(index, materialStage)
			results[index] = result
		}(index)
	}
	wg.Wait()
	if firstErr != nil {
		return nil, firstErr
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return results, nil
}

// fullDemoVoiceProgress aggregates every track's two phase fractions into one
// monotonic value. Fraction 1 is only reachable once every track has been
// explicitly completed.
type fullDemoVoiceProgress struct {
	mu        sync.Mutex
	parent    fullDemoProgress
	analysis  []float64
	material  []float64
	completed []bool
}

func newFullDemoVoiceProgress(count int, parent fullDemoProgress) *fullDemoVoiceProgress {
	return &fullDemoVoiceProgress{
		parent:    parent,
		analysis:  make([]float64, count),
		material:  make([]float64, count),
		completed: make([]bool, count),
	}
}

func (p *fullDemoVoiceProgress) phase(index int, material bool, fraction float64, stage string) {
	if p == nil || p.parent == nil {
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if index < 0 || index >= len(p.completed) {
		return
	}
	target := &p.analysis[index]
	if material {
		target = &p.material[index]
	}
	clamped := min(1, max(0, fraction))
	if clamped <= *target {
		return
	}
	*target = clamped
	p.parent(stage, p.overallLocked())
}

func (p *fullDemoVoiceProgress) complete(index int, stage string) {
	if p == nil || p.parent == nil {
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if index < 0 || index >= len(p.completed) || p.completed[index] {
		return
	}
	p.completed[index] = true
	p.analysis[index] = 1
	p.material[index] = 1
	p.parent(stage, p.overallLocked())
}

func (p *fullDemoVoiceProgress) overallLocked() float64 {
	count := len(p.completed)
	if count == 0 {
		return 1
	}
	sum := 0.0
	allComplete := true
	for i := range p.completed {
		sum += p.analysis[i] + p.material[i]
		if !p.completed[i] {
			allComplete = false
		}
	}
	if allComplete {
		return 1
	}
	// While any track is unfinished, keep the aggregate strictly below 1 even
	// if every phase fraction individually reached its maximum. Do not shrink
	// the cap with the track count: that would stall a one-track
	// materialization phase at 50%.
	if aggregate := sum / (2 * float64(count)); aggregate >= 1 {
		return math.Nextafter(1, 0)
	} else {
		return aggregate
	}
}

func prepareFullDemoTracks(ctx context.Context, short *ShortEdit, progress fullDemoProgress) error {
	runtime := short.fullDemo
	if err := os.MkdirAll(runtime.workDir, 0700); err != nil {
		return err
	}
	options := short.FullDemo.Effective.Options.Audio
	voices := make([]fullDemoVoiceInput, len(runtime.execution.VoiceTracks))
	for i, voice := range runtime.execution.VoiceTracks {
		voices[i] = fullDemoVoiceInput{Path: voice.Path, StorageKey: voice.StorageKey}
	}
	voiceDuration := 0.0
	if runtime.recording.Plan.Tickrate > 0 {
		voiceDuration = float64(runtime.recording.Plan.DemoDurationTicks) / float64(runtime.recording.Plan.Tickrate)
	}
	prepared, err := prepareFullDemoVoiceTracks(ctx, runtime.ffmpeg, runtime.workDir, voices, options, voiceDuration, progress, fullDemoVoiceJobs(len(voices)))
	if err != nil {
		return err
	}
	for _, voice := range prepared {
		runtime.voicePaths = append(runtime.voicePaths, voice.Path)
		short.FullDemo.TrackLevels = append(short.FullDemo.TrackLevels, FullDemoTrackLevel{Ref: voice.Ref, Role: "team-voice", Measurement: voice.Measurement, AppliedGainDB: voice.GainDB, Policy: options.Voice.Normalization})
	}
	progress.report("Audio preparado", 1)
	return nil
}

// fullDemoItemStreams selects which half of a timeline item one FFmpeg process
// renders. The video and audio graphs never exchange frames, so production
// renders them as two independent processes: item video does not wait for voice
// preparation, and the program audio can be mastered while video still encodes.
// The muxed form builds the same two graphs in one process and stays as the
// equivalence reference.
type fullDemoItemStreams int

const (
	fullDemoItemMuxed fullDemoItemStreams = iota
	fullDemoItemVideoOnly
	fullDemoItemAudioOnly
)

func fullDemoItemCommand(short ShortEdit, item recapplan.TimelineItem, output string) ([]string, error) {
	return fullDemoItemStreamCommand(short, item, output, fullDemoItemMuxed)
}

func fullDemoItemVideoCommand(short ShortEdit, item recapplan.TimelineItem, output string) ([]string, error) {
	return fullDemoItemStreamCommand(short, item, output, fullDemoItemVideoOnly)
}

func fullDemoItemAudioCommand(short ShortEdit, item recapplan.TimelineItem, output string) ([]string, error) {
	return fullDemoItemStreamCommand(short, item, output, fullDemoItemAudioOnly)
}

func fullDemoItemStreamCommand(short ShortEdit, item recapplan.TimelineItem, output string, streams fullDemoItemStreams) ([]string, error) {
	withVideo, withAudio := streams != fullDemoItemAudioOnly, streams != fullDemoItemVideoOnly
	runtime := short.fullDemo
	options := short.FullDemo.Effective.Options
	frames, samples := item.EndFrame-item.StartFrame, item.EndSample-item.StartSample
	edges := fullDemoEdges(short, item)
	command := []string{runtime.ffmpeg, "-y", "-v", "error"}
	var audio string
	var maps []string
	var sourceOffset, trimStart int64
	if item.Role == "round" {
		var input string
		var captureStart int
		for _, part := range short.Parts {
			if part.SegmentID == item.SourceRef {
				input = part.Input
				break
			}
		}
		for _, segment := range runtime.recording.Plan.Segments {
			if segment.ID == item.SourceRef {
				captureStart = segment.TickStart
				break
			}
		}
		if input == "" {
			return nil, fmt.Errorf("full demo round input missing: %s", item.SourceRef)
		}
		offset, err := recapplan.TickFrames(item.SourceStartTick-captureStart, short.FullDemo.Effective.Clock.TickRate)
		if err != nil {
			return nil, err
		}
		sourceOffset = offset + item.SourceOffsetFrames
		var seekFrames int64
		seekFrames, trimStart = fullDemoCaptureSeek(sourceOffset)
		if seekFrames > 0 {
			command = append(command, "-ss", decimal(float64(seekFrames)/recapplan.OutputFPS))
		}
		command = append(command, "-i", input)
		voiceFrame, err := recapplan.TickFrames(item.SourceStartTick, short.FullDemo.Effective.Clock.TickRate)
		if err != nil {
			return nil, err
		}
		if withAudio {
			// Voice inputs keep the same indices in the muxed and audio-only
			// forms; overlay images are appended after them and only for video.
			voiceSample := (voiceFrame + item.SourceOffsetFrames) * recapplan.SamplesPerFrame
			for _, voice := range runtime.voicePaths {
				command = append(command, "-ss", decimal(float64(voiceSample)/recapplan.SampleRate), "-i", voice)
			}
			tailStart, tailSamples := fullDemoCommsTail(short.FullDemo.Effective, edges.in)
			if tailSamples > 0 {
				edges.tailInput, edges.tailCount, edges.tailSamples = 1+len(runtime.voicePaths), len(runtime.voicePaths), tailSamples
				for _, voice := range runtime.voicePaths {
					command = append(command, "-ss", decimal(float64(tailStart)/recapplan.SampleRate), "-i", voice)
				}
			}
			audio = fullDemoRoundAudioWithTransitions(options.Audio, trimStart*recapplan.SamplesPerFrame, samples, len(runtime.voicePaths), edges)
		}
	} else if item.Role == "sponsor" {
		video, err := runtime.execution.assetPath(*options.Sponsor.Video)
		if err != nil {
			return nil, err
		}
		command = append(command, "-i", video)
		audioInput := "[0:a]"
		if withAudio && options.Sponsor.AudioPolicy == "replace-narration" {
			narration, err := runtime.execution.assetPath(*options.Sponsor.Narration)
			if err != nil {
				return nil, err
			}
			command = append(command, "-i", narration)
			audioInput = "[1:a]"
		}
		audio = sampleWindow(audioInput, 0, samples, 1, "a")
	} else if item.Role == "bumper" {
		// Uploaded intro/outro clips retain their embedded audio. A silent clip
		// gets a silent bed instead of mapping a missing [0:a] stream; boundary
		// transition sounds are mixed below without extending the clip.
		ref, evidence, err := fullDemoBumperAsset(short.FullDemo.Effective, item)
		if err != nil {
			return nil, err
		}
		video, err := runtime.execution.assetPath(ref)
		if err != nil {
			return nil, err
		}
		command = append(command, "-i", video)
		if evidence.HasAudio {
			audio = sampleWindow("[0:a]", 0, samples, 1, "a")
		} else {
			audio = silentBus(samples, "a")
		}
	} else {
		return nil, fmt.Errorf("unsupported Full Demo timeline role %s", item.Role)
	}
	var clauses []string
	if withVideo {
		video := fmt.Sprintf("[0:v]fps=60,trim=start_frame=%d:end_frame=%d,setpts=PTS-STARTPTS,scale=1920:1080:force_original_aspect_ratio=decrease:force_divisible_by=2,pad=1920:1080:(ow-iw)/2:(oh-ih)/2,setsar=1,format=yuv420p%s", trimStart, trimStart+frames, fullDemoTransitionVideo(short, item, edges))
		hudFilter, err := fullDemoHUDFilter(short, item, output)
		if err != nil {
			return nil, err
		}
		if hudFilter != "" {
			video += "," + hudFilter
		}
		command, video, err = fullDemoHUDPortrait(short, item, command, video)
		if err != nil {
			return nil, err
		}
		// Supported global intro/outro overlays are composed after the item's
		// transitions and HUD. The item base is shifted onto the global frame clock
		// and the original whole-program graph is reused unchanged, then the output
		// is shifted back to item-local PTS, so the program concat can copy the
		// compatible H.264 stream. Unsupported effect combinations keep the
		// byte-identical legacy item chain and the legacy post-concat pass.
		videoClauses, videoLabel := func() ([]string, string) {
			if !fullDemoItemOverlayEligible(short) {
				return fullDemoItemVideoClauses(short, item, video, nil, 0)
			}
			images := imageEffects(short.Effects)
			for _, effect := range images {
				command = append(command, "-i", effect.Path)
			}
			imageInputStart := fullDemoInputCount(command) - len(images)
			return fullDemoItemVideoClauses(short, item, video, images, imageInputStart)
		}()
		clauses = append(clauses, strings.Join(videoClauses, ";"))
		maps = append(maps, "-map", videoLabel)
	}
	if withAudio {
		// Five millisecond de-clicks keep hard cuts while preserving every frame
		// and sample in the approved timeline, including very short inserts.
		fadeSamples := min(int64(240), samples/2)
		audio += fmt.Sprintf(";[a]afade=t=in:ss=0:ns=%d,afade=t=out:ss=%d:ns=%d[declicked]", fadeSamples, samples-fadeSamples, fadeSamples)
		sfx, audioLabel := fullDemoTransitionSFX(edges, samples)
		clauses = append(clauses, audio+sfx)
		maps = append(maps, "-map", "["+audioLabel+"]")
	}
	command = append(append(command, "-filter_complex", strings.Join(clauses, ";")), maps...)
	if withVideo {
		command = appendVideoEncodeArgs(command, short)
		command = append(command, "-bf", "0")
	}
	if withAudio {
		command = append(command, "-c:a", "pcm_f32le", "-ar", "48000", "-ac", "2")
	}
	if withVideo {
		command = appendThreadArgs(command, short)
	}
	return append(command, output), nil
}

type fullDemoItemProgress struct {
	mu        sync.Mutex
	fractions []float64
	done      []bool
	weights   []float64
	total     float64
	progress  fullDemoProgress
}

func (s *fullDemoItemProgress) overallLocked() float64 {
	var completed float64
	for j, w := range s.weights {
		f := s.fractions[j]
		if s.done[j] {
			f = 1
		}
		completed += w * f
	}
	if s.total <= 0 {
		return 1
	}
	return completed / s.total
}

func (s *fullDemoItemProgress) set(i int, stage string, fraction float64) {
	if s == nil {
		return
	}
	if fraction < 0 {
		fraction = 0
	}
	if fraction > 1 {
		fraction = 1
	}
	s.mu.Lock()
	if i < 0 || i >= len(s.fractions) || s.done[i] {
		s.mu.Unlock()
		return
	}
	s.fractions[i] = fraction
	overall := s.overallLocked()
	s.mu.Unlock()
	s.progress.report(stage, overall)
}

func (s *fullDemoItemProgress) markDone(i int, stage string) {
	if s == nil {
		return
	}
	s.mu.Lock()
	if i < 0 || i >= len(s.fractions) {
		s.mu.Unlock()
		return
	}
	s.done[i] = true
	s.fractions[i] = 1
	overall := s.overallLocked()
	s.mu.Unlock()
	s.progress.report(stage, overall)
}

// fullDemoBumperAsset resolves which bumper slot a timeline item plays and its
// planner evidence, so the audio graph can decide between the clip's own track
// and silence without probing the file again at render time.
func fullDemoBumperAsset(d recapplan.Document, item recapplan.TimelineItem) (recapplan.AssetRef, recapplan.AssetEvidence, error) {
	var slot recapplan.BumperSlot
	var ok bool
	switch item.Reason {
	case recapplan.BumperRoleIntro:
		slot, ok = d.Options.IntroBumper()
	case recapplan.BumperRoleOutro:
		slot, ok = d.Options.OutroBumper()
	}
	if !ok || slot.Video == nil || slot.Video.ID != item.SourceRef {
		return recapplan.AssetRef{}, recapplan.AssetEvidence{}, fmt.Errorf("full_demo_asset_missing: bumper %s is not approved", item.SourceRef)
	}
	for _, a := range d.Assets {
		if a.Ref == *slot.Video {
			return *slot.Video, a, nil
		}
	}
	return recapplan.AssetRef{}, recapplan.AssetEvidence{}, fmt.Errorf("full_demo_asset_missing: bumper %s has no plan evidence", item.SourceRef)
}
