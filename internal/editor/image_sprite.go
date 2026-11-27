package editor

import (
	"fmt"
	"math"
	"strings"
)

type ImageSprite struct {
	CellHeight int       `json:"cell_height"`
	Frames     int       `json:"frames"`
	FPS        int       `json:"fps"`
	PlayAt     []float64 `json:"play_at"`
}

func imageSpriteFilters(effect Effect) []string {
	sprite := effect.Sprite
	if sprite == nil || sprite.CellHeight <= 0 || sprite.Frames <= 0 || sprite.FPS <= 0 {
		return nil
	}
	cell := "0"
	for i := len(sprite.PlayAt) - 1; i >= 0; i-- {
		start := effect.StartSeconds + sprite.PlayAt[i]
		end := start + float64(sprite.Frames)/float64(sprite.FPS)
		frame := fmt.Sprintf("min(%d\\,1+trunc((t-%.3f)*%d))", sprite.Frames, start, sprite.FPS)
		cell = fmt.Sprintf("if(between(t\\,%.3f\\,%.3f)\\,%s\\,%s)", start, end, frame, cell)
	}
	return []string{fmt.Sprintf("crop=w=iw:h=%d:x=0:y='%d*%s'", sprite.CellHeight, sprite.CellHeight, cell)}
}

func spriteRingSeconds(effects []Effect, source string) []float64 {
	var rings []float64
	for _, effect := range effects {
		if effect.Source == source && effect.Sprite != nil && len(effect.Sprite.PlayAt) > 0 {
			rings = append(rings, effect.StartSeconds+effect.Sprite.PlayAt[0])
		}
	}
	return rings
}

type bellDingMix struct {
	in, out       string
	rings         []float64
	offsetSeconds float64
	samples       int64
}

const (
	bellDingSeconds    = 1.2
	bellDingGain       = 0.2
	bellDingExpression = "(1-exp(-t*250))*(0.8*sin(2*PI*523*t)*exp(-4*t)+0.2*sin(2*PI*2093*t)*exp(-14*t))"
)

func (m bellDingMix) clauses() []string {
	var clauses, labels []string
	for i, at := range m.rings {
		delay := int64(math.Round((at - m.offsetSeconds) * 48000))
		if delay < 0 || (m.samples > 0 && delay >= m.samples) {
			continue
		}
		name := fmt.Sprintf("%sding%d", m.out, i)
		clauses = append(clauses, fmt.Sprintf("aevalsrc=exprs='%s':s=48000:c=stereo:d=%.2f,volume=%.2f,adelay=%dS:all=1[%s]", bellDingExpression, bellDingSeconds, bellDingGain, delay, name))
		labels = append(labels, "["+name+"]")
	}
	if len(labels) == 0 {
		return nil
	}
	tail := ""
	if m.samples > 0 {
		tail = fmt.Sprintf(",apad=whole_len=%d,atrim=end_sample=%d,asetpts=N/SR/TB", m.samples, m.samples)
	}
	return append(clauses, fmt.Sprintf("[%s]%samix=inputs=%d:duration=first:dropout_transition=0:normalize=0%s[%s]", m.in, strings.Join(labels, ""), len(labels)+1, tail, m.out))
}
