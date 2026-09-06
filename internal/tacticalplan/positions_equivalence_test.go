package tacticalplan

import (
	"bytes"
	"encoding/binary"
	"math/rand/v2"
	"reflect"
	"testing"
)

// Keep the old scan-based wire writer here as an independent ordering oracle.
func legacyFrameBytes(f Frame, origin [3]float64, quantum float64) []byte {
	var mask uint16
	for _, s := range f.Samples {
		mask |= 1 << s.Slot
	}
	buf := binary.LittleEndian.AppendUint32(nil, uint32(int32(f.Tick)))
	buf = binary.LittleEndian.AppendUint16(buf, mask)
	for slot := uint8(0); slot < maxSlots; slot++ {
		for _, s := range f.Samples {
			if s.Slot != slot {
				continue
			}
			buf = appendSigned16(buf, quantizeAxis(s.X, origin[0], quantum))
			buf = appendSigned16(buf, quantizeAxis(s.Y, origin[1], quantum))
			buf = appendSigned16(buf, quantizeAxis(s.Z, origin[2], quantum))
			buf = binary.LittleEndian.AppendUint16(buf, encodeYaw(s.Yaw))
			buf = append(buf, clampHealth(s.Health), byte(s.Flags))
			break
		}
	}
	return buf
}

func TestSparsePositionsReserveOnlyPresentSamples(t *testing.T) {
	rounds := []RoundFrames{{Round: 1, Frames: []Frame{
		{Tick: 8, Samples: []Sample{{Slot: 15, X: 125, Health: 100, Flags: FlagAlive}}},
		{Tick: 16},
		{Tick: 24, Samples: []Sample{{Slot: 0}, {Slot: 15}}},
	}}}
	blob, err := EncodePositions(rounds, 8, 64)
	if err != nil {
		t.Fatal(err)
	}
	want := positionsHeaderSize + 3*positionsFrameHead + 3*positionsSampleSize
	if len(blob.Data) != want || cap(blob.Data) != want {
		t.Fatalf("sparse blob len/cap = %d/%d, want %d", len(blob.Data), cap(blob.Data), want)
	}
	if blob.Descriptor.SlotCount != 16 {
		t.Fatal("capacity optimization must not change the wire slot count")
	}
	_, frames, err := DecodePositions(blob.Data)
	if err != nil || len(frames) != 3 || frames[0].Samples[0].Slot != 15 || len(frames[1].Samples) != 0 {
		t.Fatalf("sparse round did not roundtrip: frames=%+v err=%v", frames, err)
	}
}

func TestPositionCodecPreservesEverySlotMask(t *testing.T) {
	rng := rand.New(rand.NewPCG(42, 17))
	origin := [3]float64{-100, 200, -300}
	for mask := 0; mask <= 0xffff; mask++ {
		frame := Frame{Tick: mask - 32768}
		for slot := 0; slot < maxSlots; slot++ {
			if mask&(1<<slot) == 0 {
				continue
			}
			frame.Samples = append(frame.Samples, Sample{
				Slot: uint8(slot), X: float64(slot*100 - 900), Y: float64(slot * 37),
				Z: float64(slot * -80), Yaw: float64(slot*47 - 90), Health: slot * 20,
				Flags: SampleFlags(slot),
			})
		}
		rng.Shuffle(len(frame.Samples), func(i, j int) { frame.Samples[i], frame.Samples[j] = frame.Samples[j], frame.Samples[i] })
		original := append([]Sample(nil), frame.Samples...)
		got, err := appendFrame(nil, frame, origin, 0.25)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(got, legacyFrameBytes(frame, origin, 0.25)) {
			t.Fatalf("wire changed for mask %04x", mask)
		}
		if !reflect.DeepEqual(original, frame.Samples) {
			t.Fatalf("input mutated for mask %04x", mask)
		}
		data := append(make([]byte, positionsHeaderSize), got...)
		decoded, err := DecodeFrames(data, positionsHeaderSize, 1, Positions{Origin: origin, Quantum: 0.25})
		if err != nil {
			t.Fatal(err)
		}
		if decoded[0].Tick != frame.Tick || len(decoded[0].Samples) != len(frame.Samples) {
			t.Fatalf("frame changed for mask %04x", mask)
		}
		previous := -1
		for _, sample := range decoded[0].Samples {
			if int(sample.Slot) <= previous || mask&(1<<sample.Slot) == 0 {
				t.Fatalf("slot order changed for mask %04x", mask)
			}
			previous = int(sample.Slot)
			if sample.X != float64(int(sample.Slot)*100-900) || sample.Health != int(clampHealth(int(sample.Slot)*20)) {
				t.Fatalf("sample changed for mask %04x: %+v", mask, sample)
			}
		}
	}
}
