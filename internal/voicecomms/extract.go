package voicecomms

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"

	demoinfocs "github.com/markus-wa/demoinfocs-golang/v5/pkg/demoinfocs"
)

func ExtractFile(demoPath, target, dir string) (Index, Report, error) {
	result, err := extractFile(context.Background(), demoPath, target, dir, false)
	return result.Index, result.Report, err
}

func extractFile(ctx context.Context, demoPath, target, dir string, strict bool) (ExtractionResult, error) {
	abs, err := filepath.Abs(demoPath)
	if err != nil {
		return ExtractionResult{}, fmt.Errorf("resolve demo path: %w", err)
	}
	// #nosec G304 -- demo path is an explicit local CLI input.
	f, err := os.Open(abs)
	if err != nil {
		return ExtractionResult{}, fmt.Errorf("open demo: %w", err)
	}
	defer f.Close()

	spill, err := newPacketSpill(filepath.Join(dir, ".voice-spill"))
	if err != nil {
		return ExtractionResult{}, err
	}
	defer func() {
		_ = spill.Close()
		_ = os.RemoveAll(filepath.Join(dir, ".voice-spill"))
	}()

	p := demoinfocs.NewParser(f)
	defer p.Close()
	report, packets, sightings, err := collectContext(ctx, p, target, abs, spill)
	if err != nil {
		return ExtractionResult{Availability: ExtractionFailed}, err
	}
	result := classifyExtraction(report, packets, sightings)
	if strict && (result.Availability == UnsupportedCodec || result.Availability == InvalidTimeline) {
		return result, nil
	}
	result.Index, err = writeTracksWithSpillContext(ctx, dir, report, packets, sightings, spill)
	if err != nil {
		result.Availability = ExtractionFailed
	}
	return result, err
}

func WriteTracks(dir string, report Report, packets []Packet, sightings []Sighting) (Index, error) {
	return writeTracksWithSpill(dir, report, packets, sightings, nil)
}

func writeTracksWithSpill(dir string, report Report, packets []Packet, sightings []Sighting, spill *packetSpill) (Index, error) {
	return writeTracksWithSpillContext(context.Background(), dir, report, packets, sightings, spill)
}

func writeTracksWithSpillContext(ctx context.Context, dir string, report Report, packets []Packet, sightings []Sighting, spill *packetSpill) (Index, error) {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return Index{}, fmt.Errorf("create voice dir: %w", err)
	}
	byXUID := map[uint64][]indexedPacket{}
	for i, pkt := range packets {
		if err := ctx.Err(); err != nil {
			return Index{}, err
		}
		if pkt.Format != FormatOpus {
			continue
		}
		if pkt.Bytes == 0 && len(pkt.Data) > 0 {
			pkt.Bytes = len(pkt.Data)
		}
		if len(pkt.Data) == 0 && pkt.Bytes == 0 {
			continue
		}
		speaker := strconv.FormatUint(pkt.XUID, 10)
		if !sameSideAt(sightings, report.Target.SteamID64, speaker, pkt.Tick) {
			continue
		}
		byXUID[pkt.XUID] = append(byXUID[pkt.XUID], indexedPacket{index: i, packet: pkt})
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
		if err := ctx.Err(); err != nil {
			return Index{}, err
		}
		pkts := byXUID[id]
		sort.Slice(pkts, func(i, j int) bool { return pkts[i].packet.Tick < pkts[j].packet.Tick })
		sid := strconv.FormatUint(id, 10)
		rel := sid + ".ogg"
		abs := filepath.Join(dir, rel)
		// #nosec G304 -- path is dir + steamid.
		out, err := os.OpenFile(abs, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600)
		if err != nil {
			return Index{}, fmt.Errorf("create track %s: %w", rel, err)
		}
		writeErr := writeTimelineOgg(ctx, out, pkts, spill, tickrate, sampleRate, uint32(id&0xffffffff))
		closeErr := out.Close()
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
			FirstTick: pkts[0].packet.Tick,
			LastTick:  pkts[len(pkts)-1].packet.Tick,
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

type indexedPacket struct {
	index  int
	packet Packet
}

func marshalIndex(index Index) ([]byte, error) {
	body, err := json.MarshalIndent(index, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("encode voice index: %w", err)
	}
	return append(body, '\n'), nil
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
	out := make([][]byte, n)
	for i := range out {
		out[i] = opusSilence20ms
	}
	return out
}
