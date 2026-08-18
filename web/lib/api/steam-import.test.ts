import assert from 'node:assert/strict';
import test from 'node:test';
import { importShareCode, STEAM_IMPORT_ENDPOINT } from './steam-import.ts';
import { SERVICE_UNAVAILABLE_CODE, STEAM_CODES } from './types.ts';

type FetchCall = { url: string; init?: RequestInit };

function stubFetch(handler: (call: FetchCall) => Response): { calls: FetchCall[]; restore: () => void } {
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

const MATCH_ID = '3230642215713767581';

test('importShareCode maps every proxy response shape', async () => {
  const cases = [
    {
      name: 'queued',
      response: () => json({ id: 'job-1', status: 'queued', matchId: MATCH_ID }, 201),
      want: { kind: 'queued' as const, id: 'job-1', status: 'queued', matchId: MATCH_ID },
    },
    {
      name: 'needs credentials',
      response: () => json({ code: STEAM_CODES.credentialsRequired, error: 'Steam login is required' }, 409),
      want: { kind: 'needCredentials' as const },
    },
    {
      name: 'demo unavailable',
      response: () => json({ code: STEAM_CODES.demoUnavailable, error: 'expired' }, 409),
      want: { kind: 'unavailable' as const, message: 'expired' },
    },
    {
      name: 'offline',
      response: () => json({ error: 'analysis service unavailable', code: SERVICE_UNAVAILABLE_CODE }, 503),
      want: { kind: 'offline' as const },
    },
  ];
  for (const tc of cases) {
    const stub = stubFetch(tc.response);
    try {
      const got = await importShareCode('CSGO-GADqf-jjyJ8-cSP2r-smZRo-TO2xK');
      assert.deepEqual(got, tc.want, tc.name);
    } finally {
      stub.restore();
    }
  }
});

test('importShareCode posts credentials only when given', async () => {
  const stub = stubFetch(() => json({ id: 'job-1', status: 'queued', matchId: MATCH_ID }, 201));
  try {
    await importShareCode('CSGO-GADqf-jjyJ8-cSP2r-smZRo-TO2xK', {
      username: 'user',
      password: 'secret',
      guard: '12345',
    });
    assert.equal(stub.calls[0]?.url, STEAM_IMPORT_ENDPOINT);
    assert.deepEqual(JSON.parse(String(stub.calls[0]?.init?.body)), {
      code: 'CSGO-GADqf-jjyJ8-cSP2r-smZRo-TO2xK',
      username: 'user',
      password: 'secret',
      guard: '12345',
    });
  } finally {
    stub.restore();
  }
});
