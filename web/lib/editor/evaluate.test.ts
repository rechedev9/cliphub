import test from 'node:test';
import assert from 'node:assert/strict';
import { evaluateTimeline, normalizeDocument, type EditorDocument } from './evaluate.ts';

const doc: EditorDocument = {
  schema_version: '1.0',
  canvas: { width: 1080, height: 1920, fps: 60 },
  tracks: [
    {
      id: 'v1',
      kind: 'video',
      items: [
        {
          id: 'base',
          asset_id: '11111111-1111-1111-1111-111111111111',
          timeline_start: 0,
          source_in: 1,
          source_out: 5,
          fade_in: 0.5,
        },
      ],
    },
    {
      id: 'v2',
      kind: 'video',
      items: [
        {
          id: 'pip',
          asset_id: '22222222-2222-2222-2222-222222222222',
          timeline_start: 1,
          source_in: 0,
          source_out: 1,
          speed: 2,
          transform: { x: 0.6, y: 0.05, width: 0.35, height: 0.25, opacity: 0.8 },
        },
      ],
    },
  ],
  overlays: [{ id: 'title', text: 'ACE', position_y: 0.1, start_seconds: 0.2, end_seconds: 4, font_size: 72 }],
};

test('evaluate: fade-in base layer', () => {
  const sample = evaluateTimeline(doc, 0.25);
  assert.equal(sample.layers.length, 1);
  assert.equal(sample.layers[0]?.item_id, 'base');
  assert.equal(sample.layers[0]?.opacity, 0.5);
  assert.equal(sample.layers[0]?.source_time, 1.25);
});

test('evaluate: stacked pip and text', () => {
  const sample = evaluateTimeline(doc, 1.1);
  assert.equal(sample.layers.length, 2);
  assert.equal(sample.texts.length, 1);
  assert.equal(sample.texts[0]?.font_size, 72);
});

test('evaluate: after pip ends', () => {
  const sample = evaluateTimeline(doc, 1.6);
  assert.equal(sample.layers.length, 1);
  assert.equal(sample.layers[0]?.item_id, 'base');
});

test('normalize: null or missing items become arrays', () => {
  const cases: { name: string; raw: string }[] = [
    {
      name: 'null items',
      raw: '{"schema_version":"1.0","canvas":{"width":1080,"height":1920,"fps":60},"tracks":[{"id":"v1","kind":"video","items":null}]}',
    },
    {
      name: 'missing items',
      raw: '{"schema_version":"1.0","canvas":{"width":1080,"height":1920,"fps":60},"tracks":[{"id":"v1","kind":"video"}]}',
    },
  ];
  for (const tc of cases) {
    const got = normalizeDocument(JSON.parse(tc.raw) as EditorDocument);
    assert.deepEqual(got.tracks[0]?.items, [], tc.name);
    assert.doesNotThrow(() => evaluateTimeline(got, 0), tc.name);
  }
});
