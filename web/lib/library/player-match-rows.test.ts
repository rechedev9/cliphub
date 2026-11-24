import test from 'node:test';
import assert from 'node:assert/strict';
import type { LibraryMatch, LibraryMatchTeam } from '../api/library.ts';
import { playerMatchRows, unreadMatchesText } from './player-match-rows.ts';

const PLAYER = '76561198000000001';

function side(key: LibraryMatchTeam['key'], score: number, name: string): LibraryMatchTeam {
  return { key, start_side: key === 'ct-start' ? 'CT' : 'T', clan_name: name, team_id: null, assignment: 'none', score };
}

function match(id: string, ct: number, t: number, withPlayer = true): LibraryMatch {
  return {
    id, demo_id: `demo-${id}`, map: 'de_mirage', played_at: '2026-10-02T18:00:00.000Z', source: 'faceit', tickrate: 64,
    duration_ticks: 1, round_count: ct + t, teams: [side('ct-start', ct, 'Vitality'), side('t-start', t, 'MOUZ')],
    players: withPlayer
      ? [{ steamid64: PLAYER, name: 'apEX', team_key: 'ct-start', slot: 0, rounds: ct + t, kills: 20, deaths: 15, assists: 3, damage: 2000 }]
      : [],
    created_at: '', updated_at: '',
  };
}

function rejected(status: number): PromiseRejectedResult {
  return { status: 'rejected', reason: Object.assign(new Error('x'), { status }) };
}

test('matches that loaded become rows and a missing or failed match is counted, not fatal', () => {
  const out = playerMatchRows({
    steamId: PLAYER,
    results: [
      { status: 'fulfilled', value: match('won', 13, 7) },
      { status: 'fulfilled', value: match('lost', 5, 13) },
      { status: 'fulfilled', value: match('absent', 13, 2, false) },
      rejected(404),
      rejected(500),
      rejected(500),
    ],
  });
  assert.deepEqual(out.rows.map((row) => [row.match.id, row.outcome, row.teamName, row.rivalName]), [
    ['won', 'win', 'Vitality', 'MOUZ'],
    ['lost', 'loss', 'Vitality', 'MOUZ'],
  ]);
  assert.equal(out.lines.length, 2);
  assert.equal(out.gone, 1);
  assert.equal(out.failed, 2);
});

test('the note says how many matches are left out and why', () => {
  assert.equal(unreadMatchesText({ gone: 0, failed: 0 }), '');
  assert.equal(unreadMatchesText({ gone: 1, failed: 0 }), '1 partida ya no está en la biblioteca. Las cifras cuentan solo las demás.');
  assert.equal(
    unreadMatchesText({ gone: 2, failed: 3 }),
    '2 partidas ya no están en la biblioteca y no se pudieron leer 3 partidas. Las cifras cuentan solo las demás.',
  );
  assert.equal(unreadMatchesText({ gone: 0, failed: 1 }), 'No se pudo leer 1 partida. Las cifras cuentan solo las demás.');
});
