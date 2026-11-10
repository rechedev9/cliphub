import test from 'node:test';
import assert from 'node:assert/strict';
import type { LibraryDemo, LibraryMatch, LibraryMatchTeam } from '../api/library.ts';
import type { LibrarySeries } from '../api/veto.ts';
import {
  groupPartidas,
  cleanTag,
  eventOptions,
  hasPov,
  libraryTags,
  listViewParams,
  matchKind,
  ownOutcome,
  ownTeamId,
  ownTeamName,
  parseListView,
  rowKind,
  seriesIndex,
  seriesScore,
  seriesVeto,
  tagSuggestions,
  withListView,
  withoutListView,
} from './partida-groups.ts';
import type { PartidaRow } from './partidas.ts';

const A = 'aaaaaaaa-0000-4000-8000-000000000001';
const B = 'aaaaaaaa-0000-4000-8000-000000000002';
const C = 'aaaaaaaa-0000-4000-8000-000000000003';

function side(key: LibraryMatchTeam['key'], score: number, teamId: string | null): LibraryMatchTeam {
  return { key, start_side: key === 'ct-start' ? 'CT' : 'T', clan_name: '', team_id: teamId, assignment: 'auto', score };
}

function row(id: string, input: { source?: LibraryDemo['source']; at: string; imported?: string; series?: string; teams?: [string | null, string | null]; tags?: string[]; jobs?: string[] }): PartidaRow {
  const [ct, t] = input.teams ?? [A, B];
  const match: LibraryMatch = {
    id, demo_id: `demo-${id}`, map: 'de_mirage', played_at: input.at, source: input.source ?? 'faceit', tickrate: 64, duration_ticks: 1,
    round_count: 20, teams: [side('ct-start', 13, ct), side('t-start', 7, t)], created_at: '', updated_at: '',
    ...(input.series === undefined ? {} : { series_id: input.series }),
  };
  const demo: LibraryDemo = {
    id: `demo-${id}`, sha256: id, file_name: `${id}.dem`, path: '', size_bytes: 1, source: input.source ?? 'faceit', source_manual: false, state: 'ready',
    tags: input.tags ?? [], job_ids: input.jobs ?? [], imported_at: input.imported ?? input.at, updated_at: input.at,
  };
  return { demo, match };
}

function series(id: string, vetoes: LibrarySeries['vetoes']): LibrarySeries {
  return {
    id, format: 'bo3', event: '', team_ids: [A, B], teams: [{ slot: 'a', team_id: A, name: 'Alfa' }, { slot: 'b', team_id: B, name: 'Beta' }],
    vetoes, match_ids: [], created_at: '', updated_at: '',
  };
}

test('the match kind comes from the demo source and the series event', () => {
  assert.equal(matchKind({ source: 'scrim', inEventSeries: true }), 'scrim');
  assert.equal(matchKind({ source: 'hltv', inEventSeries: false }), 'oficial');
  assert.equal(matchKind({ source: 'faceit', inEventSeries: true }), 'oficial');
  assert.equal(matchKind({ source: 'faceit', inEventSeries: false }), 'importada');
  assert.equal(matchKind({ source: 'valve', inEventSeries: false }), 'importada');
});

test('a row is official only through its series event', () => {
  const withEvent: LibrarySeries = { ...series('s1', []), event: 'IEM' };
  const index = seriesIndex([withEvent, series('s2', [])]);
  assert.equal(rowKind(row('a', { at: '2026-10-01T10:00:00Z', series: 's1' }), index), 'oficial');
  assert.equal(rowKind(row('b', { at: '2026-10-01T10:00:00Z', series: 's2' }), index), 'importada');
  assert.equal(rowKind(row('c', { at: '2026-10-01T10:00:00Z', source: 'scrim', series: 's1' }), index), 'scrim');
  assert.deepEqual(eventOptions([withEvent, series('s2', [])]), [{ value: 'IEM', label: 'IEM' }]);
});

test('the list view round-trips through the query and leaves other keys alone', () => {
  const view = parseListView(new URLSearchParams('orden=importacion&tipo=scrim&pov=1&voz=1&vista=tabla&map=de_nuke'));
  assert.deepEqual(view, { sort: 'imported', kind: 'scrim', pov: true, voice: true, mode: 'tabla' });
  assert.deepEqual(parseListView(new URLSearchParams('tipo=otra&orden=x')), { sort: 'played', kind: null, pov: false, voice: false, mode: 'series' });
  assert.deepEqual(listViewParams(parseListView(new URLSearchParams())), []);
  assert.equal(withoutListView(new URLSearchParams('orden=importacion&map=de_nuke')), 'map=de_nuke');
  assert.equal(withListView(new URLSearchParams('map=de_nuke&tipo=scrim'), { sort: 'played', kind: null, pov: true, voice: true, mode: 'series' }), 'map=de_nuke&pov=1&voz=1');
});

test('a partida has a POV when one of its jobs has a video', () => {
  const r = row('m1', { at: '2026-10-02T18:00:00.000Z', jobs: ['j1', 'j2'] });
  assert.equal(hasPov(r, new Set(['j2'])), true);
  assert.equal(hasPov(r, new Set(['j9'])), false);
});

