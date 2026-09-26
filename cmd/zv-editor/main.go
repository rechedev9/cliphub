package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/rechedev9/cliphub/internal/editor"
)

func main() {
	run := runEditor
	args := os.Args[1:]
	if len(args) > 0 && args[0] == "lab" {
		run, args = runLab, args[1:]
	}
	if err := run(args); err != nil {
		log.Fatal(err)
	}
}

// editorArgs is one parsed editor invocation: the render configuration plus
// the CLI-only switches around it.
type editorArgs struct {
	config      editor.Config
	format      string
	openGallery bool
	listPresets bool
}

func runEditor(args []string) error {
	parsed, err := parseEditorArgs(args, flag.ExitOnError)
	if err != nil {
		return err
	}
	if parsed.listPresets {
		fmt.Println(strings.Join(editor.PresetNames(), "\n"))
		return nil
	}
	result, err := editor.Run(context.Background(), parsed.config)
	if err != nil {
		return err
	}
	if err := writeEditorSummary(os.Stdout, parsed.format, result); err != nil {
		return err
	}
	if parsed.openGallery {
		return openPath(result.GalleryPath)
	}
	return nil
}

// parseEditorArgs parses the editor flags. The render lab replays the exact
// arguments a render received through this same parser, so both build the
// identical configuration.
func parseEditorArgs(args []string, onError flag.ErrorHandling) (editorArgs, error) {
	fs := flag.NewFlagSet("zv-editor", onError)
	var (
		recordingResultPath = fs.String("recording-result", "", "path to recording-result.json")
		killPlanPath        = fs.String("killplan", "", "optional path to kill plan JSON; auto-discovered from pipeline-result.json when omitted")
		outDir              = fs.String("out", "", "shorts output directory")
		publishDir          = fs.String("publish-dir", "", "publish pack output directory; defaults to <out>/shortslistosparasubir")
		preset              = fs.String("preset", editor.DefaultPreset().Name, "editor preset: "+strings.Join(editor.PresetNames(), ", "))
		effectsPath         = fs.String("effects", "", "optional Lua effects script; overrides --effects-preset")
		effectsPreset       = fs.String("effects-preset", "", "effects preset: viral-ultra-clean, viral-aggressive; defaults by preset")
		musicPath           = fs.String("music", "", "optional external music file to mix into rendered shorts")
		voiceDir            = fs.String("voice-dir", "", "optional directory of POV-team Ogg tracks from zv demo voice --extract")
		musicVolume         = fs.Float64("music-volume", 1.0, "music track gain in (0,1]; higher is louder")
		gameVolume          = fs.Float64("game-volume", -1, "game-audio gain in [0,1] when mixing music; <0 keeps the historical 0.20 duck")
		voiceVolume         = fs.Float64("voice-volume", -1, "team-comms gain in [0,1]; <0 keeps the historical 0.85 gain")
		rhythmPath          = fs.String("rhythm", "", "optional rhythm JSON with segment_sync entries for compiled shorts")
		outputFormat        = fs.String("output-format", editor.OutputFormatShort9x16, "output format: short-9x16 or landscape-16x9")
		killEffect          = fs.String("kill-effect", editor.KillEffectPunchIn, "kill effect: clean, punch-in, velocity, freeze-flash, shake, glitch")
		transition          = fs.String("transition", editor.TransitionFlash, "transition style: cut, flash, whip, dip, glitch, zoom-whip")
		intro               = fs.Bool("intro", false, "add an intro title overlay")
		outro               = fs.Bool("outro", false, "add an outro title overlay")
		introText           = fs.String("intro-text", "", "custom intro overlay text; defaults to the generated headline")
		outroText           = fs.String("outro-text", "", "custom outro overlay text; defaults to \"ClipHub\"")
		hook                = fs.Bool("hook", true, "draw the generated headline as a hook over the first ~2s")
		killCounter         = fs.Bool("kill-counter", true, "pop a running kill count with 2K/3K/4K/ACE milestones")
		killfeedOverlay     = fs.Bool("killfeed-overlay", true, "re-overlay the source kill notices near the top of the 9:16 frame")
		keyDropFamily       = fs.String("keydrop-family", "", "affiliate family for the banner plate: KEYDROP or CSGOSKINS; empty with a style means KEYDROP")
		keyDropStyle        = fs.String("keydrop-style", "", "optional affiliate banner style in --keydrop-family: operator, classic, tigerr, or jcorko")
		keyDropCode         = fs.String("keydrop-code", "", "affiliate sponsor code; defaults to ZACKCSGO when style is set")
		keyDropPositionY    = fs.Float64("keydrop-position-y", 0, "KeyDrop banner vertical center 0.025-0.975; 0 uses the default")
		keyDropStart        = fs.Float64("keydrop-start", -1, "KeyDrop plate appears at this second; <0 defaults to 0")
		keyDropEnd          = fs.Float64("keydrop-end", -1, "KeyDrop plate disappears at this second; <0 defaults to full short")
		tailTrim            = fs.Float64("tail-trim", 1.5, "end each kill clip this many seconds after its final kill; 0 disables")
		outputFPS           = fs.Int("fps", 0, "optional final output FPS; defaults to 60")
		compileSegments     = fs.Bool("compile-segments", false, "render selected segments as one compilation short")
		lineupCatalogPath   = fs.String("lineup-catalog", "", "optional directory with manual smoke lineup catalog JSON files")
		segments            = fs.String("segments", "", "optional comma-separated segment ids to render, e.g. seg-001,seg-004")
		limit               = fs.Int("limit", 0, "optional max number of shorts to render after segment filtering")
		rankMoments         = fs.Bool("rank-moments", false, "score and order embedded recording segments best-first before applying --limit")
		videoCRF            = fs.Int("video-crf", 0, "x264 CRF quality from 1..51; lower is higher quality; defaults by preset")
		videoPreset         = fs.String("video-preset", "", "x264 preset; defaults by preset")
		videoEncoder        = fs.String("video-encoder", "", "final render encoder: nvenc-h264 or libx264 default")
		threads             = fs.Int("threads", 0, "cap encoder threads per render; 0 lets FFmpeg pick its own default")
		hqFilters           = fs.Bool("hq-filters", false, "use Lanczos scaling and square-pixel normalization")
		audioNormalize      = fs.Bool("audio-normalize", false, "normalize audio with FFmpeg loudnorm")
		qualityChecks       = fs.Bool("quality-checks", false, "run FFmpeg black/freeze/crop detection after rendering")
		coverSheets         = fs.Bool("cover-sheets", false, "generate tiled cover contact sheets")
		coverFirstFrame     = fs.Bool("cover-first-frame", false, "freeze the cover frame over the first frames so YouTube's Shorts thumbnail selector can pick it")
		temporalSmoothing   = fs.Bool("temporal-smoothing", false, "add subtle temporal frame blending for smoother perceived motion")
		ffmpegPath          = fs.String("ffmpeg", "", "path to ffmpeg.exe; defaults to PATH")
		ffprobePath         = fs.String("ffprobe", "", "path to ffprobe.exe; defaults to PATH")
		covers              = fs.Bool("covers", true, "generate local JPG covers for publish pack")
		noCovers            = fs.Bool("no-covers", false, "disable local JPG cover generation")
		skipExisting        = fs.Bool("skip-existing", false, "reuse existing short and cover files instead of rerendering them")
		renderJobs          = fs.Int("render-jobs", 0, "max shorts rendered concurrently; 0 selects an automatic CPU-based limit")
		openGallery         = fs.Bool("open-gallery", false, "open the publish gallery after a successful run")
		fullDemoOverlay     = fs.String("full-demo-overlay", "", "optional Full Demo intro/outro overlay JSON")
		fullDemoExecution   = fs.String("full-demo-execution", "", "Full Demo approved plan and verified local media materialization")
		overlayAssets       = fs.String("overlay-assets", "", "optional Full Demo overlay background plates directory")
		dryRun              = fs.Bool("dry-run", false, "write manifests and prompts without running FFmpeg")
		progressOut         = fs.String("progress-out", "", "optional JSON file updated with render progress")
		format              = fs.String("format", "text", "result summary format: text or json")
		listPresets         = fs.Bool("list-presets", false, "print supported preset names, one per line, and exit; used by zv short to detect stale binaries")
	)
	if err := fs.Parse(args); err != nil {
		return editorArgs{}, err
	}

	if *listPresets {
		return editorArgs{listPresets: true}, nil
	}
	if *recordingResultPath == "" || *outDir == "" {
		return editorArgs{}, fmt.Errorf("--recording-result and --out are required")
	}
	if *format != "text" && *format != "json" {
		return editorArgs{}, fmt.Errorf("unsupported format %q", *format)
	}
	if err := validateMusicVolume(*musicVolume); err != nil {
		return editorArgs{}, err
	}
	if err := validateOptionalMixVolume("game-volume", *gameVolume); err != nil {
		return editorArgs{}, err
	}
	if err := validateOptionalMixVolume("voice-volume", *voiceVolume); err != nil {
		return editorArgs{}, err
	}
	if err := validateThreads(*threads); err != nil {
		return editorArgs{}, err
	}
	coverSheetsSet := false
	fs.Visit(func(f *flag.Flag) {
		if f.Name == "cover-sheets" {
			coverSheetsSet = true
		}
	})
	segmentIDs, err := parseSegments(*segments)
	if err != nil {
		return editorArgs{}, err
	}
	var keyDropPosition *float64
	if *keyDropPositionY != 0 {
		v := *keyDropPositionY
		keyDropPosition = &v
	}
	var keyDropStartSec *float64
	if *keyDropStart >= 0 {
		v := *keyDropStart
		keyDropStartSec = &v
	}
	var keyDropEndSec *float64
	if *keyDropEnd > 0 {
		v := *keyDropEnd
		keyDropEndSec = &v
	}
	config := editor.Config{
		RecordingResultPath:   *recordingResultPath,
		KillPlanPath:          *killPlanPath,
		OutputDir:             *outDir,
		PublishDir:            *publishDir,
		Preset:                *preset,
		EffectsPath:           *effectsPath,
		EffectsPreset:         *effectsPreset,
		MusicPath:             *musicPath,
		MusicVolume:           *musicVolume,
		GameVolume:            optionalMixVolume(*gameVolume),
		VoiceVolume:           optionalMixVolume(*voiceVolume),
		VoiceDir:              *voiceDir,
		RhythmPath:            *rhythmPath,
		OutputFormat:          *outputFormat,
		KillEffect:            *killEffect,
		Transition:            *transition,
		Intro:                 *intro,
		Outro:                 *outro,
		IntroText:             *introText,
		OutroText:             *outroText,
		HookText:              *hook,
		KillCounter:           *killCounter,
		KillfeedOverlay:       *killfeedOverlay,
		KeyDropFamily:         *keyDropFamily,
		KeyDropStyle:          *keyDropStyle,
		KeyDropCode:           *keyDropCode,
		KeyDropPositionY:      keyDropPosition,
		KeyDropStartSeconds:   keyDropStartSec,
		KeyDropEndSeconds:     keyDropEndSec,
		TailTrimSeconds:       *tailTrim,
		OutputFPS:             *outputFPS,
		CompileSegments:       *compileSegments,
		LineupCatalogPath:     *lineupCatalogPath,
		SegmentIDs:            segmentIDs,
		Limit:                 *limit,
		RankMoments:           *rankMoments,
		VideoCRF:              *videoCRF,
		VideoPreset:           *videoPreset,
		VideoEncoder:          *videoEncoder,
		Threads:               *threads,
		HQFilters:             *hqFilters,
		AudioNormalize:        *audioNormalize,
		QualityChecks:         *qualityChecks,
		CoverSheets:           *coverSheets,
		CoverSheetsSet:        coverSheetsSet,
		CoverFirstFrame:       *coverFirstFrame,
		TemporalSmoothing:     *temporalSmoothing,
		FFmpegPath:            *ffmpegPath,
		FFprobePath:           *ffprobePath,
		DisableCovers:         !*covers || *noCovers,
		SkipExisting:          *skipExisting,
		RenderJobs:            *renderJobs,
		DryRun:                *dryRun,
		FullDemoOverlayPath:   *fullDemoOverlay,
		FullDemoExecutionPath: *fullDemoExecution,
		OverlayAssetsDir:      *overlayAssets,
		ProgressOutPath:       *progressOut,
	}
	return editorArgs{config: config, format: *format, openGallery: *openGallery}, nil
}

