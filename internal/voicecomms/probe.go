package voicecomms

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	demoinfocs "github.com/markus-wa/demoinfocs-golang/v5/pkg/demoinfocs"
	"github.com/markus-wa/demoinfocs-golang/v5/pkg/demoinfocs/common"
	"github.com/markus-wa/demoinfocs-golang/v5/pkg/demoinfocs/events"
	"github.com/markus-wa/demoinfocs-golang/v5/pkg/demoinfocs/msg"
)

func ProbeFile(demoPath, target string) (Report, error) {
	report, _, _, err := CollectFile(demoPath, target)
	return report, err
}

func CollectFile(demoPath, target string) (Report, []Packet, []Sighting, error) {
	abs, err := filepath.Abs(demoPath)
	if err != nil {
		return Report{}, nil, nil, fmt.Errorf("resolve demo path: %w", err)
	}
	// #nosec G304 -- demo path is an explicit local CLI input.
	f, err := os.Open(abs)
	if err != nil {
		return Report{}, nil, nil, fmt.Errorf("open demo: %w", err)
	}
	defer f.Close()

	p := demoinfocs.NewParser(f)
	defer p.Close()
	return Collect(p, target, abs, nil)
}

func Collect(p demoinfocs.Parser, target, demoPath string, spill *packetSpill) (Report, []Packet, []Sighting, error) {
	return collectContext(context.Background(), p, target, demoPath, spill)
}

func collectContext(ctx context.Context, p demoinfocs.Parser, target, demoPath string, spill *packetSpill) (Report, []Packet, []Sighting, error) {
	stop := make(chan struct{})
	var wg sync.WaitGroup
	wg.Go(func() {
		select {
		case <-ctx.Done():
			p.Cancel()
		case <-stop:
		}
	})
	defer func() { close(stop); wg.Wait() }()
	var (
		mapName        string
		maxTick        int
		packets        []Packet
		sightings      []Sighting
		collectionErr  error
		collectedBytes int64
		lastPacketTick int
	)
	lastSeen := map[string]Sighting{}
	recordSighting := func(s Sighting) {
		previous, ok := lastSeen[s.SteamID64]
		if s.SteamID64 == "" || (ok && previous.Team == s.Team && previous.Name == s.Name) {
			return
		}
		lastSeen[s.SteamID64] = s
		sightings = append(sightings, s)
	}
	snapshot := func() {
		for _, s := range snapshotPlaying(p) {
			recordSighting(s)
		}
	}
	p.RegisterEventHandler(func(e events.PlayerTeamChange) {
		if e.Player != nil && e.Player.SteamID64 != 0 {
			recordSighting(Sighting{SteamID64: fmt.Sprint(e.Player.SteamID64), Name: e.Player.Name, Team: teamLabel(e.NewTeam), Tick: p.GameState().IngameTick()})
		}
	})
	p.RegisterEventHandler(func(e events.PlayerDisconnected) {
		if e.Player != nil && e.Player.SteamID64 != 0 {
			recordSighting(Sighting{SteamID64: fmt.Sprint(e.Player.SteamID64), Name: e.Player.Name, Tick: p.GameState().IngameTick()})
		}
	})

	p.RegisterNetMessageHandler(func(info *msg.CSVCMsg_ServerInfo) {
		if name := info.GetMapName(); name != "" {
			mapName = name
		}
	})
	p.RegisterEventHandler(func(events.RoundStart) {
		snapshot()
		if tick := p.GameState().IngameTick(); tick > maxTick {
			maxTick = tick
		}
	})
	p.RegisterNetMessageHandler(func(m *msg.CSVCMsg_VoiceData) {
		if m == nil || collectionErr != nil {
			return
		}
		snapshot()
		pkt := Packet{
			XUID: m.GetXuid(),
			Tick: packetTick(int(m.GetTick()), p.GameState().IngameTick()),
		}
		pkt.ClockKind = "ingame_tick"
		if p.GameState().IngameTick() <= 0 {
			pkt.ClockKind = "voice_data_tick"
		}
		if pkt.Tick < lastPacketTick {
			pkt.ClockKind = "discontinuous"
		}
		lastPacketTick = max(lastPacketTick, pkt.Tick)
		if pkt.Tick > maxTick {
			maxTick = pkt.Tick
		}
		var data []byte
		var offsets []uint32
		if audio := m.GetAudio(); audio != nil {
			data = audio.GetVoiceData()
			pkt.Bytes = len(data)
			pkt.Format = formatFromProto(audio.GetFormat())
			pkt.SampleRate = audio.GetSampleRate()
			offsets = audio.GetPacketOffsets()
			pkt.Offsets = append([]uint32(nil), offsets...)
			if pkt.Format == FormatOpus {
				for _, frame := range splitVoiceFrames(data, offsets) {
					pkt.DurationSamples += opusFrameSamples(frame)
				}
			}
		}
		index := len(packets)
		collectedBytes += int64(len(data))
		if len(packets) >= 2000000 || collectedBytes > 1<<30 || len(data) > 65535 || len(offsets) > 255 {
			collectionErr = fmt.Errorf("voice packet collection exceeds resource limits")
			p.Cancel()
			return
		}
		if spill != nil && len(data) > 0 {
			if err := spill.write(index, pkt.XUID, pkt.Tick, data, offsets); err != nil {
				collectionErr = fmt.Errorf("persist voice packet %d: %w", index, err)
				p.Cancel()
				return
			}
		} else if len(data) > 0 {
			pkt.Data = append([]byte(nil), data...)
		}
		packets = append(packets, pkt)
	})

	parseErr := p.ParseToEnd()
	if collectionErr != nil {
		return Report{}, nil, nil, collectionErr
	}
	if err := ctx.Err(); err != nil {
		return Report{}, nil, nil, err
	}
	if err := parseErr; err != nil {
		return Report{}, nil, nil, fmt.Errorf("parse demo: %w", err)
	}
	snapshot()
	if tick := p.GameState().IngameTick(); tick > maxTick {
		maxTick = tick
	}

	report, err := Classify(target, packets, sightings, Meta{
		Demo:          demoPath,
		Map:           mapName,
		Tickrate:      int(p.TickRate()),
		DurationTicks: maxTick,
	})
	if err != nil {
		return Report{}, nil, nil, err
	}
	return report, packets, sightings, nil
}

