import assert from 'node:assert/strict';
import test from 'node:test';
import { duplicateStyle, hudColor, hudPreviewUrl, hudStyleOf, isHudOptions, isHudStyle, isHudTeamNames, needsDrawnPreview, sameHudStyle } from './hud-themes.ts';

test('a style is the Go shape: six upper-case hex digits and a radius of 0 to 15', () => {
  assert.equal(isHudStyle({ name: 'Mío', ct: '112233', t: 'AABBCC', background: '000000', radius: 15 }), true);
  assert.equal(isHudStyle({ name: 'Mío', ct: '112233', t: 'aabbcc', background: '000000' }), false);
  assert.equal(isHudStyle({ name: ' ', ct: '112233', t: 'AABBCC', background: '000000' }), false);
  assert.equal(isHudStyle({ name: 'Mío', ct: '112233', t: 'AABBCC', background: '000000', radius: 16 }), false);
  assert.equal(hudColor('#a1b2c3'), 'A1B2C3');
});

test('a duplicate starts from the catalog colours or the theme it copies', () => {
  const fromCatalog = duplicateStyle('arena');
  assert.deepEqual(fromCatalog, { name: 'Arena (copia)', ct: '42C9E8', t: 'FFB32C', background: '101927' });
  const copy = duplicateStyle('arena', { name: 'Mío', ct: '111111', t: '222222', background: '333333', radius: 4 });
  assert.deepEqual(copy, { name: 'Mío (copia)', ct: '111111', t: '222222', background: '333333', radius: 4 });
  assert.ok([...duplicateStyle('arena', { ...copy, name: 'x'.repeat(40) }).name].length <= 40);
});

test('only the style fields leave a stored theme, and styles compare by value', () => {
  const stored = { id: 'a', base: 'arena', name: 'Mío', ct: '111111', t: '222222', background: '333333' };
  assert.deepEqual(hudStyleOf(stored), { name: 'Mío', ct: '111111', t: '222222', background: '333333' });
  assert.equal(sameHudStyle(hudStyleOf(stored), { ...hudStyleOf(stored) }), true);
  assert.equal(sameHudStyle(hudStyleOf(stored), { ...hudStyleOf(stored), radius: 0 }), false);
});

test('switches and team names are checked like Go and drawn in the preview only when set', () => {
  assert.equal(isHudOptions({ compact_header: true }), true);
  assert.equal(isHudOptions({ compact: true }), false);
  assert.equal(isHudTeamNames({ ct_start: 'Vitality', t_start: '' }), true);
  assert.equal(isHudTeamNames({ ct_start: ' Vitality' }), false);
  assert.equal(isHudTeamNames({ ct_start: 'x'.repeat(33) }), false);
  assert.equal(needsDrawnPreview(undefined, { compact_header: false }, { ct_start: '' }), false);
  assert.equal(needsDrawnPreview(undefined, { round_win_banner: true }, undefined), true);
  assert.equal(isHudOptions({ win_probability: true }), true);
  assert.equal(hudPreviewUrl({ base: 'arena', options: { win_probability: true } }), '/api/full-demo/hud-themes/preview?base=arena&win_probability=1');
  const url = hudPreviewUrl({ base: 'arena', style: { name: 'a', ct: '111111', t: '222222', background: '333333', radius: 0 }, options: { hide_loadout: true }, names: { t_start: 'Team Spirit' } });
  assert.equal(url, '/api/full-demo/hud-themes/preview?base=arena&ct=111111&t=222222&background=333333&radius=0&hide_loadout=1&t_name=Team+Spirit');
});
