package telemetryalert

import "strings"

// Hint is a known-issue lesson, keyed by failure_code or by a lowercase
// substring of the filtered message. Only the static text is sent; the
// message itself is used for matching on the VPS.
type Hint struct {
	match   []string
	text    string
	section string // AGENTS.md section holding the full lesson
}

func (h Hint) line() string {
	if h.section == "" {
		return "pista: " + h.text
	}
	return "pista: " + h.text + " · ver AGENTS.md › " + h.section
}

const (
	sectionAudio   = "Full Demo audio mastering"
	sectionHLAE    = "HLAE pin and CS2 updates"
	sectionBlack   = "Incident: black first-person capture with the interim pin"
	sectionOverlay = "Full Demo overlay format"
)

var hints = []Hint{
	{[]string{"capture_incompatible", "hlae_hook_incompatible", "afxhooksource2", "hlae hook crashed"},
		"exit 6 = HLAE frente a la build de CS2: revisa issues/releases de advancedfx y la hora del update de CS2 antes de bisecar ClipHub", sectionHLAE},
	{[]string{"loudnorm_param_out_of_range", "for parameter 'tp' out of range"},
		"bucle de retarget del master, clamp TP [-9,0] y cede a recoverFullDemoAAC", sectionAudio},
	{[]string{"audio_master_exhausted", "audio_loudness_failed", "after three masters"},
		"el master AAC agotó sus intentos: comprueba que el retarget llega a la recuperación AAC (aac_mf)", sectionAudio},
	{[]string{"aac_mf", "media foundation"},
		"Media Foundation (aac_mf) es del sistema: solo falta en Windows N/KN; no se puede empaquetar", sectionAudio},
	{[]string{"1073807364", "shutdown_kill"},
		"proceso cerrado al apagar o cerrar sesión en Windows (0x40010004), no es un crash", ""},
	{[]string{"capture_pov_unverified", "pov verification failed"},
		"la verificación del POV falló: revisa frames con blackdetect/signalstats y el pin de HLAE", sectionHLAE},
	{[]string{"cs2_already_running", "cs2.exe is already running"},
		"CS2 ya estaba abierto al empezar la captura: ciérralo y reintenta; no es una regresión", ""},
}

var qualityHint = Hint{nil,
	"fallo de captura dependiente de la máquina: revisa la procedencia del pin de HLAE y verifica frames con blackdetect/signalstats", sectionBlack}

var reportHints = map[string]Hint{
	"black_video":   qualityHint,
	"wrong_overlay": {nil, "el origen de la demo (source_kind) manda en el layout; compara render.profile con el último render correcto", sectionOverlay},
	"audio":         {nil, "revisa los breadcrumbs stage.entered de audio_master (target_tp) del job", sectionAudio},
}

// lookupHint matches the failure code exactly first, then message substrings.
func lookupHint(code, message string) (Hint, bool) {
	lower := strings.ToLower(message)
	for _, byCode := range []bool{true, false} {
		for _, h := range hints {
			for _, needle := range h.match {
				if byCode && code != "" && needle == code || !byCode && strings.Contains(lower, needle) {
					return h, true
				}
			}
		}
	}
	return Hint{}, false
}
