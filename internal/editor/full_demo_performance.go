package editor

import (
	"context"
	"sort"
	"sync"
	"time"
)

// Full Demo timing evidence is derived from the existing FFmpeg diagnostic
// execution. runDiagnosticFFmpeg already times every subprocess for its
// tool.started/tool.finished trace events. When a render attaches a collector
// and a stage scope through the context, the same interval and outcome are also
// recorded in memory: no second process, log sink or timing loop is added.
//
// Records are deliberately content-free: stage identifiers, original item or
// track index, attempt index, a fixed code-controlled label and an encoder name.
// No media paths, command arguments or user strings are stored.

const (
	fullDemoTimingMaxSpans = 4096
	fullDemoTimingMaxLabel = 96
)

// FullDemoTimingSpan is one completed FFmpeg execution. StartMS/EndMS are
// offsets from the collector start, so overlapping work is visible without
// wall-clock timestamps. Variant distinguishes parallel strategies of one stage
// (for example a native master from a Media Foundation recovery attempt).
type FullDemoTimingSpan struct {
	Stage         string  `json:"stage"`
	Index         int     `json:"index"`
	Attempt       int     `json:"attempt"`
	Variant       string  `json:"variant,omitempty"`
	Label         string  `json:"label"`
	Encoder       string  `json:"encoder,omitempty"`
	MediaDuration float64 `json:"media_duration_seconds,omitempty"`
	StartMS       int64   `json:"start_ms"`
	EndMS         int64   `json:"end_ms"`
	ElapsedMS     int64   `json:"elapsed_ms"`
	Outcome       string  `json:"outcome"`
}

// FullDemoTimingStage aggregates every span of one stage.
//
// WallMS is the union of the stage's active tool intervals: the elapsed time
// during which at least one tool of that stage was running. It is not the whole
// render or stage elapsed time, and it deliberately excludes any gaps between
// tools. RenderPerformance.RenderMS remains the total render wall time.
//
// ProcessElapsedSumMS is the sum of span elapsed times: total process wall time
// if the stage ran serially. It is process elapsed time, not CPU time and not
// wall time, and it must never be presented as either.
type FullDemoTimingStage struct {
	Stage               string `json:"stage"`
	Spans               int    `json:"spans"`
	WallMS              int64  `json:"wall_ms"`
	ProcessElapsedSumMS int64  `json:"process_elapsed_sum_ms"`
}

// FullDemoTimingMetrics is the optional per-render timing snapshot. WallMS is
// the union of every recorded tool interval; ProcessElapsedSumMS is the sum of
// every span's elapsed time (serialized process time), never wall time.
type FullDemoTimingMetrics struct {
	Spans               []FullDemoTimingSpan  `json:"spans"`
	Stages              []FullDemoTimingStage `json:"stages"`
	WallMS              int64                 `json:"wall_ms"`
	ProcessElapsedSumMS int64                 `json:"process_elapsed_sum_ms"`
	Truncated           bool                  `json:"truncated,omitempty"`
}

type fullDemoTimingCollectorKey struct{}
type fullDemoTimingScopeKey struct{}

type fullDemoTimingCollector struct {
	mu     sync.Mutex
	start  time.Time
	spans  []FullDemoTimingSpan
	closed bool
	trunc  bool
}

// fullDemoTimingAnnotation carries the collector reference plus the current stage
// annotation. It is immutable; a nested annotation replaces the context value.
type fullDemoTimingAnnotation struct {
	collector *fullDemoTimingCollector
	stage     string
	index     int
	attempt   int
	variant   string
	encoder   string
	duration  float64
}

func newFullDemoTimingCollector() *fullDemoTimingCollector {
	return &fullDemoTimingCollector{start: time.Now()}
}

// withFullDemoTimingCollector attaches an empty collector for one render. It is
// called from renderShort only for Full Demo shorts, so non-FullDemo renders
// carry no collector and keep their exact behavior.
func withFullDemoTimingCollector(ctx context.Context) (context.Context, *fullDemoTimingCollector) {
	collector := newFullDemoTimingCollector()
	return context.WithValue(ctx, fullDemoTimingCollectorKey{}, collector), collector
}

