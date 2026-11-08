package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/rechedev9/cliphub/internal/capturetools"
	"github.com/rechedev9/cliphub/internal/customhud"
	"github.com/rechedev9/cliphub/internal/killplan"
	"github.com/rechedev9/cliphub/internal/mediafont"
	"github.com/rechedev9/cliphub/internal/recording"
)

// hudFPS is the frame rate the custom HUD clock assumes, and the one the recorder captures at.
const hudFPS = 60

// recordTools are the programs record-clips runs on the operator PC.
type recordTools struct {
	recorder, hlae, cs2, ffmpeg string
	// fake writes placeholder captures, to prove the chain without CS2.
	fake bool
}

type recordClipsJob struct {
	plan, clips, work string
	tools             recordTools
}

func runRecordClips(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("record-clips", flag.ContinueOnError)
	plan := fs.String("plan", "", "plan file from plan-clips")
	clips := fs.String("clips", "", "folder the finished clips go to")
	work := fs.String("work", filepath.Join(os.TempDir(), "cliphub-clips"), "scratch folder for captures")
	recorder := fs.String("recorder", "zv-recorder", "zv-recorder executable")
	ffmpeg := fs.String("ffmpeg", "", "ffmpeg executable; empty uses PATH")
	fake := fs.Bool("fake", false, "placeholder captures instead of CS2, to test the chain")
	detected, _ := capturetools.Detect(capturetools.Paths{})
	hlae := fs.String("hlae", detected.HLAE, "HLAE.exe")
	cs2 := fs.String("cs2", detected.CS2, "cs2.exe")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *plan == "" || *clips == "" || (!*fake && (*hlae == "" || *cs2 == "")) {
		return errors.New("record-clips needs -plan and -clips, and HLAE and CS2 unless -fake")
	}
	return recordClips(ctx, recordClipsJob{plan: *plan, clips: *clips, work: *work,
		tools: recordTools{recorder: *recorder, hlae: *hlae, cs2: *cs2, ffmpeg: *ffmpeg, fake: *fake}})
}

// recordClips records every planned throw, burns the Focus inputs HUD and stores the clip.
func recordClips(ctx context.Context, job recordClipsJob) error {
	body, err := os.ReadFile(job.plan)
	if err != nil {
		return err
	}
	var plan clipPlan
	if err := json.Unmarshal(body, &plan); err != nil {
		return fmt.Errorf("read clip plan: %w", err)
	}
	failed := 0
	for i, d := range plan.Demos {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		n, err := recordDemoClips(ctx, job, d, filepath.Join(job.work, fmt.Sprintf("demo-%03d", i+1)))
		if err != nil {
			failed++
			log.Printf("community stage=clips class=failed demo=%s error=%v", filepath.Base(d.Demo), err)
			continue
		}
		log.Printf("community stage=clips class=recorded demo=%s clips=%d", filepath.Base(d.Demo), n)
	}
	joined := 0
	for _, c := range plan.Combos {
		ok, err := joinCombo(ctx, job, c)
		if err != nil {
			log.Printf("community stage=clips class=combo_failed combo=%s error=%v", c.Name, err)
			continue
		}
		if ok {
			joined++
		}
	}
	log.Printf("community stage=clips class=combos joined=%d planned=%d", joined, len(plan.Combos))
	if failed > 0 {
		return fmt.Errorf("%d of %d demos failed", failed, len(plan.Demos))
	}
	return nil
}

