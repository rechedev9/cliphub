package voicecomms

import (
	"context"
	"fmt"
	"io"
)

// writeTimelineOgg streams gaps and packets with bounded memory. A long source
// silence remains a long silence; it is never capped or removed from the clock.
func writeTimelineOgg(ctx context.Context, w io.Writer, packets []indexedPacket, spill *packetSpill, tickrate int, sampleRate, serial uint32) error {
	if tickrate <= 0 || tickrate > 1024 {
		return fmt.Errorf("voice timeline requires a supported source tickrate")
	}
	if sampleRate == 0 {
		sampleRate = 48000
	}
	if serial == 0 {
		serial = 1
	}
	if err := writeOggPage(w, 2, 0, serial, 0, [][]byte{opusHead(sampleRate)}); err != nil {
		return err
	}
	if err := writeOggPage(w, 0, 0, serial, 1, [][]byte{opusTags()}); err != nil {
		return err
	}
	var cursor int64
	seq := uint32(2)
	var pending []byte
	flush := func(frame []byte, last bool) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		samples := opusFrameSamples(frame)
		if samples <= 0 {
			return fmt.Errorf("invalid Opus packet duration")
		}
		cursor += int64(samples)
		kind := byte(0)
		if last {
			kind = 4
		}
		if err := writeOggPage(w, kind, cursor, serial, seq, [][]byte{frame}); err != nil {
			return err
		}
		seq++
		return nil
	}
	write := func(frame []byte) error {
		if pending != nil {
			if err := flush(pending, false); err != nil {
				return err
			}
		}
		pending = frame
		return nil
	}
	for _, item := range packets {
		if err := ctx.Err(); err != nil {
			return err
		}
		pkt := item.packet
		if pkt.Tick < 0 || int64(pkt.Tick) > int64(tickrate)*43200 {
			return fmt.Errorf("voice timestamp exceeds supported duration")
		}
		if len(pkt.Data) == 0 && spill != nil {
			var err error
			pkt.Data, pkt.Offsets, err = spill.payload(item.index)
			if err != nil {
				return err
			}
		}
		target := int64(pkt.Tick) * 48000 / int64(tickrate)
		current := cursor
		if pending != nil {
			current += int64(opusFrameSamples(pending))
		}
		for target-current >= opus48kFrameSamples {
			if err := write(opusSilence20ms); err != nil {
				return err
			}
			current += opus48kFrameSamples
		}
		frames := splitVoiceFrames(pkt.Data, pkt.Offsets)
		if len(frames) == 0 {
			return fmt.Errorf("voice packet contains no Opus frame")
		}
		for _, frame := range frames {
			if err := write(frame); err != nil {
				return err
			}
		}
	}
	if pending != nil {
		return flush(pending, true)
	}
	return nil
}
