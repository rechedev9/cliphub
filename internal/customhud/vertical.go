package customhud

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"os"
	"sort"
	"strconv"
	"strings"

	"github.com/rechedev9/cliphub/internal/overlaylang"
)

// VerticalVersion names the 9:16 HUD track format.
const VerticalVersion = "vertical-hud-v1"

// The 9:16 HUD is laid out on a 1080x1920 frame, from the hook title's line
// (the editor waits for the title to go) down to above the centered kill feed at y=300.
const (
	VerticalWidth, VerticalHeight = 1080, 1920
	verticalTop                   = 150
	verticalScoreH                = 88
	verticalClockW                = 200
	verticalSideW                 = 168
	verticalGap                   = 6
	verticalAliveY                = verticalTop + verticalScoreH + 8
	verticalAliveH                = 30
	verticalPips                  = 5
	verticalFadeFrames            = 12
	maxVerticalTrackBytes         = 8 << 20
)

// VerticalState is what the 9:16 HUD shows at one moment: the scoreboard and who is alive.
type VerticalState struct {
	CTName        string `json:"ct_name"`
	TName         string `json:"t_name"`
	CTScore       int    `json:"ct_score"`
	TScore        int    `json:"t_score"`
	Round         int    `json:"round"`
	Phase         string `json:"phase"`
	TimeRemaining int    `json:"time_remaining"` // -1 means unobserved, never a guessed timer.
	// CTAlive and TAlive are -1 when a slot has no pawn data, so the count is never a guess.
	CTAlive int `json:"ct_alive"`
	TAlive  int `json:"t_alive"`
	CTSlots int `json:"ct_slots"`
	TSlots  int `json:"t_slots"`
}

// VerticalSpan holds one state over [TickStart, TickEnd) of the source demo.
type VerticalSpan struct {
	TickStart int           `json:"tick_start"`
	TickEnd   int           `json:"tick_end"`
	State     VerticalState `json:"state"`
}

type VerticalSegment struct {
	ID    string         `json:"id"`
	Spans []VerticalSpan `json:"spans"`
}

// VerticalTrack is the scoreboard of each captured play, read from the HUD telemetry.
type VerticalTrack struct {
	Version    string            `json:"version"`
	DemoSHA256 string            `json:"demo_sha256"`
	TickRate   int               `json:"tick_rate"`
	Segments   []VerticalSegment `json:"segments"`
}

// VerticalWindow is one captured play in source ticks.
type VerticalWindow struct {
	ID                 string
	TickStart, TickEnd int
}

// VerticalCue is one state on the edited timeline of a Short.
type VerticalCue struct {
	StartSeconds float64
	EndSeconds   float64
	State        VerticalState
}

// VerticalTrack reads the scoreboard inside each window. Team names and sides
// depend on every earlier snapshot, so the whole timeline is walked once.
func (d Timeline) VerticalTrack(windows []VerticalWindow) (VerticalTrack, error) {
	if err := d.Validate(); err != nil {
		return VerticalTrack{}, err
	}
	type change struct {
		tick  int
		state VerticalState
	}
	var changes []change
	tracker := &roundTracker{mapName: d.Map, tickRate: d.TickRate}
	names := &Renderer{}
	for _, recorded := range d.Snapshots {
		state := verticalState(names, tracker.next(recorded))
		if n := len(changes); n > 0 && changes[n-1].state == state {
			continue
		}
		changes = append(changes, change{tick: recorded.Tick, state: state})
	}
	track := VerticalTrack{Version: VerticalVersion, DemoSHA256: d.DemoSHA256, TickRate: d.TickRate, Segments: []VerticalSegment{}}
	for _, w := range windows {
		if w.ID == "" || w.TickStart < 0 || w.TickEnd <= w.TickStart {
			return VerticalTrack{}, fmt.Errorf("invalid vertical HUD window %q", w.ID)
		}
		end := min(w.TickEnd, d.EndTick+1)
		segment := VerticalSegment{ID: w.ID, Spans: []VerticalSpan{}}
		i := max(0, sort.Search(len(changes), func(i int) bool { return changes[i].tick > w.TickStart })-1)
		for ; i < len(changes) && changes[i].tick < end; i++ {
			stop := end
			if i+1 < len(changes) {
				stop = min(stop, changes[i+1].tick)
			}
			if start := max(changes[i].tick, w.TickStart); stop > start {
				segment.Spans = append(segment.Spans, VerticalSpan{TickStart: start, TickEnd: stop, State: changes[i].state})
			}
		}
		track.Segments = append(track.Segments, segment)
	}
	return track, nil
}

