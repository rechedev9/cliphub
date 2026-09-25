export interface DesktopUpdateBridge {
  getStatus(): Promise<unknown>;
  check(): Promise<unknown>;
  install(): Promise<unknown>;
  onStatus(listener: (status: unknown) => void): () => void;
}

export function getDesktopUpdateBridge(scope: unknown = globalThis): DesktopUpdateBridge | null {
  if (!isRecord(scope)) return null;
  return isDesktopUpdateBridge(scope.cliphubUpdate) ? scope.cliphubUpdate : null;
}

function isDesktopUpdateBridge(value: unknown): value is DesktopUpdateBridge {
  if (!isRecord(value)) return false;
  return (
    typeof value.getStatus === 'function'
    && typeof value.check === 'function'
    && typeof value.install === 'function'
    && typeof value.onStatus === 'function'
  );
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === 'object' && value !== null;
}
