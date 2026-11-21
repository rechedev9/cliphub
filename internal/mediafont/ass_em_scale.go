package mediafont

func ASSEmScale(data []byte) float64 {
	u16 := func(at int) int {
		if at < 0 || at+2 > len(data) {
			return 0
		}
		return int(data[at])<<8 | int(data[at+1])
	}
	tables := map[string]int{}
	for i := 0; i < u16(4); i++ {
		rec := 12 + 16*i
		if rec+16 > len(data) {
			break
		}
		tables[string(data[rec:rec+4])] = u16(rec+8)<<16 | u16(rec+10)
	}
	head, okHead := tables["head"]
	os2, okOS2 := tables["OS/2"]
	if !okHead || !okOS2 {
		return 1
	}
	upm, win := u16(head+18), u16(os2+74)+u16(os2+76)
	if upm == 0 || win == 0 {
		return 1
	}
	return float64(upm) / float64(win)
}
