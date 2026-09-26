package editor

import (
	"context"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/rechedev9/cliphub/internal/recapplan"
	"github.com/rechedev9/cliphub/internal/recording"
)

// Render lab modes. Each one runs a single stage of the Full Demo render from
// the same inputs a render receives, so a change can be observed at the stage
// it touches without encoding the whole program.
const (
	LabModePlan     = "plan"
	LabModeCommands = "commands"
	LabModeItem     = "item"
	LabModeAudio    = "audio"
	LabModeDelivery = "delivery"
)

// LabModes lists the supported modes in the order the CLI documents them.
func LabModes() []string {
	return []string{LabModePlan, LabModeCommands, LabModeItem, LabModeAudio, LabModeDelivery}
}

// LabOptions selects the stage and where its work files go. WorkDir is kept
// after the run: it holds the media and logs the evidence points to.
type LabOptions struct {
	Mode    string
	WorkDir string
	// Index is the timeline item rendered by the item mode.
	Index int
	// Seconds bounds the item mode to a prefix of the item; 0 renders it whole.
	Seconds float64
	// File is the delivered MP4 the delivery mode verifies.
	File string
}

// LabEvidence is the machine-readable outcome of one lab run.
type LabEvidence struct {
	Mode      string                      `json:"mode"`
	WorkDir   string                      `json:"work_dir"`
	ElapsedMS int64                       `json:"elapsed_ms"`
	Plan      *LabPlan                    `json:"plan,omitempty"`
	Commands  *LabCommands                `json:"commands,omitempty"`
	Item      *LabItemEvidence            `json:"item,omitempty"`
	Audio     *LabAudioEvidence           `json:"audio,omitempty"`
	Delivery  *LabDeliveryEvidence        `json:"delivery,omitempty"`
	Warnings  []string                    `json:"warnings,omitempty"`
	Error     string                      `json:"error,omitempty"`
	TailPads  []recording.FullDemoTailPad `json:"capture_tail_pads,omitempty"`
}

// LabPlan is the effective program as the render would build it: no FFmpeg runs.
type LabPlan struct {
	Frames          int64                        `json:"frames"`
	DurationSeconds float64                      `json:"duration_seconds"`
	HUDTheme        string                       `json:"hud_theme,omitempty"`
	Loudness        LabLoudnessPlan              `json:"loudness"`
	SilentApproved  bool                         `json:"silent_approved"`
	Items           []LabPlanItem                `json:"items"`
	Overlays        []LabPlanOverlay             `json:"overlays"`
	Transitions     []FullDemoTransitionEvidence `json:"transitions,omitempty"`
}

// LabLoudnessPlan shows the approved target and the clamped target the first
// native AAC master uses.
type LabLoudnessPlan struct {
	Target      recapplan.LoudnessOptions `json:"target"`
	FirstMaster recapplan.LoudnessOptions `json:"first_master"`
	Filter      string                    `json:"filter"`
}

type LabPlanItem struct {
	Index           int     `json:"index"`
	Role            string  `json:"role"`
	SourceRef       string  `json:"source_ref,omitempty"`
	StartFrame      int64   `json:"start_frame"`
	EndFrame        int64   `json:"end_frame"`
	StartSeconds    float64 `json:"start_seconds"`
	Seconds         float64 `json:"seconds"`
	SourceStartTick int     `json:"source_start_tick,omitempty"`
	SourceEndTick   int     `json:"source_end_tick,omitempty"`
	Reason          string  `json:"reason,omitempty"`
}

type LabPlanOverlay struct {
	Type         EffectType `json:"type"`
	StartSeconds float64    `json:"start_seconds"`
	EndSeconds   float64    `json:"end_seconds"`
	Path         string     `json:"path,omitempty"`
}

// LabCommands is every FFmpeg command of the program, built by the production
// builders and never run. Voice WAV paths are where voice preparation would
// write them; the first master's filter needs the program measurement, so it
// is shown with the target it would be built from.
type LabCommands struct {
	ItemVideo         [][]string `json:"item_video"`
	ItemAudio         [][]string `json:"item_audio"`
	ProgramVideo      []string   `json:"program_video"`
	ProgramAudio      []string   `json:"program_audio"`
	FirstMaster       []string   `json:"first_master"`
	FinalMux          []string   `json:"final_mux"`
	VoicePlaceholders []string   `json:"voice_placeholders,omitempty"`
}

// LabItemEvidence is one rendered timeline item (or a prefix of it).
type LabItemEvidence struct {
	Index          int              `json:"index"`
	Role           string           `json:"role"`
	Excerpt        bool             `json:"excerpt"`
	ExpectedFrames int64            `json:"expected_frames"`
	Command        []string         `json:"command"`
	RenderMS       int64            `json:"render_ms"`
	Media          LabMediaEvidence `json:"media"`
}

