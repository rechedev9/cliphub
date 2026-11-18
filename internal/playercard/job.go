package playercard

import "github.com/rechedev9/cliphub/internal/job"

// Target is the player a one-player Short of the job follows.
func Target(j job.Job) string {
	if j.KillPlan != nil && j.KillPlan.Target.SteamID64 != "" {
		return j.KillPlan.Target.SteamID64
	}
	return j.TargetSteamID
}

// Unavailable is why the job's Shorts cannot carry player cards, in the UI's words; empty when they can.
func Unavailable(j job.Job) string {
	switch {
	case j.IsMultiPOV():
		return "Las cartas de jugador son para un Short de un solo jugador, y este sigue a varios."
	case !validSteamID(Target(j)):
		return "Elige el jugador del Short para usar su carta."
	}
	return ""
}
