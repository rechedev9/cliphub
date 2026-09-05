package editor

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/rechedev9/cliphub/internal/mediaassets"
	"github.com/rechedev9/cliphub/internal/pathguard"
	"github.com/rechedev9/cliphub/internal/recapplan"
	"github.com/rechedev9/cliphub/internal/recording"
	"github.com/rechedev9/cliphub/internal/storage"
)

// FullDemoExecution is an attempt-local materialization of a durable approval.
// Paths live here only; approved/effective documents contain immutable refs.
type FullDemoExecution struct {
	SchemaVersion string               `json:"schema_version"`
	Approved      recapplan.Snapshot   `json:"approved"`
	Assets        []FullDemoLocalMedia `json:"assets"`
	VoiceTracks   []FullDemoLocalVoice `json:"voice_tracks"`
}

type FullDemoLocalMedia struct {
	Ref  recapplan.AssetRef `json:"ref"`
	Path string             `json:"path"`
}

type FullDemoLocalVoice struct {
	SteamID64  string `json:"steamid64"`
	StorageKey string `json:"storage_key"`
	SHA256     string `json:"sha256"`
	Path       string `json:"path"`
}

type FullDemoMusicInterval struct {
	TimelineIndex int   `json:"timeline_index"`
	StartSample   int64 `json:"playlist_start_sample"`
	EndSample     int64 `json:"playlist_end_sample"`
}

type FullDemoTrackLevel struct {
	Ref           string              `json:"ref"`
	Role          string              `json:"role"`
	Measurement   LoudnessMeasurement `json:"measurement"`
	AppliedGainDB float64             `json:"applied_gain_db"`
	Policy        string              `json:"policy"`
}

type FullDemoRenderEvidence struct {
	Delivery        *FullDemoDeliveryEvidence `json:"delivery"`
	SchemaVersion   string                    `json:"schema_version"`
	Approved        recapplan.Snapshot        `json:"approved"`
	Effective       recapplan.Document        `json:"effective"`
	MusicIntervals  []FullDemoMusicInterval   `json:"music_intervals"`
	TrackLevels     []FullDemoTrackLevel      `json:"track_levels"`
	ProgramLoudness *ProgramLoudnessEvidence  `json:"program_loudness"`
}

type fullDemoRenderContext struct {
	execution      FullDemoExecution
	recording      recording.RecordingResult
	ffmpeg         string
	workDir        string
	playlist       string
	voicePaths     []string
	preparedInputs []string
}

func readFullDemoExecution(ctx context.Context, path, outDir, publishDir string) (*FullDemoExecution, error) {
	if path == "" {
		return nil, nil
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open Full Demo execution: %w", err)
	}
	defer f.Close()
	var execution FullDemoExecution
	decoder := json.NewDecoder(io.LimitReader(f, 8<<20))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&execution); err != nil {
		return nil, fmt.Errorf("decode Full Demo execution: %w", err)
	}
	if decoder.Decode(new(any)) != io.EOF {
		return nil, fmt.Errorf("full demo execution must contain one bounded JSON document")
	}
	if execution.SchemaVersion != "1.0" {
		return nil, fmt.Errorf("unsupported Full Demo execution version")
	}
	if err := execution.Approved.Validate(); err != nil {
		return nil, err
	}
	refs := execution.Approved.Document.Options.AssetReferences()
	if len(execution.Assets) != len(refs) || len(execution.VoiceTracks) > 64 {
		return nil, fmt.Errorf("full demo materialized input count differs")
	}
	inputs := []pathguard.Input{{Flag: "Full Demo execution", Path: path}}
	seen := map[string]bool{}
	for _, media := range execution.Assets {
		if !slices.Contains(refs, media.Ref) || seen[media.Ref.ID] {
			return nil, fmt.Errorf("full demo asset differs from its approval")
		}
		seen[media.Ref.ID] = true
		if err := verifyFullDemoLocalFile(ctx, media.Path, media.Ref.SHA256); err != nil {
			return nil, err
		}
		inputs = append(inputs, pathguard.Input{Flag: "Full Demo media", Path: media.Path})
	}
	for _, voice := range execution.VoiceTracks {
		if seen[voice.StorageKey] || voice.StorageKey == "" || voice.SteamID64 == "" {
			return nil, fmt.Errorf("invalid Full Demo voice track identity")
		}
		seen[voice.StorageKey] = true
		if err := verifyFullDemoLocalFile(ctx, voice.Path, voice.SHA256); err != nil {
			return nil, err
		}
		inputs = append(inputs, pathguard.Input{Flag: "Full Demo voice", Path: voice.Path})
	}
	voiceExpected := execution.Approved.Document.Options.Audio.Voice.Enabled && execution.Approved.Document.Voice.Availability == "available"
	if voiceExpected != (len(execution.VoiceTracks) > 0) {
		return nil, fmt.Errorf("full demo materialized voice differs from its approved availability")
	}
	for _, root := range []string{outDir, publishDir} {
		if err := pathguard.RejectInputsWithinDirectory(root, inputs...); err != nil {
			return nil, err
		}
	}
	return &execution, nil
}

