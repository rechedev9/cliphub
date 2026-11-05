package mapgeo

import (
	"bytes"
	"encoding/binary"
	"errors"
	"math"
)

var errTruncated = errors.New("unexpected end of data")

// byteReader is a little-endian cursor that latches the first error, so a
// parser can read a whole record and check err once.
type byteReader struct {
	data []byte
	pos  int
	err  error
}

func (r *byteReader) take(n int) []byte {
	if r.err != nil {
		return nil
	}
	if n < 0 || len(r.data)-r.pos < n {
		r.err = errTruncated
		return nil
	}
	out := r.data[r.pos : r.pos+n]
	r.pos += n
	return out
}

func (r *byteReader) u8() uint8 {
	b := r.take(1)
	if b == nil {
		return 0
	}
	return b[0]
}

func (r *byteReader) u16() uint16 {
	b := r.take(2)
	if b == nil {
		return 0
	}
	return binary.LittleEndian.Uint16(b)
}

func (r *byteReader) u32() uint32 {
	b := r.take(4)
	if b == nil {
		return 0
	}
	return binary.LittleEndian.Uint32(b)
}

func (r *byteReader) u64() uint64 {
	b := r.take(8)
	if b == nil {
		return 0
	}
	return binary.LittleEndian.Uint64(b)
}

func (r *byteReader) f32() float32 {
	return math.Float32frombits(r.u32())
}

func (r *byteReader) bytes(n int) []byte {
	b := r.take(n)
	if b == nil {
		return nil
	}
	return bytes.Clone(b)
}

func (r *byteReader) cstring() string {
	if r.err != nil {
		return ""
	}
	end := bytes.IndexByte(r.data[r.pos:], 0)
	if end < 0 {
		r.err = errTruncated
		return ""
	}
	s := string(r.data[r.pos : r.pos+end])
	r.pos += end + 1
	return s
}

// remaining is the number of unread bytes, used to bound counts read from the
// data before allocating for them.
func (r *byteReader) remaining() int {
	return len(r.data) - r.pos
}
