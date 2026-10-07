// Unit tests for the Partidas delete error → Spanish message mapping.
// Run: node --test delete-error.test.ts
import test from 'node:test';
import assert from 'node:assert/strict';
import { SERVICE_UNAVAILABLE_CODE } from './api/types.ts';
import {
  deleteErrorMessage,
  deleteFeedbackTone,
  DELETE_GENERIC_MESSAGE,
  DELETE_JOB_BUSY_MESSAGE,
  DELETE_OFFLINE_MESSAGE,
  DELETE_RENDER_BUSY_MESSAGE,
  DELETE_STREAM_BUSY_MESSAGE,
} from './delete-error.ts';

test('offline code maps to the start-your-orchestrator hint', () => {
  const err = Object.assign(new Error('analysis service unavailable'), { code: SERVICE_UNAVAILABLE_CODE });
  assert.equal(deleteErrorMessage(err), DELETE_OFFLINE_MESSAGE);
});

test('a non-offline error surfaces its message verbatim', () => {
  const cases: { name: string; err: Error }[] = [
    { name: '409 orchestrator explanation', err: new Error('Espera a que termine la captura para borrar') },
    { name: 'non-offline code', err: Object.assign(new Error('job not found'), { code: 'not_found' }) },
  ];
  for (const { name, err } of cases) {
    assert.equal(deleteErrorMessage(err), err.message, name);
  }
});

test('a busy demo never surfaces the English wait sentence', () => {
  const statuses = ['queued', 'scanning', 'parsing', 'recording', 'composing'];
  for (const status of statuses) {
    const raw = `job is ${status}; wait for it to settle before deleting`;
    const err = Object.assign(new Error(raw), { code: 'conflict' });
    const got = deleteErrorMessage(err);
    assert.equal(got, DELETE_JOB_BUSY_MESSAGE, raw);
    assert.doesNotMatch(got, /job is|wait for it/i, raw);
  }
  const rendering = Object.assign(
    new Error('job has an active render or generate run; wait for it to settle before deleting'),
    { code: 'generate_work_active' },
  );
  assert.equal(deleteErrorMessage(rendering), DELETE_RENDER_BUSY_MESSAGE);
  assert.doesNotMatch(deleteErrorMessage(rendering), /job has an active|wait for it/i);
});

test('odd delete failures stay non-empty strings and do not throw', () => {
  const inputs: unknown[] = [
    null,
    undefined,
    0,
    false,
    '',
    '   ',
    'ñ'.repeat(4000),
    'partida «Ñoño» / 玩家',
    '<script>alert(1)</script>',
    'job is ',
    'job is recording; wait for it to settle before deleting ',
    { message: 12 },
    { code: 'conflict', message: 'Espera a que termine la captura para borrar' },
  ];
  for (const input of inputs) {
    const got = deleteErrorMessage(input);
    assert.equal(typeof got, 'string');
    assert.ok(got.trim().length > 0, `empty message for ${JSON.stringify(input)}`);
  }
  assert.equal(
    deleteErrorMessage({ code: 'conflict', message: 'Espera a que termine la captura para borrar' }),
    'Espera a que termine la captura para borrar',
  );
});

test('a busy stream project and a body-less error never surface English text', () => {
  for (const raw of [
    'stream job is rendering; wait for it to settle before deleting',
    'stream job is acquiring; wait for it to settle before deleting',
    'stream render streamer-vertical-stack-40-60 is rendering; wait for it to settle before deleting',
  ]) {
    assert.equal(deleteErrorMessage(new Error(raw)), DELETE_STREAM_BUSY_MESSAGE, raw);
  }
  assert.equal(deleteErrorMessage(new Error('request failed (500)')), DELETE_GENERIC_MESSAGE);
});

test('a busy-demo wait sentence is a notice, and a real failure stays an error', () => {
  assert.equal(deleteFeedbackTone(DELETE_JOB_BUSY_MESSAGE), 'notice');
  assert.equal(deleteFeedbackTone(DELETE_RENDER_BUSY_MESSAGE), 'notice');
  assert.equal(deleteFeedbackTone(DELETE_STREAM_BUSY_MESSAGE), 'notice');
  assert.equal(
    deleteFeedbackTone(deleteErrorMessage(new Error('job is recording; wait for it to settle before deleting'))),
    'notice',
  );
  assert.equal(deleteFeedbackTone(DELETE_OFFLINE_MESSAGE), 'error');
  assert.equal(deleteFeedbackTone(DELETE_GENERIC_MESSAGE), 'error');
  assert.equal(deleteFeedbackTone('job not found'), 'error');
});

test('missing or blank message falls back to the generic retry line', () => {
  assert.equal(deleteErrorMessage(new Error('')), DELETE_GENERIC_MESSAGE);
  assert.equal(deleteErrorMessage(new Error('   ')), DELETE_GENERIC_MESSAGE);
  assert.equal(deleteErrorMessage(null), DELETE_GENERIC_MESSAGE);
  assert.equal(deleteErrorMessage({}), DELETE_GENERIC_MESSAGE);
});
