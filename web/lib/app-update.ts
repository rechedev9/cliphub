export const APP_UPDATE_STATE = {
  unavailable: 'unavailable',
  idle: 'idle',
  checking: 'checking',
  current: 'current',
  available: 'available',
  downloading: 'downloading',
  ready: 'ready',
  installing: 'installing',
  error: 'error',
} as const;

export type AppUpdateStatus =
  | { state: typeof APP_UPDATE_STATE.unavailable }
  | { state: typeof APP_UPDATE_STATE.idle }
  | { state: typeof APP_UPDATE_STATE.checking }
  | { state: typeof APP_UPDATE_STATE.current; version: string }
  | { state: typeof APP_UPDATE_STATE.available; version: string; currentVersion: string }
  | {
    state: typeof APP_UPDATE_STATE.downloading;
    version: string;
    received: number;
    total: number | null;
  }
  | { state: typeof APP_UPDATE_STATE.ready; version: string }
  | { state: typeof APP_UPDATE_STATE.installing; version: string }
  | { state: typeof APP_UPDATE_STATE.error; message: string };

const VERSION_PATTERN = /^\d+\.\d+\.\d+$/;

export function parseAppUpdateStatus(value: unknown): AppUpdateStatus | null {
  if (!isRecord(value) || typeof value.state !== 'string') return null;
  switch (value.state) {
    case APP_UPDATE_STATE.unavailable:
    case APP_UPDATE_STATE.idle:
    case APP_UPDATE_STATE.checking:
      return exactState(value, value.state);
    case APP_UPDATE_STATE.current:
    case APP_UPDATE_STATE.ready:
    case APP_UPDATE_STATE.installing:
      return versionState(value, value.state);
    case APP_UPDATE_STATE.available:
      return availableState(value);
    case APP_UPDATE_STATE.downloading:
      return downloadingState(value);
    case APP_UPDATE_STATE.error:
      return errorState(value);
    default:
      return null;
  }
}

export function appUpdateVisible(status: AppUpdateStatus): boolean {
  return (
    status.state === APP_UPDATE_STATE.available
    || status.state === APP_UPDATE_STATE.downloading
    || status.state === APP_UPDATE_STATE.ready
    || status.state === APP_UPDATE_STATE.installing
    || status.state === APP_UPDATE_STATE.checking
    || status.state === APP_UPDATE_STATE.error
  );
}

export function appUpdatePercent(status: AppUpdateStatus): number | undefined {
  if (status.state !== APP_UPDATE_STATE.downloading) return undefined;
  if (status.total === null || status.total <= 0) return undefined;
  return Math.min(100, Math.max(0, Math.round((status.received / status.total) * 100)));
}

export function appUpdateLabel(status: AppUpdateStatus): string | null {
  switch (status.state) {
    case APP_UPDATE_STATE.available:
      return 'Actualizar';
    case APP_UPDATE_STATE.downloading: {
      const percent = appUpdatePercent(status);
      return percent === undefined ? 'Descargando' : `${percent}%`;
    }
    case APP_UPDATE_STATE.ready:
      return 'Reiniciar';
    case APP_UPDATE_STATE.installing:
      return 'Instalando';
    case APP_UPDATE_STATE.checking:
      return 'Buscando';
    case APP_UPDATE_STATE.error:
      return 'Reintentar';
    default:
      return null;
  }
}

export function appUpdateTitle(status: AppUpdateStatus, jobsBusy: boolean): string {
  if (status.state === APP_UPDATE_STATE.ready && jobsBusy) {
    return 'Espera a que terminen captura y edición para instalar';
  }
  switch (status.state) {
    case APP_UPDATE_STATE.available:
      return `Actualizar a ${status.version}`;
    case APP_UPDATE_STATE.downloading:
      return `Descargando ${status.version}`;
    case APP_UPDATE_STATE.ready:
      return `Reiniciar e instalar ${status.version}`;
    case APP_UPDATE_STATE.installing:
      return `Instalando ${status.version}`;
    case APP_UPDATE_STATE.checking:
      return 'Buscando actualizaciones';
    case APP_UPDATE_STATE.error:
      return status.message;
    default:
      return '';
  }
}

export function appUpdateAction(status: AppUpdateStatus): 'check' | 'install' | null {
  if (status.state === APP_UPDATE_STATE.error) return 'check';
  if (
    status.state === APP_UPDATE_STATE.available
    || status.state === APP_UPDATE_STATE.ready
  ) {
    return 'install';
  }
  return null;
}

function exactState<T extends typeof APP_UPDATE_STATE.unavailable | typeof APP_UPDATE_STATE.idle | typeof APP_UPDATE_STATE.checking>(
  value: Record<string, unknown>,
  state: T,
): { state: T } | null {
  return Object.keys(value).length === 1 ? { state } : null;
}

function versionState<T extends typeof APP_UPDATE_STATE.current | typeof APP_UPDATE_STATE.ready | typeof APP_UPDATE_STATE.installing>(
  value: Record<string, unknown>,
  state: T,
): { state: T; version: string } | null {
  if (!hasOnlyKeys(value, ['state', 'version']) || !isVersion(value.version)) return null;
  return { state, version: value.version };
}

function availableState(
  value: Record<string, unknown>,
): Extract<AppUpdateStatus, { state: typeof APP_UPDATE_STATE.available }> | null {
  if (
    !hasOnlyKeys(value, ['state', 'version', 'currentVersion'])
    || !isVersion(value.version)
    || !isVersion(value.currentVersion)
  ) {
    return null;
  }
  return {
    state: APP_UPDATE_STATE.available,
    version: value.version,
    currentVersion: value.currentVersion,
  };
}

function downloadingState(
  value: Record<string, unknown>,
): Extract<AppUpdateStatus, { state: typeof APP_UPDATE_STATE.downloading }> | null {
  if (
    !hasOnlyKeys(value, ['state', 'version', 'received', 'total'])
    || !isVersion(value.version)
    || typeof value.received !== 'number'
    || !Number.isFinite(value.received)
    || value.received < 0
    || !(typeof value.total === 'number' || value.total === null)
    || (typeof value.total === 'number' && (!Number.isFinite(value.total) || value.total < 0))
  ) {
    return null;
  }
  return {
    state: APP_UPDATE_STATE.downloading,
    version: value.version,
    received: value.received,
    total: value.total,
  };
}

function errorState(
  value: Record<string, unknown>,
): Extract<AppUpdateStatus, { state: typeof APP_UPDATE_STATE.error }> | null {
  if (!hasOnlyKeys(value, ['state', 'message']) || typeof value.message !== 'string' || value.message.length === 0) {
    return null;
  }
  return { state: APP_UPDATE_STATE.error, message: value.message };
}

function hasOnlyKeys(value: Record<string, unknown>, keys: readonly string[]): boolean {
  const got = Object.keys(value).sort();
  const want = [...keys].sort();
  return got.length === want.length && got.every((key, index) => key === want[index]);
}

function isVersion(value: unknown): value is string {
  return typeof value === 'string' && VERSION_PATTERN.test(value);
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === 'object' && value !== null && !Array.isArray(value);
}
