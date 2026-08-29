// Package telemetry owns the bounded, privacy-minimized event contract used by
// ClipHub Studio and the remote diagnostics collector.
package telemetry

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
)

const (
	SchemaVersion = 1

	KindError = "error"
	KindSpan  = "span"

	maxBatchEvents = 20
)

var (
	supportCodePattern = regexp.MustCompile(`^CH(?:-[A-F0-9]{4}){5}$`)
	releasePattern     = regexp.MustCompile(`^[0-9]{1,5}\.[0-9]{1,5}\.[0-9]{1,5}$`)

	allowedJournalStages = stringSet(
		"parse", "record", "render", "compose", "batch", "http", "worker",
		"tactical", "stream_acquire", "editor", "short", "unknown",
	)
	allowedJournalClasses = stringSet(
		"parse:demo", "scan:roster", "analyze:anticheat", "analyze:tactical",
		"record:demo", "compose:final", "render:variant", "render:stream-clip",
		"stream:acquire", "render:timeline", "interrupted", "not_found",
		"auth_required", "unavailable", "blocked", "too_large", "error",
		"demo_unreadable", "demo_incompatible", "map_uncalibrated", "write_artifact",
		"file_error", "corrupt", "target_not_found", "parse_failed",
		"capture_incompatible", "unplayable_start", "record_failed", "rhythm_failed",
		"render_failed", "stage_failed", "write_plan", "ffmpeg_failed", "unknown",
	)
	allowedTaskNames = stringSet(
		"parse:demo", "scan:roster", "analyze:anticheat", "analyze:tactical",
		"record:demo", "compose:final", "render:variant", "render:stream-clip",
		"stream:acquire", "render:timeline", "unknown",
	)
)

// Event is one error or sampled performance span. It deliberately has no
// arbitrary attributes or free text, so local paths, player data, credentials,
// and media metadata cannot cross the remote boundary.
type Event struct {
	SchemaVersion int       `json:"schema_version"`
	ID            string    `json:"id"`
	OccurredAt    time.Time `json:"occurred_at"`
	Kind          string    `json:"kind"`
	SupportCode   string    `json:"support_code"`
	SessionID     string    `json:"session_id"`
	Release       string    `json:"release"`
	Component     string    `json:"component"`
	Name          string    `json:"name"`
	Stage         string    `json:"stage,omitempty"`
	Class         string    `json:"class,omitempty"`
	Fingerprint   string    `json:"fingerprint,omitempty"`
	OS            string    `json:"os"`
	Arch          string    `json:"arch"`
	Outcome       string    `json:"outcome,omitempty"`
	DurationMS    int64     `json:"duration_ms,omitempty"`
}

// Batch is the only public-ingest request body.
type Batch struct {
	Events []Event `json:"events"`
}

// ValidateBatch validates and normalizes a client batch. now is injected so
// API tests and retention-boundary behavior stay deterministic.
func ValidateBatch(batch Batch, now time.Time) ([]Event, error) {
	if len(batch.Events) == 0 {
		return nil, errors.New("events must not be empty")
	}
	if len(batch.Events) > maxBatchEvents {
		return nil, fmt.Errorf("events exceeds limit %d", maxBatchEvents)
	}
	out := make([]Event, len(batch.Events))
	for i, event := range batch.Events {
		normalized, err := validateEvent(event, now)
		if err != nil {
			return nil, fmt.Errorf("event %d: %w", i, err)
		}
		out[i] = normalized
	}
	return out, nil
}

