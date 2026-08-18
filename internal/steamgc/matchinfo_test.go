package steamgc

import (
	"testing"

	"google.golang.org/protobuf/encoding/protowire"
)

// appendField appends a length-delimited field to b.
func appendField(b []byte, num protowire.Number, raw []byte) []byte {
	b = protowire.AppendTag(b, num, protowire.BytesType)
	return protowire.AppendBytes(b, raw)
}

// appendVarintField appends a varint field to b.
func appendVarintField(b []byte, num protowire.Number, v uint64) []byte {
	b = protowire.AppendTag(b, num, protowire.VarintType)
	return protowire.AppendVarint(b, v)
}

// roundStats builds a CMsgGCCStrike15_v2_MatchmakingServerRoundStats payload
// whose field 3 (map) holds mapValue, with extra unknown fields interleaved
// when withUnknown is set.
func roundStats(mapValue string, withUnknown bool) []byte {
	var b []byte
	if withUnknown {
		b = appendVarintField(b, 1, 42)                // reservationid-like unknown
		b = appendField(b, 2, []byte("unknown-bytes")) // unknown length-delimited
	}
	b = appendField(b, 3, []byte(mapValue))
	if withUnknown {
		b = appendVarintField(b, 99, 7)
	}
	return b
}

// matchInfo builds a CDataGCCStrike15_v2_MatchInfo payload with the given
// matchid and roundstatsall entries.
func matchInfo(matchID uint64, rounds [][]byte, withUnknown bool) []byte {
	var b []byte
	if withUnknown {
		b = appendField(b, 2, []byte("watchablematchinfo"))
	}
	b = appendVarintField(b, 1, matchID)
	for _, r := range rounds {
		b = appendField(b, 5, r)
		if withUnknown {
			b = appendVarintField(b, 3, 1700000000) // matchtime-like unknown
		}
	}
	return b
}

// matchList builds a CMsgGCCStrike15_v2_MatchList payload from match payloads.
func matchList(matchPayloads [][]byte, withUnknown bool) []byte {
	var b []byte
	if withUnknown {
		b = appendVarintField(b, 1, 9139) // msgrequestid-like unknown
	}
	for _, m := range matchPayloads {
		b = appendField(b, 4, m)
	}
	if withUnknown {
		b = appendField(b, 8, []byte("streams"))
	}
	return b
}

func TestEncodeRequest(t *testing.T) {
	tests := []struct {
		name string
		req  Request
	}{
		{
			name: "share code values",
			req:  Request{MatchID: 3230642215713767580, OutcomeID: 3230647599455273103, Token: 55788},
		},
		{
			name: "zero values",
			req:  Request{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := EncodeRequest(tt.req)
			if len(got) == 0 || got[0] != 0x08 {
				t.Fatalf("first tag byte = %#x, want 0x08 (buf %x)", got[0], got)
			}

			// Re-read the buffer and assert tag bytes and values in order.
			b := got
			wantTags := []byte{0x08, 0x10, 0x18}
			wantVals := []uint64{tt.req.MatchID, tt.req.OutcomeID, uint64(tt.req.Token)}
			for i, wantTag := range wantTags {
				if b[0] != wantTag {
					t.Fatalf("field %d tag byte = %#x, want %#x", i+1, b[0], wantTag)
				}
				num, typ, n := protowire.ConsumeTag(b)
				if n < 0 || num != protowire.Number(i+1) || typ != protowire.VarintType {
					t.Fatalf("field %d tag = (%d, %d, %d), want (%d, varint)", i+1, num, typ, n, i+1)
				}
				b = b[n:]
				v, n := protowire.ConsumeVarint(b)
				if n < 0 {
					t.Fatalf("field %d: malformed varint", i+1)
				}
				if v != wantVals[i] {
					t.Fatalf("field %d value = %d, want %d", i+1, v, wantVals[i])
				}
				b = b[n:]
			}
			if len(b) != 0 {
				t.Fatalf("trailing bytes after three fields: %x", b)
			}
		})
	}
}

func TestDecodeMatchList(t *testing.T) {
	const demoURL = "http://replay123.valve.net/730/003230642215713767580_1234567890.dem.bz2"
	const matchID = uint64(3230642215713767580)

	tests := []struct {
		name    string
		payload []byte
		want    []Match
		wantErr bool
	}{
		{
			name: "happy path: last roundstatsall carries demo URL",
			payload: matchList([][]byte{
				matchInfo(matchID, [][]byte{
					roundStats("de_mirage", false),
					roundStats("de_mirage", false),
					roundStats(demoURL, false),
				}, false),
			}, false),
			want: []Match{{MatchID: matchID, DemoURL: demoURL}},
		},
		{
			name: "unknown fields interleaved at every nesting level are skipped",
			payload: matchList([][]byte{
				matchInfo(matchID, [][]byte{
					roundStats("de_mirage", true),
					roundStats(demoURL, true),
				}, true),
			}, true),
			want: []Match{{MatchID: matchID, DemoURL: demoURL}},
		},
		{
			name: "last map is a map name, not a URL",
			payload: matchList([][]byte{
				matchInfo(matchID, [][]byte{
					roundStats("de_mirage", false),
				}, false),
			}, false),
			want: []Match{{MatchID: matchID}},
		},
		{
			name: "zero roundstatsall entries",
			payload: matchList([][]byte{
				matchInfo(matchID, nil, false),
			}, false),
			want: []Match{{MatchID: matchID}},
		},
		{
			name:    "empty payload yields zero matches and no error",
			payload: nil,
			want:    nil,
		},
		{
			name: "truncated bytes yield an error",
			payload: matchList([][]byte{
				matchInfo(matchID, [][]byte{roundStats(demoURL, false)}, false),
			}, false)[:10],
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := DecodeMatchList(tt.payload)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("DecodeMatchList() = %+v, want error", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("DecodeMatchList() error: %v", err)
			}
			if len(got) != len(tt.want) {
				t.Fatalf("DecodeMatchList() = %+v, want %+v", got, tt.want)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Fatalf("match %d = %+v, want %+v", i, got[i], tt.want[i])
				}
			}
		})
	}
}
