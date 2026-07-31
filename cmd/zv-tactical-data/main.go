// Command zv-tactical-data exports a sampled window of a demo as plain JSON for
// replay experiments. It owns no scanning logic of its own: the whole export is
// derived from internal/tactical's document plus the position blob it describes,
// so there is exactly one demo-scanning implementation in the product.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/rechedev9/tickcut/internal/pathguard"
	"github.com/rechedev9/tickcut/internal/tactical"
	"github.com/rechedev9/tickcut/internal/tacticalplan"
)

type frame struct {
	Tick    int      `json:"tick"`
	Players []player `json:"players"`
}

type player struct {
	SteamID64 string  `json:"steamid64"`
	Name      string  `json:"name"`
	Team      string  `json:"team"`
	Alive     bool    `json:"alive"`
	X         float64 `json:"x"`
	Y         float64 `json:"y"`
	Z         float64 `json:"z"`
	Yaw       float64 `json:"yaw"`
	Health    int     `json:"health"`
}

type kill struct {
	Tick       int    `json:"tick"`
	KillerID   string `json:"killer_steamid64"`
	KillerName string `json:"killer_name"`
	VictimID   string `json:"victim_steamid64"`
	VictimName string `json:"victim_name"`
	Weapon     string `json:"weapon"`
	Headshot   bool   `json:"headshot"`
	KillerTeam string `json:"killer_team"`
	VictimTeam string `json:"victim_team"`
}

type output struct {
	Demo     string  `json:"demo"`
	Start    int     `json:"start_tick"`
	End      int     `json:"end_tick"`
	Sample   int     `json:"sample_ticks"`
	Tickrate float64 `json:"tickrate"`
	Frames   []frame `json:"frames"`
	Kills    []kill  `json:"kills"`
}

func main() {
	var demoPath string
	var outPath string
	var startTick int
	var endTick int
	var sampleTicks int
	flag.StringVar(&demoPath, "demo", "", "path to .dem file")
	flag.StringVar(&outPath, "out", "", "output JSON path")
	flag.IntVar(&startTick, "start", 0, "first tick to sample")
	flag.IntVar(&endTick, "end", 0, "last tick to sample")
	flag.IntVar(&sampleTicks, "sample", 4, "sample interval in ticks")
	flag.Parse()

	if demoPath == "" || outPath == "" || startTick <= 0 || endTick <= startTick {
		log.Fatal("--demo, --out, --start, and --end are required")
	}
	if sampleTicks <= 0 {
		sampleTicks = 1
	}
	if err := validateOutputPath(outPath, demoPath); err != nil {
		log.Fatal(err)
	}

	// The shared scan takes a sample rate in Hz, not an interval in ticks, and a
	// CS2 demo only reveals its tick rate part-way through parsing. Scanning at
	// the maximum rate therefore samples as finely as the demo allows, and the
	// requested interval is applied while decoding, which is exact whenever the
	// scan's own interval divides it.
	scan, err := tactical.ScanFile(context.Background(), demoPath, tactical.Options{SampleHZ: tactical.MaxSampleHZ})
	if err != nil {
		log.Fatal(err)
	}

	result, err := export(scan, demoPath, startTick, endTick, sampleTicks)
	if err != nil {
		log.Fatal(err)
	}

	b, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		log.Fatal(err)
	}
	if err := os.WriteFile(outPath, append(b, '\n'), 0o600); err != nil {
		log.Fatal(err)
	}
}

func validateOutputPath(outPath, demoPath string) error {
	return pathguard.RejectOutputAliases(outPath, pathguard.Input{Flag: "--demo", Path: demoPath})
}

func export(scan tactical.Result, demoPath string, startTick, endTick, sampleTicks int) (output, error) {
	doc := scan.Document
	result := output{
		Demo:     demoPath,
		Start:    startTick,
		End:      endTick,
		Sample:   sampleTicks,
		Tickrate: doc.Demo.Tickrate,
		Frames:   []frame{},
		Kills:    []kill{},
	}

	names := map[uint8]tacticalplan.Player{}
	for _, p := range doc.Players {
		names[p.Slot] = p
	}

	lastEmitted := startTick - sampleTicks
	for _, offset := range doc.Positions.RoundOffsets {
		if offset.LastTick < startTick || offset.FirstTick > endTick {
			continue
		}
		frames, err := tacticalplan.DecodeFrames(scan.Positions.Data, offset.ByteOffset, offset.FrameCount, doc.Positions)
		if err != nil {
			return output{}, fmt.Errorf("decode round %d positions: %w", offset.Round, err)
		}
		for _, f := range frames {
			if f.Tick < startTick || f.Tick > endTick || f.Tick-lastEmitted < sampleTicks {
				continue
			}
			lastEmitted = f.Tick
			out := frame{Tick: f.Tick}
			for _, s := range f.Samples {
				identity := names[s.Slot]
				out.Players = append(out.Players, player{
					SteamID64: identity.SteamID64,
					Name:      identity.Name,
					Team:      sampleTeam(s.Flags),
					Alive:     s.Flags.Has(tacticalplan.FlagAlive),
					X:         s.X,
					Y:         s.Y,
					Z:         s.Z,
					Yaw:       s.Yaw,
					Health:    s.Health,
				})
			}
			result.Frames = append(result.Frames, out)
		}
	}

	for _, round := range doc.Rounds {
		sides := map[uint8]tacticalplan.Side{}
		for _, pr := range round.Players {
			sides[pr.Slot] = pr.Side
		}
		for _, event := range round.Events {
			if event.Kind != tacticalplan.EventKill || event.Tick < startTick || event.Tick > endTick {
				continue
			}
			// A kill with no actor is a suicide or a world kill; the export has
			// always described attacker-versus-victim pairs only.
			if event.ActorSlot == nil || event.TargetSlot == nil {
				continue
			}
			killer := names[*event.ActorSlot]
			victim := names[*event.TargetSlot]
			result.Kills = append(result.Kills, kill{
				Tick:       event.Tick,
				KillerID:   killer.SteamID64,
				KillerName: killer.Name,
				VictimID:   victim.SteamID64,
				VictimName: victim.Name,
				Weapon:     event.Weapon,
				Headshot:   event.Headshot,
				KillerTeam: string(sides[*event.ActorSlot]),
				VictimTeam: string(sides[*event.TargetSlot]),
			})
		}
	}
	return result, nil
}

func sampleTeam(flags tacticalplan.SampleFlags) string {
	if flags.Has(tacticalplan.FlagSideT) {
		return string(tacticalplan.SideT)
	}
	return string(tacticalplan.SideCT)
}