// joinCombo writes a combo clip once every throw clip of it is recorded; false means some are still missing.
func joinCombo(ctx context.Context, job recordClipsJob, c clipCombo) (bool, error) {
	dst := filepath.Join(job.clips, filepath.FromSlash(c.Name))
	if _, err := os.Stat(dst); err == nil {
		return false, nil
	}
	parts := make([]string, 0, len(c.Parts))
	for _, name := range c.Parts {
		// The concat demuxer reads relative entries against the list's own folder.
		path, err := filepath.Abs(filepath.Join(job.clips, filepath.FromSlash(name)))
		if err != nil {
			return false, err
		}
		if _, err := os.Stat(path); err != nil {
			return false, nil
		}
		parts = append(parts, path)
	}
	list := dst + ".txt"
	if err := os.WriteFile(list, []byte(concatList(parts)), 0o644); err != nil {
		return false, err
	}
	defer os.Remove(list)
	tmp := dst + ".tmp.mp4"
	ffmpeg := job.tools.ffmpeg
	if ffmpeg == "" {
		ffmpeg = "ffmpeg"
	}
	if err := runTool(ctx, ffmpeg, "-hide_banner", "-loglevel", "error", "-y", "-f", "concat", "-safe", "0", "-i", list, "-c", "copy", "-movflags", "+faststart", tmp); err != nil {
		_ = os.Remove(tmp)
		return false, err
	}
	return true, os.Rename(tmp, dst)
}

// concatList is an ffmpeg concat list; the clips share codec and size, so they join without a re-encode.
func concatList(paths []string) string {
	var b strings.Builder
	for _, p := range paths {
		fmt.Fprintf(&b, "file '%s'\n", strings.ReplaceAll(filepath.ToSlash(p), "'", `'\''`))
	}
	return b.String()
}

func recordDemoClips(ctx context.Context, job recordClipsJob, d clipDemo, dir string) (int, error) {
	if len(d.Clips) == 0 {
		return 0, nil
	}
	d, segments, err := captureDropping(ctx, job, d, dir)
	if err != nil {
		return 0, err
	}
	// The capture can rewrite the demo in place (it leaves a .bak), so the HUD digest is taken again.
	sum, err := fileSHA256(d.Demo)
	if err != nil {
		return 0, err
	}
	fonts, err := mediafont.MaterializeHUD()
	if err != nil {
		return 0, err
	}
	renderer, err := customhud.NewRenderer("focus")
	if err != nil {
		return 0, err
	}
	timelines := map[string]customhud.Timeline{}
	done := 0
	for i, c := range d.Clips {
		raw, ok := segments[clipSegmentID(i)]
		if !ok {
			log.Printf("community stage=clips class=missing clip=%s", c.Name)
			continue
		}
		tl, ok := timelines[c.SteamID64]
		if !ok {
			if tl, err = extractTimeline(ctx, d, sum, c.SteamID64); err != nil {
				return done, err
			}
			timelines[c.SteamID64] = tl
		}
		frames := clipFrames(c, d.TickRate)
		ass, err := renderer.ASS(tl, customhud.Window{StartTick: c.StartTick, Frames: int64(frames)})
		if err != nil {
			return done, fmt.Errorf("%s: HUD: %w", c.Name, err)
		}
		assPath := filepath.Join(dir, clipSegmentID(i)+".ass")
		if err := os.WriteFile(assPath, []byte(ass), 0o644); err != nil {
			return done, err
		}
		if err := burnClip(ctx, job, burnInput{raw: raw, ass: assPath, fonts: fonts, name: c.Name, frames: frames}); err != nil {
			return done, fmt.Errorf("%s: %w", c.Name, err)
		}
		done++
	}
	return done, nil
}

// maxDroppedClips bounds how many failing throws one demo may drop before it gives up.
const maxDroppedClips = 3

var failedSegment = regexp.MustCompile(`during clip-(\d{3})`)

// captureDropping records the demo; when the recorder names the throw it could not film, that throw
// is marked failed (so the planner takes another example) and the rest are recorded again.
func captureDropping(ctx context.Context, job recordClipsJob, d clipDemo, dir string) (clipDemo, map[string]string, error) {
	for dropped := 0; ; dropped++ {
		sum, err := fileSHA256(d.Demo)
		if err != nil {
			return d, nil, err
		}
		if err := os.RemoveAll(dir); err != nil {
			return d, nil, err
		}
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return d, nil, err
		}
		segments, out, err := captureClips(ctx, job, d, sum, dir)
		if err == nil {
			return d, segments, nil
		}
		i := failedClip(out, len(d.Clips))
		if i < 0 || dropped == maxDroppedClips || ctx.Err() != nil {
			return d, nil, err
		}
		name := d.Clips[i].Name
		log.Printf("community stage=clips class=clip_dropped clip=%s error=%v", name, err)
		if err := markClipFailed(job.clips, name); err != nil {
			return d, nil, err
		}
		d.Clips = append(d.Clips[:i:i], d.Clips[i+1:]...)
		if len(d.Clips) == 0 {
			return d, map[string]string{}, nil
		}
	}
}

