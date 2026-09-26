import { JOB_WAIT_TIMEOUT_CODE, SERVICE_UNAVAILABLE_CODE } from './api/types.ts';
import { MUTATION_CAPABILITY_ERROR } from './api/local-request-guard.ts';

export const DEMO_SERVICE_OFFLINE_HINT =
  'El servicio de análisis está offline. Arráncalo y vuelve a intentarlo.';
export const DEMO_LIST_FAIL_HINT =
  'No se pudieron cargar las demos parseadas. Suelta un .dem o recarga.';
export const DEMO_SCAN_FAIL_HINT = 'No se pudo escanear esa demo. Prueba con otro archivo .dem.';
export const DEMO_EMPTY_ROSTER_HINT =
  'El escaneo no encontró jugadores en esa demo. ¿Seguro que es una demo de CS2? Prueba con otro archivo .dem.';
export const DEMO_PARSE_FAIL_HINT =
  'No se pudieron extraer los highlights de ese jugador. Elige otro.';

/** Why a scan failed, in words the user can act on; keyed by the failure cause. */
export const DEMO_SCAN_HINTS = {
  csgo: 'Esa demo es de CS:GO. ClipHub solo lee demos de CS2.',
  notDemo:
    'Ese archivo no es una demo de CS2. Si lo renombraste a .dem o la descarga no terminó, vuelve a descargar la demo.',
  unreadable: 'No se pudo descomprimir la demo. Vuelve a descargarla y cárgala de nuevo.',
  incompatible:
    'ClipHub no pudo leer esa demo: está dañada o es de una versión de CS2 más nueva que esta versión de ClipHub. Actualiza ClipHub o vuelve a descargar la demo.',
  tooLarge: 'La demo supera el límite de 700 MB.',
  timeout: 'El escaneo de la demo tardó demasiado. Vuelve a intentarlo.',
  session: 'La sesión de ClipHub caducó. Recarga la ventana y vuelve a cargar la demo.',
} as const;

/** Stable codes the orchestrator attaches to a rejected upload or a failed scan job. */
const SCAN_FAILURE_HINTS: Readonly<Record<string, string>> = {
  csgo_demo: DEMO_SCAN_HINTS.csgo,
  not_a_demo: DEMO_SCAN_HINTS.notDemo,
  unreadable_demo: DEMO_SCAN_HINTS.unreadable,
  demo_incompatible: DEMO_SCAN_HINTS.incompatible,
  payload_too_large: DEMO_SCAN_HINTS.tooLarge,
  [JOB_WAIT_TIMEOUT_CODE]: DEMO_SCAN_HINTS.timeout,
};

function errorField(err: unknown, key: 'code' | 'status' | 'message'): unknown {
  if (typeof err !== 'object' || err === null || !(key in err)) return undefined;
  return (err as Record<typeof key, unknown>)[key];
}

export function isDemoServiceUnavailable(err: unknown): boolean {
  return errorField(err, 'code') === SERVICE_UNAVAILABLE_CODE;
}

export function demoListLoadError(err: unknown): string {
  return isDemoServiceUnavailable(err) ? DEMO_SERVICE_OFFLINE_HINT : DEMO_LIST_FAIL_HINT;
}

/**
 * The scan error the user sees. Every cause used to collapse into
 * DEMO_SCAN_FAIL_HINT ("prueba con otro .dem"), which is wrong advice for a
 * CS:GO demo, a demo newer than the parser, or an expired session.
 */
export function demoScanError(err: unknown): string {
  if (isDemoServiceUnavailable(err)) return DEMO_SERVICE_OFFLINE_HINT;
  const code = errorField(err, 'code');
  if (typeof code === 'string' && Object.hasOwn(SCAN_FAILURE_HINTS, code)) return SCAN_FAILURE_HINTS[code] as string;
  const status = errorField(err, 'status');
  if (status === 413) return DEMO_SCAN_HINTS.tooLarge;
  if (status === 403 && errorField(err, 'message') === MUTATION_CAPABILITY_ERROR) return DEMO_SCAN_HINTS.session;
  return DEMO_SCAN_FAIL_HINT;
}

export function demoParseError(err: unknown): string {
  return isDemoServiceUnavailable(err) ? DEMO_SERVICE_OFFLINE_HINT : DEMO_PARSE_FAIL_HINT;
}
