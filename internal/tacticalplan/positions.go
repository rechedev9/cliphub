package tacticalplan

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"math"
)

// PositionsFormat names the sidecar blob layout. It versions independently
// from the JSON schema: the two evolve for different reasons.
const PositionsFormat = "zvpos1"

// PositionsHeaderSize is the fixed header length. A reader that fetches one
// round's byte range still needs the header to decode it, so the size is part
// of the format's public contract rather than a number callers restate.
const PositionsHeaderSize = positionsHeaderSize

const (
	positionsMagic      = "ZVPOS1"
	positionsVersion    = 1
	positionsHeaderSize = 32
	positionsSampleSize = 10
	positionsFrameHead  = 6 // int32 tick + uint16 present mask
	maxSlots            = 16
)

// quantumLadder lists the world-units-per-step values tried in order. 0.25 is
// far below one radar pixel on every official map; the coarser steps exist so
// a map with an extreme altitude range (Vertigo sits near z=11700) still fits
// the int16 sample encoding instead of silently wrapping.
var quantumLadder = []float64{0.25, 0.5, 1, 2, 4, 8}

// SampleFlags are the per-sample boolean facts packed into one byte.
type SampleFlags uint8

// Sample flag bits. FlagSideT marks the terrorist side so a reader can colour a
// dot without consulting the round's player list.
const (
	FlagAlive SampleFlags = 1 << iota
	FlagBlinded
	FlagDucking
	FlagScoped
	FlagHasBomb
	FlagDefusing
	FlagAirborne
	FlagSideT
)

// Has reports whether every bit in mask is set.
func (f SampleFlags) Has(mask SampleFlags) bool { return f&mask == mask }

// Sample is one player's state at one sampled tick. It is normally read from
// the binary blob, but the per-round HTTP endpoint serves decoded frames, so
// the JSON names are part of the contract and follow the rest of the document.
type Sample struct {
	Slot   uint8       `json:"slot"`
	X      float64     `json:"x"`
	Y      float64     `json:"y"`
	Z      float64     `json:"z"`
	Yaw    float64     `json:"yaw"` // degrees counter-clockwise from +X, as CS2 reports it
	Health int         `json:"health"`
	Flags  SampleFlags `json:"flags"`
}

// Frame is every sampled player at one tick.
type Frame struct {
	Tick    int      `json:"tick"`
	Samples []Sample `json:"samples"`
}

// RoundFrames groups a round's frames for encoding, so the blob can be seeked
// per round without decoding what comes before it.
type RoundFrames struct {
	Round  int     `json:"round"`
	Frames []Frame `json:"frames"`
}

// Positions describes the sidecar blob: how it was sampled, how to decode it,
// and how to seek into it. It travels inside the JSON document; the bytes it
// describes do not.
type Positions struct {
	Format      string     `json:"format"`
	HZ          float64    `json:"hz"`
	SampleTicks int        `json:"sample_ticks"`
	Quantum     float64    `json:"quantum"`
	Origin      [3]float64 `json:"origin"`
	SlotCount   int        `json:"slot_count"`
	FrameCount  int        `json:"frame_count"`
	ByteLength  int64      `json:"byte_length"`
	// SHA256 binds the index to its blob. A reader that finds a mismatch must
	// treat the analysis as stale and re-run it rather than draw half of it.
	SHA256       string        `json:"sha256"`
	RoundOffsets []RoundOffset `json:"round_offsets"`
}

// RoundOffset locates one round's frames inside the blob.
type RoundOffset struct {
	Round      int   `json:"round"`
	ByteOffset int64 `json:"byte_offset"`
	ByteLength int64 `json:"byte_length"`
	FrameCount int   `json:"frame_count"`
	FirstTick  int   `json:"first_tick"`
	LastTick   int   `json:"last_tick"`
}

// Offset returns the descriptor for a round number.
func (p Positions) Offset(round int) (RoundOffset, bool) {
	for _, o := range p.RoundOffsets {
		if o.Round == round {
			return o, true
		}
	}
	return RoundOffset{}, false
}

// Blob is an encoded position stream and the descriptor that decodes it.
type Blob struct {
	Data       []byte
	Descriptor Positions
}

