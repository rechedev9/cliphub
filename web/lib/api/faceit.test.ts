import assert from 'node:assert/strict';
import test from 'node:test';
import {
  FACEIT_CODES,
  FaceitServiceError,
  followFaceitPlayer,
  isFaceitPlayerID,
  listFaceitMatches,
  listFollowedFaceitPlayers,
  lookupFaceitPlayer,
  unfollowFaceitPlayer,
} from './faceit.ts';
import { FACEIT_NOT_CONFIGURED_CODE, SERVICE_UNAVAILABLE_CODE } from './types.ts';

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

const player = {
  id: 'player-1',
  nickname: 'm0NESY',
  profile_url: 'https://www.faceit.com/en/players/m0NESY',
  elo: 4000,
  skill_level: 10,
};

test('player id validation', () => {
  const cases: Array<{ id: string; want: boolean }> = [
    { id: 'player-1', want: true },
    { id: 'abc_def', want: true },
    { id: 'bad id', want: false },
    { id: '', want: false },
    { id: '../x', want: false },
  ];
  for (const tc of cases) {
    assert.equal(isFaceitPlayerID(tc.id), tc.want, tc.id);
  }
});

test('lookup encodes the nickname and parses the player', async () => {
  const stub = stubFetch(() => json({ player }));
  try {
    const got = await lookupFaceitPlayer('m0NESY');
    assert.equal(got.nickname, 'm0NESY');
    assert.equal(stub.calls[0]?.url, '/api/faceit/players?nickname=m0NESY');
  } finally {
    stub.restore();
  }
});

test('followed list and follow/unfollow map the proxy surface', async () => {
  const stub = stubFetch((call) => {
    if (call.url === '/api/faceit/followed' && call.init?.method === undefined) {
      return json({
        enabled: true,
        players: [
          player,
          { ...player, id: 'seed-1', nickname: 'donk666', seeded: true, region: 'EU', position: 1 },
        ],
      });
    }
    if (call.url === '/api/faceit/followed' && call.init?.method === 'POST') {
      return json({ player: { ...player, followed_at: '2026-08-17T12:00:00Z' } });
    }
    if (call.url === '/api/faceit/followed/player-1' && call.init?.method === 'DELETE') {
      return new Response(null, { status: 204 });
    }
    return json({ error: 'unexpected' }, 500);
  });
  try {
    const listed = await listFollowedFaceitPlayers();
    assert.equal(listed.enabled, true);
    assert.equal(listed.players[0]?.id, 'player-1');
    assert.equal(listed.players[1]?.seeded, true);
    assert.equal(listed.players[1]?.region, 'EU');
    assert.equal(listed.players[1]?.position, 1);
    const followed = await followFaceitPlayer('m0NESY');
    assert.equal(followed.followed_at, '2026-08-17T12:00:00Z');
    await unfollowFaceitPlayer('player-1');
  } finally {
    stub.restore();
  }
});

test('match list keeps score and stats', async () => {
  const stub = stubFetch(() => json({
    matches: [{
      id: 'match-1',
      room_url: 'https://www.faceit.com/en/cs2/room/match-1',
      score: { for: 13, against: 8 },
      stats: { map: 'de_mirage', result: 'win', kills: 20, deaths: 10, assists: 4, adr: 90 },
    }],
  }));
  try {
    const matches = await listFaceitMatches('player-1', 10);
    assert.equal(matches[0]?.stats?.result, 'win');
    assert.equal(matches[0]?.score.for, 13);
  } finally {
    stub.restore();
  }
});

test('error codes stay stable across the proxy', async () => {
  const cases: Array<{ status: number; body: unknown; code: string }> = [
    { status: 503, body: { error: 'offline', code: SERVICE_UNAVAILABLE_CODE }, code: SERVICE_UNAVAILABLE_CODE },
    { status: 503, body: { error: 'missing key', code: FACEIT_NOT_CONFIGURED_CODE }, code: FACEIT_NOT_CONFIGURED_CODE },
    { status: 429, body: { error: 'slow down', code: FACEIT_CODES.rateLimited }, code: FACEIT_CODES.rateLimited },
  ];
  for (const tc of cases) {
    const stub = stubFetch(() => json(tc.body, tc.status));
    try {
      await lookupFaceitPlayer('m0NESY');
      assert.fail('expected error');
    } catch (err) {
      assert.ok(err instanceof FaceitServiceError);
      assert.equal(err.code, tc.code);
    } finally {
      stub.restore();
    }
  }
});
