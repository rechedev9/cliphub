package tactical

import (
	"math"
	"sort"
	"strings"

	"github.com/rechedev9/cliphub/internal/tacticalplan"
)

// siteRadius is how close to a site centre a player counts as being on it. A
// CS2 bombsite is roughly 700 units across, so 500 from the centre covers the
// plantable area and its immediate entries without reaching the next callout.
const siteRadius = 500.0

// siteMap holds where the map's objectives are, derived from the demo itself:
// bomb plants first, because a plant is proof, and the map's own place names
// second. Nothing is hard-coded per map, so a workshop map works the same way.
type siteMap struct {
	centers map[tacticalplan.Site][2]float64
	radius  float64
}

func buildSiteMap(rounds []*roundAcc, geo tacticalplan.MapGeometry) siteMap {
	m := siteMap{centers: map[tacticalplan.Site][2]float64{}, radius: siteRadius}

	sums := map[tacticalplan.Site][2]float64{}
	counts := map[tacticalplan.Site]int{}
	for _, acc := range rounds {
		for _, ev := range acc.events {
			if ev.Kind != tacticalplan.EventPlant || ev.Site == tacticalplan.SiteNone {
				continue
			}
			sum := sums[ev.Site]
			sums[ev.Site] = [2]float64{sum[0] + ev.Pos[0], sum[1] + ev.Pos[1]}
			counts[ev.Site]++
		}
	}
	for site, n := range counts {
		if n == 0 {
			continue
		}
		m.centers[site] = [2]float64{sums[site][0] / float64(n), sums[site][1] / float64(n)}
	}

	// Place names fill in what no plant proved: a site nobody planted on this
	// map, and the middle, which is never a plant location.
	fromPlaces := calloutCenters(geo)
	for site, center := range fromPlaces {
		if _, ok := m.centers[site]; !ok {
			m.centers[site] = center
		}
	}
	return m
}

// calloutCenters averages the callouts whose names identify a site. CS2 place
// names are stable strings like "BombsiteA", "TSpawn", "Middle".
func calloutCenters(geo tacticalplan.MapGeometry) map[tacticalplan.Site][2]float64 {
	sums := map[tacticalplan.Site][2]float64{}
	weights := map[tacticalplan.Site]int{}
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
	}
	out := map[tacticalplan.Site][2]float64{}
	for site, w := range weights {
		if w == 0 {
			continue
		}
		out[site] = [2]float64{sums[site][0] / float64(w), sums[site][1] / float64(w)}
	}
	return out
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

// nearest returns the closest site to a world position and its distance.
func (m siteMap) nearest(x, y float64) (tacticalplan.Site, float64) {
	best := tacticalplan.SiteNone
	bestDist := math.Inf(1)
	for _, site := range []tacticalplan.Site{tacticalplan.SiteA, tacticalplan.SiteB, tacticalplan.SiteMid} {
		center, ok := m.centers[site]
		if !ok {
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

func (m siteMap) center(site tacticalplan.Site) ([2]float64, bool) {
	c, ok := m.centers[site]
	return c, ok
}