// LabAudioEvidence is the real program audio and master loop over a black
// placeholder video.
type LabAudioEvidence struct {
	Output   string                  `json:"output"`
	Loudness ProgramLoudnessEvidence `json:"loudness"`
	LogDir   string                  `json:"log_dir"`
}

// LabDeliveryEvidence is the strict delivery check plus picture measurements.
type LabDeliveryEvidence struct {
	Strict  *FullDemoDeliveryEvidence `json:"strict,omitempty"`
	Failure string                    `json:"failure,omitempty"`
	Media   LabMediaEvidence          `json:"media"`
}

// Lab prepares the render inputs exactly as Run does, attaches the Full Demo
// execution and runs one stage. It never renders the whole program.
func Lab(ctx context.Context, cfg Config, opts LabOptions) (LabEvidence, error) {
	started := time.Now()
	evidence := LabEvidence{Mode: opts.Mode}
	fail := func(err error) (LabEvidence, error) {
		evidence.ElapsedMS = time.Since(started).Milliseconds()
		evidence.Error = err.Error()
		return evidence, err
	}
	if opts.WorkDir == "" {
		return fail(fmt.Errorf("lab work directory is required"))
	}
	workDir, err := filepath.Abs(opts.WorkDir)
	if err != nil {
		return fail(err)
	}
	evidence.WorkDir = workDir
	// Dry run keeps preparation free of renders and frame probes; the lab runs
	// only the stage it was asked for.
	cfg.DryRun = true
	cfg.OutputDir = filepath.Join(workDir, "out")
	cfg.PublishDir = filepath.Join(workDir, "out", "publish")
	cfg.ProgressOutPath = ""
	in, err := prepareRunInputs(ctx, cfg)
	if err != nil {
		return fail(err)
	}
	evidence.Warnings = append(evidence.Warnings, in.manifest.Warnings...)
	if in.fullDemoExecution == nil {
		return fail(fmt.Errorf("the render lab needs a Full Demo bundle: the editor arguments have no --full-demo-execution"))
	}
	if err := attachFullDemoExecution(&in.manifest, in.recordingResult, in.fullDemoExecution, in.commandFFmpeg); err != nil {
		return fail(err)
	}
	short := in.manifest.Shorts[0]
	short.fullDemo.workDir = filepath.Join(workDir, "media")
	if err := os.MkdirAll(short.fullDemo.workDir, 0o700); err != nil {
		return fail(err)
	}
	evidence.TailPads = short.FullDemo.CaptureTailPads
	switch opts.Mode {
	case LabModePlan:
		plan := labPlan(short)
		evidence.Plan = &plan
	case LabModeCommands:
		commands, err := labCommands(ctx, &short)
		if err != nil {
			return fail(err)
		}
		evidence.Commands = commands
	case LabModeItem:
		item, err := labItem(ctx, &short, in.ffprobePath, opts)
		if item != nil {
			evidence.Item = item
		}
		if err != nil {
			return fail(err)
		}
	case LabModeAudio:
		audio, err := labAudio(ctx, &short, workDir)
		if audio != nil {
			evidence.Audio = audio
		}
		if err != nil {
			return fail(err)
		}
	case LabModeDelivery:
		delivery, err := labDelivery(ctx, short, in.ffprobePath, opts.File, workDir)
		if delivery != nil {
			evidence.Delivery = delivery
		}
		if err != nil {
			return fail(err)
		}
	default:
		return fail(fmt.Errorf("unknown lab mode %q", opts.Mode))
	}
	evidence.ElapsedMS = time.Since(started).Milliseconds()
	return evidence, nil
}

func labPlan(short ShortEdit) LabPlan {
	d := short.FullDemo.Effective
	timeline := d.Timeline
	frames := timeline[len(timeline)-1].EndFrame
	target := d.Options.Audio.Loudness
	first := aacHeadroomTarget(target)
	plan := LabPlan{
		Frames:          frames,
		DurationSeconds: float64(frames) / recapplan.OutputFPS,
		HUDTheme:        d.Options.Overlays.HUDTheme,
		Loudness:        LabLoudnessPlan{Target: target, FirstMaster: first, Filter: loudnessFilter(first)},
		SilentApproved:  fullDemoSilentApproved(short),
		Items:           make([]LabPlanItem, 0, len(timeline)),
		Overlays:        make([]LabPlanOverlay, 0, len(short.Effects)),
		Transitions:     transitionDirections(d),
	}
	for i, item := range timeline {
		plan.Items = append(plan.Items, LabPlanItem{
			Index:           i,
			Role:            item.Role,
			SourceRef:       item.SourceRef,
			StartFrame:      item.StartFrame,
			EndFrame:        item.EndFrame,
			StartSeconds:    float64(item.StartFrame) / recapplan.OutputFPS,
			Seconds:         float64(item.EndFrame-item.StartFrame) / recapplan.OutputFPS,
			SourceStartTick: item.SourceStartTick,
			SourceEndTick:   item.SourceEndTick,
			Reason:          item.Reason,
		})
	}
	for _, effect := range short.Effects {
		plan.Overlays = append(plan.Overlays, LabPlanOverlay{Type: effect.Type, StartSeconds: effect.StartSeconds, EndSeconds: effect.EndSeconds, Path: effect.Path})
	}
	return plan
}

