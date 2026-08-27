/** Orchestrator prefix for a demo recorded on an older CS2 build. Retry cannot help. */
export const DEMO_INCOMPATIBLE_PREFIX = 'demo_incompatible:' as const;

/** Stable prefix when CS2 crashes rewinding playdemo to demo tick 0. */
export const UNPLAYABLE_START_PREFIX = 'unplayable_start:' as const;

/** Stable prefix the orchestrator stamps when a stored capture must be re-recorded. */
export const RECORDING_NOT_REUSABLE_PREFIX = 'recording_not_reusable:' as const;

/** True when compose/render must re-record; re-rendering the stored capture cannot recover. */
export function requiresRecapture(reason: string | undefined): boolean {
  if (reason === undefined || reason.trim() === '') return false;
  if (reason.startsWith(RECORDING_NOT_REUSABLE_PREFIX)) return true;
  return (
    reason.includes('capture_mode must be "real"') ||
    reason.includes('lacks completed POV verification') ||
    reason.includes('capture input fingerprint does not match') ||
    reason.includes('legacy recording result contains fields from a newer capture contract') ||
    reason.includes('recording result publication is pending')
  );
}

/** How many planned segments were captured before an incompatible demo aborted the run. */
export type CapturedCounts = { captured: number; requested: number };

/** Parsed classification of a reel's `failureReason` string. */
export type FailureReason = {
  kind: 'demo-incompatible' | 'unplayable-start' | 'recording-not-reusable' | 'pov-verification' | 'generic';
  /** Spanish message the failed-reel card should surface to the user. */
  message: string;
  /** Whether a retry could plausibly resolve the failure. */
  retryCanHelp: boolean;
  /** Populated only for demo-incompatible failures that reported partial capture. */
  counts?: CapturedCounts;
};

export type FailureContext = {
  /** Full Demo plans are regenerated rather than replayed after a stale POV boundary. */
  fullDemo?: boolean;
};

const GENERIC_MESSAGE =
  'No se pudo completar el vídeo en este equipo. Reintenta; si vuelve a fallar, comparte el diagnóstico desde Ajustes.';

const DEMO_INCOMPATIBLE_MESSAGE =
  'Esta demo se grabó en una versión antigua de CS2 y el cliente actual no puede reproducirla. ' +
  'Reintentar no lo arreglará: usa una demo jugada después del último parche.';

/** Failure-reason substring that distinguishes a demo that stops before the end. */
const DEMO_PLAYBACK_ENDED_PHRASE = 'playback stops before every protected segment completes';

const DEMO_PLAYBACK_ENDED_MESSAGE =
  'La demo termina antes de que todas las jugadas seleccionadas puedan grabarse. ' +
  'Reintentar no lo arreglará: usa una demo que llegue al final de las jugadas seleccionadas.';

const UNPLAYABLE_START_MESSAGE =
  'Esta demo empieza a mitad y CS2 crashea al rebobinar a tick 0. ' +
  'No relances CS2: no es un fallo de POV ni de HLAE.';

const RECORDING_NOT_REUSABLE_MESSAGE =
  'La captura guardada no es reutilizable con esta versión de ClipHub. ' +
  'Reintenta: se volverá a grabar la POV con el contrato actual.';

const POV_VERIFICATION_MESSAGE =
  'ClipHub perdió el POV al terminar una ronda. La demo sigue intacta, pero este plan no es reutilizable: ' +
  'vuelve a preparar la demo para generar sus rondas con el contrato actual.';

// Matches the orchestrator's "; captured N/M segments before the failure" clause.
const CAPTURED_CLAUSE = /captured\s+(\d+)\/(\d+)\s+segments/i;

function capturedSentence(counts: CapturedCounts): string {
  return ` Se capturaron ${counts.captured} de ${counts.requested} jugadas antes del fallo y siguen disponibles.`;
}

/** Classifies `failureReason` into a Spanish card message. Pure; the card does not parse the raw string. */
export function parseFailureReason(reason: string | undefined, context: FailureContext = {}): FailureReason {
  if (reason === undefined || reason.trim() === '') {
    return { kind: 'generic', message: GENERIC_MESSAGE, retryCanHelp: true };
  }

  if (reason.startsWith(UNPLAYABLE_START_PREFIX)) {
    return { kind: 'unplayable-start', message: UNPLAYABLE_START_MESSAGE, retryCanHelp: false };
  }

  if (reason.startsWith(DEMO_INCOMPATIBLE_PREFIX)) {
    const baseMessage = reason.includes(DEMO_PLAYBACK_ENDED_PHRASE)
      ? DEMO_PLAYBACK_ENDED_MESSAGE
      : DEMO_INCOMPATIBLE_MESSAGE;
    const match = CAPTURED_CLAUSE.exec(reason);
    if (match === null) {
      return { kind: 'demo-incompatible', message: baseMessage, retryCanHelp: false };
    }

    const counts: CapturedCounts = { captured: Number(match[1]), requested: Number(match[2]) };
    return {
      kind: 'demo-incompatible',
      message: baseMessage + capturedSentence(counts),
      retryCanHelp: false,
      counts,
    };
  }

  if (requiresRecapture(reason)) {
    return { kind: 'recording-not-reusable', message: RECORDING_NOT_REUSABLE_MESSAGE, retryCanHelp: true };
  }

  if (context.fullDemo && reason.includes('observer target remained unknown during')) {
    return { kind: 'pov-verification', message: POV_VERIFICATION_MESSAGE, retryCanHelp: false };
  }

  return { kind: 'generic', message: GENERIC_MESSAGE, retryCanHelp: true };
}
