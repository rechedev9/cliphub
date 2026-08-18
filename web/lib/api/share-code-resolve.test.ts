import assert from 'node:assert/strict';
import test from 'node:test';
import { SHARE_CODE_ENDPOINT, resolveShareCode, type ShareCodeResolution } from './share-code-resolve.ts';
import { SERVICE_UNAVAILABLE_CODE } from './types.ts';

type FetchCall = { url: string; init?: RequestInit };

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

// 64-bit ids near 3.2e18: unrepresentable as exact JS numbers, kept as strings.
const MATCH_ID = '3230642215713767581';
const OUTCOME_ID = '3230642252279119992';

test('resolveShareCode maps every proxy response shape', async () => {
  const cases: Array<{ name: string; response: () => Response; want: ShareCodeResolution }> = [
    {
      name: 'decoded without a demo url',
      response: () => json({ status: 'decoded', matchId: MATCH_ID, outcomeId: OUTCOME_ID, tokenId: 31463 }),
      want: { kind: 'decoded', matchId: MATCH_ID, outcomeId: OUTCOME_ID, tokenId: 31463, demoUrl: undefined },
    },
    {
      name: 'resolved with a demo url',
      response: () => json({
        status: 'resolved',
        matchId: MATCH_ID,
        outcomeId: OUTCOME_ID,
        tokenId: 31463,
        demoUrl: 'http://replay1.valve.net/730/demo.dem.bz2',
      }),
      want: {
        kind: 'resolved',
        matchId: MATCH_ID,
        outcomeId: OUTCOME_ID,
        tokenId: 31463,
        demoUrl: 'http://replay1.valve.net/730/demo.dem.bz2',
      },
    },
    {
      name: '400 invalid share code carries the upstream message',
      response: () => json({ code: 'invalid_share_code', message: 'ese código no decodifica' }, 400),
      want: { kind: 'invalid', message: 'ese código no decodifica' },
    },
    {
      name: '503 service_unavailable means the local service is down',
      response: () => json({ error: 'analysis service unavailable', code: SERVICE_UNAVAILABLE_CODE }, 503),
      want: { kind: 'offline' },
    },
    {
      name: '502 transport failure maps to failed',
      response: () => json({ error: 'upstream error' }, 502),
      want: { kind: 'failed', message: 'upstream error' },
    },
    {
      name: 'non-JSON 500 still yields a failed message',
      response: () => new Response('boom', { status: 500 }),
      want: { kind: 'failed', message: 'error del servicio (HTTP 500)' },
    },
  ];

  for (const tc of cases) {
    const stub = stubFetch(tc.response);
    try {
      const got = await resolveShareCode('CSGO-GADqf-jjyJ8-cSP2r-smZRo-TO2xK');
      assert.deepEqual(got, tc.want, tc.name);
    } finally {
      stub.restore();
    }
  }
});

test('resolveShareCode POSTs the code to the proxy endpoint', async () => {
  const stub = stubFetch(() => json({ status: 'decoded', matchId: MATCH_ID, outcomeId: OUTCOME_ID, tokenId: 1 }));
  try {
    await resolveShareCode('CSGO-GADqf-jjyJ8-cSP2r-smZRo-TO2xK');
    assert.equal(stub.calls[0]?.url, SHARE_CODE_ENDPOINT);
    assert.equal(stub.calls[0]?.init?.method, 'POST');
    assert.deepEqual(
      JSON.parse(String(stub.calls[0]?.init?.body)),
      { code: 'CSGO-GADqf-jjyJ8-cSP2r-smZRo-TO2xK' },
    );
  } finally {
    stub.restore();
  }
});

test('resolveShareCode keeps the 64-bit ids as exact strings', async () => {
  const stub = stubFetch(() => json({ status: 'decoded', matchId: MATCH_ID, outcomeId: OUTCOME_ID, tokenId: 31463 }));
  try {
    const got = await resolveShareCode('CSGO-GADqf-jjyJ8-cSP2r-smZRo-TO2xK');
    assert.equal(got.kind, 'decoded');
    if (got.kind !== 'decoded') return;
    assert.equal(got.matchId, MATCH_ID);
    assert.notEqual(got.matchId, String(Number(MATCH_ID)));
  } finally {
    stub.restore();
  }
});

test('resolveShareCode reports a thrown fetch as failed', async () => {
  const stub = stubFetch(() => { throw new TypeError('fetch failed'); });
  try {
    const got = await resolveShareCode('CSGO-GADqf-jjyJ8-cSP2r-smZRo-TO2xK');
    assert.equal(got.kind, 'failed');
  } finally {
    stub.restore();
  }
});
