// Unit tests for the client data-plane addressing (same-origin local proxy).
// Run: node --test dataplane.test.ts
import test from 'node:test';
import assert from 'node:assert/strict';
import { dataPlane } from './dataplane.ts';

const JOB = '11111111-1111-4111-8111-111111111111';
const SERIES = '22222222-2222-4222-8222-222222222222';

test('targets the same-origin /api/demos proxy with no auth header', () => {
  const dp = dataPlane();
  assert.deepEqual(dp.headers, {});
  assert.equal(dp.scanUrl, '/api/demos/scan');
  assert.equal(dp.scanField, 'demo');
  assert.equal(dp.scanSeriesField, 'series_id');
  assert.equal(dp.jobStatusUrl(JOB), `/api/demos/${JOB}/status`);
  assert.equal(dp.jobDeleteUrl(JOB), `/api/demos/${JOB}`);
  assert.equal(dp.rosterUrl(JOB), `/api/demos/${JOB}/roster`);
  assert.equal(dp.jobsUrl, '/api/demos/jobs');
  assert.equal(dp.seriesUrl(SERIES), `/api/demos/series/${SERIES}`);
  assert.equal(dp.parseUrl(JOB), `/api/demos/${JOB}/parse`);
  assert.equal(dp.planUrl(JOB), `/api/demos/${JOB}/plan`);
  assert.equal(dp.recapPlanUrl(JOB), `/api/demos/${JOB}/recap-plan`);
  assert.equal(dp.recordUrl(JOB), `/api/demos/${JOB}/record`);
  assert.equal(dp.generateUrl(JOB), `/api/demos/${JOB}/generate`);
  assert.equal(dp.renderUrl(JOB, 'viral-60-clean'), `/api/demos/${JOB}/renders/viral-60-clean`);
  assert.equal(dp.videoUrl(JOB, 'viral-60-clean', 'a_b'), `/api/demos/${JOB}/renders/viral-60-clean/videos/a_b`);
  assert.equal(
    dp.publishAssistantUrl(JOB, 'viral-60-clean', 'a_b'),
    `/api/demos/${JOB}/renders/viral-60-clean/videos/a_b/publish-assistant?days=7`,
  );
  assert.equal(dp.coverUrl(JOB, 'viral-60-clean', 'a_b'), `/api/demos/${JOB}/renders/viral-60-clean/covers/a_b`);
  assert.equal(dp.capabilitiesUrl, '/api/capabilities');
});

test('cloud calls stay on the same-origin proxy, never on the portal', () => {
  const dp = dataPlane();
  assert.equal(dp.cloudAccountUrl, '/api/cloud/account');
  assert.equal(dp.cloudLinkUrl, '/api/cloud/link');
  assert.equal(dp.cloudJobsUrl, '/api/cloud/jobs');
  assert.equal(dp.cloudJobUrl(JOB), `/api/cloud/jobs/${JOB}`);
  assert.equal(dp.cloudJobCancelUrl(JOB), `/api/cloud/jobs/${JOB}/cancel`);
  assert.equal(dp.cloudVideoUrl(JOB, 'short-01.mp4'), `/api/cloud/jobs/${JOB}/videos/short-01.mp4`);
  assert.equal(dp.cloudVideoUrl(JOB, '../x y'), `/api/cloud/jobs/${JOB}/videos/..%2Fx%20y`);
});

test('reads the proxy scan response key and posts { steamId }', () => {
  const dp = dataPlane();
  assert.equal(dp.scanJobId({ jobId: JOB }), JOB);
  assert.equal(dp.scanJobId({ id: 'ignored' }), '');
  assert.deepEqual(dp.parseBody('76561198000000001'), { steamId: '76561198000000001' });
});
