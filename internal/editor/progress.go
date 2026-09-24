package editor

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"regexp"
	"strconv"
	"sync"
	"time"

	"github.com/rechedev9/cliphub/internal/obs"
)

const EditorProgressSchema = "editor-progress/1"

// EditorProgress is a throttled status document written to --progress-out.
type EditorProgress struct {
	Schema  string `json:"schema"`
	Stage   string `json:"stage"`
	Percent int    `json:"percent"`
}

func (p EditorProgress) Validate() error {
	if p.Schema != EditorProgressSchema {
		return fmt.Errorf("unsupported editor progress schema %q", p.Schema)
	}
	if p.Percent < 0 || p.Percent > 100 {
		return fmt.Errorf("editor progress percent %d is out of range", p.Percent)
	}
	if p.Stage == "" {
		return fmt.Errorf("editor progress stage is required")
	}
	return nil
}

const (
	progressPrepPercent   = 5
	progressEncodeStart   = 5
	progressEncodeSpan    = 87 // 5..92
	progressFinalizeStart = 92
	progressFinalizeEnd   = 100
	progressWriteInterval = time.Second
)

// ProgressTracker writes monotonic editor progress to a JSON file.
type ProgressTracker struct {
	path        string
	mu          sync.Mutex
	lastPercent int
	lastStage   string
	lastWrite   time.Time
	now         func() time.Time
}

func NewProgressTracker(path string) *ProgressTracker {
	if path == "" {
		return nil
	}
	return &ProgressTracker{path: path, now: time.Now}
}

