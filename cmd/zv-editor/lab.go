package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/rechedev9/cliphub/internal/editor"
)

// labBundleFile is written by the orchestrator next to the render inputs it
// materializes: the exact editor arguments a render of that job would run.
const labBundleFile = "editor-args.json"

type labBundle struct {
	SchemaVersion string            `json:"schema_version"`
	Args          []string          `json:"args"`
	Env           map[string]string `json:"env"`
}

const labUsage = `usage: zv-editor lab <mode> --bundle <dir> [options]

Runs one stage of a Full Demo render from a render input bundle
(zv full-demo lab-bundle) without rendering the whole program.

modes:
  plan       timeline, overlays, transitions and loudness targets; no FFmpeg
  commands   every FFmpeg command of the program, built but not run
  item       render one timeline item (--index, --seconds) and measure it
  audio      program audio and the real master loop over a black video
  delivery   strict delivery check and measurements of --file
`

func runLab(args []string) error {
	if len(args) == 0 || !slices.Contains(editor.LabModes(), args[0]) {
		fmt.Fprint(os.Stderr, labUsage)
		if len(args) == 0 {
			return fmt.Errorf("lab mode is required")
		}
		return fmt.Errorf("unknown lab mode %q", args[0])
	}
	mode := args[0]
	fs := flag.NewFlagSet("zv-editor lab "+mode, flag.ExitOnError)
	bundleDir := fs.String("bundle", "", "render input bundle directory containing "+labBundleFile)
	workDir := fs.String("work-dir", "", "directory kept for lab media, logs and lab-evidence.json; defaults to <bundle>/lab-work/<mode>-<time>")
	index := fs.Int("index", 0, "item mode: timeline item index, as listed by the plan mode")
	seconds := fs.Float64("seconds", 8, "item mode: render only this many seconds from the item start; 0 renders the whole item")
	file := fs.String("file", "", "delivery mode: delivered MP4 to verify")
	format := fs.String("format", "text", "summary format on stdout: text or json")
	if err := fs.Parse(args[1:]); err != nil {
		return err
	}
	if *bundleDir == "" {
		return fmt.Errorf("--bundle is required")
	}
	if *format != "text" && *format != "json" {
		return fmt.Errorf("unsupported format %q", *format)
	}
	if mode == editor.LabModeDelivery && *file == "" {
		return fmt.Errorf("delivery mode needs --file with the delivered MP4")
	}
	bundle, err := readLabBundle(*bundleDir)
	if err != nil {
		return err
	}
	// The editor reads a few settings from the environment Studio gives the
	// orchestrator (the overlay renderer); the bundle recorded them.
	for name, value := range bundle.Env {
		if os.Getenv(name) == "" {
			if err := os.Setenv(name, value); err != nil {
				return err
			}
		}
	}
	parsed, err := parseEditorArgs(bundle.Args, flag.ContinueOnError)
	if err != nil {
		return fmt.Errorf("replay %s: %w", labBundleFile, err)
	}
	dir := *workDir
	if dir == "" {
		dir = filepath.Join(*bundleDir, "lab-work", mode+"-"+time.Now().Format("20060102-150405"))
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	evidence, labErr := editor.Lab(ctx, parsed.config, editor.LabOptions{Mode: mode, WorkDir: dir, Index: *index, Seconds: *seconds, File: *file})
	evidencePath := filepath.Join(dir, "lab-evidence.json")
	if err := writeLabEvidence(evidencePath, evidence); err != nil {
		return err
	}
	if *format == "json" {
		encoder := json.NewEncoder(os.Stdout)
		encoder.SetEscapeHTML(false)
		encoder.SetIndent("", "  ")
		if err := encoder.Encode(evidence); err != nil {
			return err
		}
	} else {
		writeLabSummary(os.Stdout, evidence)
		fmt.Fprintf(os.Stdout, "evidence\t%s\n", evidencePath)
	}
	return labErr
}

func readLabBundle(dir string) (labBundle, error) {
	path := filepath.Join(dir, labBundleFile)
	// #nosec G304 -- the bundle directory is an explicit local CLI input.
	body, err := os.ReadFile(path)
	if err != nil {
		return labBundle{}, fmt.Errorf("read render input bundle: %w", err)
	}
	var bundle labBundle
	if err := json.Unmarshal(body, &bundle); err != nil {
		return labBundle{}, fmt.Errorf("decode %s: %w", path, err)
	}
	if len(bundle.Args) == 0 {
		return labBundle{}, fmt.Errorf("%s has no editor arguments", path)
	}
	return bundle, nil
}

func writeLabEvidence(path string, evidence editor.LabEvidence) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return err
	}
	body, err := json.MarshalIndent(evidence, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(body, '\n'), 0o600)
}

