import test from 'node:test';
import assert from 'node:assert/strict';
import { OUTPUT_STATE, type HubModel } from './hub.ts';
import { queuedOutputCount, resultsActivityLabel } from './results.ts';

test('the activity line reports running work before the backlog', () => {
  assert.equal(resultsActivityLabel({ running: 1, queued: 4 }), '1 vídeo en marcha');
  assert.equal(resultsActivityLabel({ running: 2, queued: 0 }), '2 vídeos en marcha');
  assert.equal(resultsActivityLabel({ running: 0, queued: 3 }), '3 en cola');
  assert.equal(resultsActivityLabel({ running: 0, queued: 0 }), 'Nada en marcha');
});

test('only queued outputs count as backlog', () => {
  const clips = [OUTPUT_STATE.queue, OUTPUT_STATE.rec, OUTPUT_STATE.queue, OUTPUT_STATE.ready].map((state) => ({ state }));
  assert.equal(queuedOutputCount({ clips } as unknown as Pick<HubModel, 'clips'>), 2);
});
