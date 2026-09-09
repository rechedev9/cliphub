package customhud

import (
	"embed"
	"encoding/xml"
	"fmt"
	"path"
	"strconv"
	"strings"
	"sync"
)

// Artwork is vendored with its license and source revision in assets/.
// All paths use absolute M/L/C coordinates, the subset also supported by ASS.
//
//go:embed assets/weapons/*.svg assets/status/*.svg
var iconFiles embed.FS

type iconToken struct {
	command string
	value   float64
}

type vectorIcon struct {
	x, y, w, h float64
	tokens     []iconToken
}

var iconsOnce sync.Once
var icons map[string]vectorIcon
var iconsError error
var iconPaths sync.Map

func loadIcons() error {
	iconsOnce.Do(func() {
		icons = map[string]vectorIcon{}
		for _, group := range []string{"weapons", "status"} {
			files, err := iconFiles.ReadDir("assets/" + group)
			if err != nil {
				iconsError = err
				return
			}
			for _, file := range files {
				body, err := iconFiles.ReadFile("assets/" + group + "/" + file.Name())
				if err != nil {
					iconsError = err
					return
				}
				icon, err := parseIcon(body)
				if err != nil {
					iconsError = fmt.Errorf("bundled HUD icon %s: %w", file.Name(), err)
					return
				}
				icons[group+"/"+strings.TrimSuffix(file.Name(), path.Ext(file.Name()))] = icon
			}
		}
	})
	return iconsError
}

func parseIcon(body []byte) (vectorIcon, error) {
	var doc struct {
		ViewBox string `xml:"viewBox,attr"`
		Paths   []struct {
			D string `xml:"d,attr"`
		} `xml:"path"`
	}
	if err := xml.Unmarshal(body, &doc); err != nil {
		return vectorIcon{}, err
	}
	var icon vectorIcon
	if n, err := fmt.Sscan(doc.ViewBox, &icon.x, &icon.y, &icon.w, &icon.h); err != nil || n != 4 || icon.w <= 0 || icon.h <= 0 || len(doc.Paths) == 0 {
		return icon, fmt.Errorf("invalid SVG view box or paths")
	}
	for _, p := range doc.Paths {
		// Bundled sources separate commands and coordinates with whitespace.
		for _, token := range strings.Fields(strings.ReplaceAll(p.D, ",", " ")) {
			switch token {
			case "M", "L", "C", "Z":
				icon.tokens = append(icon.tokens, iconToken{command: token})
			default:
				n, err := strconv.ParseFloat(token, 64)
				if err != nil {
					return icon, fmt.Errorf("unsupported SVG token %q", token)
				}
				icon.tokens = append(icon.tokens, iconToken{value: n})
			}
		}
	}
	return icon, nil
}

func iconPath(key string, x, y, w, h int, flip bool) string {
	icon, ok := icons[key]
	if !ok {
		return ""
	}
	cacheKey := struct {
		Key        string
		X, Y, W, H int
		Flip       bool
	}{key, x, y, w, h, flip}
	if cached, ok := iconPaths.Load(cacheKey); ok {
		return cached.(string)
	}
	scale := min(float64(w)/icon.w, float64(h)/icon.h)
	dx := float64(x) + (float64(w)-icon.w*scale)/2
	dy := float64(y) + (float64(h)-icon.h*scale)/2
	var out strings.Builder
	axis := 0
	for _, token := range icon.tokens {
		if token.command != "" {
			out.WriteString(token.command + " ")
			axis = 0
			continue
		}
		v := (token.value-icon.y)*scale + dy
		if axis%2 == 0 {
			v = (token.value-icon.x)*scale + dx
			if flip {
				v = 2*float64(x) + float64(w) - v
			}
		}
		fmt.Fprintf(&out, "%.2f ", v)
		axis++
	}
	result := out.String()
	iconPaths.Store(cacheKey, result)
	return result
}

func weaponIcon(weapon string) string {
	key := strings.ToLower(strings.TrimSpace(weapon))
	if strings.HasPrefix(key, "weapon_") || strings.Contains(key, "_") {
		internal := strings.TrimPrefix(key, "weapon_")
		if _, ok := icons["weapons/"+internal]; ok {
			return "weapons/" + internal
		}
	}
	key = strings.TrimPrefix(key, "weapon_")
	key = strings.NewReplacer("-", "", " ", "", "_", "").Replace(key)
	aliases := map[string]string{
		"m4a1": "m4a1_silencer", "m4a1s": "m4a1_silencer", "m4a4": "m4a1", "usps": "usp_silencer", "usp": "usp_silencer",
		"glock18": "glock", "decoygrenade": "decoy",
		"p2000": "hkp2000", "g3sg1": "g3sg1", "sg553": "sg556", "galilar": "galilar", "galil": "galilar",
		"dualberettas": "elite", "berettas": "elite", "fiveseven": "fiveseven", "deserteagle": "deagle",
		"cz75auto": "cz75a", "cz75": "cz75a", "r8revolver": "revolver", "ppbizon": "bizon",
		"mp5sd": "mp5sd", "hegrenade": "hegrenade", "smokegrenade": "smokegrenade", "smoke": "smokegrenade",
		"incendiarygrenade": "incgrenade", "incendiary": "incgrenade", "zeusx27": "taser", "zeus": "taser",
	}
	if alias, ok := aliases[key]; ok {
		key = alias
	}
	if key == "bayonet" {
		key = "knife"
	}
	if _, ok := icons["weapons/"+key]; ok {
		return "weapons/" + key
	}
	return ""
}

func (s *scene) icon(id, key string, x, y, w, h int, color string, flip bool) bool {
	p := iconPath(key, x, y, w, h, flip)
	if p == "" {
		return false
	}
	s.path(id, p, color, 8)
	return true
}
