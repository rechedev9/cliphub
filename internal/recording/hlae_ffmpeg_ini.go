package recording

import (
	"bytes"
	"encoding/binary"
	"strings"
	"unicode/utf16"
)

var utf16LEBOM = []byte{0xFF, 0xFE}

// HLAEFFmpegINI renders HLAE's ffmpeg\ffmpeg.ini for an ffmpeg path. HLAE reads
// it with GetPrivateProfileStringW, which decodes a file without a BOM in the
// ANSI code page, so UTF-16LE with a BOM is the only form that keeps a path
// like C:\Users\José intact.
func HLAEFFmpegINI(ffmpegPath string) []byte {
	units := utf16.Encode([]rune("[Ffmpeg]\r\nPath=" + ffmpegPath + "\r\n"))
	out := make([]byte, len(utf16LEBOM), len(utf16LEBOM)+2*len(units))
	copy(out, utf16LEBOM)
	for _, unit := range units {
		out = binary.LittleEndian.AppendUint16(out, unit)
	}
	return out
}

// HLAEFFmpegINIPath returns the Path an ffmpeg.ini declares, from either the
// UTF-16LE file Studio writes or an older single-byte one.
func HLAEFFmpegINIPath(data []byte) string {
	text := string(data)
	if rest, ok := bytes.CutPrefix(data, utf16LEBOM); ok {
		units := make([]uint16, len(rest)/2)
		for i := range units {
			units[i] = binary.LittleEndian.Uint16(rest[2*i:])
		}
		text = string(utf16.Decode(units))
	}
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if path, ok := strings.CutPrefix(line, "Path="); ok && strings.TrimSpace(path) != "" {
			return strings.TrimSpace(path)
		}
	}
	return ""
}
