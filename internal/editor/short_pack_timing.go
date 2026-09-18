package editor

import (
	"context"
	"sort"
	"sync"
	"time"
)

// Shorts-pack timing evidence follows the Full Demo conventions in
// full_demo_performance.go: every interval is an offset in milliseconds from
// one collector start, wall time is the union of active intervals (gaps
// excluded) and the elapsed sum is the serial sum of span elapsed times, which
// is process/stage elapsed time and never CPU time.
//
// Unlike Full Demo, the shorts pack has no single execution point to hook: the
// encode, the quality check, the covers and the probes each run through their
// own helper. So the collector is held by the renderer and every stage records
// exactly the interval its existing RenderPerformance counter already measures.
// Nothing new is executed, no extra process is spawned and no clock is read
// that the render did not already read: a span's ElapsedMS is the same interval
// that produced RenderMS, ProbeMS, QualityCheckMS, CoverMS or CoverSheetMS.
//
// Records are content-free: a fixed code-controlled stage identifier, the short
// index, an optional fixed variant and an outcome. No paths, command arguments
// or user strings are stored.
const shortPackTimingMaxSpans = 4096

// Stage identifiers. They are fixed constants, never derived from input.
const (
	shortPackStageEncode       = "encode"
	shortPackStageProbe        = "probe"
	shortPackStagePublish      = "publish"
	shortPackStageQualityCheck = "quality_check"
	shortPackStageCover        = "cover"
	shortPackStageCoverSheet   = "cover_sheet"
)

// Probe variants distinguish which artifact a probe span measured, so the probe
// stage stays one stage without losing what was probed.
const (
	shortPackProbeOutput     = "output"
	shortPackProbeCover      = "cover"
	shortPackProbeCoverSheet = "cover-sheet"
)

// ShortPackTimingSpan is one completed stage interval of one short. StartMS and
// EndMS are offsets from the pack collector start, so work that overlapped
// across shorts (the render pool runs several shorts at once) stays visible
// without wall-clock timestamps.
type ShortPackTimingSpan struct {
	Stage     string `json:"stage"`
	Index     int    `json:"index"`
	Variant   string `json:"variant,omitempty"`
	StartMS   int64  `json:"start_ms"`
	EndMS     int64  `json:"end_ms"`
	ElapsedMS int64  `json:"elapsed_ms"`
	Outcome   string `json:"outcome"`
}

// ShortPackTimingStage aggregates every span of one stage of one short.
//
// WallMS is the union of that stage's active intervals: the elapsed time during
// which at least one span of the stage was running, excluding gaps. It is not
// the short's total render time; RenderPerformance.RenderMS remains that.
//
// ProcessElapsedSumMS is the sum of span elapsed times: the total time the
// stage would take if it ran serially. Every shorts stage except publish (a
// hardlink, or a byte copy on the cross-device fallback) is one subprocess, so
// this is process elapsed time, not CPU time and not overlap-aware wall time.
type ShortPackTimingStage struct {
	Stage               string `json:"stage"`
	Spans               int    `json:"spans"`
	WallMS              int64  `json:"wall_ms"`
	ProcessElapsedSumMS int64  `json:"process_elapsed_sum_ms"`
}

// ShortPackSlotOccupancy is how long this short held one of the bounded render
// job slots, split at the encode boundary. StartMS/EndMS are offsets from the
// pack collector start; HeldMS is the whole occupancy, and it is always >=
// EncodeMS because the encode runs inside the slot.
//
// PostEncodeMS is the part of the occupancy spent after the encode finished
// (output probe, publish, quality check, covers). It is the exact upper bound
// of what releasing the slot right after the encode could hand back to the next
// queued short; nothing else in this record is a prediction.
//
// With no encode span (a reused output) PreEncodeMS and EncodeMS are 0 and the
// whole occupancy is post-encode work.
type ShortPackSlotOccupancy struct {
	StartMS      int64 `json:"start_ms"`
	EndMS        int64 `json:"end_ms"`
	HeldMS       int64 `json:"held_ms"`
	PreEncodeMS  int64 `json:"pre_encode_ms"`
	EncodeMS     int64 `json:"encode_ms"`
	PostEncodeMS int64 `json:"post_encode_ms"`
}

