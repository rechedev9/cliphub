package parser

import (
	"errors"
	"fmt"
	"strconv"

	demoinfocs "github.com/markus-wa/demoinfocs-golang/v5/pkg/demoinfocs"
	"github.com/markus-wa/demoinfocs-golang/v5/pkg/demoinfocs/events"
	"github.com/markus-wa/demoinfocs-golang/v5/pkg/demoinfocs/msg"

	"github.com/rechedev9/cliphub/internal/killplan"
	"github.com/rechedev9/cliphub/internal/rules"
)

func runUtility(p demoinfocs.Parser, target string, r rules.Rules, m PlanMeta) (killplan.Plan, error) {
	targetID, err := strconv.ParseUint(target, 10, 64)
	if err != nil {
		return killplan.Plan{}, fmt.Errorf("invalid target steamid %q: %w", target, err)
	}

	c := NewUtilityCollector(target, r)
	var mapName string
	var maxTick int
	watch := newUtilityWatch(targetID, target, &maxTick, c.RecordTargetIdentity)

	p.RegisterNetMessageHandler(func(info *msg.CSVCMsg_ServerInfo) {
		if name := info.GetMapName(); name != "" {
			mapName = name
		}
	})
	p.RegisterEventHandler(func(events.MatchStart) {
		c.resetForMatchStart()
	})
	watch.bind(p)

	p.RegisterEventHandler(func(events.RoundEnd) {
		gs := p.GameState()
		tick := gs.IngameTick()
		if tick > maxTick {
			maxTick = tick
		}
		c.RecordRoundEnd(RoundEnd{Round: gs.TotalRoundsPlayed() + 1, Tick: tick})
	})

	if err := parseToEnd(p); err != nil {
		return killplan.Plan{}, fmt.Errorf("parsing demo: %w", err)
	}

	if m.Tickrate <= 0 {
		m.Tickrate = int(p.TickRate())
	}
	if m.Map == "" {
		m.Map = mapName
	}
	if m.DurationTicks <= 0 {
		m.DurationTicks = maxTick
	}

	for _, u := range watch.flush() {
		annotateUtilityDestination(u, m.Map)
		c.RecordUtility(*u)
	}

	plan, err := c.Build(m)
	if err != nil {
		if errors.Is(err, ErrTargetNotFound) {
			return killplan.Plan{}, ErrTargetNotFound
		}
		return killplan.Plan{}, err
	}
	return plan, nil
}