// editorSummary is the {ok, dry_run, executed} success envelope emitted on
// stdout, mirroring the record and compose-final stages. The durable
// shorts-result.json artifact keeps its own richer schema.
type editorSummary struct {
	OK         bool     `json:"ok"`
	DryRun     bool     `json:"dry_run"`
	Executed   bool     `json:"executed"`
	ResultPath string   `json:"result_path"`
	PublishDir string   `json:"publish_dir"`
	ShortCount int      `json:"short_count"`
	Warnings   []string `json:"warnings"`
}

func writeEditorSummary(w io.Writer, format string, result editor.Result) error {
	summary := editorSummary{
		OK:         true,
		DryRun:     result.DryRun,
		Executed:   result.Executed,
		ResultPath: filepath.Join(result.OutputDir, "shorts-result.json"),
		PublishDir: result.PublishDir,
		ShortCount: len(result.Shorts),
		Warnings:   append([]string{}, result.Warnings...),
	}
	if format == "json" {
		encoder := json.NewEncoder(w)
		encoder.SetEscapeHTML(false)
		encoder.SetIndent("", "  ")
		return encoder.Encode(summary)
	}
	fmt.Fprintf(w, "shorts_result\t%s\n", summary.ResultPath)
	fmt.Fprintf(w, "publish_dir\t%s\n", summary.PublishDir)
	fmt.Fprintf(w, "shorts\t%d\n", summary.ShortCount)
	fmt.Fprintf(w, "dry_run\t%t\n", summary.DryRun)
	return nil
}

