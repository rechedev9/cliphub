import test from 'node:test';
import assert from 'node:assert/strict';
import type { LibraryDemo, LibraryMatch, LibraryMatchPlayer, LibraryMatchTeam } from '../api/library.ts';
import {
  adrLabel,
  buildPartidaRows,
  inDateRange,
  kdLabel,
  mapPerformance,
  matchScoreLabel,
  matchTitle,
  partidaDate,
  playerTotals,
  teamFilterOptions,
  teamLabel,
  teamOutcome,
  utilityRatioLabel,
} from './partidas.ts';

const VIT = 'aaaaaaaa-0000-4000-8000-000000000001';
const MOUZ = 'aaaaaaaa-0000-4000-8000-000000000002';

function demo(id: string, extra: Partial<LibraryDemo> = {}): LibraryDemo {
  return {
    id, sha256: id, file_name: `${id}.dem`, path: `library/${id}/demo.dem`, size_bytes: 1, source: 'unknown',
    source_manual: false, state: 'ready', tags: [], job_ids: [], imported_at: '2026-10-01T10:00:00.000Z',
    updated_at: '2026-10-01T10:00:00.000Z', ...extra,
  };
}

function side(key: LibraryMatchTeam['key'], score: number, teamId: string | null, name = ''): LibraryMatchTeam {
  return {
    key, start_side: key === 'ct-start' ? 'CT' : 'T', clan_name: name, team_id: teamId,
    team_name: teamId === null ? undefined : name, assignment: teamId === null ? 'none' : 'auto', score,
  };
}

function match(id: string, map: string, ct: LibraryMatchTeam, t: LibraryMatchTeam): LibraryMatch {
  return {
    id, demo_id: `demo-${id}`, map, played_at: '2026-10-02T18:00:00.000Z', source: 'faceit', tickrate: 64,
    duration_ticks: 1, round_count: ct.score + t.score, teams: [ct, t], created_at: '', updated_at: '',
  };
}

test('a partida row joins a demo to its match and falls back to the import date', () => {
  const m = match('m1', 'de_mirage', side('ct-start', 13, VIT, 'Vitality'), side('t-start', 7, null));
  const rows = buildPartidaRows([demo('demo-m1'), demo('queued', { state: 'queued', played_at: '2026-09-30T00:00:00.000Z' })], [m]);
  const [played, queued] = rows;
  assert.ok(played && queued);
  assert.equal(played.match?.id, 'm1');
  assert.equal(queued.match, null);
  assert.equal(partidaDate(played), '2026-10-02T18:00:00.000Z');
  assert.equal(partidaDate(queued), '2026-09-30T00:00:00.000Z');
  assert.equal(partidaDate({ demo: demo('x'), match: null }), '2026-10-01T10:00:00.000Z');
});

test('teams are named by the library, then the clan, then the starting side', () => {
  const m = match('m1', 'de_mirage', side('ct-start', 13, VIT, 'Vitality'), side('t-start', 7, null));
  assert.equal(matchTitle(m), 'Vitality vs Equipo T');
  assert.equal(matchScoreLabel(m), '13-7');
  assert.equal(teamLabel({ ...side('t-start', 0, null), clan_name: 'team_xKacpersky' }), 'team_xKacpersky');
  assert.equal(teamLabel(undefined), 'Sin equipo');
});

test('a team outcome reads its own score against the rival', () => {
  const m = match('m1', 'de_mirage', side('ct-start', 13, VIT, 'Vitality'), side('t-start', 7, MOUZ, 'MOUZ'));
  assert.equal(teamOutcome(m, VIT), 'win');
  assert.equal(teamOutcome(m, MOUZ), 'loss');
  assert.equal(teamOutcome(match('m2', 'de_nuke', side('ct-start', 15, VIT), side('t-start', 15, MOUZ)), VIT), 'draw');
  assert.equal(teamOutcome(m, 'other'), null);
});

test('map performance sums each map once per match, most played first', () => {
  const matches = [
    match('m1', 'de_mirage', side('ct-start', 13, VIT, 'Vitality'), side('t-start', 7, MOUZ, 'MOUZ')),
    match('m2', 'de_mirage', side('ct-start', 9, MOUZ, 'MOUZ'), side('t-start', 13, VIT, 'Vitality')),
    match('m3', 'de_mirage', side('ct-start', 13, MOUZ, 'MOUZ'), side('t-start', 11, VIT, 'Vitality')),
    match('m4', 'de_ancient', side('ct-start', 13, VIT, 'Vitality'), side('t-start', 2, null)),
    match('m5', 'de_nuke', side('ct-start', 13, MOUZ), side('t-start', 2, null)),
  ];
  assert.deepEqual(mapPerformance(matches, VIT), [
    { map: 'de_mirage', matches: 3, wins: 2, losses: 1, draws: 0, roundsWon: 37, roundsLost: 29 },
    { map: 'de_ancient', matches: 1, wins: 1, losses: 0, draws: 0, roundsWon: 13, roundsLost: 2 },
  ]);
  assert.deepEqual(teamFilterOptions([...matches, null]), [
    { value: MOUZ, label: 'MOUZ' },
    { value: VIT, label: 'Vitality' },
  ]);
});

test('date ranges end now and keep everything for an unknown range', () => {
  const now = Date.parse('2026-10-30T12:00:00Z');
  assert.equal(inDateRange('7d', '2026-10-25T12:00:00Z', now), true);
  assert.equal(inDateRange('7d', '2026-10-20T12:00:00Z', now), false);
  assert.equal(inDateRange('7d', '2026-11-01T12:00:00Z', now), false);
  assert.equal(inDateRange('', 'nunca', now), true);
  assert.equal(inDateRange('30d', 'nunca', now), false);
});

test('player totals add the match lines and derive ADR and K/D from the sums', () => {
  const line = (rounds: number, kills: number, deaths: number, damage: number): LibraryMatchPlayer => ({
    steamid64: '1', name: 'a', team_key: 'ct-start', slot: 0, rounds, kills, deaths, assists: 1, damage,
  });
  const totals = playerTotals([line(20, 18, 12, 1700), line(24, 20, 20, 2100)]);
  assert.deepEqual(totals, { matches: 2, rounds: 44, kills: 38, deaths: 32, assists: 2, damage: 3800 });
  assert.equal(adrLabel(totals.damage, totals.rounds), '86.4');
  assert.equal(kdLabel(totals.kills, totals.deaths), '1.19');
  assert.equal(adrLabel(0, 0), '');
  assert.equal(kdLabel(5, 0), '5.00');
});

test('the utility ratio compares grenades per round with the teammates', () => {
  assert.equal(utilityRatioLabel({ count: 60, total: 40 }, { count: 160, total: 160 }), '1.50');
  assert.equal(utilityRatioLabel({ count: 3, total: 4 }, { count: 0, total: 16 }), '');
  assert.equal(utilityRatioLabel({ count: 0, total: 0 }, { count: 4, total: 4 }), '');
});
