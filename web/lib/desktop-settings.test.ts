import test from 'node:test';
import assert from 'node:assert/strict';
import { getDesktopSettingsBridge, type DesktopSettingsBridge } from './desktop-settings.ts';

function bridge(): DesktopSettingsBridge {
  return {
    getAppInfo: async () => ({ version: '2.2.9', build: 'production', electronVersion: '37.0.0', chromiumVersion: '138.0.0' }),
  };
}

test('returns null outside Electron instead of falling back to HTTP', () => {
  assert.equal(getDesktopSettingsBridge({}), null);
  assert.equal(getDesktopSettingsBridge(null), null);
  assert.equal(getDesktopSettingsBridge({ cliphubSettings: {} }), null);
});

test('rejects a preload surface without the app info call', () => {
  assert.equal(getDesktopSettingsBridge({ cliphubSettings: { getAppInfo: 'nope' } }), null);
});

test('returns the complete narrow preload bridge', async () => {
  const expected = bridge();
  const got = getDesktopSettingsBridge({ cliphubSettings: expected });

  assert.equal(got, expected);
  if (got === null) throw new Error('expected the desktop settings bridge');
  assert.equal((await got.getAppInfo()).version, '2.2.9');
});
