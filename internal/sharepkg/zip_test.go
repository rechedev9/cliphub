package sharepkg

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/rechedev9/cliphub/internal/library"
	"github.com/rechedev9/cliphub/internal/radarmap"
	"github.com/rechedev9/cliphub/internal/tacticalplan"
)

var testSHA = strings.Repeat("ab", 32)

type matchFixture struct {
	doc  []byte
	blob []byte
}

func newMatchFixture(t *testing.T) matchFixture {
	t.Helper()
	blob, err := tacticalplan.EncodePositions([]tacticalplan.RoundFrames{
		{Round: 1, Frames: []tacticalplan.Frame{{Tick: 100, Samples: []tacticalplan.Sample{{Slot: 0, X: 1, Y: 2, Health: 100}}}}},
		{Round: 2, Frames: []tacticalplan.Frame{{Tick: 500, Samples: []tacticalplan.Sample{{Slot: 0, X: 3, Y: 4, Health: 100}}}}},
	}, 8, 64)
	if err != nil {
		t.Fatal(err)
	}
	doc := tacticalplan.NewDocument()
	doc.Demo = tacticalplan.Demo{Map: "de_mirage", SHA256: testSHA, Tickrate: 64}
	doc.Rounds = []tacticalplan.Round{{Number: 1}, {Number: 2}}
	cal, _ := radarmap.Lookup("de_mirage")
	doc.Geometry = tacticalplan.MapGeometry{Map: "de_mirage", Calibration: cal}
	doc.Positions = blob.Descriptor
	b, err := json.Marshal(doc)
	if err != nil {
		t.Fatal(err)
	}
	return matchFixture{doc: b, blob: blob.Data}
}

func testManifest() Manifest {
	matchID := uuid.New()
	return Manifest{
		Format: Format, Version: Version, CreatedAt: time.Now().UTC(), Kind: KindReport, Title: "Prueba", Notes: "Mirad la 2",
		Report: &library.Report{ID: uuid.New(), Title: "Prueba", TeamName: "Alpha", MatchIDs: []uuid.UUID{matchID}},
		Matches: []Match{{
			Match: library.Match{ID: matchID, Map: "de_mirage", RoundCount: 2, Teams: []library.MatchTeam{{Key: "ct-start", TeamName: "Alpha"}}},
			Demo:  Demo{SHA256: testSHA, FileName: "match.dem"},
		}},
		Annotations: []library.Annotation{{ID: uuid.New(), MatchID: matchID, Round: 2, Tick: 500, Text: "aquí"}},
	}
}

