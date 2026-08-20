import assert from 'node:assert/strict';
import test from 'node:test';
import { SERVICE_UNAVAILABLE_CODE } from './api/types.ts';
import {
  DEMO_LIST_FAIL_HINT,
  DEMO_PARSE_FAIL_HINT,
  DEMO_SCAN_FAIL_HINT,
  DEMO_SERVICE_OFFLINE_HINT,
  demoListLoadError,
  demoParseError,
  demoScanError,
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
