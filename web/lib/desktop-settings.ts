/**
 * Narrow bridge exposed only by the TickCut Studio Electron preload.
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

export interface DesktopSettingsBridge {
  getAppInfo(): Promise<StudioAppInfo>;
}

/**
 * Returns the preload bridge when running inside TickCut Studio. A normal
 * browser (including frontend-only development) receives null and must render
 * the desktop-only state instead of attempting a network fallback.
 */
export function getDesktopSettingsBridge(scope: unknown = globalThis): DesktopSettingsBridge | null {
  if (!isRecord(scope)) return null;
  const candidate = scope.tickcutSettings;
  return isDesktopSettingsBridge(candidate) ? candidate : null;
}

function isDesktopSettingsBridge(value: unknown): value is DesktopSettingsBridge {
  if (!isRecord(value)) return false;
  return typeof value.getAppInfo === 'function';
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === 'object' && value !== null;
}
