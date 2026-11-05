package httpapi

import (
	"errors"
	"net/http"

	"github.com/rechedev9/cliphub/internal/practice"
)

const (
	codeInvalidPractice     = "invalid_practice"
	codePracticeUnavailable = "practice_unavailable"
	codeCS2NotFound         = "cs2_not_found"
)

// WithPractice sets the CS2 install "Practicar en CS2" writes to and launches.
func WithPractice(game practice.Game) Option {
	return func(h *Handlers) {
		h.practiceGame = game
	}
}

type practiceSpotView struct {
	Label   string     `json:"label"`
	Alias   string     `json:"alias"`
	Key     string     `json:"key,omitempty"`
	Pos     [3]float64 `json:"pos"`
	Ang     [2]float64 `json:"ang"`
	Grenade string     `json:"grenade,omitempty"`
}

type practiceLaunchResponse struct {
	Launched       bool               `json:"launched"`
	AlreadyRunning bool               `json:"already_running"`
	CfgPath        string             `json:"cfg_path"`
	ExecCommand    string             `json:"exec_command"`
	NextKey        string             `json:"next_key"`
	Spots          []practiceSpotView `json:"spots"`
}

// LaunchPractice writes the practice cfg and opens CS2 on it. When CS2 is
// already open it only rewrites the cfg; the UI then asks the player to run
// exec_command in the console.
func (h *Handlers) LaunchPractice(w http.ResponseWriter, r *http.Request) {
	if h.practiceGame == nil {
		writeCodedError(w, http.StatusServiceUnavailable, codePracticeUnavailable, "practice is not available")
		return
	}
	var in practice.Input
	if err := decodeSingleJSONBody(w, r, &in, true); err != nil {
		writeCodedError(w, http.StatusBadRequest, codeInvalidPractice, "invalid JSON body")
		return
	}
	plan, err := practice.Validate(in)
	if err != nil {
		writeCodedError(w, http.StatusBadRequest, codeInvalidPractice, err.Error())
		return
	}
	result, err := practice.Start(h.practiceGame, plan)
	if errors.Is(err, practice.ErrCS2NotFound) {
		writeCodedError(w, http.StatusServiceUnavailable, codeCS2NotFound, "CS2 is not installed")
		return
	}
	if err != nil {
		internalError(w, "launch practice", err)
		return
	}
	spots := make([]practiceSpotView, len(plan.Spots))
	for i, spot := range plan.Spots {
		spots[i] = practiceSpotView{
			Label:   spot.Label,
			Alias:   practice.SpotAlias(i + 1),
			Key:     practice.SpotKey(i + 1),
			Pos:     spot.Pos,
			Ang:     spot.Ang,
			Grenade: spot.Grenade,
		}
	}
	writeJSON(w, http.StatusOK, practiceLaunchResponse{
		Launched:       result.Launched,
		AlreadyRunning: result.AlreadyRunning,
		CfgPath:        result.CfgPath,
		ExecCommand:    practice.ExecCommand,
		NextKey:        practice.NextKey,
		Spots:          spots,
	})
}