func verifyFullDemoLocalFile(ctx context.Context, path, hash string) error {
	if !filepath.IsAbs(path) || len(hash) != 64 {
		return fmt.Errorf("full demo media requires an absolute local path and SHA-256")
	}
	store, err := storage.NewLocal(filepath.Dir(path))
	if err != nil {
		return err
	}
	return mediaassets.VerifyContent(ctx, store, filepath.Base(path), hash, 8<<30)
}

func (e FullDemoExecution) assetPath(ref recapplan.AssetRef) (string, error) {
	for _, media := range e.Assets {
		if media.Ref == ref {
			return media.Path, nil
		}
	}
	return "", fmt.Errorf("full_demo_asset_missing: %s", ref.ID)
}

func attachFullDemoExecution(manifest *Manifest, result recording.RecordingResult, execution *FullDemoExecution, ffmpeg string) error {
	if execution == nil {
		if result.Plan.FullDemo != nil {
			return fmt.Errorf("full demo capture requires its approved execution document")
		}
		return nil
	}
	if len(manifest.Shorts) != 1 || !isFullDemoNative(manifest.Preset, manifest.OutputFormat, manifest.CompileSegments) {
		return fmt.Errorf("full demo execution requires one existing landscape native compilation")
	}
	if err := recording.ValidateUploadResult(result); err != nil {
		return err
	}
	d := execution.Approved.Document
	if result.Plan.FullDemo == nil || result.Plan.DemoSHA256 != d.Input.DemoSHA256 || result.Plan.TargetSteamID64 != d.Input.TargetSteamID64 || result.Plan.Stream.FullDemoCapture != d.Options.Capture || !slices.Equal(result.Plan.FullDemo.Crosshairs, d.Crosshairs) {
		return fmt.Errorf("pov_contract_failed: recorded source differs from the approved Full Demo profile")
	}
	for _, round := range d.Rounds {
		covered := false
		for _, segment := range result.Plan.Segments {
			if segment.ID == round.ID && segment.TickStart <= round.RequestedStartTick && segment.TickEnd >= round.RequestedEndTick {
				covered = true
				break
			}
		}
		if !covered {
			return fmt.Errorf("recording_not_reusable: expanded round %s requires capture", round.ID)
		}
	}
	effective, err := recapplan.ApplyCertifiedEnds(execution.Approved, result.FullDemoEvidence.CertifiedEnds)
	if err != nil {
		return err
	}
	short := &manifest.Shorts[0]
	evidence := &FullDemoRenderEvidence{SchemaVersion: "1.0", Approved: execution.Approved, Effective: effective, MusicIntervals: []FullDemoMusicInterval{}, TrackLevels: []FullDemoTrackLevel{}}
	var musicCursor int64
	for i, item := range effective.Timeline {
		if item.Role == "round" {
			next := musicCursor + item.EndSample - item.StartSample
			if d.Options.Audio.Music.Enabled {
				evidence.MusicIntervals = append(evidence.MusicIntervals, FullDemoMusicInterval{TimelineIndex: i, StartSample: musicCursor, EndSample: next})
			}
			musicCursor = next
		}
	}
	short.FullDemo = evidence
	short.fullDemo = &fullDemoRenderContext{execution: *execution, recording: result, ffmpeg: ffmpeg, workDir: filepath.Join(manifest.OutputDir, "full-demo-media")}
	short.DurationSeconds = float64(effective.Timeline[len(effective.Timeline)-1].EndFrame) / recapplan.OutputFPS
	short.AudioNormalize = false
	short.VoiceTracks, short.MusicPath = nil, ""
	short.Effects = nil
	if !d.Options.Overlays.Roster {
		short.FullDemoIntroImagePath = ""
	}
	if !d.Options.Overlays.Scoreboard {
		short.FullDemoOutroImagePath = ""
	}
	if (d.Options.Overlays.Roster && short.FullDemoIntroImagePath == "") || (d.Options.Overlays.Scoreboard && short.FullDemoOutroImagePath == "") {
		return fmt.Errorf("full_demo_asset_missing: selected roster or scoreboard overlay is unavailable")
	}
	short.Effects = generatedFullDemoOverlayEffects(*short)
	if err := projectFullDemoTimeline(short, effective); err != nil {
		return err
	}
	short.Title = fmt.Sprintf("%s — %s | Full match POV", short.Player, short.Map)
	short.Headline, short.Label = short.Title, short.Title
	short.Hashtags = []string{"#CS2", "#FullDemo"}
	short.Caption = fmt.Sprintf("%s on %s. %d chronological rounds, including rounds without kills.", short.Player, short.Map, len(effective.Rounds))
	if len(execution.VoiceTracks) > 0 && d.Options.Audio.Voice.Gain > 0 {
		short.Caption += " Team communications from the demo."
	}
	if d.Options.Sponsor.Enabled {
		short.Caption += " Includes a sponsor video insert."
	}
	for _, asset := range d.Assets {
		if asset.Attribution != "" {
			short.Caption += "\n" + strings.TrimSpace(asset.Attribution)
		}
	}
	// The command is materialized from this exact effective document at render
	// time after lossless audio buses and sponsor inputs have been prepared.
	short.FFmpegCommand = buildFullDemoCompilationCommand(ffmpeg, *short)
	if short.CoverPath != "" {
		short.CoverCommand = BuildCoverFFmpegCommand(ffmpeg, *short)
	}
	if short.CoverSheetPath != "" {
		short.CoverSheetCommand = BuildCoverSheetFFmpegCommand(ffmpeg, *short)
	}
	return nil
}

