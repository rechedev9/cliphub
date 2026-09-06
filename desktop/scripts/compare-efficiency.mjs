import { readFileSync } from 'node:fs';
import { pathToFileURL } from 'node:url';

const SCHEMA_VERSION = 1;

export const SCENARIOS = Object.freeze([
  'foreground-idle',
  'background-idle',
  'stream-static',
  'stream-playback',
  'tactical-analysis',
  'tactical-replay',
]);

export const COMPARE_METRICS = Object.freeze([
  'cpu_p95_percent',
  'working_set_peak_bytes',
  'private_bytes_peak',
]);

const SCENARIO_SET = new Set(SCENARIOS);

function metricsFor(scenario) {
  if (scenario === 'tactical-analysis') return [...COMPARE_METRICS, 'operation_ms'];
  if (scenario === 'tactical-replay') return [...COMPARE_METRICS, 'frame_p95_ms', 'frame_p99_ms'];
  return COMPARE_METRICS;
}

/**
 * Accept a candidate only when every comparable raw-process summary is no
 * worse than the baseline and at least one is strictly better.
 *
 * Comparable reports share schema_version 1, the same named scenario, and
 * the same requested duration_seconds. GPU counters are informational:
 * they may be zero on machines without GPU engine counters and never
 * decide accept/reject.
 */
export function compareReports(baseline, candidate) {
  const left = inspectReport(baseline, 'baseline');
  if (left.error) return incomparable(left.error);
  const right = inspectReport(candidate, 'candidate');
  if (right.error) return incomparable(right.error);
  if (left.scenario !== right.scenario) {
    return incomparable(
      `scenario ${left.scenario} vs ${right.scenario} is not comparable`,
    );
  }
  if (left.durationSeconds !== right.durationSeconds) {
    return incomparable(
      `duration_seconds ${left.durationSeconds} vs ${right.durationSeconds} is not comparable`,
    );
  }

  if (left.workloadID !== right.workloadID) return incomparable('workload_id differs');
  if (baseline.gpu_counters_enabled !== candidate.gpu_counters_enabled) return incomparable('GPU sampling mode differs');
  if (baseline.cpu_sampling_version !== candidate.cpu_sampling_version) return incomparable('CPU sampling version differs');
  const metrics = {};
  let improved = 0;
  let worsened = 0;
  for (const name of metricsFor(left.scenario)) {
    const from = left.summary[name];
    const to = right.summary[name];
    metrics[name] = { baseline: from, candidate: to, delta: to - from };
    if (to < from) improved += 1;
    else if (to > from) worsened += 1;
  }

  if (worsened > 0) {
    return result({
      accept: false,
      verdict: 'regress',
      reason: worsenedReason(metrics),
      scenario: left.scenario,
      duration_seconds: left.durationSeconds,
      metrics,
    });
  }
  if (improved > 0) {
    return result({
      accept: true,
      verdict: 'improve',
      reason: improvedReason(metrics),
      scenario: left.scenario,
      duration_seconds: left.durationSeconds,
      metrics,
    });
  }
  return result({
    accept: false,
    verdict: 'unchanged',
    reason: `${metricsFor(left.scenario).join(', ')} are unchanged`,
    scenario: left.scenario,
    duration_seconds: left.durationSeconds,
    metrics,
  });
}

function parseReportText(text) {
  return JSON.parse(String(text).replace(/^\uFEFF/, ''));
}

export function runCompareCli(argv, io = {
  readFile: readFileSync,
  write: (text) => process.stdout.write(text),
  writeError: (text) => process.stderr.write(text),
}) {
  const parsed = parseCompareArgs(argv);
  if (parsed.error) {
    io.writeError(`${parsed.error}\n`);
    return 2;
  }
  let baseline;
  let candidate;
  try {
    baseline = parseReportText(io.readFile(parsed.baseline, { encoding: 'utf8' }));
    candidate = parseReportText(io.readFile(parsed.candidate, { encoding: 'utf8' }));
  } catch (error) {
    io.writeError(`${error instanceof Error ? error.message : error}\n`);
    return 2;
  }
  const comparison = compareReports(baseline, candidate);
  io.write(`${JSON.stringify(comparison, null, 2)}\n`);
  return comparison.accept ? 0 : 1;
}

function parseCompareArgs(argv) {
  let baseline;
  let candidate;
  for (let i = 0; i < argv.length; i += 1) {
    const flag = argv[i];
    const value = argv[i + 1];
    if (flag === '--baseline' && value) {
      baseline = value;
      i += 1;
      continue;
    }
    if (flag === '--candidate' && value) {
      candidate = value;
      i += 1;
      continue;
    }
    return { error: `unknown argument ${flag}` };
  }
  if (!baseline || !candidate) {
    return { error: 'usage: compare-efficiency.mjs --baseline <path> --candidate <path>' };
  }
  return { baseline, candidate };
}

function inspectReport(report, label) {
  if (!isPlainObject(report)) {
    return { error: `${label} is not a report object` };
  }
  if (report.schema_version !== SCHEMA_VERSION) {
    return { error: `${label} schema_version must be ${SCHEMA_VERSION}` };
  }
  if (!SCENARIO_SET.has(report.scenario)) {
    return { error: `${label} scenario must be one of ${SCENARIOS.join(', ')}` };
  }
  if (!Number.isFinite(report.duration_seconds) || report.duration_seconds <= 0) {
    return { error: `${label} duration_seconds must be a positive number` };
  }
  if (!isPlainObject(report.summary)) {
    return { error: `${label} summary is required` };
  }
  if (!isPlainObject(report.roles)) {
    return { error: `${label} roles breakdown is required` };
  }
  if (report.scenario.startsWith('tactical-') && (typeof report.workload_id !== 'string' || report.workload_id === '')) {
    return { error: `${label} workload_id is required for tactical comparisons` };
  }
  const summary = {};
  for (const name of metricsFor(report.scenario)) {
    const value = report.summary[name];
    if (!Number.isFinite(value) || value < 0) {
      return { error: `${label} summary.${name} must be numeric` };
    }
    summary[name] = value;
  }
  return {
    scenario: report.scenario,
    durationSeconds: report.duration_seconds,
    workloadID: report.workload_id,
    summary,
  };
}

function worsenedReason(metrics) {
  const parts = [];
  for (const [name, row] of Object.entries(metrics)) {
    if (row.delta > 0) parts.push(`${name} rose from ${row.baseline} to ${row.candidate}`);
  }
  return parts.join('; ');
}

function improvedReason(metrics) {
  const parts = [];
  for (const [name, row] of Object.entries(metrics)) {
    if (row.delta < 0) parts.push(`${name} fell from ${row.baseline} to ${row.candidate}`);
  }
  return parts.join('; ');
}

function incomparable(reason) {
  return result({
    accept: false,
    verdict: 'incomparable',
    reason,
    scenario: null,
    duration_seconds: null,
    metrics: {},
  });
}

function result(value) {
  return {
    accept: value.accept,
    verdict: value.verdict,
    reason: value.reason,
    scenario: value.scenario,
    duration_seconds: value.duration_seconds,
    metrics: value.metrics,
  };
}

function isPlainObject(value) {
  return value !== null && typeof value === 'object' && !Array.isArray(value);
}

const invokedDirectly = process.argv[1]
  && pathToFileURL(process.argv[1]).href === import.meta.url;
if (invokedDirectly) {
  process.exitCode = runCompareCli(process.argv.slice(2));
}
