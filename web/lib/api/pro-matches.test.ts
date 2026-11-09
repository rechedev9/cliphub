import assert from 'node:assert/strict';
import test from 'node:test';
import { DEFAULT_PRO_QUERY, groupByDay, proCatalogNotice, proMapPart, proMatchSearch, proMatchTitle, type ProMatch } from './pro-matches.ts';

const match = (id: number, playedAt: string): ProMatch => ({
  id,
  event: 'IEM Chengdu',
  played_at: playedAt,
  teams: [
    { id: 1, name: 'Vitality', followed: false },
    { id: 2, name: 'FaZe', followed: true },
  ],
  maps: [],
});

test('a catalog read sends only what differs from the defaults', () => {
  assert.equal(proMatchSearch(DEFAULT_PRO_QUERY), '');
  assert.equal(proMatchSearch({ ...DEFAULT_PRO_QUERY, days: 30, team: '9565', page: 2 }), '?days=30&team=9565&page=2');
});

test('matches group by the day they were played, in the server order', () => {
  const groups = groupByDay([match(3, '2026-11-09T20:00:00Z'), match(2, '2026-11-09T12:00:00Z'), match(1, '2026-11-08T18:00:00Z')]);
  assert.deepEqual(
    groups.map((g) => [g.day, g.matches.map((m) => m.id)]),
    [
      ['2026-11-09', [3, 2]],
      ['2026-11-08', [1]],
    ],
  );
});

test('a match reads as its two teams and a restart part is named', () => {
  assert.equal(proMatchTitle(match(1, '2026-11-09T00:00:00Z')), 'Vitality vs FaZe');
  assert.equal(proMatchTitle({ teams: [], event: 'BLAST' }), 'BLAST');
  assert.equal(proMapPart('a-vs-b-m2-nuke-p2'), 'parte 2');
  assert.equal(proMapPart('a-vs-b-m2-nuke'), null);
});

test('every catalog state but ok has its own message', () => {
  assert.equal(proCatalogNotice('ok'), null);
  const titles = (['disabled', 'missing', 'unavailable'] as const).map((s) => proCatalogNotice(s)?.title);
  assert.equal(new Set(titles).size, 3);
  for (const t of titles) assert.ok(t && !/[–—]/.test(t));
});
