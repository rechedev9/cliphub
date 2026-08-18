import test from 'node:test';
import assert from 'node:assert/strict';
import { defaultEditorDocument, itemTimelineEnd, type EditorDocument } from './evaluate.ts';
import {
  addItem,
  addOverlay,
  addTrack,
  deleteItem,
  deleteOverlay,
  deleteTrack,
  duplicateItem,
  moveItem,
  removeTransition,
  setCanvas,
  setItemProps,
  setMusic,
  setTransitionAfter,
  splitItemAt,
  trimItem,
  updateOverlay,
} from './document.ts';
import { EDITOR_LIMITS } from './validate.ts';

const ASSET = { id: '11111111-1111-1111-1111-111111111111', probe: { duration_seconds: 4 } };

function clip(doc: EditorDocument, index = 0) {
  return doc.tracks[0]?.items[index];
}

test('addItem success and no-op cases', () => {
  const cases: {
    name: string;
    doc: () => EditorDocument;
    trackId: string;
    asset: typeof ASSET | { id: string; probe: { duration_seconds?: number } };
    at?: number;
    wantCount: number;
    wantOut?: number;
    wantStart?: number;
    sameRef?: boolean;
  }[] = [
    {
      name: 'appends at duration with probe length',
      doc: defaultEditorDocument,
      trackId: 'v1',
      asset: ASSET,
      wantCount: 1,
      wantOut: 4,
      wantStart: 0,
    },
    {
      name: 'missing duration defaults to 2',
      doc: defaultEditorDocument,
      trackId: 'v1',
      asset: { id: ASSET.id, probe: {} },
      wantCount: 1,
      wantOut: 2,
    },
    {
      name: 'clamps tiny duration to 0.2',
      doc: defaultEditorDocument,
      trackId: 'v1',
      asset: { id: ASSET.id, probe: { duration_seconds: 0.05 } },
      wantCount: 1,
      wantOut: 0.2,
    },
    {
      name: 'honors explicit at',
      doc: defaultEditorDocument,
      trackId: 'v1',
      asset: ASSET,
      at: 1.5,
      wantCount: 1,
      wantStart: 1.5,
    },
    {
      name: 'missing track is no-op',
      doc: defaultEditorDocument,
      trackId: 'missing',
      asset: ASSET,
      wantCount: 0,
      sameRef: true,
    },
  ];
  for (const tc of cases) {
    const doc = tc.doc();
    const next = addItem(doc, tc.trackId, tc.asset, tc.at);
    assert.equal(next.tracks[0]?.items.length, tc.wantCount, tc.name);
    if (tc.sameRef) assert.equal(next, doc, tc.name);
    else {
      assert.notEqual(next, doc, tc.name);
      assert.equal(doc.tracks[0]?.items.length, 0, `${tc.name} immutable`);
    }
    if (tc.wantOut !== undefined) assert.equal(clip(next)?.source_out, tc.wantOut, tc.name);
    if (tc.wantStart !== undefined) assert.equal(clip(next)?.timeline_start, tc.wantStart, tc.name);
    if (tc.wantCount > 0) assert.match(clip(next)?.id ?? '', /^clip-[0-9a-z]+$/, tc.name);
  }
});

test('addItem no-op at item limit', () => {
  let doc = defaultEditorDocument();
  for (let i = 0; i < EDITOR_LIMITS.maxItemsPerTrack; i += 1) {
    doc = addItem(doc, 'v1', ASSET, i);
  }
  const blocked = addItem(doc, 'v1', ASSET, 99);
  assert.equal(blocked, doc);
  assert.equal(blocked.tracks[0]?.items.length, EDITOR_LIMITS.maxItemsPerTrack);
});

