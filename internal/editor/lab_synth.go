package editor

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/rechedev9/cliphub/internal/demooverlay"
	"github.com/rechedev9/cliphub/internal/killplan"
	"github.com/rechedev9/cliphub/internal/recapplan"
	"github.com/rechedev9/cliphub/internal/recording"
	"github.com/rechedev9/cliphub/internal/voicecomms"
)

// LabModeSynth writes a synthetic render input bundle instead of running a
// stage: the approved plan of a fixture with generated media in place of the
// CS2 captures, so the lab runs without a real job.
const LabModeSynth = "synth"

// LabSynthOptions selects the plan a synthetic bundle is built from and where
// it is written.
type LabSynthOptions struct {
	// PlanPath is an approved plan snapshot ({"document", "approval"}) or a
	// bare plan document, which is approved as planned. Empty plans a built-in
	// synthetic demo of Rounds rounds (3 when 0) with the current planner.
	PlanPath string
	Rounds   int
	Dir      string
	FFmpeg   string
	// Width and Height size the generated captures; 0 keeps the capture
	// stream size (1920x1080).
	Width, Height int
}

// LabSynthBundle is the LabBundleFile of a synthetic bundle. Synthetic is
// always true: its captures are generated test patterns and its capture
// evidence is written from the plan, not read back from CS2.
type LabSynthBundle struct {
	SchemaVersion string   `json:"schema_version"`
	Synthetic     bool     `json:"synthetic"`
	SourcePlan    string   `json:"source_plan"`
	PlanHash      string   `json:"plan_hash"`
	Dir           string   `json:"dir"`
	Args          []string `json:"args"`
	// Env carries the overlay renderer the neon overlays need, as the real
	// bundle does, when this process had it.
	Env map[string]string `json:"env,omitempty"`
}

