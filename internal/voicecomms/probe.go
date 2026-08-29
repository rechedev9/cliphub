package voicecomms

import (
	"fmt"
	"os"
	"path/filepath"

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
	return Collect(p, target, abs)
}

func Collect(p demoinfocs.Parser, target, demoPath string) (Report, []Packet, []Sighting, error) {
	var (
		mapName   string
		maxTick   int
		packets   []Packet
		sightings []Sighting
	)

	p.RegisterNetMessageHandler(func(info *msg.CSVCMsg_ServerInfo) {
		if name := info.GetMapName(); name != "" {
			mapName = name
		}
	})
	p.RegisterEventHandler(func(events.RoundStart) {
		sightings = append(sightings, snapshotPlaying(p)...)
		if tick := p.GameState().IngameTick(); tick > maxTick {
			maxTick = tick
		}
	})
	p.RegisterNetMessageHandler(func(m *msg.CSVCMsg_VoiceData) {
		if m == nil {
			return
		}
		pkt := Packet{
			XUID: m.GetXuid(),
			Tick: packetTick(int(m.GetTick()), p.GameState().IngameTick()),
		}
		if pkt.Tick > maxTick {
			maxTick = pkt.Tick
		}
		if audio := m.GetAudio(); audio != nil {
			data := audio.GetVoiceData()
			pkt.Data = append([]byte(nil), data...)
			pkt.Bytes = len(pkt.Data)
			pkt.Format = formatFromProto(audio.GetFormat())
			pkt.SampleRate = audio.GetSampleRate()
			pkt.Offsets = append([]uint32(nil), audio.GetPacketOffsets()...)
		}
		packets = append(packets, pkt)
	})

	if err := p.ParseToEnd(); err != nil {
		return Report{}, nil, nil, fmt.Errorf("parse demo: %w", err)
	}
	sightings = append(sightings, snapshotPlaying(p)...)
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
