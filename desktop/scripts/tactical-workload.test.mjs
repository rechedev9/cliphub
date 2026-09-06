import test from 'node:test';
import assert from 'node:assert/strict';
import { attachTacticalWorkload } from './tactical-workload.mjs';
import { compareReports } from './compare-efficiency.mjs';

function raw(scenario) {
  return { schema_version: 1, scenario, duration_seconds: 15, roles: {}, summary: {
    cpu_p95_percent: 10, working_set_peak_bytes: 1000, private_bytes_peak: 500,
  } };
}

test('tactical comparisons require the same workload and measured timing', () => {
  const report = raw('tactical-analysis');
  assert.equal(compareReports(report, report).verdict, 'incomparable');
  const before = attachTacticalWorkload(report, { scenario: report.scenario, id: 'sha256:demo:8hz', operationMS: 1000 });
  const after = attachTacticalWorkload(report, { scenario: report.scenario, id: 'sha256:demo:8hz', operationMS: 800 });
  assert.equal(compareReports(before, after).verdict, 'improve');
  assert.equal(compareReports(after, before).verdict, 'regress');
  assert.equal(compareReports(before, { ...after, workload_id: 'different-demo' }).verdict, 'incomparable');
  assert.equal(compareReports(before, { ...after, cpu_sampling_version: 2 }).verdict, 'incomparable');
  assert.equal(compareReports(before, { ...after, gpu_counters_enabled: false }).verdict, 'incomparable');
});

test('replay reports carry nearest-rank frame p95/p99; missing and invalid samples fail', () => {
  const report = raw('tactical-replay');
  const workload = { scenario: report.scenario, id: 'demo:round1:size600:dpr1:speed1:60hz', frameIntervalsMS: Array.from({ length: 100 }, (_, i) => i + 1) };
  const measured = attachTacticalWorkload(report, workload);
  assert.equal(measured.summary.frame_p95_ms, 95);
  assert.equal(measured.summary.frame_p99_ms, 99);
  assert.equal(measured.summary.frame_count, 100);
  assert.equal(compareReports(measured, measured).verdict, 'unchanged');
  for (const samples of [[], [1], [NaN, 1], [0, 1], [-1, 1]]) {
    assert.throws(() => attachTacticalWorkload(report, { ...workload, frameIntervalsMS: samples }));
  }
  assert.throws(() => attachTacticalWorkload(report, { ...workload, scenario: 'tactical-analysis' }));
});
