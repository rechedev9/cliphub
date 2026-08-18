// Package steamgc implements the wire protocol for asking the CS2 Game
// Coordinator for a match's demo URL. It encodes the request and decodes the
// reply as raw protobuf bytes. It does NOT open a Steam session, does not do
// networking, and must not import anything that does — the transport is a
// separate, later concern.
package steamgc

import (
	"fmt"
	"net/url"

	"google.golang.org/protobuf/encoding/protowire"
)

// MsgID is a CS2 Game Coordinator message identifier (enum ECsgoGCMsg in
// SteamDatabase/GameTracking-CS2 Protobufs/cstrike15_gcmessages.proto).
type MsgID uint32

// Game Coordinator message identifiers used by this package.
const (
	// MsgMatchListRequestFullGameInfo is k_EMsgGCCStrike15_v2_MatchListRequestFullGameInfo.
	MsgMatchListRequestFullGameInfo MsgID = 9147
	// MsgMatchList is k_EMsgGCCStrike15_v2_MatchList.
	MsgMatchList MsgID = 9139
)

// Request is CMsgGCCStrike15_v2_MatchListRequestFullGameInfo: the three values
// decoded from a CS2 share code that identify one match to the Game Coordinator.
type Request struct {
	MatchID   uint64
	OutcomeID uint64
	Token     uint32
}

// Match is the subset of CDataGCCStrike15_v2_MatchInfo this package decodes.
// DemoURL is empty when the Game Coordinator returned the match without a
// downloadable demo (for example an expired replay); that is not an error.
type Match struct {
	MatchID uint64
	DemoURL string
}

// EncodeRequest serializes r as a CMsgGCCStrike15_v2_MatchListRequestFullGameInfo
// protobuf payload.
func EncodeRequest(r Request) []byte {
	var b []byte
	b = protowire.AppendTag(b, 1, protowire.VarintType)
	b = protowire.AppendVarint(b, r.MatchID)
	b = protowire.AppendTag(b, 2, protowire.VarintType)
	b = protowire.AppendVarint(b, r.OutcomeID)
	b = protowire.AppendTag(b, 3, protowire.VarintType)
	b = protowire.AppendVarint(b, uint64(r.Token))
	return b
}

// DecodeMatchList decodes a CMsgGCCStrike15_v2_MatchList payload, keeping only
// the fields that lead to each match's demo URL. Unknown fields are skipped so
// newly added Game Coordinator fields never break the parse.
func DecodeMatchList(payload []byte) ([]Match, error) {
	var matches []Match
	offset := 0
	b := payload
	for len(b) > 0 {
		num, typ, n := protowire.ConsumeTag(b)
		if n < 0 {
			return nil, fmt.Errorf("steamgc: match list: malformed tag at offset %d: %w", offset, protowire.ParseError(n))
		}
		b = b[n:]
		offset += n
		if num == 4 && typ == protowire.BytesType {
			raw, n := protowire.ConsumeBytes(b)
			if n < 0 {
				return nil, fmt.Errorf("steamgc: match list field 4 (matches): malformed length prefix at offset %d: %w", offset, protowire.ParseError(n))
			}
			m, err := decodeMatchInfo(raw)
			if err != nil {
				return nil, err
			}
			matches = append(matches, m)
			b = b[n:]
			offset += n
			continue
		}
		n = protowire.ConsumeFieldValue(num, typ, b)
		if n < 0 {
			return nil, fmt.Errorf("steamgc: match list field %d: malformed value at offset %d: %w", num, offset, protowire.ParseError(n))
		}
		b = b[n:]
		offset += n
	}
	return matches, nil
}

// decodeMatchInfo decodes a CDataGCCStrike15_v2_MatchInfo message.
func decodeMatchInfo(payload []byte) (Match, error) {
	var m Match
	var lastRoundStats []byte
	haveRoundStats := false
	offset := 0
	b := payload
	for len(b) > 0 {
		num, typ, n := protowire.ConsumeTag(b)
		if n < 0 {
			return Match{}, fmt.Errorf("steamgc: match info: malformed tag at offset %d: %w", offset, protowire.ParseError(n))
		}
		b = b[n:]
		offset += n
		switch {
		case num == 1 && typ == protowire.VarintType:
			v, n := protowire.ConsumeVarint(b)
			if n < 0 {
				return Match{}, fmt.Errorf("steamgc: match info field 1 (matchid): malformed varint at offset %d: %w", offset, protowire.ParseError(n))
			}
			m.MatchID = v
			b = b[n:]
			offset += n
		case num == 5 && typ == protowire.BytesType:
			raw, n := protowire.ConsumeBytes(b)
			if n < 0 {
				return Match{}, fmt.Errorf("steamgc: match info field 5 (roundstatsall): malformed length prefix at offset %d: %w", offset, protowire.ParseError(n))
			}
			lastRoundStats = raw
			haveRoundStats = true
			b = b[n:]
			offset += n
		default:
			n = protowire.ConsumeFieldValue(num, typ, b)
			if n < 0 {
				return Match{}, fmt.Errorf("steamgc: match info field %d: malformed value at offset %d: %w", num, offset, protowire.ParseError(n))
			}
			b = b[n:]
			offset += n
		}
	}
	if !haveRoundStats {
		return m, nil
	}
	mapValue, err := decodeRoundStatsMap(lastRoundStats)
	if err != nil {
		return Match{}, err
	}
	// The map field carries the demo URL only on the last roundstatsall entry;
	// earlier entries hold a real map name like de_mirage. Report it only when
	// it is an absolute http(s) URL; otherwise the demo is not downloadable.
	if u, err := url.Parse(mapValue); err == nil && (u.Scheme == "http" || u.Scheme == "https") && u.Host != "" {
		m.DemoURL = mapValue
	}
	return m, nil
}

// decodeRoundStatsMap extracts field 3 (map) from a
// CMsgGCCStrike15_v2_MatchmakingServerRoundStats message.
func decodeRoundStatsMap(payload []byte) (string, error) {
	var mapValue string
	offset := 0
	b := payload
	for len(b) > 0 {
		num, typ, n := protowire.ConsumeTag(b)
		if n < 0 {
			return "", fmt.Errorf("steamgc: round stats: malformed tag at offset %d: %w", offset, protowire.ParseError(n))
		}
		b = b[n:]
		offset += n
		if num == 3 && typ == protowire.BytesType {
			raw, n := protowire.ConsumeBytes(b)
			if n < 0 {
				return "", fmt.Errorf("steamgc: round stats field 3 (map): malformed length prefix at offset %d: %w", offset, protowire.ParseError(n))
			}
			mapValue = string(raw)
			b = b[n:]
			offset += n
			continue
		}
		n = protowire.ConsumeFieldValue(num, typ, b)
		if n < 0 {
			return "", fmt.Errorf("steamgc: round stats field %d: malformed value at offset %d: %w", num, offset, protowire.ParseError(n))
		}
		b = b[n:]
		offset += n
	}
	return mapValue, nil
}
