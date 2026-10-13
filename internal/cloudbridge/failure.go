package cloudbridge

import (
	"github.com/rechedev9/cliphub/internal/obs"
)

// Failure codes the worker reports. The portal owns the policy (retry, fail,
// pause the worker); the worker only names what happened.
const (
	codeCaptureFlake        = "capture_flake"
	codeInterrupted         = "interrupted"
	codeDemoDownloadFailed  = "demo_download_failed"
	codeInternal            = "internal"
	codeDemoIncompatible    = "demo_incompatible"
	codeUnplayableStart     = "unplayable_start"
	codeTargetNotFound      = "target_not_found"
	codeSpecMismatch        = "spec_mismatch"
	codeInvalidSpec         = "invalid_spec"
	codeRenderFailed        = "render_failed"
	codeJobFailed           = "job_failed"
	codeTimeout             = "timeout"
	codeUploadFailed        = "upload_failed"
	codeCaptureIncompatible = "capture_incompatible"
	codeCanceled            = "canceled"
)

// failureMessages is the one sentence a user reads for each code. It is
// fixed text on purpose: the raw cause can carry paths and tool output, and
// travels only in the operator-facing detail.
var failureMessages = map[string]string{
	codeCaptureFlake:        "La captura falló por un problema puntual del juego. La repetimos automáticamente.",
	codeInterrupted:         "El equipo de la nube se reinició durante tu trabajo. Lo repetimos automáticamente.",
	"worker_lost":           "Perdimos la conexión con el equipo de la nube. Repetimos tu trabajo automáticamente.",
	codeDemoDownloadFailed:  "El equipo de la nube no pudo descargar la demo. Lo intentamos de nuevo automáticamente.",
	codeInternal:            "Hubo un error interno en la nube. Repetimos tu trabajo automáticamente.",
	codeDemoIncompatible:    "Esta demo es de una versión de CS2 que ya no se puede reproducir.",
	codeUnplayableStart:     "CS2 no puede reproducir esta demo desde el principio.",
	codeTargetNotFound:      "El jugador elegido no aparece en esta demo.",
	codeSpecMismatch:        "Las jugadas elegidas no coinciden con la demo. Vuelve a analizarla en Studio y crea el vídeo otra vez.",
	codeInvalidSpec:         "La configuración del vídeo no es válida. Actualiza ClipHub Studio y vuelve a intentarlo.",
	codeRenderFailed:        "La captura salió bien, pero falló el montaje del vídeo.",
	codeJobFailed:           "No pudimos completar el vídeo en la nube.",
	codeTimeout:             "El trabajo tardó más de lo permitido y lo detuvimos.",
	codeUploadFailed:        "El vídeo se generó, pero no pudimos subirlo.",
	codeCaptureIncompatible: "CS2 se actualizó y la captura en la nube está en pausa. Tu trabajo sigue en la cola.",
	"steam_unavailable":     "Steam no está disponible en el equipo de la nube. Tu trabajo sigue en la cola.",
	"disk_full":             "El equipo de la nube se quedó sin espacio. Tu trabajo sigue en la cola.",
	"tools_missing":         "Al equipo de la nube le faltan herramientas de captura. Tu trabajo sigue en la cola.",
	codeCanceled:            "Trabajo cancelado.",
}

// jobFailure is the end of one attempt: a code for the portal's policy and
// the untouched cause for the operator.
type jobFailure struct {
	Code   string
	Detail string
}

func failureOf(code, detail string) *jobFailure {
	return &jobFailure{Code: code, Detail: detail}
}

func (f jobFailure) request(machineSeconds int) failRequest {
	message, known := failureMessages[f.Code]
	if !known {
		message = failureMessages[codeJobFailed]
	}
	return failRequest{Code: f.Code, Message: message, Detail: f.Detail, MachineSeconds: max(machineSeconds, 0)}
}

// localFailureCodes are the local failure classes with a cloud code of the
// same name; every other local failure is a plain job_failed.
var localFailureCodes = map[string]string{
	obs.ClassCaptureIncompatible: codeCaptureIncompatible,
	obs.ClassCaptureFlake:        codeCaptureFlake,
	obs.ClassInterrupted:         codeInterrupted,
	obs.ClassDemoIncompatible:    codeDemoIncompatible,
	obs.ClassUnplayableStart:     codeUnplayableStart,
	obs.ClassTargetNotFound:      codeTargetNotFound,
}

// localJobFailure maps a failed local job onto a cloud failure. The detail is
// always the local failure reason, untouched.
func localJobFailure(job LocalJob) *jobFailure {
	class := job.FailureCode
	if class == "" {
		class = obs.ClassOf(job.FailureReason)
	}
	code, mapped := localFailureCodes[class]
	if !mapped {
		code = codeJobFailed
	}
	return failureOf(code, job.FailureReason)
}

// renderFailure maps a failed render variant: the capture was fine, so
// repeating the job would only burn queue time.
func renderFailure(variant LocalVariant) *jobFailure {
	return failureOf(codeRenderFailed, variant.Error)
}
