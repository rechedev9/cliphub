package recapplan

import (
	"cmp"
	"fmt"
	"math"
	"regexp"
	"slices"
	"strconv"

	"github.com/rechedev9/cliphub/internal/artifacts"
	"github.com/rechedev9/cliphub/internal/killplan"
)

const (
	SecondCameraLeadSeconds   = 5
	SecondCameraTailSeconds   = 1
	SecondCameraStillUnits    = 128.0
	SecondCameraRewindSeconds = 2
	SecondCameraDefaultMax    = 5
	SecondCameraMaxWindows    = 10
	SecondCameraMaxRemoved    = 60

	WarnSecondCameraSkipped     = "second_camera_skipped"
	WarnSecondCameraUnavailable = "second_camera_unavailable"
	WarnSecondCameraOmitted     = "second_camera_omitted"

	firstIndividualSteamID64 = 76561197960265728
)

var cameraWindowIDPattern = regexp.MustCompile(`^round-[0-9]{3,4}-camera-[0-9]{1,9}$`)

type SecondCameraOptions struct {
	Enabled    bool     `json:"enabled"`
	MaxWindows int      `json:"max_windows"`
	Removed    []string `json:"removed,omitempty"`
}

type SecondCamera struct {
	Windows []CameraWindow `json:"windows"`
}

type CameraWindow struct {
	ID             string  `json:"id"`
	RoundID        string  `json:"round_id"`
	Round          int     `json:"round"`
	KillTick       int     `json:"kill_tick"`
	StartTick      int     `json:"start_tick"`
	EndTick        int     `json:"end_tick"`
	EnemySteamID64 string  `json:"enemy_steamid64"`
	EnemyName      string  `json:"enemy_name"`
	Score          float64 `json:"score"`
	MoveUnits      float64 `json:"move_units"`
}

func CameraWindowID(roundID string, killTick int) string {
	return fmt.Sprintf("%s-camera-%d", roundID, killTick)
}

func (o SecondCameraOptions) Cap() int {
	if o.MaxWindows == 0 {
		return SecondCameraDefaultMax
	}
	return o.MaxWindows
}

func canonicalSecondCamera(o *SecondCameraOptions) *SecondCameraOptions {
	if o == nil || !o.Enabled {
		return nil
	}
	canonical := &SecondCameraOptions{Enabled: true, MaxWindows: o.Cap()}
	if len(o.Removed) > 0 {
		canonical.Removed = slices.Compact(slices.Sorted(slices.Values(o.Removed)))
	}
	return canonical
}

func (o Options) validateSecondCameraOptions() error {
	c := o.SecondCamera
	if c == nil {
		return nil
	}
	if c.MaxWindows < 0 || c.MaxWindows > SecondCameraMaxWindows {
		return fmt.Errorf("second_camera.max_windows must be between 1 and %d", SecondCameraMaxWindows)
	}
	if len(c.Removed) > SecondCameraMaxRemoved {
		return fmt.Errorf("second_camera.removed holds at most %d moments", SecondCameraMaxRemoved)
	}
	for _, id := range c.Removed {
		if !validCameraWindowID(id) {
			return fmt.Errorf("invalid second camera moment %q", id)
		}
	}
	return nil
}

func validCameraWindowID(id string) bool {
	return cameraWindowIDPattern.MatchString(id) && artifacts.ValidateArtifactToken("second camera moment", id) == nil
}

func humanSteamID(id string) bool {
	if !steamIDPattern.MatchString(id) {
		return false
	}
	n, err := strconv.ParseUint(id, 10, 64)
	return err == nil && n > firstIndividualSteamID64
}

func cameraWindowBounds(r Round, killTick, rate int) (int, int) {
	return killTick - SecondCameraLeadSeconds*rate, min(killTick+SecondCameraTailSeconds*rate, r.RequestedEndTick)
}

