package editor

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/rechedev9/cliphub/internal/composition"
	"github.com/rechedev9/cliphub/internal/recapplan"
)

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
	clauses := []string{sampleWindow("[0:a]", gameStartSample, samples, options.Game.Gain, "graw")}
	voices := []string{}
	for i := 0; i < voiceCount; i++ {
		label := fmt.Sprintf("voice%d", i)
		clauses = append(clauses, sampleWindow(fmt.Sprintf("[%d:a]", 1+i), 0, samples, options.Voice.Gain, label))
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

func prepareFullDemoTracks(ctx context.Context, short *ShortEdit) error {
	runtime := short.fullDemo
	if err := os.MkdirAll(runtime.workDir, 0700); err != nil {
		return err
	}
	options := short.FullDemo.Effective.Options.Audio
	reference := options.Loudness
	reference.TargetILUFS, reference.TargetTPDBTP = -16, -1.5
	playlistParts := []string{}
	for i, ref := range options.Music.Assets {
		if !options.Music.Enabled {
			break
		}
		source, err := runtime.execution.assetPath(ref)
		if err != nil {
			return err
		}
		measured, err := measureLoudness(ctx, runtime.ffmpeg, source, reference, filepath.Join(runtime.workDir, fmt.Sprintf("music-%d-reference.txt", i)))
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
		var frames int64
		for _, asset := range short.FullDemo.Effective.Assets {
			if asset.Ref == ref {
				frames = asset.DurationFrames
			}
		}
		if frames < 1 {
			return fmt.Errorf("invalid music asset frame duration")
		}
		path := filepath.Join(runtime.workDir, fmt.Sprintf("music-%d.wav", i))
		samples := frames * recapplan.SamplesPerFrame
		command := []string{runtime.ffmpeg, "-y", "-v", "error", "-i", source, "-map", "0:a:0", "-vn", "-af", filter + fmt.Sprintf(",aresample=48000,aformat=channel_layouts=stereo,apad=whole_len=%d,atrim=end_sample=%d", samples, samples), "-c:a", "pcm_f32le", "-rf64", "auto", path}
		if err := runFFmpegAtomic(ctx, command, "Full Demo music reference", "", path); err != nil {
			return err
		}
		short.FullDemo.TrackLevels = append(short.FullDemo.TrackLevels, FullDemoTrackLevel{Ref: ref.ID, Role: "music", Measurement: measured, AppliedGainDB: options.Music.BedGainDB, Policy: options.Music.ReferenceLevel})
		playlistParts = append(playlistParts, path)
	}
	if len(playlistParts) > 0 {
		list := filepath.Join(runtime.workDir, "music-playlist.ffconcat")
		if err := writeMediaConcatList(list, playlistParts); err != nil {
			return err
		}
		runtime.playlist = filepath.Join(runtime.workDir, "music-playlist.wav")
		command := []string{runtime.ffmpeg, "-y", "-v", "error", "-f", "concat", "-safe", "0", "-i", list, "-map", "0:a:0", "-c:a", "pcm_f32le", "-rf64", "auto", runtime.playlist}
		if err := runFFmpegAtomic(ctx, command, "Full Demo playlist", "", runtime.playlist); err != nil {
			return err
		}
	}
	for i, voice := range runtime.execution.VoiceTracks {
		measurement, err := measureLoudness(ctx, runtime.ffmpeg, voice.Path, reference, filepath.Join(runtime.workDir, fmt.Sprintf("voice-%d-reference.txt", i)))
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
		if err := runFFmpegAtomic(ctx, command, "Full Demo voice reference", "", path); err != nil {
			return err
		}
		runtime.voicePaths = append(runtime.voicePaths, path)
		short.FullDemo.TrackLevels = append(short.FullDemo.TrackLevels, FullDemoTrackLevel{Ref: voice.StorageKey, Role: "team-voice", Measurement: measurement, AppliedGainDB: gainDB, Policy: options.Voice.Normalization})
	}
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
	command := []string{runtime.ffmpeg, "-y", "-v", "error"}
	var audio string
	var sourceOffset int64
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
		command = append(command, "-i", input)
		voiceFrame, err := recapplan.TickFrames(item.SourceStartTick, short.FullDemo.Effective.Clock.TickRate)
		if err != nil {
			return nil, err
		}
		voiceSample := (voiceFrame + item.SourceOffsetFrames) * recapplan.SamplesPerFrame
		for _, voice := range runtime.voicePaths {
			command = append(command, "-ss", decimal(float64(voiceSample)/recapplan.SampleRate), "-i", voice)
		}
		musicInput := -1
		if runtime.playlist != "" {
			if options.Audio.Music.LoopPolicy == "ordered-loop" {
				command = append(command, "-stream_loop", "-1")
			}
			command = append(command, "-ss", decimal(float64(musicSample)/recapplan.SampleRate), "-i", runtime.playlist)
			musicInput = 1 + len(runtime.voicePaths)
		}
		audio = fullDemoRoundAudio(options.Audio, sourceOffset*recapplan.SamplesPerFrame, samples, len(runtime.voicePaths), musicInput)
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
	video := fmt.Sprintf("[0:v]fps=60,trim=start_frame=%d:end_frame=%d,setpts=PTS-STARTPTS,scale=1920:1080:force_original_aspect_ratio=decrease:force_divisible_by=2,pad=1920:1080:(ow-iw)/2:(oh-ih)/2,setsar=1,format=yuv420p[v]", sourceOffset, sourceOffset+frames)
	// Five millisecond de-clicks keep hard cuts while preserving every frame
	// and sample in the approved timeline, including very short inserts.
	fadeSamples := min(int64(240), samples/2)
	audio += fmt.Sprintf(";[a]afade=t=in:ss=0:ns=%d,afade=t=out:ss=%d:ns=%d[declicked]", fadeSamples, samples-fadeSamples, fadeSamples)
	command = append(command, "-filter_complex", video+";"+audio, "-map", "[v]", "-map", "[declicked]")
	command = appendVideoEncodeArgs(command, short)
	command = append(command, "-bf", "0", "-c:a", "pcm_f32le", "-ar", "48000", "-ac", "2")
	command = appendThreadArgs(command, short)
	return append(command, output), nil
}

func prepareFullDemoCompilation(ctx context.Context, short *ShortEdit) error {
	if short.fullDemo == nil {
		return nil
	}
	if err := prepareFullDemoTracks(ctx, short); err != nil {
		return err
	}
	var musicSample int64
	for i, item := range short.FullDemo.Effective.Timeline {
		path := filepath.Join(short.fullDemo.workDir, fmt.Sprintf("item-%03d.nut", i))
		command, err := fullDemoItemCommand(*short, item, musicSample, path)
		if err != nil {
			return err
		}
		if err := runFFmpegAtomic(ctx, command, "Full Demo timeline item", filepath.Join(short.fullDemo.workDir, fmt.Sprintf("item-%03d.log", i)), path); err != nil {
			return err
		}
		short.fullDemo.preparedInputs = append(short.fullDemo.preparedInputs, path)
		if item.Role == "round" {
			musicSample += item.EndSample - item.StartSample
		}
	}
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
	return os.WriteFile(fullDemoConcatListPath(*short), []byte(list.String()), 0600)
}
