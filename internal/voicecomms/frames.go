package voicecomms

func splitVoiceFrames(data []byte, offsets []uint32) [][]byte {
	if len(data) == 0 {
		return nil
	}
	if len(offsets) == 0 {
		return [][]byte{append([]byte(nil), data...)}
	}
	if isStartOffsets(offsets, len(data)) {
		out := make([][]byte, 0, len(offsets))
		for i, off := range offsets {
			if int(off) >= len(data) {
				break
			}
			end := len(data)
			if i+1 < len(offsets) && int(offsets[i+1]) <= len(data) {
				end = int(offsets[i+1])
			}
			if end > int(off) {
				out = append(out, append([]byte(nil), data[off:end]...))
			}
		}
		return out
	}
	out := make([][]byte, 0, len(offsets))
	pos := 0
	for _, n := range offsets {
		next := pos + int(n)
		if n == 0 || next > len(data) {
			break
		}
		out = append(out, append([]byte(nil), data[pos:next]...))
		pos = next
	}
	if len(out) == 0 {
		return [][]byte{append([]byte(nil), data...)}
	}
	return out
}

func isStartOffsets(offsets []uint32, dataLen int) bool {
	if len(offsets) == 0 || offsets[0] != 0 {
		return false
	}
	for i := 1; i < len(offsets); i++ {
		if offsets[i] <= offsets[i-1] {
			return false
		}
	}
	return int(offsets[len(offsets)-1]) < dataLen
}
