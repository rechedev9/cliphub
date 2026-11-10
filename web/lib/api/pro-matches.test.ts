import assert from 'node:assert/strict';
import test from 'node:test';
import { DEFAULT_PRO_QUERY, groupByDay, parseProQuery, proQueryAddress, localDay, proCatalogNotice, proDayLabel, proMapDemos, proMatchSearch, proMatchTitle, type ProMatch } from './pro-matches.ts';

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

test('day headings read Hoy and Ayer in local time', () => {
  const now = new Date(2026, 10, 9, 0, 10);
  assert.equal(proDayLabel(localDay(now), now), 'Hoy');
  assert.equal(proDayLabel(localDay(new Date(2026, 10, 8, 23, 50)), now), 'Ayer');
  assert.equal(proDayLabel('2026-11-01', now), '1 nov');
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