func fullDemoTimingCollectorFrom(ctx context.Context) *fullDemoTimingCollector {
	collector, _ := ctx.Value(fullDemoTimingCollectorKey{}).(*fullDemoTimingCollector)
	return collector
}

func fullDemoTimingScopeFrom(ctx context.Context) *fullDemoTimingAnnotation {
	scope, _ := ctx.Value(fullDemoTimingScopeKey{}).(*fullDemoTimingAnnotation)
	return scope
}

// fullDemoTimingScope annotates every FFmpeg run started under the returned
// context with a stable stage, the original item/track index, the attempt index
// (-1 when the work is not attempt-specific) and the expected media duration.
// Without an attached collector it returns ctx unchanged, so callers can wrap
// unconditionally and non-FullDemo code is untouched.
func fullDemoTimingScope(ctx context.Context, stage string, index int, attempt int, durationSeconds float64) context.Context {
	return fullDemoTimingScopeEncoder(ctx, stage, index, attempt, "", durationSeconds)
}

func fullDemoTimingScopeEncoder(ctx context.Context, stage string, index int, attempt int, encoder string, durationSeconds float64) context.Context {
	collector := fullDemoTimingCollectorFrom(ctx)
	if collector == nil {
		return ctx
	}
	return context.WithValue(ctx, fullDemoTimingScopeKey{}, &fullDemoTimingAnnotation{
		collector: collector,
		stage:     stage,
		index:     index,
		attempt:   attempt,
		encoder:   encoder,
		duration:  durationSeconds,
	})
}

// fullDemoTimingStage keeps the current index, encoder and duration but changes
// the stage and attempt. It lets audio/assembly code annotate sub-stages
// without re-specifying the render's identity.
func fullDemoTimingStage(ctx context.Context, stage string, attempt int) context.Context {
	return fullDemoTimingStageVariant(ctx, stage, attempt, "", "")
}

// fullDemoTimingStageEncoder is fullDemoTimingStage with an explicit encoder.
func fullDemoTimingStageEncoder(ctx context.Context, stage string, attempt int, encoder string) context.Context {
	return fullDemoTimingStageVariant(ctx, stage, attempt, encoder, "")
}

// fullDemoTimingStageVariant additionally records a fixed strategy name, so a
// native master ("native") and a Media Foundation recovery attempt ("recovery")
// of the same stage stay distinguishable without changing the stage vocabulary.
func fullDemoTimingStageVariant(ctx context.Context, stage string, attempt int, encoder, variant string) context.Context {
	scope := fullDemoTimingScopeFrom(ctx)
	if scope == nil {
		return ctx
	}
	return context.WithValue(ctx, fullDemoTimingScopeKey{}, &fullDemoTimingAnnotation{
		collector: scope.collector,
		stage:     stage,
		index:     scope.index,
		attempt:   attempt,
		variant:   variant,
		encoder:   encoder,
		duration:  scope.duration,
	})
}

// fullDemoTimingRecord stores the exact interval and outcome runDiagnosticFFmpeg
// reports in its trace events. Failure is "error" while a context cancellation
// is "cancelled", so a failed and a cancelled attempt stay distinguishable. The
// encoder is inferred from the actual command when the caller did not name one,
// so item encodes (which only know their stage scope) still carry it.
func fullDemoTimingRecord(ctx context.Context, label string, args []string, start, end time.Time, err error) {
	scope := fullDemoTimingScopeFrom(ctx)
	if scope == nil || scope.collector == nil {
		return
	}
	outcome := "ok"
	if err != nil {
		outcome = "error"
		if ctx.Err() != nil {
			outcome = "cancelled"
		}
	}
	scope.collector.append(scope, label, args, start, end, outcome)
}