func labCommands(ctx context.Context, short *ShortEdit) (*LabCommands, error) {
	if err := prepareFullDemoTransitions(ctx, short); err != nil {
		return nil, err
	}
	commands := &LabCommands{}
	for i := range short.fullDemo.execution.VoiceTracks {
		path := filepath.Join(short.fullDemo.workDir, fmt.Sprintf("voice-%d.wav", i))
		short.fullDemo.voicePaths = append(short.fullDemo.voicePaths, path)
		commands.VoicePlaceholders = append(commands.VoicePlaceholders, path)
	}
	timeline := short.FullDemo.Effective.Timeline
	videoPaths := make([]string, len(timeline))
	audioPaths := make([]string, len(timeline))
	for i, item := range timeline {
		videoPaths[i] = filepath.Join(short.fullDemo.workDir, fmt.Sprintf("item-%03d.nut", i))
		audioPaths[i] = filepath.Join(short.fullDemo.workDir, fmt.Sprintf("item-%03d-audio.nut", i))
		video, err := fullDemoItemStreamCommand(*short, item, videoPaths[i], fullDemoItemVideoOnly)
		if err != nil {
			return nil, fmt.Errorf("item %d video command: %w", i, err)
		}
		audio, err := fullDemoItemStreamCommand(*short, item, audioPaths[i], fullDemoItemAudioOnly)
		if err != nil {
			return nil, fmt.Errorf("item %d audio command: %w", i, err)
		}
		commands.ItemVideo = append(commands.ItemVideo, video)
		commands.ItemAudio = append(commands.ItemAudio, audio)
	}
	short.fullDemo.preparedInputs = videoPaths
	if err := writeFullDemoItemConcatList(*short, fullDemoConcatListPath(*short), videoPaths); err != nil {
		return nil, err
	}
	commands.ProgramVideo = BuildFFmpegCommand(short.fullDemo.ffmpeg, *short)
	audioList := filepath.Join(short.fullDemo.workDir, "concat-audio-list.txt")
	if err := writeFullDemoItemConcatList(*short, audioList, audioPaths); err != nil {
		return nil, err
	}
	target := short.FullDemo.Effective.Options.Audio.Loudness
	programAudio := fullDemoProgramAudioPath(*short)
	commands.ProgramAudio = buildFullDemoProgramAudioMeasuredCommand(short.fullDemo.ffmpeg, audioList, programAudio, target)
	duration := short.DurationSeconds
	candidate := short.Output + ".candidate.m4a"
	filter := loudnessFilter(aacHeadroomTarget(target)) + ":measured_I=<program>:measured_TP=<program>:measured_LRA=<program>:measured_thresh=<program>:offset=<program>:linear=true"
	commands.FirstMaster = fullDemoNativeCandidateCommand(short.fullDemo.ffmpeg, programAudio, candidate, filter, int64(math.Round(duration*recapplan.SampleRate)), duration)
	commands.FinalMux = fullDemoFinalMuxCommand(short.fullDemo.ffmpeg, fullDemoProgramPath(*short), candidate, short.Output)
	return commands, nil
}

func labItem(ctx context.Context, short *ShortEdit, ffprobe string, opts LabOptions) (*LabItemEvidence, error) {
	timeline := short.FullDemo.Effective.Timeline
	if opts.Index < 0 || opts.Index >= len(timeline) {
		return nil, fmt.Errorf("item index %d is outside the timeline (0-%d); run the plan mode to list items", opts.Index, len(timeline)-1)
	}
	if opts.Seconds < 0 {
		return nil, fmt.Errorf("seconds must be >= 0")
	}
	if err := prepareFullDemoTransitions(ctx, short); err != nil {
		return nil, err
	}
	// Item audio decodes the prepared voice WAVs, so voice preparation runs as it
	// does in a render.
	if err := prepareFullDemoTracks(ctx, short, nil); err != nil {
		return nil, err
	}
	source := timeline[opts.Index]
	item := source
	if opts.Seconds > 0 {
		item = fullDemoTimingExcerpt(source, opts.Seconds)
	}
	output := filepath.Join(short.fullDemo.workDir, fmt.Sprintf("lab-item-%03d.nut", opts.Index))
	command, err := fullDemoItemStreamCommand(*short, item, output, fullDemoItemMuxed)
	if err != nil {
		return nil, err
	}
	result := &LabItemEvidence{
		Index:          opts.Index,
		Role:           source.Role,
		Excerpt:        item.EndFrame != source.EndFrame,
		ExpectedFrames: item.EndFrame - item.StartFrame,
		Command:        command,
	}
	duration := float64(result.ExpectedFrames) / recapplan.OutputFPS
	started := time.Now()
	if err := runFFmpegAtomicWithProgress(ctx, command, "lab timeline item", filepath.Join(short.fullDemo.workDir, fmt.Sprintf("lab-item-%03d.log", opts.Index)), output, duration, nil); err != nil {
		return result, err
	}
	result.RenderMS = time.Since(started).Milliseconds()
	media, err := measureLabMedia(ctx, short.fullDemo.ffmpeg, ffprobe, output, filepath.Join(short.fullDemo.workDir, fmt.Sprintf("lab-item-%03d", opts.Index)), result.ExpectedFrames)
	result.Media = media
	return result, err
}

