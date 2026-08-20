import assert from 'node:assert/strict';
import test from 'node:test';
import { APP_UPDATE_ACTION, parseAppUpdateRequest } from './app-update-ipc.ts';

test('parses only the narrow Studio update action shape', () => {
  for (const action of [
    APP_UPDATE_ACTION.status,
    APP_UPDATE_ACTION.check,
    APP_UPDATE_ACTION.install,
  ]) {
    assert.deepEqual(parseAppUpdateRequest({ action }), { action });
  }

  for (const invalid of [
    null,
    {},
    { action: 'app-info' },
    { action: 'download' },
    { action: APP_UPDATE_ACTION.install, extra: true },
    Object.create({ action: APP_UPDATE_ACTION.status }),
  ]) {
    assert.throws(() => parseAppUpdateRequest(invalid), /invalid Studio update request/);
  }
});