func snapshotPlaying(p demoinfocs.Parser) []Sighting {
	players := p.GameState().Participants().Playing()
	out := make([]Sighting, 0, len(players))
	for _, pl := range players {
		if pl == nil || pl.SteamID64 == 0 {
			continue
		}
		out = append(out, Sighting{
			SteamID64: fmt.Sprintf("%d", pl.SteamID64),
			Name:      pl.Name,
			Team:      teamLabel(pl.Team),
			Tick:      p.GameState().IngameTick(),
		})
	}
	return out
}

// packetTick prefers IngameTick (the recap/capture clock). VoiceData.tick is
// 0 on current CS2 demos and can otherwise be a different net/match clock.
func packetTick(protoTick, ingameTick int) int {
	if ingameTick > 0 {
		return ingameTick
	}
	if protoTick > 0 {
		return protoTick
	}
	return 0
}

func teamLabel(t common.Team) string {
	switch t {
	case common.TeamCounterTerrorists:
		return "CT"
	case common.TeamTerrorists:
		return "T"
	default:
		return ""
	}
}

func formatFromProto(format msg.VoiceDataFormatT) string {
	switch format {
	case msg.VoiceDataFormatT_VOICEDATA_FORMAT_OPUS:
		return FormatOpus
	case msg.VoiceDataFormatT_VOICEDATA_FORMAT_STEAM:
		return FormatSteam
	case msg.VoiceDataFormatT_VOICEDATA_FORMAT_ENGINE:
		return FormatEngine
	default:
		return ""
	}
}