// failedClip is the index of the clip the recorder output names as failed, or -1.
func failedClip(out string, n int) int {
	m := failedSegment.FindStringSubmatch(out)
	if m == nil {
		return -1
	}
	i, err := strconv.Atoi(m[1])
	if err != nil || i < 1 || i > n {
		return -1
	}
	return i - 1
}

// markClipFailed leaves name.failed beside the clips so later plans skip that throw.
func markClipFailed(clips, name string) error {
	path := filepath.Join(clips, filepath.FromSlash(name)) + ".failed"
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, nil, 0o644)
}

// captureClips runs zv-recorder once for the demo and returns each segment's raw capture and the recorder output.
func captureClips(ctx context.Context, job recordClipsJob, d clipDemo, sum, dir string) (map[string]string, string, error) {
	body, err := json.Marshal(clipKillPlan(d, sum))
	if err != nil {
		return nil, "", err
	}
	planPath := filepath.Join(dir, "killplan.json")
	if err := os.WriteFile(planPath, body, 0o644); err != nil {
		return nil, "", err
	}
	out := filepath.Join(dir, "capture")
	args := recorderArgs(job.tools, recorderFiles{plan: planPath, demo: d.Demo, out: out})
	var output strings.Builder
	cmd := exec.CommandContext(ctx, job.tools.recorder, args...)
	cmd.Stdout = os.Stderr
	cmd.Stderr = io.MultiWriter(os.Stderr, &output)
	if err := cmd.Run(); err != nil {
		return nil, output.String(), fmt.Errorf("zv-recorder: %w", err)
	}
	segments, err := capturedSegments(out)
	return segments, output.String(), err
}

type recorderFiles struct{ plan, demo, out string }

// recorderArgs keeps the game HUD to the thrower's own crosshair (and their kills), so the aim reads in the clip.
func recorderArgs(tools recordTools, files recorderFiles) []string {
	args := []string{"--killplan", files.plan, "--demo", files.demo, "--out", files.out, "--hud", string(recording.HUDModeDeathnotices), "--fps", fmt.Sprint(hudFPS)}
	if tools.fake {
		args = append(args, "--fake")
	} else {
		args = append(args, "--hlae", tools.hlae, "--cs2", tools.cs2)
	}
	if tools.ffmpeg != "" {
		args = append(args, "--ffmpeg", tools.ffmpeg)
	}
	return args
}

// clipKillPlan films each throw from its thrower: the first one is the plan target, the rest are POVs.
func clipKillPlan(d clipDemo, sum string) killplan.Plan {
	p := killplan.Plan{SchemaVersion: "1.2", GeneratedAt: time.Now().UTC(),
		Demo:   killplan.Demo{Path: d.Demo, SHA256: sum, Map: d.Map, Tickrate: d.TickRate, DurationTicks: d.DurationTicks},
		Target: killplan.Target{SteamID64: d.Clips[0].SteamID64}}
	for i, c := range d.Clips {
		s := killplan.Segment{ID: clipSegmentID(i), Round: c.Round, TickStart: c.StartTick, TickEnd: c.EndTick}
		if c.SteamID64 != p.Target.SteamID64 {
			s.POV = &killplan.Target{SteamID64: c.SteamID64}
		}
		p.Segments = append(p.Segments, s)
	}
	p.Stats.SegmentsCreated = len(p.Segments)
	return p
}

func clipSegmentID(i int) string { return fmt.Sprintf("clip-%03d", i+1) }

