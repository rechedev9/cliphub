package voicecomms

import (
	"errors"
	"testing"
)

func TestClassify(t *testing.T) {
	const (
		target  = "76561198190463461"
		mateA   = "76561198186361079"
		mateB   = "76561199031491193"
		enemy   = "76561199101837428"
		unknown = "76561198000000000"
	)
	sighted := []Sighting{
		{SteamID64: target, Name: "s1n", Team: "T"},
		{SteamID64: mateA, Name: "FinigaN", Team: "T"},
		{SteamID64: mateB, Name: "1njustice", Team: "T"},
		{SteamID64: enemy, Name: "parker1778", Team: "CT"},
	}
	meta := Meta{Demo: "match.dem", Map: "de_mirage", Tickrate: 64, DurationTicks: 100000}

	tests := []struct {
		name      string
		target    string
		packets   []Packet
		sightings []Sighting
		wantErr   error
		check     func(t *testing.T, got Report)
	}{
		{
			name:      "no packets",
			target:    target,
			sightings: sighted,
			check: func(t *testing.T, got Report) {
				if got.VoicePresent || got.Format != FormatNone || got.TotalPackets != 0 {
					t.Fatalf("report = %+v, want empty", got)
				}
				if got.Target.SteamID64 != target || got.Target.Name != "s1n" || got.Target.Team != "T" {
					t.Fatalf("target = %+v", got.Target)
				}
				if got.Others.Players != 0 || got.Others.Packets != 0 {
					t.Fatalf("others = %+v", got.Others)
				}
			},
		},
		{
			name:   "team packets listed, others aggregated",
			target: target,
			packets: []Packet{
				{XUID: mustID(target), Tick: 1000, Bytes: 40, Format: FormatOpus, SampleRate: 24000},
				{XUID: mustID(mateA), Tick: 1100, Bytes: 80, Format: FormatOpus, SampleRate: 24000},
				{XUID: mustID(mateA), Tick: 1200, Bytes: 80, Format: FormatOpus, SampleRate: 24000},
				{XUID: mustID(enemy), Tick: 1300, Bytes: 60, Format: FormatOpus, SampleRate: 24000},
				{XUID: mustID(enemy), Tick: 1400, Bytes: 60, Format: FormatOpus, SampleRate: 24000},
				{XUID: mustID(enemy), Tick: 1500, Bytes: 60, Format: FormatOpus, SampleRate: 24000},
			},
			sightings: sighted,
			check: func(t *testing.T, got Report) {
				if !got.VoicePresent || got.Format != FormatOpus || got.SampleRateHz != 24000 || got.TotalPackets != 6 {
					t.Fatalf("header = present=%v format=%s rate=%d packets=%d", got.VoicePresent, got.Format, got.SampleRateHz, got.TotalPackets)
				}
				if got.Target.Packets != 1 || got.Target.Bytes != 40 {
					t.Fatalf("target stats = %+v", got.Target)
				}
				if len(got.Teammates) != 1 {
					t.Fatalf("teammates = %#v, want FinigaN only", got.Teammates)
				}
				mate := got.Teammates[0]
				if mate.SteamID64 != mateA || mate.Name != "FinigaN" || mate.Packets != 2 || mate.Bytes != 160 {
					t.Fatalf("mate = %+v", mate)
				}
				if mate.FirstTick != 1100 || mate.LastTick != 1200 || mate.SecondsEstimate != 0.04 {
					t.Fatalf("mate window = %+v", mate)
				}
				if got.Others.Players != 1 || got.Others.Packets != 3 || got.Others.Bytes != 180 {
					t.Fatalf("others = %+v, want 1 player / 3 packets", got.Others)
				}
			},
		},
		{
			name:      "target never sighted",
			target:    unknown,
			sightings: sighted,
			wantErr:   ErrTargetNotFound,
		},
		{
			name:      "invalid steamid",
			target:    "not-a-steamid",
			sightings: sighted,
			wantErr:   ErrInvalidTarget,
		},
		{
			name:   "mixed formats",
			target: target,
			packets: []Packet{
				{XUID: mustID(target), Tick: 10, Bytes: 8, Format: FormatOpus, SampleRate: 24000},
				{XUID: mustID(target), Tick: 20, Bytes: 8, Format: FormatSteam, SampleRate: 24000},
			},
			sightings: sighted,
			check: func(t *testing.T, got Report) {
				if got.Format != FormatMixed {
					t.Fatalf("format = %q, want mixed", got.Format)
				}
			},
		},
		{
			name:   "teammates sorted by steamid",
			target: target,
			packets: []Packet{
				{XUID: mustID(mateB), Tick: 1, Bytes: 1, Format: FormatOpus, SampleRate: 24000},
				{XUID: mustID(mateA), Tick: 2, Bytes: 1, Format: FormatOpus, SampleRate: 24000},
			},
			sightings: sighted,
			check: func(t *testing.T, got Report) {
				if len(got.Teammates) != 2 {
					t.Fatalf("teammates len = %d", len(got.Teammates))
				}
				if got.Teammates[0].SteamID64 != mateA || got.Teammates[1].SteamID64 != mateB {
					t.Fatalf("order = %q, %q", got.Teammates[0].SteamID64, got.Teammates[1].SteamID64)
				}
			},
		},
		{
			name:   "half-time swap classifies by side at speak tick",
			target: target,
			packets: []Packet{
				{XUID: mustID(mateA), Tick: 100, Bytes: 10, Format: FormatOpus, SampleRate: 24000},
				{XUID: mustID(mateA), Tick: 20000, Bytes: 10, Format: FormatOpus, SampleRate: 24000},
			},
			sightings: []Sighting{
				{SteamID64: target, Name: "s1n", Team: "T", Tick: 0},
				{SteamID64: mateA, Name: "FinigaN", Team: "T", Tick: 0},
				{SteamID64: target, Name: "s1n", Team: "CT", Tick: 10000},
				{SteamID64: mateA, Name: "FinigaN", Team: "CT", Tick: 10000},
			},
			check: func(t *testing.T, got Report) {
				if len(got.Teammates) != 1 || got.Teammates[0].Packets != 2 {
					t.Fatalf("teammates = %#v, want FinigaN both halves", got.Teammates)
				}
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Classify(tt.target, tt.packets, tt.sightings, meta)
			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("err = %v, want %v", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if got.SchemaVersion != SchemaVersion {
				t.Fatalf("schema = %q", got.SchemaVersion)
			}
			if got.Demo != meta.Demo || got.Map != meta.Map || got.Tickrate != 64 {
				t.Fatalf("meta = %+v", got)
			}
			if len(got.Limitations) == 0 {
				t.Fatal("missing limitations")
			}
			tt.check(t, got)
		})
	}
}

func mustID(s string) uint64 {
	var id uint64
	for _, r := range s {
		id = id*10 + uint64(r-'0')
	}
	return id
}
