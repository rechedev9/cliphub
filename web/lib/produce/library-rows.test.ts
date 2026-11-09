import test from 'node:test';
import assert from 'node:assert/strict';
import type { LibraryDemo, LibraryMatch } from '../api/library.ts';
import type { Match } from '../api/types.ts';
import { HUB_ROW_STAGE, type HubMatch } from '../clips/hub.ts';
import { libraryOnlyRows, libraryRowTeams } from './library-rows.ts';

function demo(id: string, patch: Partial<LibraryDemo> = {}): LibraryDemo {
  return {
    id, sha256: `sha-${id}`, file_name: `${id}.dem`, path: '', size_bytes: 1, source: 'faceit', source_manual: false,
    state: 'ready', tags: [], job_ids: [], match_id: `m-${id}`, imported_at: '2026-11-09T10:00:00Z', updated_at: '', ...patch,
  };
}

function match(id: string, playedAt: string): LibraryMatch {
  return {
    id: `m-${id}`, demo_id: id, map: 'de_cache', played_at: playedAt, source: 'faceit', tickrate: 64, duration_ticks: 1, round_count: 18,
    teams: [
      { key: 'ct-start', start_side: 'CT', clan_name: 'Los Galacticos', team_id: null, assignment: 'auto', score: 13 },
      { key: 't-start', start_side: 'T', clan_name: 'm0kAsYny', team_id: null, team_name: 'Mokas', assignment: 'auto', score: 5 },
    ],
    created_at: '', updated_at: '',
  };
}

function jobRow(id: string, demoSha256?: string): HubMatch {
  const row: Match = { id, map: 'Cache', score: '', playedAt: '', stats: { kills: 0, deaths: 0, assists: 0, mvps: 0, kd: 0 }, decentPlays: 0, ...(demoSha256 === undefined ? {} : { demoSha256 }) };
  return { match: row, stage: HUB_ROW_STAGE.ready, shorts: [], fulls: [] };
}

test('ready library demos without a Crear job join the list, newest played first', () => {
  const demos = [demo('a'), demo('b'), demo('c', { state: 'parsing' }), demo('d', { job_ids: ['job-d'] }), demo('e')];
  const matches = [match('a', '2026-11-01'), match('b', '2026-11-08'), match('c', '2026-11-08'), match('d', '2026-11-08'), match('e', '2026-11-08')];
  const rows = [jobRow('job-d'), jobRow('job-x', 'sha-e')];
  assert.deepEqual(libraryOnlyRows({ demos, matches, rows }).map((row) => row.demo.id), ['b', 'a']);
});

test('a demo whose match is not listed yet waits for the next read', () => {
  assert.deepEqual(libraryOnlyRows({ demos: [demo('a')], matches: [], rows: [] }), []);
});

test('the card names both teams in start order and prefers the corrected team name', () => {
  assert.deepEqual(libraryRowTeams(match('a', '')), { title: 'Los Galacticos vs Mokas', score: '13-5' });
});

test('a side the demo did not name reads by the side it started on', () => {
  const unnamed = match('a', '');
  const teams = unnamed.teams.map((team) => ({ ...team, clan_name: '', team_name: undefined }));
  assert.equal(libraryRowTeams({ ...unnamed, teams }).title, 'Equipo CT vs Equipo T');
});
