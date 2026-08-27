import assert from 'node:assert/strict';
import test from 'node:test';
import { claimAccountDevice, listAccountDevices, loginAccount } from './account.ts';

test('login validates and returns the authenticated account', async (t) => {
  t.mock.method(globalThis, 'fetch', async () => Response.json({ user: { id: 'user-1', email: 'player@example.com' } }));
  assert.deepEqual(await loginAccount('player@example.com', 'long-password'), {
    ok: true,
    value: { id: 'user-1', email: 'player@example.com' },
  });
});

test('account API preserves structured errors and rejects malformed devices', async (t) => {
  t.mock.method(globalThis, 'fetch', async () => Response.json({
    code: 'invalid_credentials', error: 'Correo o contraseña incorrectos.',
  }, { status: 401 }));
  assert.deepEqual(await loginAccount('player@example.com', 'wrong-password'), {
    ok: false,
    code: 'invalid_credentials',
    error: 'Correo o contraseña incorrectos.',
  });

  t.mock.restoreAll();
  t.mock.method(globalThis, 'fetch', async () => Response.json({ devices: [{ id: 42 }] }));
  assert.deepEqual(await listAccountDevices(), {
    ok: false,
    code: 'invalid_response',
    error: 'El servidor devolvió una respuesta no válida.',
  });
});

test('claim parses an offline device whose heartbeat has not arrived yet', async (t) => {
  t.mock.method(globalThis, 'fetch', async () => Response.json({
    device: { id: 'device-1', name: 'Gaming PC', platform: 'windows-amd64', version: '2.5.0' },
  }));
  assert.deepEqual(await claimAccountDevice('ABCDE-23456'), {
    ok: true,
    value: {
      id: 'device-1', name: 'Gaming PC', platform: 'windows-amd64', version: '2.5.0', online: false, lastSeen: '',
    },
  });
});