// EncodePositions packs per-round frames into the zvpos1 blob. Quantisation is
// chosen from the observed extent so the encoding stays lossless to within one
// quantum on every map, and the choice is recorded in the descriptor.
func EncodePositions(rounds []RoundFrames, sampleTicks int, tickrate float64) (Blob, error) {
	if sampleTicks <= 0 {
		return Blob{}, fmt.Errorf("encode positions: sample ticks %d must be positive", sampleTicks)
	}
	if sampleTicks > math.MaxUint16 {
		return Blob{}, fmt.Errorf("encode positions: sample ticks %d exceed the %d-tick header limit", sampleTicks, math.MaxUint16)
	}
	origin, quantum, slotCount, err := quantize(rounds)
	if err != nil {
		return Blob{}, err
	}
	if slotCount > maxSlots {
		return Blob{}, fmt.Errorf("encode positions: slot count %d exceeds the %d-slot encoding", slotCount, maxSlots)
	}

	frameCount := 0
	for _, r := range rounds {
		if len(r.Frames) > math.MaxUint32-frameCount {
			return Blob{}, fmt.Errorf("encode positions: frame count exceeds the %d-frame header limit", uint64(math.MaxUint32))
		}
		frameCount += len(r.Frames)
	}

	buf := make([]byte, positionsHeaderSize, positionsHeaderSize+frameCount*(positionsFrameHead+positionsSampleSize*slotCount))
	copy(buf, positionsMagic)
	binary.LittleEndian.PutUint16(buf[6:], positionsVersion)
	// #nosec G115 -- slotCount and sampleTicks are validated against their wire widths above.
	binary.LittleEndian.PutUint16(buf[8:], uint16(slotCount))
	// #nosec G115 -- slotCount and sampleTicks are validated against their wire widths above.
	binary.LittleEndian.PutUint16(buf[10:], uint16(sampleTicks))
	binary.LittleEndian.PutUint32(buf[12:], math.Float32bits(float32(quantum)))
	binary.LittleEndian.PutUint32(buf[16:], math.Float32bits(float32(origin[0])))
	binary.LittleEndian.PutUint32(buf[20:], math.Float32bits(float32(origin[1])))
	binary.LittleEndian.PutUint32(buf[24:], math.Float32bits(float32(origin[2])))
	binary.LittleEndian.PutUint32(buf[28:], uint32(frameCount))

	offsets := make([]RoundOffset, 0, len(rounds))
	for _, r := range rounds {
		start := int64(len(buf))
		offset := RoundOffset{Round: r.Round, ByteOffset: start, FrameCount: len(r.Frames)}
		for i, f := range r.Frames {
			if i == 0 {
				offset.FirstTick = f.Tick
			}
			offset.LastTick = f.Tick
			buf, err = appendFrame(buf, f, origin, quantum)
			if err != nil {
				return Blob{}, fmt.Errorf("encode round %d: %w", r.Round, err)
			}
		}
		offset.ByteLength = int64(len(buf)) - start
		offsets = append(offsets, offset)
	}

	sum := sha256.Sum256(buf)
	hz := 0.0
	if tickrate > 0 {
		hz = tickrate / float64(sampleTicks)
	}
	return Blob{
		Data: buf,
		Descriptor: Positions{
			Format:       PositionsFormat,
			HZ:           hz,
			SampleTicks:  sampleTicks,
			Quantum:      quantum,
			Origin:       origin,
			SlotCount:    slotCount,
			FrameCount:   frameCount,
			ByteLength:   int64(len(buf)),
			SHA256:       hex.EncodeToString(sum[:]),
			RoundOffsets: offsets,
		},
	}, nil
}

func appendFrame(buf []byte, f Frame, origin [3]float64, quantum float64) ([]byte, error) {
	if f.Tick < math.MinInt32 || f.Tick > math.MaxInt32 {
		return nil, fmt.Errorf("tick %d is outside the int32 position encoding", f.Tick)
	}
	var mask uint16
	for _, s := range f.Samples {
		if s.Slot >= maxSlots {
			return nil, fmt.Errorf("slot %d exceeds the %d-slot encoding", s.Slot, maxSlots)
		}
		if mask&(1<<s.Slot) != 0 {
			return nil, fmt.Errorf("slot %d appears twice in the frame at tick %d", s.Slot, f.Tick)
		}
		mask |= 1 << s.Slot
	}

	// #nosec G115 -- the signed tick is range-checked above and its two's-complement bits are the wire format.
	buf = binary.LittleEndian.AppendUint32(buf, uint32(int32(f.Tick)))
	buf = binary.LittleEndian.AppendUint16(buf, mask)
	// Samples are written in ascending slot order so the mask alone tells a
	// decoder which slot each fixed-size record belongs to.
	for slot := uint8(0); slot < maxSlots; slot++ {
		if mask&(1<<slot) == 0 {
			continue
		}
		s := sampleForSlot(f.Samples, slot)
		buf = appendSigned16(buf, quantizeAxis(s.X, origin[0], quantum))
		buf = appendSigned16(buf, quantizeAxis(s.Y, origin[1], quantum))
		buf = appendSigned16(buf, quantizeAxis(s.Z, origin[2], quantum))
		buf = binary.LittleEndian.AppendUint16(buf, encodeYaw(s.Yaw))
		buf = append(buf, clampHealth(s.Health), byte(s.Flags))
	}
	return buf, nil
}

