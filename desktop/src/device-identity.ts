import { randomBytes, randomUUID } from 'node:crypto';
import * as fs from 'node:fs';
import * as path from 'node:path';

const IDENTITY_SCHEMA_VERSION = 1;
const DEVICE_SECRET_PATTERN = /^[a-f0-9]{64}$/;
const UUID_PATTERN = /^[0-9a-f]{8}-[0-9a-f]{4}-[1-8][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/i;

interface StoredDeviceIdentity {
  schemaVersion: 1;
  deviceId: string;
  encryptedSecret: string;
}

export interface DeviceIdentity {
  readonly deviceId: string;
  readonly secret: string;
}

export interface SecretProtector {
  isEncryptionAvailable(): boolean;
  encryptString(value: string): Buffer;
  decryptString(value: Buffer): string;
}

export function loadOrCreateDeviceIdentity(
  identityPath: string,
  protector: SecretProtector,
  generateID: () => string = randomUUID,
  generateSecret: () => string = () => randomBytes(32).toString('hex'),
): DeviceIdentity {
  if (!protector.isEncryptionAvailable()) {
    throw new Error('Windows credential encryption is unavailable');
  }

  const stored = readStoredIdentity(identityPath);
  if (stored !== null) {
    const secret = protector.decryptString(Buffer.from(stored.encryptedSecret, 'base64'));
    if (!DEVICE_SECRET_PATTERN.test(secret)) throw new Error('stored device identity is invalid');
    return { deviceId: stored.deviceId, secret };
  }

  const deviceId = generateID();
  const secret = generateSecret();
  if (!UUID_PATTERN.test(deviceId) || !DEVICE_SECRET_PATTERN.test(secret)) {
    throw new Error('device identity generator returned an invalid value');
  }
  const encoded: StoredDeviceIdentity = {
    schemaVersion: IDENTITY_SCHEMA_VERSION,
    deviceId,
    encryptedSecret: protector.encryptString(secret).toString('base64'),
  };
  writeIdentityAtomically(identityPath, encoded);
  return { deviceId, secret };
}

function readStoredIdentity(identityPath: string): StoredDeviceIdentity | null {
  let raw: string;
  try {
    raw = fs.readFileSync(identityPath, 'utf8');
  } catch (error: unknown) {
    if (isNodeError(error) && error.code === 'ENOENT') return null;
    throw error;
  }

  let parsed: unknown;
  try {
    parsed = JSON.parse(raw) as unknown;
  } catch (error: unknown) {
    throw new Error('stored device identity is not valid JSON', { cause: error });
  }
  if (!isStoredDeviceIdentity(parsed)) throw new Error('stored device identity is invalid');
  return parsed;
}

function isStoredDeviceIdentity(value: unknown): value is StoredDeviceIdentity {
  if (typeof value !== 'object' || value === null) return false;
  const candidate = value as Record<string, unknown>;
  return candidate.schemaVersion === IDENTITY_SCHEMA_VERSION
    && typeof candidate.deviceId === 'string'
    && UUID_PATTERN.test(candidate.deviceId)
    && typeof candidate.encryptedSecret === 'string'
    && candidate.encryptedSecret.length > 0;
}

function writeIdentityAtomically(identityPath: string, identity: StoredDeviceIdentity): void {
  const directory = path.dirname(identityPath);
  fs.mkdirSync(directory, { recursive: true, mode: 0o700 });
  const temporary = `${identityPath}.${process.pid}.tmp`;
  try {
    fs.writeFileSync(temporary, `${JSON.stringify(identity)}\n`, { encoding: 'utf8', mode: 0o600, flag: 'wx' });
    fs.renameSync(temporary, identityPath);
  } catch (error: unknown) {
    try {
      fs.unlinkSync(temporary);
    } catch (cleanupError: unknown) {
      if (!isNodeError(cleanupError) || cleanupError.code !== 'ENOENT') {
        throw new AggregateError([error, cleanupError], 'write device identity and clean temporary file');
      }
    }
    throw error;
  }
}

function isNodeError(error: unknown): error is NodeJS.ErrnoException {
  return error instanceof Error && 'code' in error;
}