func (d *Document) planSecondCamera(f Facts) {
	d.SecondCamera = nil
	o := d.Options.SecondCamera
	if o == nil || !o.Enabled {
		return
	}
	still, ok := f.Holds.byKill()
	if !ok {
		d.Warnings = append(d.Warnings, Notice{Code: WarnSecondCameraUnavailable, Message: "No se han podido leer los movimientos de los enemigos en esta demo, así que el vídeo sale sin segunda cámara."})
		return
	}
	scores := playScores(d.Rounds, d.Clock.TickRate)
	var candidates []CameraWindow
	for _, r := range d.Rounds {
		var pick CameraWindow
		found := false
		for _, k := range r.Kills {
			w, ok := d.cameraCandidate(r, k, still)
			if !ok || !f.leavesTheMatchRunning(w, d.Clock.TickRate) {
				continue
			}
			w.Score = scores[playKill{r.ID, k.Tick, k.Victim.SteamID64}]
			if !found || betterInRound(w, pick) {
				pick, found = w, true
			}
		}
		if found {
			candidates = append(candidates, pick)
		}
	}
	if len(candidates) == 0 {
		d.Warnings = append(d.Warnings, Notice{Code: WarnSecondCameraSkipped, Message: "Ninguna baja del jugador cumple las condiciones de la segunda cámara, así que el vídeo sale sin ella."})
		return
	}
	slices.SortStableFunc(candidates, func(a, b CameraWindow) int {
		return cmp.Or(cmp.Compare(b.Score, a.Score), cmp.Compare(a.Round, b.Round))
	})
	candidates = candidates[:min(len(candidates), o.Cap())]
	candidates = slices.DeleteFunc(candidates, func(w CameraWindow) bool { return slices.Contains(o.Removed, w.ID) })
	if len(candidates) == 0 {
		return
	}
	slices.SortStableFunc(candidates, func(a, b CameraWindow) int { return cmp.Compare(a.KillTick, b.KillTick) })
	d.SecondCamera = &SecondCamera{Windows: candidates}
}

func (f Facts) leavesTheMatchRunning(w CameraWindow, rate int) bool {
	if w.EndTick+SecondCameraRewindSeconds*rate > f.EndTick {
		return false
	}
	last := len(f.Rounds) - 1
	return last < 0 || f.Rounds[last].ID != w.RoundID || w.EndTick+rate <= f.Rounds[last].RoundEndTick
}

func betterInRound(a, b CameraWindow) bool {
	return cmp.Or(cmp.Compare(b.Score, a.Score), cmp.Compare(a.MoveUnits, b.MoveUnits), cmp.Compare(a.KillTick, b.KillTick)) < 0
}

func (d Document) cameraCandidate(r Round, k killplan.Kill, still map[holdKey]KillHold) (CameraWindow, bool) {
	enemy := k.Victim.SteamID64
	if k.Killer.SteamID64 != d.Input.TargetSteamID64 || !humanSteamID(enemy) || enemy == d.Input.TargetSteamID64 || k.Victim.TeamAtKill == k.Killer.TeamAtKill {
		return CameraWindow{}, false
	}
	start, end := cameraWindowBounds(r, k.Tick, d.Clock.TickRate)
	if start < r.LiveStartTick || k.Tick > r.LiveEndTick || end <= k.Tick {
		return CameraWindow{}, false
	}
	hold, ok := still[holdKey{k.Tick, enemy}]
	if !ok || !hold.Covered || math.IsNaN(hold.MoveUnits) || hold.MoveUnits < 0 || hold.MoveUnits > SecondCameraStillUnits {
		return CameraWindow{}, false
	}
	return CameraWindow{ID: CameraWindowID(r.ID, k.Tick), RoundID: r.ID, Round: r.Number, KillTick: k.Tick, StartTick: start, EndTick: end, EnemySteamID64: enemy, EnemyName: k.Victim.NameInDemo, MoveUnits: hold.MoveUnits}, true
}

type playKill struct {
	roundID string
	tick    int
	victim  string
}

