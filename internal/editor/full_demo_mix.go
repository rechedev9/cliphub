package editor

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/rechedev9/cliphub/internal/composition"
	"github.com/rechedev9/cliphub/internal/recapplan"
)

const fullDemoCaptureSeekPrerollFrames int64 = 2 * recapplan.OutputFPS

func fullDemoMusicSamples(timeline []recapplan.TimelineItem) []int64 {
	samples := make([]int64, len(timeline))
	var music int64
	for i, item := range timeline {
		samples[i] = music
		if item.Role == "round" {
			music += item.EndSample - item.StartSample
		}
	}
	return samples
}

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
func fullDemoRoundAudio(options recapplan.AudioOptions, gameStartSample, samples int64, voiceCount, musicInput int) string {
	return fullDemoRoundAudioWithTransitions(options, gameStartSample, samples, voiceCount, musicInput, fullDemoTransitionEdges{})
}

func fullDemoRoundAudioWithTransitions(options recapplan.AudioOptions, gameStartSample, samples int64, voiceCount, musicInput int, edges fullDemoTransitionEdges) string {
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
	if musicInput >= 0 {
		clauses = append(clauses, sampleWindow(fmt.Sprintf("[%d:a]", musicInput), 0, samples, 1, "musicraw"), "[musicraw]volume="+decimal(options.Music.BedGainDB)+"dB[musicbed]")
		clauses = append(clauses, "[gsc]volume="+decimal(options.Music.Ducking.GameContribution)+"[gduck]", "[gduck][vscm]amix=inputs=2:duration=first:normalize=0:dropout_transition=0[trigger]")
		if options.Music.Ducking.Enabled {
			clauses = append(clauses, "[trigger]apad[triggerpad]", "[musicbed][triggerpad]"+sidechainFilter(options.Music.Ducking)+"[music]")
		} else {
			clauses = append(clauses, "[musicbed]anull[music]", "[trigger]anullsink")
		}
		clauses = append(clauses, "[game][vm][music]amix=inputs=3:duration=first:normalize=0:dropout_transition=0[mixed]")
	} else {
		clauses = append(clauses, "[gsc]anullsink", "[vscm]anullsink", "[game][vm]amix=inputs=2:duration=first:normalize=0:dropout_transition=0[mixed]")
	}
	clauses = append(clauses, fmt.Sprintf("[mixed]apad=whole_len=%d,atrim=end_sample=%d,asetpts=N/SR/TB[a]", samples, samples))
	return strings.Join(clauses, ";")
}

