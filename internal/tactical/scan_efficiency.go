package tactical

import (
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"math/bits"
	"strings"

	"github.com/rechedev9/cliphub/internal/tacticalplan"
)

// Hash exactly the bytes read by the parser, then drain any trailing bytes on
// the SAME reader. Parser read-ahead has already crossed the tee and must not
// be hashed twice. No cached hash is trusted as a substitute for reading.
func scanHashed(ctx context.Context, source io.Reader, opts Options, scan func(io.Reader) (Result, error)) (Result, error) {
	h := sha256.New()
	reader := io.TeeReader(contextReader{ctx, source}, h)
	result, err := scan(reader)
	if err != nil {
		return Result{}, err
	}
	if _, err := io.Copy(io.Discard, reader); err != nil {
		return Result{}, fmt.Errorf("checksum demo %q: %w", opts.DemoPath, err)
	}
	if err := ctx.Err(); err != nil {
		return Result{}, err
	}
	sum := fmt.Sprintf("%x", h.Sum(nil))
	if opts.SHA256 != "" && !strings.EqualFold(opts.SHA256, sum) {
		return Result{}, fmt.Errorf("checksum demo %q: supplied SHA256 does not match file", opts.DemoPath)
	}
	result.Document.Demo.SHA256 = sum
	return result, nil
}

type contextReader struct {
	ctx    context.Context
	source io.Reader
}

func (r contextReader) Read(p []byte) (int, error) {
	if err := r.ctx.Err(); err != nil {
		return 0, err
	}
	return r.source.Read(p)
}

// sampleSlots holds one frame's samples by slot. slotFor bounds slots below 16 and
// playersBySlot leaves one player per slot, so add only sets.
type sampleSlots struct {
	values  [16]tacticalplan.Sample
	present uint16
}

func (s *sampleSlots) add(sample tacticalplan.Sample) {
	s.values[sample.Slot] = sample
	s.present |= 1 << sample.Slot
}

func (s *sampleSlots) ordered() []tacticalplan.Sample {
	count := bits.OnesCount16(s.present)
	if count == 0 {
		return nil
	}
	samples := make([]tacticalplan.Sample, 0, count)
	for slot := range s.values {
		if s.present&(1<<slot) != 0 {
			samples = append(samples, s.values[slot])
		}
	}
	return samples
}
