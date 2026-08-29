export const STUDIO_SETTINGS_CHANNEL = 'cliphub:studio-settings';

export const STUDIO_SETTINGS_ACTION = {
  appInfo: 'app-info',
  telemetryStatus: 'telemetry-status',
  telemetryUpdate: 'telemetry-update',
} as const;

export type StudioSettingsRequest =
  | { action: typeof STUDIO_SETTINGS_ACTION.appInfo }
  | { action: typeof STUDIO_SETTINGS_ACTION.telemetryStatus }
  | { action: typeof STUDIO_SETTINGS_ACTION.telemetryUpdate; enabled: boolean };

export interface TrustedSettingsSenderInput {
  expectedOrigin: string | null;
  expectedWebContentsID: number | null;
  isMainFrame: boolean;
  senderURL: string;
  senderWebContentsID: number;
}

/** Parses the only messages accepted from the sandboxed preload bridge. */
export function parseStudioSettingsRequest(value: unknown): StudioSettingsRequest {
  if (!isRecord(value) || !Object.hasOwn(value, 'action') || typeof value.action !== 'string') {
    throw new Error('invalid Studio settings request');
  }
  const action = value.action;
  if (action === STUDIO_SETTINGS_ACTION.appInfo || action === STUDIO_SETTINGS_ACTION.telemetryStatus) {
    requireExactKeys(value, ['action']);
    return { action };
  }
  if (action === STUDIO_SETTINGS_ACTION.telemetryUpdate && typeof value.enabled === 'boolean') {
    requireExactKeys(value, ['action', 'enabled']);
    return { action, enabled: value.enabled };
  }
  throw new Error('invalid Studio settings request');
}

/** Accepts IPC only from the active Studio page's top frame and exact web origin. */
export function isTrustedSettingsSender(input: TrustedSettingsSenderInput): boolean {
  if (input.expectedOrigin === null
    || input.expectedWebContentsID === null
    || input.senderWebContentsID !== input.expectedWebContentsID
    || !input.isMainFrame) return false;
  try {
    return new URL(input.senderURL).origin === input.expectedOrigin;
  } catch {
    return false;
  }
}

function requireExactKeys(value: Record<string, unknown>, expected: string[]): void {
  const keys = Object.keys(value).sort();
  const wanted = [...expected].sort();
  if (keys.length !== wanted.length || keys.some((key, index) => key !== wanted[index])) {
    throw new Error('invalid Studio settings request');
  }
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === 'object' && value !== null && !Array.isArray(value);
}
