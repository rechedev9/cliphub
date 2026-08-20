import assert from 'node:assert/strict';
import test from 'node:test';
import { getDesktopUpdateBridge, readDesktopUpdateStatus } from './desktop-update.ts';

function bridge() {
  return {
    getStatus: async () => ({ state: 'available', version: '2.4.29', currentVersion: '2.4.28' }),
    check: async () => ({ ok: true }),
    install: async () => ({ ok: true }),
    onStatus: () => () => {},
  };
}

test('returns null outside Electron instead of falling back to HTTP', () => {
  assert.equal(getDesktopUpdateBridge({}), null);
  assert.equal(getDesktopUpdateBridge(null), null);
  assert.equal(getDesktopUpdateBridge({ cliphubUpdate: { getStatus: () => null } }), null);
});

test('returns the complete update preload bridge', async () => {
  const expected = bridge();
  const got = getDesktopUpdateBridge({ cliphubUpdate: expected });
  assert.equal(got, expected);
  if (got === null) throw new Error('expected the desktop update bridge');
  const status = await readDesktopUpdateStatus(got);
  assert.deepEqual(status, { state: 'available', version: '2.4.29', currentVersion: '2.4.28' });
});
