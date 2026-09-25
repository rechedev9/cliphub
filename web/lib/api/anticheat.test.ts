import assert from 'node:assert/strict';
import test from 'node:test';
import {
  ANTICHEAT_VERDICT,
  AnticheatServiceError,
  anticheatErrorMessage,
  fetchAnticheat,
  fetchDossier,
  isDemoStillIngesting,
  isReviewable,
  startAnticheat,
} from './anticheat.ts';
import { SERVICE_UNAVAILABLE_CODE } from './types.ts';

type FetchCall = { url: string; init?: RequestInit };

/** Installs a fetch stub for one test and restores the original afterwards. */
function stubFetch(handler: (call: FetchCall) => Response | Promise<Response>): { calls: FetchCall[]; restore: () => void } {
  const original = globalThis.fetch;
  const calls: FetchCall[] = [];
  globalThis.fetch = (async (input: string | URL | Request, init?: RequestInit) => {
    const call: FetchCall = { url: String(input), init };
    calls.push(call);
    return handler(call);
  }) as typeof globalThis.fetch;
  return { calls, restore: () => { globalThis.fetch = original; } };
}

function json(body: unknown, status = 200): Response {
  return new Response(JSON.stringify(body), { status, headers: { 'content-type': 'application/json' } });
}

test('only the two review bands unlock the dossier', () => {
  assert.equal(isReviewable(ANTICHEAT_VERDICT.highlyAnomalous), true);
  assert.equal(isReviewable(ANTICHEAT_VERDICT.anomalous), true);
  assert.equal(isReviewable(ANTICHEAT_VERDICT.inconclusive), false);
  assert.equal(isReviewable(ANTICHEAT_VERDICT.clean), false);
  assert.equal(isReviewable(ANTICHEAT_VERDICT.insufficient), false);
});

test('a demo that was never screened reads as null, not as an error', async () => {
  const stub = stubFetch(() => json({ error: 'anticheat analysis not started' }, 409));
  try {
    assert.equal(await fetchAnticheat('job-1'), null);
  } finally {
    stub.restore();
  }
});

test('a stored analysis is returned as-is', async () => {
  const stub = stubFetch(() => json({ status: 'ready', job_id: 'job-1', schema_version: 1, started_at: 'now' }));
  try {
    const doc = await fetchAnticheat('job-1');
    assert.equal(doc?.status, 'ready');
  } finally {
    stub.restore();
  }
});

test('an offline orchestrator surfaces the shared service-unavailable code', async () => {
  const stub = stubFetch(() => json({ error: 'service offline' }, 503));
  try {
    await assert.rejects(fetchAnticheat('job-1'), (err: unknown) => {
      assert.ok(err instanceof AnticheatServiceError);
      assert.equal(err.code, SERVICE_UNAVAILABLE_CODE);
      return true;
    });
  } finally {
    stub.restore();
  }
});

test('a thrown fetch is reported as a service outage rather than crashing the page', async () => {
  const stub = stubFetch(() => {
    throw new Error('ECONNREFUSED');
  });
  try {
    await assert.rejects(fetchAnticheat('job-1'), (err: unknown) => {
      assert.ok(err instanceof AnticheatServiceError);
      assert.equal(err.code, SERVICE_UNAVAILABLE_CODE);
      return true;
    });
  } finally {
    stub.restore();
  }
});

test('starting an analysis posts to the job and returns its status', async () => {
  const stub = stubFetch(() => json({ jobId: 'job-1', status: 'running' }, 202));
  try {
    assert.equal(await startAnticheat('job-1'), 'running');
    assert.equal(stub.calls[0]?.url, '/api/demos/job-1/anticheat');
    assert.equal(stub.calls[0]?.init?.method, 'POST');
  } finally {
    stub.restore();
  }
});

test('an upstream refusal keeps its reason instead of becoming a generic failure', async () => {
  const stub = stubFetch(() => json({ error: 'demo is still being ingested' }, 409));
  try {
    await assert.rejects(startAnticheat('job-1'), (err: unknown) => {
      assert.ok(err instanceof AnticheatServiceError);
      assert.match(err.message, /still being ingested/);
      return true;
    });
  } finally {
    stub.restore();
  }
});

test('the dossier is fetched per player and carries its reporting policy', async () => {
  const stub = stubFetch(() =>
    json({
      steamid64: '76561198012345678',
      name: 'x',
      verdict: 'anomalous',
      score: 70,
      confidence: 0.9,
      markdown: '# expediente',
      channels: [],
      policy: { summary: 's', rules: [], rejected: 'ClipHub no envía denuncias automáticamente' },
    }),
  );
  try {
    const dossier = await fetchDossier('job-1', '76561198012345678');
    assert.equal(stub.calls[0]?.url, '/api/demos/job-1/anticheat/dossier/76561198012345678');
    assert.match(dossier.policy.rejected, /no envía denuncias automáticamente/);
  } finally {
    stub.restore();
  }
});

test('request failures become Spanish copy tied to the action that failed', () => {
  const conflict = new AnticheatServiceError('demo is still being ingested', 'http_409');
  const missing = new AnticheatServiceError('not found', 'http_404');
  const cases: Array<{ name: string; err: unknown; action: 'start' | 'load' | 'dossier'; want: RegExp }> = [
    { name: 'offline', err: new AnticheatServiceError('x', SERVICE_UNAVAILABLE_CODE), action: 'load', want: /no responde/ },
    { name: 'start while ingesting', err: conflict, action: 'start', want: /aún se está importando/ },
    { name: 'dossier before ready', err: conflict, action: 'dossier', want: /aún no ha terminado/ },
    { name: 'dossier unknown player', err: missing, action: 'dossier', want: /no aparece/ },
    { name: 'deleted demo', err: missing, action: 'load', want: /ya no está/ },
    { name: 'unknown error', err: new Error('boom'), action: 'start', want: /No se pudo completar/ },
  ];
  for (const tc of cases) {
    const got = anticheatErrorMessage(tc.err, tc.action);
    assert.match(got, tc.want, tc.name);
    assert.doesNotMatch(got, /ingested|not found|boom/, tc.name);
  }
});

test('only the roster-scan statuses block a screening', () => {
  assert.equal(isDemoStillIngesting('queued'), true);
  assert.equal(isDemoStillIngesting('scanning'), true);
  assert.equal(isDemoStillIngesting('failed'), false);
  assert.equal(isDemoStillIngesting('scanned'), false);
  assert.equal(isDemoStillIngesting(undefined), false);
});
