package voicecomms

import (
	"slices"
	"strconv"
)

// SpeechByPlayer is when each speaker's in-game voice played, in ticks, by SteamID64.
// Team voice sync compares it with an outside recording; packets of other codecs count too.
func SpeechByPlayer(packets []Packet, tickrate int) map[string][]ActivityRange {
	out := map[string][]ActivityRange{}
	if tickrate <= 0 {
		return out
	}
	for _, p := range packets {
		if p.Tick <= 0 || (p.Bytes == 0 && len(p.Data) == 0) {
			continue
		}
		samples := p.DurationSamples
		if samples == 0 && p.Format == FormatOpus {
			for _, frame := range splitVoiceFrames(p.Data, p.Offsets) {
				samples += opusFrameSamples(frame)
			}
		}
		if samples <= 0 {
			// A packet of an unknown codec carries about 20 ms of voice.
			samples = 960
		}
		id := strconv.FormatUint(p.XUID, 10)
		out[id] = append(out[id], ActivityRange{StartTick: p.Tick, EndTick: p.Tick + max(1, (samples*tickrate+47999)/48000)})
	}
	for id, ranges := range out {
		slices.SortFunc(ranges, func(a, b ActivityRange) int { return a.StartTick - b.StartTick })
		merged := ranges[:0]
		for _, r := range ranges {
			if n := len(merged); n > 0 && r.StartTick <= merged[n-1].EndTick {
				merged[n-1].EndTick = max(merged[n-1].EndTick, r.EndTick)
				continue
			}
			merged = append(merged, r)
		}
		out[id] = merged
	}
	return out
}
