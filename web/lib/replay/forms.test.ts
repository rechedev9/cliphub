import test from 'node:test';
import assert from 'node:assert/strict';
import type { TacticalFilter } from '../api/tactical.ts';
import { filterTacticalRounds, tacticalFilterCount } from '../tactical-filter.ts';
import { filterChips, replayFormFromQuery, writeReplayForm } from './forms.ts';

const NAMES = {
  teams: [
    { key: 'ct-start', name: 'FUT' },
    { key: 't-start', name: 'MOUZ' },
  ],
  players: [{ slot: 3, name: 'krabeni' }],
};

test('replay forms: the form reads from the query and Repetición is the default', () => {
  assert.equal(replayFormFromQuery(new URLSearchParams('forma=calor')), 'calor');
  assert.equal(replayFormFromQuery(new URLSearchParams('forma=3d')), 'repeticion');
  assert.equal(replayFormFromQuery(new URLSearchParams('')), 'repeticion');
});

test('replay forms: changing form keeps every filter in the query', () => {
  assert.equal(writeReplayForm('side=T&buy=full', 'granadas'), 'side=T&buy=full&forma=granadas');
  assert.equal(writeReplayForm('side=T&forma=granadas', 'repeticion'), 'side=T');
});

test('filter chips: one chip per constraint, each named in words', () => {
  const filter: TacticalFilter = { team_key: 'ct-start', side: 'T', outcome: 'win', buys: ['full', 'force'], round_from: 3, round_to: 12, slots: [3] };
  const chips = filterChips(filter, NAMES);
  assert.deepEqual(
    chips.map((chip) => chip.label),
    ['FUT', 'Lado T', 'Ganadas', 'Rondas 3 a 12', 'Compra: Full', 'Compra: Force', 'krabeni'],
  );
});

test('filter chips: removing one chip leaves the rest of the filter', () => {
  const filter: TacticalFilter = { side: 'T', buys: ['full', 'force'], sites: ['A'] };
  const chips = filterChips(filter, NAMES);
  const force = chips.find((chip) => chip.key === 'buy:force');
  assert.deepEqual(force?.without, { side: 'T', buys: ['full'], sites: ['A'] });
  const site = chips.find((chip) => chip.key === 'site:A');
  assert.equal(site?.without.sites, undefined);
  assert.equal(tacticalFilterCount(site?.without ?? {}), 2);
});

test('filter chips: removing the only perspective drops the result with it', () => {
  const chips = filterChips({ side: 'CT', outcome: 'loss' }, NAMES);
  const side = chips.find((chip) => chip.key === 'side');
  assert.equal(side?.without.outcome, undefined);
  assert.equal(filterTacticalRounds([], side?.without ?? {}, []).length, 0);
});

test('filter chips: an empty filter has none', () => {
  assert.deepEqual(filterChips({}, NAMES), []);
});
