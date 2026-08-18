import test from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { fileURLToPath } from 'node:url';
import { defaultEditorDocument, type EditorDocument } from './evaluate.ts';
import { validateDocument, validateForRender } from './validate.ts';

type ParityCase = {
  name: string;
  doc: EditorDocument;
  validateError: string | null;
  renderError: string | null;
};

const parityCases = JSON.parse(
  readFileSync(fileURLToPath(new URL('./testdata/parity-plans.json', import.meta.url)), 'utf8'),
) as ParityCase[];

test('validate parity with Go fixtures', () => {
  for (const tc of parityCases) {
    const validate = validateDocument(tc.doc);
    const render = validateForRender(tc.doc);
    assert.deepEqual(validate, tc.validateError === null ? [] : [tc.validateError], `${tc.name} validate`);
    assert.deepEqual(render, tc.renderError === null ? [] : [tc.renderError], `${tc.name} render`);
  }
});

test('default draft validates and cannot render', () => {
  const doc = defaultEditorDocument();
  assert.deepEqual(validateDocument(doc), []);
  assert.deepEqual(validateForRender(doc), ['timeline has no items']);
});

test('item and overlay messages match Go', () => {
  const base = (): EditorDocument => ({
    schema_version: '1.0',
    canvas: { width: 1080, height: 1920, fps: 60 },
    tracks: [
      {
        id: 'v1',
        kind: 'video',
        items: [
          {
            id: 'clip-1',
            asset_id: '11111111-1111-1111-1111-111111111111',
            timeline_start: 0,
            source_in: 0,
            source_out: 2,
          },
        ],
      },
    ],
  });
  const cases: { name: string; mutate: (doc: EditorDocument) => void; want: string }[] = [
    {
      name: 'bad schema',
      mutate: (doc) => {
        doc.schema_version = '9.9';
      },
      want: 'schema_version must be "1.0"',
    },
    {
      name: 'bad fps',
      mutate: (doc) => {
        doc.canvas.fps = 30;
      },
      want: 'canvas fps must be 60',
    },
    {
      name: 'no tracks',
      mutate: (doc) => {
        doc.tracks = [];
      },
      want: 'timeline needs at least one track',
    },
    {
      name: 'too many tracks',
      mutate: (doc) => {
        doc.tracks = Array.from({ length: 9 }, (_, i) => ({
          id: `v${i + 1}`,
          kind: 'video' as const,
          items: [],
        }));
      },
      want: 'timeline has at most 8 tracks',
    },
    {
      name: 'volume too high',
      mutate: (doc) => {
        const item = doc.tracks[0]?.items[0];
        if (item) item.volume = 3;
      },
      want: 'item clip-1 volume must be between 0 and 2',
    },
    {
      name: 'fade too long',
      mutate: (doc) => {
        const item = doc.tracks[0]?.items[0];
        if (item) item.fade_in = 6;
      },
      want: 'item clip-1 fade_in must be between 0 and 5',
    },
    {
      name: 'fades exceed output',
      mutate: (doc) => {
        const item = doc.tracks[0]?.items[0];
        if (item) {
          item.fade_in = 1.2;
          item.fade_out = 1.2;
        }
      },
      want: 'item clip-1 fades must fit within the item output duration',
    },
    {
      name: 'pip out of frame',
      mutate: (doc) => {
        const item = doc.tracks[0]?.items[0];
        if (item) item.transform = { x: 0.8, y: 0.8, width: 0.5, height: 0.5 };
      },
      want: 'item clip-1 transform must stay within the canvas',
    },
  ];
  for (const tc of cases) {
    const doc = base();
    tc.mutate(doc);
    assert.deepEqual(validateDocument(doc), [tc.want], tc.name);
    assert.deepEqual(validateForRender(doc), [tc.want], tc.name);
  }
});
