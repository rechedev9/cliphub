package tacticalplan

import (
	"encoding/binary"
	"math"
	"testing"
)

func sampleRounds() []RoundFrames {
	return []RoundFrames{
		{Round: 1, Frames: []Frame{
			{Tick: 100, Samples: []Sample{
				{Slot: 0, X: -1500.25, Y: -600.5, Z: -180, Yaw: 90, Health: 100, Flags: FlagAlive},
				{Slot: 3, X: 250, Y: 1200.75, Z: -160, Yaw: 271.5, Health: 56, Flags: FlagAlive | FlagSideT},
			}},
			{Tick: 108, Samples: []Sample{
				{Slot: 0, X: -1490, Y: -590, Z: -180, Yaw: 359.9, Health: 100, Flags: FlagAlive | FlagScoped},
			}},
		}},
		{Round: 2, Frames: []Frame{
			{Tick: 500, Samples: []Sample{
				{Slot: 3, X: 0, Y: 0, Z: 0, Yaw: 0, Health: 0, Flags: 0},
			}},
		}},
	}
}

func TestEncodeDecodeRoundTrip(t *testing.T) {
	blob, err := EncodePositions(sampleRounds(), 8, 64)
	if err != nil {
		t.Fatalf("EncodePositions: %v", err)
	}
	if blob.Descriptor.Format != PositionsFormat {
		t.Fatalf("format = %q, want %q", blob.Descriptor.Format, PositionsFormat)
	}
	if blob.Descriptor.HZ != 8 {
		t.Fatalf("hz = %v, want 8", blob.Descriptor.HZ)
	}
	if blob.Descriptor.FrameCount != 3 {
		t.Fatalf("frame count = %d, want 3", blob.Descriptor.FrameCount)
	}
	if blob.Descriptor.SHA256 == "" {
		t.Fatal("descriptor must carry the blob checksum")
	}
	if blob.Descriptor.ByteLength != int64(len(blob.Data)) {
		t.Fatalf("byte length %d does not match %d bytes", blob.Descriptor.ByteLength, len(blob.Data))
	}

	desc, frames, err := DecodePositions(blob.Data)
	if err != nil {
		t.Fatalf("DecodePositions: %v", err)
	}
	if len(frames) != 3 {
		t.Fatalf("decoded %d frames, want 3", len(frames))
	}
	if frames[0].Tick != 100 || frames[1].Tick != 108 || frames[2].Tick != 500 {
		t.Fatalf("ticks not preserved: %d, %d, %d", frames[0].Tick, frames[1].Tick, frames[2].Tick)
	}

	want := sampleRounds()[0].Frames[0].Samples[0]
	got := frames[0].Samples[0]
	if got.Slot != want.Slot || got.Health != want.Health || got.Flags != want.Flags {
		t.Fatalf("sample identity lost: %+v", got)
	}
	// Positions are quantised, so equality is to within one step.
	tolerance := desc.Quantum
	if math.Abs(got.X-want.X) > tolerance || math.Abs(got.Y-want.Y) > tolerance || math.Abs(got.Z-want.Z) > tolerance {
		t.Fatalf("position drifted beyond one quantum (%v): got %+v want %+v", tolerance, got, want)
	}
	if math.Abs(got.Yaw-want.Yaw) > 0.01 {
		t.Fatalf("yaw = %v, want %v", got.Yaw, want.Yaw)
	}
}

func TestDecodeSingleRoundFromOffset(t *testing.T) {
	blob, err := EncodePositions(sampleRounds(), 8, 64)
	if err != nil {
		t.Fatalf("EncodePositions: %v", err)
	}
	offset, ok := blob.Descriptor.Offset(2)
	if !ok {
		t.Fatal("round 2 must have an offset")
	}
	if offset.FrameCount != 1 || offset.FirstTick != 500 || offset.LastTick != 500 {
		t.Fatalf("unexpected offset: %+v", offset)
	}

	frames, err := DecodeFrames(blob.Data, offset.ByteOffset, offset.FrameCount, blob.Descriptor)
	if err != nil {
		t.Fatalf("DecodeFrames: %v", err)
	}
	if len(frames) != 1 || frames[0].Tick != 500 {
		t.Fatalf("seeking to round 2 returned %+v", frames)
	}
	if len(frames[0].Samples) != 1 || frames[0].Samples[0].Slot != 3 {
		t.Fatalf("round 2 sample lost: %+v", frames[0].Samples)
	}
}

