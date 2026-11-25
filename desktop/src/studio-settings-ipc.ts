export const STUDIO_SETTINGS_CHANNEL = 'cliphub:studio-settings';

export const STUDIO_SETTINGS_ACTION = {
  appInfo: 'app-info',
  playbackInfo: 'playback-info',
  telemetryStatus: 'telemetry-status',
  telemetryUpdate: 'telemetry-update',
  assistantStatus: 'assistant-status',
  assistantUpdate: 'assistant-update',
  teamspeakPluginInstall: 'teamspeak-plugin-install',
} as const;

export type StudioSettingsRequest =
  | { action: typeof STUDIO_SETTINGS_ACTION.appInfo }
  | { action: typeof STUDIO_SETTINGS_ACTION.playbackInfo }
  | { action: typeof STUDIO_SETTINGS_ACTION.telemetryStatus }
  | { action: typeof STUDIO_SETTINGS_ACTION.telemetryUpdate; enabled: boolean }
  | { action: typeof STUDIO_SETTINGS_ACTION.assistantStatus }
  | {
    action: typeof STUDIO_SETTINGS_ACTION.assistantUpdate;
    enabled: boolean;
    autoApproveStarts?: boolean;
    autoApproveDeletes?: boolean;
  }
  | { action: typeof STUDIO_SETTINGS_ACTION.teamspeakPluginInstall };

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
  if (action === STUDIO_SETTINGS_ACTION.appInfo
    || action === STUDIO_SETTINGS_ACTION.playbackInfo
    || action === STUDIO_SETTINGS_ACTION.telemetryStatus
    || action === STUDIO_SETTINGS_ACTION.assistantStatus
    || action === STUDIO_SETTINGS_ACTION.teamspeakPluginInstall) {
    requireExactKeys(value, ['action']);
    return { action };
  }
  if (action === STUDIO_SETTINGS_ACTION.telemetryUpdate && typeof value.enabled === 'boolean') {
    requireExactKeys(value, ['action', 'enabled']);
    return { action, enabled: value.enabled };
  }
  if (action === STUDIO_SETTINGS_ACTION.assistantUpdate && typeof value.enabled === 'boolean') {
    const autoApproveStarts = optionalFlag(value, 'autoApproveStarts');
    const autoApproveDeletes = optionalFlag(value, 'autoApproveDeletes');
    const request = {
      action,
      enabled: value.enabled,
      ...(autoApproveStarts === undefined ? {} : { autoApproveStarts }),
      ...(autoApproveDeletes === undefined ? {} : { autoApproveDeletes }),
    };
    requireExactKeys(value, Object.keys(request));
    return request;
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

function optionalFlag(value: Record<string, unknown>, key: string): boolean | undefined {
  if (!Object.hasOwn(value, key)) return undefined;
  const flag = value[key];
  if (typeof flag !== 'boolean') throw new Error('invalid Studio settings request');
  return flag;
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