// SynthLabBundle writes a render input bundle for the plan at opts.PlanPath:
// one generated capture per planned round with the frame count a real capture
// of that round has, a Full Demo recording result that passes the render's
// validation, the approved execution, a team-voice track when the plan has
// voice, and the editor arguments a Full Demo render passes.
func SynthLabBundle(ctx context.Context, opts LabSynthOptions) (LabSynthBundle, error) {
	var snapshot recapplan.Snapshot
	var err error
	source := opts.PlanPath
	if opts.PlanPath == "" {
		rounds := opts.Rounds
		if rounds == 0 {
			rounds = labSynthDefaultRounds
		}
		snapshot, err = labSynthSnapshot(rounds)
		source = fmt.Sprintf("built-in synthetic demo (rounds: %d)", rounds)
	} else {
		snapshot, err = readLabSynthPlan(opts.PlanPath)
	}
	if err != nil {
		return LabSynthBundle{}, err
	}
	d := snapshot.Document
	if d.Options.Overlays.HUDTheme != "" {
		return LabSynthBundle{}, fmt.Errorf("synthetic bundles cannot draw the %q custom HUD: it needs the demo's HUD telemetry; use a plan with the native HUD or a real bundle", d.Options.Overlays.HUDTheme)
	}
	if refs := d.Options.AssetReferences(); len(refs) > 0 {
		return LabSynthBundle{}, fmt.Errorf("synthetic bundles cannot supply the plan's %d media assets (music, bumpers or overlay images); use a plan without them or a real bundle", len(refs))
	}
	if (d.Options.Overlays.Roster || d.Options.Overlays.Scoreboard) && demooverlay.UsesFACEITEnrichment(d.Options.OverlaySource()) {
		return LabSynthBundle{}, fmt.Errorf("synthetic bundles cannot supply the FACEIT match data a %q overlay needs; use a plan from a plain demo or a real bundle", d.Options.OverlaySource())
	}
	ffmpeg := opts.FFmpeg
	if ffmpeg == "" {
		ffmpeg = recording.FindFFmpeg()
	}
	stream := recording.DefaultStreamConfig()
	width, height := stream.Width, stream.Height
	if opts.Width > 0 || opts.Height > 0 {
		if opts.Width <= 0 || opts.Height <= 0 || opts.Width%2 != 0 || opts.Height%2 != 0 {
			return LabSynthBundle{}, fmt.Errorf("capture size %dx%d must be two positive even numbers", opts.Width, opts.Height)
		}
		width, height = opts.Width, opts.Height
	}
	dir, err := filepath.Abs(opts.Dir)
	if err != nil {
		return LabSynthBundle{}, err
	}
	if err := os.MkdirAll(filepath.Join(dir, "segments"), 0o750); err != nil {
		return LabSynthBundle{}, err
	}
	// A bundle rewritten in place is only replayable once it is complete: drop
	// the previous editor arguments first, so an interrupted run cannot leave
	// them pointing at half-regenerated captures.
	if err := os.Remove(filepath.Join(dir, LabBundleFile)); err != nil && !os.IsNotExist(err) {
		return LabSynthBundle{}, err
	}

	base := killplan.NewPlan()
	base.Demo = killplan.Demo{SHA256: d.Input.DemoSHA256, Map: labSynthMap, Tickrate: d.Clock.TickRate}
	base.Target.SteamID64 = d.Input.TargetSteamID64
	killPlan := d.KillPlan(base)
	plan, err := recording.NewPlanFromKillPlan(killPlan, filepath.Join(dir, "synthetic.dem"), dir, stream, &d)
	if err != nil {
		return LabSynthBundle{}, fmt.Errorf("recording plan from the approved plan: %w", err)
	}
	result, err := synthesizeLabCaptures(ctx, ffmpeg, dir, plan, width, height)
	if err != nil {
		return LabSynthBundle{}, err
	}

	execution := FullDemoExecution{SchemaVersion: "1.0", Approved: snapshot, Assets: []FullDemoLocalMedia{}, VoiceTracks: []FullDemoLocalVoice{}}
	if d.Options.Audio.Voice.Enabled && d.Voice.Availability == voicecomms.Available {
		voice, err := synthesizeLabVoice(ctx, ffmpeg, dir, d)
		if err != nil {
			return LabSynthBundle{}, err
		}
		execution.VoiceTracks = append(execution.VoiceTracks, voice)
	}

	recordingPath := filepath.Join(dir, "recording-result.json")
	killPlanPath := filepath.Join(dir, "killplan.json")
	executionPath := filepath.Join(dir, "full-demo-execution.json")
	for path, value := range map[string]any{recordingPath: result, killPlanPath: killPlan, executionPath: execution} {
		if err := writeLabJSON(path, value); err != nil {
			return LabSynthBundle{}, err
		}
	}
	outDir := filepath.Join(dir, "out")
	// These are the arguments RenderWorker.writeEditorInputs passes for a
	// Full Demo render of the gameplay-pov-60 variant.
	args := []string{
		"--recording-result", recordingPath,
		"--killplan", killPlanPath,
		"--out", outDir,
		"--publish-dir", filepath.Join(outDir, "shortslistosparasubir"),
		"--preset", PresetGameplayPOV60,
		"--output-format", OutputFormatLandscape16x9,
		"--kill-effect", "clean",
		"--transition", "cut",
		"--hook=false",
		"--kill-counter=false",
		"--intro=false",
		"--outro=false",
		"--cover-sheets=true",
		"--cover-first-frame=false",
		"--covers=false",
	}
	if segments := recording.EditorialSegmentIDs(result); len(segments) > 1 {
		args = append(args, "--compile-segments", "--segments", strings.Join(segments, ","))
	}
	args = append(args, "--full-demo-execution", executionPath, "--tail-trim=0", "--compile-segments")
	if d.Options.Overlays.Roster || d.Options.Overlays.Scoreboard {
		overlayPath := filepath.Join(dir, "full-demo-overlay.json")
		if err := demooverlay.Write(overlayPath, labSynthOverlay(d)); err != nil {
			return LabSynthBundle{}, err
		}
		args = append(args, "--full-demo-overlay", overlayPath)
	}
	args = append(args, "--ffmpeg", ffmpeg)
	bundle := LabSynthBundle{SchemaVersion: "1.0", Synthetic: true, SourcePlan: source, PlanHash: d.PlanHash, Dir: dir, Args: args, Env: LabBundleEnv()}
	if err := writeLabJSON(filepath.Join(dir, LabBundleFile), bundle); err != nil {
		return LabSynthBundle{}, err
	}
	return bundle, nil
}

