import assert from 'node:assert/strict';
import { test } from 'node:test';
import { BOTH_FLOORS, PANE_GAP, WHOLE_FLOORS, floorWindow, followFloors, layoutFloors, localProjection, nearestPane, paneAt, paneProjection, panFloors, rememberFloorView, rememberedFloorView, viewOfWindow, zoomFloors } from './floor-split.ts';
import type { FloorRects } from './floor-split.ts';

/** A wide upper floor and a narrower, taller lower one, as on Nuke. */
const RECTS: FloorRects = { default: { x: 0.1, y: 0.3, width: 0.6, height: 0.3 }, lower: { x: 0.55, y: 0.2, width: 0.2, height: 0.4 } };

test('one floor takes the whole box and the fit its extent gets in it', () => {
  const layout = layoutFloors({ box: { width: 800, height: 600 }, rects: RECTS, levels: ['default'] });
  assert.deepEqual(layout.panes, [{ level: 'default', x: 0, y: 0, width: 800, height: 600, rect: RECTS.default }]);
  assert.equal(layout.scale, Math.min(800 / 0.6, 600 / 0.3));
});

test('two floors sit side by side in a landscape box, upper first and larger, with a gap', () => {
  const { panes, stacked } = layoutFloors({ box: { width: 1200, height: 600 }, rects: RECTS, levels: BOTH_FLOORS });
  const [upper, lower] = panes;
  assert.equal(stacked, false);
  assert.equal(upper.level, 'default');
  assert.ok(upper.width > lower.width);
  assert.ok(lower.x - (upper.x + upper.width) >= PANE_GAP - 1);
  // Each pane is its floor's extent at the one scale, so a unit of the radar square is the same number of pixels in both.
  assert.ok(Math.abs(upper.width / RECTS.default.width - lower.width / RECTS.lower.width) < 2);
  assert.ok(Math.abs(upper.height / RECTS.default.height - lower.height / RECTS.lower.height) < 2);
  // The group is centred.
  assert.ok(Math.abs(upper.x - (1200 - (lower.x + lower.width))) <= 1);
  assert.ok(upper.x + upper.width <= lower.x && lower.x + lower.width <= 1200);
});

test('two floors stack, upper on top, only in a portrait box', () => {
  const { panes, stacked } = layoutFloors({ box: { width: 500, height: 900 }, rects: RECTS, levels: BOTH_FLOORS });
  assert.equal(stacked, true);
  assert.ok(panes[0].y + panes[0].height <= panes[1].y);
  assert.ok(panes[1].y + panes[1].height <= 900);
  assert.equal(layoutFloors({ box: { width: 600, height: 600 }, rects: RECTS, levels: BOTH_FLOORS }).stacked, false);
});

test('both panes share one size at any zoom, and a floor window never leaves its extent', () => {
  const layout = layoutFloors({ box: { width: 1200, height: 600 }, rects: RECTS, levels: BOTH_FLOORS });
  const view = { zoom: 2, panX: 5, panY: -5 };
  const [a, b] = layout.panes.map((pane) => paneProjection({ layout, pane, view }));
  assert.equal(a.size, b.size);
  assert.equal(a.size, Math.floor(layout.scale * 2));
  const window = floorWindow(RECTS.lower, view);
  const centre = window.x + window.width / 2;
  assert.ok(centre >= RECTS.lower.x && centre <= RECTS.lower.x + RECTS.lower.width);
  assert.equal(window.width, RECTS.lower.width / 2);
  // Unzoomed, the window is the extent itself.
  assert.deepEqual(floorWindow(RECTS.default, WHOLE_FLOORS), RECTS.default);
});

test('a single floor fits like the replay always did', () => {
  const rect = { x: 0.2, y: 0.2, width: 0.6, height: 0.5 };
  const layout = layoutFloors({ box: { width: 800, height: 400 }, rects: { default: rect, lower: rect }, levels: ['default'] });
  const projection = paneProjection({ layout, pane: layout.panes[0], view: WHOLE_FLOORS });
  assert.equal(projection.size, Math.floor(Math.min(800 / 0.6, 400 / 0.5)));
  // The extent's centre lands on the box's centre.
  assert.ok(Math.abs(projection.offsetX + projection.size * 0.5 - 400) <= 1);
  assert.ok(Math.abs(projection.offsetY + projection.size * 0.45 - 200) <= 1);
});