func writeLabSummary(w io.Writer, e editor.LabEvidence) {
	fmt.Fprintf(w, "mode\t%s\nelapsed\t%.1fs\n", e.Mode, float64(e.ElapsedMS)/1000)
	if e.Error != "" {
		fmt.Fprintf(w, "error\t%s\n", e.Error)
	}
	for _, pad := range e.TailPads {
		fmt.Fprintf(w, "capture_tail_pad\t%+v\n", pad)
	}
	switch {
	case e.Plan != nil:
		p := e.Plan
		fmt.Fprintf(w, "program\t%d frames, %s\n", p.Frames, clock(p.DurationSeconds))
		fmt.Fprintf(w, "hud\t%s\n", valueOr(p.HUDTheme, "off"))
		fmt.Fprintf(w, "loudness\tI %.1f LUFS, TP %.1f dBTP, LRA %.1f (first master %s)\n", p.Loudness.Target.TargetILUFS, p.Loudness.Target.TargetTPDBTP, p.Loudness.Target.TargetLRA, p.Loudness.Filter)
		fmt.Fprintf(w, "silent_approved\t%t\n", p.SilentApproved)
		fmt.Fprintln(w, "items\tindex role start duration source")
		for _, item := range p.Items {
			fmt.Fprintf(w, "\t%3d %-9s %s %6.2fs %s\n", item.Index, item.Role, clock(item.StartSeconds), item.Seconds, item.SourceRef)
		}
		fmt.Fprintln(w, "overlays\ttype start end")
		for _, overlay := range p.Overlays {
			fmt.Fprintf(w, "\t%-18s %s %s\n", overlay.Type, clock(overlay.StartSeconds), clock(overlay.EndSeconds))
		}
		fmt.Fprintf(w, "transitions\t%d\n", len(p.Transitions))
	case e.Commands != nil:
		c := e.Commands
		fmt.Fprintf(w, "item_commands\t%d video, %d audio\n", len(c.ItemVideo), len(c.ItemAudio))
		fmt.Fprintf(w, "program_video\t%s\n", strings.Join(c.ProgramVideo, " "))
		fmt.Fprintf(w, "program_audio\t%s\n", strings.Join(c.ProgramAudio, " "))
		fmt.Fprintf(w, "first_master\t%s\n", strings.Join(c.FirstMaster, " "))
		fmt.Fprintf(w, "final_mux\t%s\n", strings.Join(c.FinalMux, " "))
	case e.Item != nil:
		i := e.Item
		fmt.Fprintf(w, "item\t%d (%s), excerpt %t, rendered in %.1fs\n", i.Index, i.Role, i.Excerpt, float64(i.RenderMS)/1000)
		writeLabMedia(w, i.Media)
	case e.Audio != nil:
		a := e.Audio
		fmt.Fprintf(w, "status\t%s\n", a.Loudness.Status)
		fmt.Fprintf(w, "input\t%s\n", level(a.Loudness.Input))
		for n, target := range a.Loudness.MasterTargets {
			decoded := "not decoded"
			if n < len(a.Loudness.DecodedAAC) {
				decoded = level(a.Loudness.DecodedAAC[n])
			}
			fmt.Fprintf(w, "master_%d\ttarget I %.2f TP %.2f -> %s\n", n+1, target.TargetILUFS, target.TargetTPDBTP, decoded)
		}
		fmt.Fprintf(w, "recovery_masters\t%d\n", len(a.Loudness.FallbackMasters))
		if a.Loudness.FinalMuxedAAC != nil {
			fmt.Fprintf(w, "final_muxed\t%s\n", level(*a.Loudness.FinalMuxedAAC))
		}
		fmt.Fprintf(w, "output\t%s\nlogs\t%s\n", a.Output, a.LogDir)
	case e.Delivery != nil:
		d := e.Delivery
		if d.Failure != "" {
			fmt.Fprintf(w, "strict\tFAILED: %s\n", d.Failure)
		} else if d.Strict != nil {
			fmt.Fprintf(w, "strict\tok, %d frames, %.3fs\n", d.Strict.FrameCount, d.Strict.DurationSeconds)
		}
		writeLabMedia(w, d.Media)
	}
	for _, warning := range e.Warnings {
		fmt.Fprintf(w, "warning\t%s\n", warning)
	}
}

func writeLabMedia(w io.Writer, m editor.LabMediaEvidence) {
	fmt.Fprintf(w, "file\t%s\n", m.Path)
	fmt.Fprintf(w, "video\t%s %dx%d @ %s, %.0f kb/s\n", m.VideoCodec, m.Width, m.Height, m.FrameRate, m.BitrateKbps)
	fmt.Fprintf(w, "frames\t%d (expected %d, match %t)\n", m.Frames, m.ExpectedFrames, m.FramesMatch)
	fmt.Fprintf(w, "luma\tmean %.1f, min %.1f, black %.2fs in %d spans\n", m.MeanLuma, m.MinLuma, m.BlackSeconds, len(m.BlackIntervals))
	if m.IntegratedLUFS != nil && m.TruePeakDBTP != nil {
		fmt.Fprintf(w, "audio\t%s, I %.1f LUFS, TP %.1f dBTP\n", m.AudioCodec, *m.IntegratedLUFS, *m.TruePeakDBTP)
	} else if m.AudioCodec != "" {
		fmt.Fprintf(w, "audio\t%s, silent\n", m.AudioCodec)
	}
	for _, still := range m.Stills {
		fmt.Fprintf(w, "still\t%s\n", still)
	}
}

func level(m editor.LoudnessMeasurement) string {
	if m.IntegratedLUFS == nil || m.TruePeakDBTP == nil {
		return m.Status
	}
	return fmt.Sprintf("I %.2f LUFS, TP %.2f dBTP", *m.IntegratedLUFS, *m.TruePeakDBTP)
}

func clock(seconds float64) string {
	return fmt.Sprintf("%02d:%06.3f", int(seconds)/60, seconds-float64(int(seconds)/60*60))
}

func valueOr(value, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}