test('move trim split duplicate delete item', () => {
  const seeded = addItem(defaultEditorDocument(), 'v1', ASSET, 0);
  const cases: { name: string; run: (doc: EditorDocument) => EditorDocument; check: (doc: EditorDocument, src: EditorDocument) => void }[] = [
    {
      name: 'moveItem',
      run: (doc) => moveItem(doc, clip(doc)?.id ?? '', 3),
      check: (doc) => assert.equal(clip(doc)?.timeline_start, 3),
    },
    {
      name: 'moveItem clamps negative',
      run: (doc) => moveItem(doc, clip(doc)?.id ?? '', -2),
      check: (doc) => assert.equal(clip(doc)?.timeline_start, 0),
    },
    {
      name: 'moveItem missing is no-op',
      run: (doc) => moveItem(doc, 'missing', 1),
      check: (doc, src) => assert.equal(doc, src),
    },
    {
      name: 'trimItem',
      run: (doc) => trimItem(doc, clip(doc)?.id ?? '', 0.5, 2.5),
      check: (doc) => {
        assert.equal(clip(doc)?.source_in, 0.5);
        assert.equal(clip(doc)?.source_out, 2.5);
      },
    },
    {
      name: 'trimItem clamps to asset duration',
      run: (doc) => trimItem(doc, clip(doc)?.id ?? '', -1, 9, 3),
      check: (doc) => {
        assert.equal(clip(doc)?.source_in, 0);
        assert.equal(clip(doc)?.source_out, 3);
      },
    },
    {
      name: 'trimItem inverted is no-op',
      run: (doc) => trimItem(doc, clip(doc)?.id ?? '', 3, 1),
      check: (doc, src) => assert.equal(doc, src),
    },
    {
      name: 'splitItemAt midpoint',
      run: (doc) => splitItemAt(doc, clip(doc)?.id ?? '', 2),
      check: (doc) => {
        assert.equal(doc.tracks[0]?.items.length, 2);
        assert.equal(clip(doc, 0)?.source_out, 2);
        assert.equal(clip(doc, 1)?.source_in, 2);
        assert.equal(clip(doc, 1)?.timeline_start, 2);
      },
    },
    {
      name: 'splitItemAt outside is no-op',
      run: (doc) => splitItemAt(doc, clip(doc)?.id ?? '', 0),
      check: (doc, src) => assert.equal(doc, src),
    },
    {
      name: 'duplicateItem places after original',
      run: (doc) => duplicateItem(doc, clip(doc)?.id ?? ''),
      check: (doc, src) => {
        assert.equal(doc.tracks[0]?.items.length, 2);
        const original = clip(src);
        assert.ok(original);
        assert.equal(clip(doc, 1)?.timeline_start, itemTimelineEnd(original));
        assert.notEqual(clip(doc, 1)?.id, original.id);
      },
    },
    {
      name: 'deleteItem removes clip',
      run: (doc) => deleteItem(doc, clip(doc)?.id ?? ''),
      check: (doc) => assert.equal(doc.tracks[0]?.items.length, 0),
    },
    {
      name: 'deleteItem missing is no-op',
      run: (doc) => deleteItem(doc, 'missing'),
      check: (doc, src) => assert.equal(doc, src),
    },
  ];
  for (const tc of cases) {
    const next = tc.run(seeded);
    tc.check(next, seeded);
    assert.equal(seeded.tracks[0]?.items.length, 1, `${tc.name} source intact`);
  }
});

test('deleteItem drops transitions after that item', () => {
  const seeded = addItem(defaultEditorDocument(), 'v1', ASSET, 0);
  const id = clip(seeded)?.id ?? '';
  const withTr = setTransitionAfter(seeded, id, 'crossfade', 0.4);
  const next = deleteItem(withTr, id);
  assert.equal(next.tracks[0]?.items.length, 0);
  assert.deepEqual(next.transitions, []);
});

test('tracks respect the 8-track cap and last video', () => {
  const cases: { name: string; run: (doc: EditorDocument) => EditorDocument; wantTracks: number; sameAs?: 'in' }[] = [
    { name: 'add video track', run: (doc) => addTrack(doc, 'video'), wantTracks: 2 },
    { name: 'add audio track', run: (doc) => addTrack(doc, 'audio'), wantTracks: 2 },
    { name: 'delete missing track', run: (doc) => deleteTrack(doc, 'nope'), wantTracks: 1 },
    { name: 'keep last video track', run: (doc) => deleteTrack(doc, 'v1'), wantTracks: 1 },
  ];
  for (const tc of cases) {
    const doc = defaultEditorDocument();
    const next = tc.run(doc);
    assert.equal(next.tracks.length, tc.wantTracks, tc.name);
    if (tc.wantTracks === 1 && tc.name !== 'add video track') assert.equal(next, doc, tc.name);
  }
  let filled = defaultEditorDocument();
  for (let i = 0; i < 7; i += 1) filled = addTrack(filled, i % 2 === 0 ? 'audio' : 'video');
  assert.equal(filled.tracks.length, 8);
  assert.equal(addTrack(filled, 'audio'), filled);
  const withAudio = addTrack(defaultEditorDocument(), 'audio');
  const audioId = withAudio.tracks[1]?.id ?? '';
  const removed = deleteTrack(withAudio, audioId);
  assert.equal(removed.tracks.length, 1);
  assert.equal(removed.tracks[0]?.id, 'v1');
});