func verticalState(r *Renderer, s Snapshot) VerticalState {
	v := VerticalState{CTName: r.teamName(s, "CT"), TName: r.teamName(s, "T"), CTScore: s.CTScore, TScore: s.TScore,
		Round: s.Round, Phase: s.Phase, TimeRemaining: s.TimeRemaining, CTAlive: -1, TAlive: -1}
	var ct, tr []Player
	for _, p := range s.Players {
		switch {
		case p.Inactive:
		case p.Side == "CT":
			ct = append(ct, p)
		default:
			tr = append(tr, p)
		}
	}
	v.CTSlots, v.TSlots = len(ct), len(tr)
	if alive, ok := aliveCount(ct); ok {
		v.CTAlive = alive
	}
	if alive, ok := aliveCount(tr); ok {
		v.TAlive = alive
	}
	return v
}

func (t VerticalTrack) Validate() error {
	if t.Version != VerticalVersion || len(t.DemoSHA256) != 64 || t.TickRate < 1 || t.TickRate > 1024 {
		return fmt.Errorf("invalid vertical HUD track identity")
	}
	for _, segment := range t.Segments {
		if segment.ID == "" || len(segment.ID) > 128 {
			return fmt.Errorf("invalid vertical HUD segment")
		}
		last := -1
		for _, span := range segment.Spans {
			s := span.State
			if span.TickStart < last || span.TickEnd <= span.TickStart || s.Round < 0 || s.TimeRemaining < -1 || s.TimeRemaining > 86400 ||
				s.CTScore < 0 || s.TScore < 0 || s.CTAlive < -1 || s.TAlive < -1 || s.CTSlots < 0 || s.TSlots < 0 ||
				s.CTAlive > s.CTSlots || s.TAlive > s.TSlots || len(s.CTName) > 256 || len(s.TName) > 256 || !validPhase(s.Phase) {
				return fmt.Errorf("invalid vertical HUD span in %s at tick %d", segment.ID, span.TickStart)
			}
			last = span.TickEnd
		}
	}
	return nil
}

func validPhase(phase string) bool {
	switch phase {
	case "unknown", "freeze", "live", "planted", "defusing", "ended":
		return true
	}
	return false
}

// LoadVerticalTrack reads and validates the track the render worker wrote.
func LoadVerticalTrack(path string) (VerticalTrack, error) {
	file, err := os.Open(path)
	if err != nil {
		return VerticalTrack{}, err
	}
	defer file.Close()
	body, err := io.ReadAll(io.LimitReader(file, maxVerticalTrackBytes+1))
	if err != nil {
		return VerticalTrack{}, err
	}
	if len(body) > maxVerticalTrackBytes {
		return VerticalTrack{}, fmt.Errorf("vertical HUD track exceeds byte limit")
	}
	var track VerticalTrack
	dec := json.NewDecoder(bytes.NewReader(body))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&track); err != nil {
		return VerticalTrack{}, fmt.Errorf("decode vertical HUD track: %w", err)
	}
	return track, track.Validate()
}

// VerticalTheme is a catalog design for the 9:16 HUD; an empty id is the first design.
func VerticalTheme(id string) (Theme, error) {
	if id == "" {
		return catalog[0], nil
	}
	theme, ok := Lookup(id)
	if !ok {
		return Theme{}, fmt.Errorf("unknown custom HUD %q", id)
	}
	return theme, nil
}

