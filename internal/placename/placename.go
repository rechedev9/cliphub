// Package placename turns CS2 place tokens (m_szLastPlaceName, such as
// "BombsiteA" or "TopofMid") into the callouts players say. It mirrors
// placeLabel in web/lib/tactical-labels.ts, so the Studio UI and the generated
// video text name a place the same way.
package placename

import "regexp"

// known holds the tokens whose English name in csgo_english.txt is not their
// CamelCase split.
var known = map[string]string{
	"TopofMid":      "Top of Mid",
	"BackofA":       "Back of A",
	"BackofB":       "Back of B",
	"SnipersNest":   "Sniper's Nest",
	"VipRescueZone": "VIP Rescue Zone",
	"Control":       "Control Room",
	"Floor50":       "50th Floor",
	"Floor51":       "51st Floor",
	"MotorBoats":    "Motorboats",
	"BoatHouse":     "Boathouse",
	"DriveWay":      "Driveway",
	"PopDog":        "Popdog",
	"LongDog":       "Longdog",
	"Gravelpit":     "Gravel Pit",
}

var (
	lowerThenUpper = regexp.MustCompile(`([a-z0-9])([A-Z])`)
	acronymEnd     = regexp.MustCompile(`([A-Z])([A-Z][a-z])`)
	letterDigit    = regexp.MustCompile(`([A-Za-z])([0-9])`)
)

// Label names a CS2 place token as players say it: "BombsiteA" reads
// "Bombsite A" and "CTSpawn" reads "CT Spawn". An unknown token is split at
// its case changes, so a workshop map still reads as words.
func Label(token string) string {
	if name, ok := known[token]; ok {
		return name
	}
	out := lowerThenUpper.ReplaceAllString(token, "$1 $2")
	out = acronymEnd.ReplaceAllString(out, "$1 $2")
	return letterDigit.ReplaceAllString(out, "$1 $2")
}
