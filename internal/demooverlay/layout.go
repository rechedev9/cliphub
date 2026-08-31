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
	HeaderH     int
	CardInset   int
	AvatarSize  int
	NameSize    int
	StatSize    int
	LabelSize   int
	BadgeSize   int
}

type OutroLayout struct {
	Margin    int
	HeaderH   int
	RowHeight int
	FullFrame bool
	ColGap    int
}

func DefaultLayout() Layout {
	const (
		panelW = 563
		inset  = 42
		top    = 28
		height = 1020
		rightX = 1308
	)
	return Layout{
		Width:  FrameWidth,
		Height: FrameHeight,
		Intro: IntroLayout{
			PanelWidth:  panelW,
			PanelHeight: height,
			PanelTop:    top,
			LeftPanelX:  inset,
			RightPanelX: rightX,
			CenterGap:   rightX - inset - panelW,
			RowHeight:   204,
			MaxPlayers:  5,
			HeaderH:     0,
			CardInset:   100,
			AvatarSize:  72,
			NameSize:    22,
			StatSize:    13,
			LabelSize:   9,
			BadgeSize:   28,
		},
		Outro: OutroLayout{
			Margin:    270,
			HeaderH:   60,
			RowHeight: 145,
			FullFrame: true,
			ColGap:    760,
		},
	}
}

func (l Layout) NativeHUDVisible() bool {
	return l.Intro.CenterGap >= 700
}
