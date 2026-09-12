import assert from 'node:assert/strict';
import { randomUUID } from 'node:crypto';
import test from 'node:test';
import { collectDiagnostics, summarizeDiagnostics } from '../../scripts/telemetry-debug.mjs';

test('remote query follows cursors and session context without mixing jobs', async () => {
  const job = randomUUID(), other = randomUUID(), session = randomUUID(), attempt = randomUUID();
  const support = 'CH-1111-2222-3333-4444-5555';
  const make = (cursor, event, extra = {}) => ({ cursor, id: randomUUID(), support_code: support, session_id: session, sequence: cursor, event, level: 'info', message: event, ...extra });
  const start = make(1, 'attempt.started', {job_id: job, attempt_id: attempt});
  const cause = make(2, 'process.stderr', {job_id: job, attempt_id: attempt, message: 'encoder device unavailable'});
  const finish = make(3, 'attempt.finished', {job_id: job, attempt_id: attempt, level: 'error', outcome: 'error'});
  const context = make(4, 'studio.log');
  const unrelated = make(5, 'attempt.finished', {job_id: other, attempt_id: randomUUID(), level: 'error'});
  const gap = make(6, 'delivery.gap', {lost_records: 9});
  const calls = [];
  const report = await collectDiagnostics({job_id: job}, async (url) => {
    calls.push(url);
    const parsed = new URL(url, 'http://localhost');
    const params = parsed.searchParams;
    if (parsed.pathname === '/v1/incidents') return {events: []};
    if (params.has('job_id')) return params.get('after') === '0'
      ? {records: [start,cause],next_cursor: 2,has_more: true}
      : {records: [finish],next_cursor: 3,has_more: false};
    if (params.has('session_id')) return {records: [start,cause,finish,context,unrelated],next_cursor: 5,has_more: false};
    return {records: params.get('event') === 'delivery.gap' ? [gap] : [],next_cursor: 6,has_more: false};
  });
  assert.equal(report.records.length,5);
  assert.equal(report.attempts.length,1);
  assert.equal(report.attempts[0].state,'error');
  assert.equal(report.evidence.first_failure.id,finish.id);
  assert.ok(report.evidence.failure_context.some((r)=>r.id===cause.id));
  assert.equal(report.evidence.installation_delivery_gaps[0].lost_records,9);
  assert.ok(calls.some((url)=>url.includes('after=2')));
});

test('missing terminal evidence and a retrieval cap cannot imply success', async () => {
  const session = randomUUID();
  const record = {id:randomUUID(),cursor:1,sequence:1,session_id:session,attempt_id:randomUUID(),event:'attempt.started'};
  const summary = summarizeDiagnostics({filters:{session_id:session},records:[record]});
  assert.equal(summary.attempts[0].state,'no_terminal_record');
  assert.equal(summary.evidence.first_failure,null);
  const capped = await collectDiagnostics({session_id:session},async()=>({records:[record,{...record,id:randomUUID(),cursor:2}],next_cursor:2,has_more:false}),1);
  assert.equal(capped.evidence.retrieval_complete,false);
  await assert.rejects(collectDiagnostics({session_id:session},async()=>({records:[],next_cursor:0,has_more:true})),/did not advance/);
});