test('the own team is the one picked, else the one that plays most scrims, and only when it is clear', () => {
  const rows = [
    row('1', { at: '2026-10-01T10:00:00Z', source: 'scrim' }),
    row('2', { at: '2026-10-02T10:00:00Z', source: 'scrim', teams: [A, C] }),
    row('3', { at: '2026-10-03T10:00:00Z', source: 'scrim', teams: [C, A] }),
  ];
  const [first, second, third] = rows;
  assert.ok(first && second && third);
  assert.equal(ownTeamId({ rows, explicit: null }), A);
  assert.equal(ownTeamId({ rows: rows.slice(0, 2), explicit: null }), null);
  assert.equal(ownTeamId({ rows, explicit: C }), C);
  const pugs = rows.map((entry) => ({ ...entry, demo: { ...entry.demo, source: 'faceit' as const } }));
  assert.equal(ownTeamId({ rows: pugs, explicit: null }), null);
  assert.equal(ownTeamName(rows, A), 'Equipo CT');
  assert.equal(ownOutcome(first, A), 'win');
  assert.equal(ownOutcome(second, A), 'win');
  assert.equal(ownOutcome(third, A), 'loss');
  assert.equal(ownOutcome(first, null), null);
  assert.equal(ownOutcome(first, 'x'), null);
});

test('series group their maps in play order and the list is split by day', () => {
  const s = series('s1', []);
  const rows = [
    row('late', { at: '2026-10-05T20:00:00Z', series: 's1' }),
    row('solo', { at: '2026-10-05T10:00:00Z' }),
    row('early', { at: '2026-10-05T18:00:00Z', series: 's1' }),
    row('old', { at: '2026-09-01T12:00:00Z', series: 'gone' }),
  ];
  const days = groupPartidas({ rows, series: [s], sort: 'played' });
  assert.equal(days.length, 2);
  const [first, second] = days;
  assert.ok(first && second);
  const [group, solo] = first.groups;
  assert.ok(group && solo);
  assert.equal(group.kind, 'series');
  if (group.kind === 'series') assert.deepEqual(group.rows.map((r) => r.match?.id), ['early', 'late']);
  assert.equal(solo.kind, 'single');
  assert.equal(second.groups[0]?.kind, 'single');
});

test('sorting by import date reorders the groups', () => {
  const rows = [
    row('a', { at: '2026-10-01T10:00:00Z', imported: '2026-10-09T10:00:00Z' }),
    row('b', { at: '2026-10-08T10:00:00Z', imported: '2026-10-02T10:00:00Z' }),
  ];
  const ids = (sort: 'played' | 'imported'): (string | undefined)[] =>
    groupPartidas({ rows, series: [], sort }).flatMap((day) => day.groups.map((g) => (g.kind === 'single' ? g.row.match?.id : undefined)));
  assert.deepEqual(ids('played'), ['b', 'a']);
  assert.deepEqual(ids('imported'), ['a', 'b']);
});

test('the stored veto gives the bans and how each map came to be played', () => {
  const veto = seriesVeto(
    series('s1', [
      { team: 'a', action: 'ban', map: 'de_nuke' },
      { team: 'b', action: 'ban', map: 'de_inferno' },
      { team: 'a', action: 'pick', map: 'de_mirage' },
      { team: '', action: 'decider', map: 'de_ancient' },
    ]),
  );
  assert.deepEqual(veto.bans, [{ map: 'de_nuke', team: 'Alfa' }, { map: 'de_inferno', team: 'Beta' }]);
  assert.deepEqual(veto.marks.get('de_mirage'), { action: 'pick', team: 'Alfa' });
  assert.deepEqual(veto.marks.get('de_ancient'), { action: 'decider', team: '' });
});

test('tag suggestions skip the tags a demo already has and rank prefixes first', () => {
  const all = ['ESL', 'scrim vs X', 'Pracc', 'escuadra'];
  assert.deepEqual(tagSuggestions({ all, typed: 'es', current: [] }), ['ESL', 'escuadra']);
  assert.deepEqual(tagSuggestions({ all, typed: 'cr', current: ['Pracc'] }), ['scrim vs X']);
  assert.deepEqual(tagSuggestions({ all, typed: '', current: ['ESL'] }), ['scrim vs X', 'Pracc', 'escuadra']);
});

test('a tag is trimmed, comma free and no longer than the library allows', () => {
  assert.equal(cleanTag('  ESL, Pro  '), 'ESL Pro');
  assert.equal(cleanTag('x'.repeat(60)).length, 40);
});

test('library tags come most used first', () => {
  const rows = [row('1', { at: '2026-10-01T10:00:00Z', tags: ['b', 'a'] }), row('2', { at: '2026-10-02T10:00:00Z', tags: ['b'] })];
  assert.deepEqual(libraryTags(rows), ['b', 'a']);
});

test('the series score counts the maps each team won', () => {
  const rows = [
    row('m1', { at: '2026-10-01T10:00:00Z', series: 's1' }),
    row('m2', { at: '2026-10-01T11:00:00Z', series: 's1', teams: [B, A] }),
    row('m3', { at: '2026-10-01T12:00:00Z', series: 's1', teams: [B, A] }),
  ];
  assert.deepEqual(seriesScore(series('s1', []), rows), [1, 2]);
  assert.equal(seriesScore({ ...series('s1', []), teams: [] }, rows), null);
});
