// Package demostart writes the capture copy of a demo that CS2 can start
// without its playdemo rewind to demo tick 0.
//
// CS2 demos open with signon records that carry no tick, then a Packet and a
// FullPacket at demo tick 1. Playback always rewinds to demo tick 0, and on
// some demos that rewind crashes CS2 before round 1. The copy moves the two
// opening records to tick 0, so the rewind has nothing to skip; every other
// byte, and the file size, stay the same.
package demostart

import (
	"bufio"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
)

// ErrLayout marks a demo whose opening records are not the layout the copy patches.
var ErrLayout = errors.New("demo does not open with a Packet and a FullPacket at demo tick 1")

const (
	magic          = "PBDEMS2\x00"
	headerBytes    = 16
	demPacket      = 7
	demFullPacket  = 13
	demCompressed  = 64
	signonTick     = 0xFFFFFFFF
	maxSignonCount = 64
)

// TickOneOffsets returns the file offsets of the tick fields of the first
// Packet and FullPacket. Both must be one-byte ticks of 1, right after signon
// records that carry no tick; any other opening returns ErrLayout.
func TickOneOffsets(path string) ([2]int64, error) {
	// #nosec G304 -- path is the local capture demo the caller already validated.
	file, err := os.Open(path)
	if err != nil {
		return [2]int64{}, err
	}
	defer file.Close()
	r := &recordReader{r: bufio.NewReader(file)}
	head := make([]byte, headerBytes)
	if _, err := io.ReadFull(r, head); err != nil || string(head[:len(magic)]) != magic {
		return [2]int64{}, fmt.Errorf("%w: not a Source 2 demo", ErrLayout)
	}
	var packet *recordHeader
	for i := 0; ; i++ {
		h, err := r.header()
		if err != nil {
			return [2]int64{}, fmt.Errorf("%w: %v", ErrLayout, err)
		}
		switch {
		case packet != nil && h.tickOne(demFullPacket):
			return [2]int64{packet.tickOffset, h.tickOffset}, nil
		case packet != nil:
			return [2]int64{}, fmt.Errorf("%w: the first Packet is followed by type %d at tick %d", ErrLayout, h.kind, int32(h.tick))
		case h.tickOne(demPacket):
			packet = &h
		case h.tick != signonTick || h.kind == demPacket || h.kind == demFullPacket:
			return [2]int64{}, fmt.Errorf("%w: record %d is type %d at tick %d", ErrLayout, i, h.kind, int32(h.tick))
		case i == maxSignonCount:
			return [2]int64{}, fmt.Errorf("%w: no Packet within the first %d records", ErrLayout, maxSignonCount)
		}
		if err := r.skip(h.size); err != nil {
			return [2]int64{}, fmt.Errorf("%w: %v", ErrLayout, err)
		}
	}
}

// WriteTickZeroCopy copies src to a new file dst with the first Packet and
// FullPacket moved from demo tick 1 to demo tick 0.
func WriteTickZeroCopy(src, dst string) (retErr error) {
	offsets, err := TickOneOffsets(src)
	if err != nil {
		return err
	}
	// #nosec G304 -- src is the local capture demo the caller already validated.
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	// #nosec G304 -- dst is a fresh path the caller owns.
	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return err
	}
	defer func() {
		retErr = errors.Join(retErr, out.Close())
		if retErr != nil {
			_ = os.Remove(dst)
		}
	}()
	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	for _, offset := range offsets {
		if _, err := out.WriteAt([]byte{0}, offset); err != nil {
			return err
		}
	}
	return nil
}

type recordHeader struct {
	kind, tick, size uint32
	tickOffset       int64
	tickBytes        int64
}

// tickOne reports a record of kind at demo tick 1 whose tick is one byte, so
// overwriting that byte with 0 moves it to tick 0 without resizing the file.
func (h recordHeader) tickOne(kind uint32) bool {
	return h.kind == kind && h.tick == 1 && h.tickBytes == 1
}

// recordReader tracks the file offset of everything it reads, so a header can
// report where its tick field sits.
type recordReader struct {
	r      *bufio.Reader
	offset int64
}

func (r *recordReader) Read(p []byte) (int, error) {
	n, err := r.r.Read(p)
	r.offset += int64(n)
	return n, err
}

func (r *recordReader) ReadByte() (byte, error) {
	b, err := r.r.ReadByte()
	if err == nil {
		r.offset++
	}
	return b, err
}

func (r *recordReader) skip(n uint32) error {
	skipped, err := r.r.Discard(int(n))
	r.offset += int64(skipped)
	return err
}

func (r *recordReader) header() (recordHeader, error) {
	command, err := r.varint()
	if err != nil {
		return recordHeader{}, err
	}
	h := recordHeader{kind: command &^ demCompressed, tickOffset: r.offset}
	if h.tick, err = r.varint(); err != nil {
		return recordHeader{}, err
	}
	h.tickBytes = r.offset - h.tickOffset
	if h.size, err = r.varint(); err != nil {
		return recordHeader{}, err
	}
	return h, nil
}

// varint reads a protobuf varint32, the encoding of every record header field.
func (r *recordReader) varint() (uint32, error) {
	value, err := binary.ReadUvarint(r)
	if err != nil {
		return 0, err
	}
	if value > math.MaxUint32 {
		return 0, errors.New("varint larger than 32 bits")
	}
	return uint32(value), nil
}
