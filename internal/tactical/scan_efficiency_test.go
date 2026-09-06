package tactical

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/rechedev9/cliphub/internal/tacticalplan"
)

func TestScanHashedIncludesReadAheadAndTrailingBytes(t *testing.T) {
	data := bytes.Repeat([]byte("demo bytes\x00"), 10000)
	want := fmt.Sprintf("%x", sha256.Sum256(data))
	for _, consumed := range []int{0, 1, 4096, len(data)} {
		for _, supplied := range []string{"", want, strings.ToUpper(want)} {
			source := bytes.NewReader(data)
			result, err := scanHashed(context.Background(), source, Options{SHA256: supplied}, func(r io.Reader) (Result, error) {
				_, err := io.CopyN(io.Discard, r, int64(consumed))
				return Result{}, err
			})
			if err != nil {
				t.Fatal(err)
			}
			if result.Document.Demo.SHA256 != want || source.Len() != 0 {
				t.Fatal("incomplete or double hash")
			}
		}
	}
}

func TestScanHashedErrorsAndCancellation(t *testing.T) {
	failure := errors.New("read failed")
	_, err := scanHashed(context.Background(), failingReader{failure}, Options{}, func(io.Reader) (Result, error) { return Result{}, nil })
	if !errors.Is(err, failure) {
		t.Fatalf("drain error = %v", err)
	}
	_, err = scanHashed(context.Background(), bytes.NewReader(nil), Options{}, func(io.Reader) (Result, error) { return Result{}, failure })
	if !errors.Is(err, failure) {
		t.Fatalf("parser error = %v", err)
	}
	_, err = scanHashed(context.Background(), bytes.NewBufferString("demo"), Options{SHA256: "wrong"}, func(io.Reader) (Result, error) { return Result{}, nil })
	if err == nil {
		t.Fatal("trusted an unverified supplied checksum")
	}
	ctx, cancel := context.WithCancel(context.Background())
	_, err = scanHashed(ctx, bytes.NewBufferString("trailing"), Options{}, func(io.Reader) (Result, error) { cancel(); return Result{}, nil })
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("cancellation = %v", err)
	}
	ctx, cancel = context.WithCancel(context.Background())
	cancel()
	_, err = ScanFile(ctx, "must-not-be-opened.dem", Options{})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("pre-cancellation = %v", err)
	}
}

type failingReader struct{ err error }

func (r failingReader) Read([]byte) (int, error) { return 0, r.err }

func TestSampleSlotsAllMasksMatchSortedSamples(t *testing.T) {
	for mask := 0; mask < 1<<16; mask++ {
		var slots sampleSlots
		var want []tacticalplan.Sample
		for slot := 15; slot >= 0; slot-- {
			if mask&(1<<slot) == 0 {
				continue
			}
			sample := tacticalplan.Sample{Slot: uint8(slot), X: float64(slot), Health: 100}
			slots.add(sample)
			want = append(want, sample)
		}
		sort.Slice(want, func(i, j int) bool { return want[i].Slot < want[j].Slot })
		got := slots.ordered()
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("mask %x: got %v want %v", mask, got, want)
		}
		if len(got) != cap(got) {
			t.Fatal("capacity must follow present samples")
		}
		slots.add(tacticalplan.Sample{Slot: 0, X: 999})
		if !reflect.DeepEqual(got, want) {
			t.Fatal("later frame mutated retained samples")
		}
	}
}

func TestSampleSlotsRetainsInvalidDuplicates(t *testing.T) {
	var slots sampleSlots
	slots.add(tacticalplan.Sample{Slot: 15, X: 1})
	slots.add(tacticalplan.Sample{Slot: 15, X: 2})
	slots.add(tacticalplan.Sample{Slot: 17, X: 3})
	got := slots.ordered()
	if len(got) != 3 || got[0].X != 1 || got[1].X != 2 || got[2].Slot != 17 {
		t.Fatal(got)
	}
}

var collectedSamples []tacticalplan.Sample

func BenchmarkCollectSamples(b *testing.B) {
	for _, count := range []int{1, 10, 16} {
		input := make([]tacticalplan.Sample, count)
		for i := range input {
			input[i] = tacticalplan.Sample{Slot: uint8(15 - i), X: float64(i)}
		}
		b.Run(fmt.Sprintf("slots-%d/legacy", count), func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				var samples []tacticalplan.Sample
				for _, sample := range input {
					samples = append(samples, sample)
				}
				sort.Slice(samples, func(i, j int) bool { return samples[i].Slot < samples[j].Slot })
				collectedSamples = samples
			}
		})
		b.Run(fmt.Sprintf("slots-%d/indexed", count), func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				var slots sampleSlots
				for _, sample := range input {
					slots.add(sample)
				}
				collectedSamples = slots.ordered()
			}
		})
	}
}
