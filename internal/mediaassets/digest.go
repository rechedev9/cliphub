package mediaassets

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
)

// FileDigest binds a materialized input to its bytes before publication or use.
func FileDigest(ctx context.Context, path string, maxBytes int64) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	buffer := make([]byte, 128<<10)
	var total int64
	for {
		if err := ctx.Err(); err != nil {
			return "", err
		}
		n, err := f.Read(buffer)
		total += int64(n)
		if total > maxBytes {
			return "", fmt.Errorf("media content exceeds digest resource limit")
		}
		if n > 0 {
			_, _ = h.Write(buffer[:n])
		}
		if err == io.EOF {
			return hex.EncodeToString(h.Sum(nil)), nil
		}
		if err != nil {
			return "", err
		}
	}
}
