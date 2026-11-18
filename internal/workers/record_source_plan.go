package workers

import (
	"fmt"

	"github.com/rechedev9/cliphub/internal/deathplan"
	"github.com/rechedev9/cliphub/internal/job"
	"github.com/rechedev9/cliphub/internal/killplan"
	"github.com/rechedev9/cliphub/internal/parser"
	"github.com/rechedev9/cliphub/internal/recapplan"
	"github.com/rechedev9/cliphub/internal/recording"
	"github.com/rechedev9/cliphub/internal/renderplan"
	"github.com/rechedev9/cliphub/internal/storage"
)

// planSource is the plan a capture records from.
type planSource int

const (
	planSourceKills planSource = iota
	planSourceRecap
	planSourceDeaths
)

// recordSourcePlan returns the plan a capture records from and the requested segment ids mapped onto it:
// a pre-roll recuts the kill plan, so an id merged into an earlier play becomes that play's id.
func recordSourcePlan(store storage.Storage, j job.Job, req recordRequest) (*killplan.Plan, []string, error) {
	switch req.source {
	case planSourceKills:
		plan, err := preRolledKillPlan(store, j, req.preRollSeconds)
		if err != nil {
			return nil, nil, err
		}
		return plan, parser.PreRollIDs(*j.KillPlan, req.preRollSeconds, req.segmentIDs), nil
	case planSourceDeaths:
		deaths, ok, err := deathplan.Load(store, j.ID)
		if err != nil {
			return nil, nil, err
		}
		if !ok || len(deaths.Segments) == 0 {
			return nil, nil, fmt.Errorf("job %s has no death clips", j.ID)
		}
		return &deaths, req.segmentIDs, nil
	}
	recap, ok, err := recapplan.Load(store, j.ID)
	if err != nil {
		return nil, nil, err
	}
	if !ok {
		return nil, nil, fmt.Errorf("job %s has no recap plan", j.ID)
	}
	if len(recap.Segments) == 0 {
		return nil, nil, fmt.Errorf("job %s recap plan has no rounds", j.ID)
	}
	return &recap, req.segmentIDs, nil
}

// preRolledKillPlan is the job's kill plan with each play starting seconds before its first kill, never before the
// start of its round as the parse's round facts know it; 0 seconds keeps the analysed plan.
func preRolledKillPlan(store storage.Storage, j job.Job, seconds int) (*killplan.Plan, error) {
	if j.KillPlan == nil {
		return nil, fmt.Errorf("job %s has no kill plan", j.ID)
	}
	if seconds == 0 {
		return j.KillPlan, nil
	}
	// A legacy parse without round facts clamps the start at the demo start only.
	facts, _, err := recapplan.LoadFacts(store, j.ID)
	if err != nil {
		return nil, fmt.Errorf("load round starts for the pre-roll: %w", err)
	}
	plan := parser.WithPreRoll(*j.KillPlan, seconds, parser.FactsRoundStarts(facts))
	return &plan, nil
}

// preRollRender recuts the kill plan of a render of plays to its pre-roll and maps the selection onto it.
// Death reels, match recaps and Full Demos keep their own plans.
func preRollRender(store storage.Storage, j *job.Job, edit renderplan.EditRequest, segmentIDs []string) ([]string, bool, error) {
	if edit.DeathReel || edit.MatchRecap || edit.FullDemo != nil || j.KillPlan == nil {
		return segmentIDs, false, nil
	}
	plan, err := preRolledKillPlan(store, *j, edit.PreRollSeconds)
	if err != nil {
		return nil, false, err
	}
	segmentIDs = parser.PreRollIDs(*j.KillPlan, edit.PreRollSeconds, segmentIDs)
	j.KillPlan = plan
	return segmentIDs, true, nil
}

// requirePlayWindows refuses a stored capture whose play windows differ from the plan's, as when another video of
// the job recorded them with another pre-roll; Studio then records them again.
func requirePlayWindows(result recording.RecordingResult, plan killplan.Plan) error {
	windows := make(map[string][2]int, len(plan.Segments))
	for _, s := range plan.Segments {
		windows[s.ID] = [2]int{s.TickStart, s.TickEnd}
	}
	for _, s := range result.Plan.Segments {
		if w, ok := windows[s.ID]; ok && w != [2]int{s.TickStart, s.TickEnd} {
			return recording.MarkNotReusable(fmt.Errorf("segment %s was recorded over another window", s.ID))
		}
	}
	return nil
}
