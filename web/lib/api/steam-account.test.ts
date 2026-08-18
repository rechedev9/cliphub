import assert from 'node:assert/strict';
import test from 'node:test';
import { loadSteamAccount, saveSteamAccount, STEAM_ACCOUNT_ENDPOINT } from './steam-account.ts';
import { SERVICE_UNAVAILABLE_CODE } from './types.ts';

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

test('loadSteamAccount maps every proxy response shape', async () => {
  const cases = [
    {
      name: 'configured account keeps match ids as strings',
      response: () => json({
        steamId: '76561198000000001',
        authCodeSet: true,
        apiKeySet: true,
        knownCode: 'CSGO-xxxxx',
        historyConfigured: true,
        gcConfigured: false,
        matches: [{ shareCode: 'CSGO-xxxxx', matchId: MATCH_ID }],
      }),
      wantKind: 'ok' as const,
    },
    {
      name: 'offline',
      response: () => json({ error: 'analysis service unavailable', code: SERVICE_UNAVAILABLE_CODE }, 503),
      wantKind: 'offline' as const,
    },
    {
      name: 'invalid account',
      response: () => json({ error: 'steam id is required', code: 'steam_account_invalid' }, 400),
      wantKind: 'failed' as const,
    },
  ];
  for (const tc of cases) {
    const stub = stubFetch(tc.response);
    try {
      const got = await loadSteamAccount();
      assert.equal(got.kind, tc.wantKind, tc.name);
      if (got.kind === 'ok') {
        assert.equal(got.account.matches[0]?.matchId, MATCH_ID);
        assert.notEqual(got.account.matches[0]?.matchId, String(Number(MATCH_ID)));
      }
    } finally {
      stub.restore();
    }
  }
});

test('saveSteamAccount PUTs only the form fields', async () => {
  const stub = stubFetch(() => json({ steamId: '76561198000000001', historyConfigured: true, matches: [] }));
  try {
    await saveSteamAccount({ steamId: '76561198000000001', authCode: 'AAAAA-BBBBB-CCCCC' });
    assert.equal(stub.calls[0]?.url, STEAM_ACCOUNT_ENDPOINT);
    assert.equal(stub.calls[0]?.init?.method, 'PUT');
    assert.deepEqual(JSON.parse(String(stub.calls[0]?.init?.body)), {
      steamId: '76561198000000001',
      authCode: 'AAAAA-BBBBB-CCCCC',
    });
  } finally {
    stub.restore();
  }
});