// Synthetic demo layout: every round has a 15 s freeze, 30 s of live play with
// two kills by the target and a 5 s post-round gap, at 64 ticks per second.
const (
	labSynthTickRate      = 64
	labSynthFreezeTicks   = 15 * labSynthTickRate
	labSynthLiveTicks     = 30 * labSynthTickRate
	labSynthPostTicks     = 5 * labSynthTickRate
	labSynthTarget        = "76561198000000001"
	labSynthDefaultRounds = 3
	labSynthMap           = "de_mirage"
)

// labSynthSnapshot plans and approves a synthetic demo of the given rounds
// with the current planner and default options, minus what a synthetic bundle
// cannot supply: the custom HUD, which needs demo telemetry. The HUD profile
// is the native one.
func labSynthSnapshot(rounds int) (recapplan.Snapshot, error) {
	if rounds <= 0 || rounds > 30 {
		return recapplan.Snapshot{}, fmt.Errorf("synthetic rounds %d must be between 1 and 30", rounds)
	}
	roundTicks := labSynthFreezeTicks + labSynthLiveTicks + labSynthPostTicks
	facts := recapplan.Facts{SchemaVersion: recapplan.DocumentVersion, DemoSHA256: strings.Repeat("5", 64), TargetSteamID64: labSynthTarget, ClockKind: recapplan.ClockIngame, TickRate: labSynthTickRate, EndTick: rounds * roundTicks, Complete: true,
		Crosshairs: []recapplan.CrosshairSample{{Tick: 0, Code: "CSGO-WsnnD-eHaMw-QNDf9-oxuDh-ydOUD"}}}
	target := killplan.Player{SteamID64: labSynthTarget, NameInDemo: "synthetic-target", TeamAtKill: "CT"}
	for i := range rounds {
		start := i * roundTicks
		freezeEnd := start + labSynthFreezeTicks
		end := freezeEnd + labSynthLiveTicks
		round := recapplan.RoundFacts{ID: fmt.Sprintf("round-%03d", i+1), Number: i + 1, StartTick: start, FreezeEndTick: freezeEnd, RoundEndTick: end, Evidence: "round-events", Utility: []killplan.UtilityThrow{}}
		if i+1 < rounds {
			round.NextStartTick = start + roundTicks
		}
		for k, offset := range []int{10, 20} {
			victim := killplan.Player{SteamID64: fmt.Sprintf("7656119800000%04d", 100+i*2+k), NameInDemo: fmt.Sprintf("synthetic-victim-%d", k+1), TeamAtKill: "TERRORIST"}
			round.Kills = append(round.Kills, killplan.Kill{Tick: freezeEnd + offset*labSynthTickRate, Weapon: "ak47", Killer: target, Victim: victim})
		}
		facts.Rounds = append(facts.Rounds, round)
	}
	options := recapplan.DefaultOptions()
	options.Overlays.HUDTheme = ""
	options.Capture.HUDProfile = recapplan.NativeHUDProfile
	voice := recapplan.VoiceEvidence{Availability: "available", IndexRef: "synthetic/voice-index.json", IndexHash: strings.Repeat("6", 64), ExtractorVersion: "team-packet-clock-v2", ClockKind: recapplan.ClockIngame, SelectedPackets: rounds,
		Activity: []recapplan.TickRange{{Start: 0, End: facts.EndTick}}}
	document, err := recapplan.Plan(facts, options, voice, nil, "synthetic/facts.json")
	if err != nil {
		return recapplan.Snapshot{}, fmt.Errorf("plan the synthetic demo: %w", err)
	}
	snapshot := recapplan.Snapshot{Document: document, Approval: recapplan.Approval{PlanHash: document.PlanHash, AllowSafeTailTrim: document.Options.Editorial.AllowSafeTailTrim, Timestamp: time.Now().UTC()}}
	if err := snapshot.Validate(); err != nil {
		return recapplan.Snapshot{}, fmt.Errorf("synthetic plan is not renderable: %w", err)
	}
	return snapshot, nil
}

