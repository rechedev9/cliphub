// Each active filter shows as a chip that names it the way the bar does.
import test from 'node:test';
import assert from 'node:assert/strict';
import { DEFAULT_SCOPE, type AnalysisScope } from './scope.ts';
import { equipText, scopeChips } from './scope-chips.ts';

const RIVAL = '0b7c4a1e-6f0d-4a51-9d55-3f1f3a9c2b10';
const PLAY = '1c8d5b2f-7a1e-4b62-8e66-4a2a4b9d3c21';
const NAMES = { opponents: new Map([[RIVAL, 'MOUZ']]), tactics: new Map([[PLAY, 'Exec A']]) };

test('chips name the active filters in the bar order and skip the defaults', () => {
  const scope: AnalysisScope = {
    ...DEFAULT_SCOPE,
    side: 'CT',
    buy: 'half',
    opponent: RIVAL,
    weapon: 'hero_ak',
    site: 'none',
    equip: { from: 3250, to: null },
    tactic: PLAY,
    rounds: { from: null, to: 12 },
  };
  const fields = ['side', 'period', 'buy', 'opponent', 'weapon', 'site', 'equip', 'tactic', 'rounds'] as const;
  assert.deepEqual(scopeChips({ scope, fields, names: NAMES }), [
    { field: 'side', label: 'Lado CT' },
    { field: 'buy', label: 'Compra propia: Media' },
    { field: 'opponent', label: 'Rival: MOUZ' },
    { field: 'weapon', label: 'AK héroe propia' },
    { field: 'site', label: 'Sin sitio' },
    { field: 'equip', label: 'Equipo propio: Desde $3250' },
    { field: 'tactic', label: 'Táctica: Exec A' },
    { field: 'rounds', label: 'Hasta la ronda 12' },
  ]);
});

test('a chip whose name is not loaded still reads', () => {
  const scope: AnalysisScope = { ...DEFAULT_SCOPE, opponent: RIVAL, tactic: PLAY };
  const empty = { opponents: new Map<string, string>(), tactics: new Map<string, string>() };
  assert.deepEqual(scopeChips({ scope, fields: ['opponent', 'tactic'], names: empty }).map((chip) => chip.label), ['Rival: otro equipo', 'Táctica: jugada de la Libreta']);
});

test('an equipment range reads open on either end', () => {
  assert.equal(equipText({ from: 1000, to: 3250 }), '$1000 a $3250');
  assert.equal(equipText({ from: null, to: 1000 }), 'Hasta $1000');
  assert.equal(equipText({ from: null, to: null }), '');
});
