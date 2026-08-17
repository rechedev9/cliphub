package parser

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"

	demoinfocs "github.com/markus-wa/demoinfocs-golang/v5/pkg/demoinfocs"
	"github.com/markus-wa/demoinfocs-golang/v5/pkg/demoinfocs/msg"
)

// PlayabilitySchemaVersion is the JSON schema id of playability.json.
const PlayabilitySchemaVersion = "tickcut.playability/v1"

// FirstPacketWatch records the first full-snapshot tick seen on a live parser.
type FirstPacketWatch struct {
	mu      sync.Mutex
	seen    bool
	ingame  int
	netTick int
	sawNet  bool
}

// TrackFirstPacketTick registers handlers for the first PacketEntities
// snapshot and the first CNETMsg_Tick. CS2's playdemo "game tick" often
// lives on the net tick, while IngameTick can start at 0 on remapped clocks.
func TrackFirstPacketTick(p demoinfocs.Parser) *FirstPacketWatch {
	w := &FirstPacketWatch{}
	p.RegisterNetMessageHandler(func(*msg.CSVCMsg_PacketEntities) {
		w.mu.Lock()
		defer w.mu.Unlock()
		if w.seen {
			return
		}
		w.seen = true
		w.ingame = p.GameState().IngameTick()
	})
	p.RegisterNetMessageHandler(func(tick *msg.CNETMsg_Tick) {
		w.mu.Lock()
		defer w.mu.Unlock()
		if w.sawNet {
			return
		}
		w.sawNet = true
		w.netTick = int(tick.GetTick())
	})
	return w
}

// Snapshot returns whether a PacketEntities message arrived and the
// IngameTick at that snapshot. CNETMsg_Tick on FACEIT GOTV is often a
// high match clock even when playdemo to tick 0 is safe, so it is not
// used for classification.
func (w *FirstPacketWatch) Snapshot() (tick int, seen bool) {
	if w == nil {
		return 0, false
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.ingame, w.seen
}

// PlayabilityReport is the JSON contract of `zv demo probe`.
type PlayabilityReport struct {
	OK                  bool             `json:"ok"`
	SchemaVersion       string           `json:"schema_version"`
	Demo                string           `json:"demo"`
	SHA256              string           `json:"sha256"`
	Bytes               int64            `json:"bytes"`
	Map                 string           `json:"map,omitempty"`
	Tickrate            int              `json:"tickrate"`
	HeaderPlaybackTicks int              `json:"header_playback_ticks"`
	FirstIngameTick     int              `json:"first_ingame_tick"`
	FirstNetTick        int              `json:"first_net_tick"`
	FirstFullPacketTick int              `json:"first_full_packet_tick"`
	Class               PlayabilityClass `json:"class"`
	Reason              string           `json:"reason"`
	CS2Smoke            string           `json:"cs2_smoke"`
}

// ProbeDemo walks a demo until the first full snapshot, then classifies it.
// It never launches CS2 or HLAE.
func ProbeDemo(demoPath string) (PlayabilityReport, error) {
	abs, err := filepath.Abs(demoPath)
	if err != nil {
		return PlayabilityReport{}, fmt.Errorf("resolve demo path: %w", err)
	}
	info, err := os.Stat(abs)
	if err != nil {
		return PlayabilityReport{}, fmt.Errorf("stat demo: %w", err)
	}
	sum, err := fileSHA256(abs)
	if err != nil {
		return PlayabilityReport{}, fmt.Errorf("hash demo: %w", err)
	}

	// #nosec G304 -- demo path is an explicit local CLI input.
	f, err := os.Open(abs)
	if err != nil {
		return PlayabilityReport{}, fmt.Errorf("open demo: %w", err)
	}
	defer f.Close()

	p := demoinfocs.NewParser(f)
	defer p.Close()

	var mapName string
	p.RegisterNetMessageHandler(func(info *msg.CSVCMsg_ServerInfo) {
		if name := info.GetMapName(); name != "" {
			mapName = name
		}
	})
	watch := TrackFirstPacketTick(p)
	p.RegisterNetMessageHandler(func(*msg.CSVCMsg_PacketEntities) {
		if _, seen := watch.Snapshot(); seen {
			p.Cancel()
		}
	})

	parseErr := p.ParseToEnd()
	tick, seen := watch.Snapshot()
	watch.mu.Lock()
	netTick := watch.netTick
	watch.mu.Unlock()
	report := PlayabilityReport{
		OK:                  true,
		SchemaVersion:       PlayabilitySchemaVersion,
		Demo:                abs,
		SHA256:              sum,
		Bytes:               info.Size(),
		Map:                 mapName,
		Tickrate:            int(p.TickRate()),
		FirstIngameTick:     tick,
		FirstNetTick:        netTick,
		FirstFullPacketTick: tick,
		CS2Smoke:            "not_run",
	}
	if !seen {
		if parseErr != nil && !errors.Is(parseErr, demoinfocs.ErrUnexpectedEndOfDemo) {
			report.OK = false
		}
		report.Class = PlayabilityCorrupt
		report.Reason = PlayabilityReason(report.Class, report.Tickrate, tick)
		return report, nil
	}
	report.Class = ClassifyPlayability(report.Tickrate, tick, true)
	report.Reason = PlayabilityReason(report.Class, report.Tickrate, tick)
	return report, nil
}

func fileSHA256(path string) (string, error) {
	// #nosec G304 -- path is a local demo the caller already opened.
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
