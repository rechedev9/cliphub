import test from 'node:test';
import assert from 'node:assert/strict';
import { layerZones, parsePlatformLayer, PLATFORMS, platformZones, type ZoneRect } from './platform-zones.ts';

test('every area of every platform lies inside the 9:16 frame and has a size', () => {
  for (const platform of PLATFORMS) {
    const zones = platformZones(platform);
    assert.ok(zones.length > 0, `${platform} covers something`);
    for (const zone of zones) {
      assert.ok(zone.left >= 0 && zone.top >= 0, `${platform} starts inside the frame`);
      assert.ok(zone.right <= 1 && zone.bottom <= 1, `${platform} ends inside the frame`);
      assert.ok(zone.right > zone.left && zone.bottom > zone.top, `${platform} has a size`);
    }
  }
});

function covers(zones: readonly ZoneRect[], x: number, y: number): boolean {
  return zones.some((zone) => x >= zone.left && x < zone.right && y >= zone.top && y < zone.bottom);
}

const GRID = Array.from({ length: 200 }, (_, at) => (at + 0.5) / 200);

test('"Todas" covers every point a platform covers and nothing that none covers', () => {
  const all = layerZones('todas');
  for (const x of GRID) {
    for (const y of GRID) {
      const byAny = PLATFORMS.some((platform) => covers(platformZones(platform), x, y));
      assert.equal(covers(all, x, y), byAny, `point ${x}, ${y}`);
    }
  }
});

test('one platform draws only its own areas and "Ninguna" draws none', () => {
  assert.equal(covers(layerZones('youtube-shorts'), 0.83, 0.45), true);
  assert.equal(covers(layerZones('reels'), 0.83, 0.45), false);
  assert.equal(covers(layerZones('reels'), 0.5, 0.145), false);
  assert.equal(covers(layerZones('todas'), 0.5, 0.145), true);
  assert.deepEqual(layerZones('ninguna'), []);
});

test('the remembered choice falls back to "Todas" when it is missing or unknown', () => {
  assert.equal(parsePlatformLayer('reels'), 'reels');
  assert.equal(parsePlatformLayer('ninguna'), 'ninguna');
  assert.equal(parsePlatformLayer(null), 'todas');
  assert.equal(parsePlatformLayer('instagram'), 'todas');
});