// ShortPackTimingMetrics is the optional per-short timing snapshot. WallMS is
// the union of every recorded interval of this short; ProcessElapsedSumMS is
// the sum of every span's elapsed time (serialized stage time), never wall
// time.
type ShortPackTimingMetrics struct {
	Index               int                     `json:"index"`
	Spans               []ShortPackTimingSpan   `json:"spans"`
	Stages              []ShortPackTimingStage  `json:"stages"`
	WallMS              int64                   `json:"wall_ms"`
	ProcessElapsedSumMS int64                   `json:"process_elapsed_sum_ms"`
	Slot                *ShortPackSlotOccupancy `json:"slot,omitempty"`
	Truncated           bool                    `json:"truncated,omitempty"`
}

// shortPackSlotRecord is the raw occupancy of one render job slot. It is
// written twice: once when the pool granted the slot and once after the slot
// was released, both from the scheduling goroutine of that short.
type shortPackSlotRecord struct {
	start    time.Time
	end      time.Time
	acquired bool
	released bool
}

type shortPackTimingCollector struct {
	mu    sync.Mutex
	start time.Time
	spans []ShortPackTimingSpan
	slots []shortPackSlotRecord
	trunc bool
}

// newShortPackTimingCollector starts one collector for a whole pack render, so
// offsets of different shorts are comparable and pool overlap is readable.
func newShortPackTimingCollector(shorts int) *shortPackTimingCollector {
	if shorts < 0 {
		shorts = 0
	}
	return &shortPackTimingCollector{start: time.Now(), slots: make([]shortPackSlotRecord, shorts)}
}

// beginSlot records the moment the bounded pool granted a render job slot to
// this short. It is a no-op without a collector, so renderers built directly in
// tests keep working.
func (c *shortPackTimingCollector) beginSlot(index int) {
	if c == nil {
		return
	}
	now := time.Now()
	c.mu.Lock()
	defer c.mu.Unlock()
	if index < 0 || index >= len(c.slots) || c.slots[index].acquired {
		return
	}
	c.slots[index] = shortPackSlotRecord{start: now, acquired: true}
}

// finishSlot records the moment the slot was released back to the pool. The
// caller invokes it after the release so the occupancy is never understated.
func (c *shortPackTimingCollector) finishSlot(index int) {
	if c == nil {
		return
	}
	now := time.Now()
	c.mu.Lock()
	defer c.mu.Unlock()
	if index < 0 || index >= len(c.slots) || !c.slots[index].acquired || c.slots[index].released {
		return
	}
	if now.Before(c.slots[index].start) {
		now = c.slots[index].start
	}
	c.slots[index].end = now
	c.slots[index].released = true
}

// record stores one stage interval, deriving the outcome the way Full Demo
// does: a failure is "error" while a failure under a cancelled context is
// "cancelled", so an aborted pack stays distinguishable from a real failure.
func (c *shortPackTimingCollector) record(ctx context.Context, index int, stage, variant string, start, end time.Time, err error) {
	c.recordOutcome(index, stage, variant, start, end, shortPackTimingOutcome(ctx, err))
}

func (c *shortPackTimingCollector) recordOutcome(index int, stage, variant string, start, end time.Time, outcome string) {
	if c == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(c.spans) >= shortPackTimingMaxSpans {
		c.trunc = true
		return
	}
	if end.Before(start) {
		end = start
	}
	c.spans = append(c.spans, ShortPackTimingSpan{
		Stage:     stage,
		Index:     index,
		Variant:   variant,
		StartMS:   start.Sub(c.start).Milliseconds(),
		EndMS:     end.Sub(c.start).Milliseconds(),
		ElapsedMS: end.Sub(start).Milliseconds(),
		Outcome:   outcome,
	})
}

func shortPackTimingOutcome(ctx context.Context, err error) string {
	if err == nil {
		return "ok"
	}
	if ctx != nil && ctx.Err() != nil {
		return "cancelled"
	}
	return "error"
}

