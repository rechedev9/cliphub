package mapgeo

import "errors"

var errLZ4 = errors.New("corrupt lz4 block")

// lz4DecodeBlock decodes one raw LZ4 block into dst[start:], using dst[:start]
// as history, and returns the number of bytes written. KV3 chains its blob
// frames this way, so a match may reach back into an earlier frame.
func lz4DecodeBlock(src, dst []byte, start int) (int, error) {
	out := start
	for i := 0; i < len(src); {
		token := src[i]
		i++
		literals := int(token >> 4)
		if literals == 15 {
			n, next, err := lz4Length(src, i)
			if err != nil {
				return 0, err
			}
			literals, i = literals+n, next
		}
		if literals > len(src)-i || literals > len(dst)-out {
			return 0, errLZ4
		}
		out += copy(dst[out:], src[i:i+literals])
		i += literals
		if i == len(src) {
			break // the last sequence carries literals only
		}
		if i+2 > len(src) {
			return 0, errLZ4
		}
		offset := int(src[i]) | int(src[i+1])<<8
		i += 2
		if offset == 0 || offset > out {
			return 0, errLZ4
		}
		match := int(token&15) + 4
		if token&15 == 15 {
			n, next, err := lz4Length(src, i)
			if err != nil {
				return 0, err
			}
			match, i = match+n, next
		}
		if match > len(dst)-out {
			return 0, errLZ4
		}
		// Byte by byte: a match may overlap the bytes it is producing.
		for from := out - offset; match > 0; match-- {
			dst[out] = dst[from]
			out++
			from++
		}
	}
	return out - start, nil
}

func lz4Length(src []byte, i int) (int, int, error) {
	n := 0
	for {
		if i >= len(src) {
			return 0, 0, errLZ4
		}
		b := src[i]
		i++
		n += int(b)
		if b != 255 {
			return n, i, nil
		}
	}
}
