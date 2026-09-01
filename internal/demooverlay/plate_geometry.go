package demooverlay

const outroPlateMaxRows = 10

const (
	introPlateTeamNameSize = 30
	introPlatePOVBadgeW    = 36
	introPlatePOVBadgeH    = 18
	// introPlatePOVRightPad keeps the POV badge inside the panel inner border.
	introPlatePOVRightPad = 68 // badge width + 12px pad + ~20px inner inset
)

// IntroPlateGeometry holds measured row slots in 1920x1080 frame space after
// scale-to-cover (≈1.891) and center crop from the 1024x571 source plates.
type IntroPlateGeometry struct {
	TeamNameY      int
	TeamNameXOff   int
	TeamNameSize   int
	SubtitleY      int
	RowNameCenterY [5]int
}

// OutroPlateGeometry holds measured scoreboard row bands in frame space.
type OutroPlateGeometry struct {
	HeaderY        int
	ColLabelY      int
	RowNameCenterY [outroPlateMaxRows]int
}

// introPlateGeometryTable is measured from data/overlay-assets/plates/*-intro.jpg.
var introPlateGeometryTable = map[string]IntroPlateGeometry{
	SourceProfessional: {
		TeamNameY:      126,
		TeamNameXOff:   130,
		TeamNameSize:   introPlateTeamNameSize,
		SubtitleY:      160,
		RowNameCenterY: [5]int{293, 454, 613, 772, 932},
	},
	SourcePremier: {
		TeamNameY:      168,
		TeamNameXOff:   140,
		TeamNameSize:   28,
		SubtitleY:      198,
		RowNameCenterY: [5]int{293, 452, 603, 753, 908},
	},
	SourceFACEIT: {
		TeamNameY:      78,
		TeamNameXOff:   120,
		TeamNameSize:   28,
		SubtitleY:      104,
		// Row 0 from avatar ring detection; rows 1-4 from divider bands.
		RowNameCenterY: [5]int{276, 437, 598, 764, 929},
	},
}

// outroPlateGeometryTable is measured from data/overlay-assets/plates/*-outro.jpg.
var outroPlateGeometryTable = map[string]OutroPlateGeometry{
	SourceProfessional: {
		HeaderY:   118,
		ColLabelY: 248,
		RowNameCenterY: [outroPlateMaxRows]int{
			329, 410, 490, 571, 653, 732, 811, 893, 972, 1052,
		},
	},
	SourcePremier: {
		HeaderY:   118,
		ColLabelY: 248,
		RowNameCenterY: [outroPlateMaxRows]int{
			329, 410, 490, 571, 651, 732, 811, 893, 972, 1052,
		},
	},
	SourceFACEIT: {
		HeaderY:   128,
		ColLabelY: 278,
		RowNameCenterY: [outroPlateMaxRows]int{
			316, 397, 477, 558, 637, 721, 804, 887, 976, 1054,
		},
	},
}

const (
	outroPlateNameYOffset = 22
	outroPlateStatYOffset = 8
)

func introPlatePOVBadgeX(panelX, panelWidth int) int {
	return panelX + panelWidth - introPlatePOVRightPad
}

func (g OutroPlateGeometry) rowCount() int {
	for i := len(g.RowNameCenterY) - 1; i >= 0; i-- {
		if g.RowNameCenterY[i] > 0 {
			return i + 1
		}
	}
	return 0
}

func (g OutroPlateGeometry) rowNameY(i int) int {
	if i < 0 || i >= len(g.RowNameCenterY) || g.RowNameCenterY[i] == 0 {
		return 0
	}
	return g.RowNameCenterY[i] - outroPlateNameYOffset
}

func (g OutroPlateGeometry) rowStatY(i int) int {
	if i < 0 || i >= len(g.RowNameCenterY) || g.RowNameCenterY[i] == 0 {
		return 0
	}
	return g.RowNameCenterY[i] + outroPlateStatYOffset
}

func IntroPlateGeo(source string, hasPlate bool) (IntroPlateGeometry, bool) {
	if !hasPlate {
		return IntroPlateGeometry{}, false
	}
	geo, ok := introPlateGeometryTable[NormalizeSource(source)]
	return geo, ok
}

func OutroPlateGeo(source string, hasPlate bool) (OutroPlateGeometry, bool) {
	if !hasPlate {
		return OutroPlateGeometry{}, false
	}
	geo, ok := outroPlateGeometryTable[NormalizeSource(source)]
	return geo, ok
}

func IntroPlateLayout(source string, hasPlate bool) (IntroLayout, bool) {
	geo, ok := IntroPlateGeo(source, hasPlate)
	if !ok {
		return IntroLayout{}, false
	}
	layout := DefaultLayout().Intro
	if len(geo.RowNameCenterY) >= 2 {
		layout.RowHeight = geo.RowNameCenterY[1] - geo.RowNameCenterY[0]
	}
	layout.HeaderH = geo.RowNameCenterY[0] - DefaultLayout().Intro.PanelTop
	return layout, true
}

func OutroLayoutForSourceWithPlate(source string, hasPlate bool) (OutroLayout, OutroPlateGeometry, bool) {
	base := DefaultLayout().Outro
	geo, ok := OutroPlateGeo(source, hasPlate)
	if !ok {
		return base, OutroPlateGeometry{}, false
	}
	base.HeaderY = geo.HeaderY
	base.Row0 = geo.rowNameY(0)
	if geo.rowCount() >= 2 {
		base.RowHeight = geo.RowNameCenterY[1] - geo.RowNameCenterY[0]
	}
	return base, geo, true
}
