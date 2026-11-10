import test from 'node:test';
import assert from 'node:assert/strict';
import { parseBotReplayResult, sidePlayers } from './bot-replay.ts';
import { LIBRARY_ROUTES, matchLibraryRoute } from '../../app/api/library/_routes.ts';

const BODY = {
  launched: true,
  already_running: false,
  cfg_path: 'C:\\cs2\\cliphub_practice.cfg',
  exec_command: 'exec cliphub_practice',
  map: 'de_mirage',
  round: 3,
  side: 'CT',
  bot_side: 'T',
  keys: { prepare: 'KP_INS', restart: 'KP_ENTER', next: 'KP_PLUS', start: 'KP_MINUS', base: 'KP_DEL' },
  bots: [{ name: 'carl', steamid64: '3', index: 2 }],
  starts: [{ name: 'ana', steamid64: '1' }, { name: 'bea', steamid64: '2' }],
  current: 1,
  moments: [{ tick: 1000, seconds: 0, label: 'Salida', notes: [] }, { tick: 1600, seconds: 9.4, label: 'Granadas', notes: ['Humo de carl', 7] }],
  lineups: [{ seconds: 9.4, kind: 'smoke', player: 'carl', key: 'KP_END' }],
  limits: ['Los bots no repiten el recorrido.'],
};

test('parseBotReplayResult reads the answer and the player the user starts as', () => {
  const result = parseBotReplayResult(BODY);
  assert.ok(result);
  assert.equal(result.startsAs, 'bea');
  assert.equal(result.keys.next, 'KP_PLUS');
  assert.equal(result.keys.base, 'KP_DEL');
  assert.deepEqual(result.moments[1], { seconds: 9.4, label: 'Granadas', notes: ['Humo de carl'] });
  assert.deepEqual(result.bots, [{ name: 'carl', steamid64: '3' }]);
  assert.equal(result.limits.length, 1);
});

test('sidePlayers lists the players of one side in slot order', () => {
  const doc = { players: [{ slot: 0, steamid64: '1', name: 'ana', team_key: 'a', start_side: 'CT' as const }, { slot: 2, steamid64: '3', name: 'carl', team_key: 'b', start_side: 'T' as const }, { slot: 1, steamid64: '2', name: 'bea', team_key: 'a', start_side: 'CT' as const }] };
  const base = { kills: 0, deaths: 0, assists: 0, damage: 0, equip_value: 0, money: 0, survived: true };
  const round = { players: [{ ...base, slot: 1, side: 'CT' as const }, { ...base, slot: 2, side: 'T' as const }, { ...base, slot: 0, side: 'CT' as const }] };
  assert.deepEqual(sidePlayers({ doc, round, side: 'CT' }).map((player) => player.name), ['ana', 'bea']);
  assert.deepEqual(sidePlayers({ doc, round, side: 'T' }), [{ name: 'carl', steamid64: '3' }]);
});

test('parseBotReplayResult refuses an answer without the command or moments', () => {
  assert.equal(parseBotReplayResult(null), null);
  assert.equal(parseBotReplayResult({ ...BODY, exec_command: 1 }), null);
  assert.equal(parseBotReplayResult({ ...BODY, moments: 'x' }), null);
  assert.equal(parseBotReplayResult({ ...BODY, keys: null }), null);
});

test('the library proxy allows the bot replay POST with its fields and keys', () => {
  const route = matchLibraryRoute('POST', ['matches', '9be6d5a6-abda-4110-908d-deae67dee533', 'tactical', 'rounds', '3', 'bot-replay']);
  assert.ok(route);
  assert.deepEqual(route.body, { kind: 'json', fields: ['side', 'replace_steamid'] });
  assert.ok(route.response.kind === 'json' && route.response.keys.includes('limits'));
  assert.ok(LIBRARY_ROUTES.includes(route));
});