test('a pane projection relative to its own corner, and the pane under a point', () => {
  const layout = layoutFloors({ box: { width: 1200, height: 600 }, rects: RECTS, levels: BOTH_FLOORS });
  const [upper, lower] = layout.panes;
  const projection = paneProjection({ layout, pane: lower, view: WHOLE_FLOORS });
  assert.deepEqual(localProjection(lower, projection), { size: projection.size, offsetX: projection.offsetX - lower.x, offsetY: projection.offsetY - lower.y });
  assert.equal(paneAt(layout.panes, upper.x + 3, upper.y + 4)?.pane, upper);
  assert.equal(paneAt(layout.panes, upper.x + upper.width + 2, upper.y + 4), null);
  assert.equal(nearestPane(layout.panes, upper.x + upper.width + 2, upper.y + 4).pane, upper);
  assert.equal(nearestPane(layout.panes, lower.x - 2, lower.y + 4).pane, lower);
});

test('zooming keeps the point under the cursor on the pane the cursor is in', () => {
  const layout = layoutFloors({ box: { width: 1200, height: 600 }, rects: RECTS, levels: BOTH_FLOORS });
  const pane = layout.panes[1];
  const cursor = { x: pane.width * 0.25, y: pane.height * 0.75 };
  const before = floorWindow(pane.rect, WHOLE_FLOORS);
  const view = zoomFloors({ pane, view: WHOLE_FLOORS, cursor, factor: 2 });
  assert.equal(view.zoom, 2);
  const after = floorWindow(pane.rect, view);
  // The cursor sits at the same fraction of a window that is half the size.
  const pointBefore = { x: before.x + 0.25 * before.width, y: before.y + 0.75 * before.height };
  const pointAfter = { x: after.x + 0.25 * after.width, y: after.y + 0.75 * after.height };
  assert.ok(Math.abs(pointBefore.x - pointAfter.x) < 1e-9);
  assert.ok(Math.abs(pointBefore.y - pointAfter.y) < 1e-9);
  assert.deepEqual(zoomFloors({ pane, view, cursor, factor: 0.25 }), WHOLE_FLOORS);
});

test('a pan moves both floors by the same world distance', () => {
  const layout = layoutFloors({ box: { width: 1200, height: 600 }, rects: RECTS, levels: BOTH_FLOORS });
  const zoomed = zoomFloors({ pane: layout.panes[0], view: WHOLE_FLOORS, cursor: { x: layout.panes[0].width / 2, y: layout.panes[0].height / 2 }, factor: 3 });
  const moved = panFloors({ pane: layout.panes[0], view: zoomed, dx: -30, dy: 0 });
  assert.ok(moved.panX > zoomed.panX);
  assert.equal(moved.panY, zoomed.panY);
  const [a, b] = layout.panes.map((pane) => floorWindow(pane.rect, moved).x - floorWindow(pane.rect, zoomed).x);
  assert.ok(a > 0);
  assert.ok(Math.abs(a - b) < 1e-9);
});

test('following a player on the lower pane references that pane, not the first', () => {
  const layout = layoutFloors({ box: { width: 1200, height: 600 }, rects: RECTS, levels: BOTH_FLOORS });
  const point = { x: 0.62, y: 0.4 };
  const view = followFloors({ pane: layout.panes[1], view: WHOLE_FLOORS, point });
  assert.ok(view.zoom > 1);
  const window = floorWindow(RECTS.lower, view);
  assert.ok(Math.abs(window.x + window.width / 2 - point.x) < 1e-9);
  assert.ok(Math.abs(window.y + window.height / 2 - point.y) < 1e-9);
});

test('a window turns back into the view that shows it, and the whole extent is the whole view', () => {
  const view = { zoom: 2.5, panX: 0.03, panY: -0.02 };
  const back = viewOfWindow(RECTS.default, floorWindow(RECTS.default, view));
  assert.ok(Math.abs(back.zoom - view.zoom) < 1e-9);
  assert.ok(Math.abs(back.panX - view.panX) < 1e-9);
  assert.deepEqual(viewOfWindow(RECTS.default, RECTS.default), WHOLE_FLOORS);
});

test('the framing is remembered per map until it is back to the whole map', () => {
  const view = { zoom: 3, panX: 0.1, panY: 0 };
  rememberFloorView('de_nuke', view);
  assert.deepEqual(rememberedFloorView('de_nuke'), view);
  assert.deepEqual(rememberedFloorView('de_vertigo'), WHOLE_FLOORS);
  rememberFloorView('de_nuke', WHOLE_FLOORS);
  assert.deepEqual(rememberedFloorView('de_nuke'), WHOLE_FLOORS);
});
