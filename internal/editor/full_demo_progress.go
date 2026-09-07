package editor

// fullDemoProgress reports completed media work within one Full Demo render.
// Fixed phase weights are estimates, while each pass advances from FFmpeg's
// media timestamps or successful completion, never elapsed wall-clock time.
type fullDemoProgress func(stage string, fraction float64)

func (p fullDemoProgress) report(stage string, fraction float64) {
	if p != nil {
		p(stage, min(1, max(0, fraction)))
	}
}

func (p fullDemoProgress) pass(stage string, start, end float64) func(float64) {
	if p == nil {
		return nil
	}
	p.report(stage, start)
	return func(fraction float64) {
		p.report(stage, start+(end-start)*min(1, max(0, fraction)))
	}
}

func (p fullDemoProgress) within(start, end float64) fullDemoProgress {
	if p == nil {
		return nil
	}
	return func(stage string, fraction float64) {
		p.report(stage, start+(end-start)*fraction)
	}
}
