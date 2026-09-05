package sharecode

import (
	"encoding/binary"
	"fmt"
	"reflect"
	"testing"
)

func TestCrosshairInteroperabilityAndChecksum(t *testing.T) {
	const code = "CSGO-WsnnD-eHaMw-QNDf9-oxuDh-ydOUD"
	want := map[string]float64{"cl_crosshairgap": -2.2, "cl_crosshair_outlinethickness": 1, "cl_crosshaircolor_r": 50, "cl_crosshaircolor_g": 250, "cl_crosshaircolor_b": 50, "cl_crosshairalpha": 200, "cl_crosshair_dynamic_splitdist": 3, "cl_crosshair_recoil": 1, "cl_fixedcrosshairgap": 3, "cl_crosshaircolor": 1, "cl_crosshair_drawoutline": 1, "cl_crosshair_dynamic_splitalpha_innermod": 0, "cl_crosshair_dynamic_splitalpha_outermod": 1, "cl_crosshair_dynamic_maxdist_splitratio": 1, "cl_crosshairthickness": .6, "cl_crosshairdot": 0, "cl_crosshairgap_useweaponvalue": 1, "cl_crosshairusealpha": 1, "cl_crosshair_t": 0, "cl_crosshairstyle": 2, "cl_crosshairsize": 10}
	got, err := CrosshairCvars(code)
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Fatalf("decoded crosshair=%v, err=%v", got, err)
	}
	for _, tc := range []struct{ name, code string }{
		{"changed character", "CSGO-WsnnE-eHaMw-QNDf9-oxuDh-ydOUD"},
		{"match code", "CSGO-GADqf-jjyJ8-cSP2r-smZRo-TO2xK"},
		{"shape only", "CSGO-AAAAA-AAAAA-AAAAA-AAAAA-AAAAA"},
		{"overflow", "CSGO-99999-99999-99999-99999-99999"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := CrosshairCvars(tc.code); err == nil {
				t.Fatal("invalid crosshair accepted")
			}
		})
	}
	for _, field := range []int{1, 15, 16, 17} {
		t.Run(fmt.Sprintf("unsupported byte %d", field), func(t *testing.T) {
			m, _ := Decode(code)
			var b [18]byte
			binary.LittleEndian.PutUint64(b[:8], m.MatchID)
			binary.LittleEndian.PutUint64(b[8:16], m.OutcomeID)
			binary.LittleEndian.PutUint16(b[16:], m.TokenID)
			b[field]++
			b[0] = 0
			for _, v := range b[1:] {
				b[0] += v
			}
			unknown := Encode(Match{MatchID: binary.LittleEndian.Uint64(b[:8]), OutcomeID: binary.LittleEndian.Uint64(b[8:16]), TokenID: binary.LittleEndian.Uint16(b[16:])})
			if _, err := CrosshairCvars(unknown); err == nil {
				t.Fatal("unknown payload accepted despite valid checksum")
			}
		})
	}
}