func sampleForSlot(samples []Sample, slot uint8) Sample {
	for _, s := range samples {
		if s.Slot == slot {
			return s
		}
	}
	return Sample{Slot: slot}
}

// DecodePositions decodes a whole blob. It is the reference decoder the
// TypeScript one is tested against.
func DecodePositions(data []byte) (Positions, []Frame, error) {
	desc, err := DecodeHeader(data)
	if err != nil {
		return Positions{}, nil, err
	}
	frames, err := DecodeFrames(data, positionsHeaderSize, desc.FrameCount, desc)
	if err != nil {
		return Positions{}, nil, err
	}
	return desc, frames, nil
}

// DecodeHeader reads the fixed header. The returned descriptor carries no round
// offsets: those live in the JSON document, which is the index into the blob.
func DecodeHeader(data []byte) (Positions, error) {
	if len(data) < positionsHeaderSize {
		return Positions{}, fmt.Errorf("decode positions: %d bytes is shorter than the %d-byte header", len(data), positionsHeaderSize)
	}
	if string(data[:6]) != positionsMagic {
		return Positions{}, fmt.Errorf("decode positions: bad magic %q", data[:6])
	}
	if v := binary.LittleEndian.Uint16(data[6:]); v != positionsVersion {
		return Positions{}, fmt.Errorf("decode positions: unsupported blob version %d", v)
	}
	slotCount := int(binary.LittleEndian.Uint16(data[8:]))
	if slotCount > maxSlots {
		return Positions{}, fmt.Errorf("decode positions: slot count %d exceeds %d", slotCount, maxSlots)
	}
	quantum := float64(math.Float32frombits(binary.LittleEndian.Uint32(data[12:])))
	if quantum <= 0 {
		return Positions{}, fmt.Errorf("decode positions: quantum %v must be positive", quantum)
	}
	return Positions{
		Format:      PositionsFormat,
		SampleTicks: int(binary.LittleEndian.Uint16(data[10:])),
		Quantum:     quantum,
		Origin: [3]float64{
			float64(math.Float32frombits(binary.LittleEndian.Uint32(data[16:]))),
			float64(math.Float32frombits(binary.LittleEndian.Uint32(data[20:]))),
			float64(math.Float32frombits(binary.LittleEndian.Uint32(data[24:]))),
		},
		SlotCount:  slotCount,
		FrameCount: int(binary.LittleEndian.Uint32(data[28:])),
		ByteLength: int64(len(data)),
	}, nil
}

// DecodeFrames decodes frameCount frames starting at byteOffset, which is what
// a RoundOffset gives a caller that wants one round and nothing else.
func DecodeFrames(data []byte, byteOffset int64, frameCount int, desc Positions) ([]Frame, error) {
	if desc.Quantum <= 0 {
		return nil, fmt.Errorf("decode frames: descriptor has no quantum")
	}
	if byteOffset < positionsHeaderSize || byteOffset > int64(len(data)) {
		return nil, fmt.Errorf("decode frames: offset %d is outside the %d-byte blob", byteOffset, len(data))
	}
	frames := make([]Frame, 0, frameCount)
	pos := int(byteOffset)
	for i := 0; i < frameCount; i++ {
		if pos+positionsFrameHead > len(data) {
			return nil, fmt.Errorf("decode frames: truncated frame header at byte %d", pos)
		}
		tick := int(decodeSigned32(data[pos:]))
		mask := binary.LittleEndian.Uint16(data[pos+4:])
		pos += positionsFrameHead

		present := 0
		for slot := 0; slot < maxSlots; slot++ {
			if mask&(1<<slot) != 0 {
				present++
			}
		}
		if pos+present*positionsSampleSize > len(data) {
			return nil, fmt.Errorf("decode frames: truncated samples at byte %d", pos)
		}
		frame := Frame{Tick: tick, Samples: make([]Sample, 0, present)}
		for slot := uint8(0); slot < maxSlots; slot++ {
			if mask&(1<<slot) == 0 {
				continue
			}
			frame.Samples = append(frame.Samples, Sample{
				Slot:   slot,
				X:      dequantizeAxis(decodeSigned16(data[pos:]), desc.Origin[0], desc.Quantum),
				Y:      dequantizeAxis(decodeSigned16(data[pos+2:]), desc.Origin[1], desc.Quantum),
				Z:      dequantizeAxis(decodeSigned16(data[pos+4:]), desc.Origin[2], desc.Quantum),
				Yaw:    decodeYaw(binary.LittleEndian.Uint16(data[pos+6:])),
				Health: int(data[pos+8]),
				Flags:  SampleFlags(data[pos+9]),
			})
			pos += positionsSampleSize
		}
		frames = append(frames, frame)
	}
	return frames, nil
}

