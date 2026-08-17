package parser

import "testing"

func TestFirstPacketWatchSnapshotUsesIngameTick(t *testing.T) {
	w := &FirstPacketWatch{seen: true, ingame: 1, sawNet: true, netTick: 5326}
	tick, seen := w.Snapshot()
	if !seen || tick != 1 {
		t.Fatalf("Snapshot() = %d, %v; want ingame 1, not net tick 5326", tick, seen)
	}
}

func TestClassifyPlayability(t *testing.T) {
	cases := []struct {
		name      string
		tickrate  int
		firstTick int
		sawPacket bool
		want      PlayabilityClass
	}{
		{name: "no packet", sawPacket: false, want: PlayabilityCorrupt},
		{name: "missing tickrate", tickrate: 0, firstTick: 10, sawPacket: true, want: PlayabilityUnknown},
		{name: "negative tickrate", tickrate: -1, firstTick: 0, sawPacket: true, want: PlayabilityUnknown},
		{name: "64 playable at 0", tickrate: 64, firstTick: 0, sawPacket: true, want: PlayabilityPlayable},
		{name: "64 playable at rate", tickrate: 64, firstTick: 64, sawPacket: true, want: PlayabilityPlayable},
		{name: "64 unplayable at 65", tickrate: 64, firstTick: 65, sawPacket: true, want: PlayabilityUnplayableStart},
		{name: "64 unplayable at 5328", tickrate: 64, firstTick: 5328, sawPacket: true, want: PlayabilityUnplayableStart},
		{name: "128 playable mid", tickrate: 128, firstTick: 100, sawPacket: true, want: PlayabilityPlayable},
		{name: "128 unplayable after rate", tickrate: 128, firstTick: 129, sawPacket: true, want: PlayabilityUnplayableStart},
		{name: "32 playable", tickrate: 32, firstTick: 16, sawPacket: true, want: PlayabilityPlayable},
		{name: "32 gap unknown", tickrate: 32, firstTick: 40, sawPacket: true, want: PlayabilityUnknown},
		{name: "32 unplayable after 64", tickrate: 32, firstTick: 65, sawPacket: true, want: PlayabilityUnplayableStart},
		{name: "negative first tick", tickrate: 64, firstTick: -1, sawPacket: true, want: PlayabilityUnknown},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := ClassifyPlayability(tc.tickrate, tc.firstTick, tc.sawPacket)
			if got != tc.want {
				t.Fatalf("ClassifyPlayability(%d, %d, %v) = %q, want %q",
					tc.tickrate, tc.firstTick, tc.sawPacket, got, tc.want)
			}
		})
	}
}
