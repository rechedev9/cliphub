package demooverlay

// Layout is the 1920x1080 overlay geometry. Tests lock these so the intro
// keeps a native-HUD center channel and the outro stays full-frame.
type Layout struct {
	Width  int
	Height int
	Intro  IntroLayout
	Outro  OutroLayout
}

type IntroLayout struct {
	PanelWidth  int
	PanelHeight int
	PanelTop    int
	LeftPanelX  int
	RightPanelX int
	CenterGap   int
	RowHeight   int
	MaxPlayers  int
}

type OutroLayout struct {
	Margin    int
	HeaderH   int
	RowHeight int
	FullFrame bool
}

func DefaultLayout() Layout {
	const (
		panelW = 520
		inset  = 24
		top    = 72
		height = 936
	)
	return Layout{
		Width:  FrameWidth,
		Height: FrameHeight,
		Intro: IntroLayout{
			PanelWidth:  panelW,
			PanelHeight: height,
			PanelTop:    top,
			LeftPanelX:  inset,
			RightPanelX: FrameWidth - inset - panelW,
			CenterGap:   FrameWidth - 2*(inset+panelW),
			RowHeight:   180,
			MaxPlayers:  5,
		},
		Outro: OutroLayout{
			Margin:    32,
			HeaderH:   88,
			RowHeight: 72,
			FullFrame: true,
		},
	}
}

func (l Layout) NativeHUDVisible() bool {
	return l.Intro.CenterGap >= 800
}
