// Package jobprogress is the durable in-flight snapshot every long Studio
// wait reads: percent plus current/total in real worker units (ticks,
// segments, rounds, bytes, clips, maps). Workers write it; HTTP serves it;
// leaving a screen and coming back resumes the same numbers.
package jobprogress

import (
	"encoding/json"
	"fmt"
	"io"
	"time"
)

const SchemaVersion = 1

const (
	StageScan      = "scan"
	StageParse     = "parse"
	StageAnticheat = "anticheat"
	StageTactical  = "tactical"
	StageRecord    = "record"
	StageCompose   = "compose"
	StageRender    = "render"
	StageAcquire   = "acquire"
	StageUpload    = "upload"
)

const (
	UnitTicks    = "ticks"
	UnitSegments = "segments"
	UnitRounds   = "rounds"
	UnitBytes    = "bytes"
	UnitClips    = "clips"
	UnitMaps     = "maps"
)

// Snapshot is one durable progress observation. Done/Total are the worker's
// real units; Percent is 0-100 derived from them (never a wall-clock fake).
type Snapshot struct {
	SchemaVersion int       `json:"schema_version"`
	Stage         string    `json:"stage"`
	Unit          string    `json:"unit"`
	Done          int64     `json:"done"`
	Total         int64     `json:"total"`
	Percent       int       `json:"percent"`
	Label         string    `json:"label,omitempty"`
	UpdatedAt     time.Time `json:"updated_at"`
}

// PercentOf is the 0-100 share of done/total. A zero total is 0, not a panic.
func PercentOf(done, total int64) int {
	if total <= 0 {
		return 0
	}
	if done <= 0 {
		return 0
	}
	if done >= total {
		return 100
	}
	pct := int((done*100 + total/2) / total)
	if pct < 0 {
		return 0
	}
	if pct > 100 {
		return 100
	}
	return pct
}

// NewSnapshot builds a validated snapshot. Label is the Spanish unit word the
// UI prints next to the count (ticks, rondas, segmentos, bytes, clips, mapas).
func NewSnapshot(stage, unit, label string, done, total int64, now time.Time) (Snapshot, error) {
	if stage == "" {
		return Snapshot{}, fmt.Errorf("job progress stage is required")
	}
	if unit == "" {
		return Snapshot{}, fmt.Errorf("job progress unit is required")
	}
	if done < 0 {
		done = 0
	}
	if total < 0 {
		total = 0
	}
	if total > 0 && done > total {
		done = total
	}
	if now.IsZero() {
		now = time.Now().UTC()
	}
	return Snapshot{
		SchemaVersion: SchemaVersion,
		Stage:         stage,
		Unit:          unit,
		Done:          done,
		Total:         total,
		Percent:       PercentOf(done, total),
		Label:         label,
		UpdatedAt:     now.UTC(),
	}, nil
}

func (s Snapshot) Validate() error {
	if s.SchemaVersion != SchemaVersion {
		return fmt.Errorf("unsupported job progress schema %d", s.SchemaVersion)
	}
	if s.Stage == "" || s.Unit == "" {
		return fmt.Errorf("job progress stage and unit are required")
	}
	if s.Done < 0 || s.Total < 0 {
		return fmt.Errorf("job progress done/total must be >= 0")
	}
	if s.Percent < 0 || s.Percent > 100 {
		return fmt.Errorf("job progress percent %d is out of range", s.Percent)
	}
	if s.UpdatedAt.IsZero() {
		return fmt.Errorf("job progress updated at is required")
	}
	return nil
}

func (s Snapshot) Encode(w io.Writer) error {
	enc := json.NewEncoder(w)
	if err := enc.Encode(s); err != nil {
		return fmt.Errorf("encode job progress: %w", err)
	}
	return nil
}

func Decode(r io.Reader) (Snapshot, error) {
	var s Snapshot
	if err := json.NewDecoder(r).Decode(&s); err != nil {
		return Snapshot{}, fmt.Errorf("decode job progress: %w", err)
	}
	if err := s.Validate(); err != nil {
		return Snapshot{}, err
	}
	return s, nil
}
