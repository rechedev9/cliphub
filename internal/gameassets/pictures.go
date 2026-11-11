package gameassets

import (
	"errors"
	"strings"
	"unicode"

	"github.com/rechedev9/cliphub/internal/mapgeo"
)

// The loading-screen picture of each map and the team logos of the Major stickers, both read
// from the user's own install like the radars: nothing is bundled or downloaded.
const (
	shotDir = "panorama/images/map_icons/screenshots/720p/"
	logoDir = "panorama/images/tournaments/teams/"
)

var (
	// ErrShotNotFound reports that the install has no picture for the map.
	ErrShotNotFound = errors.New("map picture not found in the cs2 install")
	// ErrLogoNotFound reports that no logo in the install belongs to the team.
	ErrLogoNotFound = errors.New("team logo not found in the cs2 install")
)

// teamLogoCodes maps a team name, folded by teamKey, to its sticker code in the install.
var teamLogoCodes = map[string]string{
	"3dmax": "3dm", "amkal": "amka", "apeks": "apex", "astralis": "astr", "aurora": "aura",
	"b8": "b8", "betboom": "bb", "betboomteam": "bb", "big": "big", "chinggiswarriors": "chin",
	"cloud9": "c9", "complexity": "cplx", "ecstatic": "ecst", "ence": "ence", "eternalfire": "eter",
	"evilgeniuses": "evl", "falcons": "fal", "teamfalcons": "fal", "faze": "faze", "fazeclan": "faze",
	"flyquest": "fq", "furia": "furi", "furiaesports": "furi", "fut": "fut", "futesports": "fut",
	"g2": "g2", "g2esports": "g2", "gaimingladiators": "gaim", "gamerlegion": "gl", "grayhound": "gray",
	"heroic": "hero", "imperial": "imp", "imperialesports": "imp", "koi": "koi", "legacy": "lgcy",
	"liquid": "liq", "teamliquid": "liq", "lynnvision": "lynn", "lynnvisiongaming": "lynn", "m80": "m80",
	"metizport": "meti", "mibr": "mibr", "monte": "mont", "mouz": "mouz", "mousesports": "mouz",
	"natusvincere": "navi", "navi": "navi", "nemiga": "nemi", "ninjasinpyjamas": "nip", "nip": "nip",
	"nrg": "nrg", "og": "og", "outsiders": "out", "pain": "pain", "paingaming": "pain",
	"parivision": "pari", "passionua": "psnu", "redcanids": "redc", "saw": "saw", "sharks": "shrk",
	"sinners": "sinn", "spirit": "spir", "teamspirit": "spir", "themongolz": "mngz", "mongolz": "mngz",
	"thehuns": "huns", "tsm": "tsm", "tyloo": "tyl", "vitality": "vita", "teamvitality": "vita",
	"virtuspro": "vp", "vp": "vp", "wildcard": "wcrd", "ldlc": "ldlc",
}

// teamKey folds a team name to lowercase letters and digits, so "Natus Vincere" and
// "natus-vincere" find the same logo.
func teamKey(name string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(name) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// TeamLogoCode is the sticker code of a team's logo, or "" when the install has none for it.
func TeamLogoCode(name string) string {
	return teamLogoCodes[teamKey(name)]
}

// MapShot decodes the map's loading-screen picture and encodes it as PNG.
func (i Install) MapShot(mapName string) ([]byte, error) {
	pack, err := i.open()
	if err != nil {
		return nil, err
	}
	defer pack.Close()
	data, err := pack.ReadFile(shotDir + mapName + "_png.vtex_c")
	if errors.Is(err, mapgeo.ErrNotInVPK) {
		return nil, ErrShotNotFound
	}
	if err != nil {
		return nil, err
	}
	body, _, err := encodeTexture(data)
	return body, err
}

// TeamLogo returns the team's logo as sanitized SVG.
func (i Install) TeamLogo(name string) ([]byte, error) {
	code := TeamLogoCode(name)
	if code == "" {
		return nil, ErrLogoNotFound
	}
	pack, err := i.open()
	if err != nil {
		return nil, err
	}
	defer pack.Close()
	data, err := pack.ReadFile(logoDir + code + ".vsvg_c")
	if errors.Is(err, mapgeo.ErrNotInVPK) {
		return nil, ErrLogoNotFound
	}
	if err != nil {
		return nil, err
	}
	svg, err := ExtractSVG(data)
	if err != nil {
		return nil, err
	}
	return SanitizeSVG(svg)
}