func labAudio(ctx context.Context, short *ShortEdit, workDir string) (*LabAudioEvidence, error) {
	if err := prepareFullDemoTransitions(ctx, short); err != nil {
		return nil, err
	}
	assembled, err := prepareFullDemoProgramAudio(ctx, short, nil)
	if err != nil {
		return nil, err
	}
	timeline := short.FullDemo.Effective.Timeline
	frames := timeline[len(timeline)-1].EndFrame
	duration := float64(frames) / recapplan.OutputFPS
	// The master loop muxes its passing candidate with the program video; a
	// small black video of the exact frame count stands in for it.
	placeholder := filepath.Join(short.fullDemo.workDir, "lab-black-program.nut")
	black := []string{short.fullDemo.ffmpeg, "-y", "-v", "error", "-f", "lavfi", "-i", "color=c=black:s=320x180:r=60", "-frames:v", strconv.FormatInt(frames, 10), "-c:v", "libx264", "-preset", "ultrafast", "-pix_fmt", "yuv420p", placeholder}
	if _, err := runFFmpegOutput(ctx, black, "lab placeholder video"); err != nil {
		return nil, err
	}
	logDir := filepath.Join(workDir, "logs")
	if err := os.MkdirAll(logDir, 0o700); err != nil {
		return nil, err
	}
	output := filepath.Join(workDir, "lab-audio-master.mp4")
	loudness, err := masterFullDemoMeasuredProgram(ctx, short.fullDemo.ffmpeg, fullDemoProgramAudioPath(*short), committedFullDemoProgramVideo(placeholder), output, logDir, short.FullDemo.Effective.Options.Audio.Loudness, fullDemoSilentApproved(*short), duration, nil, assembled)
	return &LabAudioEvidence{Output: output, Loudness: loudness, LogDir: logDir}, err
}

func labDelivery(ctx context.Context, short ShortEdit, ffprobe, file, workDir string) (*LabDeliveryEvidence, error) {
	if file == "" {
		return nil, fmt.Errorf("delivery mode needs --file with the delivered MP4")
	}
	timeline := short.FullDemo.Effective.Timeline
	frames := timeline[len(timeline)-1].EndFrame
	result := &LabDeliveryEvidence{}
	outcome, err := verifyFullDemoDeliveryWithDiagnostics(ctx, short.fullDemo.ffmpeg, ffprobe, file, frames, nil, nil)
	if err != nil {
		result.Failure = err.Error()
	} else {
		result.Strict = outcome.Evidence
	}
	media, measureErr := measureLabMedia(ctx, short.fullDemo.ffmpeg, ffprobe, file, filepath.Join(workDir, "delivery"), frames)
	result.Media = media
	if measureErr != nil {
		return result, measureErr
	}
	if err != nil {
		return result, err
	}
	return result, nil
}

// fullDemoTimingExcerpt bounds an item to a coherent prefix: the global start
// frame/sample and the source start tick stay exact, so the HUD window, source
// trim and incoming transition remain the production ones; only the end is
// shortened and clearly labelled.
func fullDemoTimingExcerpt(item recapplan.TimelineItem, seconds float64) recapplan.TimelineItem {
	fps := float64(recapplan.OutputFPS)
	frames := int64(math.Round(seconds * fps))
	sourceFrames := item.EndFrame - item.StartFrame
	if frames <= 0 || frames > sourceFrames {
		frames = sourceFrames
	}
	excerpt := item
	excerpt.EndFrame = item.StartFrame + frames
	excerpt.EndSample = item.StartSample + frames*int64(recapplan.SamplesPerFrame)
	if sourceFrames > 0 {
		tickSpan := item.SourceEndTick - item.SourceStartTick
		excerpt.SourceEndTick = item.SourceStartTick + int(math.Round(float64(tickSpan)*float64(frames)/float64(sourceFrames)))
	}
	return excerpt
}
