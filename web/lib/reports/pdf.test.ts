import test from 'node:test';
import assert from 'node:assert/strict';
import { saveReportPdf } from './pdf.ts';

test('uses the desktop bridge and reads its answer', async () => {
  const calls: string[] = [];
  const bridge = (answer: unknown) => ({
    cliphubReports: {
      savePdf: async (html: string, fileName: string) => {
        calls.push(`${fileName}:${html.length}`);
        return answer;
      },
    },
  });
  assert.deepEqual(await saveReportPdf('<p>x</p>', 'a.pdf', bridge({ ok: true })), { kind: 'saved' });
  assert.deepEqual(await saveReportPdf('<p>x</p>', 'a.pdf', bridge({ ok: false, canceled: true })), { kind: 'canceled' });
  assert.deepEqual(await saveReportPdf('<p>x</p>', 'a.pdf', bridge({ ok: false, error: 'Disco lleno.' })), { kind: 'failed', message: 'Disco lleno.' });
  assert.deepEqual(await saveReportPdf('<p>x</p>', 'a.pdf', bridge(null)), { kind: 'failed', message: 'No se pudo guardar el PDF.' });
  assert.deepEqual(calls, ['a.pdf:8', 'a.pdf:8', 'a.pdf:8', 'a.pdf:8']);
});

test('without the bridge the caller is told to print instead', async () => {
  assert.deepEqual(await saveReportPdf('<p>x</p>', 'a.pdf', {}), { kind: 'unavailable' });
  assert.deepEqual(await saveReportPdf('<p>x</p>', 'a.pdf', { cliphubReports: { savePdf: 'no' } }), { kind: 'unavailable' });
});
