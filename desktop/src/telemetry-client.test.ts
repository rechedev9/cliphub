import assert from 'node:assert/strict';
import * as fs from 'node:fs';
import * as os from 'node:os';
import * as path from 'node:path';
import test from 'node:test';
import { TelemetryClient } from './telemetry-client.ts';
import { TelemetrySettingsStore } from './telemetry-settings.ts';

function fixture(responseStatus = 202, release = '2.4.35'): {
  client: TelemetryClient;
  queuePath: string;
  requests: Array<{ body: string; headers: Record<string, string> }>;
} {
  const directory = fs.mkdtempSync(path.join(os.tmpdir(), 'cliphub-telemetry-client-'));
  const settings = new TelemetrySettingsStore(path.join(directory, 'settings.json'));
  const queuePath = path.join(directory, 'queue.json');
  const requests: Array<{ body: string; headers: Record<string, string> }> = [];
  const client = new TelemetryClient({
    settings,
    queuePath,
    release,
    config: { endpoint: 'https://collector.example/v1/ingest', ingestKey: 'public-ingest-key-with-24-characters' },
    fetch: async (_input, init) => {
      requests.push({ body: init.body, headers: init.headers });
      return { ok: responseStatus >= 200 && responseStatus < 300, status: responseStatus };
    },
    log: () => {},
  });
  return { client, queuePath, requests };
}

test('waits for the informed choice and uploads only fixed error labels', async () => {
  const { client, queuePath, requests } = fixture();
  const event = {
    component: 'electron',
    name: 'desktop.boot_failed',
    stage: 'boot',
    class: 'boot_failed',
  };
  client.recordError(event);
  assert.equal(fs.existsSync(queuePath), false);

  client.update(true);
  client.recordError(event);
  await client.flush();

  assert.equal(requests.length, 1);
  assert.equal(requests[0].headers['X-ClipHub-Ingest-Key'], 'public-ingest-key-with-24-characters');
  const posted = requests[0].body;
  assert.match(posted, /desktop\.boot_failed/);
  assert.doesNotMatch(posted, /summary|fingerprint|Users|7656119|token/);
  assert.equal(JSON.parse(fs.readFileSync(queuePath, 'utf8')).events.length, 0);
});

test('normalizes prerelease versions to the collector release contract', async () => {
  const { client, requests } = fixture(202, '2.4.35-beta.1+local');
  client.update(true);
  client.recordError({ component: 'renderer', name: 'route.error', stage: 'renderer', class: 'exception' });
  await client.flush();
  assert.equal(JSON.parse(requests[0].body).events[0].release, '2.4.35');
});

test('uses a valid sentinel for malformed app versions', async () => {
  const { client, requests } = fixture(202, 'development');
  client.update(true);
  client.recordError({ component: 'renderer', name: 'route.error', stage: 'renderer', class: 'exception' });
  await client.flush();
  assert.equal(JSON.parse(requests[0].body).events[0].release, '0.0.0');
});

test('disabling diagnostics aborts an in-flight upload before clearing the queue', async () => {
  const directory = fs.mkdtempSync(path.join(os.tmpdir(), 'cliphub-telemetry-revoke-'));
  const settings = new TelemetrySettingsStore(path.join(directory, 'settings.json'));
  const queuePath = path.join(directory, 'queue.json');
  let aborted = false;
  const client = new TelemetryClient({
    settings,
    queuePath,
    release: '2.4.35',
    config: { endpoint: 'https://collector.example/v1/ingest', ingestKey: 'public-ingest-key-with-24-characters' },
    fetch: async (_input, init) => new Promise((_resolve, reject) => {
      init.signal.addEventListener('abort', () => {
        aborted = true;
        reject(new Error('aborted'));
      }, { once: true });
    }),
    log: () => {},
  });
  client.update(true);
  client.recordError({ component: 'renderer', name: 'route.error', stage: 'renderer', class: 'exception' });
  const flushing = client.flush();
  await new Promise((resolve) => setImmediate(resolve));
  client.update(false);
  await flushing;
  assert.equal(aborted, true);
  assert.equal(fs.existsSync(queuePath), false);
});

test('a stale queue cannot upload after revocation even when deletion fails', async () => {
  const directory = fs.mkdtempSync(path.join(os.tmpdir(), 'cliphub-telemetry-epoch-'));
  const settings = new TelemetrySettingsStore(path.join(directory, 'settings.json'));
  const queuePath = path.join(directory, 'queue.json');
  let fetches = 0;
  const client = new TelemetryClient({
    settings,
    queuePath,
    release: '2.4.35',
    config: { endpoint: 'https://collector.example/v1/ingest', ingestKey: 'public-ingest-key-with-24-characters' },
    fetch: async () => {
      fetches++;
      return { ok: true, status: 202 };
    },
    removeQueue: () => { throw new Error('locked by antivirus'); },
    log: () => {},
  });
  client.update(true);
  client.recordError({ component: 'renderer', name: 'route.error', stage: 'renderer', class: 'exception' });
  assert.equal(fs.existsSync(queuePath), true);
  client.update(false);
  client.update(true);
  await client.flush();
  assert.equal(fetches, 0);
  assert.equal(fs.existsSync(queuePath), true);
});

test('disabling diagnostics immediately deletes the pending queue', () => {
  const { client, queuePath } = fixture();
  client.update(true);
  client.recordError({ component: 'renderer', name: 'route.error', stage: 'renderer', class: 'exception' });
  assert.equal(fs.existsSync(queuePath), true);
  client.update(false);
  assert.equal(fs.existsSync(queuePath), false);
});

test('isolates a rejected batch before discarding only its poison event', async () => {
  const directory = fs.mkdtempSync(path.join(os.tmpdir(), 'cliphub-telemetry-isolate-'));
  const settings = new TelemetrySettingsStore(path.join(directory, 'settings.json'));
  const queuePath = path.join(directory, 'queue.json');
  const batchClasses: string[][] = [];
  const client = new TelemetryClient({
    settings,
    queuePath,
    release: '2.4.35',
    config: { endpoint: 'https://collector.example/v1/ingest', ingestKey: 'public-ingest-key-with-24-characters' },
    fetch: async (_input, init) => {
      const events = JSON.parse(init.body).events as Array<{ class: string }>;
      batchClasses.push(events.map((event) => event.class));
      const rejected = events.length > 1 || events[0]?.class === 'poison';
      return { ok: !rejected, status: rejected ? 422 : 202 };
    },
    log: () => {},
  });
  client.update(true);
  client.recordError({ component: 'renderer', name: 'route.error', stage: 'renderer', class: 'valid' });
  client.recordError({ component: 'renderer', name: 'route.error', stage: 'renderer', class: 'poison' });
  await client.flush();
  for (let attempt = 0; attempt < 20 && batchClasses.length < 3; attempt++) {
    await new Promise((resolve) => setTimeout(resolve, 5));
  }
  assert.deepEqual(batchClasses, [['valid', 'poison'], ['valid'], ['poison']]);
  assert.equal(JSON.parse(fs.readFileSync(queuePath, 'utf8')).events.length, 0);
});

test('discards an isolated poison event rejected by the collector schema', async () => {
  const { client, queuePath } = fixture(422);
  client.update(true);
  client.recordError({ component: 'renderer', name: 'route.error', stage: 'renderer', class: 'exception' });
  await client.flush();
  assert.equal(JSON.parse(fs.readFileSync(queuePath, 'utf8')).events.length, 0);
});
