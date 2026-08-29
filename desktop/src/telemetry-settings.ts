import { randomBytes, randomUUID } from 'node:crypto';
import * as fs from 'node:fs';
import * as path from 'node:path';

const TELEMETRY_SETTINGS_SCHEMA_VERSION = 3;

interface StoredTelemetrySettings {
  schemaVersion: typeof TELEMETRY_SETTINGS_SCHEMA_VERSION;
  installationId: string;
  supportCode: string;
  enabled: boolean;
  noticeAcknowledged: boolean;
  consentEpoch: string;
}

export interface TelemetrySettings {
  enabled: boolean;
  noticeAcknowledged: boolean;
  supportCode: string;
}

/** Owns the pseudonymous installation ID and the user's diagnostic choice. */
export class TelemetrySettingsStore {
  private readonly filePath: string;
  private value: StoredTelemetrySettings;

  constructor(filePath: string) {
    this.filePath = filePath;
    this.value = this.load();
  }

  get(): TelemetrySettings {
    return {
      enabled: this.value.enabled,
      noticeAcknowledged: this.value.noticeAcknowledged,
      supportCode: this.value.supportCode,
    };
  }

  consentEpoch(): string {
    return this.value.consentEpoch;
  }

  eligible(): boolean {
    return this.value.enabled && this.value.noticeAcknowledged;
  }

  update(enabled: boolean): TelemetrySettings {
    const next = {
      ...this.value,
      enabled,
      noticeAcknowledged: true,
      consentEpoch: randomUUID(),
    };
    this.persistValue(next);
    this.value = next;
    return this.get();
  }

  private load(): StoredTelemetrySettings {
    try {
      const parsed: unknown = JSON.parse(fs.readFileSync(this.filePath, 'utf8'));
      if (isStoredTelemetrySettings(parsed)) return parsed;
    } catch {
      // A missing or corrupt file creates a fresh pseudonymous installation.
    }
    const initial: StoredTelemetrySettings = {
      schemaVersion: TELEMETRY_SETTINGS_SCHEMA_VERSION,
      installationId: randomUUID(),
      supportCode: generateSupportCode(),
      enabled: true,
      noticeAcknowledged: false,
      consentEpoch: randomUUID(),
    };
    try {
      this.persistValue(initial);
    } catch {
      // Diagnostics stay ineligible in memory when the choice cannot persist.
    }
    return initial;
  }

  private persistValue(value: StoredTelemetrySettings): void {
    fs.mkdirSync(path.dirname(this.filePath), { recursive: true, mode: 0o700 });
    const temporary = `${this.filePath}.${process.pid}.${randomUUID()}.tmp`;
    fs.writeFileSync(temporary, `${JSON.stringify(value)}\n`, { encoding: 'utf8', mode: 0o600 });
    try {
      fs.renameSync(temporary, this.filePath);
    } catch (error) {
      try {
        fs.rmSync(temporary, { force: true });
      } catch {
        // Preserve the original persistence error.
      }
      throw error;
    }
  }
}

function generateSupportCode(): string {
  const value = randomBytes(10).toString('hex').toUpperCase();
  return `CH-${[0, 4, 8, 12, 16].map((offset) => value.slice(offset, offset + 4)).join('-')}`;
}

function isStoredTelemetrySettings(value: unknown): value is StoredTelemetrySettings {
  if (!isRecord(value)) return false;
  const keys = Object.keys(value).sort();
  const expected = ['consentEpoch', 'enabled', 'installationId', 'noticeAcknowledged', 'schemaVersion', 'supportCode'];
  if (keys.length !== expected.length || keys.some((key, index) => key !== expected[index])) return false;
  return value.schemaVersion === TELEMETRY_SETTINGS_SCHEMA_VERSION
    && typeof value.installationId === 'string'
    && /^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/i.test(value.installationId)
    && typeof value.supportCode === 'string'
    && /^CH(?:-[A-F0-9]{4}){5}$/.test(value.supportCode)
    && typeof value.enabled === 'boolean'
    && typeof value.noticeAcknowledged === 'boolean'
    && typeof value.consentEpoch === 'string'
    && /^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/i.test(value.consentEpoch);
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === 'object' && value !== null && !Array.isArray(value);
}
