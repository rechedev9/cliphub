import assert from 'node:assert/strict';
import { mkdtempSync, readFileSync, rmSync, writeFileSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { dirname, join } from 'node:path';
import { fileURLToPath } from 'node:url';
import test from 'node:test';
import {
  COMPARE_METRICS,
  SCENARIOS,
  compareReports,
  runCompareCli,
} from './compare-efficiency.mjs';

const scriptDir = dirname(fileURLToPath(import.meta.url));
const repoRoot = join(scriptDir, '..', '..');

function report(overrides = {}) {
  const base = {
    schema_version: 1,
    scenario: 'foreground-idle',
    duration_seconds: 15,
    summary: {
      cpu_p95_percent: 12.5,
      gpu_p95_percent: 0,
      working_set_peak_bytes: 400_000_000,
      private_bytes_peak: 280_000_000,
      gpu_memory_peak_bytes: 0,
    },
    roles: {
      'electron-main': {
        cpu_p95_percent: 3.1,
        working_set_peak_bytes: 80_000_000,
        private_bytes_peak: 60_000_000,
      },
    },
  };
  return {
    ...base,
    ...overrides,
    summary: { ...base.summary, ...(overrides.summary ?? {}) },
    roles: overrides.roles ?? base.roles,
  };
}

const cases = [
  {
    name: 'improve',
    baseline: report(),
    candidate: report({
      summary: {
        cpu_p95_percent: 9.25,
        working_set_peak_bytes: 350_000_000,
        private_bytes_peak: 280_000_000,
      },
    }),
    wantAccept: true,
    wantVerdict: 'improve',
  },
  {
    name: 'regress',
    baseline: report(),
    candidate: report({
      summary: {
        cpu_p95_percent: 18.75,
        working_set_peak_bytes: 400_000_000,
        private_bytes_peak: 280_000_000,
      },
    }),
    wantAccept: false,
    wantVerdict: 'regress',
  },
  {
    name: 'incomparable-scenario',
    baseline: report({ scenario: 'foreground-idle' }),
    candidate: report({ scenario: 'background-idle' }),
    wantAccept: false,
    wantVerdict: 'incomparable',
  },
  {
    name: 'unchanged',
    baseline: report(),
    candidate: report(),
    wantAccept: false,
    wantVerdict: 'unchanged',
  },
  {
    name: 'mixed-is-regress',
    baseline: report(),
    candidate: report({
      summary: {
        cpu_p95_percent: 8,
        working_set_peak_bytes: 500_000_000,
      },
    }),
    wantAccept: false,
    wantVerdict: 'regress',
  },
  {
    name: 'incomparable-duration',
    baseline: report({ duration_seconds: 15 }),
    candidate: report({ duration_seconds: 30 }),
    wantAccept: false,
    wantVerdict: 'incomparable',
  },
];

test('compareReports accepts only documented raw-process wins', () => {
  for (const row of cases) {
    const got = compareReports(row.baseline, row.candidate);
    assert.equal(got.verdict, row.wantVerdict, row.name);
    assert.equal(got.accept, row.wantAccept, row.name);
    if (row.wantVerdict === 'improve' || row.wantVerdict === 'regress' || row.wantVerdict === 'unchanged') {
      assert.equal(got.scenario, row.baseline.scenario, row.name);
      assert.equal(got.duration_seconds, row.baseline.duration_seconds, row.name);
      for (const metric of COMPARE_METRICS) {
        assert.equal(got.metrics[metric].baseline, row.baseline.summary[metric], `${row.name} ${metric}`);
        assert.equal(got.metrics[metric].candidate, row.candidate.summary[metric], `${row.name} ${metric}`);
      }
    } else {
      assert.equal(got.scenario, null, row.name);
      assert.equal(got.accept, false, row.name);
    }
  }
});

test('runCompareCli reads Windows PowerShell UTF-8 BOM reports', (t) => {
  const directory = mkdtempSync(join(tmpdir(), 'cliphub-compare-efficiency-bom-'));
  t.after(() => rmSync(directory, { recursive: true, force: true }));
  const baselinePath = join(directory, 'baseline.json');
  const candidatePath = join(directory, 'candidate.json');
  writeFileSync(baselinePath, `\uFEFF${JSON.stringify(report())}`);
  writeFileSync(candidatePath, `\uFEFF${JSON.stringify(report({
    summary: { cpu_p95_percent: 10, working_set_peak_bytes: 300_000_000 },
  }))}`);

  let stdout = '';
  const code = runCompareCli(
    ['--baseline', baselinePath, '--candidate', candidatePath],
    {
      readFile: readFileSync,
      write: (text) => { stdout += text; },
      writeError: () => {},
    },
  );
  const parsed = JSON.parse(stdout);
  assert.equal(code, 0);
  assert.equal(parsed.verdict, 'improve');
});

test('runCompareCli drives the shipped compare entry on two report files', (t) => {
  const directory = mkdtempSync(join(tmpdir(), 'cliphub-compare-efficiency-'));
  t.after(() => rmSync(directory, { recursive: true, force: true }));
  const baselinePath = join(directory, 'baseline.json');
  const candidatePath = join(directory, 'candidate.json');
  writeFileSync(baselinePath, JSON.stringify(report()));
  writeFileSync(candidatePath, JSON.stringify(report({
    summary: { cpu_p95_percent: 10, working_set_peak_bytes: 300_000_000 },
  })));

  let stdout = '';
  const code = runCompareCli(
    ['--baseline', baselinePath, '--candidate', candidatePath],
    {
      readFile: readFileSync,
      write: (text) => { stdout += text; },
      writeError: () => {},
    },
  );
  const parsed = JSON.parse(stdout);
  assert.equal(code, 0);
  assert.equal(parsed.accept, true);
  assert.equal(parsed.verdict, 'improve');
});

test('workflow encodes the four scenarios and the measure-iterate loop', () => {
  const workflow = readFileSync(
    join(repoRoot, '.grok', 'workflows', 'desktop-efficiency.rhai'),
    'utf8',
  );
  for (const scenario of SCENARIOS) {
    assert.match(workflow, new RegExp(scenario));
  }
  assert.match(workflow, /measure-desktop-efficiency\.ps1/);
  assert.match(workflow, /compare-efficiency\.mjs/);
  assert.match(workflow, /persist a baseline|persist baseline/i);
  assert.match(workflow, /remasure|remeasure/i);
  assert.match(workflow, /same scenario/);
  assert.match(workflow, /same(?: requested)? duration|same duration/i);
  assert.match(workflow, /accept only/i);
  assert.match(workflow, /HLAE/);
  assert.match(workflow, /CS2/);
});
