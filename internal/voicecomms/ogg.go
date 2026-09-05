package voicecomms

import (
	"encoding/binary"
	"fmt"
	"io"
)

// 20ms CELT silence @ 48 kHz, mono. ffmpeg/libopus decodes this as digital silence.
var opusSilence20ms = []byte{0xF8, 0xFF, 0xFE}

const opus48kFrameSamples = 960

func WriteOggOpus(w io.Writer, frames [][]byte, sampleRate uint32, serial uint32) error {
	if sampleRate == 0 {
		sampleRate = 48000
	}
	if serial == 0 {
		serial = 1
	}
	head := opusHead(sampleRate)
	if err := writeOggPage(w, 0x02, 0, serial, 0, [][]byte{head}); err != nil {
		return err
	}
	if err := writeOggPage(w, 0x00, 0, serial, 1, [][]byte{opusTags()}); err != nil {
		return err
	}
	granule := int64(0)
	seq := uint32(2)
	for i, frame := range frames {
		if len(frame) == 0 {
			continue
		}
		granule += int64(opusFrameSamples(frame))
		headerType := byte(0)
		if i == len(frames)-1 {
			headerType = 0x04
		}
		if err := writeOggPage(w, headerType, granule, serial, seq, [][]byte{frame}); err != nil {
			return err
		}
		seq++
	}
	return nil
}

func opusHead(sampleRate uint32) []byte {
	buf := make([]byte, 19)
	copy(buf, "OpusHead")
	buf[8] = 1
	buf[9] = 1
	// Pre-skip 0: these are remuxed CS2 frames, not libopus output.
	// A fake encoder delay would make ffmpeg PTS lag the capture ticks.
	binary.LittleEndian.PutUint16(buf[10:12], 0)
	binary.LittleEndian.PutUint32(buf[12:16], sampleRate)
	return buf
}

func opusTags() []byte {
	vendor := []byte("cliphub")
	buf := make([]byte, 8+4+len(vendor)+4)
	copy(buf, "OpusTags")
	binary.LittleEndian.PutUint32(buf[8:12], uint32(len(vendor))) // #nosec G115 -- vendor is the 7-byte "cliphub" literal
	copy(buf[12:], vendor)
	return buf
}

func writeOggPage(w io.Writer, headerType byte, granule int64, serial, seq uint32, packets [][]byte) error {
	var segs []byte
	var body []byte
	for _, pkt := range packets {
		body = append(body, pkt...)
		n := len(pkt)
		for n >= 255 {
			segs = append(segs, 255)
			n -= 255
		}
		segs = append(segs, byte(n)) // #nosec G115 -- remainder n is < 255 after the 255-chunk loop
	}
	if len(segs) > 255 {
		return fmt.Errorf("ogg page has too many segments")
	}
	page := make([]byte, 27+len(segs)+len(body))
	copy(page[0:4], "OggS")
	page[5] = headerType
	// A page granule is a sample position; negative values would serialize as a
	// huge unsigned marker and poison the Ogg timestamp stream.
	if granule < 0 {
		return fmt.Errorf("ogg page granule must be non-negative: %d", granule)
	}
	binary.LittleEndian.PutUint64(page[6:14], uint64(granule))
	binary.LittleEndian.PutUint32(page[14:18], serial)
	binary.LittleEndian.PutUint32(page[18:22], seq)
	page[26] = byte(len(segs)) // #nosec G115 -- len(segs) <= 255 enforced above
	copy(page[27:], segs)
	copy(page[27+len(segs):], body)
	crc := oggCRC(page)
	binary.LittleEndian.PutUint32(page[22:26], crc)
	n, err := w.Write(page)
	if err == nil && n != len(page) {
		return io.ErrShortWrite
	}
	return err
}

func opusFrameSamples(frame []byte) int {
	if len(frame) == 0 {
		return 0
	}
	count := 1
	switch frame[0] & 3 {
	case 1, 2:
		count = 2
	case 3:
		if len(frame) < 2 {
			return 0
		}
		count = int(frame[1] & 63)
	}
	samples := count * opusTOCSamples(frame[0])
	if samples > 5760 {
		return 0
	}
	return samples
}

func opusTOCSamples(toc byte) int {
	config := toc >> 3
	switch {
	case config < 12:
		// SILK: 10, 20, 40, 60 ms cycling every 4 configs
		return []int{480, 960, 1920, 2880}[config%4]
	case config < 16:
		return 480 << (config & 1)
	default:
		return []int{120, 240, 480, 960}[(config-16)%4]
	}
}

var oggCRCTable = func() [256]uint32 {
	var table [256]uint32
	for i := range table {
		r := uint32(i) << 24
		for j := 0; j < 8; j++ {
			if r&0x80000000 != 0 {
				r = (r << 1) ^ 0x04c11db7
			} else {
				r <<= 1
			}
		}
		table[i] = r
	}
	return table
}()

func oggCRC(page []byte) uint32 {
	var crc uint32
	for i, b := range page {
		if i >= 22 && i < 26 {
			b = 0
		}
		crc = (crc << 8) ^ oggCRCTable[byte(crc>>24)^b]
	}
	return crc
}
