package customhud

import (
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"strings"
	"testing"

	demoinfocs "github.com/markus-wa/demoinfocs-golang/v5/pkg/demoinfocs"
	"github.com/markus-wa/demoinfocs-golang/v5/pkg/demoinfocs/common"
	"github.com/markus-wa/demoinfocs-golang/v5/pkg/demoinfocs/events"
	st "github.com/markus-wa/demoinfocs-golang/v5/pkg/demoinfocs/sendtables"
	dp "github.com/markus-wa/godispatch"
)

type extractionState struct {
	demoinfocs.GameState
	tick, rounds   int
	warmup, freeze bool
	players        []*common.Player
}

func (s *extractionState) IngameTick() int                          { return s.tick }
func (s *extractionState) TotalRoundsPlayed() int                   { return s.rounds }
func (s *extractionState) IsWarmupPeriod() bool                     { return s.warmup }
func (s *extractionState) IsFreezetimePeriod() bool                 { return s.freeze }
func (s *extractionState) Rules() demoinfocs.GameRules              { return extractionRules{} }
func (s *extractionState) TeamCounterTerrorists() *common.TeamState { return nil }
func (s *extractionState) TeamTerrorists() *common.TeamState        { return nil }
func (s *extractionState) Participants() demoinfocs.Participants {
	return extractionPlayers{players: s.players}
}

type extractionRules struct{ demoinfocs.GameRules }

func (extractionRules) Entity() st.Entity { return nil }

type extractionPlayers struct {
	demoinfocs.Participants
	players []*common.Player
}

func (s extractionPlayers) All() []*common.Player { return s.players }

type extractionParser struct {
	demoinfocs.Parser
	state    *extractionState
	dispatch dp.Dispatcher
	frames   []func(*extractionParser)
}

func (p *extractionParser) GameState() demoinfocs.GameState { return p.state }
func (p *extractionParser) RegisterEventHandler(handler any) dp.HandlerIdentifier {
	return p.dispatch.RegisterHandler(handler)
}
func (p *extractionParser) RegisterNetMessageHandler(any) dp.HandlerIdentifier {
	var id dp.HandlerIdentifier
	return id
}
func (p *extractionParser) Close() error { return nil }
func (p *extractionParser) ParseToEnd() error {
	for _, frame := range p.frames {
		frame(p)
		p.dispatch.Dispatch(events.FrameDone{})
	}
	return nil
}

func extractFrames(t *testing.T, frames ...func(*extractionParser)) Timeline {
	t.Helper()
	pl := &common.Player{SteamID64: 76561199000000001, Name: "observed", Team: common.TeamCounterTerrorists, IsConnected: true, Entity: propertyEntity{}}
	p := &extractionParser{state: &extractionState{players: []*common.Player{pl}}, frames: frames}
	d, err := extract(context.Background(), strings.NewReader(""), fmt.Sprintf("%x", sha256.Sum256(nil)), ExampleTarget, 64, func(io.Reader) demoinfocs.Parser { return p })
	if err != nil {
		t.Fatal(err)
	}
	return d
}

func TestExtractionCoversFreezePrefixWithoutRoundStart(t *testing.T) {
	d := extractFrames(t,
		func(p *extractionParser) { p.state.tick, p.state.freeze = 100, true },
		func(p *extractionParser) {
			p.state.tick, p.state.freeze = 228, false
			p.dispatch.Dispatch(events.RoundFreezetimeEnd{})
		},
		func(p *extractionParser) { p.state.tick = 292; p.dispatch.Dispatch(events.RoundEnd{}) },
		func(p *extractionParser) { p.state.tick, p.state.rounds, p.state.freeze = 356, 1, true },
		func(p *extractionParser) {
			p.state.tick, p.state.freeze = 484, false
			p.dispatch.Dispatch(events.RoundFreezetimeEnd{})
		},
		func(p *extractionParser) { p.state.tick = 548 },
	)
	for _, tc := range []struct {
		tick, round int
		phase       string
	}{{100, 1, "freeze"}, {228, 1, "live"}, {292, 1, "ended"}, {356, 2, "freeze"}, {484, 2, "live"}} {
		got, ok := d.At(tc.tick)
		if !ok || got.Round != tc.round || got.Phase != tc.phase {
			t.Fatalf("at %d: round=%d phase=%q covered=%v", tc.tick, got.Round, got.Phase, ok)
		}
	}
	r, _ := NewRenderer("arena")
	for _, start := range []int{100, 356} {
		if _, err := r.ASS(d, Window{StartTick: start, Frames: 180}); err != nil {
			t.Fatalf("freeze prefix at %d: %v", start, err)
		}
	}
}

