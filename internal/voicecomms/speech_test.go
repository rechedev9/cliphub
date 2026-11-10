package voicecomms

import (
	"reflect"
	"testing"
)

func TestSpeechByPlayerMergesTouchingPackets(t *testing.T) {
	packets := []Packet{
		{XUID: 7, Tick: 102, Bytes: 40, DurationSamples: 960},
		{XUID: 7, Tick: 100, Bytes: 40, DurationSamples: 960},
		{XUID: 7, Tick: 101, Bytes: 40, DurationSamples: 960},
		{XUID: 7, Tick: 200, Bytes: 40},
		{XUID: 9, Tick: 150, Bytes: 40, DurationSamples: 48000},
		{XUID: 9, Tick: 0, Bytes: 40, DurationSamples: 960},
		{XUID: 9, Tick: 300},
	}
	got := SpeechByPlayer(packets, 64)
	want := map[string][]ActivityRange{
		"7": {{StartTick: 100, EndTick: 104}, {StartTick: 200, EndTick: 202}},
		"9": {{StartTick: 150, EndTick: 214}},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("SpeechByPlayer = %+v, want %+v", got, want)
	}
	if len(SpeechByPlayer(packets, 0)) != 0 {
		t.Fatal("a zero tickrate must give no speech")
	}
}
