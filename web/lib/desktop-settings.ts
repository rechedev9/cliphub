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
  logDelivery?: {
    pendingBytes: number;
    pendingFiles: number;
    lastAcknowledgedAt: string | null;
    lastError: string | null;
    droppedRecords: number;
    rejectedRecords: number;
  };
};

export type StudioPlaybackInfo =
  | { available: false; state: 'initializing' }
  | {
    available: true;
    state: 'ready';
    electronVersion: string;
    chromiumVersion: string;
    hardwareAcceleration: 'enabled' | 'disabled';
    videoDecode: 'hardware-accelerated' | 'software-only' | 'unavailable' | 'unknown';
    scope: 'global';
  };

export type StudioAssistantAccess = {
  enabled: boolean;
  /** Absolute path of the stable zv-mcp copy under userData that an assistant launches. */
  mcpPath: string;
};

export interface DesktopSettingsBridge {
  getAppInfo(): Promise<StudioAppInfo>;
  /** Optional so a current UI can still use settings exposed by an older preload. */
  getPlaybackInfo?(): Promise<StudioPlaybackInfo>;
  getTelemetry(): Promise<StudioTelemetryStatus>;
  updateTelemetry(enabled: boolean): Promise<StudioTelemetryStatus>;
  /** Optional so a current UI can still use settings exposed by an older preload. */
  getAssistantAccess?(): Promise<unknown>;
  updateAssistantAccess?(enabled: boolean): Promise<unknown>;
  /** Opens the bundled .ts3_plugin package with TeamSpeak 3; optional for an older preload. */
  installTeamSpeakPlugin?(): Promise<unknown>;
}

/** Narrows an assistant-access reply; the main process answers a failure with another shape. */
export function parseAssistantAccess(value: unknown): StudioAssistantAccess | null {
  if (!isRecord(value) || typeof value.enabled !== 'boolean') return null;
  if (typeof value.mcpPath !== 'string' || value.mcpPath === '') return null;
  return { enabled: value.enabled, mcpPath: value.mcpPath };
}

/** Narrows the install reply to the error to show, or null when TeamSpeak opened the package. */
export function teamSpeakInstallError(value: unknown): string | null {
  if (isRecord(value) && value.ok === true) return null;
  if (isRecord(value) && typeof value.error === 'string' && value.error !== '') return value.error;
  return 'No se pudo abrir el plugin de TeamSpeak.';
}

/** The Claude Desktop config block that registers the bundled server. */
export function claudeDesktopConfigSnippet(mcpPath: string): string {
  return JSON.stringify({ mcpServers: { 'cliphub-studio': { command: mcpPath } } }, null, 2);
}

/** The Claude Code command that registers the bundled server. */
export function claudeCodeCommand(mcpPath: string): string {
  return `claude mcp add cliphub-studio -- "${mcpPath}"`;
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
    && (value.getPlaybackInfo === undefined || typeof value.getPlaybackInfo === 'function')
    && typeof value.getTelemetry === 'function'
    && typeof value.updateTelemetry === 'function'
    && (value.getAssistantAccess === undefined || typeof value.getAssistantAccess === 'function')
    && (value.updateAssistantAccess === undefined || typeof value.updateAssistantAccess === 'function')
    && (value.installTeamSpeakPlugin === undefined || typeof value.installTeamSpeakPlugin === 'function');
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === 'object' && value !== null;
}
