package editor

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"sync"
	"time"
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
	progressPrepPercent    = 5
	progressEncodeStart    = 5
	progressEncodeSpan     = 87 // 5..92
	progressFinalizeStart  = 92
	progressFinalizeEnd    = 100
	progressWriteInterval  = time.Second
)

// ProgressTracker writes monotonic editor progress to a JSON file.
type ProgressTracker struct {
	path        string
	mu          sync.Mutex
	lastPercent int
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
	if !t.lastWrite.IsZero() && now.Sub(t.lastWrite) < progressWriteInterval && percent < 100 {
		return
	}
	t.lastWrite = now
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
	s.tracker.Set(s.stage, s.percentLocked())
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

func MapPassPercent(startPct, endPct int, fraction float64) int {
	if endPct <= startPct {
		return startPct
	}
	if fraction < 0 {
		fraction = 0
	}
	if fraction > 1 {
		fraction = 1
	}
	return startPct + int(math.Round(float64(endPct-startPct)*fraction))
}