// VerticalASS draws the cues as a 1080x1920 libass script. Each node is written
// only when it changes; a run of cues fades in after a gap and out before one.
func VerticalASS(script VerticalScript) (string, error) {
	if err := loadAssets(); err != nil {
		return "", err
	}
	r, cues := &Renderer{Theme: script.Theme}, script.Cues
	var b strings.Builder
	fmt.Fprintf(&b, "[Script Info]\nScriptType: v4.00+\nPlayResX: %d\nPlayResY: %d\nScaledBorderAndShadow: yes\nWrapStyle: 2\n\n", VerticalWidth, VerticalHeight)
	b.WriteString("[V4+ Styles]\nFormat: Name, Fontname, Fontsize, PrimaryColour, SecondaryColour, OutlineColour, BackColour, Bold, Italic, Underline, StrikeOut, ScaleX, ScaleY, Spacing, Angle, BorderStyle, Outline, Shadow, Alignment, MarginL, MarginR, MarginV, Encoding\nStyle: HUD,Montserrat ExtraBold,20,&H00FFFFFF,&H00FFFFFF,&H00000000,&H00000000,0,0,0,0,100,100,0,0,1,0,0,7,0,0,0,1\n\n[Events]\nFormat: Layer, Start, End, Style, Name, MarginL, MarginR, MarginV, Effect, Text\n")
	type activeNode struct {
		node     Node
		start    int64
		entrance bool
	}
	active := map[string]activeNode{}
	emit := func(a activeNode, end int64, exit bool) {
		if end <= a.start {
			return
		}
		fade := min(verticalFadeFrames, (end-a.start)/2)
		in, out := int64(0), int64(0)
		if a.entrance {
			in = fade
		}
		if exit {
			out = fade
		}
		text := assNode(a.node)
		if in > 0 || out > 0 {
			text = strings.Replace(text, "{", fmt.Sprintf(`{\fad(%d,%d)`, in*1000/60, out*1000/60), 1)
		}
		fmt.Fprintf(&b, "Dialogue: %d,%s,%s,HUD,vertical,0,0,0,,%s\n", a.node.Layer, assTime(a.start), assTime(end), text)
	}
	sortedKeys := func() []string {
		keys := make([]string, 0, len(active))
		for key := range active {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		return keys
	}
	runEnd := int64(-1)
	for _, cue := range cues {
		start, end := secondsFrame(cue.StartSeconds), secondsFrame(cue.EndSeconds)
		if start < 0 {
			return "", fmt.Errorf("vertical HUD cue starts before the video")
		}
		start = max(start, runEnd)
		if end <= start {
			continue
		}
		if start > runEnd {
			for _, key := range sortedKeys() {
				emit(active[key], runEnd, true)
			}
			clear(active)
		}
		next := map[string]Node{}
		for _, node := range r.verticalScene(cue.State, script.Language) {
			next[node.ID] = node
		}
		for _, key := range sortedKeys() {
			if node, ok := next[key]; !ok || node != active[key].node {
				emit(active[key], start, false)
				delete(active, key)
			}
		}
		entrance := len(active) == 0
		for key, node := range next {
			if _, ok := active[key]; !ok {
				active[key] = activeNode{node: node, start: start, entrance: entrance}
			}
		}
		runEnd = end
	}
	for _, key := range sortedKeys() {
		emit(active[key], runEnd, true)
	}
	return b.String(), nil
}

func secondsFrame(seconds float64) int64 { return int64(math.Round(seconds * 60)) }

func (r *Renderer) verticalScene(v VerticalState, language overlaylang.Language) []Node {
	s := &scene{r: r}
	t := r.Theme
	clockX := VerticalWidth/2 - verticalClockW/2
	sides := []struct {
		id, name, color         string
		x, points, alive, slots int
	}{
		{"ct", v.CTName, t.CT, clockX - verticalGap - verticalSideW, v.CTScore, v.CTAlive, v.CTSlots},
		{"t", v.TName, t.T, clockX + verticalClockW + verticalGap, v.TScore, v.TAlive, v.TSlots},
	}
	s.panel("score", clockX, verticalTop, verticalClockW, verticalScoreH, t.Muted)
	for _, side := range sides {
		id, mid := "score/"+side.id, side.x+verticalSideW/2
		s.path(id+"-shadow", t.plate(side.x, verticalTop+3, verticalSideW, verticalScoreH), "000000", 0)
		s.fade(.18)
		s.path(id+"-plate", t.plate(side.x, verticalTop, verticalSideW, verticalScoreH), side.color, 2)
		s.typeText(id+"-points", strconv.Itoa(side.points), mid, verticalTop+36, 58, verticalSideW-16, 700, t.Background, "center")
		s.typeText(id+"-name", side.name, mid, verticalTop+71, 20, verticalSideW-20, 600, t.Background, "center")
		s.verticalAlive(id, side.x, side.alive, side.slots, side.color)
	}
	color, text := t.Text, verticalTexts(language)
	phase := fmt.Sprintf(text.round, v.Round)
	switch v.Phase {
	case "freeze":
		phase = text.freeze
	case "planted":
		phase, color = text.planted, t.T
	case "defusing":
		phase, color = text.defusing, t.CT
	case "ended":
		phase = text.ended
	}
	center := VerticalWidth / 2
	if v.Phase == "ended" {
		s.typeText("score/phase", phase, center, verticalTop+verticalScoreH/2, 24, verticalClockW-24, 700, color, "center")
	} else {
		clock := "--:--"
		if v.TimeRemaining >= 0 {
			clock = fmt.Sprintf("%d:%02d", v.TimeRemaining/60, v.TimeRemaining%60)
		}
		s.typeText("score/time", clock, center, verticalTop+36, 50, verticalClockW-56, 700, color, "center")
		s.typeText("score/phase", phase, center, verticalTop+71, 19, verticalClockW-24, 600, color, "center")
	}
	if v.Phase == "planted" || v.Phase == "defusing" {
		s.icon("score/bomb", "status/icon_bomb_default", clockX+12, verticalTop+20, 24, 32, color, false)
		s.rect("score/bomb-phase", clockX+16, verticalTop+verticalScoreH-5, verticalClockW-32, 3, color, 4)
	}
	sort.SliceStable(s.nodes, func(i, j int) bool { return s.nodes[i].Layer < s.nodes[j].Layer })
	return s.nodes
}

// verticalAlive is a count and one pip per roster slot under a score plate; an unknown count draws nothing.
func (s *scene) verticalAlive(id string, x, alive, slots int, color string) {
	if alive < 0 || slots == 0 {
		return
	}
	t := s.r.Theme
	s.path(id+"-alive-plate", t.plate(x, verticalAliveY, verticalSideW, verticalAliveH), t.Background, 1)
	s.fade(t.Opacity)
	s.typeText(id+"-alive", strconv.Itoa(alive), x+22, verticalAliveY+verticalAliveH/2, 25, 28, 700, color, "center")
	const pipW, pipGap = 20, 4
	for i := range min(slots, verticalPips) {
		pipColor := t.Surface
		if i < alive {
			pipColor = color
		}
		s.rect(fmt.Sprintf("%s-pip-%d", id, i), x+42+i*(pipW+pipGap), verticalAliveY+verticalAliveH/2-5, pipW, 10, pipColor, 3)
	}
}

// VerticalExample is the illustrative state of the design picker; a render always reads the demo.
func VerticalExample() VerticalState {
	return VerticalState{CTName: "CT", TName: "T", CTScore: 7, TScore: 9, Round: 17, Phase: "live", TimeRemaining: 74, CTAlive: 3, TAlive: 4, CTSlots: 5, TSlots: 5}
}

// VerticalSVG draws the band of the 9:16 HUD with the same display list as the video.
func (r *Renderer) VerticalSVG(v VerticalState, language overlaylang.Language) string {
	left := VerticalWidth/2 - verticalClockW/2 - verticalGap - verticalSideW - 40
	top, width := verticalTop-30, VerticalWidth-2*left
	height := verticalAliveY + verticalAliveH + 30 - top
	return svgDocument(svgFrame{name: r.Theme.Name, x: left, y: top, w: width, h: height}, r.verticalScene(v, language))
}
