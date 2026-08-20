export const APP_UPDATE_CHANNEL = 'cliphub:app-update';
export const APP_UPDATE_STATUS_CHANNEL = 'cliphub:app-update-status';

export const APP_UPDATE_ACTION = {
  status: 'status',
  check: 'check',
  install: 'install',
} as const;

export type AppUpdateAction = typeof APP_UPDATE_ACTION[keyof typeof APP_UPDATE_ACTION];

export interface AppUpdateRequest {
  action: AppUpdateAction;
}

// Parses the only update messages accepted from the sandboxed preload bridge.
export function parseAppUpdateRequest(value: unknown): AppUpdateRequest {
  if (!isRecord(value) || !Object.hasOwn(value, 'action') || typeof value.action !== 'string') {
    throw new Error('invalid Studio update request');
  }
  const action = value.action;
  if (
    action !== APP_UPDATE_ACTION.status
    && action !== APP_UPDATE_ACTION.check
    && action !== APP_UPDATE_ACTION.install
  ) {
    throw new Error('invalid Studio update request');
  }
  requireExactKeys(value, ['action']);
  return { action };
}

function requireExactKeys(value: Record<string, unknown>, expected: string[]): void {
  const keys = Object.keys(value).sort();
  const wanted = [...expected].sort();
  if (keys.length !== wanted.length || keys.some((key, index) => key !== wanted[index])) {
    throw new Error('invalid Studio update request');
  }
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === 'object' && value !== null && !Array.isArray(value);
}
