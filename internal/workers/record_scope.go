package workers

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/hibiken/asynq"

	"github.com/rechedev9/cliphub/internal/job"
	"github.com/rechedev9/cliphub/internal/killplan"
	"github.com/rechedev9/cliphub/internal/recapplan"
	"github.com/rechedev9/cliphub/internal/recording"
	"github.com/rechedev9/cliphub/internal/renderplan"
	"github.com/rechedev9/cliphub/internal/tasks"
)

// recordRequest is the capture a record task asks for. victimReplay comes from
// the generate intent; approval is the Full Demo snapshot, if any.
type recordRequest struct {
	hudMode              string
	segmentIDs           []string
	portraitSafeKillfeed bool
	source               planSource
	victimReplay         bool
	approval             *recapplan.Snapshot
	preRollSeconds       int
}

func recordRequestFor(payload tasks.RecordDemoPayload, intent renderplan.GenerateIntent) recordRequest {
	source := planSourceKills
	if payload.UseRecapPlan {
		source = planSourceRecap
	} else if payload.UseDeathPlan {
		source = planSourceDeaths
	}
	return recordRequest{
		hudMode:              payload.HUDMode,
		segmentIDs:           payload.SegmentIDs,
		portraitSafeKillfeed: payload.PortraitSafeKillfeed,
		source:               source,
		victimReplay:         intent.Edit.VictimReplay,
		approval:             intent.Edit.FullDemo,
		preRollSeconds:       payload.PreRollSeconds,
	}
}

// recordScope is what a record task captures and which of its segments still lack a reusable clip.
type recordScope struct {
	job            job.Job
	req            recordRequest
	sourcePlan     *killplan.Plan
	fullDemo       *recapplan.Document
	requested      []string
	cfg            RecordWorkerConfig
	captureEncoder string
	expectedStream recording.StreamConfig
	missing        []string
	reuse          recordingReuse
}

// CaptureNeeded reports whether record task t would open CS2, or reuse every stored
// clip and only chain its render. It decides as record does and never runs the recorder.
func (w *RecordWorker) CaptureNeeded(ctx context.Context, t *asynq.Task) (bool, error) {
	var payload tasks.RecordDemoPayload
	if err := json.Unmarshal(t.Payload(), &payload); err != nil {
		return false, fmt.Errorf("decode payload: %w", err)
	}
	intent, _, err := tasks.GenerateIntentFromTask(t)
	if err != nil {
		return false, fmt.Errorf("decode record task generate intent: %w", err)
	}
	j, err := w.repo.Get(ctx, payload.JobID)
	if err != nil {
		return false, fmt.Errorf("load job %s: %w", payload.JobID, err)
	}
	if j.KillPlan == nil {
		return false, fmt.Errorf("job %s has no kill plan", j.ID)
	}
	scope := recordScope{job: j, req: recordRequestFor(payload, intent)}
	if err := w.resolveSource(ctx, &scope); err != nil {
		return false, err
	}
	if err := w.resolveMissing(ctx, &scope); err != nil {
		return false, err
	}
	return len(scope.missing) > 0, nil
}

// resolveSource loads the plan s captures from; a Full Demo approval is checked against the stored demo.
func (w *RecordWorker) resolveSource(ctx context.Context, s *recordScope) error {
	j, req := s.job, s.req
	if req.approval == nil {
		plan, segmentIDs, err := recordSourcePlan(w.storage, j, req)
		s.sourcePlan, s.requested = plan, segmentIDs
		return err
	}
	snapshot, err := recapplan.ResolveApproval(ctx, w.storage, j.ID, j.DemoPath, j.TargetSteamID, recording.FindFFmpeg(), *req.approval)
	if err != nil {
		return err
	}
	adapted := snapshot.Document.KillPlan(*j.KillPlan)
	s.sourcePlan, s.fullDemo, s.requested = &adapted, &snapshot.Document, req.segmentIDs
	return nil
}

// resolveMissing builds the capture identity of s and lists the requested segments no stored clip covers.
func (w *RecordWorker) resolveMissing(ctx context.Context, s *recordScope) error {
	// An empty selection means the whole source plan (the CLI all-kills default, or every recap round).
	if len(s.requested) == 0 {
		s.requested = killPlanSegmentIDs(s.sourcePlan)
	}
	// requestedPlan scopes only the identity; the recorder gets the missing subset.
	requestedPlan, err := filterKillPlanSegments(s.sourcePlan, s.requested)
	if err != nil {
		return err
	}
	s.cfg = w.cfg.withDefaults()
	if s.req.hudMode != "" {
		s.cfg.HUDMode = s.req.hudMode
	}
	// The encoder is part of the recording identity: it must reach both the recorder
	// argv and the expected plan, or attempt validation rejects the echoed plan.
	s.captureEncoder = studioCaptureEncoder(s.cfg.HLAEPath, s.cfg.FFmpegPath)
	s.expectedStream, err = normalizedRecordingStream(requestedPlan, recording.StreamConfig{
		HUDMode:              recording.HUDMode(s.cfg.HUDMode),
		PortraitSafeKillfeed: s.req.portraitSafeKillfeed,
		Encoder:              s.captureEncoder,
		VictimReplay:         s.req.victimReplay && s.fullDemo == nil && s.req.source == planSourceKills,
	})
	if err != nil {
		return fmt.Errorf("build recording profile: %w", err)
	}
	expectedProfile, err := recording.NewPlanFromKillPlan(*requestedPlan, "profile.dem", "profile", s.expectedStream, s.fullDemo)
	if err != nil {
		return fmt.Errorf("build expected recording identity: %w", err)
	}
	s.missing, s.reuse, err = recordingOutputsReady(ctx, w.storage, s.job.ID, s.requested, expectedProfile)
	return err
}
