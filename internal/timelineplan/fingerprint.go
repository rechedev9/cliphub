package timelineplan

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"
)

func Fingerprint(doc Document) (string, error) {
	normalized := Normalize(doc)
	normalized.UpdatedAt = time.Time{}
	b, err := json.Marshal(normalized)
	if err != nil {
		return "", fmt.Errorf("encode normalized timeline: %w", err)
	}
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:]), nil
}
