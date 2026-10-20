import test from 'node:test';
import assert from 'node:assert/strict';
import { loginGate } from './login-gate.ts';
import type { CloudAccount } from './parse.ts';

function account(overrides: Partial<CloudAccount>): CloudAccount {
  return {
    portalUrl: 'https://cliphub.rechedev.cloud', linked: false, link: null, user: null, access: null, limits: null,
    usage: null, kinds: [], queue: null, needsApproval: false, error: null, ...overrides,
  };
}

test('a Studio without an account asks to sign in, also when the portal cannot be reached', () => {
  assert.equal(loginGate({ kind: 'ready', account: account({}) }), 'sign-in');
  assert.equal(loginGate({ kind: 'ready', account: account({ error: 'portal_unreachable' }) }), 'sign-in');
  assert.equal(loginGate({ kind: 'ready', account: account({ error: 'unauthorized' }) }), 'sign-in');
});

test('a linked Studio opens even while the portal is unreachable or the account is pending', () => {
  assert.equal(loginGate({ kind: 'ready', account: account({ linked: true, access: 'pending' }) }), 'open');
  assert.equal(loginGate({ kind: 'ready', account: account({ linked: true, error: 'portal_unreachable' }) }), 'open');
});

test('a capture worker or a stopped local service is never gated, and loading waits', () => {
  assert.equal(loginGate({ kind: 'unavailable' }), 'open');
  assert.equal(loginGate({ kind: 'offline' }), 'open');
  assert.equal(loginGate({ kind: 'loading' }), 'checking');
});
