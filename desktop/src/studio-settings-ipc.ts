export const STUDIO_SETTINGS_CHANNEL = 'fragforge:studio-settings';

export const STUDIO_SETTINGS_ACTION = {
  appInfo: 'app-info',
} as const;

export type StudioSettingsAction = typeof STUDIO_SETTINGS_ACTION[keyof typeof STUDIO_SETTINGS_ACTION];

export interface StudioSettingsRequest {
  action: typeof STUDIO_SETTINGS_ACTION.appInfo;
}

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
  if (action !== STUDIO_SETTINGS_ACTION.appInfo) throw new Error('invalid Studio settings request');
  requireExactKeys(value, ['action']);
  return { action };
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
