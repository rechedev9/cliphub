package streamclips

// User-facing acquire failure_reason strings. The web UI renders these
// verbatim; they are not Go errors. Keep them in sync with
// workers.friendlyAcquireReason.
const (
	AcquireReasonNotFound     = "No encontramos un vídeo en esa URL (puede que el clip se haya borrado)."
	AcquireReasonAuthRequired = "Ese vídeo necesita inicio de sesión o suscripción; no podemos descargarlo."
	AcquireReasonUnavailable  = "Ese vídeo no está disponible ahora mismo (privado, caducado o restringido por región)."
	AcquireReasonBlocked      = "El origen bloqueó la descarga (protección anti-bots). Espera un momento y vuelve a intentarlo."
	AcquireReasonTooLarge     = "Ese vídeo supera el límite máximo de descarga permitido."
	AcquireReasonError        = "No pudimos preparar un vídeo a partir de esa URL. Asegúrate de que es un clip o VOD público de Twitch, YouTube o Kick."
)

const (
	AcquireCodeNotFound     = "not_found"
	AcquireCodeAuthRequired = "auth_required"
	AcquireCodeUnavailable  = "unavailable"
	AcquireCodeBlocked      = "blocked"
	AcquireCodeTooLarge     = "too_large"
	AcquireCodeError        = "error"
)

// CodeFromReason maps a persisted stream failure_reason onto the same class
// the acquire worker writes to the obs journal. Spanish display text is
// otherwise invisible to ClassOf.
func CodeFromReason(reason string) string {
	switch reason {
	case AcquireReasonNotFound:
		return AcquireCodeNotFound
	case AcquireReasonAuthRequired:
		return AcquireCodeAuthRequired
	case AcquireReasonUnavailable:
		return AcquireCodeUnavailable
	case AcquireReasonBlocked:
		return AcquireCodeBlocked
	case AcquireReasonTooLarge:
		return AcquireCodeTooLarge
	case AcquireReasonError:
		return AcquireCodeError
	default:
		return ""
	}
}
