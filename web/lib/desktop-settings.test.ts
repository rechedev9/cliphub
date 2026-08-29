import test from 'node:test';
import assert from 'node:assert/strict';
import { getDesktopSettingsBridge, type DesktopSettingsBridge } from './desktop-settings.ts';

function bridge(): DesktopSettingsBridge {
  return {
    getAppInfo: async () => ({ version: '2.2.9', build: 'production', electronVersion: '37.0.0', chromiumVersion: '138.0.0' }),
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

test('returns the complete narrow preload bridge', async () => {
  const expected = bridge();
  const got = getDesktopSettingsBridge({ cliphubSettings: expected });

  assert.equal(got, expected);
  if (got === null) throw new Error('expected the desktop settings bridge');
  assert.equal((await got.getAppInfo()).version, '2.2.9');
  assert.equal((await got.getTelemetry()).supportCode, 'CH-ABCD-1234-5678-90AB-CDEF');
  assert.equal((await got.updateTelemetry(false)).enabled, false);
});
