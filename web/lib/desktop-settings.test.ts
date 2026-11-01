import test from 'node:test';
import assert from 'node:assert/strict';
import {
  claudeCodeCommand,
  claudeDesktopConfigSnippet,
  getDesktopSettingsBridge,
  parseAssistantAccess,
  type DesktopSettingsBridge,
} from './desktop-settings.ts';

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

test('rejects a malformed optional assistant operation', () => {
  assert.equal(getDesktopSettingsBridge({
    cliphubSettings: { ...bridge(), getAssistantAccess: 'not a function' },
  }), null);
  assert.equal(getDesktopSettingsBridge({
    cliphubSettings: { ...bridge(), updateAssistantAccess: 1 },
  }), null);
});

test('reads an assistant access reply and refuses a failure reply', () => {
  const mcpPath = String.raw`C:\Users\Ana\AppData\Roaming\cliphub-studio\mcp\zv-mcp.exe`;
  assert.deepEqual(parseAssistantAccess({ enabled: true, mcpPath }), { enabled: true, mcpPath });
  assert.equal(parseAssistantAccess({ ok: false, error: 'No se pudo guardar.' }), null);
  assert.equal(parseAssistantAccess({ enabled: 'yes', mcpPath }), null);
  assert.equal(parseAssistantAccess({ enabled: false, mcpPath: '' }), null);
  assert.equal(parseAssistantAccess(null), null);
});

test('builds the Claude snippets with the path escaped for each format', () => {
  const mcpPath = String.raw`C:\Users\Ana Lopez\AppData\Roaming\cliphub-studio\mcp\zv-mcp.exe`;
  const desktopSnippet = claudeDesktopConfigSnippet(mcpPath);
  // JSON needs every backslash doubled, and reading it back must give the original path.
  assert.ok(desktopSnippet.includes(String.raw`"command": "C:\\Users\\Ana Lopez\\AppData\\Roaming\\cliphub-studio\\mcp\\zv-mcp.exe"`));
  assert.deepEqual(JSON.parse(desktopSnippet), {
    mcpServers: { 'cliphub-studio': { command: mcpPath } },
  });
  // A shell command takes the path as typed, quoted because of the space.
  assert.equal(claudeCodeCommand(mcpPath), `claude mcp add cliphub-studio -- "${mcpPath}"`);
  assert.ok(!claudeCodeCommand(mcpPath).includes('\\\\'));
});
