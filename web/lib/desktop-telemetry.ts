interface DesktopTelemetryBridge {
  recordError(value: unknown): Promise<unknown>;
  recordSpan(value: unknown): Promise<unknown>;
}

/** Reports a renderer failure through Electron main; browsers remain a no-op. */
export function recordRendererError(name: string, error: Error): void {
  const bridge = getDesktopTelemetryBridge();
  if (bridge === null) return;
  void bridge.recordError({
    kind: 'error',
    name,
    message: rendererErrorText(error),
  }).catch(() => {});
}

function rendererErrorText(error: Error): string {
  const parts: string[] = [];
  const seen = new Set<Error>();
  let current: unknown = error;
  while (current instanceof Error && !seen.has(current) && parts.length < 4) {
    seen.add(current);
    parts.push(current.stack ?? `${current.name}: ${current.message}`);
    current = current.cause;
  }
  const text = parts.join('\nCaused by: ');
  return text.length > 64 * 1024 ? `${text.slice(0, 32 * 1024)}\n[truncated]\n${text.slice(-31 * 1024)}` : text;
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
