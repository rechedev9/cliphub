import assert from 'node:assert/strict';
import test from 'node:test';
import { parseReportPdfRequest, safePdfFileName } from './report-pdf-ipc.ts';

test('accepts a report document with a clean file name', () => {
  assert.deepEqual(
    parseReportPdfRequest({ html: '<!doctype html><p>Informe</p>', fileName: 'Vitality Mirage.pdf' }),
    { html: '<!doctype html><p>Informe</p>', fileName: 'Vitality Mirage.pdf' },
  );
});

test('rejects malformed, empty and oversized report requests', () => {
  for (const value of [
    null,
    [],
    'html',
    { html: '<p>x</p>' },
    { html: '<p>x</p>', fileName: 'a', extra: true },
    { html: 42, fileName: 'a' },
    { html: '   ', fileName: 'a' },
    { html: 'x'.repeat(16 * 1024 * 1024 + 1), fileName: 'a' },
  ]) {
    assert.throws(() => parseReportPdfRequest(value));
  }
});

test('file names cannot leave the chosen folder or use reserved characters', () => {
  assert.equal(safePdfFileName('..\\..\\Windows\\evil'), 'Windows evil.pdf');
  assert.equal(safePdfFileName('../informe/../final.pdf'), 'informe final.pdf');
  assert.equal(safePdfFileName('a/b:c*d?"e<f>g|h'), 'a b c d e f g h.pdf');
  assert.equal(safePdfFileName('...'), 'informe.pdf');
  assert.equal(safePdfFileName(''), 'informe.pdf');
  assert.equal(safePdfFileName('x'.repeat(200)).length, 84);
});
