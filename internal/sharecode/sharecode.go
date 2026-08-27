// Package sharecode encodes and decodes CS2 match sharing codes
// (CSGO-xxxxx-xxxxx-xxxxx-xxxxx-xxxxx).
//
// Decoding is pure offline arithmetic and needs no Steam session or
// credentials. Resolving the decoded identifiers to a downloadable .dem
// requires a logged-in CS2 Game Coordinator connection and is deliberately
// NOT part of this package.
package sharecode

import (
	"encoding/binary"
	"fmt"
	"math/big"
	"strings"
)

// dictionary is the base-57 alphabet: uppercase A-Z without I, lowercase a-z
// without g and l, digits 2-9.
const dictionary = "ABCDEFGHJKLMNOPQRSTUVWXYZabcdefhijkmnopqrstuvwxyz23456789"

const (
	prefix     = "CSGO-"
	codeLength = 25
	byteLength = 18
)

// Match holds the three integers packed into a share code.
// TokenID is the 16-bit field at bytes 16..17 of the 18-byte payload.
type Match struct {
	MatchID   uint64
	OutcomeID uint64
	TokenID   uint16
}

// Decode parses a share code, with or without the CSGO- prefix, into a Match.
func Decode(code string) (Match, error) {
	stripped := strings.TrimPrefix(code, prefix)
	stripped = strings.ReplaceAll(stripped, "-", "")
	if len(stripped) != codeLength {
		return Match{}, fmt.Errorf("share code %q: expected %d characters after separators, got %d", code, codeLength, len(stripped))
	}

	acc := new(big.Int)
	base := big.NewInt(int64(len(dictionary)))
	digit := new(big.Int)
	for i := len(stripped) - 1; i >= 0; i-- {
		idx := strings.IndexByte(dictionary, stripped[i])
		if idx < 0 {
			return Match{}, fmt.Errorf("share code %q: character %q is not in the share code dictionary", code, stripped[i])
		}
		acc.Mul(acc, base)
		acc.Add(acc, digit.SetInt64(int64(idx)))
	}

	// Byte order established against the known-good vector
	// CSGO-GADqf-jjyJ8-cSP2r-smZRo-TO2xK: the 144-bit accumulator is laid out
	// as its 18 big-endian bytes consumed forward, and each field is read
	// little-endian: MatchID from bytes 0..7, OutcomeID from bytes 8..15,
	// TokenID from bytes 16..17.
	if acc.BitLen() > 8*byteLength {
		return Match{}, fmt.Errorf("share code %q: value exceeds 144 bits", code)
	}
	var buf [byteLength]byte
	acc.FillBytes(buf[:])
	return Match{
		MatchID:   binary.LittleEndian.Uint64(buf[0:8]),
		OutcomeID: binary.LittleEndian.Uint64(buf[8:16]),
		TokenID:   binary.LittleEndian.Uint16(buf[16:18]),
	}, nil
}

// Encode packs a Match back into a CSGO- prefixed share code.
func Encode(m Match) string {
	var buf [byteLength]byte
	binary.LittleEndian.PutUint64(buf[0:8], m.MatchID)
	binary.LittleEndian.PutUint64(buf[8:16], m.OutcomeID)
	binary.LittleEndian.PutUint16(buf[16:18], m.TokenID)

	acc := new(big.Int).SetBytes(buf[:])
	base := big.NewInt(int64(len(dictionary)))
	rem := new(big.Int)

	chars := make([]byte, 0, codeLength)
	for i := 0; i < codeLength; i++ {
		acc.QuoRem(acc, base, rem)
		chars = append(chars, dictionary[rem.Int64()])
	}

	var b strings.Builder
	b.WriteString(prefix)
	for i := 0; i < codeLength; i += 5 {
		if i > 0 {
			b.WriteByte('-')
		}
		b.Write(chars[i : i+5])
	}
	return b.String()
}
