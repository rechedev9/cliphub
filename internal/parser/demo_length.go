package parser

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
)

// DemoHeaderLen is how many leading bytes of a CS2 demo CheckDemoLength reads:
// the "PBDEMS2\x00" magic, then the int32 offsets of the file info and of the
// spawn groups, both written when CS2 stops the recording.
const DemoHeaderLen = 16

// ErrTruncatedDemo is a demo that ends before its recording does: a cut
// download or copy, or a recording stopped before the first round. The parser
// reads such a file without complaint, so it has to be refused by name.
var ErrTruncatedDemo = errors.New("truncated_demo: the demo file is incomplete")

// maxFileInfoOffset bounds a file info offset worth checking. Studio admits
// demos up to 700 MiB, so a larger offset is not one CS2 wrote for a demo it reads.
const maxFileInfoOffset = 700 << 20

// CheckDemoLength returns ErrTruncatedDemo when header puts the demo's file
// info at or past size bytes. A zero offset is a recording CS2 never closed;
// RosterScan judges that one by the rounds it holds.
func CheckDemoLength(header []byte, size int64) error {
	fileInfo, ok := fileInfoOffset(header)
	if ok && fileInfo > 0 && fileInfo <= maxFileInfoOffset && fileInfo >= size {
		return fmt.Errorf("%w: it has %d bytes but its header puts the file info at byte %d", ErrTruncatedDemo, size, fileInfo)
	}
	return nil
}

// DemoRecordingUnclosed reports a CS2 demo header without its file info
// offset: the recording stopped before CS2 closed the file.
func DemoRecordingUnclosed(header []byte) bool {
	fileInfo, ok := fileInfoOffset(header)
	return ok && fileInfo == 0
}

func fileInfoOffset(header []byte) (int64, bool) {
	if len(header) < DemoHeaderLen || !bytes.HasPrefix(header, []byte("PBDEMS2\x00")) {
		return 0, false
	}
	return int64(binary.LittleEndian.Uint32(header[8:12])), true
}
