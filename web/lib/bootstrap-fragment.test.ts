import assert from 'node:assert/strict';
import test from 'node:test';
import { bootstrapCapabilityFromHash } from './bootstrap-fragment.ts';

const CAPABILITY = 'a'.repeat(64);

test('accepts only a lowercase 32-byte capability from the URL fragment', () => {
  assert.equal(bootstrapCapabilityFromHash(`#${CAPABILITY}`), CAPABILITY);
  assert.equal(bootstrapCapabilityFromHash(CAPABILITY), CAPABILITY);
  assert.equal(bootstrapCapabilityFromHash(`#${'A'.repeat(64)}`), null);
  assert.equal(bootstrapCapabilityFromHash(`#${'a'.repeat(63)}`), null);
  assert.equal(bootstrapCapabilityFromHash('#not-a-capability'), null);
  assert.equal(bootstrapCapabilityFromHash(''), null);
});
