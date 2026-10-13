import test from 'node:test';
import assert from 'node:assert/strict';
import { OUTPUT_STATE, type HubModel } from './hub.ts';
import { queuedOutputCount, resultsActivityLabel, resultsCounts, resultsOrder, resultsSections } from './results.ts';
import { RESULTS_ORIGIN } from './routes.ts';

test('the counts add both origins', () => {
  assert.deepEqual(resultsCounts({ local: 3, cloud: 2 }), { all: 5, local: 3, cloud: 2 });
});

test('without a filter an empty origin has no section', () => {
  assert.deepEqual(resultsSections(null, resultsCounts({ local: 3, cloud: 0 })), { local: true, cloud: false });
  assert.deepEqual(resultsSections(null, resultsCounts({ local: 0, cloud: 1 })), { local: false, cloud: true });
  assert.deepEqual(resultsSections(null, resultsCounts({ local: 2, cloud: 1 })), { local: true, cloud: true });
});

test('a filter shows its origin even when it is empty, so the page can say why', () => {
  const counts = resultsCounts({ local: 3, cloud: 0 });
  assert.deepEqual(resultsSections(RESULTS_ORIGIN.cloud, counts), { local: false, cloud: true });
  assert.deepEqual(resultsSections(RESULTS_ORIGIN.local, resultsCounts({ local: 0, cloud: 4 })), { local: true, cloud: false });
});

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

test('resultsOrder puts work in flight first, then the newest video, and this PC on a tie', () => {
  const idle = { active: false, newest: 0 };
  assert.deepEqual(resultsOrder({ local: idle, cloud: idle }), ['pc', 'nube']);
  assert.deepEqual(resultsOrder({ local: { active: false, newest: 5 }, cloud: { active: false, newest: 9 } }), ['nube', 'pc']);
  assert.deepEqual(resultsOrder({ local: { active: true, newest: 5 }, cloud: { active: false, newest: 9 } }), ['pc', 'nube']);
  assert.deepEqual(resultsOrder({ local: { active: true, newest: 9 }, cloud: { active: true, newest: 5 } }), ['pc', 'nube']);
  assert.deepEqual(resultsOrder({ local: { active: false, newest: 9 }, cloud: { active: true, newest: 5 } }), ['nube', 'pc']);
});
