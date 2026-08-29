/**
 * Narrow bridge exposed only by the ClipHub Studio Electron preload.
 *
 * The browser UI deliberately has no HTTP fallback for these operations: they
 * are answered by the desktop main process, so a plain browser must render the
 * desktop-only state instead of reaching for the orchestrator.
 */

export type StudioAppInfo = {
  version: string;
  build: string;
  electronVersion: string;
  chromiumVersion: string;
};

export type StudioTelemetryStatus = {
  available: boolean;
  enabled: boolean;
  noticeAcknowledged: boolean;
  supportCode: string;
  retentionDays: 30;
  performanceSamplePercent: 10;
};

export interface DesktopSettingsBridge {
  getAppInfo(): Promise<StudioAppInfo>;
  getTelemetry(): Promise<StudioTelemetryStatus>;
  updateTelemetry(enabled: boolean): Promise<StudioTelemetryStatus>;
}

/**
 * Returns the preload bridge when running inside ClipHub Studio. A normal
 * browser (including frontend-only development) receives null and must render
 * the desktop-only state instead of attempting a network fallback.
 */
export function getDesktopSettingsBridge(scope: unknown = globalThis): DesktopSettingsBridge | null {
  if (!isRecord(scope)) return null;
  const candidate = scope.cliphubSettings;
  return isDesktopSettingsBridge(candidate) ? candidate : null;
}

function isDesktopSettingsBridge(value: unknown): value is DesktopSettingsBridge {
  if (!isRecord(value)) return false;
  return typeof value.getAppInfo === 'function'
    && typeof value.getTelemetry === 'function'
    && typeof value.updateTelemetry === 'function';
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === 'object' && value !== null;
}