func writePackage(t *testing.T, m Manifest, f matchFixture) []byte {
	t.Helper()
	var buf bytes.Buffer
	err := Write(&buf, m, func(uuid.UUID) (io.ReadCloser, io.ReadCloser, error) {
		return io.NopCloser(bytes.NewReader(f.doc)), io.NopCloser(bytes.NewReader(f.blob)), nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestPackageRoundTrip(t *testing.T) {
	f := newMatchFixture(t)
	m := testManifest()
	raw := writePackage(t, m, f)
	pkg, err := Open(bytes.NewReader(raw), int64(len(raw)))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if pkg.Manifest.Title != "Prueba" || pkg.Manifest.Notes != "Mirad la 2" || len(pkg.Manifest.Annotations) != 1 {
		t.Fatalf("manifest = %+v", pkg.Manifest)
	}
	data, err := pkg.Match(m.Matches[0].Match.ID)
	if err != nil {
		t.Fatalf("Match: %v", err)
	}
	if !bytes.Equal(data.Positions, f.blob) || !bytes.Equal(data.DocumentJSON, f.doc) || len(data.Document.Rounds) != 2 {
		t.Fatal("the match files changed on the way through")
	}
}

// rawZip builds a zip by hand, for files Write would never produce.
func rawZip(t *testing.T, files map[string][]byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for name, data := range files {
		w, err := zw.CreateHeader(&zip.FileHeader{Name: name, Method: zip.Deflate})
		if err != nil {
			t.Fatal(err)
		}
		_, _ = w.Write(data)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func manifestJSON(t *testing.T, m Manifest) []byte {
	t.Helper()
	b, err := json.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestOpenRefusesBadPackages(t *testing.T) {
	f := newMatchFixture(t)
	m := testManifest()
	id := m.Matches[0].Match.ID
	valid := map[string][]byte{manifestName: manifestJSON(t, m), documentName(id): f.doc, positionsName(id): f.blob}
	with := func(edit func(files map[string][]byte)) []byte {
		files := map[string][]byte{}
		for k, v := range valid {
			files[k] = v
		}
		edit(files)
		return rawZip(t, files)
	}
	newer := m
	newer.Version = Version + 1
	notes := m
	notes.Annotations = []library.Annotation{{ID: uuid.New(), MatchID: uuid.New(), Round: 1, Text: "x"}}
	foreignMatch := m
	foreignMatch.Report = &library.Report{ID: uuid.New(), Title: "x", TeamName: "Alpha", MatchIDs: []uuid.UUID{uuid.New()}}

	cases := map[string][]byte{
		"not a zip":       []byte("hello"),
		"path traversal":  with(func(files map[string][]byte) { files["../evil.txt"] = []byte("x") }),
		"absolute path":   with(func(files map[string][]byte) { files["C:/Windows/evil.dll"] = []byte("x") }),
		"unexpected file": with(func(files map[string][]byte) { files["matches/readme.txt"] = []byte("x") }),
		"missing blob":    with(func(files map[string][]byte) { delete(files, positionsName(id)) }),
		"no manifest":     with(func(files map[string][]byte) { delete(files, manifestName) }),
		"bad json":        with(func(files map[string][]byte) { files[manifestName] = []byte("{") }),
		"newer version":   with(func(files map[string][]byte) { files[manifestName] = manifestJSON(t, newer) }),
		"foreign note":    with(func(files map[string][]byte) { files[manifestName] = manifestJSON(t, notes) }),
		"foreign match":   with(func(files map[string][]byte) { files[manifestName] = manifestJSON(t, foreignMatch) }),
		// A manifest that inflates past its cap is refused while it is read.
		"zip bomb": with(func(files map[string][]byte) { files[manifestName] = bytes.Repeat([]byte(" "), maxManifestBytes+1) }),
	}
	for name, raw := range cases {
		if _, err := Open(bytes.NewReader(raw), int64(len(raw))); !errors.Is(err, ErrInvalid) {
			t.Errorf("%s: Open = %v, want ErrInvalid", name, err)
		}
	}
	if _, err := Open(bytes.NewReader(nil), MaxPackageBytes+1); !errors.Is(err, ErrInvalid) {
		t.Errorf("oversize: Open = %v, want ErrInvalid", err)
	}
}

func TestMatchRefusesFilesThatDoNotBindTogether(t *testing.T) {
	f := newMatchFixture(t)
	m := testManifest()
	id := m.Matches[0].Match.ID
	tampered := append([]byte{}, f.blob...)
	tampered[len(tampered)-1] ^= 0xff
	otherMap := m
	otherMap.Matches = []Match{m.Matches[0]}
	otherMap.Matches[0].Match.Map = "de_inferno"

	for name, pkgBytes := range map[string][]byte{
		"tampered positions": writePackage(t, m, matchFixture{doc: f.doc, blob: tampered}),
		"other map":          writePackage(t, otherMap, f),
		"old schema":         writePackage(t, m, matchFixture{doc: bytes.Replace(f.doc, []byte(`"schema_version":"1.0"`), []byte(`"schema_version":"0.9"`), 1), blob: f.blob}),
	} {
		pkg, err := Open(bytes.NewReader(pkgBytes), int64(len(pkgBytes)))
		if err != nil {
			t.Fatalf("%s: Open = %v", name, err)
		}
		if _, err := pkg.Match(id); !errors.Is(err, ErrInvalid) {
			t.Errorf("%s: Match = %v, want ErrInvalid", name, err)
		}
	}
}
