import test from 'node:test';
import assert from 'node:assert/strict';
import {
  FILTER_ANY,
  SEARCH_FILTER,
  activeFilterCount,
  filterRows,
  nextSort,
  parseSavedViews,
  removeView,
  sortRows,
  toggleHidden,
  upsertView,
  type SortValue,
  type TableViewState,
} from './table-view.ts';

type Row = { id: string; map: string; rounds: number | null; player: string };

const ROWS: readonly Row[] = [
  { id: 'a', map: 'Mirage', rounds: 24, player: 'Jugadór Uno' },
  { id: 'b', map: 'Ancient', rounds: null, player: 'dos' },
  { id: 'c', map: 'mirage', rounds: 9, player: 'Tres' },
  { id: 'd', map: 'Nuke', rounds: 24, player: 'cuatro' },
];

const valueOf = (row: Row, column: string): SortValue => (column === 'rounds' ? row.rounds : row.map);
const ids = (rows: readonly Row[]): string[] => rows.map((row) => row.id);

const STATE: TableViewState = { sort: { column: 'rounds', direction: 'desc' }, hidden: ['player'], filters: { map: 'Mirage' } };

test('header clicks cycle ascending, descending and back to the default order', () => {
  const asc = nextSort(null, 'map');
  assert.deepEqual(asc, { column: 'map', direction: 'asc' });
  const desc = nextSort(asc, 'map');
  assert.deepEqual(desc, { column: 'map', direction: 'desc' });
  assert.equal(nextSort(desc, 'map'), null);
  assert.deepEqual(nextSort(desc, 'rounds'), { column: 'rounds', direction: 'asc' });
});

test('sorting is stable, numeric and keeps rows without a value last in both directions', () => {
  assert.deepEqual(ids(sortRows(ROWS, { column: 'rounds', direction: 'asc' }, valueOf)), ['c', 'a', 'd', 'b']);
  assert.deepEqual(ids(sortRows(ROWS, { column: 'rounds', direction: 'desc' }, valueOf)), ['a', 'd', 'c', 'b']);
  // Case-insensitive collation keeps the two Mirage rows in input order.
  assert.deepEqual(ids(sortRows(ROWS, { column: 'map', direction: 'asc' }, valueOf)), ['b', 'a', 'c', 'd']);
  assert.deepEqual(ids(sortRows(ROWS, null, valueOf)), ['a', 'b', 'c', 'd']);
});

test('filters narrow by exact field value and by accent-insensitive words', () => {
  const read = { fields: { map: (row: Row) => row.map }, searchText: (row: Row) => `${row.map} ${row.player}` };
  assert.deepEqual(ids(filterRows(ROWS, { map: 'Mirage' }, read)), ['a']);
  assert.deepEqual(ids(filterRows(ROWS, { [SEARCH_FILTER]: 'MIRAGE jugador' }, read)), ['a']);
  assert.deepEqual(ids(filterRows(ROWS, { [SEARCH_FILTER]: 'mirage' }, read)), ['a', 'c']);
  assert.deepEqual(ids(filterRows(ROWS, { map: FILTER_ANY, unknown: 'x' }, read)), ['a', 'b', 'c', 'd']);
  assert.equal(activeFilterCount({ map: 'Nuke', [SEARCH_FILTER]: FILTER_ANY }), 1);
});

test('a field with several values matches when any of them is the filter value', () => {
  const read = { fields: { tag: (row: Row) => [row.map, row.player] }, searchText: () => '' };
  assert.deepEqual(ids(filterRows(ROWS, { tag: 'Mirage' }, read)), ['a']);
  assert.deepEqual(ids(filterRows(ROWS, { tag: 'nadie' }, read)), []);
});

test('toggling a column hides it and shows it again', () => {
  assert.deepEqual(toggleHidden([], 'map'), ['map']);
  assert.deepEqual(toggleHidden(['map', 'player'], 'map'), ['player']);
});

test('saving a view under an existing name replaces it; removing drops it', () => {
  const first = upsertView([], '  Pistols en Mirage ', STATE);
  assert.deepEqual(first, [{ id: 'pistols-en-mirage', name: 'Pistols en Mirage', state: STATE }]);
  const replaced = upsertView(first, 'pistols EN mirage', { ...STATE, hidden: [] });
  assert.equal(replaced.length, 1);
  assert.deepEqual(replaced[0]?.state.hidden, []);
  assert.deepEqual(upsertView(first, '   ', STATE), first);
  assert.deepEqual(removeView(replaced, 'pistols-en-mirage'), []);
});

test('stored views survive a round trip and malformed storage reads as no views', () => {
  const views = upsertView(upsertView([], 'Uno', STATE), 'Dos', { sort: null, hidden: [], filters: {} });
  assert.deepEqual(parseSavedViews(JSON.stringify(views)), views);
  assert.deepEqual(parseSavedViews(null), []);
  assert.deepEqual(parseSavedViews('{not json'), []);
  assert.deepEqual(parseSavedViews('{"name":"x"}'), []);
  const partial = parseSavedViews(
    JSON.stringify([
      { name: 'Bien', state: { sort: { column: 'map', direction: 'sideways' }, hidden: ['a', 3], filters: { map: 'Nuke', n: 1 } } },
      { name: 4, state: STATE },
      { name: 'Sin estado' },
    ]),
  );
  assert.deepEqual(partial, [{ id: 'bien', name: 'Bien', state: { sort: null, hidden: ['a'], filters: { map: 'Nuke' } } }]);
});
