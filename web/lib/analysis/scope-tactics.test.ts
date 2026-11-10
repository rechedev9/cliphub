import assert from 'node:assert/strict';
import test from 'node:test';
import { mapPatch, tacticChoices, tacticOptions, unknownTactic } from './scope-tactics.ts';

const PLAYS = [
  { id: 'a', title: 'Exec A', map: 'de_mirage', side: 'T', status: 'active' },
  { id: 'b', title: 'Retake B', map: 'de_nuke', side: 'CT', status: 'archived' },
  { id: 'c', title: 'Ataque B', map: 'de_mirage', side: 'T', status: 'draft' },
] as const;
const OPTIONS = tacticOptions(PLAYS);

test('every play is labelled with its map, archived ones marked', () => {
  assert.deepEqual(
    OPTIONS.map((option) => option.label),
    ['Ataque B · Mirage', 'Exec A · Mirage', 'Retake B · Nuke (archivada)'],
  );
});

test('the choices keep the plays of the scope map and side', () => {
  assert.deepEqual(tacticChoices(OPTIONS, { map: 'de_mirage', side: 'T' }).map((option) => option.value), ['c', 'a']);
  assert.deepEqual(tacticChoices(OPTIONS, { map: 'de_mirage', side: 'CT' }), []);
  assert.equal(tacticChoices(OPTIONS, { map: '', side: '' }).length, 3);
});

test('a new map clears a tactic read on another map', () => {
  assert.deepEqual(mapPatch({ map: 'de_nuke', tactic: 'a', options: OPTIONS }), { map: 'de_nuke', tactic: '' });
  assert.deepEqual(mapPatch({ map: 'de_mirage', tactic: 'a', options: OPTIONS }), { map: 'de_mirage' });
  assert.deepEqual(mapPatch({ map: '', tactic: 'a', options: OPTIONS }), { map: '' });
});

test('a tactic outside the team plays is unknown', () => {
  assert.equal(unknownTactic('gone', OPTIONS), true);
  assert.equal(unknownTactic('b', OPTIONS), false);
  assert.equal(unknownTactic('', OPTIONS), false);
});
