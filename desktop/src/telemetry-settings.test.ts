import assert from 'node:assert/strict';
import * as fs from 'node:fs';
import * as os from 'node:os';
import * as path from 'node:path';
import test from 'node:test';
import { TelemetrySettingsStore } from './telemetry-settings.ts';

test('creates an enabled but unacknowledged pseudonymous installation', () => {
  const directory = fs.mkdtempSync(path.join(os.tmpdir(), 'cliphub-telemetry-settings-'));
  const file = path.join(directory, 'telemetry.json');
  const store = new TelemetrySettingsStore(file);
  const status = store.get();

  assert.equal(status.enabled, true);
  assert.equal(status.noticeAcknowledged, false);
  assert.match(status.supportCode, /^CH(?:-[A-F0-9]{4}){5}$/);
  assert.equal(store.eligible(), false);

  assert.deepEqual(store.update(false), { ...status, enabled: false, noticeAcknowledged: true });
  assert.equal(store.eligible(), false);
  assert.equal(new TelemetrySettingsStore(file).get().supportCode, status.supportCode);
});

test('each consent change rotates a persisted queue epoch', () => {
  const directory = fs.mkdtempSync(path.join(os.tmpdir(), 'cliphub-telemetry-consent-'));
  const file = path.join(directory, 'telemetry.json');
  const store = new TelemetrySettingsStore(file);
  const initial = store.consentEpoch();
  store.update(false);
  const revoked = store.consentEpoch();
  assert.notEqual(revoked, initial);
  assert.equal(new TelemetrySettingsStore(file).consentEpoch(), revoked);
  store.update(true);
  assert.notEqual(store.consentEpoch(), revoked);
});

test('support code stays stable on disk and carries 80 random bits', () => {
  const directory = fs.mkdtempSync(path.join(os.tmpdir(), 'cliphub-telemetry-code-'));
  const file = path.join(directory, 'telemetry.json');
  const first = new TelemetrySettingsStore(file).get().supportCode;
  const second = new TelemetrySettingsStore(file).get().supportCode;
  assert.equal(second, first);
  assert.match(first, /^CH(?:-[A-F0-9]{4}){5}$/);
});
