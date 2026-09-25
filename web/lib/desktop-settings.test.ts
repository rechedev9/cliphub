import test from 'node:test';
import assert from 'node:assert/strict';
import { getDesktopSettingsBridge, type DesktopSettingsBridge } from './desktop-settings.ts';

function bridge(): DesktopSettingsBridge {
  return {
    getAppInfo: async () => ({ version: '2.2.9', build: 'production', electronVersion: '37.0.0', chromiumVersion: '138.0.0' }),
    getPlaybackInfo: async () => ({
      available: true,
      state: 'ready',
      electronVersion: '37.0.0',
      chromiumVersion: '138.0.0',
      hardwareAcceleration: 'enabled',
      videoDecode: 'hardware-accelerated',
      scope: 'global',
    }),
    getTelemetry: async () => ({
      available: true,
      enabled: true,
      noticeAcknowledged: true,
      supportCode: 'CH-ABCD-1234-5678-90AB-CDEF',
      retentionDays: 30,
      performanceSamplePercent: 10,
    }),
    updateTelemetry: async (enabled) => ({
      available: true,
      enabled,
      noticeAcknowledged: true,
      supportCode: 'CH-ABCD-1234-5678-90AB-CDEF',
      retentionDays: 30,
      performanceSamplePercent: 10,
    }),
  };
}

test('returns null outside Electron instead of falling back to HTTP', () => {
  assert.equal(getDesktopSettingsBridge({}), null);
  assert.equal(getDesktopSettingsBridge(null), null);
  assert.equal(getDesktopSettingsBridge({ cliphubSettings: {} }), null);
});

test('rejects an incomplete preload settings surface', () => {
  assert.equal(getDesktopSettingsBridge({ cliphubSettings: { getAppInfo: 'nope' } }), null);
  assert.equal(getDesktopSettingsBridge({ cliphubSettings: { getAppInfo() {}, getTelemetry() {} } }), null);
});

test('returns the complete narrow preload bridge', () => {
  const expected = bridge();
  const got = getDesktopSettingsBridge({ cliphubSettings: expected });

  assert.equal(got, expected);
});

test('keeps the previous settings bridge usable during a desktop update', () => {
  const previousBridge = bridge();
  delete previousBridge.getPlaybackInfo;
  assert.equal(getDesktopSettingsBridge({ cliphubSettings: previousBridge }), previousBridge);
});

test('rejects a malformed optional playback operation', () => {
  assert.equal(getDesktopSettingsBridge({
    cliphubSettings: { ...bridge(), getPlaybackInfo: 'not a function' },
  }), null);
});
