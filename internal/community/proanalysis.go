package community

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"

	"github.com/klauspost/compress/zstd"

	"github.com/rechedev9/cliphub/internal/tacticalplan"
)

// ProAnalysisFormat opens a pro analysis file: zstd of one JSON header line and the raw
// position blob, one per demo of a pro map, which Studio imports without the demo.
const ProAnalysisFormat = "zvpro1"

// ProAnalysisSuffix ends the name of every pro analysis file.
const ProAnalysisSuffix = ".zvpro.zst"

const (
	maxProHeaderBytes    = 64 << 20
	maxProPositionsBytes = 128 << 20
)

// ProSidecar is the HLTV match a pro analysis came from, as the harvester writes it.
type ProSidecar struct {
	Source   string           `json:"source"`
	MatchID  int              `json:"match_id"`
	DemoPath string           `json:"demo_path"`
	Event    string           `json:"event"`
	PlayedAt string           `json:"played_at"`
	Teams    []ProSidecarTeam `json:"teams"`
	Maps     []string         `json:"maps"`
}

// ProSidecarTeam is one team of the sidecar; Rank is nil outside the HLTV ranking.
type ProSidecarTeam struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
	Rank *int   `json:"rank"`
}

// ProAnalysisHeader is the JSON line of a pro analysis file.
type ProAnalysisHeader struct {
	Format    string                 `json:"format"`
	Match     ProSidecar             `json:"match"`
	Entry     string                 `json:"entry"`
	Document  tacticalplan.Document  `json:"document"`
	Positions tacticalplan.Positions `json:"positions"`
}

// WriteProAnalysis writes the header line and the position blob as one zstd stream.
func WriteProAnalysis(w io.Writer, h ProAnalysisHeader, blob []byte) error {
	line, err := json.Marshal(h)
	if err != nil {
		return err
	}
	zw, err := zstd.NewWriter(w, zstd.WithEncoderLevel(zstd.SpeedBetterCompression))
	if err != nil {
		return err
	}
	_, werr := zw.Write(append(line, '\n'))
	if werr == nil {
		_, werr = zw.Write(blob)
	}
	if cerr := zw.Close(); werr == nil {
		werr = cerr
	}
	return werr
}

// ReadProAnalysis decodes a pro analysis: its header and its position blob.
func ReadProAnalysis(r io.Reader) (ProAnalysisHeader, []byte, error) {
	zr, err := zstd.NewReader(r)
	if err != nil {
		return ProAnalysisHeader{}, nil, err
	}
	defer zr.Close()
	br := bufio.NewReaderSize(io.LimitReader(zr, maxProHeaderBytes+maxProPositionsBytes+1), 1<<20)
	line, err := readProLine(br)
	if err != nil {
		return ProAnalysisHeader{}, nil, err
	}
	var h ProAnalysisHeader
	if err := json.Unmarshal(line, &h); err != nil {
		return ProAnalysisHeader{}, nil, fmt.Errorf("decode analysis header: %w", err)
	}
	if h.Format != ProAnalysisFormat {
		return ProAnalysisHeader{}, nil, fmt.Errorf("analysis format %q", h.Format)
	}
	blob, err := io.ReadAll(br)
	if err != nil {
		return ProAnalysisHeader{}, nil, fmt.Errorf("read positions: %w", err)
	}
	if len(blob) > maxProPositionsBytes {
		return ProAnalysisHeader{}, nil, errors.New("analysis positions are too large")
	}
	return h, blob, nil
}

func readProLine(br *bufio.Reader) ([]byte, error) {
	var buf bytes.Buffer
	for {
		chunk, err := br.ReadSlice('\n')
		buf.Write(chunk)
		if err == nil {
			return bytes.TrimSuffix(buf.Bytes(), []byte("\n")), nil
		}
		if !errors.Is(err, bufio.ErrBufferFull) {
			return nil, fmt.Errorf("read analysis header: %w", err)
		}
		if buf.Len() > maxProHeaderBytes {
			return nil, errors.New("analysis header is too large")
		}
	}
}

// CheckProAnalysis refuses an analysis Studio cannot draw: another schema, no
// radar calibration, or a position blob its document does not describe.
func CheckProAnalysis(h ProAnalysisHeader, blob []byte) error {
	doc := h.Document
	switch {
	case doc.SchemaVersion != tacticalplan.SchemaVersion:
		return fmt.Errorf("analysis format %q is not supported", doc.SchemaVersion)
	case doc.Demo.Map == "" || len(doc.Rounds) == 0:
		return errors.New("analysis holds no rounds")
	case !doc.Geometry.Calibration.Valid():
		return errors.New("analysis has no radar calibration")
	}
	desc := doc.Positions
	sum := sha256.Sum256(blob)
	if int64(len(blob)) != desc.ByteLength || hex.EncodeToString(sum[:]) != desc.SHA256 {
		return errors.New("position data does not match its analysis")
	}
	if _, err := tacticalplan.DecodeHeader(blob); err != nil {
		return err
	}
	for _, o := range desc.RoundOffsets {
		if o.ByteOffset < tacticalplan.PositionsHeaderSize || o.ByteLength < 0 || o.ByteOffset+o.ByteLength > desc.ByteLength {
			return fmt.Errorf("round %d points outside the position data", o.Round)
		}
	}
	return nil
}