test('setItemProps overlays transitions music canvas', () => {
  const seeded = addItem(defaultEditorDocument(), 'v1', ASSET, 0);
  const id = clip(seeded)?.id ?? '';
  const cases: { name: string; run: (doc: EditorDocument) => EditorDocument; check: (doc: EditorDocument, src: EditorDocument) => void }[] = [
    {
      name: 'setItemProps speed',
      run: (doc) => setItemProps(doc, id, { speed: 1.5, volume: 0.4 }),
      check: (doc) => {
        assert.equal(clip(doc)?.speed, 1.5);
        assert.equal(clip(doc)?.volume, 0.4);
      },
    },
    {
      name: 'setItemProps missing is no-op',
      run: (doc) => setItemProps(doc, 'missing', { speed: 2 }),
      check: (doc, src) => assert.equal(doc, src),
    },
    {
      name: 'addOverlay',
      run: (doc) => addOverlay(doc, { text: 'ACE', position_y: 0.1, start_seconds: 0, font_size: 72 }),
      check: (doc) => {
        assert.equal(doc.overlays?.length, 1);
        assert.equal(doc.overlays?.[0]?.text, 'ACE');
      },
    },
    {
      name: 'updateOverlay',
      run: (doc) => {
        const withOv = addOverlay(doc, { id: 'title', text: 'ACE', position_y: 0.1, start_seconds: 0 });
        return updateOverlay(withOv, 'title', { text: 'CLUTCH' });
      },
      check: (doc) => assert.equal(doc.overlays?.[0]?.text, 'CLUTCH'),
    },
    {
      name: 'deleteOverlay',
      run: (doc) => {
        const withOv = addOverlay(doc, { id: 'title', text: 'ACE', position_y: 0.1, start_seconds: 0 });
        return deleteOverlay(withOv, 'title');
      },
      check: (doc) => assert.deepEqual(doc.overlays, []),
    },
    {
      name: 'overlay missing ops are no-op',
      run: (doc) => deleteOverlay(updateOverlay(doc, 'nope', { text: 'x' }), 'nope'),
      check: (doc, src) => assert.equal(doc, src),
    },
    {
      name: 'setTransitionAfter',
      run: (doc) => setTransitionAfter(doc, id, 'crossfade', 0.3),
      check: (doc) => {
        assert.equal(doc.transitions?.length, 1);
        assert.equal(doc.transitions?.[0]?.after_item, id);
        assert.equal(doc.transitions?.[0]?.kind, 'crossfade');
      },
    },
    {
      name: 'setTransitionAfter replaces same item',
      run: (doc) => setTransitionAfter(setTransitionAfter(doc, id, 'crossfade', 0.3), id, 'cut'),
      check: (doc) => {
        assert.equal(doc.transitions?.length, 1);
        assert.equal(doc.transitions?.[0]?.kind, 'cut');
      },
    },
    {
      name: 'removeTransition',
      run: (doc) => {
        const withTr = setTransitionAfter(doc, id, 'cut');
        return removeTransition(withTr, withTr.transitions?.[0]?.id ?? '');
      },
      check: (doc) => assert.deepEqual(doc.transitions, []),
    },
    {
      name: 'setMusic',
      run: (doc) => setMusic(doc, 'pulse', 0.25),
      check: (doc) => assert.deepEqual(doc.music, { key: 'pulse', volume: 0.25 }),
    },
    {
      name: 'setMusic clear',
      run: (doc) => setMusic(setMusic(doc, 'pulse', 0.25), undefined),
      check: (doc) => assert.equal(doc.music, undefined),
    },
    {
      name: 'setCanvas landscape',
      run: (doc) => setCanvas(doc, 'landscape'),
      check: (doc) => assert.deepEqual(doc.canvas, { width: 1920, height: 1080, fps: 60 }),
    },
    {
      name: 'setCanvas portrait',
      run: (doc) => setCanvas(setCanvas(doc, 'landscape'), 'portrait'),
      check: (doc) => assert.deepEqual(doc.canvas, { width: 1080, height: 1920, fps: 60 }),
    },
  ];
  for (const tc of cases) {
    const next = tc.run(seeded);
    tc.check(next, seeded);
  }
});

test('addOverlay no-op at overlay limit', () => {
  let doc = defaultEditorDocument();
  for (let i = 0; i < EDITOR_LIMITS.maxOverlays; i += 1) {
    doc = addOverlay(doc, { text: `t${i}`, position_y: 0.1, start_seconds: 0 });
  }
  const blocked = addOverlay(doc, { text: 'extra', position_y: 0.1, start_seconds: 0 });
  assert.equal(blocked, doc);
});
