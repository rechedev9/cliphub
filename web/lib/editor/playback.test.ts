import test from 'node:test';
import assert from 'node:assert/strict';
import type { EditorDocument, EditorItem } from './evaluate.ts';
import { clampVolumeForPreview, itemPlaybackRate, upcomingItems } from './playback.ts';

const ASSET = '11111111-1111-1111-1111-111111111111';

function item(partial: Partial<EditorItem> & Pick<EditorItem, 'id' | 'timeline_start' | 'source_in' | 'source_out'>): EditorItem {
  return { asset_id: ASSET, ...partial };
}

const doc: EditorDocument = {
  schema_version: '1.0',
  canvas: { width: 1080, height: 1920, fps: 60 },
  tracks: [
    {
      id: 'v1',
      kind: 'video',
      items: [item({ id: 'a', timeline_start: 0, source_in: 0, source_out: 2 }), item({ id: 'b', timeline_start: 3, source_in: 0, source_out: 1 })],
    },
    {
      id: 'a1',
      kind: 'audio',
      items: [item({ id: 'c', timeline_start: 1.5, source_in: 0, source_out: 2, speed: 2 })],
    },
  ],
};

test('upcomingItems intersects the preview window', () => {
  const cases: { name: string; time: number; horizon: number; want: string[] }[] = [
    { name: 'at start', time: 0, horizon: 1, want: ['a'] },
    { name: 'covers first and audio', time: 1.4, horizon: 0.2, want: ['a', 'c'] },
    { name: 'gap before b', time: 2, horizon: 0.5, want: ['c'] },
    { name: 'touches b start', time: 2.5, horizon: 0.5, want: ['b'] },
    { name: 'after everything', time: 5, horizon: 1, want: [] },
    { name: 'full span', time: 0, horizon: 4, want: ['a', 'b', 'c'] },
  ];
  for (const tc of cases) {
    assert.deepEqual(
      upcomingItems(doc, tc.time, tc.horizon).map((entry) => entry.id),
      tc.want,
      tc.name,
    );
  }
});

test('clampVolumeForPreview', () => {
  const cases: { name: string; volume: number | undefined; want: number }[] = [
    { name: 'default', volume: undefined, want: 1 },
    { name: 'zero', volume: 0, want: 0 },
    { name: 'mid', volume: 0.4, want: 0.4 },
    { name: 'one', volume: 1, want: 1 },
    { name: 'clamp high', volume: 1.8, want: 1 },
    { name: 'clamp low', volume: -0.2, want: 0 },
    { name: 'nan', volume: Number.NaN, want: 1 },
  ];
  for (const tc of cases) {
    assert.equal(clampVolumeForPreview(tc.volume), tc.want, tc.name);
  }
});

test('itemPlaybackRate uses itemSpeed', () => {
  const cases: { name: string; item: EditorItem; want: number }[] = [
    { name: 'default', item: item({ id: 'x', timeline_start: 0, source_in: 0, source_out: 1 }), want: 1 },
    { name: 'explicit', item: item({ id: 'y', timeline_start: 0, source_in: 0, source_out: 1, speed: 2 }), want: 2 },
    { name: 'zero treated as 1', item: item({ id: 'z', timeline_start: 0, source_in: 0, source_out: 1, speed: 0 }), want: 1 },
  ];
  for (const tc of cases) {
    assert.equal(itemPlaybackRate(tc.item), tc.want, tc.name);
  }
});
