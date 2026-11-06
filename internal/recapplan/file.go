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
	d, err := DecodeDocument(b)
	if err != nil {
		return d, err
	}
	return d, d.Validate()
}

// DecodeDocument rejects unknown and missing fields, so a reader built from
// another version fails here instead of recomputing a different content hash.
func DecodeDocument(b []byte) (Document, error) {
	var d Document
	err := decodeStrict(b, &d)
	return d, err
}
