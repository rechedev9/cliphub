import assert from 'node:assert/strict';
import test from 'node:test';
import {
  APP_UPDATE_STATE,
  appUpdateAction,
  appUpdateLabel,
  appUpdatePercent,
  appUpdateTitle,
  appUpdateVisible,
  parseAppUpdateStatus,
} from './app-update.ts';

test('parses desktop update snapshots and rejects extras', () => {
  const cases: Array<{ name: string; value: unknown; ok: boolean }> = [
    { name: 'idle', value: { state: 'idle' }, ok: true },
    { name: 'available', value: { state: 'available', version: '2.4.29', currentVersion: '2.4.28' }, ok: true },
    { name: 'downloading', value: { state: 'downloading', version: '2.4.29', received: 10, total: 100 }, ok: true },
    { name: 'ready', value: { state: 'ready', version: '2.4.29' }, ok: true },
    { name: 'error', value: { state: 'error', message: 'No se ha podido consultar GitHub Releases.' }, ok: true },
    { name: 'unknown state', value: { state: 'restart' }, ok: false },
    { name: 'extra key', value: { state: 'idle', extra: true }, ok: false },
    { name: 'bad version', value: { state: 'ready', version: 'v2.4.29' }, ok: false },
    { name: 'ipc error wrapper', value: { ok: false, error: 'nope' }, ok: false },
  ];
  for (const row of cases) {
    const parsed = parseAppUpdateStatus(row.value);
    assert.equal(parsed !== null, row.ok, row.name);
  }
});

test('hides current builds and labels the actionable states', () => {
  assert.equal(appUpdateVisible({ state: APP_UPDATE_STATE.idle }), false);
  assert.equal(appUpdateVisible({ state: APP_UPDATE_STATE.current, version: '2.4.28' }), false);
  assert.equal(appUpdateVisible({ state: APP_UPDATE_STATE.unavailable }), false);
  assert.equal(appUpdateVisible({ state: APP_UPDATE_STATE.available, version: '2.4.29', currentVersion: '2.4.28' }), true);

  assert.equal(
    appUpdateLabel({ state: APP_UPDATE_STATE.available, version: '2.4.29', currentVersion: '2.4.28' }),
    'Actualizar',
  );
  assert.equal(
    appUpdateLabel({ state: APP_UPDATE_STATE.downloading, version: '2.4.29', received: 41, total: 100 }),
    '41%',
  );
  assert.equal(
    appUpdatePercent({ state: APP_UPDATE_STATE.downloading, version: '2.4.29', received: 10, total: null }),
    undefined,
  );
  assert.equal(appUpdateAction({ state: APP_UPDATE_STATE.error, message: 'falló' }), 'check');
  assert.equal(
    appUpdateAction({ state: APP_UPDATE_STATE.available, version: '2.4.29', currentVersion: '2.4.28' }),
    'install',
  );
  assert.equal(
    appUpdateTitle({ state: APP_UPDATE_STATE.ready, version: '2.4.29' }, true),
    'Espera a que terminen captura y edición para instalar',
  );
});
