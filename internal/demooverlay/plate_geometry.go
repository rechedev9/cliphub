package demooverlay

const (
	outroPlateMaxRows        = 5
	outroMaxPlayersPerTeam   = 5
	outroPlateNameYOffset    = 22
	outroPlateStatYOffset    = 10
	outroPlatePOVBadgeW      = 36
	outroPlatePOVBadgeH      = 16
	outroPlatePOVStatGap     = 10
)

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
	HeaderY  int
	ColLabelY int
	// PlateCropTop and PlateCropBottom bound the scoreboard artwork after
	// scale-to-cover: from the outer frame through the bottom of row 5, excluding
	// empty lower slots. The cropped region is vertically centered in FrameHeight.
	PlateCropTop    int
	PlateCropBottom int
	RowNameCenterY  [outroPlateMaxRows]int
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
		HeaderY:         118,
		ColLabelY:       248,
		PlateCropTop:    34,
		PlateCropBottom: 699,
		RowNameCenterY:  [outroPlateMaxRows]int{329, 410, 490, 571, 653},
	},
	SourcePremier: {
		HeaderY:         118,
		ColLabelY:       248,
		PlateCropTop:    44,
		PlateCropBottom: 697,
		RowNameCenterY:  [outroPlateMaxRows]int{329, 410, 490, 571, 651},
	},
	SourceFACEIT: {
		HeaderY:         108,
		ColLabelY:       252,
		PlateCropTop:    59,
		PlateCropBottom: 680,
		RowNameCenterY:  [outroPlateMaxRows]int{316, 397, 477, 558, 637},
	},
}

func introPlatePOVBadgeX(panelX, panelWidth int) int {
	return panelX + panelWidth - introPlatePOVRightPad
}

func (g OutroPlateGeometry) rowCount() int {
	n := 0
	for i := range g.RowNameCenterY {
		if g.RowNameCenterY[i] > 0 {
			n = i + 1
		}
	}
	return n
}

func (g OutroPlateGeometry) contentHeight() int {
	if g.PlateCropBottom <= g.PlateCropTop {
		return 0
	}
	return g.PlateCropBottom - g.PlateCropTop
}

func (g OutroPlateGeometry) verticalPadTop() int {
	h := g.contentHeight()
	if h <= 0 || h >= FrameHeight {
		return 0
	}
	return (FrameHeight - h) / 2
}

func (g OutroPlateGeometry) mapFrameY(y int) int {
	h := g.contentHeight()
	if h <= 0 || h >= FrameHeight {
		return y
	}
	return y - g.PlateCropTop + g.verticalPadTop()
}

func (g OutroPlateGeometry) mappedHeaderY() int {
	return g.mapFrameY(g.HeaderY)
}

func (g OutroPlateGeometry) mappedColLabelY() int {
	return g.mapFrameY(g.ColLabelY)
}

func (g OutroPlateGeometry) rowNameY(i int) int {
	if i < 0 || i >= len(g.RowNameCenterY) || g.RowNameCenterY[i] == 0 {
		return 0
	}
	return g.mapFrameY(g.RowNameCenterY[i] - outroPlateNameYOffset)
}

func (g OutroPlateGeometry) rowStatY(i int) int {
	if i < 0 || i >= len(g.RowNameCenterY) || g.RowNameCenterY[i] == 0 {
		return 0
	}
	return g.mapFrameY(g.RowNameCenterY[i] + outroPlateStatYOffset)
}

func outroPlatePOVBadgeX(nameColX, nameWidth int) int {
	return nameColX + nameWidth - outroPlatePOVBadgeW - outroPlatePOVStatGap
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
	base.HeaderY = geo.mappedHeaderY()
	base.Row0 = geo.rowNameY(0)
	if geo.rowCount() >= 2 {
		base.RowHeight = geo.rowNameY(1) - geo.rowNameY(0)
	}
	return base, geo, true
}