// clipFrames is the clip's length in HUD frames.
func clipFrames(c clipShot, tickRate int) int {
	return int(math.Round(float64(c.EndTick-c.StartTick) * hudFPS / float64(tickRate)))
}

func capturedSegments(out string) (map[string]string, error) {
	body, err := os.ReadFile(filepath.Join(out, "recording-result.json"))
	if err != nil {
		return nil, err
	}
	var result recording.RecordingResult
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, err
	}
	clips, _, err := recording.ResolveSegmentClips(result)
	paths := map[string]string{}
	for _, c := range clips {
		path := c.Path
		if !filepath.IsAbs(path) {
			path = filepath.Join(out, path)
		}
		paths[c.SegmentID] = path
	}
	if len(paths) == 0 && err != nil {
		return nil, err
	}
	return paths, nil
}

func extractTimeline(ctx context.Context, d clipDemo, sum, steamID string) (customhud.Timeline, error) {
	f, err := os.Open(d.Demo)
	if err != nil {
		return customhud.Timeline{}, err
	}
	defer f.Close()
	tl, err := customhud.Extract(ctx, f, sum, steamID, d.TickRate)
	if err != nil {
		return customhud.Timeline{}, fmt.Errorf("inputs of %s: %w", steamID, err)
	}
	return tl, nil
}

type burnInput struct {
	raw, ass, fonts, name string
	frames                int
}

// burnClip draws the HUD on the capture and encodes a small silent 720p loop.
func burnClip(ctx context.Context, job recordClipsJob, in burnInput) error {
	dst := filepath.Join(job.clips, filepath.FromSlash(in.name))
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	tmp := dst + ".tmp.mp4"
	ffmpeg := job.tools.ffmpeg
	if ffmpeg == "" {
		ffmpeg = "ffmpeg"
	}
	filter := clipFilter(in.ass, in.fonts)
	if err := runTool(ctx, ffmpeg, "-hide_banner", "-loglevel", "error", "-y", "-i", in.raw, "-vf", filter, "-frames:v", fmt.Sprint(in.frames*clipFPS/hudFPS),
		"-an", "-c:v", "libx264", "-preset", "slow", "-crf", "30", "-movflags", "+faststart", tmp); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return os.Rename(tmp, dst)
}

// clipFPS keeps clips small enough to stream from the server; the HUD is drawn at hudFPS first.
const clipFPS = 30

// clipFilter burns the HUD and a bold aim mark on the 1080p capture, then scales it to 720p at clipFPS.
func clipFilter(ass, fonts string) string {
	return "scale=1920:1080," + aimMark() + "," + customhud.ASSFilter(ass, fonts, false) + ",scale=1280:720:flags=lanczos,fps=" + fmt.Sprint(clipFPS) + ",format=yuv420p"
}

// aimMark draws a white cross with a dark edge at the screen centre, where the in-eye view aims.
// Its centre stays open, so the pro's own crosshair still shows.
func aimMark() string {
	const cx, cy, gap, arm, w = 960, 540, 6, 16, 3
	boxes := [][4]int{
		{cx - gap - arm, cy - w/2, arm, w}, {cx + gap, cy - w/2, arm, w},
		{cx - w/2, cy - gap - arm, w, arm}, {cx - w/2, cy + gap, w, arm},
	}
	parts := make([]string, 0, 2*len(boxes))
	for _, b := range boxes {
		parts = append(parts, fmt.Sprintf("drawbox=x=%d:y=%d:w=%d:h=%d:color=black@0.8:t=fill", b[0]-2, b[1]-2, b[2]+4, b[3]+4))
	}
	for _, b := range boxes {
		parts = append(parts, fmt.Sprintf("drawbox=x=%d:y=%d:w=%d:h=%d:color=white:t=fill", b[0], b[1], b[2], b[3]))
	}
	return strings.Join(parts, ",")
}

func runTool(ctx context.Context, name string, args ...string) error {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Stdout, cmd.Stderr = os.Stderr, os.Stderr
	return cmd.Run()
}

func fileSHA256(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}
