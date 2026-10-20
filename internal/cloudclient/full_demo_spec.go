package cloudclient

import (
	"encoding/json"
	"net/http"

	"github.com/rechedev9/cliphub/internal/job"
	"github.com/rechedev9/cliphub/internal/recapplan"
)

const (
	kindShort    = "short"
	kindFullDemo = "full_demo"

	fullDemoPreset = "gameplay-pov-60"
)

// specFullDemo carries what the worker needs to rebuild the approved plan:
// its options, and the portable hash the rebuilt plan must reproduce.
type specFullDemo struct {
	Options  json.RawMessage `json:"options"`
	PlanHash string          `json:"planHash"`
}

func invalidFullDemo(message string) *Error {
	return &Error{Status: http.StatusBadRequest, Code: "invalid_request", Message: message}
}

// buildFullDemoSpec turns the approved snapshot of a local generate request into a
// cloud spec; the snapshot stays here, and the worker re-plans to the same hash.
func buildFullDemoSpec(local job.Job, in SubmitInput, studioVersion string) (jobSpec, error) {
	var edit map[string]json.RawMessage
	if err := json.Unmarshal(in.Edit, &edit); err != nil || edit == nil {
		return jobSpec{}, invalidFullDemo("edit debe llevar el plan aprobado del vídeo largo.")
	}
	var snapshot recapplan.Snapshot
	if err := json.Unmarshal(edit["full_demo"], &snapshot); err != nil {
		return jobSpec{}, invalidFullDemo("El plan aprobado del vídeo largo no es válido.")
	}
	if err := snapshot.Validate(); err != nil {
		return jobSpec{}, invalidFullDemo("El plan aprobado del vídeo largo no es válido: " + err.Error())
	}
	document := snapshot.Document
	if document.Input.DemoSHA256 != local.KillPlan.Demo.SHA256 || document.Input.TargetSteamID64 != local.TargetSteamID {
		return jobSpec{}, &Error{Status: http.StatusConflict, Code: "plan_stale", Message: "El plan es de otra demo o de otro jugador. Prepáralo de nuevo."}
	}
	if len(document.Assets) > 0 {
		return jobSpec{}, &Error{Status: http.StatusConflict, Code: "cloud_assets_unsupported", Message: "La nube todavía no admite vídeos o capturas propias en el vídeo largo."}
	}
	options, err := json.Marshal(document.Options)
	if err != nil {
		return jobSpec{}, err
	}
	planHash, err := document.PortableHash()
	if err != nil {
		return jobSpec{}, err
	}
	delete(edit, "full_demo")
	strippedEdit, err := json.Marshal(edit)
	if err != nil {
		return jobSpec{}, err
	}

	var spec jobSpec
	for _, round := range document.Rounds {
		spec.Capture.Windows = append(spec.Capture.Windows, specWindow{ID: round.ID, TickStart: round.CaptureStartTick, TickEnd: round.CaptureEndTick})
	}
	if len(spec.Capture.Windows) == 0 {
		return jobSpec{}, invalidFullDemo("El plan del vídeo largo no tiene rondas que grabar.")
	}
	spec.Version = 1
	spec.TargetSteamID = local.TargetSteamID
	spec.Rules = local.Rules
	spec.Capture.Tickrate = local.KillPlan.Demo.Tickrate
	spec.Generate = specGenerate{Preset: fullDemoPreset, Music: json.RawMessage("null"), SegmentIDs: []string{}, Edit: strippedEdit}
	spec.FullDemo = &specFullDemo{Options: options, PlanHash: planHash}
	spec.Client.StudioVersion = studioVersion
	spec.Client.PlanSchema = local.KillPlan.SchemaVersion
	return spec, nil
}
