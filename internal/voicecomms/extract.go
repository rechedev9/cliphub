package voicecomms

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
)

func ExtractFile(demoPath, target, dir string) (Index, Report, error) {
	report, packets, sightings, err := CollectFile(demoPath, target)
	if err != nil {
		return Index{}, Report{}, err
	}
	index, err := WriteTracks(dir, report, packets, sightings)
	return index, report, err
}

func WriteTracks(dir string, report Report, packets []Packet, sightings []Sighting) (Index, error) {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return Index{}, fmt.Errorf("create voice dir: %w", err)
	}
	byXUID := map[uint64][]Packet{}
	for _, pkt := range packets {
		if pkt.Format != FormatOpus || len(pkt.Data) == 0 {
			continue
		}
		speaker := strconv.FormatUint(pkt.XUID, 10)
		if !sameSideAt(sightings, report.Target.SteamID64, speaker, pkt.Tick) {
			continue
		}
		byXUID[pkt.XUID] = append(byXUID[pkt.XUID], pkt)
	}
	sampleRate := report.SampleRateHz
	if sampleRate == 0 {
		sampleRate = 48000
	}
	tickrate := report.Tickrate
	if tickrate <= 0 {
		tickrate = 64
	}

	identity := map[string]PlayerVoice{report.Target.SteamID64: report.Target}
	for _, mate := range report.Teammates {
		identity[mate.SteamID64] = mate
	}

	index := Index{
		SchemaVersion: SchemaVersion,
		Demo:          report.Demo,
		Tickrate:      tickrate,
		SampleRateHz:  sampleRate,
		Tracks:        []Track{},
	}
	ids := make([]uint64, 0, len(byXUID))
	for id := range byXUID {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })

	for _, id := range ids {
		pkts := byXUID[id]
		sort.Slice(pkts, func(i, j int) bool { return pkts[i].Tick < pkts[j].Tick })
		sid := strconv.FormatUint(id, 10)
		rel := sid + ".ogg"
		abs := filepath.Join(dir, rel)
		// #nosec G304 -- path is dir + steamid.
		f, err := os.OpenFile(abs, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600)
		if err != nil {
			return Index{}, fmt.Errorf("create track %s: %w", rel, err)
		}
		frames := timelineFrames(pkts, tickrate, 0)
		writeErr := WriteOggOpus(f, frames, sampleRate, uint32(id&0xffffffff))
		closeErr := f.Close()
		if writeErr != nil {
			return Index{}, fmt.Errorf("write track %s: %w", rel, writeErr)
		}
		if closeErr != nil {
			return Index{}, fmt.Errorf("close track %s: %w", rel, closeErr)
		}
		info := identity[sid]
		index.Tracks = append(index.Tracks, Track{
			SteamID64: sid,
			Name:      info.Name,
			Team:      info.Team,
			Path:      abs,
			Packets:   len(pkts),
			FirstTick: pkts[0].Tick,
			LastTick:  pkts[len(pkts)-1].Tick,
		})
	}

	indexPath := filepath.Join(dir, "voice-index.json")
	body, err := marshalIndex(index)
	if err != nil {
		return Index{}, err
	}
	if err := os.WriteFile(indexPath, body, 0o600); err != nil {
		return Index{}, fmt.Errorf("write voice index: %w", err)
	}
	return index, nil
}

func timelineFrames(packets []Packet, tickrate, fromTick int) [][]byte {
	if tickrate <= 0 {
		tickrate = 64
	}
	samplesPerTick := 48000.0 / float64(tickrate)
	cursor := float64(fromTick) * samplesPerTick
	var frames [][]byte
	for _, pkt := range packets {
		target := float64(pkt.Tick) * samplesPerTick
		if target > cursor {
			gap := int((target - cursor) / float64(opus48kFrameSamples))
			frames = append(frames, silenceFramesN(gap)...)
			cursor += float64(gap * opus48kFrameSamples)
		}
		opus := splitVoiceFrames(pkt.Data, pkt.Offsets)
		frames = append(frames, opus...)
		for _, frame := range opus {
			cursor += float64(opusFrameSamples(frame))
		}
		if len(opus) == 0 {
			cursor += float64(opus48kFrameSamples)
		}
	}
	return frames
}

func silenceFramesN(n int) [][]byte {
	if n <= 0 {
		return nil
	}
	if n > 50*60*30 {
		n = 50 * 60 * 30
	}
	out := make([][]byte, n)
	for i := range out {
		out[i] = opusSilence20ms
	}
	return out
}

func marshalIndex(index Index) ([]byte, error) {
	body, err := json.MarshalIndent(index, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("encode voice index: %w", err)
	}
	return append(body, '\n'), nil
}