func prepareFullDemoTracks(ctx context.Context, short *ShortEdit, progress fullDemoProgress) error {
	runtime := short.fullDemo
	if err := os.MkdirAll(runtime.workDir, 0700); err != nil {
		return err
	}
	options := short.FullDemo.Effective.Options.Audio
	reference := options.Loudness
	reference.TargetILUFS, reference.TargetTPDBTP = -16, -1.5
	steps := 2 * len(runtime.execution.VoiceTracks)
	if options.Music.Enabled && len(options.Music.Assets) > 0 {
		steps += 2*len(options.Music.Assets) + 1
	}
	step := 0
	nextPass := func(stage string) func(float64) {
		start := float64(step) / float64(max(1, steps))
		step++
		return progress.pass(stage, start, float64(step)/float64(max(1, steps)))
	}
	playlistParts := []string{}
	var playlistDuration float64
	for i, ref := range options.Music.Assets {
		if !options.Music.Enabled {
			break
		}
		source, err := runtime.execution.assetPath(ref)
		if err != nil {
			return err
		}
		var frames int64
		for _, asset := range short.FullDemo.Effective.Assets {
			if asset.Ref == ref {
				frames = asset.DurationFrames
			}
		}
		duration := float64(frames) / recapplan.OutputFPS
		measured, err := measureLoudness(ctx, runtime.ffmpeg, source, reference, filepath.Join(runtime.workDir, fmt.Sprintf("music-%d-reference.txt", i)), duration, nextPass(fmt.Sprintf("Analizando música (%d/%d)", i+1, len(options.Music.Assets))))
		if err != nil {
			return err
		}
		if measured.Status != "measured" {
			return fmt.Errorf("full_demo_asset_missing: enabled music asset %s is silent", ref.ID)
		}
		filter, err := measuredLoudnessFilter(reference, measured)
		if err != nil {
			return err
		}
		if frames < 1 {
			return fmt.Errorf("invalid music asset frame duration")
		}
		path := filepath.Join(runtime.workDir, fmt.Sprintf("music-%d.wav", i))
		samples := frames * recapplan.SamplesPerFrame
		command := []string{runtime.ffmpeg, "-y", "-v", "error", "-i", source, "-map", "0:a:0", "-vn", "-af", filter + fmt.Sprintf(",aresample=48000,aformat=channel_layouts=stereo,apad=whole_len=%d,atrim=end_sample=%d", samples, samples), "-c:a", "pcm_f32le", "-rf64", "auto", path}
		if err := runFFmpegAtomicWithProgress(ctx, command, "Full Demo music reference", "", path, duration, nextPass(fmt.Sprintf("Preparando música (%d/%d)", i+1, len(options.Music.Assets)))); err != nil {
			return err
		}
		short.FullDemo.TrackLevels = append(short.FullDemo.TrackLevels, FullDemoTrackLevel{Ref: ref.ID, Role: "music", Measurement: measured, AppliedGainDB: options.Music.BedGainDB, Policy: options.Music.ReferenceLevel})
		playlistParts = append(playlistParts, path)
		playlistDuration += duration
	}
	if len(playlistParts) > 0 {
		list := filepath.Join(runtime.workDir, "music-playlist.ffconcat")
		if err := writeMediaConcatList(list, playlistParts); err != nil {
			return err
		}
		runtime.playlist = filepath.Join(runtime.workDir, "music-playlist.wav")
		command := []string{runtime.ffmpeg, "-y", "-v", "error", "-f", "concat", "-safe", "0", "-i", list, "-map", "0:a:0", "-c:a", "pcm_f32le", "-rf64", "auto", runtime.playlist}
		if err := runFFmpegAtomicWithProgress(ctx, command, "Full Demo playlist", "", runtime.playlist, playlistDuration, nextPass("Uniendo pistas de música")); err != nil {
			return err
		}
	}
	voiceDuration := 0.0
	if runtime.recording.Plan.Tickrate > 0 {
		voiceDuration = float64(runtime.recording.Plan.DemoDurationTicks) / float64(runtime.recording.Plan.Tickrate)
	}
	for i, voice := range runtime.execution.VoiceTracks {
		measurement, err := measureLoudness(ctx, runtime.ffmpeg, voice.Path, reference, filepath.Join(runtime.workDir, fmt.Sprintf("voice-%d-reference.txt", i)), voiceDuration, nextPass(fmt.Sprintf("Analizando voces (%d/%d)", i+1, len(runtime.execution.VoiceTracks))))
		if err != nil {
			return err
		}
		gainDB := 0.0
		if options.Voice.Normalization == "bounded-activity-v1" && measurement.Status == "measured" {
			// Gated integrated loudness ignores long silent spans; bound gain to
			// 9 dB and preserve 3 dB peak headroom instead of boosting silence.
			gainDB = min(9.0, max(-9.0, -20-*measurement.IntegratedLUFS))
			gainDB = min(gainDB, -3-*measurement.TruePeakDBTP)
		}
		path := filepath.Join(runtime.workDir, fmt.Sprintf("voice-%d.wav", i))
		command := []string{runtime.ffmpeg, "-y", "-v", "error", "-i", voice.Path, "-map", "0:a:0", "-af", "aresample=48000,aformat=channel_layouts=stereo,volume=" + decimal(gainDB) + "dB", "-c:a", "pcm_f32le", "-rf64", "auto", path}
		if err := runFFmpegAtomicWithProgress(ctx, command, "Full Demo voice reference", "", path, voiceDuration, nextPass(fmt.Sprintf("Preparando voces (%d/%d)", i+1, len(runtime.execution.VoiceTracks)))); err != nil {
			return err
		}
		runtime.voicePaths = append(runtime.voicePaths, path)
		short.FullDemo.TrackLevels = append(short.FullDemo.TrackLevels, FullDemoTrackLevel{Ref: voice.StorageKey, Role: "team-voice", Measurement: measurement, AppliedGainDB: gainDB, Policy: options.Voice.Normalization})
	}
	progress.report("Audio preparado", 1)
	return nil
}

func writeMediaConcatList(path string, inputs []string) error {
	var content strings.Builder
	content.WriteString("ffconcat version 1.0\n")
	for _, input := range inputs {
		content.WriteString(composition.ConcatFileLine(input))
	}
	return os.WriteFile(path, []byte(content.String()), 0600)
}

