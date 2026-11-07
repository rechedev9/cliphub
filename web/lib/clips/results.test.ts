import test from 'node:test';
import assert from 'node:assert/strict';
import type { StreamJob } from '../api/streams.ts';
import type { Video } from '../api/types.ts';
import { FULL_DEMO_EDIT } from '../full-demo.ts';
import { OUTPUT_STATE, type HubModel } from './hub.ts';
import {
  itemStage,
  libraryChipCounts,
  libraryFilterCounts,
  librarySummary,
  outputStateCount,
  streamActivity,
  videoKind,
  type LibraryItem,
  type LibrarySummaryCounts,
} from './results.ts';

const NONE: LibrarySummaryCounts = {
  ready: 0, recording: 0, working: 0, queued: 0, failed: 0, analysing: 0, streamsDownloading: 0, streamsExporting: 0,
};

test('the summary names ready videos first and skips empty counts', () => {
  assert.equal(librarySummary({ ...NONE, ready: 6, recording: 1 }), '6 listos, 1 grabándose ahora.');
  assert.equal(librarySummary({ ...NONE, ready: 1, working: 2, queued: 3, failed: 1 }), '1 listo, 2 en marcha, 3 en cola, 1 falló.');
  assert.equal(librarySummary({ ...NONE, failed: 2 }), '2 fallaron.');
  assert.equal(librarySummary(NONE), 'Nada en marcha.');
});

test('the summary also tells about demos being analysed and stream projects at work', () => {
  assert.equal(
    librarySummary({ ...NONE, ready: 2, analysing: 1, streamsDownloading: 2, streamsExporting: 1 }),
    '2 listos, 1 demo analizándose, 2 proyectos de stream descargándose, 1 proyecto de stream exportándose.',
  );
  assert.equal(librarySummary({ ...NONE, analysing: 3 }), '3 demos analizándose.');
});

test('outputs count by state', () => {
  const clips = [OUTPUT_STATE.queue, OUTPUT_STATE.rec, OUTPUT_STATE.queue, OUTPUT_STATE.ready].map((state) => ({ state }));
  assert.equal(outputStateCount({ clips } as unknown as Pick<HubModel, 'clips'>, OUTPUT_STATE.queue), 2);
});

test('a demo video files under its format and the chips count every kind', () => {
  const base = { id: 'v', title: 't', map: 'de_mirage', score: '', mode: 'viral', status: 'ready', createdAt: 1 } as unknown as Video;
  assert.equal(videoKind(base), 'short');
  assert.equal(videoKind({ ...base, editConfig: FULL_DEMO_EDIT }), 'full');
  assert.equal(videoKind({ ...base, editConfig: { ...FULL_DEMO_EDIT, matchRecap: false } }), 'summary');
  assert.deepEqual(libraryFilterCounts(['short', 'short', 'stream', 'summary']), { all: 4, short: 2, full: 0, summary: 1, stream: 1 });
});

test('a failed video is neither ready nor en marcha', () => {
  assert.equal(itemStage(OUTPUT_STATE.ready), 'ready');
  assert.equal(itemStage(OUTPUT_STATE.queue), 'working');
  assert.equal(itemStage(OUTPUT_STATE.rec), 'working');
  assert.equal(itemStage(OUTPUT_STATE.failed), 'failed');
});

test('each chip group counts what the other group lets through', () => {
  const items: LibraryItem[] = [
    { kind: 'short', stage: 'ready' },
    { kind: 'short', stage: 'working' },
    { kind: 'full', stage: 'working' },
    { kind: 'stream', stage: 'working' },
    { kind: 'short', stage: 'failed' },
  ];
  const all = libraryChipCounts(items, { format: 'all', stage: 'all' });
  assert.deepEqual(all.format, { all: 5, short: 3, full: 1, summary: 0, stream: 1 });
  assert.deepEqual(all.stage, { all: 5, ready: 1, working: 3 });
  const narrowed = libraryChipCounts(items, { format: 'short', stage: 'working' });
  assert.deepEqual(narrowed.format, { all: 3, short: 1, full: 1, summary: 0, stream: 1 });
  assert.deepEqual(narrowed.stage, { all: 3, ready: 1, working: 1 });
});

test('stream activity lists downloads and exports with their progress', () => {
  const jobs: StreamJob[] = [
    { id: 'd', status: 'acquiring', title: 'VOD', created_at: '', download_progress: { downloaded_bytes: 1, percent: 41.6 } },
    { id: 'l', status: 'acquiring', title: 'Directo', created_at: '' },
    { id: 'e', status: 'rendering', title: 'Clip', created_at: '', render_progress: { variant: 'v', percent: 37, clip: 2, clips: 4 } },
    { id: 'r', status: 'rendered', title: 'Hecho', created_at: '' },
  ];
  assert.deepEqual(streamActivity(jobs), [
    { jobId: 'd', title: 'VOD', stage: 'download', percent: 42, clipLabel: null },
    { jobId: 'l', title: 'Directo', stage: 'download', percent: null, clipLabel: null },
    { jobId: 'e', title: 'Clip', stage: 'export', percent: 37, clipLabel: 'Short 2 de 4' },
  ]);
});