func (t *ProgressTracker) Set(stage string, percent int) {
	if t == nil {
		return
	}
	if percent < 0 {
		percent = 0
	}
	if percent > 100 {
		percent = 100
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	if percent < t.lastPercent {
		percent = t.lastPercent
	}
	t.lastPercent = percent
	now := t.now()
	if stage == t.lastStage && !t.lastWrite.IsZero() && now.Sub(t.lastWrite) < progressWriteInterval && percent < 100 {
		return
	}
	t.lastWrite = now
	t.lastStage = stage
	_ = t.writeLocked(stage, percent)
}

func (t *ProgressTracker) Flush(stage string, percent int) {
	if t == nil {
		return
	}
	if percent < 0 {
		percent = 0
	}
	if percent > 100 {
		percent = 100
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	if percent < t.lastPercent {
		percent = t.lastPercent
	}
	t.lastPercent = percent
	t.lastWrite = t.now()
	t.lastStage = stage
	_ = t.writeLocked(stage, percent)
}

func (t *ProgressTracker) writeLocked(stage string, percent int) error {
	body, err := json.Marshal(EditorProgress{
		Schema:  EditorProgressSchema,
		Stage:   stage,
		Percent: percent,
	})
	if err != nil {
		return err
	}
	tmp := t.path + ".tmp"
	if err := os.WriteFile(tmp, body, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, t.path)
}

type encodeProgressSlot struct {
	startPct int
	endPct   int
	duration float64
}

type encodeProgressPlan struct {
	slots []encodeProgressSlot
}

func buildEncodeProgressPlan(shorts []ShortEdit) encodeProgressPlan {
	if len(shorts) == 0 {
		return encodeProgressPlan{}
	}
	weights := make([]float64, len(shorts))
	total := 0.0
	for i, short := range shorts {
		w := expectedShortDuration(short)
		if w <= 0 {
			w = 1
		}
		weights[i] = w
		total += w
	}
	plan := encodeProgressPlan{slots: make([]encodeProgressSlot, len(shorts))}
	recorded := 0.0
	for i, w := range weights {
		startFrac := recorded / total
		recorded += w
		endFrac := recorded / total
		plan.slots[i] = encodeProgressSlot{
			startPct: progressEncodeStart + int(math.Round(startFrac*float64(progressEncodeSpan))),
			endPct:   progressEncodeStart + int(math.Round(endFrac*float64(progressEncodeSpan))),
			duration: expectedShortDuration(shorts[i]),
		}
		if plan.slots[i].endPct < plan.slots[i].startPct {
			plan.slots[i].endPct = plan.slots[i].startPct
		}
	}
	return plan
}

func expectedShortDuration(short ShortEdit) float64 {
	if short.DurationSeconds > 0 {
		return short.DurationSeconds
	}
	var total float64
	tickrate := short.Tickrate
	if tickrate <= 0 {
		tickrate = short.VoiceTickrate
	}
	for _, part := range short.Parts {
		if part.DurationSeconds > 0 {
			total += part.DurationSeconds
			continue
		}
		if d := partSyncDuration(part, tickrate); d > 0 {
			total += d
			continue
		}
		total += compilationPartDuration(short, part)
	}
	return total
}

type encodeProgressState struct {
	mu       sync.Mutex
	plan     encodeProgressPlan
	done     []bool
	fraction []float64
	tracker  *ProgressTracker
	stage    string
}

func newEncodeProgressState(plan encodeProgressPlan, tracker *ProgressTracker, stage string) *encodeProgressState {
	if tracker == nil || len(plan.slots) == 0 {
		return nil
	}
	return &encodeProgressState{
		plan:     plan,
		done:     make([]bool, len(plan.slots)),
		fraction: make([]float64, len(plan.slots)),
		tracker:  tracker,
		stage:    stage,
	}
}

func (s *encodeProgressState) setFraction(index int, fraction float64) {
	if s != nil {
		s.setStageFraction(index, s.stage, fraction)
	}
}

func (s *encodeProgressState) setStageFraction(index int, stage string, fraction float64) {
	if s == nil || index < 0 || index >= len(s.plan.slots) {
		return
	}
	if fraction < 0 {
		fraction = 0
	}
	if fraction > 0.99 {
		fraction = 0.99
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.done[index] {
		return
	}
	s.fraction[index] = fraction
	s.tracker.Set(stage, s.percentLocked())
}

func (s *encodeProgressState) markDone(index int) {
	if s == nil || index < 0 || index >= len(s.plan.slots) {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.done[index] = true
	s.fraction[index] = 1
	s.tracker.Flush(s.stage, s.percentLocked())
}

func (s *encodeProgressState) percentLocked() int {
	if len(s.plan.slots) == 0 {
		return progressPrepPercent
	}
	best := progressEncodeStart
	for i, slot := range s.plan.slots {
		span := slot.endPct - slot.startPct
		if span < 0 {
			span = 0
		}
		frac := s.fraction[i]
		if s.done[i] {
			frac = 1
		}
		candidate := slot.startPct + int(math.Round(float64(span)*frac))
		if candidate > best {
			best = candidate
		}
	}
	if best > progressFinalizeStart {
		return progressFinalizeStart
	}
	return best
}

// Full Demo stage breadcrumbs. A failed render shows only its UI percentage,
// which support used to map to a pipeline stage by hand (76-82 % master loop,
// 83-86 % AAC recovery, 87 %+ delivery). stage.entered records name the
// weighted stage instead. They derive from the timing annotation every FFmpeg
// run of a Full Demo render already carries: the first run of a stage attempt
// emits one record, so concurrent branches (item encodes next to the audio
// master, the speculative AAC recovery next to the native masters) cannot make
// the breadcrumbs flap. audio_master records carry the loudnorm TP target of
// that attempt, read from its command.
const stageEnteredTraceEvent = "stage.entered"

type fullDemoStageBreadcrumbsKey struct{}

type fullDemoStageBreadcrumbs struct {
	mu      sync.Mutex
	entered map[string]bool
}

// withFullDemoStageBreadcrumbs attaches the breadcrumb state for one Full Demo
// render; without it enterFullDemoStage does nothing.
func withFullDemoStageBreadcrumbs(ctx context.Context) context.Context {
	return context.WithValue(ctx, fullDemoStageBreadcrumbsKey{}, &fullDemoStageBreadcrumbs{entered: map[string]bool{}})
}

// loudnormTargetTPPattern matches the TP option of a loudnorm filter, not its
// measured_TP input.
var loudnormTargetTPPattern = regexp.MustCompile(`loudnorm=(?:[^,;\[\]\s]*:)?TP=(-?[0-9]+(?:\.[0-9]+)?)`)

// fullDemoBreadcrumbStage maps a timing stage to the weighted progress stage.
// Stages without a weighted counterpart emit nothing.
func fullDemoBreadcrumbStage(stage, variant string, attempt int) string {
	switch stage {
	case "voice_analysis", "voice_prepare":
		return "voice_prepare"
	case "transitions":
		return "transitions"
	case "items":
		return "video_items"
	case "items_audio":
		return "audio_items"
	case "assembly":
		return "video_assembly"
	case "audio_assembly":
		return "audio_assembly"
	case "audio_input_analysis":
		if attempt < 0 {
			return "audio_analysis"
		}
		return "audio_master"
	case "audio_candidate_encode", "audio_candidate_analysis":
		if variant == "recovery" {
			return "aac_recovery"
		}
		return "audio_master"
	case "final_mux", "final_audio_analysis":
		return "audio_publish"
	case "delivery":
		return "delivery_verify"
	default:
		return ""
	}
}

// enterFullDemoStage is called before every FFmpeg run with its original
// command. Attempts are 1-based like the UI's "(n/3)"; stages that are not
// attempt-specific report attempt=1.
func enterFullDemoStage(ctx context.Context, command []string) {
	crumbs, _ := ctx.Value(fullDemoStageBreadcrumbsKey{}).(*fullDemoStageBreadcrumbs)
	scope := fullDemoTimingScopeFrom(ctx)
	if crumbs == nil || scope == nil {
		return
	}
	stage := fullDemoBreadcrumbStage(scope.stage, scope.variant, scope.attempt)
	if stage == "" {
		return
	}
	attempt := max(scope.attempt, 0) + 1
	message := fmt.Sprintf("stage=%s attempt=%d", stage, attempt)
	crumbs.mu.Lock()
	entered := crumbs.entered[message]
	crumbs.entered[message] = true
	crumbs.mu.Unlock()
	if entered {
		return
	}
	if stage == "audio_master" {
		if tp, ok := loudnormTargetTP(command); ok {
			message += " target_tp=" + tp
		}
	}
	obs.EmitTrace(ctx, obs.TraceEntry{Event: stageEnteredTraceEvent, Level: "info", Message: message})
}

func loudnormTargetTP(command []string) (string, bool) {
	for _, arg := range command {
		match := loudnormTargetTPPattern.FindStringSubmatch(arg)
		if match == nil {
			continue
		}
		value, err := strconv.ParseFloat(match[1], 64)
		if err != nil {
			return "", false
		}
		return strconv.FormatFloat(value, 'f', -1, 64), true
	}
	return "", false
}
