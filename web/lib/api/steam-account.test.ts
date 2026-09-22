import assert from 'node:assert/strict';
import test from 'node:test';
import {
  isValidShareCode,
  loadSteamAccount,
  parseSteamId64,
  saveSteamAccount,
  STEAM_ACCOUNT_ENDPOINT,
  STEAM_FIELD_ERROR,
  STEAM_NETWORK_ERROR_CODE,
  steamAccountFailure,
  validateSteamAccountInput,
} from './steam-account.ts';
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

// Vectors copied from internal/steamresolve/account_test.go TestParseSteamID.
test('parseSteamId64 mirrors steamresolve.ParseSteamID', () => {
  const cases: Array<{ raw: string; want: string | null }> = [
    { raw: '76561198000000001', want: '76561198000000001' },
    { raw: '  76561198000000001 ', want: '76561198000000001' },
    { raw: 'https://steamcommunity.com/profiles/76561198000000001', want: '76561198000000001' },
    { raw: 'https://steamcommunity.com/profiles/76561198000000001/', want: '76561198000000001' },
    { raw: 'https://steamcommunity.com/profiles/76561198000000001?l=spanish', want: '76561198000000001' },
    { raw: '  ', want: null },
    { raw: 'https://steamcommunity.com/id/someone', want: null },
    { raw: '7656119', want: null },
    { raw: '12345678901234567', want: null },
    { raw: '7656119800000000a', want: null },
  ];
  for (const { raw, want } of cases) {
    assert.equal(parseSteamId64(raw), want, raw);
  }
});

// Vectors copied from internal/sharecode/sharecode_test.go.
test('isValidShareCode mirrors sharecode.Decode', () => {
  const cases: Array<{ code: string; want: boolean }> = [
    { code: 'CSGO-GADqf-jjyJ8-cSP2r-smZRo-TO2xK', want: true },
    { code: 'GADqf-jjyJ8-cSP2r-smZRo-TO2xK', want: true },
    { code: '', want: false },
    { code: 'CSGO-GADqf-jjyJ8-cSP2r-smZRo-TO2x', want: false },
    { code: 'CSGO-GADqf-jjyJ8-cSP2r-smZRo-TO2xKA', want: false },
    { code: 'CSGO-GADqf-jjyJ8-cSP2r-smZRo-TO2xI', want: false },
    { code: 'CSGO-GADqf-jjyJ8-cSP2r-smZRo-TO2xg', want: false },
    { code: 'CSGO-GADqf-jjyJ8-cSP2r-smZRo-TO2xl', want: false },
    { code: 'CSGO-GADqf-jjyJ8-cSP2r-smZRo-TO2x0', want: false },
    { code: 'CSGO-GADqf-jjyJ8-cSP2r-smZRo-TO2x1', want: false },
    { code: 'CSG-OGADqf-jjyJ8-cSP2r-smZRo-TO2xK', want: false },
    { code: 'CSGO-99999-99999-99999-99999-99999', want: false },
  ];
  for (const { code, want } of cases) {
    assert.equal(isValidShareCode(code), want, code);
  }
});

test('validateSteamAccountInput flags each field the server would reject', () => {
  const none = null;
  const saved = { steamId: '76561198000000001' };
  const cases: Array<{ name: string; input: Parameters<typeof validateSteamAccountInput>[0]; stored: { steamId: string } | null; want: Record<string, string> }> = [
    {
      name: 'an empty form with nothing saved needs a SteamID64',
      input: { steamId: '', authCode: '', apiKey: '', knownCode: '' },
      stored: none,
      want: { steamId: STEAM_FIELD_ERROR.steamIdMissing },
    },
    {
      name: 'an empty SteamID64 keeps the saved one',
      input: { steamId: '  ', authCode: '', apiKey: '', knownCode: '' },
      stored: saved,
      want: {},
    },
    {
      name: 'the e2e credentials are valid',
      input: { steamId: '76561198000000001', authCode: 'AAAAA-BBBBB-CCCCC', apiKey: '0123456789ABCDEF', knownCode: '' },
      stored: none,
      want: {},
    },
    {
      name: 'every field malformed',
      input: { steamId: 'someone', authCode: 'abcd', apiKey: 'short', knownCode: 'CSGO-abc' },
      stored: none,
      want: {
        steamId: STEAM_FIELD_ERROR.steamIdInvalid,
        authCode: STEAM_FIELD_ERROR.authCode,
        apiKey: STEAM_FIELD_ERROR.apiKey,
        knownCode: STEAM_FIELD_ERROR.knownCode,
      },
    },
    {
      name: 'upper length bounds',
      input: { steamId: saved.steamId, authCode: 'a'.repeat(65), apiKey: 'k'.repeat(65) },
      stored: none,
      want: { authCode: STEAM_FIELD_ERROR.authCode, apiKey: STEAM_FIELD_ERROR.apiKey },
    },
    {
      name: 'a profiles URL and a valid share code pass',
      input: { steamId: 'https://steamcommunity.com/profiles/76561198000000001/', knownCode: 'CSGO-GADqf-jjyJ8-cSP2r-smZRo-TO2xK' },
      stored: none,
      want: {},
    },
  ];
  for (const tc of cases) {
    assert.deepEqual(validateSteamAccountInput(tc.input, tc.stored), tc.want, tc.name);
  }
});

test('steamAccountFailure never shows the orchestrator text and names the field it can', () => {
  const cases: Array<{ name: string; failure: { message: string; code?: string }; action: 'load' | 'save' | 'sync' | 'clear'; field?: string; want: RegExp }> = [
    { name: 'bad steam id', failure: { message: 'steam id "x" is not a 64-bit SteamID or profiles URL', code: 'steam_account_invalid' }, action: 'save', field: 'steamId', want: /SteamID64 válido/ },
    { name: 'bad auth code', failure: { message: 'authentication code must be between 5 and 64 characters', code: 'steam_account_invalid' }, action: 'save', field: 'authCode', want: /código de autenticación/ },
    { name: 'bad api key', failure: { message: 'steam web api key must be between 8 and 64 characters', code: 'steam_account_invalid' }, action: 'save', field: 'apiKey', want: /Web API/ },
    { name: 'bad known code', failure: { message: 'known share code: share code "x": expected 25 characters', code: 'steam_account_invalid' }, action: 'save', field: 'knownCode', want: /CSGO-/ },
    { name: 'sync before configuring', failure: { message: 'Connect a Steam authentication code before syncing matches', code: 'history_not_configured' }, action: 'sync', want: /Guarda primero/ },
    { name: 'sync without a known code', failure: { message: 'Pega primero un código', code: 'need_known_code' }, action: 'sync', field: 'knownCode', want: /código de partida conocido/ },
    { name: 'steam down', failure: { message: 'Steam match history request failed', code: 'steam_history_unavailable' }, action: 'sync', want: /Steam no respondió/ },
    { name: 'network', failure: { message: 'no se pudo contactar', code: STEAM_NETWORK_ERROR_CODE }, action: 'clear', want: /servicio local/ },
    { name: 'unknown save failure', failure: { message: 'invalid steam account JSON' }, action: 'save', want: /No se pudo guardar/ },
  ];
  for (const tc of cases) {
    const got = steamAccountFailure(tc.failure, tc.action);
    assert.equal(got.field, tc.field, tc.name);
    assert.match(got.message, tc.want, tc.name);
    assert.notEqual(got.message, tc.failure.message, tc.name);
  }
});
