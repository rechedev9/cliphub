package customhud

import (
	"fmt"
	"strings"
)

// Round facts sit in the strip below the scoreboard, clear of the reticle.
const (
	roundFactsY  = 96
	streakY      = 120
	mvpY         = 142
	plateWidth   = 74
	ctPlateX     = Width/2 - 160
	tPlateX      = Width/2 + 86
	defuseX      = Width/2 - 80
	defuseWidth  = 160
	defuseHeight = 34
)

// defuseTrack is shared by the static track and the animated fill in ASS.
func defuseTrack() (x, y, w, h int) {
	return defuseX + 10, roundFactsY + 25, defuseWidth - 20, 4
}

func (s *scene) roundFacts(state Snapshot, ct, tr []Player) {
	t := s.r.Theme
	for _, side := range []struct {
		name, color string
		x           int
		players     []Player
	}{{"CT", t.CT, ctPlateX, ct}, {"T", t.T, tPlateX, tr}} {
		id := "score/" + strings.ToLower(side.name)
		if alive, ok := aliveCount(side.players); ok {
			s.path(id+"-alive-plate", t.plate(side.x, roundFactsY, plateWidth, 22), t.Background, 1)
			s.fade(t.Opacity)
			s.typeText(id+"-alive", fmt.Sprintf("%d ALIVE", alive), side.x+plateWidth/2, roundFactsY+11, 13, plateWidth-8, 700, side.color, "center")
		}
		if state.Streak.Side == side.name && state.Streak.Rounds >= 4 {
			start := len(s.nodes)
			s.path(id+"-streak-plate", t.plate(side.x, streakY, plateWidth, 18), side.color, 2)
			s.typeText(id+"-streak", fmt.Sprintf("%d IN A ROW", state.Streak.Rounds), side.x+plateWidth/2, streakY+9, 11, plateWidth-6, 700, t.Background, "center")
			s.fadeInFrom(start)
		}
	}
	if state.Phase == "defusing" && state.Defuse != nil {
		s.defusePanel(*state.Defuse)
	}
	if state.Phase == "ended" && state.Winner != "" && s.r.Options.RoundWinBanner {
		s.roundWinBanner(state)
	}
	if state.Phase == "ended" && state.MVP != nil {
		for _, p := range state.Players {
			if p.SteamID == state.MVP.SteamID {
				s.mvpCard(p, state.MVP.Reason)
				break
			}
		}
	}
}

// Any slot without pawn data makes the count unknown, so it is left out.
func aliveCount(players []Player) (int, bool) {
	alive := 0
	for _, p := range players {
		if !p.Known {
			return 0, false
		}
		if p.Alive {
			alive++
		}
	}
	return alive, len(players) > 0
}

func (s *scene) roundKills(p Player, x int, accent string) {
	if p.RoundKills < 3 {
		return
	}
	label := fmt.Sprintf("%dK", p.RoundKills)
	if p.RoundKills >= 5 {
		label = "ACE"
	}
	t, start := s.r.Theme, len(s.nodes)
	id := "player/" + p.SteamID + "/round-kills"
	s.path(id+"-plate", t.plate(x+12, roundFactsY, 40, 18), accent, 2)
	s.typeText(id, label, x+32, roundFactsY+9, 12, 36, 700, t.Background, "center")
	s.fadeInFrom(start)
}

func (s *scene) defusePanel(d Defuse) {
	t, start := s.r.Theme, len(s.nodes)
	s.panel("defuse", defuseX, roundFactsY, defuseWidth, defuseHeight, t.CT)
	label, labelX := "NO KIT", defuseX+12
	if d.Kit {
		label, labelX = "WITH KIT", defuseX+34
		s.icon("defuse/kit", "status/icon_defuse_default", defuseX+10, roundFactsY+4, 18, 16, t.CT, false)
	}
	s.typeText("defuse/label", label, labelX, roundFactsY+12, 13, defuseWidth-(labelX-defuseX)-10, 700, t.Text, "left")
	x, y, w, h := defuseTrack()
	s.rect("defuse/track", x, y, w, h, t.Surface, 4)
	s.fadeInFrom(start)
}

func (s *scene) mvpCard(p Player, reason string) {
	t, start := s.r.Theme, len(s.nodes)
	const x, w, h = Width/2 - 170, 340, 58
	accent := t.CT
	if p.Side == "T" {
		accent = t.T
	}
	s.panel("mvp", x, mvpY, w, h, accent)
	s.path("mvp/badge", t.plate(x+8, mvpY+8, 64, h-16), accent, 2)
	s.typeText("mvp/label", "MVP", x+40, mvpY+h/2, 20, 56, 700, t.Background, "center")
	s.text("mvp/name", p.Name, x+84, mvpY+20, 22, w-96, t.Text, "left")
	var detail []string
	switch {
	case p.RoundKills == 1:
		detail = append(detail, "1 KILL")
	case p.RoundKills > 1:
		detail = append(detail, fmt.Sprintf("%d KILLS", p.RoundKills))
	}
	switch reason {
	case MVPEliminations:
		detail = append(detail, "MOST ELIMINATIONS")
	case MVPBombDefused:
		detail = append(detail, "BOMB DEFUSED")
	case MVPBombPlanted:
		detail = append(detail, "BOMB PLANTED")
	}
	if len(detail) > 0 {
		s.typeText("mvp/detail", strings.Join(detail, " · "), x+84, mvpY+42, 13, w-96, 600, t.Muted, "left")
	}
	s.fadeInFrom(start)
}

func (s *scene) fadeInFrom(start int) {
	for i := start; i < len(s.nodes); i++ {
		s.nodes[i].FadeIn = true
	}
}

// The banner sits between the alive tabs, where the defuse panel shows while defusing.
func (s *scene) roundWinBanner(state Snapshot) {
	t, start := s.r.Theme, len(s.nodes)
	accent := t.CT
	if state.Winner == "T" {
		accent = t.T
	}
	s.panel("round-win", defuseX, roundFactsY, defuseWidth, 40, accent)
	s.typeText("round-win/team", s.r.teamName(state, state.Winner), Width/2, roundFactsY+14, 16, defuseWidth-20, 700, accent, "center")
	s.typeText("round-win/label", "WINS THE ROUND", Width/2, roundFactsY+30, 11, defuseWidth-20, 600, t.Text, "center")
	s.fadeInFrom(start)
}

// teamName is the name of the team now on side: the plan's name for the side
// it started on, then the demo's clan name, then the side itself.
func (r *Renderer) teamName(state Snapshot, side string) string {
	startCT := side == "CT"
	if state.Swapped {
		startCT = !startCT
	}
	name, clan := r.Teams.TStart, state.TName
	if startCT {
		name = r.Teams.CTStart
	}
	if !state.SidesKnown {
		name = ""
	}
	if side == "CT" {
		clan = state.CTName
	}
	switch {
	case strings.TrimSpace(name) != "":
		return strings.ToUpper(strings.TrimSpace(name))
	case strings.TrimSpace(clan) != "" && clan != "COUNTER-TERRORISTS" && clan != "TERRORISTS":
		return strings.ToUpper(strings.TrimSpace(clan))
	}
	return side
}
