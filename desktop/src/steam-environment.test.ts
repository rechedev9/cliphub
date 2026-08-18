import test from 'node:test';
import assert from 'node:assert/strict';
import { STEAM_ENVIRONMENT_KEYS, steamEnvironment } from './steam-environment.ts';
import { createOrchestratorEnvironment } from './orchestrator-environment.ts';

const CASES: Array<{ name: string; source: NodeJS.ProcessEnv; want: NodeJS.ProcessEnv }> = [
  { name: 'nothing set', source: {}, want: {} },
  {
    name: 'all three set',
    source: { ZV_STEAM_USERNAME: 'user', ZV_STEAM_PASSWORD: 'pw', ZV_STEAM_GUARD: 'secret' },
    want: { ZV_STEAM_USERNAME: 'user', ZV_STEAM_PASSWORD: 'pw', ZV_STEAM_GUARD: 'secret' },
  },
  {
    name: 'empty and whitespace values are dropped, not forwarded as configured',
    source: { ZV_STEAM_USERNAME: 'user', ZV_STEAM_PASSWORD: '', ZV_STEAM_GUARD: '   ' },
    want: { ZV_STEAM_USERNAME: 'user' },
  },
  {
    name: 'unrelated variables are never forwarded',
    source: { ZV_STEAM_USERNAME: 'user', PATH: '/bin', ZV_HLAE_PATH: 'C:\\HLAE.exe' },
    want: { ZV_STEAM_USERNAME: 'user' },
  },
];

for (const testCase of CASES) {
  test(`steamEnvironment: ${testCase.name}`, () => {
    assert.deepEqual(steamEnvironment(testCase.source), testCase.want);
  });
}

test('steamEnvironment: covers every key steamresolve reads', () => {
  assert.deepEqual([...STEAM_ENVIRONMENT_KEYS], [
    'ZV_STEAM_USERNAME',
    'ZV_STEAM_PASSWORD',
    'ZV_STEAM_GUARD',
  ]);
});

test('orchestrator environment carries the credentials through to the process', () => {
  const env = createOrchestratorEnvironment({
    dataDir: 'data',
    httpAddress: '127.0.0.1:8080',
    musicDir: 'music',
    recorderPath: 'bin/zv-recorder.exe',
    securityEnvironment: {},
    toolEnvironment: { ZV_HLAE_PATH: 'tools/HLAE.exe' },
    steamEnvironment: steamEnvironment({ ZV_STEAM_USERNAME: 'user', ZV_STEAM_PASSWORD: 'pw' }),
  });

  assert.equal(env.ZV_STEAM_USERNAME, 'user');
  assert.equal(env.ZV_STEAM_PASSWORD, 'pw');
  // The recorder pin still wins: credentials must not disturb that ordering.
  assert.equal(env.ZV_RECORDER_PATH, 'bin/zv-recorder.exe');
  assert.equal(env.ZV_HLAE_PATH, 'tools/HLAE.exe');
});

test('orchestrator environment omits the block entirely when nothing is set', () => {
  const env = createOrchestratorEnvironment({
    dataDir: 'data',
    httpAddress: '127.0.0.1:8080',
    musicDir: 'music',
    recorderPath: 'bin/zv-recorder.exe',
    securityEnvironment: {},
    toolEnvironment: {},
    steamEnvironment: steamEnvironment({}),
  });

  for (const key of STEAM_ENVIRONMENT_KEYS) {
    assert.equal(key in env, false, `${key} should be absent, not empty`);
  }
});
