import assert from 'node:assert/strict';
import test from 'node:test';
import { safeNext } from './client.ts';

test('safeNext keeps a path inside the app', () => {
  assert.equal(safeNext('/invitacion?token=t#x'), '/invitacion?token=t#x');
  assert.equal(safeNext(null), '/partidas');
});

test('safeNext refuses any path the router would read as another site', () => {
  for (const value of ['//evil.com', '/\\evil.com', '/\t/evil.com', '/\n/evil.com', 'https://evil.com', 'evil.com']) {
    assert.equal(safeNext(value), '/partidas', value);
  }
});
