import assert from 'node:assert/strict';
import test from 'node:test';
import { JOB_WAIT_TIMEOUT_CODE, SERVICE_UNAVAILABLE_CODE } from './api/types.ts';
import { MUTATION_CAPABILITY_ERROR } from './api/local-request-guard.ts';
import {
  DEMO_LIST_FAIL_HINT,
  DEMO_PARSE_FAIL_HINT,
  DEMO_SCAN_FAIL_HINT,
  DEMO_SCAN_HINTS,
  DEMO_SERIES_PARSE_FAIL_HINT,
  DEMO_SERVICE_OFFLINE_HINT,
  demoListLoadError,
  demoParseError,
  demoScanError,
  demoSeriesParseError,
  isDemoServiceUnavailable,
} from './demo-parse-flow.ts';

test('detects the orchestrator-down code without treating other errors as offline', () => {
  const cases: Array<{ name: string; value: unknown; want: boolean }> = [
    { name: 'unavailable', value: { code: SERVICE_UNAVAILABLE_CODE }, want: true },
    { name: 'other code', value: { code: 'bad_demo' }, want: false },
    { name: 'error instance', value: Object.assign(new Error('down'), { code: SERVICE_UNAVAILABLE_CODE }), want: true },
    { name: 'plain error', value: new Error('boom'), want: false },
    { name: 'null', value: null, want: false },
    { name: 'string', value: 'service_unavailable', want: false },
  ];
  for (const row of cases) {
    assert.equal(isDemoServiceUnavailable(row.value), row.want, row.name);
  }
});

test('list, scan, and parse copy distinguish offline from a failed request', () => {
  const offline = { code: SERVICE_UNAVAILABLE_CODE };
  const other = { code: 'bad_demo' };
  const cases: Array<{ name: string; fn: (err: unknown) => string; err: unknown; want: string }> = [
    { name: 'list offline', fn: demoListLoadError, err: offline, want: DEMO_SERVICE_OFFLINE_HINT },
    { name: 'list other', fn: demoListLoadError, err: other, want: DEMO_LIST_FAIL_HINT },
    { name: 'scan offline', fn: demoScanError, err: offline, want: DEMO_SERVICE_OFFLINE_HINT },
    { name: 'scan other', fn: demoScanError, err: other, want: DEMO_SCAN_FAIL_HINT },
    { name: 'parse offline', fn: demoParseError, err: offline, want: DEMO_SERVICE_OFFLINE_HINT },
    { name: 'parse other', fn: demoParseError, err: other, want: DEMO_PARSE_FAIL_HINT },
  ];
  for (const row of cases) {
    assert.equal(row.fn(row.err), row.want, row.name);
  }
});

// A user's .dem failed with "No se pudo escanear esa demo. Prueba con otro
// archivo .dem." for every cause; each one now gets advice that fits it.
test('scan copy names the cause the orchestrator or proxy reported', () => {
  const coded = (code: string, extra: Record<string, unknown> = {}) => Object.assign(new Error(code), { code, ...extra });
  const cases: Array<{ name: string; err: unknown; want: string }> = [
    { name: 'CS:GO demo rejected at upload', err: coded('csgo_demo', { status: 400 }), want: DEMO_SCAN_HINTS.csgo },
    { name: 'not a demo', err: coded('not_a_demo', { status: 400 }), want: DEMO_SCAN_HINTS.notDemo },
    { name: 'broken .dem.zst', err: coded('unreadable_demo', { status: 400 }), want: DEMO_SCAN_HINTS.unreadable },
    { name: 'scan job failed parsing', err: coded('demo_incompatible'), want: DEMO_SCAN_HINTS.incompatible },
    { name: 'orchestrator 413', err: coded('payload_too_large', { status: 413 }), want: DEMO_SCAN_HINTS.tooLarge },
    { name: 'proxy 413 without code', err: Object.assign(new Error('file too large'), { status: 413 }), want: DEMO_SCAN_HINTS.tooLarge },
    { name: 'scan never finished', err: coded(JOB_WAIT_TIMEOUT_CODE), want: DEMO_SCAN_HINTS.timeout },
    {
      name: 'expired session capability',
      err: Object.assign(new Error(MUTATION_CAPABILITY_ERROR), { status: 403 }),
      want: DEMO_SCAN_HINTS.session,
    },
    { name: 'other 403', err: Object.assign(new Error('cross-site request blocked'), { status: 403 }), want: DEMO_SCAN_FAIL_HINT },
    { name: 'inherited key is not a code', err: coded('toString'), want: DEMO_SCAN_FAIL_HINT },
  ];
  for (const row of cases) {
    assert.equal(demoScanError(row.err), row.want, row.name);
  }
});

// A failed parse job's error message is the worker's raw English reason; a
// series map row must show Spanish advice, never that text.
test('series parse copy never surfaces the raw worker reason', () => {
  const raw = 'parsing demo: demo_incompatible: parser panicked: proto: cannot parse invalid wire-format data';
  assert.equal(demoSeriesParseError(Object.assign(new Error(raw), { code: 'demo_incompatible' })), DEMO_SCAN_HINTS.incompatible);
  assert.equal(demoSeriesParseError(new Error('target steamid 76561198000000000 not found in demo')), DEMO_SERIES_PARSE_FAIL_HINT);
  assert.equal(demoSeriesParseError(Object.assign(new Error('offline'), { code: SERVICE_UNAVAILABLE_CODE })), DEMO_SERVICE_OFFLINE_HINT);
});
