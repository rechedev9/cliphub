import assert from 'node:assert/strict';
import test from 'node:test';
import { getFaceitScoreboard, parseFaceitScoreboard } from './faceit-scoreboard.ts';

const upstream = {
  scoreboard: {
    match_id: '1-2009a7d4-6c68-46e1-aebc-46629bd92649',
    room_url: 'https://www.faceit.com/en/cs2/room/1-2009a7d4-6c68-46e1-aebc-46629bd92649',
    map: 'de_mirage',
    rounds: 20,
    teams: [
      {
        name: 'team_Magnojezzz',
        score: 13,
        first_half: 7,
        second_half: 6,
        overtime: 0,
        won: true,
        average_elo: 3835,
        players: [
          {
            player_id: 'b0bf2cc2',
            nickname: 'Magnojezzz',
            steam_id64: '76561198372023214',
            avatar: 'https://distribution.faceit-cdn.net/images/magno.jpeg',
            skill_level: 10,
            elo: 4271,
            kills: 14, deaths: 14, assists: 10, headshots: 11, hs_pct: 78.6, adr: 86, kd: 1, kr: 0.7, mvps: 4,
            double_kills: 2, triple_kills: 1, quadro_kills: 0, penta_kills: 0,
            api_key: 'must not pass',
          },
        ],
      },
    ],
  },
};

test('parseFaceitScoreboard reshapes the upstream body and drops unknown keys', () => {
  const board = parseFaceitScoreboard(upstream);
  assert.ok(board);
  assert.equal(board.teams[0].averageElo, 3835);
  assert.equal(board.teams[0].firstHalf, 7);
  const player = board.teams[0].players[0];
  assert.deepEqual(
    { steamId: player.steamId, elo: player.elo, hsPct: player.hsPct, rounds3k: player.rounds3k, rounds2k: player.rounds2k },
    { steamId: '76561198372023214', elo: 4271, hsPct: 78.6, rounds3k: 1, rounds2k: 2 },
  );
  assert.equal(JSON.stringify(board).includes('must not pass'), false);
});

test('parseFaceitScoreboard rejects a body that cannot drive the picker', () => {
  assert.equal(parseFaceitScoreboard({}), null);
  assert.equal(parseFaceitScoreboard({ scoreboard: { ...upstream.scoreboard, room_url: 'https://evil.example/room' } }), null);
  assert.equal(parseFaceitScoreboard({ scoreboard: { ...upstream.scoreboard, teams: [] } }), null);
  assert.equal(
    parseFaceitScoreboard({ scoreboard: { ...upstream.scoreboard, teams: [{ players: [{ nickname: 'no id' }] }] } }),
    null,
  );
});

test('parseFaceitScoreboard leaves an unresolved ELO or a malformed Steam ID unset', () => {
  const [team] = upstream.scoreboard.teams;
  const player = { ...team.players[0], elo: undefined, steam_id64: 'not-a-steam-id' };
  const board = parseFaceitScoreboard({ scoreboard: { ...upstream.scoreboard, teams: [{ ...team, average_elo: undefined, players: [player] }] } });
  assert.ok(board);
  assert.equal(board.teams[0].averageElo, undefined);
  assert.equal(board.teams[0].players[0].elo, undefined);
  assert.equal(board.teams[0].players[0].steamId, undefined);
});

async function withFetch<T>(impl: typeof fetch, run: () => Promise<T>): Promise<T> {
  const original = globalThis.fetch;
  globalThis.fetch = impl;
  try {
    return await run();
  } finally {
    globalThis.fetch = original;
  }
}

test('getFaceitScoreboard falls back to null so the picker keeps the demo tables', async () => {
  const jobId = '8019aa74-60cc-48dc-8334-4f2d017a80b6';
  const cases: Array<[string, typeof fetch]> = [
    ['not a FACEIT demo', async () => Response.json({ code: 'not_faceit_demo' }, { status: 404 })],
    ['FACEIT unavailable', async () => Response.json({ code: 'faceit_unavailable' }, { status: 502 })],
    ['network failure', async () => { throw new TypeError('fetch failed'); }],
    ['unexpected body', async () => Response.json({ scoreboard: { teams: [] } })],
  ];
  for (const [name, impl] of cases) {
    assert.equal(await withFetch(impl, () => getFaceitScoreboard(jobId)), null, name);
  }
  const served = parseFaceitScoreboard(upstream);
  const board = await withFetch(async () => Response.json({ scoreboard: served }), () => getFaceitScoreboard(jobId));
  assert.deepEqual(board, served);
});
