import assert from 'node:assert/strict';
import { test } from 'node:test';
import { humanErrorText, serverError } from './human-error.ts';

const LEAD = 'No se pudo guardar el kit de marca.';

test('a server answer never shows its own text', () => {
  const text = humanErrorText({ error: serverError('brand kit: invalid opacity', { status: 400 }), lead: LEAD, edition: 'desktop' });
  assert.equal(text, `${LEAD} Vuelve a intentarlo.`);
  assert.doesNotMatch(text, /opacity/);
});

test('a known code reads as its own sentence', () => {
  const error = serverError('Wait for the active job', { status: 409 }, 'generate_work_active');
  assert.equal(humanErrorText({ error, lead: LEAD, codes: { generate_work_active: 'Espera a que termine.' }, edition: 'desktop' }), 'Espera a que termine.');
});

test('a lost connection says how to get it back in each edition', () => {
  assert.match(humanErrorText({ error: new TypeError('Failed to fetch'), lead: LEAD, edition: 'desktop' }), /cierra ClipHub Studio/);
  assert.match(humanErrorText({ error: serverError('Bad gateway', { status: 502 }), lead: LEAD, edition: 'web' }), /ClipMind/);
});

test('a check made before sending keeps its own Spanish sentence', () => {
  assert.equal(humanErrorText({ error: new Error('Elige una imagen PNG o JPG.'), lead: LEAD, edition: 'desktop' }), 'Elige una imagen PNG o JPG.');
  assert.equal(humanErrorText({ error: 'boom', lead: LEAD, edition: 'desktop' }), `${LEAD} Vuelve a intentarlo.`);
});