// snapshot builds the evidence of one short. It is called once that short's
// slot has been released and all of its stages have returned, so the spans of
// this index are complete; spans of other shorts may still be appended and are
// read under the same lock.
func (c *shortPackTimingCollector) snapshot(index int) *ShortPackTimingMetrics {
	if c == nil {
		return nil
	}
	c.mu.Lock()
	spans := make([]ShortPackTimingSpan, 0, 8)
	for _, span := range c.spans {
		if span.Index == index {
			spans = append(spans, span)
		}
	}
	var slot shortPackSlotRecord
	if index >= 0 && index < len(c.slots) {
		slot = c.slots[index]
	}
	truncated := c.trunc
	start := c.start
	c.mu.Unlock()

	if len(spans) == 0 && !slot.acquired {
		return nil
	}
	// Append order follows completion, which is not deterministic under the
	// render pool. Sort by the recorded interval and fixed fields so equal work
	// always serializes identically.
	sort.Slice(spans, func(i, j int) bool {
		a, b := spans[i], spans[j]
		if a.StartMS != b.StartMS {
			return a.StartMS < b.StartMS
		}
		if a.EndMS != b.EndMS {
			return a.EndMS < b.EndMS
		}
		if a.Stage != b.Stage {
			return a.Stage < b.Stage
		}
		return a.Variant < b.Variant
	})
	metrics := &ShortPackTimingMetrics{
		Index:     index,
		Spans:     spans,
		Stages:    shortPackTimingStages(spans),
		Truncated: truncated,
	}
	metrics.WallMS = shortPackTimingUnion(spans)
	for _, span := range spans {
		metrics.ProcessElapsedSumMS += span.ElapsedMS
	}
	metrics.Slot = shortPackSlotOccupancy(slot, start, spans)
	return metrics
}

// shortPackTimingStages groups spans in first-appearance order of the sorted
// spans, so stage ordering is deterministic too.
func shortPackTimingStages(spans []ShortPackTimingSpan) []ShortPackTimingStage {
	order := make([]string, 0, 6)
	byStage := map[string][]ShortPackTimingSpan{}
	for _, span := range spans {
		if _, seen := byStage[span.Stage]; !seen {
			order = append(order, span.Stage)
		}
		byStage[span.Stage] = append(byStage[span.Stage], span)
	}
	stages := make([]ShortPackTimingStage, 0, len(order))
	for _, stage := range order {
		group := byStage[stage]
		entry := ShortPackTimingStage{Stage: stage, Spans: len(group), WallMS: shortPackTimingUnion(group)}
		for _, span := range group {
			entry.ProcessElapsedSumMS += span.ElapsedMS
		}
		stages = append(stages, entry)
	}
	return stages
}

// shortPackTimingUnion returns the length of the merged interval set. Spans are
// already sorted by start offset.
func shortPackTimingUnion(spans []ShortPackTimingSpan) int64 {
	if len(spans) == 0 {
		return 0
	}
	var union, start, end int64
	start, end = spans[0].StartMS, spans[0].EndMS
	for _, span := range spans[1:] {
		if span.StartMS > end {
			union += end - start
			start, end = span.StartMS, span.EndMS
			continue
		}
		if span.EndMS > end {
			end = span.EndMS
		}
	}
	return union + end - start
}

// shortPackSlotOccupancy splits a released slot occupancy at the encode
// boundary. The encode boundary is the union of this short's encode spans, so a
// short that never encoded (reused output) reports its whole occupancy as
// post-encode work.
func shortPackSlotOccupancy(slot shortPackSlotRecord, collectorStart time.Time, spans []ShortPackTimingSpan) *ShortPackSlotOccupancy {
	if !slot.acquired || !slot.released {
		return nil
	}
	occupancy := &ShortPackSlotOccupancy{
		StartMS: slot.start.Sub(collectorStart).Milliseconds(),
		EndMS:   slot.end.Sub(collectorStart).Milliseconds(),
		HeldMS:  slot.end.Sub(slot.start).Milliseconds(),
	}
	encodeStart, encodeEnd, encoded := int64(0), int64(0), false
	for _, span := range spans {
		if span.Stage != shortPackStageEncode {
			continue
		}
		if !encoded {
			encodeStart, encodeEnd, encoded = span.StartMS, span.EndMS, true
			continue
		}
		if span.StartMS < encodeStart {
			encodeStart = span.StartMS
		}
		if span.EndMS > encodeEnd {
			encodeEnd = span.EndMS
		}
	}
	if !encoded {
		occupancy.PostEncodeMS = occupancy.HeldMS
		return occupancy
	}
	occupancy.PreEncodeMS = clampNonNegative(encodeStart - occupancy.StartMS)
	occupancy.EncodeMS = clampNonNegative(encodeEnd - encodeStart)
	occupancy.PostEncodeMS = clampNonNegative(occupancy.EndMS - encodeEnd)
	if occupancy.EncodeMS > occupancy.HeldMS {
		// Millisecond truncation of two independently rounded offsets can only
		// ever differ by one; the encode cannot outlast the slot that ran it.
		occupancy.EncodeMS = occupancy.HeldMS
	}
	return occupancy
}

func clampNonNegative(value int64) int64 {
	if value < 0 {
		return 0
	}
	return value
}