// labSynthOverlay is the roster the overlays show: ten synthetic players, the
// target on CT with the plan's kills, laid out the way the worker builds it
// from a demo roster scan.
func labSynthOverlay(d recapplan.Document) demooverlay.Document {
	kills := 0
	for _, round := range d.Rounds {
		kills += len(round.Kills)
	}
	roster := demooverlay.Roster{TargetSteamID64: d.Input.TargetSteamID64, Map: labSynthMap, ScoreCT: len(d.Rounds), Rounds: len(d.Rounds), ClanNameCT: "Synthetic CT", ClanNameT: "Synthetic T"}
	for i := range 10 {
		player := demooverlay.RosterPlayer{SteamID64: fmt.Sprintf("7656119800000%04d", 900+i), Name: fmt.Sprintf("synthetic-%02d", i+1), Team: "CT", Kills: i % 4, Deaths: 1 + i%3, Rounds: len(d.Rounds), ADR: 60 + float64(i*5), Rating: 0.9 + float64(i)/20}
		if i >= 5 {
			player.Team = "TERRORIST"
		}
		if i == 0 {
			player.SteamID64, player.Name, player.Kills, player.Deaths = d.Input.TargetSteamID64, "synthetic-target", kills, 0
		}
		roster.Players = append(roster.Players, player)
	}
	doc := demooverlay.BuildForSource(roster, d.Options.OverlaySource(), nil)
	doc.Theme = demooverlay.NormalizeTheme(d.Options.Overlays.Theme)
	return doc
}

// readLabSynthPlan reads an approved snapshot, or approves a bare document as
// planned, and validates it the way the render admits it.
func readLabSynthPlan(path string) (recapplan.Snapshot, error) {
	// #nosec G304 -- the plan is an explicit local CLI input.
	body, err := os.ReadFile(path)
	if err != nil {
		return recapplan.Snapshot{}, fmt.Errorf("read plan: %w", err)
	}
	var probe struct {
		Document json.RawMessage `json:"document"`
	}
	if err := json.Unmarshal(body, &probe); err != nil {
		return recapplan.Snapshot{}, fmt.Errorf("decode plan %s: %w", path, err)
	}
	var snapshot recapplan.Snapshot
	if len(probe.Document) > 0 {
		err = json.Unmarshal(body, &snapshot)
	} else {
		err = json.Unmarshal(body, &snapshot.Document)
		snapshot.Approval = recapplan.Approval{PlanHash: snapshot.Document.PlanHash, AllowSafeTailTrim: snapshot.Document.Options.Editorial.AllowSafeTailTrim, Timestamp: time.Now().UTC()}
	}
	if err != nil {
		return recapplan.Snapshot{}, fmt.Errorf("decode plan %s: %w", path, err)
	}
	if err := snapshot.Validate(); err != nil {
		return recapplan.Snapshot{}, fmt.Errorf("plan %s is not renderable by this build: %w", path, err)
	}
	return snapshot, nil
}