func TestYawWrapsAtFullCircle(t *testing.T) {
	rounds := []RoundFrames{{Round: 1, Frames: []Frame{{Tick: 1, Samples: []Sample{
		{Slot: 0, Yaw: 359.9999, Flags: FlagAlive},
	}}}}}
	blob, err := EncodePositions(rounds, 1, 64)
	if err != nil {
		t.Fatalf("EncodePositions: %v", err)
	}
	_, frames, err := DecodePositions(blob.Data)
	if err != nil {
		t.Fatalf("DecodePositions: %v", err)
	}
	// Just under a full turn must decode as either ~360 or ~0, never as a
	// wrapped-around negative or an out-of-range value.
	yaw := frames[0].Samples[0].Yaw
	if yaw < 0 || yaw >= 360 {
		t.Fatalf("yaw %v is outside [0, 360)", yaw)
	}
	if yaw > 0.01 && yaw < 359.9 {
		t.Fatalf("yaw %v is nowhere near the encoded 359.9999", yaw)
	}
}

func TestNegativeYawNormalises(t *testing.T) {
	rounds := []RoundFrames{{Round: 1, Frames: []Frame{{Tick: 1, Samples: []Sample{
		{Slot: 0, Yaw: -90, Flags: FlagAlive},
	}}}}}
	blob, err := EncodePositions(rounds, 1, 64)
	if err != nil {
		t.Fatalf("EncodePositions: %v", err)
	}
	_, frames, err := DecodePositions(blob.Data)
	if err != nil {
		t.Fatalf("DecodePositions: %v", err)
	}
	if got := frames[0].Samples[0].Yaw; math.Abs(got-270) > 0.01 {
		t.Fatalf("yaw -90 decoded as %v, want 270", got)
	}
}

// A map like Vertigo sits at an altitude that would overflow the int16 sample
// encoding at the finest quantum; the encoder must coarsen rather than wrap.
func TestQuantumCoarsensForLargeSpans(t *testing.T) {
	rounds := []RoundFrames{{Round: 1, Frames: []Frame{
		{Tick: 1, Samples: []Sample{{Slot: 0, X: -20000, Y: 0, Z: 11700, Flags: FlagAlive}}},
		{Tick: 2, Samples: []Sample{{Slot: 0, X: 20000, Y: 0, Z: 12000, Flags: FlagAlive}}},
	}}}
	blob, err := EncodePositions(rounds, 1, 64)
	if err != nil {
		t.Fatalf("EncodePositions: %v", err)
	}
	desc, frames, err := DecodePositions(blob.Data)
	if err != nil {
		t.Fatalf("DecodePositions: %v", err)
	}
	if desc.Quantum <= 0.25 {
		t.Fatalf("quantum %v should have coarsened for a 40000-unit span", desc.Quantum)
	}
	for i, want := range []float64{-20000, 20000} {
		if got := frames[i].Samples[0].X; math.Abs(got-want) > desc.Quantum {
			t.Fatalf("x %v decoded as %v with quantum %v", want, got, desc.Quantum)
		}
	}
	if got := frames[0].Samples[0].Z; math.Abs(got-11700) > desc.Quantum {
		t.Fatalf("z 11700 decoded as %v", got)
	}
}

func TestEncodeRejectsDuplicateSlot(t *testing.T) {
	rounds := []RoundFrames{{Round: 1, Frames: []Frame{{Tick: 1, Samples: []Sample{
		{Slot: 2, Flags: FlagAlive},
		{Slot: 2, Flags: FlagAlive},
	}}}}}
	if _, err := EncodePositions(rounds, 1, 64); err == nil {
		t.Fatal("a slot appearing twice in one frame must be an error")
	}
}

func TestEncodeRejectsBadSampleTicks(t *testing.T) {
	if _, err := EncodePositions(sampleRounds(), 0, 64); err == nil {
		t.Fatal("a zero sample interval must be an error")
	}
	if _, err := EncodePositions(sampleRounds(), math.MaxUint16+1, 64); err == nil {
		t.Fatal("a sample interval wider than the uint16 header must be an error")
	}
}

