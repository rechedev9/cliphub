import assert from 'node:assert/strict';
import * as fs from 'node:fs';
import test from 'node:test';
import { diagnosticMessage, MAX_DIAGNOSTIC_BYTES } from './diagnostic-message.ts';

const fixtures: Array<{ name: string; input: string; expected: string }> = JSON.parse(
  fs.readFileSync(new URL('../../testdata/telemetry-diagnostics.json', import.meta.url), 'utf8'),
);

for (const fixture of fixtures) {
  test(`diagnostic message: ${fixture.name}`, () => {
    assert.equal(diagnosticMessage(fixture.input), fixture.expected);
    assert.equal(diagnosticMessage(fixture.expected), fixture.expected, 'filter is idempotent');
  });
}

test('long UTF-8 errors preserve the beginning and final cause within the wire limit', () => {
  const text = diagnosticMessage(`recorder failed: ${'é漢🙂 '.repeat(2000)}\nFinal cause: encoder device unavailable`);
  assert.ok(Buffer.byteLength(text) <= MAX_DIAGNOSTIC_BYTES);
  assert.match(text, /^recorder failed:/);
  assert.match(text, /Final cause: encoder device unavailable$/);
  assert.doesNotMatch(text, /\ufffd/);
  assert.equal(diagnosticMessage(text), text);
});

test('Error causes keep network codes and stop on a cycle', () => {
  const cause = Object.assign(new Error('connect failed'), { code: 'ECONNRESET' });
  const error = new Error('download failed', { cause });
  cause.cause = error;
  assert.equal(diagnosticMessage(error), 'Error: download failed\nCaused by: Error (ECONNRESET): connect failed');
  assert.equal(diagnosticMessage({ credentials: 'do not serialize arbitrary objects' }), '');
});
