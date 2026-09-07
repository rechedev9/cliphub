import test from 'node:test';
import assert from 'node:assert/strict';
import { BRIDGE_ENVIRONMENT_KEYS, bridgeEnvironment } from './bridge-environment.ts';
import { createOrchestratorEnvironment } from './orchestrator-environment.ts';

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

test('bridgeEnvironment: covers every key the bridge reads', () => {
  assert.deepEqual([...BRIDGE_ENVIRONMENT_KEYS], ['ZV_BRIDGE_URL', 'ZV_BRIDGE_TOKEN']);
});

test('orchestrator environment carries the bridge settings through to the process', () => {
  const env = createOrchestratorEnvironment({
    dataDir: 'data',
    httpAddress: '127.0.0.1:8080',
    musicDir: 'music',
    recorderPath: 'bin/zv-recorder.exe',
    securityEnvironment: {},
    toolEnvironment: {},
    bridgeEnvironment: bridgeEnvironment({
      ZV_BRIDGE_URL: 'https://portal.example',
      ZV_BRIDGE_TOKEN: 'abc',
    }),
  });

  assert.equal(env.ZV_BRIDGE_URL, 'https://portal.example');
  assert.equal(env.ZV_BRIDGE_TOKEN, 'abc');
  assert.equal(env.ZV_RECORDER_PATH, 'bin/zv-recorder.exe');
});

test('orchestrator environment omits the block entirely when the bridge is off', () => {
  const env = createOrchestratorEnvironment({
    dataDir: 'data',
    httpAddress: '127.0.0.1:8080',
    musicDir: 'music',
    recorderPath: 'bin/zv-recorder.exe',
    securityEnvironment: {},
    toolEnvironment: {},
    bridgeEnvironment: bridgeEnvironment({}),
  });

  for (const key of BRIDGE_ENVIRONMENT_KEYS) {
    assert.equal(key in env, false, `${key} should be absent, not empty`);
  }
});