func fullDemoItemCommand(short ShortEdit, item recapplan.TimelineItem, musicSample int64, output string) ([]string, error) {
	runtime := short.fullDemo
	options := short.FullDemo.Effective.Options
	frames, samples := item.EndFrame-item.StartFrame, item.EndSample-item.StartSample
	edges := fullDemoEdges(short, item)
	command := []string{runtime.ffmpeg, "-y", "-v", "error"}
	var audio string
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
		musicInput := -1
		if runtime.playlist != "" {
			if options.Audio.Music.LoopPolicy == "ordered-loop" {
				command = append(command, "-stream_loop", "-1")
			}
			command = append(command, "-ss", decimal(float64(musicSample)/recapplan.SampleRate), "-i", runtime.playlist)
			musicInput = 1 + len(runtime.voicePaths) + edges.tailCount
		}
		audio = fullDemoRoundAudioWithTransitions(options.Audio, trimStart*recapplan.SamplesPerFrame, samples, len(runtime.voicePaths), musicInput, edges)
	} else if item.Role == "sponsor" {
		video, err := runtime.execution.assetPath(*options.Sponsor.Video)
		if err != nil {
			return nil, err
		}
		command = append(command, "-i", video)
		audioInput := "[0:a]"
		if options.Sponsor.AudioPolicy == "replace-narration" {
			narration, err := runtime.execution.assetPath(*options.Sponsor.Narration)
			if err != nil {
				return nil, err
			}
			command = append(command, "-i", narration)
			audioInput = "[1:a]"
		}
		audio = sampleWindow(audioInput, 0, samples, 1, "a")
	} else {
		return nil, fmt.Errorf("unsupported Full Demo timeline role %s", item.Role)
	}
	video := fmt.Sprintf("[0:v]fps=60,trim=start_frame=%d:end_frame=%d,setpts=PTS-STARTPTS,scale=1920:1080:force_original_aspect_ratio=decrease:force_divisible_by=2,pad=1920:1080:(ow-iw)/2:(oh-ih)/2,setsar=1,format=yuv420p%s[v]", trimStart, trimStart+frames, fullDemoTransitionVideo(short, item, edges))
	hudFilter, err := fullDemoHUDFilter(short, item, output)
	if err != nil {
		return nil, err
	}
	if hudFilter != "" {
		video = strings.TrimSuffix(video, "[v]") + "," + hudFilter + "[v]"
	}
	// Five millisecond de-clicks keep hard cuts while preserving every frame
	// and sample in the approved timeline, including very short inserts.
	fadeSamples := min(int64(240), samples/2)
	audio += fmt.Sprintf(";[a]afade=t=in:ss=0:ns=%d,afade=t=out:ss=%d:ns=%d[declicked]", fadeSamples, samples-fadeSamples, fadeSamples)
	sfx, audioLabel := fullDemoTransitionSFX(edges, samples)
	command = append(command, "-filter_complex", video+";"+audio+sfx, "-map", "[v]", "-map", "["+audioLabel+"]")
	command = appendVideoEncodeArgs(command, short)
	command = append(command, "-bf", "0", "-c:a", "pcm_f32le", "-ar", "48000", "-ac", "2")
	command = appendThreadArgs(command, short)
	return append(command, output), nil
}

func prepareFullDemoCompilation(ctx context.Context, short *ShortEdit, progress fullDemoProgress) error {
	if short.fullDemo == nil {
		return nil
	}
	if err := prepareFullDemoTracks(ctx, short, progress.within(0, .3)); err != nil {
		return err
	}
	if err := prepareFullDemoTransitions(ctx, short); err != nil {
		return err
	}
	timeline := short.FullDemo.Effective.Timeline
	totalFrames := float64(timeline[len(timeline)-1].EndFrame)
	musicSamples := fullDemoMusicSamples(timeline)
	paths := make([]string, len(timeline))
	tracker := &fullDemoItemProgress{
		fractions: make([]float64, len(timeline)),
		done:      make([]bool, len(timeline)),
		weights:   make([]float64, len(timeline)),
		total:     totalFrames,
		progress:  progress,
	}
	for i, item := range timeline {
		tracker.weights[i] = float64(item.EndFrame - item.StartFrame)
	}
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	var (
		wg       sync.WaitGroup
		once     sync.Once
		firstErr error
	)
	sem := make(chan struct{}, fullDemoItemJobs())
	for i, item := range timeline {
		path := filepath.Join(short.fullDemo.workDir, fmt.Sprintf("item-%03d.nut", i))
		command, err := fullDemoItemCommand(*short, item, musicSamples[i], path)
		if err != nil {
			cancel()
			wg.Wait()
			return err
		}
		paths[i] = path
		if ctx.Err() != nil {
			break
		}
		wg.Add(1)
		sem <- struct{}{}
		go func(i int, item recapplan.TimelineItem, command []string, path string) {
			defer wg.Done()
			defer func() { <-sem }()
			stage := fmt.Sprintf("Montando corte %d de %d", i+1, len(timeline))
			onFraction := func(fraction float64) { tracker.set(i, stage, fraction) }
			if err := runFFmpegAtomicWithProgress(ctx, command, "Full Demo timeline item", filepath.Join(short.fullDemo.workDir, fmt.Sprintf("item-%03d.log", i)), path, float64(item.EndFrame-item.StartFrame)/recapplan.OutputFPS, onFraction); err != nil {
				once.Do(func() {
					firstErr = err
					cancel()
				})
				return
			}
			tracker.markDone(i, stage)
		}(i, item, command, path)
	}
	wg.Wait()
	if firstErr != nil {
		return firstErr
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	short.fullDemo.preparedInputs = paths
	var list strings.Builder
	list.WriteString("ffconcat version 1.0\n")
	for i, path := range short.fullDemo.preparedInputs {
		item := short.FullDemo.Effective.Timeline[i]
		list.WriteString(composition.ConcatFileLine(path))
		// NUT's last packet timestamp is not the duration of a complete CFR
		// interval. Declare the canonical duration so concat never loses one
		// frame at each join or advances the next audio bus too early.
		fmt.Fprintf(&list, "duration %.9f\n", float64(item.EndFrame-item.StartFrame)/recapplan.OutputFPS)
	}
	if err := os.WriteFile(fullDemoConcatListPath(*short), []byte(list.String()), 0600); err != nil {
		return err
	}
	return releaseFullDemoAudio(*short)
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
	return .3 + .7*(completed/s.total)
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