func validateEvent(event Event, now time.Time) (Event, error) {
	if event.SchemaVersion != SchemaVersion {
		return Event{}, fmt.Errorf("unsupported schema_version %d", event.SchemaVersion)
	}
	if _, err := uuid.Parse(event.ID); err != nil {
		return Event{}, errors.New("id must be a UUID")
	}
	if _, err := uuid.Parse(event.SessionID); err != nil {
		return Event{}, errors.New("session_id must be a UUID")
	}
	if !supportCodePattern.MatchString(event.SupportCode) {
		return Event{}, errors.New("support_code is invalid")
	}
	if event.OccurredAt.IsZero() {
		return Event{}, errors.New("occurred_at is required")
	}
	utcNow := now.UTC()
	if event.OccurredAt.After(utcNow.Add(5 * time.Minute)) {
		return Event{}, errors.New("occurred_at is in the future")
	}
	if event.OccurredAt.Before(utcNow.Add(-31 * 24 * time.Hour)) {
		return Event{}, errors.New("occurred_at is outside the retention window")
	}
	if !releasePattern.MatchString(event.Release) {
		return Event{}, errors.New("release is invalid")
	}
	if event.OS != "win32" {
		return Event{}, errors.New("os is invalid")
	}
	if event.Arch != "x64" && event.Arch != "arm64" && event.Arch != "ia32" {
		return Event{}, errors.New("arch is invalid")
	}
	if event.Fingerprint != "" {
		return Event{}, errors.New("fingerprint is server generated and must be omitted")
	}
	switch event.Kind {
	case KindError:
		if event.Class == "" {
			return Event{}, errors.New("error class is required")
		}
		if event.DurationMS != 0 {
			return Event{}, errors.New("error duration_ms must be zero")
		}
		if event.Outcome != "" {
			return Event{}, errors.New("error outcome must be empty")
		}
	case KindSpan:
		if event.DurationMS < 1 || event.DurationMS > int64((24*time.Hour)/time.Millisecond) {
			return Event{}, errors.New("span duration_ms is invalid")
		}
		if event.Outcome == "" {
			return Event{}, errors.New("span outcome is required")
		}
		if event.Class != "" {
			return Event{}, errors.New("span class must be empty")
		}
	default:
		return Event{}, errors.New("kind is invalid")
	}
	if err := validateEventCode(event); err != nil {
		return Event{}, err
	}
	event.OccurredAt = event.OccurredAt.UTC()
	event.Fingerprint = canonicalFingerprint(event)
	return event, nil
}

func validateEventCode(event Event) error {
	switch event.Component {
	case "electron":
		if event.Kind == KindError {
			allowed := map[string][2]string{
				"process.uncaught_exception":  {"runtime", "uncaught_exception"},
				"process.unhandled_rejection": {"runtime", "unhandled_rejection"},
				"desktop.boot_failed":         {"boot", "boot_failed"},
			}
			if labels, ok := allowed[event.Name]; ok && event.Stage == labels[0] && event.Class == labels[1] {
				return nil
			}
		} else if event.Name == "desktop.boot" && event.Stage == "boot" && event.Outcome == "ok" {
			return nil
		}
	case "renderer":
		if event.Kind == KindError {
			if (event.Name == "route.error" || event.Name == "global.error") && event.Stage == "renderer" && event.Class == "exception" {
				return nil
			}
			if event.Name == "renderer.process_gone" && event.Stage == "runtime" && event.Class == "process_gone" {
				return nil
			}
		} else if (event.Name == "navigation.dom_content_loaded" || event.Name == "navigation.load") && event.Stage == "renderer" && event.Outcome == "ok" {
			return nil
		}
	case "orchestrator":
		if event.Kind == KindError && event.Name == "pipeline.error" && allowedJournalStages[event.Stage] && allowedJournalClasses[event.Class] {
			return nil
		}
		if event.Kind == KindSpan && allowedTaskNames[event.Name] && (event.Stage == "worker" || event.Stage == "unknown") &&
			(event.Outcome == "ok" || event.Outcome == "error" || event.Outcome == "timeout" || event.Outcome == "cancelled") {
			return nil
		}
	}
	return errors.New("event code is not allowlisted")
}

func stringSet(values ...string) map[string]bool {
	set := make(map[string]bool, len(values))
	for _, value := range values {
		set[value] = true
	}
	return set
}

func canonicalFingerprint(event Event) string {
	value := strings.Join([]string{
		event.Kind,
		event.Release,
		event.Component,
		event.Name,
		event.Stage,
		event.Class,
		event.Outcome,
	}, "|")
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}
