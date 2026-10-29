package workers

import (
	"log"
	"strconv"
	"strings"

	"github.com/google/uuid"
	demoinfocs "github.com/markus-wa/demoinfocs-golang/v5/pkg/demoinfocs"

	"github.com/rechedev9/cliphub/internal/job"
	"github.com/rechedev9/cliphub/internal/parser"
)

func logWorkerTransition(id uuid.UUID, taskType string, status job.Status) {
	log.Printf("worker job=%s task=%s transition=%s", id, taskType, status)
}

func logWorkerArtifacts(id uuid.UUID, taskType string, keys []string) {
	if len(keys) == 0 {
		return
	}
	log.Printf("worker job=%s task=%s artifact_keys=%s", id, taskType, strings.Join(keys, ","))
}

func logWorkerError(id uuid.UUID, op string, err error) {
	log.Printf("worker job=%s op=%s error=%v", id, op, err)
}

func logWorkerSkip(id uuid.UUID, taskType string, keys []string) {
	if len(keys) == 0 {
		log.Printf("worker job=%s task=%s skip=artifacts_ready", id, taskType)
		return
	}
	log.Printf("worker job=%s task=%s skip=artifacts_ready artifact_keys=%s", id, taskType, strings.Join(keys, ","))
}

// logDemoFacts names the map and CS2 build behind a job, success or failure,
// so a playback failure reported through telemetry can be matched to its demo.
func logDemoFacts(id uuid.UUID, taskType string, p demoinfocs.Parser, header *parser.DemoHeaderWatch, first *parser.FirstPacketWatch) {
	snapshot := "none"
	if tick, seen := first.Snapshot(); seen {
		snapshot = strconv.Itoa(tick)
	}
	log.Printf("worker job=%s task=%s demo %s tickrate=%d first_snapshot_tick=%s", id, taskType, header.Summary(), int(p.TickRate()), snapshot)
}
