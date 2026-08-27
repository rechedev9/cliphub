import assert from 'node:assert/strict';
import * as fs from 'node:fs';
import * as os from 'node:os';
import * as path from 'node:path';
import test from 'node:test';
import { loadOrCreateDeviceIdentity, type SecretProtector } from './device-identity.ts';

const DEVICE_ID = '11111111-1111-4111-8111-111111111111';
const DEVICE_SECRET = 'a'.repeat(64);

test('creates an encrypted identity and reuses it without persisting the plaintext secret', (t) => {
  const directory = fs.mkdtempSync(path.join(os.tmpdir(), 'cliphub-identity-test-'));
  t.after(() => fs.rmSync(directory, { recursive: true, force: true }));
  const identityPath = path.join(directory, 'device.json');
  const protector = fakeProtector();

  const created = loadOrCreateDeviceIdentity(identityPath, protector, () => DEVICE_ID, () => DEVICE_SECRET);
  const persisted = fs.readFileSync(identityPath, 'utf8');
  const loaded = loadOrCreateDeviceIdentity(identityPath, protector);

  assert.deepEqual(created, { deviceId: DEVICE_ID, secret: DEVICE_SECRET });
  assert.deepEqual(loaded, created);
  assert.equal(persisted.includes(DEVICE_SECRET), false);
});

test('fails closed when OS encryption is unavailable or persisted identity is malformed', (t) => {
  const directory = fs.mkdtempSync(path.join(os.tmpdir(), 'cliphub-identity-test-'));
  t.after(() => fs.rmSync(directory, { recursive: true, force: true }));
  const identityPath = path.join(directory, 'device.json');
  const unavailable = fakeProtector(false);
  assert.throws(() => loadOrCreateDeviceIdentity(identityPath, unavailable), /encryption is unavailable/);

  fs.writeFileSync(identityPath, '{}');
  assert.throws(() => loadOrCreateDeviceIdentity(identityPath, fakeProtector()), /identity is invalid/);
});

function fakeProtector(available = true): SecretProtector {
  return {
    isEncryptionAvailable: () => available,
    encryptString: (value) => Buffer.from(`protected:${value}`, 'utf8'),
    decryptString: (value) => value.toString('utf8').replace(/^protected:/, ''),
  };
}
