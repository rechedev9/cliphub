package tactical

import (
	"math"
	"sort"
	"strings"

	"github.com/rechedev9/cliphub/internal/radarmap"
	"github.com/rechedev9/cliphub/internal/tacticalplan"
)

// siteRadius is how close to a site centre a player counts as being on it. A
// CS2 bombsite is roughly 700 units across, so 500 from the centre covers the
// plantable area and its immediate entries without reaching the next callout.
const siteRadius = 500.0

// siteMap holds where the map's objectives are, derived from the demo itself:
// bomb plants first, because a plant is proof, and the map's own place names
// second. Nothing is hard-coded per map, so a workshop map works the same way.
//
// On a split-level map a site only holds positions on its own radar level:
// Nuke's A sits above B, closer in x/y than siteRadius.
type siteMap struct {
	centers map[tacticalplan.Site][2]float64
	levels  map[tacticalplan.Site]string
	cal     radarmap.Calibration
	radius  float64
}

func buildSiteMap(rounds []*roundAcc, geo tacticalplan.MapGeometry) siteMap {
	m := siteMap{
		centers: map[tacticalplan.Site][2]float64{},
		levels:  map[tacticalplan.Site]string{},
		cal:     geo.Calibration,
		radius:  siteRadius,
	}

	sums := map[tacticalplan.Site][3]float64{}
	counts := map[tacticalplan.Site]int{}
	for _, acc := range rounds {
		for _, ev := range acc.events {
			if ev.Kind != tacticalplan.EventPlant || ev.Site == tacticalplan.SiteNone {
				continue
			}
			sum := sums[ev.Site]
			sums[ev.Site] = [3]float64{sum[0] + ev.Pos[0], sum[1] + ev.Pos[1], sum[2] + ev.Pos[2]}
			counts[ev.Site]++
		}
	}
	for site, n := range counts {
		if n == 0 {
			continue
		}
		m.centers[site] = [2]float64{sums[site][0] / float64(n), sums[site][1] / float64(n)}
		m.levels[site] = m.cal.Level(sums[site][2] / float64(n))
	}

	// Place names fill in what no plant proved: a site nobody planted on this
	// map, and the middle, which is never a plant location.
	centers, levels := calloutCenters(geo)
	for site, center := range centers {
		if _, ok := m.centers[site]; !ok {
			m.centers[site] = center
			m.levels[site] = levels[site]
		}
	}
	return m
}

// calloutCenters averages the callouts whose names (like "BombsiteA") identify
// a site, and gives each site the level of its busiest callout.
func calloutCenters(geo tacticalplan.MapGeometry) (map[tacticalplan.Site][2]float64, map[tacticalplan.Site]string) {
	sums := map[tacticalplan.Site][2]float64{}
	weights := map[tacticalplan.Site]int{}
	levels := map[tacticalplan.Site]string{}
	busiest := map[tacticalplan.Site]int{}
	callouts := append([]tacticalplan.Callout(nil), geo.Callouts...)
	sort.Slice(callouts, func(i, j int) bool { return callouts[i].Name < callouts[j].Name })
	for _, c := range callouts {
		site := siteFromPlace(c.Name)
		if site == tacticalplan.SiteNone {
			continue
		}
		sum := sums[site]
		sums[site] = [2]float64{
			sum[0] + c.Center[0]*float64(c.Samples),
			sum[1] + c.Center[1]*float64(c.Samples),
		}
		weights[site] += c.Samples
		if c.Samples > busiest[site] {
			busiest[site] = c.Samples
			levels[site] = c.Level
		}
	}
	out := map[tacticalplan.Site][2]float64{}
	for site, w := range weights {
		if w == 0 {
			continue
		}
		out[site] = [2]float64{sums[site][0] / float64(w), sums[site][1] / float64(w)}
	}
	return out, levels
}

// siteFromPlace classifies a CS2 place name. Only names that unambiguously
// identify a site count; "Apartments" is near A on Mirage but is not A.
func siteFromPlace(place string) tacticalplan.Site {
	name := strings.ToLower(strings.TrimSpace(place))
	if name == "" {
		return tacticalplan.SiteNone
	}
	switch {
	case strings.Contains(name, "bombsite a"), strings.Contains(name, "bombsitea"),
		name == "site a", name == "a site":
		return tacticalplan.SiteA
	case strings.Contains(name, "bombsite b"), strings.Contains(name, "bombsiteb"),
		name == "site b", name == "b site":
		return tacticalplan.SiteB
	case name == "middle", name == "mid", strings.Contains(name, "midsection"):
		return tacticalplan.SiteMid
	default:
		return tacticalplan.SiteNone
	}
}

// nearest returns the closest site on the same level as a world position and
// its distance in x/y.
func (m siteMap) nearest(x, y, z float64) (tacticalplan.Site, float64) {
	best := tacticalplan.SiteNone
	bestDist := math.Inf(1)
	for _, site := range []tacticalplan.Site{tacticalplan.SiteA, tacticalplan.SiteB, tacticalplan.SiteMid} {
		center, ok := m.centers[site]
		if !ok || !m.sameLevel(site, z) {
			continue
		}
		if d := math.Hypot(x-center[0], y-center[1]); d < bestDist {
			best, bestDist = site, d
		}
	}
	return best, bestDist
}

// bombsites returns the sites that can be taken, in a stable order.
func (m siteMap) bombsites() []tacticalplan.Site {
	var out []tacticalplan.Site
	for _, site := range []tacticalplan.Site{tacticalplan.SiteA, tacticalplan.SiteB} {
		if _, ok := m.centers[site]; ok {
			out = append(out, site)
		}
	}
	return out
}

// sameLevel reports whether altitude z is on the radar level of site. A site
// with no known level, or a map with one level, accepts every altitude.
func (m siteMap) sameLevel(site tacticalplan.Site, z float64) bool {
	level, ok := m.levels[site]
	return !ok || m.cal.Level(z) == level
}

func (m siteMap) center(site tacticalplan.Site) ([2]float64, bool) {
	c, ok := m.centers[site]
	return c, ok
}
