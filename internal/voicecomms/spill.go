package voicecomms

import (
	"encoding/binary"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
)

type packetSpill struct {
	dir     string
	files   map[uint64]*spillFile
	records []spillRecord
}

type spillRecord struct {
	xuid    uint64
	offset  int64
	length  int
	offsets []uint32
}

func newPacketSpill(dir string) (*packetSpill, error) {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, fmt.Errorf("create voice spill dir: %w", err)
	}
	return &packetSpill{dir: dir, files: map[uint64]*spillFile{}}, nil
}

func (s *packetSpill) Close() error {
	if s == nil {
		return nil
	}
	var first error
	for _, f := range s.files {
		if err := f.Close(); err != nil && first == nil {
			first = err
		}
	}
	s.files = nil
	return first
}

func (s *packetSpill) write(index int, xuid uint64, tick int, data []byte, offsets []uint32) error {
	if s == nil || len(data) == 0 {
		return nil
	}
	if len(data) > 65535 || len(offsets) > 255 || tick < 0 || uint64(tick) > math.MaxUint32 || index < 0 || index >= 2000000 {
		return fmt.Errorf("invalid voice spill record size or timestamp")
	}
	f, err := s.file(xuid)
	if err != nil {
		return err
	}
	offset, err := f.Seek(0, io.SeekCurrent)
	if err != nil {
		return fmt.Errorf("seek spill packet %d: %w", index, err)
	}
	header := make([]byte, 9+len(offsets)*4)
	binary.LittleEndian.PutUint32(header[0:4], uint32(tick))
	binary.LittleEndian.PutUint16(header[4:6], uint16(len(data)))
	header[6] = byte(len(offsets))
	pos := 9
	for _, off := range offsets {
		binary.LittleEndian.PutUint32(header[pos:pos+4], off)
		pos += 4
	}
	if _, err := f.Write(header); err != nil {
		return fmt.Errorf("write spill header for packet %d: %w", index, err)
	}
	if _, err := f.Write(data); err != nil {
		return fmt.Errorf("write spill payload for packet %d: %w", index, err)
	}
	for len(s.records) <= index {
		s.records = append(s.records, spillRecord{})
	}
	s.records[index] = spillRecord{
		xuid:    xuid,
		offset:  offset,
		length:  len(header) + len(data),
		offsets: append([]uint32(nil), offsets...),
	}
	return nil
}

func (s *packetSpill) payload(index int) ([]byte, []uint32, error) {
	if s == nil || index < 0 || index >= len(s.records) {
		return nil, nil, fmt.Errorf("voice spill record %d missing", index)
	}
	rec := s.records[index]
	if rec.length == 0 {
		return nil, rec.offsets, nil
	}
	path := filepath.Join(s.dir, fmt.Sprintf("%d.dat", rec.xuid))
	// #nosec G304 -- path is spill dir + numeric xuid.
	f, err := os.Open(path)
	if err != nil {
		return nil, nil, fmt.Errorf("open spill for packet %d: %w", index, err)
	}
	defer f.Close()
	buf := make([]byte, rec.length)
	if _, err := f.ReadAt(buf, rec.offset); err != nil {
		return nil, nil, fmt.Errorf("read spill for packet %d: %w", index, err)
	}
	dataLen := int(binary.LittleEndian.Uint16(buf[4:6]))
	offsetCount := int(buf[6])
	if offsetCount != len(rec.offsets) {
		return nil, nil, fmt.Errorf("spill offsets mismatch for packet %d", index)
	}
	headerLen := 9 + offsetCount*4
	if headerLen+dataLen != len(buf) {
		return nil, nil, fmt.Errorf("spill length mismatch for packet %d", index)
	}
	return append([]byte(nil), buf[headerLen:]...), rec.offsets, nil
}

type spillFile struct {
	*os.File
}

func (s *packetSpill) file(xuid uint64) (*spillFile, error) {
	if f, ok := s.files[xuid]; ok {
		return f, nil
	}
	if s.files == nil {
		return nil, fmt.Errorf("voice spill is closed")
	}
	if len(s.files) >= 64 {
		return nil, fmt.Errorf("voice spill exceeds speaker limit")
	}
	path := filepath.Join(s.dir, fmt.Sprintf("%d.dat", xuid))
	// #nosec G304 -- path is spill dir + numeric xuid.
	raw, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return nil, fmt.Errorf("open spill file for xuid %d: %w", xuid, err)
	}
	f := &spillFile{File: raw}
	s.files[xuid] = f
	return f, nil
}
