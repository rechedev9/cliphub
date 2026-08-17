/**
 * Machine-readable prefix the orchestrator stamps on a job whose demo cannot be
 * replayed because it was recorded on an older CS2 build. This failure is
 * deterministic in the `.dem` file itself, so retrying can never help; the
 * reason may end with a "; captured N/M segments before the failure" clause.
 */
export const DEMO_INCOMPATIBLE_PREFIX = 'demo_incompatible:' as const;

/** Stable prefix when CS2 crashes rewinding playdemo to demo tick 0. */
export const UNPLAYABLE_START_PREFIX = 'unplayable_start:' as const;

/** Stable prefix the orchestrator stamps when a stored capture must be re-recorded. */
export const RECORDING_NOT_REUSABLE_PREFIX = 'recording_not_reusable:' as const;

/**
 * True when a failure reason means the durable recording result is not safe to
 * compose/render under the current capture contract. The only recovery is to
 * re-record; re-rendering loops forever on the same stale result.
 *
 * Matches the orchestrator's `recording_not_reusable:` prefix and the English
 * validation strings that 2.4.6 already stamped on failed reels before the
 * prefix existed.
 */
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
  kind: 'demo-incompatible' | 'unplayable-start' | 'recording-not-reusable' | 'generic';
  /** Spanish message the failed-reel card should surface to the user. */
  message: string;
  /** Whether a retry could plausibly resolve the failure. */
  retryCanHelp: boolean;
  /** Populated only for demo-incompatible failures that reported partial capture. */
  counts?: CapturedCounts;
};

const GENERIC_MESSAGE = 'El reel falló en tu equipo.';

const DEMO_INCOMPATIBLE_MESSAGE =
  'Esta demo se grabó en una versión antigua de CS2 y el cliente actual no puede reproducirla. ' +
  'Reintentar no lo arreglará: usa una demo jugada después del último parche.';

const UNPLAYABLE_START_MESSAGE =
  'Esta demo empieza a mitad y CS2 crashea al rebobinar a tick 0. ' +
  'No relances CS2: no es un fallo de POV ni de HLAE.';

const RECORDING_NOT_REUSABLE_MESSAGE =
  'La captura guardada no es reutilizable con esta versión de TickCut. ' +
  'Reintenta: se volverá a grabar la POV con el contrato actual.';

// Matches the orchestrator's "; captured N/M segments before the failure" clause.
const CAPTURED_CLAUSE = /captured\s+(\d+)\/(\d+)\s+segments/i;

function capturedSentence(counts: CapturedCounts): string {
  return ` Se capturaron ${counts.captured} de ${counts.requested} jugadas antes del fallo y siguen disponibles.`;
}

/**
 * Classifies a reel's raw `failureReason`. A reason beginning with
 * `demo_incompatible:` is a deterministic, non-retryable demo-build mismatch and
 * gets a Spanish explanation (plus a captured-counts sentence when the
 * orchestrator reported partial capture). A non-reusable capture (stale
 * recording result after a contract upgrade) is retryable via re-record.
 * Everything else stays generic and retryable. Pure so the card component never
 * branches on the raw string.
 */
export function parseFailureReason(reason: string | undefined): FailureReason {
  if (reason === undefined || reason.trim() === '') {
    return { kind: 'generic', message: GENERIC_MESSAGE, retryCanHelp: true };
  }

  if (reason.startsWith(UNPLAYABLE_START_PREFIX)) {
    return { kind: 'unplayable-start', message: UNPLAYABLE_START_MESSAGE, retryCanHelp: false };
  }

  if (reason.startsWith(DEMO_INCOMPATIBLE_PREFIX)) {
    const match = CAPTURED_CLAUSE.exec(reason);
    if (match === null) {
      return { kind: 'demo-incompatible', message: DEMO_INCOMPATIBLE_MESSAGE, retryCanHelp: false };
    }

    const counts: CapturedCounts = { captured: Number(match[1]), requested: Number(match[2]) };
    return {
      kind: 'demo-incompatible',
      message: DEMO_INCOMPATIBLE_MESSAGE + capturedSentence(counts),
      retryCanHelp: false,
      counts,
    };
  }

  if (requiresRecapture(reason)) {
    return { kind: 'recording-not-reusable', message: RECORDING_NOT_REUSABLE_MESSAGE, retryCanHelp: true };
  }

  return { kind: 'generic', message: reason, retryCanHelp: true };
}
