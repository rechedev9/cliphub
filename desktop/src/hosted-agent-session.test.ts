import assert from 'node:assert/strict';
import test from 'node:test';
import { buildHostedStudioURL } from './hosted-agent-session.ts';

const CAPABILITY = 'a'.repeat(64);

test('builds a hosted URL with a one-time pairing query and browser-local capability fragment', () => {
  assert.equal(buildHostedStudioURL({
    webOrigin: 'https://cliphub.example',
    localWebPort: 43123,
    browserCapability: CAPABILITY,
    pairingCode: 'ABCDE23456',
  }), `https://cliphub.example/connect?pair=ABCDE23456#agent=43123.${CAPABILITY}`);
});

test('builds returning-device URL without a pairing code and rejects unsafe inputs', () => {
  assert.equal(buildHostedStudioURL({
    webOrigin: 'https://cliphub.example',
    localWebPort: 43123,
    browserCapability: CAPABILITY,
  }), `https://cliphub.example/connect#agent=43123.${CAPABILITY}`);
  assert.throws(() => buildHostedStudioURL({
    webOrigin: 'http://cliphub.example', localWebPort: 43123, browserCapability: CAPABILITY,
  }), /must use HTTPS/);
  assert.throws(() => buildHostedStudioURL({
    webOrigin: 'https://cliphub.example', localWebPort: 0, browserCapability: CAPABILITY,
  }), /port is invalid/);
});
