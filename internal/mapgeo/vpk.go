package mapgeo

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

const (
	vpkSignature = 0x55aa1234
	// vpkInline marks an entry stored in the directory file itself, which is
	// how CS2 packs every per-map VPK.
	vpkInline     = 0x7fff
	vpkTerminator = 0xffff
	// maxVPKEntryBytes caps one entry so a corrupt length cannot ask for gigabytes.
	maxVPKEntryBytes = 256 << 20
)

// ErrNotInVPK reports that the archive has no entry with the requested path.
var ErrNotInVPK = errors.New("file not in vpk")

type vpkEntry struct {
	preload []byte
	archive uint16
	offset  uint32
	length  uint32
}

// VPK is an open Valve package (versions 1 and 2). A directory file named
// <prefix>_dir.vpk reads entries split into <prefix>_NNN.vpk archives.
type VPK struct {
	file     *os.File
	path     string
	dataBase int64
	entries  map[string]vpkEntry

	mu       sync.Mutex
	archives map[uint16]*os.File
}

// OpenVPK reads the directory tree of a VPK file. Close releases the file.
func OpenVPK(path string) (*VPK, error) {
	// #nosec G304 -- path is a map package under the detected CS2 install.
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	v, err := readVPKDirectory(f)
	if err != nil {
		_ = f.Close()
		return nil, fmt.Errorf("read vpk %s: %w", path, err)
	}
	v.path = path
	return v, nil
}

func readVPKDirectory(f *os.File) (*VPK, error) {
	var head [12]byte
	if _, err := io.ReadFull(f, head[:]); err != nil {
		return nil, fmt.Errorf("header: %w", err)
	}
	if binary.LittleEndian.Uint32(head[0:]) != vpkSignature {
		return nil, errors.New("not a vpk file")
	}
	version := binary.LittleEndian.Uint32(head[4:])
	treeSize := binary.LittleEndian.Uint32(head[8:])
	headerSize := int64(12)
	switch version {
	case 1:
	case 2:
		headerSize = 28
	default:
		return nil, fmt.Errorf("unsupported vpk version %d", version)
	}
	info, err := f.Stat()
	if err != nil {
		return nil, err
	}
	if headerSize+int64(treeSize) > info.Size() {
		return nil, fmt.Errorf("tree size %d exceeds file size %d", treeSize, info.Size())
	}
	tree := make([]byte, treeSize)
	if _, err := f.ReadAt(tree, headerSize); err != nil {
		return nil, fmt.Errorf("tree: %w", err)
	}
	entries, err := parseVPKTree(tree)
	if err != nil {
		return nil, err
	}
	return &VPK{file: f, dataBase: headerSize + int64(treeSize), entries: entries}, nil
}

// parseVPKTree walks the extension / directory / file-name nesting of the tree.
func parseVPKTree(tree []byte) (map[string]vpkEntry, error) {
	r := &byteReader{data: tree}
	entries := map[string]vpkEntry{}
	for {
		ext := r.cstring()
		if r.err != nil || ext == "" {
			return entries, r.err
		}
		for {
			dir := r.cstring()
			if r.err != nil || dir == "" {
				break
			}
			for {
				name := r.cstring()
				if r.err != nil || name == "" {
					break
				}
				entry, err := readVPKEntry(r)
				if err != nil {
					return nil, fmt.Errorf("entry %s/%s.%s: %w", dir, name, ext, err)
				}
				entries[vpkPath(dir, name, ext)] = entry
			}
		}
		if r.err != nil {
			return nil, r.err
		}
	}
}

func readVPKEntry(r *byteReader) (vpkEntry, error) {
	r.u32() // CRC32, not checked: the archive is read from a local install
	preloadSize := r.u16()
	entry := vpkEntry{archive: r.u16(), offset: r.u32(), length: r.u32()}
	if r.u16() != vpkTerminator && r.err == nil {
		return vpkEntry{}, errors.New("bad entry terminator")
	}
	entry.preload = r.bytes(int(preloadSize))
	return entry, r.err
}

// vpkPath joins tree parts the way Valve tools name files; a single space
// stands for an empty directory or extension.
func vpkPath(dir, name, ext string) string {
	path := name
	if ext != " " {
		path += "." + ext
	}
	if dir != " " {
		path = dir + "/" + path
	}
	return path
}

// ReadFile returns the full contents of one entry.
func (v *VPK) ReadFile(path string) ([]byte, error) {
	entry, ok := v.entries[path]
	if !ok {
		return nil, fmt.Errorf("%w: %s", ErrNotInVPK, path)
	}
	if entry.length == 0 {
		return bytes.Clone(entry.preload), nil
	}
	if int64(entry.length)+int64(len(entry.preload)) > maxVPKEntryBytes {
		return nil, fmt.Errorf("%s is %d bytes, over the %d byte limit", path, entry.length, maxVPKEntryBytes)
	}
	src, base := v.file, v.dataBase
	if entry.archive != vpkInline {
		archive, err := v.archive(entry.archive)
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", path, err)
		}
		src, base = archive, 0
	}
	out := make([]byte, len(entry.preload)+int(entry.length))
	copy(out, entry.preload)
	if _, err := src.ReadAt(out[len(entry.preload):], base+int64(entry.offset)); err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	return out, nil
}

// archive opens <prefix>_NNN.vpk next to the directory file once and keeps it.
func (v *VPK) archive(index uint16) (*os.File, error) {
	v.mu.Lock()
	defer v.mu.Unlock()
	if f, ok := v.archives[index]; ok {
		return f, nil
	}
	prefix, ok := strings.CutSuffix(v.path, "_dir.vpk")
	if !ok {
		return nil, fmt.Errorf("split archive %d needs a _dir.vpk directory file", index)
	}
	name := fmt.Sprintf("%s_%03d.vpk", prefix, index)
	// #nosec G304 -- sibling archive of the opened directory file.
	f, err := os.Open(filepath.Clean(name))
	if err != nil {
		return nil, fmt.Errorf("open split archive: %w", err)
	}
	if v.archives == nil {
		v.archives = map[uint16]*os.File{}
	}
	v.archives[index] = f
	return f, nil
}

// Close releases the directory file and every split archive it opened.
func (v *VPK) Close() error {
	v.mu.Lock()
	defer v.mu.Unlock()
	errs := []error{v.file.Close()}
	for _, f := range v.archives {
		errs = append(errs, f.Close())
	}
	v.archives = nil
	return errors.Join(errs...)
}