// Project the canonical frame timeline into the existing Library metadata.
// Split rounds retain their source identity and never invent a kill at an ad.
func projectFullDemoTimeline(short *ShortEdit, d recapplan.Document) error {
	sources := map[string]ShortPart{}
	for _, part := range short.Parts {
		sources[part.SegmentID] = part
	}
	short.Parts, short.Kills = []ShortPart{}, []KillCue{}
	for _, item := range d.Timeline {
		if item.Role != "round" {
			continue
		}
		part, ok := sources[item.SourceRef]
		if !ok {
			return fmt.Errorf("full demo metadata lacks round input %s", item.SourceRef)
		}
		part.DurationSeconds = float64(item.EndFrame-item.StartFrame) / recapplan.OutputFPS
		part.TimelineStartSeconds = float64(item.StartFrame) / recapplan.OutputFPS
		part.GapBeforeSeconds = 0
		part.TickStart = item.SourceStartTick + int((item.SourceOffsetFrames*int64(d.Clock.TickRate)+recapplan.OutputFPS/2)/recapplan.OutputFPS)
		part.TickEnd = min(item.SourceEndTick, part.TickStart+int(((item.EndFrame-item.StartFrame)*int64(d.Clock.TickRate)+recapplan.OutputFPS/2)/recapplan.OutputFPS))
		for _, segment := range short.fullDemo.recording.Plan.Segments {
			if segment.ID == part.SegmentID {
				part.CaptureTickStart, part.CaptureTickEnd = segment.TickStart, short.fullDemo.recording.FullDemoEvidence.CertifiedEnds[segment.ID]
			}
		}
		part.Kills = []KillCue{}
		for _, round := range d.Rounds {
			if round.ID != item.SourceRef {
				continue
			}
			for _, kill := range round.Kills {
				frame, err := recapplan.TickFrames(kill.Tick-round.RequestedStartTick, d.Clock.TickRate)
				if err != nil {
					return err
				}
				if frame < item.SourceOffsetFrames || frame >= item.SourceOffsetFrames+item.EndFrame-item.StartFrame {
					continue
				}
				cue := KillCue{Tick: kill.Tick, TimeSeconds: float64(frame-item.SourceOffsetFrames) / recapplan.OutputFPS, Weapon: kill.Weapon, Victim: kill.Victim.NameInDemo, Headshot: kill.Headshot, Wallbang: kill.Wallbang}
				part.Kills = append(part.Kills, cue)
				cue.TimeSeconds += part.TimelineStartSeconds
				short.Kills = append(short.Kills, cue)
			}
		}
		short.Parts = append(short.Parts, part)
	}
	short.KillCount = len(short.Kills)
	short.CoverTimeSeconds = coverTimeSeconds(short.Kills, short.DurationSeconds)
	for _, item := range d.Timeline {
		if item.Role == "sponsor" && short.CoverTimeSeconds >= float64(item.StartFrame)/60 && short.CoverTimeSeconds < float64(item.EndFrame)/60 {
			short.CoverTimeSeconds = float64(max(int64(0), item.StartFrame-1)) / 60
		}
	}
	return nil
}
