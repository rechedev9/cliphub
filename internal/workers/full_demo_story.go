package workers

import (
	"github.com/rechedev9/cliphub/internal/demooverlay"
	"github.com/rechedev9/cliphub/internal/job"
	"github.com/rechedev9/cliphub/internal/renderplan"
)

const fullDemoStoryOverlayPolicy = "story-overlays-v1"

func fullDemoHalftime(j job.Job, edit renderplan.EditRequest, doc demooverlay.Document) *demooverlay.Halftime {
	if edit.FullDemo == nil || j.KillPlan == nil || !edit.FullDemo.Document.Options.Overlays.Halftime {
		return nil
	}
	return demooverlay.BuildHalftime(doc, j.KillPlan.Rounds, edit.FullDemo.Document.Options.Overlays.TeamNames)
}
