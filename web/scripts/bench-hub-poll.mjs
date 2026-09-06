// node web/scripts/bench-hub-poll.mjs
// Idle hub rebuild over 50 roster-inlined jobs. Not an Electron FPS test.
import assert from 'node:assert/strict';
import { performance } from 'node:perf_hooks';
import { enrichmentFromSummary, jobToMatch, listableJobs } from '../lib/api/jobs-index.ts';
import { buildHubModel, hubPollUnchanged } from '../lib/clips/hub.ts';

function listedJob(index) {
  const jobId = `11111111-1111-4111-8111-${String(index).padStart(12, '0')}`;
  return {
    jobId,
    status: 'parsed',
    createdAt: '2026-09-06T12:00:00Z',
    fileName: `match-${String(index).padStart(2, '0')}.dem`,
    targetSteamId: '76561198000000002',
    summary: {
      match: { map: 'de_mirage', score_ct: 9, score_t: 13, rounds: 22 },
      target: {
        steamid64: '76561198000000002',
        name: 'donk',
        team: 'T',
        kills: 20 + index,
        deaths: 10,
        assists: 3,
      },
    },
  };
}

const jobs = Array.from({ length: 50 }, (_, index) => listedJob(index));
const matches = listableJobs(jobs).map((job) => jobToMatch(job, enrichmentFromSummary(job) ?? undefined));
const videos = [];
const snapshot = { matches, videos, streams: [], failure: null };
const model = buildHubModel(matches, videos);
assert.equal(model.rows.length, 50);
assert.equal(hubPollUnchanged(snapshot, model, buildHubModel(matches, videos), snapshot), true);

function measure(fn) {
  const start = performance.now();
  for (let i = 0; i < 2000; i++) fn();
  return performance.now() - start;
}

const samples = [];
for (let i = 0; i < 5; i++) {
  samples.push(measure(() => {
    const next = buildHubModel(matches, videos);
    if (!hubPollUnchanged(snapshot, model, next, snapshot)) throw new Error('idle poll looked changed');
  }));
}
samples.sort((a, b) => a - b);
const median = samples[2];
console.log(JSON.stringify({
  operation: 'rebuild 50-job hub model and detect unchanged poll',
  iterations: 2000,
  samplesMs: samples.map((value) => Number(value.toFixed(3))),
  medianMs: Number(median.toFixed(3)),
}, null, 2));
