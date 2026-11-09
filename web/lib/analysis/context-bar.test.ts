// The context bar is the page's frame: its dots must be the latest results in
// reading order, and switching entity must keep what the person was looking at.
import test from 'node:test';
import assert from 'node:assert/strict';
import { lastResults, resultRecord, scoreOutcome, switchEntityHref, tabFromQuery, writeTab } from './context-bar.ts';
import { buyIcon, grenadeIcon, iconAspect } from './icons.ts';
import { deltaArrow } from './compare.ts';
import { smoothPath } from './chart.ts';

test('the dots are the latest results, oldest first, without unknown outcomes', () => {
  const rows = [
    { playedAt: '2026-11-03T10:00:00Z', outcome: 'loss' as const },
    { playedAt: '2026-11-01T10:00:00Z', outcome: 'win' as const },
    { playedAt: '2026-11-04T10:00:00Z', outcome: null },
    { playedAt: '2026-11-05T10:00:00Z', outcome: 'draw' as const },
  ];
  assert.deepEqual(lastResults(rows, 2), ['loss', 'draw']);
  assert.deepEqual(lastResults(rows, 10), ['win', 'loss', 'draw']);
  assert.equal(resultRecord(['win', 'loss', 'win']), '2-1');
  assert.equal(resultRecord(['win', 'draw']), '1-0-1');
  assert.equal(scoreOutcome(13, 9), 'win');
  assert.equal(scoreOutcome(12, 12), 'draw');
  assert.equal(scoreOutcome(0, 0), null);
});

test('switching entity keeps tab and filters but drops the rival', () => {
  const href = switchEntityHref('/equipos/b', 'tab=veto&map=de_mirage&opponent=0b7c4a1e-6f0d-4a51-9d55-3f1f3a9c2b10&lineup=all');
  assert.equal(href, '/equipos/b?tab=veto&map=de_mirage');
  assert.equal(switchEntityHref('/jugadores/1', ''), '/jugadores/1');
});

test('the tab lives in the query and the default one is left out', () => {
  const tabs = ['plantilla', 'mapas'];
  assert.equal(tabFromQuery(new URLSearchParams('tab=mapas'), tabs, 'plantilla'), 'mapas');
  assert.equal(tabFromQuery(new URLSearchParams('tab=nope'), tabs, 'plantilla'), 'plantilla');
  assert.equal(writeTab('map=de_nuke', 'mapas', 'plantilla'), 'map=de_nuke&tab=mapas');
  assert.equal(writeTab('map=de_nuke&tab=mapas', 'plantilla', 'plantilla'), 'map=de_nuke');
});

test('each buy and grenade has its own icon, and long guns are drawn wider than grenades', () => {
  assert.equal(buyIcon('full'), 'ak47');
  assert.equal(buyIcon('unknown'), null);
  assert.notEqual(buyIcon('eco'), buyIcon('pistol'));
  assert.equal(grenadeIcon('molotov'), 'molotov');
  assert.ok(iconAspect('awp') > iconAspect('glock'));
  assert.ok(iconAspect('glock') > iconAspect('smokegrenade'));
});

test('the arrow follows the sign of the difference, not whether it is good', () => {
  assert.equal(deltaArrow(0.2), '▲');
  assert.equal(deltaArrow(-0.2), '▼');
  assert.equal(deltaArrow(0.001, 0.01), '');
  assert.equal(deltaArrow(Number.NaN), '');
});

test('a smooth line passes through every point and never overshoots a flat stretch', () => {
  const path = smoothPath([{ x: 0, y: 10 }, { x: 10, y: 0 }, { x: 20, y: 0 }, { x: 30, y: 5 }]);
  assert.match(path, /^M0 10 C/);
  assert.match(path, / 10 0 C/);
  assert.match(path, / 30 5$/);
  // The flat middle segment keeps both controls on y=0, so the curve cannot dip below it.
  assert.match(path, /C13\.3 0 16\.7 0 20 0/);
  assert.equal(smoothPath([{ x: 0, y: 1 }, { x: 5, y: 2 }]), 'M0 1 L5 2');
});
