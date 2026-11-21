package subtitles

import "slices"

const povColour = "FFFFFF"

var palette = [5]string{"4FD7FF", "FFC85C", "87E76E", "4A9FFF", "D08FFF"}

func Slot(team []string, steamID string) int {
	sorted := slices.Sorted(slices.Values(team))
	at, found := slices.BinarySearch(sorted, steamID)
	if !found {
		return 0
	}
	return at % len(palette)
}

func colourOf(c Cue) string {
	if c.POV {
		return povColour
	}
	return palette[((c.Slot%len(palette))+len(palette))%len(palette)]
}
