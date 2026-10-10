package cloudbridge

import (
	"bufio"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/rechedev9/cliphub/internal/killplan"
)

const (
	steamImage = "steam.exe"
	cs2Image   = "cs2.exe"
	// Process checks shell out, so while a capture records they are reused
	// for a while instead of competing with CS2 on every heartbeat.
	steamCheckIdleTTL = 10 * time.Second
	steamCheckBusyTTL = 60 * time.Second
)

// Health is the machine report sent with every heartbeat. A value the worker
// cannot determine is left empty, never guessed.
type Health struct {
	Hostname      string `json:"hostname"`
	StudioVersion string `json:"studioVersion"`
	// PlanSchema is the kill plan schema this worker parses demos into. The
	// portal refuses a job whose Studio wrote its plan with another one.
	PlanSchema      string   `json:"planSchema"`
	CS2PatchVersion string   `json:"cs2PatchVersion"`
	HLAEVersion     string   `json:"hlaeVersion"`
	SteamRunning    bool     `json:"steamRunning"`
	RecordEnabled   bool     `json:"recordEnabled"`
	DiskFreeBytes   uint64   `json:"diskFreeBytes"`
	DiskTotalBytes  uint64   `json:"diskTotalBytes"`
	Kinds           []string `json:"kinds"`
	UptimeSeconds   int64    `json:"uptimeSeconds"`
}

// Machine is what the worker asks about the PC it runs on. Capturing tells
// the probe a capture is recording, so it can avoid extra process listings.
type Machine interface {
	Health(ctx context.Context, capturing bool) Health
	CS2Running(ctx context.Context) (bool, error)
}

// MachineConfig describes the installation a SystemMachine reports on.
type MachineConfig struct {
	DataDir       string
	CS2Path       string
	HLAEPath      string
	StudioVersion string
	RecordEnabled bool
	Kinds         []string
}

// SystemMachine reads the real machine: disk, installed versions and the
// Steam and CS2 processes.
type SystemMachine struct {
	cfg     MachineConfig
	started time.Time
	now     func() time.Time
	running func(ctx context.Context, image string) (bool, error)

	mu           sync.Mutex
	steamChecked time.Time
	steamRunning bool
}

// NewSystemMachine builds the probe for this process.
func NewSystemMachine(cfg MachineConfig) *SystemMachine {
	return &SystemMachine{cfg: cfg, started: time.Now(), now: time.Now, running: processRunning}
}

// Health never fails: a probe that errors leaves its field empty or false.
func (m *SystemMachine) Health(ctx context.Context, capturing bool) Health {
	hostname, _ := os.Hostname()
	free, total, _ := diskUsage(m.cfg.DataDir)
	kinds := m.cfg.Kinds
	if kinds == nil {
		kinds = []string{}
	}
	return Health{
		Hostname:        hostname,
		StudioVersion:   m.cfg.StudioVersion,
		PlanSchema:      killplan.SchemaVersion,
		CS2PatchVersion: cs2PatchVersion(m.cfg.CS2Path),
		HLAEVersion:     hlaeVersion(m.cfg.HLAEPath),
		SteamRunning:    m.steam(ctx, capturing),
		RecordEnabled:   m.cfg.RecordEnabled,
		DiskFreeBytes:   free,
		DiskTotalBytes:  total,
		Kinds:           kinds,
		UptimeSeconds:   int64(m.now().Sub(m.started).Seconds()),
	}
}

func (m *SystemMachine) steam(ctx context.Context, capturing bool) bool {
	ttl := steamCheckIdleTTL
	if capturing {
		ttl = steamCheckBusyTTL
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if !m.steamChecked.IsZero() && m.now().Sub(m.steamChecked) < ttl {
		return m.steamRunning
	}
	running, err := m.running(ctx, steamImage)
	m.steamRunning = err == nil && running
	m.steamChecked = m.now()
	return m.steamRunning
}

// CS2Running reports whether a cs2.exe is alive. The recorder refuses to
// start next to one, so the worker must not claim while it is.
func (m *SystemMachine) CS2Running(ctx context.Context) (bool, error) {
	return m.running(ctx, cs2Image)
}

// cs2PatchVersion reads PatchVersion from game\csgo\steam.inf, found by
// walking up from the configured cs2.exe (…\game\bin\win64\cs2.exe).
func cs2PatchVersion(cs2Path string) string {
	if cs2Path == "" {
		return ""
	}
	dir := filepath.Dir(cs2Path)
	for range 6 {
		for _, candidate := range []string{
			filepath.Join(dir, "game", "csgo", "steam.inf"),
			filepath.Join(dir, "csgo", "steam.inf"),
		} {
			if version := patchVersionFromFile(candidate); version != "" {
				return version
			}
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return ""
}

var patchVersionPattern = regexp.MustCompile(`^[0-9]+(\.[0-9]+){1,4}$`)

func patchVersionFromFile(path string) string {
	file, err := os.Open(path) //nolint:gosec // path derives from the configured CS2 install
	if err != nil {
		return ""
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		key, value, found := strings.Cut(strings.TrimSpace(scanner.Text()), "=")
		if !found || !strings.EqualFold(strings.TrimSpace(key), "PatchVersion") {
			continue
		}
		if version := strings.TrimSpace(value); patchVersionPattern.MatchString(version) {
			return version
		}
		return ""
	}
	return ""
}

var hlaeVersionPattern = regexp.MustCompile(`^[0-9]+(\.[0-9]+){1,3}([.-][A-Za-z0-9.-]{1,32})?$`)

// hlaeVersion reads the version Studio recorded in its install marker next to
// HLAE.exe, and otherwise takes it from the install directory name
// (C:\HLAE-2.192.6 or …\tools\hlae\2.192.6). Best effort.
func hlaeVersion(hlaePath string) string {
	if hlaePath == "" {
		return ""
	}
	dir := filepath.Dir(hlaePath)
	if raw, err := os.ReadFile(filepath.Join(dir, ".cliphub-install.json")); err == nil { //nolint:gosec // fixed marker name next to the configured HLAE
		var marker struct {
			Version string `json:"version"`
		}
		if json.Unmarshal(raw, &marker) == nil && hlaeVersionPattern.MatchString(marker.Version) {
			return marker.Version
		}
	}
	name := filepath.Base(dir)
	const prefix = "hlae-"
	if len(name) > len(prefix) && strings.EqualFold(name[:len(prefix)], prefix) {
		name = name[len(prefix):]
	} else if !strings.EqualFold(filepath.Base(filepath.Dir(dir)), "hlae") {
		return ""
	}
	if hlaeVersionPattern.MatchString(name) {
		return name
	}
	return ""
}
