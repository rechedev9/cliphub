import { SERVICE_UNAVAILABLE_CODE } from './api/types.ts';

export const DEMO_SERVICE_OFFLINE_HINT =
  'El servicio de análisis está offline. Arráncalo y vuelve a intentarlo.';
export const DEMO_LIST_FAIL_HINT =
  'No se pudieron cargar las demos parseadas. Suelta un .dem o recarga.';
export const DEMO_SCAN_FAIL_HINT = 'No se pudo escanear esa demo. Prueba con otro archivo .dem.';
export const DEMO_EMPTY_ROSTER_HINT =
  'El escaneo no encontró jugadores en esa demo. ¿Seguro que es una demo de CS2? Prueba con otro archivo .dem.';
export const DEMO_PARSE_FAIL_HINT =
  'No se pudieron extraer los highlights de ese jugador. Elige otro.';
export const DEMO_SINGLE_FILE_HINT = 'Esta sección forja una partida. Suelta un solo .dem.';

export function isDemoServiceUnavailable(err: unknown): boolean {
  if (typeof err !== 'object' || err === null || !('code' in err)) return false;
  return err.code === SERVICE_UNAVAILABLE_CODE;
}

export function demoListLoadError(err: unknown): string {
  return isDemoServiceUnavailable(err) ? DEMO_SERVICE_OFFLINE_HINT : DEMO_LIST_FAIL_HINT;
}

export function demoScanError(err: unknown): string {
  return isDemoServiceUnavailable(err) ? DEMO_SERVICE_OFFLINE_HINT : DEMO_SCAN_FAIL_HINT;
}

export function demoParseError(err: unknown): string {
  return isDemoServiceUnavailable(err) ? DEMO_SERVICE_OFFLINE_HINT : DEMO_PARSE_FAIL_HINT;
}
