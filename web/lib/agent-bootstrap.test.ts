import assert from 'node:assert/strict';
import test from 'node:test';
import { parseAgentFragment, parseStoredAgentConfig } from './agent-bootstrap.ts';

const CAPABILITY = 'a'.repeat(64);

test('parses only an explicit loopback port and lowercase capability', () => {
  assert.deepEqual(parseAgentFragment(`#agent=43123.${CAPABILITY}`), {
    version: 1,
    origin: 'http://127.0.0.1:43123',
    capability: CAPABILITY,
  });
  for (const fragment of [
    '',
    `#agent=0.${CAPABILITY}`,
    `#agent=65536.${CAPABILITY}`,
    `#agent=43123.${'A'.repeat(64)}`,
    '#agent=http://evil.example',
  ]) {
    assert.equal(parseAgentFragment(fragment), null, fragment);
  }
});

test('stored transport config is revalidated rather than trusted after JSON.parse', () => {
  const valid = JSON.stringify({ version: 1, origin: 'http://127.0.0.1:43123', capability: CAPABILITY });
  assert.deepEqual(parseStoredAgentConfig(valid), parseAgentFragment(`#agent=43123.${CAPABILITY}`));
  assert.equal(parseStoredAgentConfig('{'), null);
  assert.equal(parseStoredAgentConfig(JSON.stringify({ version: 1, origin: 'https://evil.example', capability: CAPABILITY })), null);
});
