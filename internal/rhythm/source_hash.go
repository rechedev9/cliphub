package rhythm

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"

	"github.com/rechedev9/cliphub/internal/rooted"
)

// SourceSHA256 streams a media source into its canonical content fingerprint.
func SourceSHA256(path string) (string, error) {
	file, err := rooted.Open(path)
	if err != nil {
		return "", err
	}
	hash := sha256.New()
	_, copyErr := io.Copy(hash, file)
	closeErr := file.Close()
	if copyErr != nil {
		return "", fmt.Errorf("read source: %w", copyErr)
	}
	if closeErr != nil {
		return "", fmt.Errorf("close source: %w", closeErr)
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}