func TestEncodeRejectsTicksOutsideWireRange(t *testing.T) {
	for _, tick := range []int{math.MinInt32 - 1, math.MaxInt32 + 1} {
		rounds := []RoundFrames{{Round: 1, Frames: []Frame{{Tick: tick}}}}
		if _, err := EncodePositions(rounds, 1, 64); err == nil {
			t.Fatalf("tick %d outside int32 must be an error", tick)
		}
	}
}

func TestEncodeEmptyStreamStaysDecodable(t *testing.T) {
	blob, err := EncodePositions(nil, 8, 64)
	if err != nil {
		t.Fatalf("EncodePositions: %v", err)
	}
	desc, frames, err := DecodePositions(blob.Data)
	if err != nil {
		t.Fatalf("an empty stream must still decode: %v", err)
	}
	if len(frames) != 0 || desc.FrameCount != 0 {
		t.Fatalf("expected no frames, got %d", len(frames))
	}
}

func TestDecodeRejectsCorruptBlobs(t *testing.T) {
	good, err := EncodePositions(sampleRounds(), 8, 64)
	if err != nil {
		t.Fatalf("EncodePositions: %v", err)
	}

	t.Run("short header", func(t *testing.T) {
		if _, err := DecodeHeader(good.Data[:10]); err == nil {
			t.Fatal("want an error")
		}
	})
	t.Run("bad magic", func(t *testing.T) {
		corrupt := append([]byte(nil), good.Data...)
		corrupt[0] = 'X'
		if _, err := DecodeHeader(corrupt); err == nil {
			t.Fatal("want an error")
		}
	})
	t.Run("unsupported version", func(t *testing.T) {
		corrupt := append([]byte(nil), good.Data...)
		binary.LittleEndian.PutUint16(corrupt[6:], 99)
		if _, err := DecodeHeader(corrupt); err == nil {
			t.Fatal("want an error")
		}
	})
	t.Run("truncated samples", func(t *testing.T) {
		corrupt := append([]byte(nil), good.Data[:positionsHeaderSize+8]...)
		if _, _, err := DecodePositions(corrupt); err == nil {
			t.Fatal("want an error")
		}
	})
	t.Run("offset outside blob", func(t *testing.T) {
		if _, err := DecodeFrames(good.Data, int64(len(good.Data)+1), 1, good.Descriptor); err == nil {
			t.Fatal("want an error")
		}
	})
	t.Run("offset inside header", func(t *testing.T) {
		if _, err := DecodeFrames(good.Data, 4, 1, good.Descriptor); err == nil {
			t.Fatal("want an error")
		}
	})
}

func TestSampleFlagsHas(t *testing.T) {
	flags := FlagAlive | FlagSideT
	if !flags.Has(FlagAlive) || !flags.Has(FlagSideT) {
		t.Fatal("set bits must report as present")
	}
	if flags.Has(FlagBlinded) {
		t.Fatal("unset bit reported as present")
	}
	if !flags.Has(FlagAlive | FlagSideT) {
		t.Fatal("Has must accept a combined mask")
	}
}

// The blob is the largest artifact the feature produces, so its size per
// sampled player-tick is a contract worth pinning: 10 bytes, plus 6 per frame.
func TestBlobSizeIsTenBytesPerSample(t *testing.T) {
	rounds := []RoundFrames{{Round: 1, Frames: []Frame{
		{Tick: 1, Samples: []Sample{
			{Slot: 0}, {Slot: 1}, {Slot: 2}, {Slot: 3}, {Slot: 4},
			{Slot: 5}, {Slot: 6}, {Slot: 7}, {Slot: 8}, {Slot: 9},
		}},
	}}}
	blob, err := EncodePositions(rounds, 8, 64)
	if err != nil {
		t.Fatalf("EncodePositions: %v", err)
	}
	want := positionsHeaderSize + positionsFrameHead + 10*positionsSampleSize
	if len(blob.Data) != want {
		t.Fatalf("blob is %d bytes, want %d", len(blob.Data), want)
	}
}