func playScores(rounds []Round, rate int) map[playKill]float64 {
	var segments []killplan.Segment
	var owners []string
	for _, r := range rounds {
		for _, play := range roundPlays(r, rate) {
			segments = append(segments, play.segment(fmt.Sprintf("%06d", len(segments)), r, rate))
			owners = append(owners, r.ID)
		}
	}
	bySegment := map[string]float64{}
	for _, m := range rankSegments(segments, rate) {
		bySegment[m.SegmentID] = m.Score
	}
	scores := map[playKill]float64{}
	for i, segment := range segments {
		for _, k := range segment.Kills {
			scores[playKill{owners[i], k.Tick, k.Victim.SteamID64}] = bySegment[segment.ID]
		}
	}
	return scores
}

func (d Document) validateSecondCamera() error {
	c := d.SecondCamera
	if c == nil {
		return nil
	}
	o := d.Options.SecondCamera
	if o == nil || !o.Enabled {
		return fmt.Errorf("second camera is not enabled in the options")
	}
	if len(c.Windows) == 0 || len(c.Windows) > o.Cap() {
		return fmt.Errorf("second camera holds between 1 and %d moments", o.Cap())
	}
	rounds := map[string]Round{}
	for _, r := range d.Rounds {
		rounds[r.ID] = r
	}
	used := map[string]bool{}
	for i, w := range c.Windows {
		if w.ID != CameraWindowID(w.RoundID, w.KillTick) || !validCameraWindowID(w.ID) {
			return fmt.Errorf("invalid second camera moment %q", w.ID)
		}
		if used[w.RoundID] || (i > 0 && w.KillTick < c.Windows[i-1].KillTick) {
			return fmt.Errorf("second camera moments must be one per round in kill order: %s", w.ID)
		}
		used[w.RoundID] = true
		r, ok := rounds[w.RoundID]
		if !ok || w.Round != r.Number {
			return fmt.Errorf("second camera moment references a round outside the video: %s", w.ID)
		}
		start, end := cameraWindowBounds(r, w.KillTick, d.Clock.TickRate)
		if w.StartTick != start || start < r.LiveStartTick || w.KillTick > r.LiveEndTick || w.EndTick != end || end <= w.KillTick {
			return fmt.Errorf("second camera moment lies outside the live play of %s: %s", r.ID, w.ID)
		}
		killed := slices.ContainsFunc(r.Kills, func(k killplan.Kill) bool {
			return k.Tick == w.KillTick && k.Victim.SteamID64 == w.EnemySteamID64 && k.Killer.SteamID64 == d.Input.TargetSteamID64
		})
		if !killed || !humanSteamID(w.EnemySteamID64) || w.EnemySteamID64 == d.Input.TargetSteamID64 {
			return fmt.Errorf("second camera moment is not a kill of the player in %s: %s", r.ID, w.ID)
		}
		if slices.Contains(o.Removed, w.ID) {
			return fmt.Errorf("second camera moment was removed: %s", w.ID)
		}
	}
	return nil
}

func WithCameraEvidence(d Document, usable map[string]bool) (Document, error) {
	if d.SecondCamera == nil {
		return d, nil
	}
	var kept []CameraWindow
	warnings := slices.Clone(d.Warnings)
	for _, w := range d.SecondCamera.Windows {
		if usable[w.ID] {
			kept = append(kept, w)
			continue
		}
		enemy := cmp.Or(w.EnemyName, "enemigo")
		warnings = append(warnings, Notice{Code: WarnSecondCameraOmitted, Message: fmt.Sprintf("La segunda cámara de la ronda %d (%s) no se pudo grabar y el vídeo sale sin ella.", w.Round, enemy), RoundID: w.RoundID})
	}
	if len(kept) == len(d.SecondCamera.Windows) {
		return d, nil
	}
	d.Warnings = warnings
	d.SecondCamera = nil
	if len(kept) > 0 {
		d.SecondCamera = &SecondCamera{Windows: kept}
	}
	hash, err := d.Hash()
	if err != nil {
		return Document{}, err
	}
	d.PlanHash = hash
	return d, nil
}
