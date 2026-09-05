package sharecode

import (
	"encoding/binary"
	"fmt"
)

// CrosshairCvars decodes the version-1 crosshair payload into CS2 settings.
// Layout and interoperability vector: github.com/akiver/csgo-sharecode.
// It uses the same offline base-57 arithmetic as match codes.
func CrosshairCvars(code string) (map[string]float64, error) {
	m, err := Decode(code)
	if err != nil {
		return nil, err
	}
	var b [18]byte
	binary.LittleEndian.PutUint64(b[0:8], m.MatchID)
	binary.LittleEndian.PutUint64(b[8:16], m.OutcomeID)
	binary.LittleEndian.PutUint16(b[16:18], m.TokenID)
	var checksum byte
	for _, value := range b[1:] {
		checksum += value
	}
	if checksum != b[0] {
		return nil, fmt.Errorf("crosshair checksum mismatch")
	}
	if b[1] != 1 || b[15] != 0 || b[16] != 0 || b[17] != 0 {
		return nil, fmt.Errorf("unsupported crosshair payload version or reserved fields")
	}
	bit := func(value byte, mask byte) float64 {
		if value&mask != 0 {
			return 1
		}
		return 0
	}
	return map[string]float64{
		"cl_crosshairgap":                          float64(int8(b[2])) / 10,
		"cl_crosshair_outlinethickness":            float64(b[3]) / 2,
		"cl_crosshaircolor_r":                      float64(b[4]),
		"cl_crosshaircolor_g":                      float64(b[5]),
		"cl_crosshaircolor_b":                      float64(b[6]),
		"cl_crosshairalpha":                        float64(b[7]),
		"cl_crosshair_dynamic_splitdist":           float64(b[8] & 7),
		"cl_crosshair_recoil":                      bit(b[8], 128),
		"cl_fixedcrosshairgap":                     float64(int8(b[9])) / 10,
		"cl_crosshaircolor":                        float64(b[10] & 7),
		"cl_crosshair_drawoutline":                 bit(b[10], 8),
		"cl_crosshair_dynamic_splitalpha_innermod": float64(b[10]>>4) / 10,
		"cl_crosshair_dynamic_splitalpha_outermod": float64(b[11]&15) / 10,
		"cl_crosshair_dynamic_maxdist_splitratio":  float64(b[11]>>4) / 10,
		"cl_crosshairthickness":                    float64(b[12]) / 10,
		"cl_crosshairdot":                          bit(b[13], 16),
		"cl_crosshairgap_useweaponvalue":           bit(b[13], 32),
		"cl_crosshairusealpha":                     bit(b[13], 64),
		"cl_crosshair_t":                           bit(b[13], 128),
		"cl_crosshairstyle":                        float64((b[13] & 15) >> 1),
		"cl_crosshairsize":                         float64(b[14]) / 10,
	}, nil
}
