interface DesktopTelemetryBridge {
  recordError(value: unknown): Promise<unknown>;
  recordSpan(value: unknown): Promise<unknown>;
}

export type RendererErrorContext = {
  /** Next's server digest, which also appears in the web server's log. */
  digest?: string;
  pathname?: string;
};

/** Reports a renderer failure through Electron main; browsers remain a no-op. */
export function recordRendererError(name: string, error: Error, context: RendererErrorContext = {}): void {
  const bridge = getDesktopTelemetryBridge();
  if (bridge === null) return;
  const pathname = context.pathname ?? currentPathname();
  // First line, so it survives the head/tail truncation of long stacks.
  const header = `digest=${context.digest?.trim() || 'none'} route=${rendererRoute(pathname)}`;
  void bridge.recordError({
    kind: 'error',
    name,
    message: `${header}\n${rendererErrorText(error)}`,
  }).catch(() => {});
}

// Static app segments; anything else is a job, clip or player id.
const STATIC_ROUTE_SEGMENTS: ReadonlySet<string> = new Set([
  'bootstrap', 'cheaters', 'clips', 'editor', 'feed', 'full-demo', 'matches', 'nueva', 'nuevo',
  'onboarding', 'players', 'publicar', 'series', 'settings', 'streams', 'tactical', 'upload', 'videos',
]);

/**
 * Route pattern for the renderer event, e.g. "/clips/<uuid>/nuevo" ->
 * "clips.{id}.nuevo". Dots instead of slashes: the diagnostic filter redacts
 * any "/"-token outside /api/ as a path, which would leave only "[path]".
 */
export function rendererRoute(pathname: string | undefined): string {
  const segments = (pathname ?? '').split('?')[0]?.split('/').filter(Boolean) ?? [];
  if (segments.length === 0) return pathname === undefined ? 'unknown' : 'root';
  return segments.map((segment) => (STATIC_ROUTE_SEGMENTS.has(segment) ? segment : '{id}')).join('.');
}

function currentPathname(): string | undefined {
  const location = isRecord(globalThis) ? (globalThis as { location?: unknown }).location : undefined;
  return isRecord(location) && typeof location.pathname === 'string' ? location.pathname : undefined;
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