func (c *fullDemoTimingCollector) append(scope *fullDemoTimingAnnotation, label string, args []string, start, end time.Time, outcome string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return
	}
	if len(c.spans) >= fullDemoTimingMaxSpans {
		c.trunc = true
		return
	}
	if end.Before(start) {
		end = start
	}
	encoder := scope.encoder
	if encoder == "" {
		encoder = fullDemoTimingEncoder(args)
	}
	c.spans = append(c.spans, FullDemoTimingSpan{
		Stage:         scope.stage,
		Index:         scope.index,
		Attempt:       scope.attempt,
		Variant:       scope.variant,
		Label:         fullDemoTimingLabel(label),
		Encoder:       encoder,
		MediaDuration: scope.duration,
		StartMS:       start.Sub(c.start).Milliseconds(),
		EndMS:         end.Sub(c.start).Milliseconds(),
		ElapsedMS:     end.Sub(start).Milliseconds(),
		Outcome:       outcome,
	})
}

// snapshotInto persists the snapshot on the render performance record. The
// collector is closed by the first snapshot, so late callbacks are ignored
// instead of racing the reader.
func (c *fullDemoTimingCollector) snapshotInto(performance *RenderPerformance) {
	if c == nil || performance == nil {
		return
	}
	performance.FullDemoTiming = c.snapshot()
}

func (c *fullDemoTimingCollector) snapshot() *FullDemoTimingMetrics {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return nil
	}
	c.closed = true
	if len(c.spans) == 0 {
		return nil
	}
	spans := append([]FullDemoTimingSpan(nil), c.spans...)
	// Append order follows process completion, which is not deterministic under
	// concurrency. Sort by the recorded interval and fixed fields so equal work
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
		if a.Index != b.Index {
			return a.Index < b.Index
		}
		if a.Attempt != b.Attempt {
			return a.Attempt < b.Attempt
		}
		return a.Label < b.Label
	})
	metrics := &FullDemoTimingMetrics{
		Spans:     spans,
		Stages:    fullDemoTimingStages(spans),
		Truncated: c.trunc,
	}
	metrics.WallMS = fullDemoTimingUnion(metrics.Spans)
	for _, span := range metrics.Spans {
		metrics.ProcessElapsedSumMS += span.ElapsedMS
	}
	return metrics
}

// fullDemoTimingStages groups spans in first-appearance order of the sorted
// spans so stage ordering is deterministic too.
func fullDemoTimingStages(spans []FullDemoTimingSpan) []FullDemoTimingStage {
	order := make([]string, 0, 8)
	byStage := map[string][]FullDemoTimingSpan{}
	for _, span := range spans {
		if _, seen := byStage[span.Stage]; !seen {
			order = append(order, span.Stage)
		}
		byStage[span.Stage] = append(byStage[span.Stage], span)
	}
	stages := make([]FullDemoTimingStage, 0, len(order))
	for _, stage := range order {
		group := byStage[stage]
		entry := FullDemoTimingStage{Stage: stage, Spans: len(group), WallMS: fullDemoTimingUnion(group)}
		for _, span := range group {
			entry.ProcessElapsedSumMS += span.ElapsedMS
		}
		stages = append(stages, entry)
	}
	return stages
}

// fullDemoTimingUnion returns the length of the merged interval set. Spans are
// already sorted by start offset.
func fullDemoTimingUnion(spans []FullDemoTimingSpan) int64 {
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

// fullDemoTimingEncoder returns a fixed encoder name inferred from the actual
// FFmpeg command. It is only a whitelist of known codec names, so no command
// arguments or paths can leak into the evidence.
func fullDemoTimingEncoder(args []string) string {
	for i := 0; i+1 < len(args); i++ {
		switch args[i] {
		case "-c:v", "-codec:v", "-c:a", "-codec:a":
			switch args[i+1] {
			case "h264_nvenc", "libx264", "aac_mf", "aac":
				return args[i+1]
			}
		}
	}
	return ""
}

// fullDemoTimingLabel bounds and sanitizes the code-controlled label. Labels
// must not carry paths or user text; control characters are dropped and the
// result is truncated deterministically.
func fullDemoTimingLabel(label string) string {
	out := make([]rune, 0, len(label))
	for _, r := range label {
		if r < 0x20 || r == 0x7f {
			continue
		}
		out = append(out, r)
		if len(out) == fullDemoTimingMaxLabel {
			break
		}
	}
	return string(out)
}
