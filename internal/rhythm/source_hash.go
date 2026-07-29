package rhythm

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
)

// SourceSHA256 streams a media source into its canonical content fingerprint.
func SourceSHA256(path string) (string, error) {
	// #nosec G304 -- path is the explicit local music/video source selected by the caller.
	file, err := os.Open(path)
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
