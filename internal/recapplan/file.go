package recapplan

import (
	"fmt"
	"io"
	"os"
)

// ReadDocumentFile is the versioned CLI boundary shared by recorder and editor.
func ReadDocumentFile(path string) (Document, error) {
	f, err := os.Open(path)
	if err != nil {
		return Document{}, fmt.Errorf("open Full Demo document: %w", err)
	}
	defer f.Close()
	b, err := io.ReadAll(io.LimitReader(f, (4<<20)+1))
	if err != nil {
		return Document{}, err
	}
	var d Document
	if err := decodeStrict(b, &d); err != nil {
		return d, err
	}
	return d, d.Validate()
}