// synthesizeLabCaptures writes one H.264/AAC test-pattern clip per segment,
// with the frame count TickFrames gives for the segment, and the Full Demo
// recording result of a verified capture of them.
func synthesizeLabCaptures(ctx context.Context, ffmpeg, dir string, plan recording.RecordingPlan, width, height int) (recording.RecordingResult, error) {
	expected, err := recording.FullDemoExpectedCaptureCvars(plan)
	if err != nil {
		return recording.RecordingResult{}, err
	}
	evidence := &recording.FullDemoCaptureEvidence{SchemaVersion: "1.0", Restored: true, FilesRestored: true, CertifiedEnds: map[string]int{}}
	names := make([]string, 0, len(expected))
	for name := range expected {
		names = append(names, name)
	}
	slices.Sort(names)
	for _, name := range names {
		evidence.Before = append(evidence.Before, recording.CvarValue{Name: name, Value: json.RawMessage("1")})
		evidence.Applied = append(evidence.Applied, recording.CvarValue{Name: name, Value: expected[name]})
	}
	result := recording.RecordingResult{Plan: plan, Script: filepath.Join(dir, "synthetic-recording.js"), CaptureMode: recording.CaptureModeReal, CaptureVerified: true, CaptureRevision: uuid.NewString(), FullDemoEvidence: evidence}
	if err := os.WriteFile(result.Script, []byte("// synthetic render lab capture: no CS2 script ran\n"), 0o600); err != nil {
		return recording.RecordingResult{}, err
	}
	for i, segment := range plan.Segments {
		frames, err := recapplan.TickFrames(segment.TickEnd-segment.TickStart, plan.Tickrate)
		if err != nil {
			return recording.RecordingResult{}, fmt.Errorf("segment %s frames: %w", segment.ID, err)
		}
		clip := filepath.Join(dir, "segments", segment.ID+".mp4")
		seconds := strconv.FormatFloat(float64(frames)/60, 'f', 6, 64)
		// Each round gets its own hue and tone so stills and loudness tell
		// rounds apart; the pattern moves, so it is never black or frozen.
		command := []string{ffmpeg, "-y", "-v", "error",
			"-f", "lavfi", "-i", fmt.Sprintf("testsrc2=s=%dx%d:r=60,hue=h=%d", width, height, (i*47)%360),
			"-f", "lavfi", "-i", fmt.Sprintf("sine=f=%d:r=44100,volume=-18dB,pan=stereo|c0=c0|c1=c0", 220+(i%8)*55),
			"-frames:v", strconv.FormatInt(frames, 10), "-t", seconds,
			"-map", "0:v", "-map", "1:a",
			"-c:v", "libx264", "-preset", "ultrafast", "-pix_fmt", "yuv420p", "-g", "60",
			"-c:a", "aac", "-b:a", "160k", "-ar", "44100",
			"-movflags", "+faststart", clip}
		if _, err := runFFmpegOutput(ctx, command, "synthesize capture "+segment.ID); err != nil {
			return recording.RecordingResult{}, err
		}
		info, err := os.Stat(clip)
		if err != nil {
			return recording.RecordingResult{}, err
		}
		result.Artifacts = append(result.Artifacts, recording.RecordingArtifact{SegmentID: segment.ID, Role: "segment", Type: "video", Path: clip, SizeBytes: info.Size(), FrameCount: frames, FrameRate: "60/1", DurationSeconds: float64(frames) / 60})
		evidence.CertifiedEnds[segment.ID] = segment.TickEnd
	}
	result.CaptureInputFingerprint, err = recording.CaptureInputFingerprint(plan)
	if err != nil {
		return recording.RecordingResult{}, err
	}
	if err := result.DigestSegmentFiles(ctx); err != nil {
		return recording.RecordingResult{}, err
	}
	if err := recording.ValidateUploadResult(result); err != nil {
		return recording.RecordingResult{}, fmt.Errorf("synthetic recording result is not renderable: %w", err)
	}
	return result, nil
}

// synthesizeLabVoice writes a team-voice track covering the whole demo clock:
// a pulsed tone, so voice ducking and loudness have speech-like gaps.
func synthesizeLabVoice(ctx context.Context, ffmpeg, dir string, d recapplan.Document) (FullDemoLocalVoice, error) {
	endTick := 0
	for _, round := range d.Rounds {
		endTick = max(endTick, round.CaptureEndTick)
	}
	seconds := strconv.FormatFloat(float64(endTick)/float64(d.Clock.TickRate)+1, 'f', 3, 64)
	path := filepath.Join(dir, "voice-00.ogg")
	command := []string{ffmpeg, "-y", "-v", "error",
		"-f", "lavfi", "-i", "sine=f=180:r=48000:d=" + seconds + ",volume='0.3*gt(sin(2*PI*t/3),0)':eval=frame",
		"-ac", "1", "-c:a", "libopus", "-b:a", "48k", path}
	if _, err := runFFmpegOutput(ctx, command, "synthesize team voice"); err != nil {
		return FullDemoLocalVoice{}, err
	}
	hash, err := labFileSHA256(path)
	if err != nil {
		return FullDemoLocalVoice{}, err
	}
	return FullDemoLocalVoice{SteamID64: d.Input.TargetSteamID64, StorageKey: "synthetic/voice/" + d.Input.TargetSteamID64, SHA256: hash, Path: path}, nil
}

func labFileSHA256(path string) (string, error) {
	// #nosec G304 -- the path was written by this bundle.
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

func writeLabJSON(path string, value any) error {
	var body bytes.Buffer
	encoder := json.NewEncoder(&body)
	encoder.SetEscapeHTML(false)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(value); err != nil {
		return err
	}
	return os.WriteFile(path, body.Bytes(), 0o600)
}
