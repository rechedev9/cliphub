package sharepkg

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"

	"github.com/google/uuid"

	"github.com/rechedev9/cliphub/internal/tacticalplan"
)

// Size limits. A match is a few megabytes unpacked, so these leave room for
// long overtimes while refusing a zip bomb before it is inflated.
const (
	MaxPackageBytes   int64 = 256 << 20
	maxManifestBytes        = 16 << 20
	maxDocumentBytes        = 64 << 20
	maxPositionsBytes       = 128 << 20
	maxUnpackedBytes        = 2 << 30
)

const manifestName = "manifest.json"

func documentName(id uuid.UUID) string  { return "matches/" + id.String() + "/tactical.json" }
func positionsName(id uuid.UUID) string { return "matches/" + id.String() + "/positions.zvpos" }

// MatchFiles opens the stored tactical document and position blob of a match
// while a package is written.
type MatchFiles func(id uuid.UUID) (document, positions io.ReadCloser, err error)

// Write writes the package: the manifest, then the two files of every match.
func Write(w io.Writer, m Manifest, open MatchFiles) error {
	zw := zip.NewWriter(w)
	b, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return fmt.Errorf("encode manifest: %w", err)
	}
	if err := writeEntry(zw, manifestName, bytes.NewReader(b)); err != nil {
		return err
	}
	for _, entry := range m.Matches {
		id := entry.Match.ID
		doc, pos, err := open(id)
		if err != nil {
			return fmt.Errorf("open files of match %s: %w", id, err)
		}
		err = writeEntry(zw, documentName(id), doc)
		if err == nil {
			err = writeEntry(zw, positionsName(id), pos)
		}
		_ = doc.Close()
		_ = pos.Close()
		if err != nil {
			return err
		}
	}
	return zw.Close()
}

func writeEntry(zw *zip.Writer, name string, r io.Reader) error {
	f, err := zw.CreateHeader(&zip.FileHeader{Name: name, Method: zip.Deflate})
	if err != nil {
		return fmt.Errorf("add %s: %w", name, err)
	}
	if _, err := io.Copy(f, r); err != nil {
		return fmt.Errorf("write %s: %w", name, err)
	}
	return nil
}

// Reader is an opened, validated package. Its manifest is checked and the
// zip holds exactly the files the manifest names; each match's files are
// checked when they are read.
type Reader struct {
	Manifest Manifest
	files    map[string]*zip.File
	matches  map[uuid.UUID]Match
}

// Open reads and validates the manifest and the file list of a package.
func Open(r io.ReaderAt, size int64) (*Reader, error) {
	if size > MaxPackageBytes {
		return nil, invalidf("the file is larger than %d MB", MaxPackageBytes>>20)
	}
	zr, err := zip.NewReader(r, size)
	if err != nil {
		return nil, invalidf("the file is not a ClipHub package")
	}
	if len(zr.File) > 1+2*MaxMatches {
		return nil, invalidf("the package holds too many files")
	}
	files := make(map[string]*zip.File, len(zr.File))
	var total uint64
	for _, f := range zr.File {
		if _, dup := files[f.Name]; dup {
			return nil, invalidf("the package holds %q twice", f.Name)
		}
		files[f.Name] = f
		total += f.UncompressedSize64
		if total > maxUnpackedBytes {
			return nil, invalidf("the package unpacks to more than %d MB", maxUnpackedBytes>>20)
		}
	}
	mf, ok := files[manifestName]
	if !ok {
		return nil, invalidf("the package has no manifest")
	}
	raw, err := readEntry(mf, maxManifestBytes)
	if err != nil {
		return nil, err
	}
	var m Manifest
	if err := json.Unmarshal(raw, &m); err != nil {
		return nil, invalidf("the manifest is not valid JSON")
	}
	if err := m.Validate(); err != nil {
		return nil, err
	}
	pkg := &Reader{Manifest: m, files: files, matches: make(map[uuid.UUID]Match, len(m.Matches))}
	expected := map[string]bool{manifestName: true}
	for _, entry := range m.Matches {
		pkg.matches[entry.Match.ID] = entry
		expected[documentName(entry.Match.ID)] = true
		expected[positionsName(entry.Match.ID)] = true
	}
	for name := range files {
		if !expected[name] {
			return nil, invalidf("the package holds an unexpected file %q", name)
		}
	}
	for name := range expected {
		if _, ok := files[name]; !ok {
			return nil, invalidf("the package is missing %q", name)
		}
	}
	return pkg, nil
}