// quantize picks the origin and step that keep every sample inside int16, and
// reports the highest slot index in use.
func quantize(rounds []RoundFrames) ([3]float64, float64, int, error) {
	min := [3]float64{math.Inf(1), math.Inf(1), math.Inf(1)}
	max := [3]float64{math.Inf(-1), math.Inf(-1), math.Inf(-1)}
	highest := -1
	seen := false
	for _, r := range rounds {
		for _, f := range r.Frames {
			for _, s := range f.Samples {
				seen = true
				if int(s.Slot) > highest {
					highest = int(s.Slot)
				}
				for axis, v := range [3]float64{s.X, s.Y, s.Z} {
					if v < min[axis] {
						min[axis] = v
					}
					if v > max[axis] {
						max[axis] = v
					}
				}
			}
		}
	}
	if !seen {
		// An empty stream still needs a decodable header.
		return [3]float64{}, quantumLadder[0], 0, nil
	}

	var origin [3]float64
	span := 0.0
	for axis := 0; axis < 3; axis++ {
		origin[axis] = math.Round((min[axis] + max[axis]) / 2)
		if s := max[axis] - min[axis]; s > span {
			span = s
		}
	}
	const int16Span = 65534.0 // one step of headroom against rounding at the edges
	for _, q := range quantumLadder {
		if span/q <= int16Span {
			return origin, q, highest + 1, nil
		}
	}
	return [3]float64{}, 0, 0, fmt.Errorf("encode positions: coordinate span %.0f units does not fit the sample encoding", span)
}

func quantizeAxis(v, origin, quantum float64) int16 {
	steps := math.Round((v - origin) / quantum)
	if steps > math.MaxInt16 {
		steps = math.MaxInt16
	}
	if steps < math.MinInt16 {
		steps = math.MinInt16
	}
	return int16(steps)
}

func dequantizeAxis(v int16, origin, quantum float64) float64 {
	return origin + float64(v)*quantum
}

func appendSigned16(buf []byte, value int16) []byte {
	// #nosec G115 -- converting the signed sample to its two's-complement bits is the wire format.
	return binary.LittleEndian.AppendUint16(buf, uint16(value))
}

func decodeSigned16(data []byte) int16 {
	// #nosec G115 -- the wire value is a two's-complement signed coordinate.
	return int16(binary.LittleEndian.Uint16(data))
}

func decodeSigned32(data []byte) int32 {
	// #nosec G115 -- the wire value is a two's-complement signed demo tick.
	return int32(binary.LittleEndian.Uint32(data))
}

// encodeYaw maps degrees onto the full uint16 range, giving ~0.0055° of
// resolution and making wraparound free.
func encodeYaw(deg float64) uint16 {
	if math.IsNaN(deg) || math.IsInf(deg, 0) {
		return 0
	}
	norm := math.Mod(deg, 360)
	if norm < 0 {
		norm += 360
	}
	// Rounding can land on 65536 just below 360°, which is 0 after wrapping.
	return uint16(uint32(math.Round(norm*65536/360)) & 0xFFFF)
}

func decodeYaw(v uint16) float64 { return float64(v) * 360 / 65536 }

func clampHealth(h int) uint8 {
	switch {
	case h < 0:
		return 0
	case h > 255:
		return 255
	default:
		return uint8(h)
	}
}
