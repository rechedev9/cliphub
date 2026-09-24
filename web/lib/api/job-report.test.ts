import assert from 'node:assert/strict';
import test from 'node:test';
import {
  isJobReportCategory,
  JOB_REPORT_CATEGORIES,
  jobReportUrl,
  reportableJobId,
  reportDelivery,
  reportJob,
  reportsStayLocal,
} from './job-report.ts';

const JOB = '3f2b8c1e-4d5a-4b6c-8d7e-9f0a1b2c3d4e';
const CONSENTED = { available: true, enabled: true, noticeAcknowledged: true, supportCode: 'CH-AAAA-BBBB-CCCC-DDDD-EEEE' };

test('offers exactly the five contract categories', () => {
  assert.deepEqual(JOB_REPORT_CATEGORIES.map((category) => category.value), ['black_video', 'wrong_overlay', 'audio', 'cuts', 'other']);
  assert.equal(isJobReportCategory('audio'), true);
  assert.equal(isJobReportCategory('bad'), false);
  assert.equal(isJobReportCategory(undefined), false);
});

test('reports only against an orchestrator job id', () => {
  assert.equal(reportableJobId(undefined, JOB), JOB);
  assert.equal(reportableJobId(JOB.toUpperCase(), JOB), JOB.toUpperCase());
  assert.equal(reportableJobId('orphan', 'reel-42'), null);
  assert.equal(reportableJobId(), null);
});

test('posts only the category to the same-origin report route', async () => {
  const calls: Array<{ url: string; body: string; method: string }> = [];
  const result = await reportJob(JOB, 'black_video', async (url, init) => {
    calls.push({ url, body: init.body, method: init.method });
    return { status: 202 };
  });
  assert.deepEqual(result, { ok: true });
  assert.deepEqual(calls, [{ url: `/api/demos/${JOB}/report`, body: '{"category":"black_video"}', method: 'POST' }]);
  assert.equal(jobReportUrl('a/b'), '/api/demos/a%2Fb/report');
});

test('maps each failure to copy the dialog can show', async () => {
  const status = (code: number) => async () => ({ status: code });
  assert.match((await reportJob(JOB, 'cuts', status(429)) as { message: string }).message, /menos de un minuto/);
  assert.match((await reportJob(JOB, 'cuts', status(404)) as { message: string }).message, /ya no existe/);
  assert.match((await reportJob(JOB, 'cuts', status(503)) as { message: string }).message, /no responde/);
  assert.match((await reportJob(JOB, 'cuts', status(500)) as { message: string }).message, /No se pudo enviar/);
  const offline = await reportJob(JOB, 'cuts', async () => { throw new TypeError('fetch failed'); });
  assert.deepEqual(offline, { ok: false, status: 0, message: 'Studio no responde ahora mismo. Inténtalo de nuevo en unos segundos.' });
});

test('only promises delivery when diagnostics can leave the machine', () => {
  assert.deepEqual(reportDelivery(CONSENTED), { tone: 'sent', text: 'Enviado. Código de soporte: CH-AAAA-BBBB-CCCC-DDDD-EEEE' });
  const declined = reportDelivery({ ...CONSENTED, enabled: false });
  assert.equal(declined.tone, 'local');
  assert.match(declined.text, /solo en este equipo/);
  assert.match(declined.text, /CH-AAAA/);
  assert.equal(reportDelivery({ ...CONSENTED, noticeAcknowledged: false }).tone, 'local');
  assert.equal(reportDelivery({ ...CONSENTED, available: false }).tone, 'local');
  assert.deepEqual(reportDelivery(null), { tone: 'local', text: 'Guardado en Studio.' });
  assert.equal(reportsStayLocal(CONSENTED), false);
  assert.equal(reportsStayLocal({ ...CONSENTED, enabled: false }), true);
  assert.equal(reportsStayLocal(null), false, 'a plain browser has no desktop consent to warn about');
});
