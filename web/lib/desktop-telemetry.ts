interface DesktopTelemetryBridge {
  recordError(value: unknown): Promise<unknown>;
  recordSpan(value: unknown): Promise<unknown>;
}

/** Reports a renderer failure through Electron main; browsers remain a no-op. */
export function recordRendererError(name: string, _error: Error): void {
  const bridge = getDesktopTelemetryBridge();
  if (bridge === null) return;
  void bridge.recordError({
    kind: 'error',
    name,
  }).catch(() => {});
}

/** Reports a renderer duration; Electron main owns the 10% sampling decision. */
export function recordRendererSpan(name: string, durationMS: number): void {
  const bridge = getDesktopTelemetryBridge();
  if (bridge === null || !Number.isFinite(durationMS) || durationMS < 0) return;
  void bridge.recordSpan({
    kind: 'span',
    name,
    durationMS,
  }).catch(() => {});
}

function getDesktopTelemetryBridge(scope: unknown = globalThis): DesktopTelemetryBridge | null {
  if (!isRecord(scope)) return null;
  const candidate = scope.cliphubTelemetry;
  return isDesktopTelemetryBridge(candidate) ? candidate : null;
}

function isDesktopTelemetryBridge(value: unknown): value is DesktopTelemetryBridge {
  return isRecord(value)
    && typeof value.recordError === 'function'
    && typeof value.recordSpan === 'function';
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === 'object' && value !== null && !Array.isArray(value);
}
