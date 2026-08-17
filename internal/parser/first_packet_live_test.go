package parser

import (
	"os"
	"testing"
)

func TestProbeDemoKnownLocalHalves(t *testing.T) {
	cases := []struct {
		name string
		path string
	}{
		{"first-half", `C:\Users\reche\AppData\Roaming\cliphub-studio\data\demos\a03f752b-cd6c-410c-8327-4e57206e65e3.dem`},
		{"second-half", `C:\Users\reche\AppData\Roaming\cliphub-studio\data\demos\5a44a22b-47a7-43a8-86d8-1af4eab62b26.dem`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := os.Stat(tc.path); err != nil {
				t.Skip(tc.path)
			}
			rep, err := ProbeDemo(tc.path)
			if err != nil {
				t.Fatal(err)
			}
			t.Logf("class=%s full=%d ingame=%d net=%d rate=%d map=%s",
				rep.Class, rep.FirstFullPacketTick, rep.FirstIngameTick, rep.FirstNetTick, rep.Tickrate, rep.Map)
		})
	}
}
