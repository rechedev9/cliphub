package tacticalplan

import "testing"

func benchmarkPositionRounds() []RoundFrames {
	rounds := make([]RoundFrames, 24)
	for r := range rounds {
		rounds[r] = RoundFrames{Round: r + 1, Frames: make([]Frame, 720)}
		for f := range rounds[r].Frames {
			frame := &rounds[r].Frames[f]
			frame.Tick = (r*960 + f) * 8
			for slot := 9; slot >= 0; slot-- {
				frame.Samples = append(frame.Samples, Sample{
					Slot: uint8(slot), X: float64(f*2 - 1500), Y: float64(slot * 100),
					Z: -180, Yaw: float64(f % 360), Health: 100, Flags: FlagAlive,
				})
			}
		}
	}
	return rounds
}

func BenchmarkEncodePositions(b *testing.B) {
	rounds := benchmarkPositionRounds()
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		if _, err := EncodePositions(rounds, 8, 64); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkEncodeSparsePositions(b *testing.B) {
	rounds := benchmarkPositionRounds()
	for r := range rounds {
		for f := range rounds[r].Frames {
			frame := &rounds[r].Frames[f]
			frame.Samples = frame.Samples[:1:1]
			frame.Samples[0].Slot = 15
		}
	}
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		if _, err := EncodePositions(rounds, 8, 64); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkDecodePositionsRound(b *testing.B) {
	blob, err := EncodePositions(benchmarkPositionRounds(), 8, 64)
	if err != nil {
		b.Fatal(err)
	}
	offset := blob.Descriptor.RoundOffsets[12]
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		if _, err := DecodeFrames(blob.Data, offset.ByteOffset, offset.FrameCount, blob.Descriptor); err != nil {
			b.Fatal(err)
		}
	}
}
