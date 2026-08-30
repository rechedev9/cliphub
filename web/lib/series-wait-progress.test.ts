import test from 'node:test';
import assert from 'node:assert/strict';
import { seriesWaitProgress } from './series-wait-progress.ts';
import type { SeriesDemo } from './api/types.ts';
import type { SeriesGroup } from './series-grouping.ts';

function group(demos: SeriesDemo[]): SeriesGroup<SeriesDemo> {
  return { key: demos[0]?.jobId ?? 'g', mapOrder: null, demos };
}

test('series LongOperation during Shorts recording keeps segmentos, not rondas', () => {
  const recording: SeriesDemo = {
    jobId: 'j1',
    status: 'recording',
    progress: { done: 2, total: 8, percent: 25, unit: 'segments', label: 'segmentos', stage: 'record' },
  };
  const parsed: SeriesDemo = { jobId: 'j2', status: 'parsed' };
  const got = seriesWaitProgress([group([recording]), group([parsed])], [recording, parsed]);
  assert.deepEqual(got, recording.progress);
  assert.equal(got.label, 'segmentos');
  assert.notEqual(got.label, 'rondas');
});

test('series wait without a snapshot uses maps, not invented 0 / 0', () => {
  const recording: SeriesDemo = { jobId: 'j1', status: 'recording' };
  const parsed: SeriesDemo = { jobId: 'j2', status: 'parsed' };
  const got = seriesWaitProgress([group([recording]), group([parsed])], [recording, parsed]);
  assert.deepEqual(got, { done: 1, total: 2, unit: 'maps', label: 'mapas' });
});
