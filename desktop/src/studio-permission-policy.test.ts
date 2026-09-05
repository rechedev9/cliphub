import assert from 'node:assert/strict';
import test from 'node:test';
import {
  isAllowedStudioPermission,
  type StudioPermissionRequest,
} from './studio-permission-policy.ts';

const allowed: StudioPermissionRequest = {
  permission: 'fullscreen',
  expectedOrigin: 'http://127.0.0.1:43120',
  expectedWebContentsID: 17,
  requestingWebContentsID: 17,
  requestingOrigin: 'http://127.0.0.1:43120',
  requestingURL: 'http://127.0.0.1:43120/library?tab=videos',
  isMainFrame: true,
  windowFocused: true,
};

test('allows fullscreen for the focused Studio top frame at the exact active origin', () => {
  assert.equal(isAllowedStudioPermission(allowed), true);
});

test('denies every permission except fullscreen', () => {
  for (const permission of [
    'clipboard-read',
    'clipboard-sanitized-write',
    'media',
    'notifications',
    'openExternal',
    'pointerLock',
    'unknown',
    'automatic-fullscreen',
  ]) {
    assert.equal(isAllowedStudioPermission({ ...allowed, permission }), false, permission);
  }
});

test('denies fullscreen outside the active trusted top frame', () => {
  const denied: Array<[string, Partial<StudioPermissionRequest>]> = [
    ['unfocused window', { windowFocused: false }],
    ['subframe', { isMainFrame: false }],
    ['different web contents', { requestingWebContentsID: 18 }],
    ['missing requesting web contents', { requestingWebContentsID: null }],
    ['missing active web contents', { expectedWebContentsID: null }],
    ['missing active origin', { expectedOrigin: null }],
    ['different origin', { requestingOrigin: 'http://127.0.0.1:43121' }],
    ['different URL origin', { requestingURL: 'https://example.com/watch' }],
    ['malformed origin', { requestingOrigin: 'not a URL' }],
    ['malformed URL', { requestingURL: 'not a URL' }],
  ];

  for (const [name, override] of denied) {
    assert.equal(isAllowedStudioPermission({ ...allowed, ...override }), false, name);
  }
});

test('accepts a request-handler check that only exposes the requesting URL', () => {
  assert.equal(
    isAllowedStudioPermission({
      ...allowed,
      requestingOrigin: allowed.requestingURL!,
      requestingURL: undefined,
    }),
    true,
  );
});
