import assert from 'node:assert/strict';
import { test } from 'node:test';
import { cardRoleText, mapRoleText, mapRolesText } from './map-role.ts';

const lurk = { role: 'b_lurk', map_role: 'Canal', label: 'Lurk de B' } as const;
const anchor = { role: 'a_anchor', map_role: 'BombsiteA', label: 'Ancla A' } as const;

test('a side role reads with its side, and nothing without one', () => {
  assert.equal(mapRoleText('T', lurk), 'T · Lurk de B');
  assert.equal(mapRoleText('CT', undefined), '');
});

test('a card reads the map role before the library role, either alone without the other', () => {
  assert.equal(cardRoleText({ side: 'T', mapRole: lurk, role: 'AWP' }), 'T · Lurk de B, AWP');
  assert.equal(cardRoleText({ side: 'CT', mapRole: undefined, role: 'Rifler' }), 'Rifler');
  assert.equal(cardRoleText({ side: 'CT', mapRole: anchor, role: '' }), 'CT · Ancla A');
});

test('both sides read T first and the side filter keeps one', () => {
  assert.equal(mapRolesText({ t: lurk, ct: anchor }, ''), 'T · Lurk de B, CT · Ancla A');
  assert.equal(mapRolesText({ t: lurk, ct: anchor }, 'CT'), 'CT · Ancla A');
  assert.equal(mapRolesText({ ct: anchor }, 'T'), '');
  assert.equal(mapRolesText(undefined, ''), '');
});
