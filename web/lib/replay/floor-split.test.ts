import assert from 'node:assert/strict';
import { test } from 'node:test';
import type { RadarCalibration } from '../api/tactical.ts';
import { worldToRendered } from '../tactical-transform.ts';
import { BOTH_FLOORS, PANE_GAP, floorPanes, localProjection, onFloorOf, paneAt, paneProjection } from './floor-split.ts';

const NUKE: RadarCalibration = { map: 'de_nuke', source: 'overview', pos_x: -3453, pos_y: 2887, scale: 7, size: 1024, lower_altitude_max: -495 };
const WHOLE = { x: 0, y: 0, width: 1, height: 1 };

test('one floor takes the whole box', () => {
  assert.deepEqual(floorPanes({ width: 800, height: 600 }, ['default']), [{ level: 'default', x: 0, y: 0, width: 800, height: 600 }]);
});

test('two floors sit side by side in a wide box and stacked in a tall one, upper first', () => {
  const wide = floorPanes({ width: 1202, height: 600 }, BOTH_FLOORS);
  assert.deepEqual(wide.map((pane) => [pane.level, pane.x, pane.width, pane.height]), [['default', 0, 600, 600], ['lower', 602, 600, 600]]);
  assert.equal(wide[1].x - (wide[0].x + wide[0].width), PANE_GAP);
  const tall = floorPanes({ width: 500, height: 900 }, BOTH_FLOORS);
  assert.deepEqual(tall.map((pane) => [pane.level, pane.y, pane.height]), [['default', 0, 449], ['lower', 451, 449]]);
});

test('one view projects the same spot of the map into both panes', () => {
  const [upper, lower] = floorPanes({ width: 1202, height: 600 }, BOTH_FLOORS);
  const view = { x: 0.25, y: 0.3, width: 0.5, height: 0.5 };
  const a = paneProjection(upper, view);
  const b = paneProjection(lower, view);
  assert.equal(a.size, b.size);
  assert.equal(b.offsetX - a.offsetX, lower.x - upper.x);
  assert.equal(a.offsetY, b.offsetY);
  // A world point lands at the same place inside each pane.
  const at = worldToRendered(NUKE, -500, -800, a.size);
  assert.ok(Math.abs(at.x + a.offsetX - upper.x - (at.x + b.offsetX - lower.x)) < 1e-9);
  assert.deepEqual(localProjection(lower, b), { size: b.size, offsetX: a.offsetX, offsetY: a.offsetY });
  assert.deepEqual(paneProjection(floorPanes({ width: 800, height: 600 }, ['default'])[0], WHOLE), { size: 600, offsetX: 100, offsetY: 0 });
});

test('a point finds its pane, and the line between them belongs to neither', () => {
  const panes = floorPanes({ width: 1202, height: 600 }, BOTH_FLOORS);
  assert.deepEqual(paneAt(panes, 10, 20), { pane: panes[0], x: 10, y: 20 });
  assert.deepEqual(paneAt(panes, 700, 20), { pane: panes[1], x: 98, y: 20 });
  assert.equal(paneAt(panes, 601, 20), null);
});

test('players and grenades go to the floor of their height', () => {
  const items = [{ id: 'ramp', z: -400 }, { id: 'secret', z: -640 }, { id: 'edge', z: -495 }];
  assert.deepEqual(onFloorOf(NUKE, 'default', items).map((item) => item.id), ['ramp']);
  assert.deepEqual(onFloorOf(NUKE, 'lower', items).map((item) => item.id), ['secret', 'edge']);
});
