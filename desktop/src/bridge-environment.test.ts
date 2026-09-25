import test from 'node:test';
import assert from 'node:assert/strict';
import { bridgeEnvironment } from './bridge-environment.ts';

const CASES: Array<{ name: string; source: NodeJS.ProcessEnv; want: NodeJS.ProcessEnv }> = [
  { name: 'nothing set', source: {}, want: {} },
  {
    name: 'both set',
    source: { ZV_BRIDGE_URL: 'https://portal.example', ZV_BRIDGE_TOKEN: 'abc' },
    want: { ZV_BRIDGE_URL: 'https://portal.example', ZV_BRIDGE_TOKEN: 'abc' },
  },
  {
    name: 'url alone forwards nothing, because a lone value is a startup error',
    source: { ZV_BRIDGE_URL: 'https://portal.example' },
    want: {},
  },
  {
    name: 'token alone forwards nothing',
    source: { ZV_BRIDGE_TOKEN: 'abc' },
    want: {},
  },
  {
    name: 'whitespace counts as unset',
    source: { ZV_BRIDGE_URL: '   ', ZV_BRIDGE_TOKEN: 'abc' },
    want: {},
  },
  {
    name: 'unrelated variables are never forwarded',
    source: {
      ZV_BRIDGE_URL: 'https://portal.example',
      ZV_BRIDGE_TOKEN: 'abc',
      ZV_MUTATION_TOKEN: 'nope',
    },
    want: { ZV_BRIDGE_URL: 'https://portal.example', ZV_BRIDGE_TOKEN: 'abc' },
  },
];

for (const testCase of CASES) {
  test(`bridgeEnvironment: ${testCase.name}`, () => {
    assert.deepEqual(bridgeEnvironment(testCase.source), testCase.want);
  });
}
