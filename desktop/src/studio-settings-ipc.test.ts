import assert from 'node:assert/strict';
import test from 'node:test';
import {
  isTrustedSettingsSender,
  parseStudioSettingsRequest,
  STUDIO_SETTINGS_ACTION,
} from './studio-settings-ipc.ts';

test('parses only the narrow Studio settings action shape', () => {
  assert.deepEqual(parseStudioSettingsRequest({ action: 'app-info' }), {
    action: STUDIO_SETTINGS_ACTION.appInfo,
  });

  for (const invalid of [
    null,
    {},
    { action: 'read-key' },
    { action: 'status' },
    { action: 'restart' },
    { action: 'save', apiKey: 'secret' },
    { action: 'app-info', extra: true },
    Object.create({ action: STUDIO_SETTINGS_ACTION.appInfo }),
  ]) {
    assert.throws(() => parseStudioSettingsRequest(invalid), /invalid Studio settings request/);
  }
});

test('trusts only the active top-level web frame and exact origin', () => {
  const trusted = {
    expectedOrigin: 'http://127.0.0.1:3010',
    expectedWebContentsID: 7,
    isMainFrame: true,
    senderURL: 'http://127.0.0.1:3010/settings',
    senderWebContentsID: 7,
  };
  assert.equal(isTrustedSettingsSender(trusted), true);
  assert.equal(isTrustedSettingsSender({ ...trusted, isMainFrame: false }), false);
  assert.equal(isTrustedSettingsSender({ ...trusted, senderWebContentsID: 8 }), false);
  assert.equal(isTrustedSettingsSender({ ...trusted, senderURL: 'http://127.0.0.1:8080/settings' }), false);
  assert.equal(isTrustedSettingsSender({ ...trusted, senderURL: 'https://example.com/settings' }), false);
  assert.equal(isTrustedSettingsSender({ ...trusted, senderURL: 'not a URL' }), false);
  assert.equal(isTrustedSettingsSender({ ...trusted, expectedOrigin: null }), false);
});
