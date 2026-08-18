package voicecomms

import (
	"math"
	"sort"
	"strconv"
)

const estimatedFrameSeconds = 0.020

func Classify(target string, packets []Packet, sightings []Sighting, meta Meta) (Report, error) {
	targetID, err := strconv.ParseUint(target, 10, 64)
	if err != nil {
		return Report{}, ErrInvalidTarget
	}
	identity := lastSightingByID(sightings)
	targetSeen, ok := identity[target]
	if !ok {
		return Report{}, ErrTargetNotFound
	}

	byXUID := map[uint64]*PlayerVoice{}
	formats := map[string]struct{}{}
	var sampleRate uint32
	for _, pkt := range packets {
		if pkt.Format != "" {
			formats[pkt.Format] = struct{}{}
		}
		if pkt.SampleRate > 0 {
			sampleRate = pkt.SampleRate
		}
		slot := byXUID[pkt.XUID]
		if slot == nil {
			id := strconv.FormatUint(pkt.XUID, 10)
			seen := identity[id]
			slot = &PlayerVoice{
				SteamID64: id,
				Name:      seen.Name,
				Team:      seen.Team,
				FirstTick: pkt.Tick,
				LastTick:  pkt.Tick,
			}
			byXUID[pkt.XUID] = slot
		}
		slot.Packets++
		slot.Bytes += pkt.Bytes
		if pkt.Tick < slot.FirstTick || slot.FirstTick == 0 {
			slot.FirstTick = pkt.Tick
		}
		if pkt.Tick > slot.LastTick {
			slot.LastTick = pkt.Tick
		}
	}

	targetVoice := PlayerVoice{
		SteamID64: target,
		Name:      targetSeen.Name,
		Team:      targetSeen.Team,
	}
	if slot := byXUID[targetID]; slot != nil {
		targetVoice.Packets = slot.Packets
		targetVoice.Bytes = slot.Bytes
		targetVoice.FirstTick = slot.FirstTick
		targetVoice.LastTick = slot.LastTick
		targetVoice.SecondsEstimate = secondsEstimate(slot.Packets)
	}

	var teammates []PlayerVoice
	var others OtherVoice
	for xuid, slot := range byXUID {
		if xuid == targetID {
			continue
		}
		slot.SecondsEstimate = secondsEstimate(slot.Packets)
		if sameSideAt(sightings, target, slot.SteamID64, slot.LastTick) {
			teammates = append(teammates, *slot)
			continue
		}
		others.Players++
		others.Packets += slot.Packets
		others.Bytes += slot.Bytes
	}
	sort.Slice(teammates, func(i, j int) bool {
		return teammates[i].SteamID64 < teammates[j].SteamID64
	})
	if teammates == nil {
		teammates = []PlayerVoice{}
	}

	return Report{
		SchemaVersion: SchemaVersion,
		Demo:          meta.Demo,
		Map:           meta.Map,
		Tickrate:      meta.Tickrate,
		DurationTicks: meta.DurationTicks,
		VoicePresent:  len(packets) > 0,
		Format:        formatLabel(formats),
		SampleRateHz:  sampleRate,
		TotalPackets:  len(packets),
		Target:        targetVoice,
		Teammates:     teammates,
		Others:        others,
		Limitations: []string{
			"seconds_estimate is packets times 20ms, not measured audio duration",
			"teammates are speakers on the POV side at the last tick they spoke",
			"extracted tracks keep only packets spoken while on the POV side",
			"others are aggregated; the probe does not list non-teammate speakers",
		},
	}, nil
}

func sameSideAt(sightings []Sighting, targetID, speakerID string, tick int) bool {
	_, targetTeam := sightingAt(sightings, targetID, tick)
	_, speakerTeam := sightingAt(sightings, speakerID, tick)
	return targetTeam != "" && targetTeam == speakerTeam
}

func sightingAt(sightings []Sighting, steamID string, tick int) (name, team string) {
	for _, s := range sightings {
		if s.SteamID64 != steamID {
			continue
		}
		if tick > 0 && s.Tick > tick {
			continue
		}
		name, team = s.Name, s.Team
	}
	return name, team
}

func lastSightingByID(sightings []Sighting) map[string]Sighting {
	out := make(map[string]Sighting, len(sightings))
	for _, s := range sightings {
		if s.SteamID64 == "" {
			continue
		}
		out[s.SteamID64] = s
	}
	return out
}

func formatLabel(formats map[string]struct{}) string {
	switch len(formats) {
	case 0:
		return FormatNone
	case 1:
		for format := range formats {
			return format
		}
	}
	return FormatMixed
}

func secondsEstimate(packets int) float64 {
	if packets <= 0 {
		return 0
	}
	return math.Round(float64(packets)*estimatedFrameSeconds*1000) / 1000
}
