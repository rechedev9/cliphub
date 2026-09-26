// RealApiClient.scanDemo must hand the UI the reason a scan failed. It used to
// throw a bare "job <id> failed", so a CS:GO demo, a corrupt one and a demo
// newer than the parser all showed the same "prueba con otro .dem".
import assert from 'node:assert/strict';
import test from 'node:test';
import { RealApiClient } from './real.ts';
import { DEMO_SCAN_HINTS, demoScanError } from '../demo-parse-flow.ts';

const JOB = '11111111-2222-4333-8444-555555555555';
const SCAN_URL = '/api/demos/scan';
const STATUS_URL = `/api/demos/${JOB}/status`;

function json(body: unknown, status = 200): Response {
  return new Response(JSON.stringify(body), { status, headers: { 'content-type': 'application/json' } });
}

function stubFetch(reply: (url: string) => Response): () => void {
  const original = globalThis.fetch;
  globalThis.fetch = (async (input: string | URL | Request) => reply(String(input))) as typeof globalThis.fetch;
  return () => {
    globalThis.fetch = original;
  };
}

const demo = () => new File([new Uint8Array([0x50, 0x42, 0x44, 0x45, 0x4d, 0x53, 0x32, 0x00])], 'match.dem');

test('a scan job that fails keeps the worker reason and failure code', async () => {
  const reason = 'scan roster: parsing demo: demo_incompatible: parser panicked: proto: cannot parse invalid wire-format data';
  const restore = stubFetch((url) => {
    if (url === SCAN_URL) return json({ jobId: JOB }, 201);
    if (url === STATUS_URL) return json({ status: 'failed', failure_reason: reason, failure_code: 'demo_incompatible' });
    throw new Error(`unexpected request ${url}`);
  });
  try {
    const err = await new RealApiClient().scanDemo(demo()).then(
      () => assert.fail('scanDemo resolved for a failed job'),
      (e: unknown) => e,
    );
    assert.equal((err as { code?: string }).code, 'demo_incompatible');
    assert.equal((err as Error).message, reason);
    assert.equal(demoScanError(err), DEMO_SCAN_HINTS.incompatible);
  } finally {
    restore();
  }
});

test('an upload the orchestrator rejects keeps its code and HTTP status', async () => {
  const restore = stubFetch((url) => {
    if (url === SCAN_URL) {
      return json({ code: 'csgo_demo', error: 'uploaded file is a CS:GO demo; only CS2 demos are supported' }, 400);
    }
    throw new Error(`unexpected request ${url}`);
  });
  try {
    const err = await new RealApiClient().scanDemo(demo()).then(
      () => assert.fail('scanDemo resolved for a rejected upload'),
      (e: unknown) => e,
    );
    assert.equal((err as { code?: string }).code, 'csgo_demo');
    assert.equal((err as { status?: number }).status, 400);
    assert.equal(demoScanError(err), DEMO_SCAN_HINTS.csgo);
  } finally {
    restore();
  }
});
