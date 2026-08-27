package voicecomms

import "testing"

func TestPacketTickPrefersIngameClock(t *testing.T) {
	tests := []struct {
		name       string
		protoTick  int
		ingameTick int
		want       int
	}{
		{name: "zero proto uses ingame", protoTick: 0, ingameTick: 5487, want: 5487},
		{name: "nonzero proto still uses ingame", protoTick: 12140, ingameTick: 100, want: 100},
		{name: "ingame zero falls back to proto", protoTick: 12140, ingameTick: 0, want: 12140},
		{name: "both zero", protoTick: 0, ingameTick: 0, want: 0},
		{name: "negative ingame falls back to proto", protoTick: 80, ingameTick: -1, want: 80},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := packetTick(tt.protoTick, tt.ingameTick)
			if got != tt.want {
				t.Fatalf("packetTick(%d, %d) = %d, want %d", tt.protoTick, tt.ingameTick, got, tt.want)
			}
		})
	}
}