// validateMusicVolume rejects a music gain outside (0,1]. The flag defaults to
// 1.0, so a valid render always keeps the historical mix unless overridden.
func validateMusicVolume(v float64) error {
	if v <= 0 || v > 1 {
		return fmt.Errorf("--music-volume must be greater than 0 and at most 1, got %v", v)
	}
	return nil
}

// validateOptionalMixVolume accepts the unset sentinel (<0) or a gain in [0,1].
func validateOptionalMixVolume(name string, v float64) error {
	if v < 0 {
		return nil
	}
	if v > 1 {
		return fmt.Errorf("--%s must be between 0 and 1, got %v", name, v)
	}
	return nil
}

// validateThreads rejects a negative encoder thread count. 0 means unset and
// lets FFmpeg pick its own default, so any explicit count must be >= 1.
func validateThreads(v int) error {
	if v < 0 {
		return fmt.Errorf("--threads must be >= 0")
	}
	return nil
}

func optionalMixVolume(v float64) *float64 {
	if v < 0 {
		return nil
	}
	return &v
}

func parseSegments(raw string) ([]string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}
	seen := map[string]bool{}
	var out []string
	for _, part := range strings.Split(raw, ",") {
		id := strings.TrimSpace(part)
		if id == "" {
			continue
		}
		if seen[id] {
			continue
		}
		seen[id] = true
		out = append(out, id)
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("--segments did not contain any segment ids")
	}
	return out, nil
}

func openPath(path string) error {
	if strings.TrimSpace(path) == "" {
		return fmt.Errorf("gallery path is empty")
	}
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		// #nosec G204 -- opens the generated local gallery path with the OS handler.
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", path)
	case "darwin":
		// #nosec G204 -- opens the generated local gallery path with the OS handler.
		cmd = exec.Command("open", path)
	default:
		// #nosec G204 -- opens the generated local gallery path with the OS handler.
		cmd = exec.Command("xdg-open", path)
	}
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("open gallery: %w", err)
	}
	return nil
}
