import assert from 'node:assert/strict';
import test from 'node:test';
import { parseTags, visibleByStatus } from './status.ts';

test('parseTags trims, dedupes without case and drops empty tags', () => {
  assert.deepEqual(parseTags(' eco ,  Rápida,ECO,, B   lenta '), ['eco', 'Rápida', 'B lenta']);
  assert.deepEqual(parseTags(''), []);
});

test('parseTags cuts long tags and caps the count', () => {
  assert.equal(parseTags('x'.repeat(50))[0]?.length, 40);
  assert.equal(parseTags(Array.from({ length: 40 }, (_, i) => `t${i}`).join(',')).length, 32);
});

test('visibleByStatus hides archived rows unless they are asked for', () => {
  const rows = [{ status: 'active' as const }, { status: 'archived' as const }, { status: 'draft' as const }];
  assert.equal(visibleByStatus(rows, 'any', 'any').length, 2);
  assert.equal(visibleByStatus(rows, 'archived', 'any').length, 1);
  assert.equal(visibleByStatus(rows, 'draft', 'any').length, 1);
});