func TestExtractionRetiresDisconnectedRosterIdentity(t *testing.T) {
	d := extractFrames(t,
		func(p *extractionParser) {
			p.state.tick, p.state.freeze = 100, true
			p.dispatch.Dispatch(events.RoundStart{})
		},
		func(p *extractionParser) { p.state.tick = 164; p.state.players[0].IsConnected = false },
		func(p *extractionParser) { p.state.tick = 228; p.state.players = nil },
	)
	for _, tick := range []int{164, 228} {
		s, _ := d.At(tick)
		r, _ := NewRenderer("arena")
		focus := false
		for _, n := range r.Scene(s, ExampleTarget) {
			if n.ID == "player/"+ExampleTarget+"/name" {
				t.Fatalf("disconnected identity still occupies a roster slot at %d", tick)
			}
			if n.ID == "alive/ct" && n.Text != "0" {
				t.Fatalf("disconnected player changed team count to %q", n.Text)
			}
			if n.ID == "focus/name" {
				focus = true
			}
		}
		if !focus {
			t.Fatal("lost observed identity after disconnect")
		}
	}
}

type missingRoundStartParser struct{ demoinfocs.Parser }

func (p missingRoundStartParser) RegisterEventHandler(handler any) dp.HandlerIdentifier {
	if _, skip := handler.(func(events.RoundStart)); skip {
		var id dp.HandlerIdentifier
		return id
	}
	return p.Parser.RegisterEventHandler(handler)
}

func TestExtractRealDemoWithoutRoundStart(t *testing.T) {
	path, target := os.Getenv("FULL_DEMO_HUD_DEMO"), os.Getenv("FULL_DEMO_HUD_TARGET")
	if path == "" || target == "" {
		t.Skip("set FULL_DEMO_HUD_DEMO and FULL_DEMO_HUD_TARGET for real demo acceptance")
	}
	file, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	digest := sha256.New()
	if _, err := io.Copy(digest, file); err != nil {
		t.Fatal(err)
	}
	sha := fmt.Sprintf("%x", digest.Sum(nil))
	read := func(missingStart bool) Timeline {
		if _, err := file.Seek(0, 0); err != nil {
			t.Fatal(err)
		}
		d, err := extract(context.Background(), file, sha, target, 64, func(r io.Reader) demoinfocs.Parser {
			p := demoinfocs.NewParser(r)
			if missingStart {
				return missingRoundStartParser{p}
			}
			return p
		})
		if err != nil {
			t.Fatal(err)
		}
		return d
	}
	normal, missing := read(false), read(true)
	r, _ := NewRenderer("arena")
	checked := map[int]bool{}
	for _, s := range normal.Snapshots {
		if s.Phase != "live" || checked[s.Round] {
			continue
		}
		if s.Round != s.CTScore+s.TScore+1 {
			t.Fatalf("source round %d disagrees with %d:%d", s.Round, s.CTScore, s.TScore)
		}
		start := s.Tick - 2*normal.TickRate
		prefix, covered := missing.At(start)
		if !covered || prefix.Round != s.Round || prefix.Phase != "freeze" {
			t.Fatalf("round %d freeze prefix not recovered: %+v", s.Round, prefix)
		}
		if _, err := r.ASS(missing, Window{StartTick: start, Frames: 180}); err != nil {
			t.Fatalf("round %d: %v", s.Round, err)
		}
		checked[s.Round] = true
	}
	if len(checked) == 0 {
		t.Fatal("no real rounds verified")
	}
	t.Logf("verified freeze prefixes and source round labels for %d rounds with every RoundStart suppressed", len(checked))
}
