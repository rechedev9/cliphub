import assert from 'node:assert/strict';
import test from 'node:test';
import { DEFAULT_PRO_QUERY, groupByDay, parseProQuery, proQueryAddress, proCatalogNotice, proMapDemos, proMatchSearch, proMatchTitle, proSeriesVeto, refreshedProPage, type ProMatch, type ProMatchesPage } from './pro-matches.ts';

const match = (id: number, playedAt: string): ProMatch => ({
  id,
  event: 'IEM Chengdu',
  played_at: playedAt,
  hltv_url: `https://www.hltv.org/matches/${id}/match`,
  teams: [
    { id: 1, name: 'Vitality', followed: false },
    { id: 2, name: 'FaZe', followed: true },
  ],
  maps: [],
  series: { maps: [], veto: [] },
});

test('a catalog read sends only what differs from the defaults', () => {
  assert.equal(proMatchSearch(DEFAULT_PRO_QUERY), '');
  assert.equal(proMatchSearch({ ...DEFAULT_PRO_QUERY, days: 30, team: '9565', page: 2 }), '?days=30&team=9565&page=2');
  const now = new Date(2026, 10, 9, 18, 30);
  const since = new URLSearchParams(proMatchSearch({ ...DEFAULT_PRO_QUERY, days: 1 }, now)).get('since');
  assert.equal(since, new Date(2026, 10, 9).toISOString(), 'Hoy starts at local midnight');
});

test('matches group by the local day they were played, in the server order', () => {
  const at = (d: number, h: number) => new Date(2026, 10, d, h, 30).toISOString();
  const groups = groupByDay([match(3, at(9, 23)), match(2, at(9, 0)), match(1, at(8, 23))]);
  assert.deepEqual(
    groups.map((g) => [g.day, g.matches.map((m) => m.id)]),
    [
      ['2026-11-09', [3, 2]],
      ['2026-11-08', [1]],
    ],
  );
});

test('a match reads as its two teams and a joined map says so', () => {
  assert.equal(proMatchTitle(match(1, '2026-11-09T00:00:00Z')), 'Vitality vs FaZe');
  assert.equal(proMatchTitle({ teams: [], event: 'BLAST' }), 'BLAST');
  assert.equal(proMapDemos({ parts: [{ entry: 'a', first_round: 1, rounds: 9 }, { entry: 'a-p2', first_round: 9, rounds: 13 }] }), '2 demos unidas');
  assert.equal(proMapDemos({ parts: [{ entry: 'a', first_round: 1, rounds: 21 }] }), null);
});

test('every catalog state but ok has its own message', () => {
  assert.equal(proCatalogNotice('ok'), null);
  const titles = (['disabled', 'missing', 'unavailable'] as const).map((s) => proCatalogNotice(s)?.title);
  assert.equal(new Set(titles).size, 3);
  for (const t of titles) assert.ok(t && !/[–—]/.test(t));
});

test('the filters travel in the address and read back the same', () => {
  const q = { days: 90, team: '9565', event: 'IEM Chengdu', map: 'de_mirage', page: 2 };
  assert.equal(proQueryAddress(q), 'dias=90&equipo=9565&evento=IEM+Chengdu&mapa=de_mirage&pagina=2');
  assert.deepEqual(parseProQuery(new URLSearchParams(proQueryAddress(q))), q);
  assert.equal(proQueryAddress(DEFAULT_PRO_QUERY), '');
  assert.deepEqual(parseProQuery(new URLSearchParams('dias=60')), { ...DEFAULT_PRO_QUERY, days: 60 });
  assert.deepEqual(parseProQuery(new URLSearchParams('dias=999&mapa=../x&pagina=0')), DEFAULT_PRO_QUERY);
});
test('a pro series reads its bans and how each map came to be played', () => {
  const m: ProMatch = {
    ...match(1, '2026-11-08T15:00:00Z'),
    series: {
      format: 'bo3',
      maps: [{ number: 1, map: 'de_ancient', picked_by: 1 }, { number: 2, map: 'de_dust2', picked_by: 7 }, { number: 3, map: 'de_nuke', decider: true }],
      veto: [{ team_id: 2, team: 'FaZe', action: 'ban', map: 'de_mirage' }, { team: 'Faze Clan', action: 'ban', map: 'de_train' }, { action: 'decider', map: 'de_nuke' }],
    },
  };
  const veto = proSeriesVeto(m);
  assert.equal(veto.format, 'BO3');
  assert.deepEqual(veto.bans, [{ map: 'de_mirage', team: 'FaZe' }, { map: 'de_train', team: 'Faze Clan' }]);
  assert.deepEqual([...veto.marks], [['de_ancient', 'PICK de Vitality'], ['de_dust2', 'PICK'], ['de_nuke', 'DECIDER']]);
  assert.deepEqual(proSeriesVeto(match(2, '2026-11-08T15:00:00Z')), { format: null, bans: [], marks: new Map() });
});

test('a refresh while the server is down keeps the catalog on screen', () => {
  const page = (status: ProMatchesPage['status'], ids: number[]): ProMatchesPage => ({
    status, days: 7, page: 1, pages: 1, total: ids.length, matches: ids.map((id) => match(id, '2026-11-09T15:00:00Z')),
    facets: { teams: [], events: [], maps: [] }, follows: [],
  });
  const shown = page('ok', [1]);
  assert.equal(refreshedProPage(shown, page('unavailable', [])), shown);
  assert.deepEqual(refreshedProPage(shown, page('ok', [2, 1])).matches.map((m) => m.id), [2, 1], 'new matches replace the old page');
  assert.equal(refreshedProPage(page('unavailable', []), page('ok', [1])).status, 'ok', 'the server coming back shows its catalog');
  assert.equal(refreshedProPage(shown, page('disabled', [])).status, 'disabled');
  assert.equal(refreshedProPage(null, page('unavailable', [])).status, 'unavailable');
});