func readEntry(f *zip.File, limit int64) ([]byte, error) {
	// #nosec G115 -- limit is a positive constant.
	if f.UncompressedSize64 > uint64(limit) {
		return nil, invalidf("%q is larger than %d MB", f.Name, limit>>20)
	}
	rc, err := f.Open()
	if err != nil {
		return nil, invalidf("%q cannot be read", f.Name)
	}
	defer rc.Close()
	b, err := io.ReadAll(io.LimitReader(rc, limit+1))
	if err != nil {
		return nil, invalidf("%q is damaged", f.Name)
	}
	if int64(len(b)) > limit {
		return nil, invalidf("%q is larger than %d MB", f.Name, limit>>20)
	}
	return b, nil
}

// MatchData is the checked content of one carried match.
type MatchData struct {
	Document     tacticalplan.Document
	DocumentJSON []byte
	Positions    []byte
}

// Match reads and checks the files of one carried match: the document must
// be the current schema, describe the same map and demo, and bind the blob by
// its length, digest and round offsets.
func (r *Reader) Match(id uuid.UUID) (MatchData, error) {
	entry, ok := r.matches[id]
	if !ok {
		return MatchData{}, invalidf("match %s is not in the package", id)
	}
	docJSON, err := readEntry(r.files[documentName(id)], maxDocumentBytes)
	if err != nil {
		return MatchData{}, err
	}
	positions, err := readEntry(r.files[positionsName(id)], maxPositionsBytes)
	if err != nil {
		return MatchData{}, err
	}
	var doc tacticalplan.Document
	if err := json.Unmarshal(docJSON, &doc); err != nil {
		return MatchData{}, invalidf("the analysis of match %s is not valid JSON", id)
	}
	if err := checkMatchFiles(entry, doc, positions); err != nil {
		return MatchData{}, invalidf("match %s: %v", id, err)
	}
	return MatchData{Document: doc, DocumentJSON: docJSON, Positions: positions}, nil
}

func checkMatchFiles(entry Match, doc tacticalplan.Document, positions []byte) error {
	switch {
	case doc.SchemaVersion != tacticalplan.SchemaVersion:
		return fmt.Errorf("analysis format %q is not supported", doc.SchemaVersion)
	case doc.Demo.Map != entry.Match.Map:
		return fmt.Errorf("analysis is of map %q, not %q", doc.Demo.Map, entry.Match.Map)
	case doc.Demo.SHA256 != "" && doc.Demo.SHA256 != entry.Demo.SHA256:
		return fmt.Errorf("analysis belongs to another demo")
	case len(doc.Rounds) != entry.Match.RoundCount:
		return fmt.Errorf("analysis has %d rounds, the match %d", len(doc.Rounds), entry.Match.RoundCount)
	case !doc.Geometry.Calibration.Valid():
		return fmt.Errorf("analysis has no radar calibration")
	}
	desc := doc.Positions
	sum := sha256.Sum256(positions)
	if int64(len(positions)) != desc.ByteLength || hex.EncodeToString(sum[:]) != desc.SHA256 {
		return fmt.Errorf("position data does not match its analysis")
	}
	if _, err := tacticalplan.DecodeHeader(positions); err != nil {
		return err
	}
	for _, o := range desc.RoundOffsets {
		if o.ByteOffset < tacticalplan.PositionsHeaderSize || o.ByteLength < 0 || o.ByteOffset+o.ByteLength > desc.ByteLength {
			return fmt.Errorf("round %d points outside the position data", o.Round)
		}
	}
	return nil
}
